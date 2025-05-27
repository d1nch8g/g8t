package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type Logger struct {
	*slog.Logger
	file   *os.File
	writer io.Writer
}

func New(level, logFile string) (*Logger, error) {
	var writer io.Writer = os.Stderr
	var file *os.File

	// Setup file logging if specified
	if logFile != "" {
		// Create directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		file = f
		writer = io.MultiWriter(os.Stderr, f)
	}

	// Parse log level
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	// Create structured logger with timestamp
	opts := &slog.HandlerOptions{
		Level: logLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(time.Now().Format("2006-01-02 15:04:05"))
			}
			return a
		},
	}

	handler := slog.NewTextHandler(writer, opts)
	logger := slog.New(handler)

	return &Logger{
		Logger: logger,
		file:   file,
		writer: writer,
	}, nil
}

func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func (l *Logger) LogCommand(step int, command, output, error string) {
	l.Info("Command executed",
		"step", step,
		"command", command,
		"output", output,
		"error", error,
	)
}

func (l *Logger) LogThought(step int, thought string) {
	l.Debug("Agent thought",
		"step", step,
		"thought", thought,
	)
}
