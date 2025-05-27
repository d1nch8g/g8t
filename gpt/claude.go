package gpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// ClaudeClient implements GPTClient for Anthropic Claude API
type ClaudeClient struct {
	APIKey     string
	HTTPClient *http.Client
	Model      string
	BaseURL    string
}

type ClaudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []ClaudeMessage `json:"messages"`
	System    string          `json:"system,omitempty"`
}

type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeResponse struct {
	Content []struct {
		Text string `json:"text"`
		Type string `json:"type"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// NewClaudeClient creates a new Claude client
func NewClaudeClient(apiKey, model string) *ClaudeClient {
	return &ClaudeClient{
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
		Model:      model,
		BaseURL:    "https://api.anthropic.com/v1",
	}
}

// Complete implements GPTClient interface
func (c *ClaudeClient) Complete(systemMessage, userMessage string) (string, error) {
	request := ClaudeRequest{
		Model:     c.Model,
		MaxTokens: 4000,
		System:    systemMessage,
		Messages: []ClaudeMessage{
			{Role: "user", Content: userMessage},
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var response ClaudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("Claude API error: %s", response.Error.Message)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return response.Content[0].Text, nil
}
