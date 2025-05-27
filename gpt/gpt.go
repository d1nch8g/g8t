package gpt

// Client interface for all GPT providers
type Client interface {
	Complete(systemMessage, userMessage string) (string, error)
}
