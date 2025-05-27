package gpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// DeepSeekClient implements GPTClient for DeepSeek API
type DeepSeekClient struct {
	APIKey     string
	HTTPClient *http.Client
	Model      string
	BaseURL    string
}

type DeepSeekRequest struct {
	Model       string            `json:"model"`
	Messages    []DeepSeekMessage `json:"messages"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
	Stream      bool              `json:"stream"`
}

type DeepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DeepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// NewDeepSeekClient creates a new DeepSeek client
func NewDeepSeekClient(apiKey, model string) *DeepSeekClient {
	return &DeepSeekClient{
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
		Model:      model,
		BaseURL:    "https://api.deepseek.com/v1",
	}
}

// Complete implements GPTClient interface
func (c *DeepSeekClient) Complete(systemMessage, userMessage string) (string, error) {
	request := DeepSeekRequest{
		Model: c.Model,
		Messages: []DeepSeekMessage{
			{Role: "system", Content: systemMessage},
			{Role: "user", Content: userMessage},
		},
		MaxTokens:   4000,
		Temperature: 0.7,
		Stream:      false,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var response DeepSeekResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("DeepSeek API error: %s", response.Error.Message)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return response.Choices[0].Message.Content, nil
}
