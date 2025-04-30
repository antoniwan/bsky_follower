package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

// Logger provides structured logging capabilities
type Logger struct {
	*log.Logger
	debugMode bool
}

// NewLogger creates a new logger instance
func NewLogger(w io.Writer) *Logger {
	return &Logger{
		Logger:    log.New(w, "", log.Ldate|log.Ltime|log.Lmicroseconds),
		debugMode: os.Getenv("DEBUG_MODE") == "true",
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
	if l.debugMode {
		l.Printf("[DEBUG] "+format, v...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	l.Printf("[WARN] "+format, v...)
}

// Audit logs an audit message
func (l *Logger) Audit(format string, v ...interface{}) {
	l.Printf("[AUDIT] "+format, v...)
}

// Trace logs a trace message
func (l *Logger) Trace(format string, v ...interface{}) {
	if l.debugMode {
		l.Printf("[TRACE] "+format, v...)
	}
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
	Followed  bool   `json:"followed"`
}

// App represents the main application
type App struct {
	config   *Config
	client   *http.Client
	followed map[string]bool
	logger   *Logger
	db       *sql.DB
}

// NewApp creates a new application instance
func NewApp(config *Config) (*App, error) {
	// Initialize SQLite database
	app := &App{
		config:   config,
		client:   &http.Client{Timeout: config.Timeout},
		followed: make(map[string]bool),
		logger:   NewLogger(os.Stdout),
	}

	// Initialize database
	db, err := sql.Open("sqlite", "users.db")
	if err != nil {
		app.logger.Error("Failed to open database: %v", err)
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	app.db = db

	// Create users table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			handle TEXT PRIMARY KEY,
			did TEXT,
			followers INTEGER,
			saved_on TEXT,
			followed BOOLEAN
		)
	`)
	if err != nil {
		app.logger.Error("Failed to create table: %v", err)
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	app.logger.Info("Application initialized with debug mode: %v", app.logger.debugMode)
	app.logger.Debug("Database connection established")
	return app, nil
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
func (app *App) getFollowerCount(session *Session, actor string) (int, error) {
	app.logger.Debug("Getting follower count for actor: %s", actor)
	
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

// loadUsersFromDB loads users from the SQLite database
func (app *App) loadUsersFromDB() ([]TargetUser, error) {
	app.logger.Debug("Loading users from database")
	startTime := time.Now()
	
	rows, err := app.db.Query("SELECT handle, did, followers, saved_on, followed FROM users")
	if err != nil {
		app.logger.Error("Failed to query users: %v", err)
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []TargetUser
	for rows.Next() {
		var user TargetUser
		err := rows.Scan(&user.Handle, &user.DID, &user.Followers, &user.SavedOn, &user.Followed)
		if err != nil {
			app.logger.Error("Failed to scan user: %v", err)
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
		app.logger.Trace("Loaded user: %+v", user)
	}

	if err = rows.Err(); err != nil {
		app.logger.Error("Error iterating rows: %v", err)
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	app.logger.Debug("Successfully loaded %d users from database in %v", len(users), time.Since(startTime))
	return users, nil
}

// saveUserToDB saves a user to the SQLite database
func (app *App) saveUserToDB(user TargetUser) error {
	app.logger.Debug("Saving user to database: %+v", user)
	startTime := time.Now()
	
	// Use UPSERT to either insert or update the user
	result, err := app.db.Exec(`
		INSERT INTO users (handle, did, followers, saved_on, followed)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(handle) DO UPDATE SET
			did = excluded.did,
			followers = excluded.followers,
			saved_on = excluded.saved_on,
			followed = CASE 
				WHEN users.followed = 1 THEN 1 
				ELSE excluded.followed 
			END
	`, user.Handle, user.DID, user.Followers, user.SavedOn, user.Followed)
	
	if err != nil {
		app.logger.Error("Failed to save user: %v", err)
		return fmt.Errorf("failed to save user: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	app.logger.Debug("Successfully saved user to database in %v (rows affected: %d)", time.Since(startTime), rowsAffected)
	app.logger.Audit("User saved/updated: %s (followers: %d, followed: %v)", user.Handle, user.Followers, user.Followed)
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
func (app *App) fetchAndSaveTopUsers(simulate bool) error {
	app.logger.Info("Starting to fetch and save top users to database")
	
	// Load existing users
	existingUsers, err := app.loadUsersFromDB()
	if err != nil {
		app.logger.Error("Failed to load existing users: %v", err)
		return fmt.Errorf("failed to load existing users: %w", err)
	}
	app.logger.Info("Loaded %d existing users from database", len(existingUsers))

	// Get a session to authenticate all requests
	session, err := app.login()
	if err != nil {
		app.logger.Error("Failed to login: %v", err)
		return fmt.Errorf("failed to login: %w", err)
	}

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
		followers, err := app.getFollowerCount(session, handle)
		if err != nil {
			app.logger.Warn("Failed to get follower count for %s: %v", handle, err)
			continue
		}

		newUser := TargetUser{
			Handle:    handle,
			Followers: followers,
			SavedOn:   time.Now().Format(time.RFC3339),
		}

		if err := app.saveUserToDB(newUser); err != nil {
			app.logger.Error("Failed to save user %s: %v", handle, err)
			return fmt.Errorf("failed to save user %s: %w", handle, err)
		}
		app.logger.Info("Successfully saved user: %s (followers: %d)", handle, followers)
	}

	app.logger.Info("Completed fetching and saving top users")
	return nil
}

// followUser follows a user on Bluesky
func (app *App) followUser(session *Session, handleOrDid string, simulate bool) error {
	app.logger.Info("Attempting to follow user: %s (simulate=%v)", handleOrDid, simulate)
	startTime := time.Now()
	
	// Check if user is already being followed in memory
	if app.followed[handleOrDid] {
		app.logger.Info("Already followed %s in memory (skipped)", handleOrDid)
		return nil
	}

	// Check if user is already being followed in the database
	var followed bool
	err := app.db.QueryRow("SELECT followed FROM users WHERE handle = ? OR did = ?", handleOrDid, handleOrDid).Scan(&followed)
	if err == nil && followed {
		app.logger.Info("User %s is already marked as followed in database (skipped)", handleOrDid)
		app.followed[handleOrDid] = true
		return nil
	}

	if simulate {
		app.logger.Info("Simulation mode: would follow: %s", handleOrDid)
		return nil
	}

	app.logger.Debug("Preparing follow request for %s", handleOrDid)
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

	app.logger.Debug("Sending follow request for %s", handleOrDid)
	resp, err := app.client.Do(req)
	if err != nil {
		app.logger.Error("Failed to execute follow request: %v", err)
		return fmt.Errorf("failed to execute follow request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		app.logger.Info("Successfully followed: %s (took %v)", handleOrDid, time.Since(startTime))
		app.followed[handleOrDid] = true
		
		// Update the user's followed status in the database
		_, err := app.db.Exec("UPDATE users SET followed = 1 WHERE handle = ? OR did = ?", handleOrDid, handleOrDid)
		if err != nil {
			app.logger.Error("Failed to update user's followed status: %v", err)
			return fmt.Errorf("failed to update user's followed status: %w", err)
		}
		app.logger.Audit("User followed: %s", handleOrDid)
	} else if resp.StatusCode == http.StatusBadRequest {
		app.logger.Info("Already following: %s (took %v)", handleOrDid, time.Since(startTime))
		app.followed[handleOrDid] = true
		return nil
	} else {
		app.logger.Error("Failed to follow %s. Status: %d (took %v)", handleOrDid, resp.StatusCode, time.Since(startTime))
		return fmt.Errorf("failed to follow %s. Status: %d", handleOrDid, resp.StatusCode)
	}
	return nil
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// Parse command line flags
	simulate := flag.Bool("simulate", false, "Simulate actions without making actual changes")
	updateTop := flag.Bool("update-top", false, "Fetch top users from bsky.directory and save to database")
	minFollowers := flag.String("min-followers", "0", "Minimum followers required to follow")
	realFollow := flag.Bool("real", false, "Actually follow users (default is simulation only)")
	flag.Parse()

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	app, err := NewApp(config)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}
	defer app.db.Close()

	app.logger.Info("Starting Bluesky follower application")
	app.logger.Info("Configuration loaded: timeout=%v, simulate=%v", config.Timeout, *simulate)

	if *updateTop {
		if err := app.fetchAndSaveTopUsers(*simulate); err != nil {
			app.logger.Error("Failed to fetch and save top users: %v", err)
			os.Exit(1)
		}
		return
	}

	// Load users from database
	users, err := app.loadUsersFromDB()
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
			count, err = app.getFollowerCount(session, actor)
			if err != nil {
				app.logger.Warn("Skipping %s (error getting follower count: %v)", actor, err)
				continue
			}
			user.Followers = count
			user.SavedOn = time.Now().UTC().Format(time.RFC3339)
			if err := app.saveUserToDB(user); err != nil {
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
