package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Config holds logger configuration
type Config struct {
	DebugMode   bool
	LogToFile   bool
	LogFilePath string
	MaxSize     int
	MaxBackups  int
	MaxAge      int
	Compress    bool
	LogLevel    string
}

// Logger represents a logger instance
type Logger struct {
	config *Config
	writer *lumberjack.Logger
}

// NewLogger creates a new logger instance
func NewLogger(config *Config) *Logger {
	logger := &Logger{
		config: config,
	}

	if config.LogToFile {
		// Ensure log directory exists
		if err := os.MkdirAll(filepath.Dir(config.LogFilePath), 0755); err != nil {
			fmt.Printf("Failed to create log directory: %v\n", err)
			os.Exit(1)
		}

		logger.writer = &lumberjack.Logger{
			Filename:   config.LogFilePath,
			MaxSize:    config.MaxSize,
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge,
			Compress:   config.Compress,
		}
	}

	return logger
}

// IsDebugMode returns whether debug mode is enabled
func (l *Logger) IsDebugMode() bool {
	return l.config.DebugMode
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log("INFO", msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	l.log("ERROR", msg, args...)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	if l.config.DebugMode {
		l.log("DEBUG", msg, args...)
	}
}

// log writes a log message
func (l *Logger) log(level, msg string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	formattedMsg := fmt.Sprintf(msg, args...)
	logEntry := fmt.Sprintf("[%s] %s: %s\n", timestamp, level, formattedMsg)

	if l.config.LogToFile {
		l.writer.Write([]byte(logEntry))
	} else {
		fmt.Print(logEntry)
	}
} 