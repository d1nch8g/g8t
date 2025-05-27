package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/d1nch8g/g8t/config"
	"github.com/d1nch8g/g8t/gpt"
	"github.com/d1nch8g/g8t/logger"
)

type Agent struct {
	config    *Config
	logger    *logger.Logger
	gptClient gpt.Client
	history   *History
	stepCount int
	startTime time.Time
}

type Config struct {
	*config.Config
}

type Step struct {
	Number    int       `json:"number"`
	Timestamp time.Time `json:"timestamp"`
	Thought   string    `json:"thought"`
	Command   string    `json:"command"`
	Output    string    `json:"output"`
	Error     string    `json:"error"`
	Success   bool      `json:"success"`
}

type History struct {
	Steps    []Step `json:"steps"`
	MaxSteps int    `json:"max_steps"`
}

func NewHistory(maxSteps int) *History {
	return &History{
		Steps:    make([]Step, 0, maxSteps),
		MaxSteps: maxSteps,
	}
}

func (h *History) AddStep(step Step) {
	h.Steps = append(h.Steps, step)
	// Keep only the last MaxSteps
	if len(h.Steps) > h.MaxSteps {
		h.Steps = h.Steps[1:]
	}
}

func (h *History) GetContext() string {
	if len(h.Steps) == 0 {
		return "No previous commands executed."
	}

	var context strings.Builder
	context.WriteString("Previous command history:\n")

	for _, step := range h.Steps {
		context.WriteString(fmt.Sprintf("\nStep %d:\n", step.Number))
		context.WriteString(fmt.Sprintf("Thought: %s\n", step.Thought))
		context.WriteString(fmt.Sprintf("Command: %s\n", step.Command))
		if step.Output != "" {
			context.WriteString(fmt.Sprintf("Output: %s\n", step.Output))
		}
		if step.Error != "" {
			context.WriteString(fmt.Sprintf("Error: %s\n", step.Error))
		}
		context.WriteString(fmt.Sprintf("Success: %t\n", step.Success))
	}

	return context.String()
}

func New(cfg *config.Config, log *logger.Logger) (*Agent, error) {
	// Create GPT client based on provider
	gptClient, err := createGPTClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create GPT client: %w", err)
	}

	return &Agent{
		config:    &Config{cfg},
		logger:    log,
		gptClient: gptClient,
		history:   NewHistory(10),
		stepCount: 0,
		startTime: time.Now(),
	}, nil
}

func createGPTClient(cfg *config.Config) (gpt.Client, error) {
	switch cfg.Provider {
	case "openai":
		return gpt.NewOpenAIClient(cfg.OpenAIKey, cfg.OpenAIModel), nil
	case "gemini":
		return gpt.NewGeminiClient(cfg.GeminiKey, cfg.GeminiModel), nil
	case "claude":
		return gpt.NewClaudeClient(cfg.ClaudeKey, cfg.ClaudeModel), nil
	case "deepseek":
		return gpt.NewDeepSeekClient(cfg.DeepSeekKey, cfg.DeepSeekModel), nil
	case "ollama":
		return gpt.NewOllamaClient(cfg.OllamaURL, cfg.OllamaModel), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}

func (a *Agent) Run(task string) error {
	a.logger.Info("Starting agent execution",
		"task", fmt.Sprintf("%s %s", a.config.Provider, task),
		"provider", a.config.Provider,
		"max_commands", a.config.MaxCommands,
		"dry_run", a.config.DryRun,
	)

	systemMessage := `You are an AI assistant that helps execute tasks by running shell commands.

Your response must be a valid JSON object with this exact structure:
{
  "thought": "your reasoning about what to do next",
  "command": "the shell command to execute"
}

Rules:
1. Always respond with valid JSON only
2. Use the "thought" field to explain your reasoning
3. Use the "command" field for the exact shell command to run
4. If the task is complete, use "command": "TASK_COMPLETE"
5. Be careful with destructive operations
6. Consider the current directory and file structure`

	for a.stepCount < a.config.MaxCommands {
		a.stepCount++
		a.logger.Info("Starting step", "step", a.stepCount)

		// Build user message with context
		userMessage := fmt.Sprintf("Task: %s\n\n%s\n\nWhat should I do next?", task, a.history.GetContext())

		// Get response from GPT
		response, err := a.gptClient.Complete(systemMessage, userMessage)
		if err != nil {
			return fmt.Errorf("failed to get GPT response: %w", err)
		}

		// Parse the response
		thought, command, err := a.parseResponse(response)
		if err != nil {
			a.logger.Error("Failed to parse response", "error", err, "response", response)
			continue
		}

		// Check if task is complete
		if command == "TASK_COMPLETE" {
			a.logger.Info("Task completed", "thought", thought)
			return nil
		}

		// Execute the command
		a.executeCommand(thought, command)
	}

	return fmt.Errorf("reached maximum number of commands (%d)", a.config.MaxCommands)
}

func (a *Agent) parseResponse(response string) (string, string, error) {
	// Try to extract JSON from the response
	jsonStr := a.extractJSON(response)
	if jsonStr == "" {
		return "", "", fmt.Errorf("no JSON found in response")
	}

	return a.parseJSON(jsonStr, a.logger)
}

func (a *Agent) extractJSON(response string) string {
	// Look for JSON object boundaries
	start := strings.Index(response, "{")
	if start == -1 {
		return ""
	}

	// Find the matching closing brace
	braceCount := 0
	for i := start; i < len(response); i++ {
		if response[i] == '{' {
			braceCount++
		} else if response[i] == '}' {
			braceCount--
			if braceCount == 0 {
				return response[start : i+1]
			}
		}
	}

	return ""
}

func (a *Agent) parseJSON(jsonStr string, logger *logger.Logger) (string, string, error) {
	var parsed struct {
		Thought string `json:"thought"`
		Command string `json:"command"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		// Log the problematic JSON for debugging, but safely truncate it
		truncatedJSON := jsonStr
		if len(jsonStr) > 255 {
			truncatedJSON = jsonStr[:255] + "..."
		}
		logger.Error("Failed to parse JSON", "error", err, "json", truncatedJSON)
		return "", "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	if parsed.Thought == "" || parsed.Command == "" {
		return "", "", fmt.Errorf("missing required fields in JSON response")
	}

	return parsed.Thought, parsed.Command, nil
}

func (a *Agent) executeCommand(thought, command string) {
	step := Step{
		Number:    a.stepCount,
		Timestamp: time.Now(),
		Thought:   thought,
		Command:   command,
	}

	a.logger.Info("Executing command", "thought", thought, "command", command)

	if a.config.DryRun {
		a.logger.Info("Dry run mode - command not executed")
		step.Output = "DRY RUN - command not executed"
		step.Success = true
		a.history.AddStep(step)
		return
	}

	// Execute the command
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	output, err := cmd.CombinedOutput()

	step.Output = string(output)
	if err != nil {
		step.Error = err.Error()
		step.Success = false
		a.logger.Error("Command failed", "error", err, "output", string(output))
	} else {
		step.Success = true
		a.logger.Info("Command succeeded", "output", string(output))
	}

	a.history.AddStep(step)
}
