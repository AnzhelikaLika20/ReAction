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
	"sync"
	"time"

	"ReAction/internal/config"
)

const (
	yandexGPTAPIURL    = "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"
	modelYandexGPT     = "yandexgpt"
	modelYandexGPTLite = "yandexgpt-lite"
)

type AIService struct {
	config          config.AIConfig
	httpClient      *http.Client
	promptTemplates map[string]string
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
		config:          cfg.AIConfig,
		httpClient:      httpClient,
		promptTemplates: make(map[string]string),
	}

	service.initPromptTemplates()
	return service, nil
}

func (s *AIService) setAuthHeader(req *http.Request) {
	if s.config.APIKey != "" {
		req.Header.Set("Authorization", "Api-Key "+s.config.APIKey)
		return
	}
	req.Header.Set("Authorization", "Bearer "+s.config.YandexIamToken)
}

func (s *AIService) initPromptTemplates() {
	s.promptTemplates["promise"] = `Ты анализируешь сообщения на наличие обещаний или обязательств. 
Определи, содержит ли сообщение обещание что-то сделать. Обрати внимание на слова:
"обещаю", "гарантирую", "обязательно", "сделаю", "выполню", "постараюсь", "пообещал", "договорились".
`

	s.promptTemplates["deadline"] = `Ты анализируешь сообщения на наличие сроков, дедлайнов или временных обещаний.
Найди любые упоминания времени: "завтра", "к пятнице", "через 2 дня", "до 18:00", "к концу недели".
`

	s.promptTemplates["intent"] = `Ты анализируешь сообщения на наличие намерений или планов.
Определи, выражает ли сообщение намерение что-то сделать: "хочу", "планирую", "собираюсь", "мечтаю", "надо бы".
`
}

func (s *AIService) CheckMessage(
	ctx context.Context,
	message string,
	contextType string,
) (*CheckResult, error) {
	return s.CheckMessageWithHistory(ctx, []string{message}, contextType)
}

func (s *AIService) CheckMessageWithHistory(
	ctx context.Context,
	history []string,
	contextType string,
) (*CheckResult, error) {
	return s.CheckMessageWithHistoryAndScenarios(ctx, history, contextType, nil, nil)
}

func (s *AIService) CheckMessageWithHistoryAndScenarios(
	ctx context.Context,
	history []string,
	contextType string,
	scenarios []UserScenarioForAI,
	existingReminders []ExistingReminderForAI,
) (*CheckResult, error) {
	return s.checkWithHistoryAndScenarios(ctx, history, contextType, scenarios, existingReminders)
}

func (s *AIService) checkWithHistoryAndScenarios(
	ctx context.Context,
	history []string,
	contextType string,
	scenarios []UserScenarioForAI,
	existingReminders []ExistingReminderForAI,
) (*CheckResult, error) {
	if len(history) == 0 {
		return nil, fmt.Errorf("empty message history")
	}
	if contextType == "" {
		contextType = "promise"
	}

	prompt, exists := s.promptTemplates[contextType]
	if !exists {
		return nil, fmt.Errorf("unknown context type: %s", contextType)
	}

	if len(history) > 1 {
		prompt += `

Тебе передаётся фрагмент переписки: сначала более старые сообщения, в конце — то, что нужно оценить.
Учитывай контекст предыдущих реплик; итоговая оценка (detected, scenario_id, reminder) относится только к последнему сообщению.`
	}

	prompt += `

Текущий момент для интерпретации «завтра», «в пятницу» и т.п. (RFC3339): ` + time.Now().Format(time.RFC3339)

	if len(scenarios) > 0 {
		raw, err := json.Marshal(scenarios)
		if err != nil {
			return nil, fmt.Errorf("marshal scenarios: %w", err)
		}
		prompt += `

Сценарии пользователя (JSON; id — UUID сценария, title — название, trigger_phrase — ключевая фраза/триггер):
` + string(raw) + `

Если последнее сообщение по смыслу однозначно подходит под один из сценариев (учитывай trigger_phrase и title): detected=true, scenario_id = поле id этого сценария (строка), заполни reminder осмысленными значениями.
Если ни один сценарий не подходит или выбор неоднозначен: detected=false, scenario_id="" и все поля reminder — пустые строки "" (схема ответа требует эти ключи всегда).`
	} else {
		prompt += `

Список сценариев в этом запросе пуст: всегда scenario_id="".`
	}

	if len(existingReminders) > 0 {
		raw, err := json.Marshal(existingReminders)
		if err != nil {
			return nil, fmt.Errorf("marshal existing reminders: %w", err)
		}
		prompt += `

Напоминания, уже поставленные по этому чату за последние 30 минут (JSON; title — название, datetime — время события):
` + string(raw) + `

Если последнее сообщение относится к той же договорённости, что уже есть в списке выше (совпадает смысл и/или время) — это дубликат: detected=false.`
	}

	prompt += `

Если detected=true, заполни объект reminder: title — короткое название встречи или задачи; description — краткое описание (1–2 предложения); datetime — начало в ISO 8601 с часовым поясом; end_datetime — окончание в том же формате (если в тексте нет — задай разумную длительность, например +1 час от начала).
Если detected=false — scenario_id="" и reminder: title="", description="", datetime="", end_datetime="".`

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
	b.WriteString("Фрагмент чата (хронологически, сверху — раньше, снизу — новее):\n\n")
	for i := 0; i < len(messages)-1; i++ {
		fmt.Fprintf(&b, "[ранее] %s\n", messages[i])
	}
	fmt.Fprintf(&b, "\n[последнее сообщение — только его оцени] %s", messages[len(messages)-1])
	return b.String()
}

func (s *AIService) CheckMessageWithCustomContext(
	ctx context.Context,
	message string,
	contextCheck *ContextCheck,
) (*CheckResult, error) {
	customPrompt := fmt.Sprintf(`Ты анализируешь сообщения на наличие контекста: %s.
Параметры анализа: %v.

Поле scenario_id всегда присутствует в JSON: используй пустую строку "".`,
		contextCheck.Description,
		contextCheck.Parameters)

	systemMessage := Message{
		Role: "system",
		Text: customPrompt,
	}

	userMessage := Message{
		Role: "user",
		Text: message,
	}

	return s.makeAPIRequest(ctx, systemMessage, userMessage, classificationResultSchema())
}

func (s *AIService) BatchCheckMessages(
	ctx context.Context,
	messages []string,
	contextType string,
) ([]*CheckResult, error) {
	var wg sync.WaitGroup
	results := make([]*CheckResult, len(messages))
	errors := make([]error, len(messages))

	semaphore := make(chan struct{}, 5)

	for i, msg := range messages {
		wg.Add(1)
		go func(idx int, message string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result, err := s.CheckMessage(ctx, message, contextType)
			if err != nil {
				errors[idx] = err
				log.Println("Error checking message",
					"index", idx, "error", err, "message_preview", s.truncateMessage(message))
			} else {
				results[idx] = result
			}
		}(i, msg)
	}

	wg.Wait()

	for _, err := range errors {
		if err != nil {
			return nil, fmt.Errorf("batch check completed with errors")
		}
	}

	return results, nil
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
				"description": "Есть ли в целевом сообщении искомый признак (обещание / срок / намерение — по задаче system).",
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

func (s *AIService) truncateMessage(msg string) string {
	if len(msg) > 100 {
		return msg[:100] + "..."
	}
	return msg
}
