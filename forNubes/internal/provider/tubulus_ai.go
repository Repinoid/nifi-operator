package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type AIConfig struct {
	DurationMs     *int64  `json:"duration_ms,omitempty"`
	FailAtStart    *bool   `json:"fail_at_start,omitempty"`
	FailInProgress *bool   `json:"fail_in_progress,omitempty"`
	WhereFail      *int64  `json:"where_fail,omitempty"`
	BodyMessage    *string `json:"body_message,omitempty"`
	ResourceRealm  *string `json:"resource_realm,omitempty"`
	MapExample     *string `json:"map_example,omitempty"`
	JsonExample    *string `json:"json_example,omitempty"`
	YamlExample    *string `json:"yaml_example,omitempty"`
}

func (r *TubulusResource) askGemini(ctx context.Context, instruction string) (*AIConfig, error) {
	// Приоритет: переменная окружения → файл → ошибка
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		// Пробуем прочитать из файла (для локальной разработки)
		keyBytes, err := os.ReadFile("gemini_api_key.txt")
		if err == nil {
			apiKey = strings.TrimSpace(string(keyBytes))
		}
	}
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not found (set env var or create gemini_api_key.txt)")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.0-flash")
	
	// Настройка модели: заставляем её возвращать строго JSON
	model.ResponseMIMEType = "application/json"
	
	systemPrompt := `Ты — эксперт по инфраструктуре и помощник для Terraform провайдера Nubes.
Твоя задача — распарсить пожелания пользователя (даже самые простые и неточные) и превратить их в JSON-конфигурацию для тестового ресурса Tubulus.

=== ПОЛЯ JSON (ВСЕ обязательны!) ===

1. "duration_ms" (integer): Сколько миллисекунд работает ресурс.
   Примеры: "5 секунд" → 5000, "минута" → 60000, "быстро" → 1000, "долго" → 120000, "очень долго" → 300000
   Дефолт если не указано: 5000

2. "fail_at_start" (boolean): Сломаться ли сразу при запуске?
   Примеры: "сломай сразу" → true, "упади в начале" → true
   Дефолт: false

3. "fail_in_progress" (boolean): Сломаться ли в процессе работы?
   Примеры: "упади в середине" → true, "сломай в процессе" → true, "упадет на втором этапе" → true
   Дефолт: false

4. "where_fail" (integer): На каком этапе сломаться? ТОЛЬКО [0, 1, 2, 3]
   0 = не ломается
   1 = подготовка (prepare)
   2 = заполнение данных (data_fill) — ДЕФОЛТ если fail_in_progress=true
   3 = после записи в Vault
   Примеры: "первый этап" → 1, "второй этап" → 2, "последний" → 3
   Дефолт: 0, но если fail_in_progress=true, то 2

5. "body_message" (string): Текст для записи в Vault (как секрет/пароль).
   Примеры: "напиши hello" → "hello", "секрет 123" → "секрет 123", "привет мир" → "привет мир"
   Если НЕ указан текст явно — оставь null (не "ai_generated")
   Дефолт: null

6. "resource_realm" (string): Окружение. Всегда "dummy".
   Дефолт: "dummy"

=== ВАЖНЫЕ ПРАВИЛА ===
• Возвращай СТРОГО JSON с ВСЕМИ 6 полями
• Понимай разговорный язык: "сделай быстро" = duration_ms: 1000, "пусть долго работает" = duration_ms: 120000
• Если пользователь написал текст для Vault ("напиши X", "положи Y") — используй его в body_message, иначе null
• Ответ БЕЗ пояснений, только {"duration_ms": ..., "fail_at_start": ..., ...}

=== ПРИМЕРЫ ===
Запрос: "сделай быстро"
→ {"duration_ms": 1000, "fail_at_start": false, "fail_in_progress": false, "where_fail": 0, "body_message": null, "resource_realm": "dummy"}

Запрос: "пусть работает минуту и напиши привет"
→ {"duration_ms": 60000, "fail_at_start": false, "fail_in_progress": false, "where_fail": 0, "body_message": "привет", "resource_realm": "dummy"}

Запрос: "сломай на втором этапе"
→ {"duration_ms": 5000, "fail_at_start": false, "fail_in_progress": true, "where_fail": 2, "body_message": null, "resource_realm": "dummy"}

Инструкция: ` + instruction

	resp, err := model.GenerateContent(ctx, genai.Text(systemPrompt))
	if err != nil {
		return nil, err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini returned no content")
	}

	part := resp.Candidates[0].Content.Parts[0]
	text, ok := part.(genai.Text)
	if !ok {
		return nil, fmt.Errorf("gemini returned non-text content")
	}

	log.Printf("[DEBUG] Gemini response: %s", string(text))

	// Some Gemini versions return an array with one object when constrained to JSON
	// Let's try to unmarshal as object first, then as array if it fails
	var config AIConfig
	if err := json.Unmarshal([]byte(text), &config); err != nil {
		var configs []AIConfig
		if errArray := json.Unmarshal([]byte(text), &configs); errArray == nil && len(configs) > 0 {
			config = configs[0]
		} else {
			return nil, fmt.Errorf("failed to unmarshal gemini response: %w", err)
		}
	}

	return &config, nil
}
