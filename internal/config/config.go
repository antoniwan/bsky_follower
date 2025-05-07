package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"bsky_follower/internal/models"

	"github.com/joho/godotenv"
)

const defaultTimeout = 10 * time.Second

// LoadConfig loads configuration from environment variables
func LoadConfig() (*models.Config, error) {
	// Try to load .env file, but don't fail if it doesn't exist
	_ = godotenv.Load()

	identifier := os.Getenv("BSKY_IDENTIFIER")
	password := os.Getenv("BSKY_PASSWORD")
	
	if identifier == "" || password == "" {
		return nil, fmt.Errorf("BSKY_IDENTIFIER and BSKY_PASSWORD environment variables must be set")
	}

	// Load fallback handles from environment variable if available
	var fallbackHandles []string
	if fallbackEnv := os.Getenv("BSKY_FALLBACK_HANDLES"); fallbackEnv != "" {
		fallbackHandles = strings.Split(fallbackEnv, ",")
	}

	// Parse timeout from environment variable
	timeout := defaultTimeout
	if timeoutStr := os.Getenv("BSKY_TIMEOUT"); timeoutStr != "" {
		if timeoutSec, err := strconv.Atoi(timeoutStr); err == nil && timeoutSec > 0 {
			timeout = time.Duration(timeoutSec) * time.Second
		}
	}
	
	return &models.Config{
		Identifier:      identifier,
		Password:        password,
		Timeout:         timeout,
		FallbackHandles: fallbackHandles,
	}, nil
} 