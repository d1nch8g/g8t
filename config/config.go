package config

import (
	"fmt"

	"github.com/jessevdk/go-flags"
)

type Config struct {
	// Provider settings
	Provider string `short:"p" long:"provider" description:"GPT provider (yandex, openai, deepseek, claude, gemini)" required:"true"`

	// Yandex GPT settings
	FolderID string `long:"folder-id" description:"Yandex Cloud folder ID"`
	IAMToken string `long:"iam-token" description:"Yandex Cloud IAM token"`

	// OpenAI settings
	OpenAIKey   string `long:"openai-key" description:"OpenAI API key"`
	OpenAIModel string `long:"openai-model" description:"OpenAI model" default:"gpt-3.5-turbo"`

	// DeepSeek settings
	DeepSeekKey   string `long:"deepseek-key" description:"DeepSeek API key"`
	DeepSeekModel string `long:"deepseek-model" description:"DeepSeek model" default:"deepseek-chat"`

	// Claude settings
	ClaudeKey   string `long:"claude-key" description:"Claude API key"`
	ClaudeModel string `long:"claude-model" description:"Claude model" default:"claude-3-sonnet-20240229"`

	// Gemini settings
	GeminiKey   string `long:"gemini-key" description:"Gemini API key"`
	GeminiModel string `long:"gemini-model" description:"Gemini model" default:"gemini-pro"`

	// Task settings
	Task        string `short:"T" long:"task" description:"Task description for the agent" required:"true"`
	MaxCommands int    `short:"m" long:"max-commands" description:"Maximum number of commands to execute" default:"20"`

	// Output settings
	Verbose bool   `short:"v" long:"verbose" description:"Enable verbose output"`
	Quiet   bool   `short:"q" long:"quiet" description:"Suppress non-essential output"`
	DryRun  bool   `short:"d" long:"dry-run" description:"Show what would be executed without running commands"`
	LogFile string `short:"l" long:"log-file" description:"Log file path (optional)"`
}

func (c *Config) GetLogLevel() string {
	if c.Quiet {
		return "error"
	}
	if c.Verbose {
		return "debug"
	}
	return "info"
}

func (c *Config) Validate() error {
	switch c.Provider {
	case "yandex":
		if c.FolderID == "" || c.IAMToken == "" {
			return fmt.Errorf("yandex provider requires folder-id and iam-token")
		}
	case "openai":
		if c.OpenAIKey == "" {
			return fmt.Errorf("openai provider requires openai-key")
		}
	case "deepseek":
		if c.DeepSeekKey == "" {
			return fmt.Errorf("deepseek provider requires deepseek-key")
		}
	case "claude":
		if c.ClaudeKey == "" {
			return fmt.Errorf("claude provider requires claude-key")
		}
	case "gemini":
		if c.GeminiKey == "" {
			return fmt.Errorf("gemini provider requires gemini-key")
		}
	default:
		return fmt.Errorf("unsupported provider: %s", c.Provider)
	}

	if c.MaxCommands <= 0 {
		return fmt.Errorf("max-commands must be greater than 0")
	}

	return nil
}

func Parse() (*Config, error) {
	var config Config

	parser := flags.NewParser(&config, flags.Default)
	parser.Usage = "[OPTIONS]"

	// Set custom application name and description
	parser.Name = "g8t"
	parser.ShortDescription = "AI Agent using various GPT providers"
	parser.LongDescription = `AI Agent that executes system commands to complete tasks using various GPT providers.

The agent will iteratively call the GPT API to determine what commands to run
and execute them until the task is completed or the maximum command limit is reached.

Supported Providers:
  yandex   - Yandex GPT (requires folder-id and iam-token)
  openai   - OpenAI GPT models (requires openai-key)
  deepseek - DeepSeek models (requires deepseek-key)
  claude   - Anthropic Claude models (requires claude-key)
  gemini   - Google Gemini models (requires gemini-key)

Examples:
  # Using Yandex GPT
  g8t -p yandex -f b1g... -t t1.9euel... -T "Create a test directory"
  
  # Using OpenAI
  g8t -p openai --openai-key=sk-... -T "Find all .go files" --verbose
  
  # Using Claude
  g8t -p claude --claude-key=sk-ant-... -T "Setup a web server" --dry-run
  
  # Using DeepSeek
  g8t -p deepseek --deepseek-key=sk-... -T "Analyze code structure"
  
  # Using Gemini
  g8t -p gemini --gemini-key=AIza... -T "Create documentation"
`

	_, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok {
			if flagsErr.Type == flags.ErrHelp {
				return nil, err
			}
		}
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}
