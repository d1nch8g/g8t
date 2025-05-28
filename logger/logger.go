package logger

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
)

type Logger struct {
	verbose bool
	quiet   bool
}

func New(verbose, quiet bool) *Logger {
	return &Logger{
		verbose: verbose,
		quiet:   quiet,
	}
}

func (l *Logger) Info(msg string, args ...interface{}) {
	if l.quiet {
		return
	}
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("🔵 %s %s\n", color.CyanString(timestamp), fmt.Sprintf(msg, args...))
}

func (l *Logger) Success(msg string, args ...interface{}) {
	if l.quiet {
		return
	}
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("✅ %s %s\n", color.GreenString(timestamp), fmt.Sprintf(msg, args...))
}

func (l *Logger) Warning(msg string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("⚠️  %s %s\n", color.YellowString(timestamp), fmt.Sprintf(msg, args...))
}

func (l *Logger) Error(msg string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05")
	fmt.Fprintf(os.Stderr, "❌ %s %s\n", color.RedString(timestamp), fmt.Sprintf(msg, args...))
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	if !l.verbose {
		return
	}
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("🔍 %s %s\n", color.MagentaString(timestamp), fmt.Sprintf(msg, args...))
}

func (l *Logger) StartAgent(provider string, task string, maxCommands int, dryRun bool) {
	fmt.Printf("\n🚀 Starting AI Agent\n")
	fmt.Printf("   Provider: %s\n", color.CyanString(provider))
	fmt.Printf("   Task: %s\n", color.WhiteString(task))
	fmt.Printf("   Max Commands: %s\n", color.YellowString("%d", maxCommands))
	if dryRun {
		fmt.Printf("   Mode: %s\n", color.MagentaString("DRY RUN"))
	}
	fmt.Println()
}

func (l *Logger) StartStep(step int) {
	fmt.Printf("⚙️  Step %s\n", color.CyanString("%d", step))
}

func (l *Logger) ExecuteCommand(command, thought string) {
	fmt.Printf("🔧 %s\n", color.WhiteString(command))
	if l.verbose && thought != "" {
		fmt.Printf("   💭 %s\n", color.HiBlackString(thought))
	}
}

func (l *Logger) CommandSuccess(output string) {
	if output != "" && l.verbose {
		fmt.Printf("   📤 %s\n", color.GreenString(output))
	}
	fmt.Printf("✅ Command completed\n\n")
}

func (l *Logger) CommandError(err error) {
	fmt.Printf("❌ Command failed: %s\n\n", color.RedString(err.Error()))
}

func (l *Logger) TaskCompleted(thought string) {
	fmt.Printf("🎉 %s\n", color.GreenString("Task completed successfully!"))
	if thought != "" && l.verbose {
		fmt.Printf("   💭 %s\n", color.HiBlackString(thought))
	}
	fmt.Println()
}
