package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

// AIResponseGenerator generates AI responses using Gemini
 type AIResponseGenerator struct {
	APIKey    string
	ModelName string
}

// NewAIResponseGenerator creates a new AIResponseGenerator with env config
func NewAIResponseGenerator() *AIResponseGenerator {
	return &AIResponseGenerator{
		APIKey:    os.Getenv("GEMINI_API_KEY"),
		ModelName: "gemini-1.5-flash", // default model name
	}
}

// GenerateAIResponse calls the Gemini API and returns the response
func (a *AIResponseGenerator) GenerateAIResponse(threadContext string) (string, error) {
	if a.APIKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY not set")
	}

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", a.ModelName, a.APIKey)

	payload := map[string]interface{}{
		"contents": []map[string]string{{"role": "user", "parts": threadContext}},
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(endpoint, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	log.Printf("Gemini API response: %s", string(body))

	// Parse the response (simplified, real response may differ)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if candidates, ok := result["candidates"].([]interface{}); ok && len(candidates) > 0 {
		if cand, ok := candidates[0].(map[string]interface{}); ok {
			if content, ok := cand["content"].(map[string]interface{}); ok {
				if parts, ok := content["parts"].([]interface{}); ok && len(parts) > 0 {
					if text, ok := parts[0].(string); ok {
						return text, nil
					}
				}
			}
		}
	}
	return "[No AI response]", nil
} 