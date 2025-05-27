package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-flags"
)

// Config holds all configuration options for the agent
type Config struct {
	// GPT Provider Configuration
	Provider string `short:"p" long:"provider" env:"GPT_PROVIDER" description:"GPT provider (yandex, openai, deepseek, claude, gemini)" default:"yandex"`

	// Yandex GPT Configuration
	FolderID string `short:"f" long:"folder-id" env:"YANDEX_FOLDER_ID" description:"Yandex Cloud Folder ID"`
	IAMToken string `short:"t" long:"iam-token" env:"YANDEX_IAM_TOKEN" description:"Yandex Cloud IAM Token"`

	// OpenAI Configuration
	OpenAIKey   string `long:"openai-key" env:"OPENAI_API_KEY" description:"OpenAI API Key"`
	OpenAIModel string `long:"openai-model" env:"OPENAI_MODEL" description:"OpenAI Model" default:"gpt-4"`

	// DeepSeek Configuration
	DeepSeekKey   string `long:"deepseek-key" env:"DEEPSEEK_API_KEY" description:"DeepSeek API Key"`
	DeepSeekModel string `long:"deepseek-model" env:"DEEPSEEK_MODEL" description:"DeepSeek Model" default:"deepseek-chat"`

	// Claude Configuration
	ClaudeKey   string `long:"claude-key" env:"CLAUDE_API_KEY" description:"Claude API Key"`
	ClaudeModel string `long:"claude-model" env:"CLAUDE_MODEL" description:"Claude Model" default:"claude-3-sonnet-20240229"`

	// Gemini Configuration
	GeminiKey   string `long:"gemini-key" env:"GEMINI_API_KEY" description:"Gemini API Key"`
	GeminiModel string `long:"gemini-model" env:"GEMINI_MODEL" description:"Gemini Model" default:"gemini-pro"`

	// Agent Configuration
	Task        string `short:"T" long:"task" description:"Task description for the agent" required:"true"`
	MaxCommands int    `short:"m" long:"max-commands" description:"Maximum number of commands to execute" default:"20"`
	Verbose     bool   `short:"v" long:"verbose" description:"Enable verbose output"`

	// Safety Configuration
	DryRun      bool     `short:"d" long:"dry-run" description:"Show commands without executing them"`
	AllowedCmds []string `short:"a" long:"allow-cmd" description:"Additional allowed commands (can be specified multiple times)"`

	// Output Configuration
	LogFile string `short:"l" long:"log-file" description:"Log file path (optional)"`
	Quiet   bool   `short:"q" long:"quiet" description:"Suppress non-essential output"`
}

// Parse parses command line arguments and environment variables
func Parse() (*Config, error) {
	var config Config

	parser := flags.NewParser(&config, flags.Default)
	parser.Usage = "[OPTIONS]"

	// Set custom application name and description
	parser.Name = "g8t-agent"
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
  g8t-agent -p yandex -f b1g... -t t1.9euel... -T "Create a test directory"
  
  # Using OpenAI
  g8t-agent -p openai --openai-key=sk-... -T "Find all .go files" --verbose
  
  # Using Claude
  g8t-agent -p claude --claude-key=sk-ant-... -T "Setup a web server" --dry-run
  
  # Using DeepSeek
  g8t-agent -p deepseek --deepseek-key=sk-... -T "Analyze code structure"
  
  # Using Gemini
  g8t-agent -p gemini --gemini-key=AIza... -T "Create documentation"

Environment Variables:
  GPT_PROVIDER        GPT provider to use
  YANDEX_FOLDER_ID    Yandex Cloud Folder ID
  YANDEX_IAM_TOKEN    Yandex Cloud IAM Token
  OPENAI_API_KEY      OpenAI API Key
  OPENAI_MODEL        OpenAI Model (default: gpt-4)
  DEEPSEEK_API_KEY    DeepSeek API Key
  DEEPSEEK_MODEL      DeepSeek Model (default: deepseek-chat)
  CLAUDE_API_KEY      Claude API Key
  CLAUDE_MODEL        Claude Model (default: claude-3-sonnet-20240229)
  GEMINI_API_KEY      Gemini API Key
  GEMINI_MODEL        Gemini Model (default: gemini-pro)`

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

// Validate performs additional validation on the configuration
func (c *Config) Validate() error {
	// Normalize provider name
	c.Provider = strings.ToLower(c.Provider)

	// Validate provider
	validProviders := []string{"yandex", "openai", "deepseek", "claude", "gemini"}
	isValidProvider := false
	for _, provider := range validProviders {
		if c.Provider == provider {
			isValidProvider = true
			break
		}
	}
	if !isValidProvider {
		return fmt.Errorf("invalid provider '%s'. Valid providers: %s", c.Provider, strings.Join(validProviders, ", "))
	}

	// Validate provider-specific configuration
	switch c.Provider {
	case "yandex":
		if c.FolderID == "" {
			return fmt.Errorf("folder-id is required for Yandex provider")
		}
		if c.IAMToken == "" {
			return fmt.Errorf("iam-token is required for Yandex provider")
		}
	case "openai":
		if c.OpenAIKey == "" {
			return fmt.Errorf("openai-key is required for OpenAI provider")
		}
	case "deepseek":
		if c.DeepSeekKey == "" {
			return fmt.Errorf("deepseek-key is required for DeepSeek provider")
		}
	case "claude":
		if c.ClaudeKey == "" {
			return fmt.Errorf("claude-key is required for Claude provider")
		}
	case "gemini":
		if c.GeminiKey == "" {
			return fmt.Errorf("gemini-key is required for Gemini provider")
		}
	}

	if c.Task == "" {
		return fmt.Errorf("task description is required")
	}

	if c.MaxCommands <= 0 {
		return fmt.Errorf("max-commands must be greater than 0")
	}

	if c.MaxCommands > 100 {
		return fmt.Errorf("max-commands cannot exceed 100 for safety reasons")
	}

	if c.Verbose && c.Quiet {
		return fmt.Errorf("verbose and quiet options are mutually exclusive")
	}

	// Validate log file path if provided
	if c.LogFile != "" {
		dir := filepath.Dir(c.LogFile)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("log file directory does not exist: %s", dir)
		}
	}

	return nil
}

// GetLogLevel returns the appropriate log level based on configuration
func (c *Config) GetLogLevel() string {
	if c.Quiet {
		return "error"
	}
	if c.Verbose {
		return "debug"
	}
	return "info"
}

// IsCommandAllowed checks if a command is in the allowed list
func (c *Config) IsCommandAllowed(cmd string) bool {
	for _, allowed := range c.AllowedCmds {
		if allowed == cmd {
			return true
		}
	}
	return false
}

// PrintConfig prints the current configuration (excluding sensitive data)
func (c *Config) PrintConfig() {
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Provider: %s\n", c.Provider)

	switch c.Provider {
	case "yandex":
		fmt.Printf("  Folder ID: %s\n", c.FolderID)
		fmt.Printf("  IAM Token: %s***\n", c.IAMToken[:min(8, len(c.IAMToken))])
	case "openai":
		fmt.Printf("  OpenAI Key: %s***\n", c.OpenAIKey[:min(8, len(c.OpenAIKey))])
		fmt.Printf("  OpenAI Model: %s\n", c.OpenAIModel)
	case "deepseek":
		fmt.Printf("  DeepSeek Key: %s***\n", c.DeepSeekKey[:min(8, len(c.DeepSeekKey))])
		fmt.Printf("  DeepSeek Model: %s\n", c.DeepSeekModel)
	case "claude":
		fmt.Printf("  Claude Key: %s***\n", c.ClaudeKey[:min(8, len(c.ClaudeKey))])
		fmt.Printf("  Claude Model: %s\n", c.ClaudeModel)
	case "gemini":
		fmt.Printf("  Gemini Key: %s***\n", c.GeminiKey[:min(8, len(c.GeminiKey))])
		fmt.Printf("  Gemini Model: %s\n", c.GeminiModel)
	}

	fmt.Printf("  Task: %s\n", c.Task)
	fmt.Printf("  Max Commands: %d\n", c.MaxCommands)
	fmt.Printf("  Verbose: %v\n", c.Verbose)
	fmt.Printf("  Dry Run: %v\n", c.DryRun)
	fmt.Printf("  Quiet: %v\n", c.Quiet)
	if len(c.AllowedCmds) > 0 {
		fmt.Printf("  Allowed Commands: %v\n", c.AllowedCmds)
	}
	if c.LogFile != "" {
		fmt.Printf("  Log File: %s\n", c.LogFile)
	}
	fmt.Println()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
