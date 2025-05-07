package logger

import (
	"io"
	"log"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// APILogger implements the api.Logger interface
type APILogger struct{}

func (l *APILogger) Info(msg string, args ...interface{}) {
	log.Printf("[INFO] "+msg, args...)
}

func (l *APILogger) Error(msg string, args ...interface{}) {
	log.Printf("[ERROR] "+msg, args...)
}

func (l *APILogger) Debug(msg string, args ...interface{}) {
	if os.Getenv("DEBUG_MODE") == "true" {
		log.Printf("[DEBUG] "+msg, args...)
	}
}

// GetAPILogger returns a new API logger instance
func GetAPILogger() *APILogger {
	return &APILogger{}
}

func InitLogger() {
	// Configure the logger to write to both file and stdout
	logFile := &lumberjack.Logger{
		Filename:   "logs/bsky_follower.log",
		MaxSize:    100, // megabytes
		MaxBackups: 3,
		MaxAge:     7,    // days
		Compress:   true, // compress rotated logs
	}

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Fatal("Failed to create logs directory:", err)
	}

	// Set up multi-writer for both file and stdout
	var writers []io.Writer
	writers = append(writers, os.Stdout)
	
	if os.Getenv("DEBUG_MODE") != "true" {
		writers = append(writers, logFile)
	}

	// Configure the standard logger
	log.SetOutput(io.MultiWriter(writers...))
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
} 