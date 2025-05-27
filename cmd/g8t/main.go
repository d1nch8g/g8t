package main

import (
	"fmt"
	"os"

	"github.com/d1nch8g/g8t/agent"
	"github.com/d1nch8g/g8t/config"
	"github.com/d1nch8g/g8t/logger"
)

func main() {
	// Parse configuration
	cfg, err := config.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New(cfg.GetLogLevel(), cfg.LogFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Close()

	// Create and run agent
	agentInstance, err := agent.New(cfg, log)
	if err != nil {
		log.Error("Failed to create agent", "error", err)
		os.Exit(1)
	}

	if err := agentInstance.Run(); err != nil {
		log.Error("Agent execution failed", "error", err)
		os.Exit(1)
	}
}
