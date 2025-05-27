package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/d1nch8g/g8t/config"
	"github.com/d1nch8g/g8t/gpt"
	"github.com/jessevdk/go-flags"
)

type CommandLog struct {
	Timestamp time.Time `json:"timestamp"`
	Command   string    `json:"command"`
	Output    string    `json:"output"`
	Error     string    `json:"error,omitempty"`
	Duration  string    `json:"duration"`
}

type AgentResponse struct {
	Done     bool   `json:"done"`
	Command  string `json:"command,omitempty"`
	Thought  string `json:"thought,omitempty"`
	Plan     string `json:"plan,omitempty"`
	Progress string `json:"progress,omitempty"`
}

type Agent struct {
	client       gpt.GPTClient
	config       *config.Config
	commandLog   []CommandLog
	logFile      *os.File
	longTermPlan string
	workingDir   string
}

const MaxRememberedCommands = 15

func NewAgent(client gpt.GPTClient, cfg *config.Config) (*Agent, error) {
	agent := &Agent{
		client:     client,
		config:     cfg,
		commandLog: make([]CommandLog, 0),
	}

	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}
	agent.workingDir = wd

	// Setup log file if specified
	if cfg.LogFile != "" {
		file, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		agent.logFile = file
	}

	return agent, nil
}

func (a *Agent) Close() {
	if a.logFile != nil {
		a.logFile.Close()
	}
}

func (a *Agent) log(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)

	if !a.config.Quiet {
		fmt.Print(message)
	}

	if a.logFile != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		a.logFile.WriteString(fmt.Sprintf("[%s] %s", timestamp, message))
	}
}

func (a *Agent) logf(format string, args ...interface{}) {
	a.log(format+"\n", args...)
}

// validateCommand checks for problematic commands and suggests alternatives
func (a *Agent) validateCommand(command string) error {
	// Prevent cd commands
	if strings.HasPrefix(strings.TrimSpace(command), "cd ") {
		return fmt.Errorf("cd commands don't work in this environment - use full paths instead")
	}

	// Suggest mkdir -p instead of mkdir
	if strings.HasPrefix(strings.TrimSpace(command), "mkdir ") && !strings.Contains(command, "-p") {
		return fmt.Errorf("use 'mkdir -p' to avoid errors if directory exists")
	}

	// Prevent problematic echo commands with \n
	if strings.Contains(command, "echo '") && strings.Contains(command, "\\n") {
		return fmt.Errorf("use 'cat > file << EOF' instead of echo with \\n escapes for multi-line files")
	}

	return nil
}

// hasCommandBeenTried checks if a command has been tried recently
func (a *Agent) hasCommandBeenTried(command string) bool {
	// Check last 3 commands to prevent immediate repetition
	start := len(a.commandLog) - 3
	if start < 0 {
		start = 0
	}

	for i := start; i < len(a.commandLog); i++ {
		if a.commandLog[i].Command == command {
			return true
		}
	}
	return false
}

func (a *Agent) executeCommand(command string) (string, error) {
	start := time.Now()

	// Validate command first
	if err := a.validateCommand(command); err != nil {
		return "", err
	}

	// Check if command was tried recently
	if a.hasCommandBeenTried(command) {
		return "", fmt.Errorf("command was already tried recently - avoid repetition")
	}

	// Security: Basic command validation
	dangerousCommands := []string{
		"rm -rf /", "sudo rm", "mkfs", "dd if=", ":(){ :|:& };:",
		"chmod -R 777 /", "chown -R", "> /dev/", "curl", "wget",
		"sudo", "su -", "passwd", "useradd", "userdel",
	}

	// Check if command is dangerous
	for _, dangerous := range dangerousCommands {
		if strings.Contains(command, dangerous) {
			// Check if it's in allowed commands
			if !a.config.IsCommandAllowed(strings.Fields(command)[0]) {
				return "", fmt.Errorf("dangerous command blocked: %s", dangerous)
			}
		}
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	if a.config.Verbose {
		a.logf("🔧 Executing: %s", command)
	}

	// Dry run mode
	if a.config.DryRun {
		a.logf("🔍 [DRY RUN] Would execute: %s", command)
		return fmt.Sprintf("[DRY RUN] Command: %s", command), nil
	}

	// Use shell execution for better compatibility
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("bash", "-c", command)
	}

	// Set working directory for command execution
	cmd.Dir = a.workingDir

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	// Log the command execution
	logEntry := CommandLog{
		Timestamp: start,
		Command:   command,
		Output:    string(output),
		Duration:  duration.String(),
	}

	if err != nil {
		logEntry.Error = err.Error()
	}

	a.commandLog = append(a.commandLog, logEntry)

	// Keep only the last MaxRememberedCommands to limit context size
	if len(a.commandLog) > MaxRememberedCommands {
		a.commandLog = a.commandLog[len(a.commandLog)-MaxRememberedCommands:]
	}

	return string(output), err
}

func (a *Agent) getRepositoryContext() string {
	context := ""

	// Check if we're in a git repository
	if _, err := os.Stat(".git"); err == nil {
		context += "📁 Working in a Git repository\n"

		// Get git status if available
		cmd := exec.Command("git", "status", "--porcelain")
		cmd.Dir = a.workingDir
		if output, err := cmd.Output(); err == nil && len(output) > 0 {
			context += fmt.Sprintf("Git status: %s\n", strings.TrimSpace(string(output)))
		}

		// Get current branch
		cmd = exec.Command("git", "branch", "--show-current")
		cmd.Dir = a.workingDir
		if output, err := cmd.Output(); err == nil {
			context += fmt.Sprintf("Current branch: %s\n", strings.TrimSpace(string(output)))
		}
	}

	// Check for common project files
	projectFiles := []string{"go.mod", "package.json", "Cargo.toml", "requirements.txt", "Makefile", "README.md"}
	foundFiles := []string{}

	for _, file := range projectFiles {
		if _, err := os.Stat(file); err == nil {
			foundFiles = append(foundFiles, file)
		}
	}

	if len(foundFiles) > 0 {
		context += fmt.Sprintf("📋 Project files found: %s\n", strings.Join(foundFiles, ", "))
	}

	return context
}

func (a *Agent) buildSystemMessage() string {
	repoContext := a.getRepositoryContext()

	systemMsg := fmt.Sprintf(`You are an AI agent that executes terminal commands to complete high-level tasks.

TASK: %s

WORKING DIRECTORY: %s
%s

CRITICAL RULES:
1. NEVER use 'cd' commands - they don't work. Use full paths or relative paths from working directory
2. Use 'mkdir -p dirname' instead of 'mkdir dirname' to avoid errors
3. For multi-line files, use this syntax:
   cat > filename << 'EOF'
   file content here
   EOF
4. DON'T repeat commands that already failed
5. Include ALL required imports in code files
6. Check your work with 'ls' and 'cat filename' before marking done
7. When working with existing repositories, respect the existing structure and conventions

PLANNING AND MEMORY:
- This is a HIGH-LEVEL task that may require multiple steps
- Create a plan and break it down into smaller actionable steps
- Remember your progress and update your plan as needed
- Use the "plan" field to store your long-term strategy
- Use the "progress" field to track what you've accomplished

RESPONSE FORMAT:
You must respond ONLY in JSON format with these fields:
{
  "done": false,
  "command": "exact_command_to_execute",
  "thought": "brief_explanation_of_current_step",
  "plan": "your_overall_strategy_and_remaining_steps",
  "progress": "what_youve_accomplished_so_far"
}

OR when task is complete:
{
  "done": true,
  "thought": "task_completion_summary",
  "progress": "final_summary_of_what_was_accomplished"
}

GOOD command examples:
- ls -la (explore current directory)
- find . -name "*.go" -type f (find specific files)
- mkdir -p new/directory/structure
- cat > path/to/file.go << 'EOF'
- git status (check repository state)
- go mod tidy (manage dependencies)

BAD command examples:
- cd directory (doesn't work)
- mkdir directory (use mkdir -p)
- echo 'line1\nline2' > file (escapes don't work)

Remember: You have a limit of %d remembered commands, so plan efficiently!`,
		a.config.Task, a.workingDir, repoContext, MaxRememberedCommands)

	return systemMsg
}

func (a *Agent) buildUserMessage() string {
	if len(a.commandLog) == 0 {
		return "No commands executed yet. Start by exploring the current environment and creating your plan."
	}

	message := ""

	// Include long-term plan if available
	if a.longTermPlan != "" {
		message += fmt.Sprintf("CURRENT PLAN:\n%s\n\n", a.longTermPlan)
	}

	// Show recent command history (limited to save tokens)
	message += "RECENT COMMAND HISTORY:\n"
	start := 0
	if len(a.commandLog) > 5 {
		start = len(a.commandLog) - 5
	}

	for i := start; i < len(a.commandLog); i++ {
		log := a.commandLog[i]
		status := "✅"
		if log.Error != "" {
			status = "❌"
		}

		// Truncate long outputs to save tokens
		output := strings.TrimSpace(log.Output)
		if len(output) > 200 {
			output = output[:200] + "... [truncated]"
		}

		message += fmt.Sprintf("%s %s\n", status, log.Command)
		if output != "" {
			message += fmt.Sprintf("   Output: %s\n", output)
		}
		if log.Error != "" {
			message += fmt.Sprintf("   Error: %s\n", log.Error)
		}
	}

	// Show last command details
	lastLog := a.commandLog[len(a.commandLog)-1]
	message += fmt.Sprintf("\nLAST COMMAND RESULT:\n")
	message += fmt.Sprintf("Command: %s\n", lastLog.Command)

	if lastLog.Output != "" {
		output := strings.TrimSpace(lastLog.Output)
		if len(output) > 500 {
			output = output[:500] + "... [truncated]"
		}
		message += fmt.Sprintf("Output: %s\n", output)
	}

	if lastLog.Error != "" {
		message += fmt.Sprintf("Error: %s\n", lastLog.Error)
	}

	message += fmt.Sprintf("\nWorking Directory: %s", a.workingDir)
	message += fmt.Sprintf("\nCommands executed: %d/%d remembered", len(a.commandLog), MaxRememberedCommands)

	return message
}

func (a *Agent) parseResponse(response string) (*AgentResponse, error) {
	var agentResp AgentResponse

	// Clean up response - remove markdown code blocks
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```") {
		lines := strings.Split(response, "\n")
		var jsonLines []string
		inJson := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				if inJson {
					break
				}
				inJson = true
				continue
			}
			if inJson {
				jsonLines = append(jsonLines, line)
			}
		}
		response = strings.Join(jsonLines, "\n")
	}

	// Try to extract JSON from response
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")

	if start == -1 || end == -1 || start >= end {
		return nil, fmt.Errorf("no valid JSON found in response: %s", response)
	}

	jsonStr := response[start : end+1]
	err := json.Unmarshal([]byte(jsonStr), &agentResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v, response: %s", err, jsonStr)
	}

	return &agentResp, nil
}

func (a *Agent) Run() error {
	a.logf("🚀 Starting agent with task: %s", a.config.Task)
	a.logf("📁 Working directory: %s", a.workingDir)
	a.logf("📝 Maximum commands allowed: %d", a.config.MaxCommands)
	a.logf("🧠 Command memory limit: %d", MaxRememberedCommands)
	if a.config.DryRun {
		a.logf("🔍 Running in DRY RUN mode - no commands will be executed")
	}
	a.logf("")

	for i := 0; i < a.config.MaxCommands; i++ {
		a.logf("--- Iteration %d/%d ---", i+1, a.config.MaxCommands)

		systemMsg := a.buildSystemMessage()
		userMsg := a.buildUserMessage()

		if a.config.Verbose {
			a.logf("📤 Sending request to Yandex GPT...")
		}

		response, err := a.client.Complete(systemMsg, userMsg)
		if err != nil {
			return fmt.Errorf("GPT request failed: %v", err)
		}

		if a.config.Verbose {
			a.logf("📥 GPT Response: %s", response)
		}

		agentResp, err := a.parseResponse(response)
		if err != nil {
			a.logf("❌ Failed to parse response: %v", err)
			continue
		}

		// Update long-term plan if provided
		if agentResp.Plan != "" {
			a.longTermPlan = agentResp.Plan
			if a.config.Verbose {
				a.logf("📋 Plan updated: %s", agentResp.Plan)
			}
		}

		if agentResp.Progress != "" {
			a.logf("📊 Progress: %s", agentResp.Progress)
		}

		if agentResp.Thought != "" {
			a.logf("💭 Agent thought: %s", agentResp.Thought)
		}

		if agentResp.Done {
			a.logf("✅ Task completed successfully!")
			a.logf("📊 Total commands executed: %d", len(a.commandLog))
			if agentResp.Progress != "" {
				a.logf("🎯 Final result: %s", agentResp.Progress)
			}
			return nil
		}

		if agentResp.Command == "" {
			a.logf("⚠️  No command provided, continuing...")
			continue
		}

		output, err := a.executeCommand(agentResp.Command)

		if err != nil {
			a.logf("❌ Command failed: %v", err)
		} else {
			a.logf("✅ Command executed successfully")
		}

		if a.config.Verbose && output != "" {
			a.logf("📄 Output: %s", strings.TrimSpace(output))
		}

		a.logf("")
	}

	return fmt.Errorf("reached maximum number of commands (%d) without completion", a.config.MaxCommands)
}

func main() {
	cfg, err := config.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok {
			if flagsErr.Type == flags.ErrHelp {
				os.Exit(0)
			}
		}
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Print configuration if verbose
	if cfg.Verbose {
		cfg.PrintConfig()
	}

	// Create Yandex GPT client
	client := gpt.NewYandexGPTClient(cfg.FolderID, cfg.IAMToken)

	// Create agent
	agent, err := NewAgent(client, cfg)
	if err != nil {
		log.Fatalf("❌ Failed to create agent: %v", err)
	}
	defer agent.Close()

	// Run agent
	if err := agent.Run(); err != nil {
		log.Fatalf("❌ Agent failed: %v", err)
	}
}
