package main

import (
	"io"
	"os"

	"github.com/d1nch8g/g8t/agent"
	"github.com/d1nch8g/g8t/config"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		PadLevelText:    true,
	})

	// Parse configuration
	cfg, err := config.Parse()
	if err != nil {
		logrus.Errorf("Error parsing configuration: %v", err)
		os.Exit(1)
	}

	// Configure log file if specified
	if cfg.LogFile != "" {
		logFile, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			logrus.Errorf("Failed to open log file: %v", err)
			os.Exit(1)
		}
		logrus.SetOutput(io.MultiWriter(logFile, os.Stderr))
	}

	// Create and run agent
	agentInstance, err := agent.New(cfg, logrus.StandardLogger())
	if err != nil {
		logrus.Errorf("Failed to create agent: %v", err)
		os.Exit(1)
	}

	if err := agentInstance.Run(cfg.Task); err != nil {
		logrus.Errorf("Agent execution failed: %v", err)
		os.Exit(1)
	}
}
