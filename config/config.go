package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Provider settings
	Provider string `yaml:"provider"`

	// Yandex GPT settings
	FolderID string `yaml:"folder_id"`
	IAMToken string `yaml:"iam_token"`

	// OpenAI settings
	OpenAIKey   string `yaml:"openai_key"`
	OpenAIModel string `yaml:"openai_model"`

	// DeepSeek settings
	DeepSeekKey   string `yaml:"deepseek_key"`
	DeepSeekModel string `yaml:"deepseek_model"`

	// Claude settings
	ClaudeKey   string `yaml:"claude_key"`
	ClaudeModel string `yaml:"claude_model"`

	// Gemini settings
	GeminiKey   string `yaml:"gemini_key"`
	GeminiModel string `yaml:"gemini_model"`

	// Ollama settings
	OllamaURL   string `yaml:"ollama_url"`
	OllamaModel string `yaml:"ollama_model"`

	// Task settings (not saved to config, passed as args)
	Task        string `yaml:"-"`
	MaxCommands int    `yaml:"max_commands"`

	// Output settings
	Verbose bool   `yaml:"verbose"`
	Quiet   bool   `yaml:"quiet"`
	DryRun  bool   `yaml:"dry_run"`
	LogFile string `yaml:"log_file"`
}

func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".g8t.yml"), nil
}

func (c *Config) Save() error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	err = os.WriteFile(configPath, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func loadConfig() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Config doesn't exist
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func promptString(prompt, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" && defaultValue != "" {
		return defaultValue
	}
	return input
}

func promptBool(prompt string, defaultValue bool) bool {
	defaultStr := "n"
	if defaultValue {
		defaultStr = "y"
	}

	response := promptString(fmt.Sprintf("%s (y/n)", prompt), defaultStr)
	return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
}

func promptInt(prompt string, defaultValue int) int {
	response := promptString(prompt, strconv.Itoa(defaultValue))
	if val, err := strconv.Atoi(response); err == nil {
		return val
	}
	return defaultValue
}

func newConfigWithDefaults() *Config {
	return &Config{
		// Provider defaults
		Provider: "openai",

		// Yandex defaults
		FolderID: "your-folder-id",
		IAMToken: "your-iam-token",

		// OpenAI defaults
		OpenAIKey:   "your-openai-key",
		OpenAIModel: "gpt-3.5-turbo",

		// DeepSeek defaults
		DeepSeekKey:   "your-deepseek-key",
		DeepSeekModel: "deepseek-chat",

		// Claude defaults
		ClaudeKey:   "your-claude-key",
		ClaudeModel: "claude-3-sonnet-20240229",

		// Gemini defaults
		GeminiKey:   "your-gemini-key",
		GeminiModel: "gemini-pro",

		// Ollama defaults
		OllamaURL:   "http://localhost:11434",
		OllamaModel: "llama2",

		// General defaults
		MaxCommands: 20,
		Verbose:     false,
		Quiet:       false,
		DryRun:      false,
		LogFile:     "",
	}
}

func setupConfig() {
	fmt.Println("Welcome to g8t! Let's set up your configuration.")
	fmt.Println("Supported providers: yandex, openai, deepseek, claude, gemini, ollama")

	config := newConfigWithDefaults()

	// Provider selection
	config.Provider = promptString("Select GPT provider", config.Provider)

	// Configure selected provider
	switch config.Provider {
	case "yandex":
		config.FolderID = promptString("Yandex Cloud Folder ID", config.FolderID)
		config.IAMToken = promptString("Yandex Cloud IAM Token", config.IAMToken)
	case "openai":
		config.OpenAIKey = promptString("OpenAI API Key", config.OpenAIKey)
		config.OpenAIModel = promptString("OpenAI Model", config.OpenAIModel)
	case "deepseek":
		config.DeepSeekKey = promptString("DeepSeek API Key", config.DeepSeekKey)
		config.DeepSeekModel = promptString("DeepSeek Model", config.DeepSeekModel)
	case "claude":
		config.ClaudeKey = promptString("Claude API Key", config.ClaudeKey)
		config.ClaudeModel = promptString("Claude Model", config.ClaudeModel)
	case "gemini":
		config.GeminiKey = promptString("Gemini API Key", config.GeminiKey)
		config.GeminiModel = promptString("Gemini Model", config.GeminiModel)
	case "ollama":
		config.OllamaURL = promptString("Ollama API URL", config.OllamaURL)
		config.OllamaModel = promptString("Ollama Model", config.OllamaModel)
	}

	// General settings
	config.MaxCommands = promptInt("Maximum number of commands to execute", config.MaxCommands)
	config.Verbose = promptBool("Enable verbose output", config.Verbose)
	config.Quiet = promptBool("Enable quiet mode", config.Quiet)
	config.DryRun = promptBool("Enable dry-run mode by default", config.DryRun)
	config.LogFile = promptString("Log file path (optional)", config.LogFile)

	// Save configuration
	err := config.Save()
	if err != nil {
		fmt.Printf("Failed to save configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Configuration saved to ~/.g8t.yml")
	fmt.Println("You can edit ~/.g8t.yml to switch providers or update settings anytime.")
	os.Exit(0)
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
		if c.FolderID == "your-folder-id" || c.IAMToken == "your-iam-token" {
			return fmt.Errorf("yandex provider requires valid folder-id and iam-token")
		}
	case "openai":
		if c.OpenAIKey == "your-openai-key" {
			return fmt.Errorf("openai provider requires valid openai-key")
		}
	case "deepseek":
		if c.DeepSeekKey == "your-deepseek-key" {
			return fmt.Errorf("deepseek provider requires valid deepseek-key")
		}
	case "claude":
		if c.ClaudeKey == "your-claude-key" {
			return fmt.Errorf("claude provider requires valid claude-key")
		}
	case "gemini":
		if c.GeminiKey == "your-gemini-key" {
			return fmt.Errorf("gemini provider requires valid gemini-key")
		}
	case "ollama":
		if c.OllamaURL == "" || c.OllamaModel == "your-ollama-model" {
			return fmt.Errorf("ollama provider requires valid ollama-url and ollama-model")
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
	// Try to load existing config
	config, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// If no config exists, run setup
	if config == nil {
		setupConfig()
	}

	// Parse command line arguments for task
	args := os.Args[1:]
	if len(args) == 0 {
		return nil, fmt.Errorf("task description is required as command line argument")
	}

	// Join all arguments as the task description
	config.Task = strings.Join(args, " ")

	// Handle special flags that might override config
	var newArgs []string
	for i, arg := range args {
		switch arg {
		case "--help", "-h":
			fmt.Println(`Usage: g8t <task>

Description:
	g8t is a command-line tool that helps you execute tasks using AI assistants.
	It supports multiple AI providers and allows configuration of various execution parameters.

Options:
	--help, -h           Show this help message
	--verbose, -v        Enable verbose output
	--quiet, -q          Suppress non-essential output
	--dry-run, -d        Show commands without executing them
	--setup              Reconfigure tool settings
	--max-commands, -m   Maximum number of commands to execute
	--provider, -p       Specify AI provider (openai, claude, gemini, yandex, ollama)`)
			os.Exit(0)
		case "--verbose", "-v":
			config.Verbose = true
		case "--quiet", "-q":
			config.Quiet = true
		case "--dry-run", "-d":
			config.DryRun = true
		case "--setup":
			setupConfig()
		case "--max-commands", "-m":
			if i+1 < len(args) {
				if val, err := strconv.Atoi(args[i+1]); err == nil {
					config.MaxCommands = val
				}
			}
		case "--provider", "-p":
			if i+1 < len(args) {
				config.Provider = args[i+1]
			}
		default:
			if arg != "--provider" && arg != "-p" {
				newArgs = append(newArgs, arg)
			}
		}
	}

	// Update task with filtered arguments
	if len(newArgs) > 0 {
		config.Task = strings.Join(newArgs, " ")
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}
