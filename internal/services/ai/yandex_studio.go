package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"ReAction/internal/config"
)

const (
	yandexGPTAPIURL    = "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"
	modelYandexGPT     = "yandexgpt"
	modelYandexGPTLite = "yandexgpt-lite"
)

type AIService struct {
	config     config.AIConfig
	httpClient *http.Client
}

func NewYandexGPTService(cfg *config.AppConfig) (*AIService, error) {
	if cfg.AIConfig.FolderID == "" {
		return nil, fmt.Errorf("Yandex GPT: YANDEX_AI_FOLDER_ID is required")
	}
	if cfg.AIConfig.APIKey == "" && cfg.AIConfig.YandexIamToken == "" {
		return nil, fmt.Errorf("Yandex GPT: set YANDEX_AI_API_KEY (recommended) or YANDEX_IAM_TOKEN")
	}

	httpClient := &http.Client{
		Timeout: 100 * time.Second,
	}

	service := &AIService{
		config:     cfg.AIConfig,
		httpClient: httpClient,
	}

	return service, nil
}

func (s *AIService) setAuthHeader(req *http.Request) {
	if s.config.APIKey != "" {
		req.Header.Set("Authorization", "Api-Key "+s.config.APIKey)
		return
	}
	req.Header.Set("Authorization", "Bearer "+s.config.YandexIamToken)
}

func getScenariosToSearch(scenarios []UserScenarioForAI, existingReminders []ExistingReminderForAI) []UserScenarioForAI {
	existing := make(map[string]struct{})
	for _, r := range existingReminders {
		existing[r.ScenarioId] = struct{}{}
	}

	var missing []UserScenarioForAI
	for _, s := range scenarios {
		if _, found := existing[s.ID]; !found {
			missing = append(missing, s)
		}
	}

	return missing
}

func (s *AIService) CheckMessageWithHistoryAndScenarios(
	ctx context.Context,
	history []string,
	scenarios []UserScenarioForAI,
	existingReminders []ExistingReminderForAI,
) (*CheckResult, error) {
	if len(history) == 0 {
		return nil, fmt.Errorf("empty message history")
	}

	prompt := ""
	if len(history) > 1 {
		prompt += `Ты ищешь новые ключевые фразы в messages (из входящего JSON: scenarios [{id, trigger_phrase}], messages [строки], history [id]).
		Найди trigger_phrase из scenarios, которые есть в messages.
		Верни массив id таких фраз. Если новых нет — верни пустой массив.`
	}

	scenariosToFind := getScenariosToSearch(scenarios, existingReminders)
	if len(scenariosToFind) > 0 {
		raw, err := json.Marshal(scenarios)
		if err != nil {
			return nil, fmt.Errorf("marshal scenarios: %w", err)
		}
		prompt += `Ключевые фразы: ` + string(raw) +
			`. Для каждой ключевой фразы которая подошла: detected=true, scenario_id = поле id ключевой фразы (строка). Eсли ни один сценарий не подходит или выбор неоднозначен: detected=false, scenario_id="".`
	} else {
		prompt += `Если список сценариев в этом запросе пуст: всегда scenario_id="".`
	}

	now := time.Now()
	endOfDay := time.Date(
		now.Year(), now.Month(), now.Day(),
		23, 59, 59, 0,
		now.Location(),
	)

	prompt += `Если detected=true, заполни объект reminder:
		title — название встречи или задачи;
		description — краткое описание (1–2 предложения);
		datetime — начало события в ISO 8601 с часовым поясом;
		end_datetime — конец события в ISO 8601 с часовым поясом.
		Считай: что утро с 06:00:00Z до 12:00:00Z, день/обед с 12:00:00Z до 17:00:00Z, вечер с 17:00:00Z до 22:00:00Z, ночь с 22:00:00Z до 06:00:00Z.
		Если по переписке нельзя определить datetime, то возвращай ` + now.Format(time.RFC3339) +
		`. Если по переписке нельзя определить end_datetime, то возвращай ` + endOfDay.Format(time.RFC3339) +
		`. Если detected=false, то scenario_id="" и reminder: title="", description="", datetime="", end_datetime="".`

	systemMessage := Message{
		Role: "system",
		Text: prompt,
	}

	userMessage := Message{
		Role: "user",
		Text: formatHistoryForModel(history),
	}

	return s.makeAPIRequest(ctx, systemMessage, userMessage, classificationResultSchema())
}

func formatHistoryForModel(messages []string) string {
	if len(messages) == 1 {
		return messages[0]
	}
	var b strings.Builder
	b.WriteString("Фрагмент чата от старого к новому:\n\n")
	for i := 0; i < len(messages)-1; i++ {
		fmt.Fprintf(&b, "%s\n", messages[i])
	}
	fmt.Fprintf(&b, "\n%s", messages[len(messages)-1])
	return b.String()
}

type JsonSchema struct {
	Schema interface{} `json:"schema"`
}

type yandexGPTRequest struct {
	ModelURI          string            `json:"modelUri"`
	CompletionOptions CompletionOptions `json:"completionOptions"`
	Messages          []Message         `json:"messages"`
	JsonSchema        *JsonSchema       `json:"jsonSchema,omitempty"`
}

type yandexGPTResponse struct {
	Result struct {
		Alternatives []struct {
			Message struct {
				Role string `json:"role"`
				Text string `json:"text"`
			} `json:"message"`
			Status string `json:"status"`
		} `json:"alternatives"`
		Usage struct {
			InputTextTokens  json.Number `json:"inputTextTokens"`
			CompletionTokens json.Number `json:"completionTokens"`
			TotalTokens      json.Number `json:"totalTokens"`
		} `json:"usage"`
	} `json:"result"`
}

func classificationResultSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"detected": map[string]interface{}{
				"type":        "boolean",
				"description": "Есть ли в фрагменте переписки один из сценариев пользователя.",
			},
			"confidence": map[string]interface{}{
				"type":        "number",
				"description": "Уверенность от 0 до 1.",
			},
			"reason": map[string]interface{}{
				"type":        "string",
				"description": "Краткое обоснование на русском.",
			},
			"scenario_id": map[string]interface{}{
				"type":        "string",
				"description": "UUID сценария из списка при detected=true; иначе пустая строка.",
			},
			"reminder": map[string]interface{}{
				"type":        "object",
				"description": "При detected=true — данные напоминания; при detected=false — все поля пустые строки.",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Название встречи или задачи.",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Краткое описание.",
					},
					"datetime": map[string]interface{}{
						"type":        "string",
						"description": "Начало события, ISO 8601 с оффсетом, например 2026-04-02T14:00:00+03:00",
					},
					"end_datetime": map[string]interface{}{
						"type":        "string",
						"description": "Окончание события, ISO 8601 с оффсетом; при неизвестности — через 1 час после начала.",
					},
				},
				"required": []string{"title", "description", "datetime", "end_datetime"},
			},
		},
		"required": []string{"detected", "confidence", "reason", "scenario_id", "reminder"},
	}
}

func (s *AIService) makeAPIRequest(
	ctx context.Context,
	systemMessage, userMessage Message,
	responseSchema map[string]interface{},
) (*CheckResult, error) {
	modelURI := fmt.Sprintf("gpt://%s/%s/latest", s.config.FolderID, modelYandexGPTLite)

	requestBody := yandexGPTRequest{
		ModelURI: modelURI,
		CompletionOptions: CompletionOptions{
			Stream:      false,
			Temperature: 0.1,
			MaxTokens:   2000,
		},
		Messages: []Message{systemMessage, userMessage},
		JsonSchema: &JsonSchema{
			Schema: responseSchema,
		},
	}

	jsonBody, err := json.Marshal(requestBody)

	log.Printf("REQUEST BODY:\n%s\n", string(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= 3; attempt++ {
		if attempt > 0 {
			log.Println("Retrying API request", "attempt", attempt)
			time.Sleep(time.Duration(attempt*500) * time.Millisecond)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", yandexGPTAPIURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		s.setAuthHeader(req)

		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		log.Printf("RESPONSE STATUS: %d\n", resp.StatusCode)
		log.Printf("FULL RESPONSE BODY:\n%s\n", string(body))

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API error: %s, body: %s", resp.Status, string(body))

			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				break
			}
			continue
		}

		var apiResp yandexGPTResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			lastErr = fmt.Errorf("failed to unmarshal response: %w", err)
			continue
		}

		log.Println(apiResp)

		if len(apiResp.Result.Alternatives) == 0 {
			lastErr = fmt.Errorf("empty response from model")
			continue
		}

		resultText := apiResp.Result.Alternatives[0].Message.Text
		return s.parseModelResponse(resultText)
	}

	return nil, fmt.Errorf("all retries failed, last error: %w", lastErr)
}

func (s *AIService) parseModelResponse(responseText string) (*CheckResult, error) {
	cleanText := strings.TrimSpace(responseText)
	cleanText = strings.TrimPrefix(cleanText, "```json")
	cleanText = strings.TrimPrefix(cleanText, "```")
	cleanText = strings.TrimSuffix(cleanText, "```")
	cleanText = strings.TrimSpace(cleanText)

	var result CheckResult
	if err := json.Unmarshal([]byte(cleanText), &result); err != nil {
		log.Println("Failed to parse model response",
			"response", cleanText, "error", err)

		return &CheckResult{
			Detected:    false,
			Confidence:  0.0,
			Reason:      "Failed to parse AI response",
			ContextType: "unknown",
		}, nil
	}

	return &result, nil
}
