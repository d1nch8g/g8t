package agent

import (
	"context"
	"fmt"
	"os"
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
		history:   NewHistory(20), // Keep last 20 steps
		stepCount: 0,
		startTime: time.Now(),
	}, nil
}

func createGPTClient(cfg *config.Config) (gpt.Client, error) {
	switch cfg.Provider {
	case "openai":
		return gpt.NewOpenAIClient(cfg.OpenAIKey, cfg.OpenAIModel), nil
	case "deepseek":
		return gpt.NewDeepSeekClient(cfg.DeepSeekKey, cfg.DeepSeekModel), nil
	case "claude":
		return gpt.NewClaudeClient(cfg.ClaudeKey, cfg.ClaudeModel), nil
	case "gemini":
		return gpt.NewGeminiClient(cfg.GeminiKey, cfg.GeminiModel), nil
	case "yandex":
		return gpt.NewYandexGPTClient(cfg.FolderID, cfg.IAMToken), nil
	case "ollama":
		return gpt.NewOllamaClient(cfg.OllamaURL, cfg.OllamaModel), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}

func (a *Agent) Run() error {
	a.logger.Info("Starting agent execution",
		"task", a.config.Task,
		"provider", a.config.Provider,
		"max_commands", a.config.MaxCommands,
		"dry_run", a.config.DryRun,
	)

	systemPrompt := a.buildSystemPrompt()

	for a.stepCount < a.config.MaxCommands {
		a.stepCount++

		a.logger.Info("Starting step", "step", a.stepCount)

		// Build user prompt with context
		userPrompt := a.buildUserPrompt()

		// Get response from GPT
		response, err := a.gptClient.Complete(systemPrompt, userPrompt)
		if err != nil {
			a.logger.Error("Failed to get GPT response", "error", err, "step", a.stepCount)
			return fmt.Errorf("GPT request failed at step %d: %w", a.stepCount, err)
		}

		// Parse response
		thought, command, completed, err := a.parseResponse(response)
		if err != nil {
			a.logger.Error("Failed to parse GPT response", "error", err, "response", response)
			return fmt.Errorf("failed to parse response at step %d: %w", a.stepCount, err)
		}

		a.logger.LogThought(a.stepCount, thought)

		// Check if task is completed
		if completed {
			a.logger.Info("Task completed successfully", "step", a.stepCount)
			return nil
		}

		// Execute command
		output, cmdErr := a.executeCommand(command)

		// Create step record
		step := Step{
			Number:    a.stepCount,
			Timestamp: time.Now(),
			Thought:   thought,
			Command:   command,
			Output:    output,
			Error:     "",
			Success:   cmdErr == nil,
		}

		if cmdErr != nil {
			step.Error = cmdErr.Error()
		}

		// Add to history
		a.history.AddStep(step)

		// Log command execution
		a.logger.LogCommand(a.stepCount, command, output, step.Error)

		if !a.config.Quiet {
			fmt.Printf("\n--- Step %d ---\n", a.stepCount)
			fmt.Printf("Thought: %s\n", thought)
			fmt.Printf("Command: %s\n", command)
			if output != "" {
				fmt.Printf("Output: %s\n", output)
			}
			if cmdErr != nil {
				fmt.Printf("Error: %s\n", cmdErr.Error())
			}
		}
	}

	a.logger.Warn("Maximum command limit reached", "max_commands", a.config.MaxCommands)
	return fmt.Errorf("maximum command limit (%d) reached without completing the task", a.config.MaxCommands)
}

func (a *Agent) buildSystemPrompt() string {
	return `You are an AI assistant that helps users complete tasks by executing system commands.

IMPORTANT RULES:
1. Always respond in the following JSON format:
   {
     "thought": "Your reasoning about what to do next",
     "command": "The shell command to execute (or 'TASK_COMPLETED' if done)",
     "completed": false
   }

2. When the task is fully completed, set "completed": true and "command": "TASK_COMPLETED"

3. Be careful with destructive commands. Always verify before making changes.

4. Use relative paths when possible and be mindful of the current working directory.

5. If a command fails, analyze the error and try a different approach.

6. Break complex tasks into smaller, manageable steps.

7. Always provide clear reasoning in the "thought" field.

8. Only execute one command at a time.

Current working directory: ` + getCurrentDir()
}

func (a *Agent) buildUserPrompt() string {
	prompt := fmt.Sprintf("Task: %s\n\n", a.config.Task)

	// Add history context
	if len(a.history.Steps) > 0 {
		prompt += a.history.GetContext() + "\n\n"
	}

	prompt += "What should be the next step? Respond in JSON format with thought, command, and completed fields."

	return prompt
}

func (a *Agent) parseResponse(response string) (thought, command string, completed bool, err error) {
	// Try to extract JSON from response
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}") + 1

	if start == -1 || end == 0 {
		return "", "", false, fmt.Errorf("no JSON found in response: %s", response)
	}

	jsonStr := response[start:end]

	// Simple JSON parsing for the expected format
	var result struct {
		Thought   string `json:"thought"`
		Command   string `json:"command"`
		Completed bool   `json:"completed"`
	}

	if err := parseJSON(jsonStr, &result); err != nil {
		return "", "", false, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return result.Thought, result.Command, result.Completed, nil
}

func (a *Agent) executeCommand(command string) (string, error) {
	if command == "TASK_COMPLETED" {
		return "", nil
	}

	if a.config.DryRun {
		a.logger.Info("DRY RUN - would execute", "command", command)
		return "[DRY RUN] Command would be executed: " + command, nil
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute command
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()

	return string(output), err
}

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return dir
}

// Simple JSON parser for our specific use case
func parseJSON(jsonStr string, result interface{}) error {
	// This is a simplified implementation
	// In a real application, you would use encoding/json
	// For now, we'll use basic string parsing

	// Extract thought
	if start := strings.Index(jsonStr, `"thought"`); start != -1 {
		start = strings.Index(jsonStr[start:], `"`) + start + 1
		start = strings.Index(jsonStr[start:], `"`) + start + 1
		end := strings.Index(jsonStr[start:], `"`) + start
		if r, ok := result.(*struct {
			Thought   string `json:"thought"`
			Command   string `json:"command"`
			Completed bool   `json:"completed"`
		}); ok {
			r.Thought = jsonStr[start:end]
		}
	}

	// Extract command
	if start := strings.Index(jsonStr, `"command"`); start != -1 {
		start = strings.Index(jsonStr[start:], `"`) + start + 1
		start = strings.Index(jsonStr[start:], `"`) + start + 1
		end := strings.Index(jsonStr[start:], `"`) + start
		if r, ok := result.(*struct {
			Thought   string `json:"thought"`
			Command   string `json:"command"`
			Completed bool   `json:"completed"`
		}); ok {
			r.Command = jsonStr[start:end]
		}
	}

	// Extract completed
	if start := strings.Index(jsonStr, `"completed"`); start != -1 {
		if strings.Contains(jsonStr[start:start+50], "true") {
			if r, ok := result.(*struct {
				Thought   string `json:"thought"`
				Command   string `json:"command"`
				Completed bool   `json:"completed"`
			}); ok {
				r.Completed = true
			}
		}
	}

	return nil
}
