package gpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// GeminiClient implements GPTClient for Google Gemini API
type GeminiClient struct {
	APIKey     string
	HTTPClient *http.Client
	Model      string
	BaseURL    string
}

type GeminiRequest struct {
	Contents          []GeminiContent          `json:"contents"`
	SystemInstruction *GeminiSystemInstruction `json:"systemInstruction,omitempty"`
	GenerationConfig  GeminiGenerationConfig   `json:"generationConfig"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiSystemInstruction struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// NewGeminiClient creates a new Gemini client
func NewGeminiClient(apiKey, model string) *GeminiClient {
	return &GeminiClient{
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
		Model:      model,
		BaseURL:    "https://generativelanguage.googleapis.com/v1beta",
	}
}

// Complete implements GPTClient interface
func (c *GeminiClient) Complete(systemMessage, userMessage string) (string, error) {
	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{{Text: userMessage}},
				Role:  "user",
			},
		},
		GenerationConfig: GeminiGenerationConfig{
			Temperature:     0.7,
			MaxOutputTokens: 4000,
		},
	}

	if systemMessage != "" {
		request.SystemInstruction = &GeminiSystemInstruction{
			Parts: []GeminiPart{{Text: systemMessage}},
		}
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.BaseURL, c.Model, c.APIKey)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var response GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("Gemini API error: %s", response.Error.Message)
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return response.Candidates[0].Content.Parts[0].Text, nil
}
