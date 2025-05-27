package gpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// OllamaClient implements Client for Ollama API
type OllamaClient struct {
	BaseURL    string
	HTTPClient *http.Client
	Model      string
}

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL, model string) *OllamaClient {
	return &OllamaClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{},
		Model:      model,
	}
}

// Complete implements Client interface
func (c *OllamaClient) Complete(systemMessage, userMessage string) (string, error) {
	// Combine system and user messages for Ollama
	prompt := fmt.Sprintf("System: %s\n\nUser: %s\n\nAssistant:", systemMessage, userMessage)

	request := OllamaRequest{
		Model:  c.Model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var response OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error != "" {
		return "", fmt.Errorf("API error: %s", response.Error)
	}

	return response.Response, nil
}
