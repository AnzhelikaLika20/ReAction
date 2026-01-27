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
	if cfg.AIConfig.APIKey == "" || cfg.AIConfig.FolderID == "" {
		return nil, fmt.Errorf("Yandex GPT API configuration is missing")
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
	if contextType == "" {
		contextType = "promise" // default
	}

	prompt, exists := s.promptTemplates[contextType]
	if !exists {
		return nil, fmt.Errorf("unknown context type: %s", contextType)
	}

	systemMessage := Message{
		Role: "system",
		Text: prompt,
	}

	userMessage := Message{
		Role: "user",
		Text: message,
	}

	return s.makeAPIRequest(ctx, systemMessage, userMessage)
}

func (s *AIService) CheckMessageWithCustomContext(
	ctx context.Context,
	message string,
	contextCheck *ContextCheck,
) (*CheckResult, error) {
	customPrompt := fmt.Sprintf(`Ты анализируешь сообщения на наличие контекста: %s.
Параметры анализа: %v.`,
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

	return s.makeAPIRequest(ctx, systemMessage, userMessage)
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

	// Проверяем, были ли ошибки
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

func CreatePromiseDetectionSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"detected": map[string]interface{}{
				"type":        "boolean",
				"description": "Было ли обнаружено обещание",
			},
		},
		"required": []string{"detected"},
	}
}

func (s *AIService) makeAPIRequest(
	ctx context.Context,
	systemMessage, userMessage Message,
) (*CheckResult, error) {
	modelURI := fmt.Sprintf("gpt://%s/%s/latest", s.config.FolderID, modelYandexGPTLite)

	requestBody := yandexGPTRequest{
		ModelURI: modelURI,
		CompletionOptions: CompletionOptions{
			Stream:      false,
			Temperature: 0.1,
			MaxTokens:   1000,
		},
		Messages: []Message{systemMessage, userMessage},
		JsonSchema: &JsonSchema{
			Schema: CreatePromiseDetectionSchema(),
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
			time.Sleep(time.Duration(attempt*500) * time.Millisecond) // Exponential backoff
		}

		req, err := http.NewRequestWithContext(ctx, "POST", yandexGPTAPIURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.YandexIamToken))

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
			body, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("API error: %s, body: %s", resp.Status, string(body))

			// Не повторяем при клиентских ошибках 4xx
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

		// Парсинг JSON ответа от модели
		resultText := apiResp.Result.Alternatives[0].Message.Text
		return s.parseModelResponse(resultText)
	}

	return nil, fmt.Errorf("all retries failed, last error: %w", lastErr)
}

func (s *AIService) parseModelResponse(responseText string) (*CheckResult, error) {
	// Очистка ответа от возможных markdown или лишних символов
	cleanText := strings.TrimSpace(responseText)
	cleanText = strings.TrimPrefix(cleanText, "```json")
	cleanText = strings.TrimPrefix(cleanText, "```")
	cleanText = strings.TrimSuffix(cleanText, "```")
	cleanText = strings.TrimSpace(cleanText)

	var result CheckResult
	if err := json.Unmarshal([]byte(cleanText), &result); err != nil {
		log.Println("Failed to parse model response",
			"response", cleanText, "error", err)

		// Fallback: эвристический анализ
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
