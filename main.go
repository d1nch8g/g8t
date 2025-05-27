package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/d1nch8g/g8t/config"
	"github.com/d1nch8g/g8t/gpt"
	"github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
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

type ProjectContext struct {
	ProjectType   string            `json:"project_type"`
	Languages     []string          `json:"languages"`
	Frameworks    []string          `json:"frameworks"`
	BuildTools    []string          `json:"build_tools"`
	Dependencies  map[string]string `json:"dependencies"`
	Structure     []string          `json:"structure"`
	GitInfo       GitContext        `json:"git_info"`
	LastAnalyzed  time.Time         `json:"last_analyzed"`
	KeyFiles      []string          `json:"key_files"`
	Documentation []string          `json:"documentation"`
}

type GitContext struct {
	IsRepo        bool     `json:"is_repo"`
	CurrentBranch string   `json:"current_branch"`
	Status        string   `json:"status"`
	RemoteURL     string   `json:"remote_url"`
	LastCommit    string   `json:"last_commit"`
	ModifiedFiles []string `json:"modified_files"`
}

type AgentMemory struct {
	SessionStartTime time.Time      `json:"session_start_time"`
	TaskObjective    string         `json:"task_objective"`
	CurrentPlan      string         `json:"current_plan"`
	CompletedSteps   []string       `json:"completed_steps"`
	FailedAttempts   []string       `json:"failed_attempts"`
	KeyFindings      []string       `json:"key_findings"`
	ProjectContext   ProjectContext `json:"project_context"`
	WorkingStrategy  string         `json:"working_strategy"`
	NextSteps        []string       `json:"next_steps"`
	Assumptions      []string       `json:"assumptions"`
}

type Agent struct {
	client       gpt.GPTClient
	config       *config.Config
	commandLog   []CommandLog
	logFile      *os.File
	longTermPlan string
	workingDir   string
	logger       *logrus.Logger
	memory       *AgentMemory
}

const MaxRememberedCommands = 15

func NewAgent(client gpt.GPTClient, cfg *config.Config) (*Agent, error) {
	// Initialize structured logger
	logger := logrus.New()

	// Set log level based on config
	if cfg.Verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else if cfg.Quiet {
		logger.SetLevel(logrus.ErrorLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	// Set log format
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		logger.WithError(err).Error("Failed to get working directory")
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Initialize agent memory
	memory := &AgentMemory{
		SessionStartTime: time.Now(),
		TaskObjective:    cfg.Task,
		CompletedSteps:   make([]string, 0),
		FailedAttempts:   make([]string, 0),
		KeyFindings:      make([]string, 0),
		NextSteps:        make([]string, 0),
		Assumptions:      make([]string, 0),
		ProjectContext: ProjectContext{
			Languages:     make([]string, 0),
			Frameworks:    make([]string, 0),
			BuildTools:    make([]string, 0),
			Dependencies:  make(map[string]string),
			Structure:     make([]string, 0),
			KeyFiles:      make([]string, 0),
			Documentation: make([]string, 0),
			LastAnalyzed:  time.Now(),
		},
	}

	agent := &Agent{
		client:     client,
		config:     cfg,
		commandLog: make([]CommandLog, 0),
		logger:     logger,
		workingDir: wd,
		memory:     memory,
	}

	logger.WithFields(logrus.Fields{
		"provider":     cfg.Provider,
		"max_commands": cfg.MaxCommands,
		"dry_run":      cfg.DryRun,
		"working_dir":  wd,
	}).Info("Initializing agent with enhanced memory")

	// Setup log file if specified
	if cfg.LogFile != "" {
		file, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logger.WithError(err).WithField("log_file", cfg.LogFile).Error("Failed to open log file")
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		agent.logFile = file
		logger.WithField("log_file", cfg.LogFile).Info("Log file opened successfully")
	}

	// Perform initial project analysis
	agent.analyzeProjectContext()

	logger.Info("Agent initialized successfully with project context")
	return agent, nil
}

func (a *Agent) Close() {
	if a.logFile != nil {
		a.logger.Debug("Closing log file")
		a.logFile.Close()
	}
	a.logger.Info("Agent closed")
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

// analyzeProjectContext performs deep analysis of the current project
func (a *Agent) analyzeProjectContext() {
	a.logger.Debug("Analyzing project context")

	// Analyze Git repository
	a.analyzeGitContext()

	// Detect project type and structure
	a.detectProjectType()

	// Analyze dependencies
	a.analyzeDependencies()

	// Map project structure
	a.mapProjectStructure()

	// Find documentation
	a.findDocumentation()

	a.memory.ProjectContext.LastAnalyzed = time.Now()
	a.logger.WithFields(logrus.Fields{
		"project_type": a.memory.ProjectContext.ProjectType,
		"languages":    a.memory.ProjectContext.Languages,
		"frameworks":   a.memory.ProjectContext.Frameworks,
		"is_git_repo":  a.memory.ProjectContext.GitInfo.IsRepo,
	}).Info("Project context analysis completed")
}

func (a *Agent) analyzeGitContext() {
	gitInfo := &a.memory.ProjectContext.GitInfo

	// Check if we're in a git repository
	if _, err := os.Stat(".git"); err == nil {
		gitInfo.IsRepo = true

		// Get current branch
		cmd := exec.Command("git", "branch", "--show-current")
		cmd.Dir = a.workingDir
		if output, err := cmd.Output(); err == nil {
			gitInfo.CurrentBranch = strings.TrimSpace(string(output))
		}

		// Get git status
		cmd = exec.Command("git", "status", "--porcelain")
		cmd.Dir = a.workingDir
		if output, err := cmd.Output(); err == nil {
			gitInfo.Status = strings.TrimSpace(string(output))
			// Parse modified files
			lines := strings.Split(gitInfo.Status, "\n")
			for _, line := range lines {
				if len(line) > 3 {
					gitInfo.ModifiedFiles = append(gitInfo.ModifiedFiles, strings.TrimSpace(line[3:]))
				}
			}
		}

		// Get remote URL
		cmd = exec.Command("git", "remote", "get-url", "origin")
		cmd.Dir = a.workingDir
		if output, err := cmd.Output(); err == nil {
			gitInfo.RemoteURL = strings.TrimSpace(string(output))
		}

		// Get last commit
		cmd = exec.Command("git", "log", "-1", "--oneline")
		cmd.Dir = a.workingDir
		if output, err := cmd.Output(); err == nil {
			gitInfo.LastCommit = strings.TrimSpace(string(output))
		}
	}
}

func (a *Agent) detectProjectType() {
	ctx := &a.memory.ProjectContext

	// Check for various project indicators
	projectIndicators := map[string]string{
		"go.mod":             "Go",
		"package.json":       "Node.js/JavaScript",
		"Cargo.toml":         "Rust",
		"requirements.txt":   "Python",
		"pyproject.toml":     "Python",
		"pom.xml":            "Java/Maven",
		"build.gradle":       "Java/Gradle",
		"Gemfile":            "Ruby",
		"composer.json":      "PHP",
		"Dockerfile":         "Docker",
		"docker-compose.yml": "Docker Compose",
	}

	for file, lang := range projectIndicators {
		if _, err := os.Stat(file); err == nil {
			if !contains(ctx.Languages, lang) {
				ctx.Languages = append(ctx.Languages, lang)
			}
			ctx.KeyFiles = append(ctx.KeyFiles, file)
		}
	}

	// Detect frameworks and build tools
	frameworkIndicators := map[string]string{
		"next.config.js":    "Next.js",
		"nuxt.config.js":    "Nuxt.js",
		"angular.json":      "Angular",
		"vue.config.js":     "Vue.js",
		"svelte.config.js":  "Svelte",
		"webpack.config.js": "Webpack",
		"vite.config.js":    "Vite",
		"Makefile":          "Make",
		"CMakeLists.txt":    "CMake",
	}

	for file, framework := range frameworkIndicators {
		if _, err := os.Stat(file); err == nil {
			if strings.Contains(framework, "Make") || strings.Contains(framework, "CMake") {
				ctx.BuildTools = append(ctx.BuildTools, framework)
			} else {
				ctx.Frameworks = append(ctx.Frameworks, framework)
			}
			ctx.KeyFiles = append(ctx.KeyFiles, file)
		}
	}

	// Determine primary project type
	if len(ctx.Languages) > 0 {
		ctx.ProjectType = ctx.Languages[0]
	} else {
		ctx.ProjectType = "Unknown"
	}
}

func (a *Agent) analyzeDependencies() {
	ctx := &a.memory.ProjectContext

	// Analyze package.json
	if data, err := os.ReadFile("package.json"); err == nil {
		var pkg map[string]interface{}
		if json.Unmarshal(data, &pkg) == nil {
			if deps, ok := pkg["dependencies"].(map[string]interface{}); ok {
				for name, version := range deps {
					if v, ok := version.(string); ok {
						ctx.Dependencies[name] = v
					}
				}
			}
		}
	}

	// Analyze go.mod
	if data, err := os.ReadFile("go.mod"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, " v") && !strings.HasPrefix(line, "module") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					ctx.Dependencies[parts[0]] = parts[1]
				}
			}
		}
	}
}

func (a *Agent) mapProjectStructure() {
	ctx := &a.memory.ProjectContext

	// Get directory structure (limited depth to avoid overwhelming)
	cmd := exec.Command("find", ".", "-type", "d", "-maxdepth", "3")
	cmd.Dir = a.workingDir
	if output, err := cmd.Output(); err == nil {
		dirs := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, dir := range dirs {
			if dir != "." && !strings.Contains(dir, ".git") && !strings.Contains(dir, "node_modules") {
				ctx.Structure = append(ctx.Structure, dir)
			}
		}
	}
}

func (a *Agent) findDocumentation() {
	ctx := &a.memory.ProjectContext

	docFiles := []string{"README.md", "README.txt", "CHANGELOG.md", "CONTRIBUTING.md", "docs/", "documentation/"}

	for _, file := range docFiles {
		if _, err := os.Stat(file); err == nil {
			ctx.Documentation = append(ctx.Documentation, file)
		}
	}
}

func (a *Agent) updateMemoryFromResponse(resp *AgentResponse) {
	if resp.Plan != "" && resp.Plan != a.memory.CurrentPlan {
		a.memory.CurrentPlan = resp.Plan
		a.logger.WithField("plan", resp.Plan).Debug("Updated current plan in memory")
	}

	if resp.Progress != "" {
		// Add to completed steps if it's a meaningful progress update
		if !contains(a.memory.CompletedSteps, resp.Progress) {
			a.memory.CompletedSteps = append(a.memory.CompletedSteps, resp.Progress)
			a.logger.WithField("progress", resp.Progress).Debug("Added progress to completed steps")
		}
	}

	if resp.Thought != "" {
		// Extract key findings from thoughts
		if strings.Contains(strings.ToLower(resp.Thought), "found") ||
			strings.Contains(strings.ToLower(resp.Thought), "discovered") ||
			strings.Contains(strings.ToLower(resp.Thought), "identified") {
			if !contains(a.memory.KeyFindings, resp.Thought) {
				a.memory.KeyFindings = append(a.memory.KeyFindings, resp.Thought)
				a.logger.WithField("finding", resp.Thought).Debug("Added key finding to memory")
			}
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// validateCommand checks for problematic commands and suggests alternatives
func (a *Agent) validateCommand(command string) error {
	a.logger.WithField("command", command).Debug("Validating command")

	// Prevent cd commands
	if strings.HasPrefix(strings.TrimSpace(command), "cd ") {
		a.logger.WithField("command", command).Warn("Blocked cd command")
		return fmt.Errorf("cd commands don't work in this environment - use full paths instead")
	}

	// Suggest mkdir -p instead of mkdir
	if strings.HasPrefix(strings.TrimSpace(command), "mkdir ") && !strings.Contains(command, "-p") {
		a.logger.WithField("command", command).Warn("Suggested mkdir -p instead of mkdir")
		return fmt.Errorf("use 'mkdir -p' to avoid errors if directory exists")
	}

	// Prevent problematic echo commands with \n
	if strings.Contains(command, "echo '") && strings.Contains(command, "\\n") {
		a.logger.WithField("command", command).Warn("Blocked problematic echo command")
		return fmt.Errorf("use 'cat > file << EOF' instead of echo with \\n escapes for multi-line files")
	}

	a.logger.WithField("command", command).Debug("Command validation passed")
	return nil
}

// hasCommandBeenTried checks if a command has been tried recently
func (a *Agent) hasCommandBeenTried(command string) bool {
	// Check last 5 commands to prevent immediate repetition
	start := len(a.commandLog) - 5
	if start < 0 {
		start = 0
	}

	for i := start; i < len(a.commandLog); i++ {
		if a.commandLog[i].Command == command {
			a.logger.WithFields(logrus.Fields{
				"command":      command,
				"found_at_idx": i,
			}).Debug("Command was tried recently")
			return true
		}
	}
	return false
}

func (a *Agent) executeCommand(command string) (string, error) {
	start := time.Now()

	a.logger.WithFields(logrus.Fields{
		"command":     command,
		"working_dir": a.workingDir,
		"dry_run":     a.config.DryRun,
	}).Info("Starting command execution")

	// Validate command first
	if err := a.validateCommand(command); err != nil {
		a.logger.WithError(err).WithField("command", command).Error("Command validation failed")
		// Add to failed attempts
		a.memory.FailedAttempts = append(a.memory.FailedAttempts, fmt.Sprintf("%s: %s", command, err.Error()))
		return "", err
	}

	// Check if command was tried recently
	if a.hasCommandBeenTried(command) {
		a.logger.WithField("command", command).Warn("Command repetition detected")
		failureMsg := "command was already tried recently - avoid repetition"
		a.memory.FailedAttempts = append(a.memory.FailedAttempts, fmt.Sprintf("%s: %s", command, failureMsg))
		return "", fmt.Errorf(failureMsg)
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
				a.logger.WithFields(logrus.Fields{
					"command":           command,
					"dangerous_pattern": dangerous,
				}).Error("Dangerous command blocked")
				failureMsg := fmt.Sprintf("dangerous command blocked: %s", dangerous)
				a.memory.FailedAttempts = append(a.memory.FailedAttempts, fmt.Sprintf("%s: %s", command, failureMsg))
				return "", fmt.Errorf(failureMsg)
			}
		}
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		a.logger.Error("Empty command provided")
		return "", fmt.Errorf("empty command")
	}

	if a.config.Verbose {
		a.logf("🔧 Executing: %s", command)
	}

	// Dry run mode
	if a.config.DryRun {
		a.logf("🔍 [DRY RUN] Would execute: %s", command)
		a.logger.WithField("command", command).Info("Dry run command simulation")
		return fmt.Sprintf("[DRY RUN] Command: %s", command), nil
	}

	// Use shell execution for better compatibility
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
		a.logger.WithField("shell", "cmd").Debug("Using Windows command shell")
	} else {
		cmd = exec.Command("bash", "-c", command)
		a.logger.WithField("shell", "bash").Debug("Using bash shell")
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

	logFields := logrus.Fields{
		"command":     command,
		"duration":    duration.String(),
		"output_size": len(output),
		"success":     err == nil,
	}

	if err != nil {
		logEntry.Error = err.Error()
		logFields["error"] = err.Error()
		a.logger.WithFields(logFields).Error("Command execution failed")
		// Add to failed attempts
		a.memory.FailedAttempts = append(a.memory.FailedAttempts, fmt.Sprintf("%s: %s", command, err.Error()))
	} else {
		a.logger.WithFields(logFields).Info("Command executed successfully")
	}

	a.commandLog = append(a.commandLog, logEntry)

	// Keep only the last MaxRememberedCommands to limit context size
	if len(a.commandLog) > MaxRememberedCommands {
		oldLen := len(a.commandLog)
		a.commandLog = a.commandLog[len(a.commandLog)-MaxRememberedCommands:]
		a.logger.WithFields(logrus.Fields{
			"old_count": oldLen,
			"new_count": len(a.commandLog),
			"max_limit": MaxRememberedCommands,
		}).Debug("Trimmed command log to stay within memory limit")
	}

	return string(output), err
}
func (a *Agent) getRepositoryContext() string {
	a.logger.Debug("Gathering repository context")
	context := ""

	gitInfo := a.memory.ProjectContext.GitInfo
	if gitInfo.IsRepo {
		context += "📁 Working in a Git repository\n"
		if gitInfo.CurrentBranch != "" {
			context += fmt.Sprintf("🌿 Current branch: %s\n", gitInfo.CurrentBranch)
		}
		if gitInfo.Status != "" {
			context += fmt.Sprintf("📊 Git status: %s\n", gitInfo.Status)
		}
		if len(gitInfo.ModifiedFiles) > 0 {
			context += fmt.Sprintf("📝 Modified files: %s\n", strings.Join(gitInfo.ModifiedFiles, ", "))
		}
		if gitInfo.LastCommit != "" {
			context += fmt.Sprintf("💾 Last commit: %s\n", gitInfo.LastCommit)
		}
	}

	projectCtx := a.memory.ProjectContext
	if projectCtx.ProjectType != "Unknown" {
		context += fmt.Sprintf("🏗️ Project type: %s\n", projectCtx.ProjectType)
	}

	if len(projectCtx.Languages) > 0 {
		context += fmt.Sprintf("💻 Languages: %s\n", strings.Join(projectCtx.Languages, ", "))
	}

	if len(projectCtx.Frameworks) > 0 {
		context += fmt.Sprintf("🔧 Frameworks: %s\n", strings.Join(projectCtx.Frameworks, ", "))
	}

	if len(projectCtx.BuildTools) > 0 {
		context += fmt.Sprintf("🛠️ Build tools: %s\n", strings.Join(projectCtx.BuildTools, ", "))
	}

	if len(projectCtx.KeyFiles) > 0 {
		context += fmt.Sprintf("📋 Key files: %s\n", strings.Join(projectCtx.KeyFiles, ", "))
	}

	if len(projectCtx.Documentation) > 0 {
		context += fmt.Sprintf("📚 Documentation: %s\n", strings.Join(projectCtx.Documentation, ", "))
	}

	a.logger.WithField("context_length", len(context)).Debug("Repository context gathered")
	return context
}

func (a *Agent) buildSystemMessage() string {
	a.logger.Debug("Building enhanced system message")
	repoContext := a.getRepositoryContext()

	// Build memory context
	memoryContext := ""
	if a.memory.CurrentPlan != "" {
		memoryContext += fmt.Sprintf("CURRENT PLAN:\n%s\n\n", a.memory.CurrentPlan)
	}

	if len(a.memory.CompletedSteps) > 0 {
		memoryContext += "COMPLETED STEPS:\n"
		for _, step := range a.memory.CompletedSteps {
			memoryContext += fmt.Sprintf("✅ %s\n", step)
		}
		memoryContext += "\n"
	}

	if len(a.memory.KeyFindings) > 0 {
		memoryContext += "KEY FINDINGS:\n"
		for _, finding := range a.memory.KeyFindings {
			memoryContext += fmt.Sprintf("🔍 %s\n", finding)
		}
		memoryContext += "\n"
	}

	if len(a.memory.FailedAttempts) > 0 {
		memoryContext += "PREVIOUS FAILURES (AVOID REPEATING):\n"
		// Show only last 3 failures to avoid overwhelming
		start := len(a.memory.FailedAttempts) - 3
		if start < 0 {
			start = 0
		}
		for i := start; i < len(a.memory.FailedAttempts); i++ {
			memoryContext += fmt.Sprintf("❌ %s\n", a.memory.FailedAttempts[i])
		}
		memoryContext += "\n"
	}

	systemMsg := fmt.Sprintf(`You are an AI agent that executes terminal commands to complete high-level tasks on EXISTING projects.

TASK OBJECTIVE: %s

WORKING DIRECTORY: %s
%s

%s

PROJECT AWARENESS:
- You are working on an EXISTING project, not creating from scratch
- Respect existing code structure, conventions, and patterns
- Analyze existing files before making changes
- Understand the project's architecture and dependencies
- Follow the project's established coding style and practices
- Be careful not to break existing functionality

ENHANCED MEMORY SYSTEM:
- Your memory persists throughout the session
- Learn from previous commands and their outcomes
- Build upon completed steps rather than repeating work
- Avoid commands that have already failed
- Use accumulated knowledge to make better decisions

CRITICAL RULES:
1. NEVER use 'cd' commands - they don't work. Use full paths or relative paths from working directory
2. Use 'mkdir -p dirname' instead of 'mkdir dirname' to avoid errors
3. For multi-line files, use this syntax:
   cat > filename << 'EOF'
   file content here
   EOF
4. DON'T repeat commands that already failed (check PREVIOUS FAILURES section)
5. Include ALL required imports in code files
6. Check your work with 'ls' and 'cat filename' before marking done
7. ALWAYS analyze existing code before modifying it
8. Respect existing project structure and naming conventions
9. Test changes incrementally to avoid breaking existing functionality
10. Use project-specific tools and commands when available

WORKING WITH EXISTING PROJECTS:
- Start by understanding the current state (ls, find, cat key files)
- Identify the project's main entry points and structure
- Look for existing tests, documentation, and configuration
- Understand dependencies and build processes
- Make minimal, targeted changes that fit the existing codebase
- Preserve existing functionality while adding new features

PLANNING AND MEMORY:
- This is a HIGH-LEVEL task that may require multiple steps
- Create a comprehensive plan considering the existing project structure
- Break down the task into smaller, incremental steps
- Remember your progress and update your plan as needed
- Use the "plan" field to store your long-term strategy
- Use the "progress" field to track what you've accomplished
- Learn from the project structure and adapt your approach accordingly

RESPONSE FORMAT:
You must respond ONLY in JSON format with these fields:
{
  "done": false,
  "command": "exact_command_to_execute",
  "thought": "brief_explanation_of_current_step_and_reasoning",
  "plan": "your_overall_strategy_considering_existing_project_structure",
  "progress": "what_youve_accomplished_so_far_in_this_session"
}

OR when task is complete:
{
  "done": true,
  "thought": "task_completion_summary_and_what_was_learned",
  "progress": "final_summary_of_what_was_accomplished"
}

GOOD command examples for existing projects:
- ls -la (explore current directory structure)
- find . -name "*.go" -type f | head -10 (find specific files, limit output)
- cat package.json (understand project configuration)
- grep -r "function_name" . --include="*.js" (search existing code)
- git log --oneline -5 (understand recent changes)
- cat README.md (understand project purpose and setup)
- tree -L 2 (get project structure overview)
- head -20 main.go (examine existing code structure)

BAD command examples:
- cd directory (doesn't work)
- mkdir directory (use mkdir -p)
- echo 'line1\nline2' > file (escapes don't work)
- rm -rf important_directory (destructive without analysis)
- overwriting files without understanding their purpose

ANALYSIS BEFORE ACTION:
- Always examine existing files before modifying them
- Understand the project's dependencies and build system
- Look for existing patterns and follow them
- Check for tests and run them when appropriate
- Understand the project's purpose and architecture

Remember: You have a limit of %d remembered commands, so plan efficiently and build upon your accumulated knowledge!`,
		a.config.Task, a.workingDir, repoContext, memoryContext, MaxRememberedCommands)

	a.logger.WithField("system_msg_length", len(systemMsg)).Debug("Enhanced system message built")
	return systemMsg
}

func (a *Agent) buildUserMessage() string {
	a.logger.Debug("Building user message")

	if len(a.commandLog) == 0 {
		message := "No commands executed yet. Start by analyzing the existing project structure and understanding the codebase before proceeding with the task."

		// Add project context hints for first message
		if a.memory.ProjectContext.ProjectType != "Unknown" {
			message += fmt.Sprintf("\n\nProject Analysis:\n- Type: %s", a.memory.ProjectContext.ProjectType)
			if len(a.memory.ProjectContext.Languages) > 0 {
				message += fmt.Sprintf("\n- Languages: %s", strings.Join(a.memory.ProjectContext.Languages, ", "))
			}
			if len(a.memory.ProjectContext.KeyFiles) > 0 {
				message += fmt.Sprintf("\n- Key files detected: %s", strings.Join(a.memory.ProjectContext.KeyFiles, ", "))
			}
		}

		a.logger.Debug("No command history, returning initial message with project context")
		return message
	}

	message := ""

	// Include session summary
	sessionDuration := time.Since(a.memory.SessionStartTime)
	message += fmt.Sprintf("SESSION INFO:\n")
	message += fmt.Sprintf("Duration: %s\n", sessionDuration.Round(time.Second))
	message += fmt.Sprintf("Commands executed: %d\n", len(a.commandLog))
	message += fmt.Sprintf("Completed steps: %d\n", len(a.memory.CompletedSteps))
	message += fmt.Sprintf("Failed attempts: %d\n\n", len(a.memory.FailedAttempts))

	// Include current plan if available
	if a.memory.CurrentPlan != "" {
		message += fmt.Sprintf("CURRENT PLAN:\n%s\n\n", a.memory.CurrentPlan)
	}

	// Show recent command history (limited to save tokens)
	message += "RECENT COMMAND HISTORY:\n"
	start := 0
	if len(a.commandLog) > 7 {
		start = len(a.commandLog) - 7
	}

	for i := start; i < len(a.commandLog); i++ {
		log := a.commandLog[i]
		status := "✅"
		if log.Error != "" {
			status = "❌"
		}

		// Truncate long outputs to save tokens
		output := strings.TrimSpace(log.Output)
		if len(output) > 150 {
			output = output[:150] + "... [truncated]"
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
	message += fmt.Sprintf("Duration: %s\n", lastLog.Duration)

	if lastLog.Output != "" {
		output := strings.TrimSpace(lastLog.Output)
		if len(output) > 400 {
			output = output[:400] + "... [truncated]"
		}
		message += fmt.Sprintf("Output: %s\n", output)
	}

	if lastLog.Error != "" {
		message += fmt.Sprintf("Error: %s\n", lastLog.Error)
	}

	// Add memory insights
	if len(a.memory.KeyFindings) > 0 {
		message += fmt.Sprintf("\nKEY INSIGHTS FROM SESSION:\n")
		// Show last 3 key findings
		start := len(a.memory.KeyFindings) - 3
		if start < 0 {
			start = 0
		}
		for i := start; i < len(a.memory.KeyFindings); i++ {
			message += fmt.Sprintf("🔍 %s\n", a.memory.KeyFindings[i])
		}
	}

	message += fmt.Sprintf("\nWorking Directory: %s", a.workingDir)
	message += fmt.Sprintf("\nProject Type: %s", a.memory.ProjectContext.ProjectType)
	message += fmt.Sprintf("\nCommands executed: %d/%d remembered", len(a.commandLog), MaxRememberedCommands)

	a.logger.WithFields(logrus.Fields{
		"user_msg_length":   len(message),
		"commands_included": len(a.commandLog) - start,
		"total_commands":    len(a.commandLog),
		"key_findings":      len(a.memory.KeyFindings),
		"completed_steps":   len(a.memory.CompletedSteps),
	}).Debug("User message built with enhanced context")

	return message
}

func (a *Agent) parseResponse(response string) (*AgentResponse, error) {
	a.logger.WithField("response_length", len(response)).Debug("Parsing GPT response")

	var agentResp AgentResponse

	// Clean up response - remove markdown code blocks
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```") {
		a.logger.Debug("Removing markdown code blocks from response")
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
		a.logger.WithField("response", response).Error("No valid JSON found in response")
		return nil, fmt.Errorf("no valid JSON found in response: %s", response)
	}

	jsonStr := response[start : end+1]
	a.logger.WithField("json_length", len(jsonStr)).Debug("Extracted JSON from response")

	err := json.Unmarshal([]byte(jsonStr), &agentResp)
	if err != nil {
		a.logger.WithError(err).WithField("json", jsonStr).Error("Failed to parse JSON response")
		return nil, fmt.Errorf("failed to parse JSON: %v, response: %s", err, jsonStr)
	}

	a.logger.WithFields(logrus.Fields{
		"done":         agentResp.Done,
		"has_command":  agentResp.Command != "",
		"has_thought":  agentResp.Thought != "",
		"has_plan":     agentResp.Plan != "",
		"has_progress": agentResp.Progress != "",
	}).Debug("Successfully parsed agent response")

	return &agentResp, nil
}

func (a *Agent) Run() error {
	a.logger.WithFields(logrus.Fields{
		"task":         a.config.Task,
		"provider":     a.config.Provider,
		"working_dir":  a.workingDir,
		"max_commands": a.config.MaxCommands,
		"dry_run":      a.config.DryRun,
		"project_type": a.memory.ProjectContext.ProjectType,
	}).Info("Starting enhanced agent execution")

	a.logf("🚀 Starting enhanced agent with task: %s", a.config.Task)
	a.logf("🤖 Using provider: %s", a.config.Provider)
	a.logf("📁 Working directory: %s", a.workingDir)
	a.logf("🏗️ Project type: %s", a.memory.ProjectContext.ProjectType)
	if len(a.memory.ProjectContext.Languages) > 0 {
		a.logf("💻 Languages: %s", strings.Join(a.memory.ProjectContext.Languages, ", "))
	}
	a.logf("📝 Maximum commands allowed: %d", a.config.MaxCommands)
	a.logf("🧠 Command memory limit: %d", MaxRememberedCommands)
	if a.config.DryRun {
		a.logf("🔍 Running in DRY RUN mode - no commands will be executed")
	}
	a.logf("")

	for i := 0; i < a.config.MaxCommands; i++ {
		iterationLogger := a.logger.WithFields(logrus.Fields{
			"iteration":       i + 1,
			"max_commands":    a.config.MaxCommands,
			"commands_used":   len(a.commandLog),
			"completed_steps": len(a.memory.CompletedSteps),
		})

		iterationLogger.Info("Starting iteration")
		a.logf("--- Iteration %d/%d ---", i+1, a.config.MaxCommands)

		systemMsg := a.buildSystemMessage()
		userMsg := a.buildUserMessage()

		if a.config.Verbose {
			a.logf("📤 Sending request to %s...", a.config.Provider)
		}

		iterationLogger.Debug("Sending request to GPT client")
		response, err := a.client.Complete(systemMsg, userMsg)
		if err != nil {
			iterationLogger.WithError(err).Error("GPT request failed")
			return fmt.Errorf("GPT request failed: %v", err)
		}

		iterationLogger.WithField("response_length", len(response)).Debug("Received GPT response")

		if a.config.Verbose {
			a.logf("📥 GPT Response: %s", response)
		}

		agentResp, err := a.parseResponse(response)
		if err != nil {
			iterationLogger.WithError(err).Warn("Failed to parse response, continuing")
			a.logf("❌ Failed to parse response: %v", err)
			continue
		}

		// Update memory with response
		a.updateMemoryFromResponse(agentResp)

		// Update long-term plan if provided
		if agentResp.Plan != "" {
			a.longTermPlan = agentResp.Plan
			iterationLogger.WithField("plan", agentResp.Plan).Debug("Updated long-term plan")
			if a.config.Verbose {
				a.logf("📋 Plan updated: %s", agentResp.Plan)
			}
		}

		if agentResp.Progress != "" {
			iterationLogger.WithField("progress", agentResp.Progress).Info("Progress update")
			a.logf("📊 Progress: %s", agentResp.Progress)
		}

		if agentResp.Thought != "" {
			iterationLogger.WithField("thought", agentResp.Thought).Debug("Agent thought")
			a.logf("💭 Agent thought: %s", agentResp.Thought)
		}

		if agentResp.Done {
			iterationLogger.WithFields(logrus.Fields{
				"total_commands":   len(a.commandLog),
				"completed_steps":  len(a.memory.CompletedSteps),
				"final_progress":   agentResp.Progress,
				"session_duration": time.Since(a.memory.SessionStartTime).Round(time.Second),
			}).Info("Task completed successfully")

			a.logf("✅ Task completed successfully!")
			a.logf("📊 Total commands executed: %d", len(a.commandLog))
			a.logf("🎯 Completed steps: %d", len(a.memory.CompletedSteps))
			a.logf("⏱️ Session duration: %s", time.Since(a.memory.SessionStartTime).Round(time.Second))
			if agentResp.Progress != "" {
				a.logf("🎯 Final result: %s", agentResp.Progress)
			}
			return nil
		}

		if agentResp.Command == "" {
			iterationLogger.Warn("No command provided in response")
			a.logf("⚠️  No command provided, continuing...")
			continue
		}

		iterationLogger.WithField("command", agentResp.Command).Info("Executing command from agent")
		output, err := a.executeCommand(agentResp.Command)

		if err != nil {
			iterationLogger.WithError(err).WithField("command", agentResp.Command).Error("Command execution failed")
			a.logf("❌ Command failed: %v", err)
		} else {
			iterationLogger.WithField("command", agentResp.Command).Info("Command executed successfully")
			a.logf("✅ Command executed successfully")
		}

		if a.config.Verbose && output != "" {
			outputPreview := strings.TrimSpace(output)
			if len(outputPreview) > 100 {
				outputPreview = outputPreview[:100] + "..."
			}
			iterationLogger.WithFields(logrus.Fields{
				"output_length":  len(output),
				"output_preview": outputPreview,
			}).Debug("Command output details")
			a.logf("📄 Output: %s", strings.TrimSpace(output))
		}

		a.logf("")
	}

	a.logger.WithFields(logrus.Fields{
		"max_commands":     a.config.MaxCommands,
		"commands_used":    len(a.commandLog),
		"completed_steps":  len(a.memory.CompletedSteps),
		"failed_attempts":  len(a.memory.FailedAttempts),
		"task":             a.config.Task,
		"session_duration": time.Since(a.memory.SessionStartTime).Round(time.Second),
	}).Error("Reached maximum commands without completion")

	return fmt.Errorf("reached maximum number of commands (%d) without completion", a.config.MaxCommands)
}

// createGPTClient creates the appropriate GPT client based on configuration
func createGPTClient(cfg *config.Config) (gpt.GPTClient, error) {
	logger := logrus.WithField("provider", cfg.Provider)
	logger.Info("Creating GPT client")

	switch cfg.Provider {
	case "yandex":
		logger.Debug("Creating Yandex GPT client")
		return gpt.NewYandexGPTClient(cfg.FolderID, cfg.IAMToken), nil
	case "openai":
		logger.WithField("model", cfg.OpenAIModel).Debug("Creating OpenAI client")
		return gpt.NewOpenAIClient(cfg.OpenAIKey, cfg.OpenAIModel), nil
	case "deepseek":
		logger.WithField("model", cfg.DeepSeekModel).Debug("Creating DeepSeek client")
		return gpt.NewDeepSeekClient(cfg.DeepSeekKey, cfg.DeepSeekModel), nil
	case "claude":
		logger.WithField("model", cfg.ClaudeModel).Debug("Creating Claude client")
		return gpt.NewClaudeClient(cfg.ClaudeKey, cfg.ClaudeModel), nil
	case "gemini":
		logger.WithField("model", cfg.GeminiModel).Debug("Creating Gemini client")
		return gpt.NewGeminiClient(cfg.GeminiKey, cfg.GeminiModel), nil
	default:
		logger.Error("Unsupported provider")
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}

func setupGlobalLogger(cfg *config.Config) {
	// Configure global logrus settings
	if cfg.Verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else if cfg.Quiet {
		logrus.SetLevel(logrus.ErrorLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	logrus.WithFields(logrus.Fields{
		"log_level": logrus.GetLevel().String(),
		"verbose":   cfg.Verbose,
		"quiet":     cfg.Quiet,
	}).Debug("Global logger configured")
}

func main() {
	// Parse configuration first
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

	// Setup global logging
	setupGlobalLogger(cfg)

	mainLogger := logrus.WithFields(logrus.Fields{
		"version": "1.0.0", // You might want to make this configurable
		"task":    cfg.Task,
	})

	mainLogger.Info("Starting g8t agent with enhanced memory and project awareness")

	// Print configuration if verbose
	if cfg.Verbose {
		mainLogger.Debug("Printing configuration")
		cfg.PrintConfig()
	}

	// Create GPT client based on provider
	mainLogger.Debug("Creating GPT client")
	client, err := createGPTClient(cfg)
	if err != nil {
		mainLogger.WithError(err).Fatal("Failed to create GPT client")
	}
	mainLogger.Info("GPT client created successfully")

	// Create agent
	mainLogger.Debug("Creating enhanced agent")
	agent, err := NewAgent(client, cfg)
	if err != nil {
		mainLogger.WithError(err).Fatal("Failed to create agent")
	}
	defer func() {
		mainLogger.Debug("Closing agent")
		agent.Close()
	}()

	mainLogger.WithFields(logrus.Fields{
		"project_type": agent.memory.ProjectContext.ProjectType,
		"languages":    agent.memory.ProjectContext.Languages,
		"is_git_repo":  agent.memory.ProjectContext.GitInfo.IsRepo,
	}).Info("Enhanced agent created successfully with project context, starting execution")

	// Run agent
	if err := agent.Run(); err != nil {
		mainLogger.WithError(err).Fatal("Agent execution failed")
	}

	mainLogger.Info("Enhanced agent execution completed successfully")
}
