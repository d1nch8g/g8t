package gpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// OpenAIClient implements GPTClient for OpenAI API
type OpenAIClient struct {
	APIKey     string
	HTTPClient *http.Client
	Model      string
	BaseURL    string
}

type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
		Model:      model,
		BaseURL:    "https://api.openai.com/v1",
	}
}

// Complete implements GPTClient interface
func (c *OpenAIClient) Complete(systemMessage, userMessage string) (string, error) {
	request := OpenAIRequest{
		Model: c.Model,
		Messages: []OpenAIMessage{
			{Role: "system", Content: systemMessage},
			{Role: "user", Content: userMessage},
		},
		MaxTokens:   4000,
		Temperature: 0.7,
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

	var response OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", response.Error.Message)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return response.Choices[0].Message.Content, nil
}
