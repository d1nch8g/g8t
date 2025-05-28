package main

import (
	"os"

	"github.com/d1nch8g/g8t/agent"
	"github.com/d1nch8g/g8t/config"
	"github.com/d1nch8g/g8t/logger"
)

func main() {
	// Parse configuration
	cfg, err := config.Parse()
	if err != nil {
		logger := logger.New(false, false)
		logger.Error("Error parsing configuration: %v", err)
		os.Exit(1)
	}

	// Create logger with config settings
	log := logger.New(cfg.Verbose, cfg.Quiet)

	// Create and run agent
	agentInstance, err := agent.New(cfg, log)
	if err != nil {
		log.Error("Failed to create agent: %v", err)
		os.Exit(1)
	}

	if err := agentInstance.Run(cfg.Task); err != nil {
		log.Error("Agent execution failed: %v", err)
		os.Exit(1)
	}
}
