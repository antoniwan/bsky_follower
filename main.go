package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Logger provides structured logging capabilities
type Logger struct {
	*log.Logger
}

// NewLogger creates a new logger instance
func NewLogger(w io.Writer) *Logger {
	return &Logger{
		Logger: log.New(w, "", log.Ldate|log.Ltime|log.Lmicroseconds),
	}
}

// Info logs an informational message
func (l *Logger) Info(format string, v ...interface{}) {
	l.Printf("[INFO] "+format, v...)
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	l.Printf("[ERROR] "+format, v...)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	l.Printf("[DEBUG] "+format, v...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	l.Printf("[WARN] "+format, v...)
}

const (
	apiBase = "https://bsky.social/xrpc"
	defaultTimeout = 10 * time.Second
)

// Config holds application configuration
type Config struct {
	Identifier string
	Password   string
	Timeout    time.Duration
	FallbackHandles []string // Configurable fallback handles
}

// Session represents an authenticated Bluesky session
type Session struct {
	AccessJwt string `json:"accessJwt"`
	Did       string `json:"did"`
	Handle    string `json:"handle"`
}

// Profile represents a user's profile information
type Profile struct {
	FollowersCount int `json:"followersCount"`
}

// FollowRecord represents a follow action
type FollowRecord struct {
	Subject string `json:"subject"`
}

// TargetUser represents a user to follow
type TargetUser struct {
	Handle    string `json:"handle"`
	DID       string `json:"did"`
	Followers int    `json:"followers"`
	SavedOn   string `json:"savedOn"`
}

// App represents the main application
type App struct {
	config   *Config
	client   *http.Client
	followed map[string]bool
	logger   *Logger
}

// NewApp creates a new application instance
func NewApp(config *Config) *App {
	return &App{
		config:   config,
		client:   &http.Client{Timeout: config.Timeout},
		followed: make(map[string]bool),
		logger:   NewLogger(os.Stdout),
	}
}

// loadConfig loads configuration from environment variables
func loadConfig() (*Config, error) {
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
	
	return &Config{
		Identifier: identifier,
		Password:   password,
		Timeout:    timeout,
		FallbackHandles: fallbackHandles,
	}, nil
}

// login authenticates with the Bluesky API
func (app *App) login() (*Session, error) {
	app.logger.Info("Attempting to login with identifier: %s", app.config.Identifier)
	
	payload := map[string]string{
		"identifier": app.config.Identifier,
		"password":   app.config.Password,
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		app.logger.Error("Failed to marshal login payload: %v", err)
		return nil, fmt.Errorf("failed to marshal login payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiBase+"/com.atproto.server.createSession", bytes.NewBuffer(jsonData))
	if err != nil {
		app.logger.Error("Failed to create login request: %v", err)
		return nil, fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.client.Do(req)
	if err != nil {
		app.logger.Error("Failed to execute login request: %v", err)
		return nil, fmt.Errorf("failed to execute login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		app.logger.Error("Login failed with status code: %d", resp.StatusCode)
		return nil, fmt.Errorf("login failed with status code: %d", resp.StatusCode)
	}

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		app.logger.Error("Failed to decode login response: %v", err)
		return nil, fmt.Errorf("failed to decode login response: %w", err)
	}

	app.logger.Info("Successfully logged in as: %s (DID: %s)", session.Handle, session.Did)
	return &session, nil
}

// getFollowerCount retrieves the follower count for a user
func (app *App) getFollowerCount(actor string) (int, error) {
	app.logger.Debug("Getting follower count for actor: %s", actor)
	
	// Get a session to authenticate
	session, err := app.login()
	if err != nil {
		app.logger.Error("Failed to login: %v", err)
		return 0, fmt.Errorf("failed to login: %w", err)
	}
	
	url := apiBase + "/app.bsky.actor.getProfile?actor=" + actor
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		app.logger.Error("Failed to create profile request: %v", err)
		return 0, fmt.Errorf("failed to create profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	
	resp, err := app.client.Do(req)
	if err != nil {
		app.logger.Error("Failed to fetch profile: %v", err)
		return 0, fmt.Errorf("failed to fetch profile: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		app.logger.Error("Profile fetch failed with status: %d", resp.StatusCode)
		return 0, fmt.Errorf("profile fetch failed with status: %d", resp.StatusCode)
	}
	
	var profile Profile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		app.logger.Error("Failed to decode profile response: %v", err)
		return 0, fmt.Errorf("failed to decode profile response: %w", err)
	}
	
	app.logger.Debug("Follower count for %s: %d", actor, profile.FollowersCount)
	return profile.FollowersCount, nil
}

// followUser follows a user on Bluesky
func (app *App) followUser(session *Session, handleOrDid string, simulate bool) error {
	app.logger.Info("Attempting to follow user: %s (simulate=%v)", handleOrDid, simulate)
	
	if app.followed[handleOrDid] {
		app.logger.Info("Already followed %s (skipped)", handleOrDid)
		return nil
	}

	if simulate {
		app.logger.Info("Simulation mode: would follow: %s", handleOrDid)
		return nil
	}

	payload := map[string]interface{}{
		"collection": "app.bsky.graph.follow",
		"repo":       session.Did,
		"record":     FollowRecord{Subject: handleOrDid},
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		app.logger.Error("Failed to marshal follow request: %v", err)
		return fmt.Errorf("failed to marshal follow request: %w", err)
	}

	req, err := http.NewRequest("POST", apiBase+"/com.atproto.repo.createRecord", bytes.NewBuffer(jsonBody))
	if err != nil {
		app.logger.Error("Failed to create follow request: %v", err)
		return fmt.Errorf("failed to create follow request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.client.Do(req)
	if err != nil {
		app.logger.Error("Failed to execute follow request: %v", err)
		return fmt.Errorf("failed to execute follow request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		app.logger.Info("Successfully followed: %s", handleOrDid)
		app.followed[handleOrDid] = true
	} else if resp.StatusCode == http.StatusBadRequest {
		app.logger.Info("Already following: %s", handleOrDid)
		app.followed[handleOrDid] = true
		return nil
	} else {
		app.logger.Error("Failed to follow %s. Status: %d", handleOrDid, resp.StatusCode)
		return fmt.Errorf("failed to follow %s. Status: %d", handleOrDid, resp.StatusCode)
	}
	return nil
}

// loadUsersFromJSON loads users from a JSON file
func (app *App) loadUsersFromJSON(filePath string) ([]TargetUser, error) {
	app.logger.Debug("Loading users from JSON file: %s", filePath)
	
	var users []TargetUser
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			app.logger.Info("Users file does not exist, returning empty list")
			return []TargetUser{}, nil
		}
		app.logger.Error("Failed to read users file: %v", err)
		return nil, fmt.Errorf("failed to read users file: %w", err)
	}
	if err := json.Unmarshal(data, &users); err != nil {
		app.logger.Error("Failed to unmarshal users: %v", err)
		return nil, fmt.Errorf("failed to unmarshal users: %w", err)
	}
	app.logger.Debug("Successfully loaded %d users from file", len(users))
	return users, nil
}

// saveUserToJSON saves users to a JSON file
func (app *App) saveUserToJSON(newUser TargetUser, filePath string) error {
	app.logger.Debug("Saving user to JSON file: %s", filePath)
	
	users, err := app.loadUsersFromJSON(filePath)
	if err != nil {
		app.logger.Error("Failed to load existing users: %v", err)
		return fmt.Errorf("failed to load existing users: %w", err)
	}
	
	// Check if user exists and update it, otherwise append
	found := false
	for i, u := range users {
		if u.Handle == newUser.Handle || u.DID == newUser.DID {
			users[i] = newUser
			found = true
			app.logger.Debug("Updated existing user: %s", newUser.Handle)
			break
		}
	}
	
	if !found {
		users = append(users, newUser)
		app.logger.Debug("Added new user: %s", newUser.Handle)
	}
	
	sort.Slice(users, func(i, j int) bool {
		return users[i].Followers > users[j].Followers
	})
	
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		app.logger.Error("Failed to marshal users: %v", err)
		return fmt.Errorf("failed to marshal users: %w", err)
	}
	
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		app.logger.Error("Failed to write users file: %v", err)
		return fmt.Errorf("failed to write users file: %w", err)
	}
	
	app.logger.Debug("Successfully saved users to file")
	return nil
}

// fetchTopHandlesFromBskyDirectory fetches top handles from Bluesky directory
func (app *App) fetchTopHandlesFromBskyDirectory() ([]string, error) {
	app.logger.Info("Fetching top handles from Bluesky directory")
	
	// First get a session to authenticate
	session, err := app.login()
	if err != nil {
		app.logger.Error("Failed to login: %v", err)
		return nil, fmt.Errorf("failed to login: %w", err)
	}

	url := apiBase + "/app.bsky.actor.getSuggestions?limit=50"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		app.logger.Error("Failed to create suggestions request: %v", err)
		return nil, fmt.Errorf("failed to create suggestions request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

	resp, err := app.client.Do(req)
	if err != nil {
		app.logger.Error("Failed to fetch from Bluesky API: %v", err)
		return nil, fmt.Errorf("failed to fetch from Bluesky API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		app.logger.Error("Suggestions fetch failed with status: %d", resp.StatusCode)
		return nil, fmt.Errorf("suggestions fetch failed with status: %d", resp.StatusCode)
	}

	var result struct {
		Actors []struct {
			Handle string `json:"handle"`
		} `json:"actors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		app.logger.Error("Failed to decode response: %v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var handles []string
	seen := make(map[string]bool)
	for _, actor := range result.Actors {
		handle := actor.Handle
		if !seen[handle] {
			handles = append(handles, handle)
			seen[handle] = true
		}
	}

	if len(handles) == 0 {
		if len(app.config.FallbackHandles) == 0 {
			app.logger.Error("No handles found and no fallback handles configured")
			return nil, fmt.Errorf("no handles found and no fallback handles configured")
		}
		app.logger.Warn("No handles found from API, using configured fallback handles")
		return app.config.FallbackHandles, nil
	}

	app.logger.Info("Successfully fetched %d unique handles", len(handles))
	return handles, nil
}

// fetchAndSaveTopUsers fetches and saves top users
func (app *App) fetchAndSaveTopUsers(filePath string, simulate bool) error {
	app.logger.Info("Starting to fetch and save top users to: %s", filePath)
	
	// Load existing users
	existingUsers, err := app.loadUsersFromJSON(filePath)
	if err != nil {
		app.logger.Error("Failed to load existing users: %v", err)
		return fmt.Errorf("failed to load existing users: %w", err)
	}
	app.logger.Info("Loaded %d existing users from file", len(existingUsers))

	// Fetch top handles from bsky.directory
	app.logger.Info("Fetching top handles from bsky.directory")
	handles, err := app.fetchTopHandlesFromBskyDirectory()
	if err != nil {
		app.logger.Error("Failed to fetch top handles: %v", err)
		return fmt.Errorf("failed to fetch top handles: %w", err)
	}
	app.logger.Info("Successfully fetched %d handles from bsky.directory", len(handles))

	// Create a map of existing users for quick lookup
	existingMap := make(map[string]bool)
	for _, user := range existingUsers {
		existingMap[user.Handle] = true
	}

	// Process each handle
	for _, handle := range handles {
		if existingMap[handle] {
			app.logger.Debug("Skipping existing user: %s", handle)
			continue
		}

		app.logger.Info("Processing new user: %s", handle)
		followers, err := app.getFollowerCount(handle)
		if err != nil {
			app.logger.Warn("Failed to get follower count for %s: %v", handle, err)
			continue
		}

		newUser := TargetUser{
			Handle:    handle,
			Followers: followers,
			SavedOn:   time.Now().Format(time.RFC3339),
		}

		if err := app.saveUserToJSON(newUser, filePath); err != nil {
			app.logger.Error("Failed to save user %s: %v", handle, err)
			return fmt.Errorf("failed to save user %s: %w", handle, err)
		}
		app.logger.Info("Successfully saved user: %s (followers: %d)", handle, followers)
	}

	app.logger.Info("Completed fetching and saving top users")
	return nil
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// Parse command line flags
	simulate := flag.Bool("simulate", false, "Simulate actions without making actual changes")
	filePath := flag.String("file", "users.json", "Path to the users JSON file")
	updateTop := flag.Bool("update-top", false, "Fetch top users from bsky.directory and save to JSON")
	minFollowers := flag.String("min-followers", "0", "Minimum followers required to follow")
	realFollow := flag.Bool("real", false, "Actually follow users (default is simulation only)")
	flag.Parse()

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	app := NewApp(config)
	app.logger.Info("Starting Bluesky follower application")
	app.logger.Info("Configuration loaded: timeout=%v, simulate=%v", config.Timeout, *simulate)

	if *updateTop {
		if err := app.fetchAndSaveTopUsers(*filePath, *simulate); err != nil {
			app.logger.Error("Failed to fetch and save top users: %v", err)
			os.Exit(1)
		}
		return
	}

	// Load users from JSON
	users, err := app.loadUsersFromJSON(*filePath)
	if err != nil {
		app.logger.Error("Failed to load users: %v", err)
		os.Exit(1)
	}

	// Parse minimum followers
	minFollowersCount, err := strconv.Atoi(*minFollowers)
	if err != nil {
		app.logger.Error("Invalid minimum followers value: %v", err)
		os.Exit(1)
	}

	// Login to get session
	session, err := app.login()
	if err != nil {
		app.logger.Error("Failed to login: %v", err)
		os.Exit(1)
	}

	// Process each user
	for _, user := range users {
		time.Sleep(3 * time.Second) // Rate limiting

		actor := user.Handle
		if actor == "" {
			actor = user.DID
		}
		if actor == "" {
			app.logger.Warn("Skipping user with no handle or DID")
			continue
		}

		// Update follower count if needed
		count := user.Followers
		if count == 0 {
			count, err = app.getFollowerCount(actor)
			if err != nil {
				app.logger.Warn("Skipping %s (error getting follower count: %v)", actor, err)
				continue
			}
			user.Followers = count
			user.SavedOn = time.Now().UTC().Format(time.RFC3339)
			if err := app.saveUserToJSON(user, *filePath); err != nil {
				app.logger.Error("Failed to save updated user %s: %v", actor, err)
			}
		}

		// Skip if below minimum followers
		if count < minFollowersCount {
			app.logger.Info("Skipping %s (%d < min %d)", actor, count, minFollowersCount)
			continue
		}

		// Follow user
		if err := app.followUser(session, actor, !*realFollow); err != nil {
			app.logger.Error("Follow error: %v", err)
		}
	}
}
