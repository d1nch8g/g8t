package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
)

// Config holds all configuration options for the agent
type Config struct {
	// Yandex GPT Configuration
	FolderID string `short:"f" long:"folder-id" env:"YANDEX_FOLDER_ID" description:"Yandex Cloud Folder ID" required:"true"`
	IAMToken string `short:"t" long:"iam-token" env:"YANDEX_IAM_TOKEN" description:"Yandex Cloud IAM Token" required:"true"`

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
	parser.ShortDescription = "AI Agent using Yandex GPT"
	parser.LongDescription = `AI Agent that executes system commands to complete tasks using Yandex GPT.

The agent will iteratively call the GPT API to determine what commands to run
and execute them until the task is completed or the maximum command limit is reached.

Examples:
  g8t-agent -f b1g... -t t1.9euel... -T "Create a test directory and list contents"
  g8t-agent --folder-id=b1g... --iam-token=t1.9euel... --task="Find all .go files" --verbose
  g8t-agent -T "Setup a simple web server" --dry-run --max-commands=10

Environment Variables:
  YANDEX_FOLDER_ID    Yandex Cloud Folder ID
  YANDEX_IAM_TOKEN    Yandex Cloud IAM Token`

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
	if c.FolderID == "" {
		return fmt.Errorf("folder-id is required")
	}

	if c.IAMToken == "" {
		return fmt.Errorf("iam-token is required")
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
	fmt.Printf("  Folder ID: %s\n", c.FolderID)
	fmt.Printf("  IAM Token: %s***\n", c.IAMToken[:min(8, len(c.IAMToken))])
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
