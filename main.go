package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"container/heap"

	"bsky_follower/v1.0.0/logger"

	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

const (
	apiBase = "https://bsky.social/xrpc"
	defaultTimeout = 10 * time.Second
	maxFollowsPerHour = 50
	maxRetries = 3
	retryDelay = 5 * time.Minute
	followCooldown = 24 * time.Hour
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
	CreatedAt time.Time
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
	Handle      string    `json:"handle"`
	DID         string    `json:"did"`
	Followers   int       `json:"followers"`
	SavedOn     time.Time `json:"savedOn"`
	Followed    bool      `json:"followed"`
	LastChecked time.Time `json:"lastChecked"`
	FollowDate  time.Time `json:"followDate"`
	Priority    int       `json:"priority"`
	Attempts    int       `json:"attempts"`
}

// FollowQueueItem represents an item in the follow queue
type FollowQueueItem struct {
	User      TargetUser
	Priority  int
	Attempts  int
	NextTry   time.Time
	index     int // for heap implementation
}

// FollowQueue implements heap.Interface for priority queue
type FollowQueue []*FollowQueueItem

// App represents the main application
type App struct {
	config   *Config
	client   *http.Client
	followed map[string]bool
	logger   *logger.Logger
	db       *sql.DB
	queue    *FollowQueue
	mu       sync.Mutex
	lastFollow time.Time
	followCount int
	followReset time.Time
}

// NewApp creates a new application instance
func NewApp(config *Config) (*App, error) {
	// Initialize SQLite database
	app := &App{
		config:   config,
		client:   &http.Client{Timeout: config.Timeout},
		followed: make(map[string]bool),
		logger:   logger.NewLogger(&logger.Config{
			DebugMode:   os.Getenv("DEBUG_MODE") == "true",
			LogToFile:   true,
			LogFilePath: "logs/bsky_follower.log",
			MaxSize:     100,    // 100MB
			MaxBackups:  3,      // Keep 3 backup files
			MaxAge:      7,      // Keep logs for 7 days
			Compress:    true,   // Compress rotated logs
			LogLevel:    "info", // Default to info level
		}),
	}

	// Initialize database
	db, err := sql.Open("sqlite", "users.db")
	if err != nil {
		app.logger.Error("Failed to open database", "error", err)
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	app.db = db

	// Create users table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			handle TEXT PRIMARY KEY,
			did TEXT,
			followers INTEGER,
			saved_on TIMESTAMP,
			followed BOOLEAN,
			last_checked TIMESTAMP,
			follow_date TIMESTAMP,
			priority INTEGER DEFAULT 1,
			attempts INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		app.logger.Error("Failed to create table", "error", err)
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	app.logger.Info("Application initialized", "debug_mode", app.logger.IsDebugMode())
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
		app.logger.Error("Failed to marshal login payload", "error", err)
		return nil, fmt.Errorf("failed to marshal login payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiBase+"/com.atproto.server.createSession", bytes.NewBuffer(jsonData))
	if err != nil {
		app.logger.Error("Failed to create login request", "error", err)
		return nil, fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.client.Do(req)
	if err != nil {
		app.logger.Error("Failed to execute login request", "error", err)
		return nil, fmt.Errorf("failed to execute login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		app.logger.Error("Login failed with status code: %d", resp.StatusCode)
		return nil, fmt.Errorf("login failed with status code: %d", resp.StatusCode)
	}

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		app.logger.Error("Failed to decode login response", "error", err)
		return nil, fmt.Errorf("failed to decode login response: %w", err)
	}

	session.CreatedAt = time.Now()
	app.logger.Info("Successfully logged in as: %s (DID: %s)", session.Handle, session.Did)
	return &session, nil
}

// getFollowerCount retrieves the follower count for a user
func (app *App) getFollowerCount(session *Session, actor string) (int, error) {
	app.logger.Debug("Getting follower count for actor: %s", actor)
	
	url := apiBase + "/app.bsky.actor.getProfile?actor=" + actor
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		app.logger.Error("Failed to create profile request", "error", err)
		return 0, fmt.Errorf("failed to create profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	
	resp, err := app.client.Do(req)
	if err != nil {
		app.logger.Error("Failed to fetch profile", "error", err)
		return 0, fmt.Errorf("failed to fetch profile: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		app.logger.Error("Profile fetch failed with status: %d", resp.StatusCode)
		return 0, fmt.Errorf("profile fetch failed with status: %d", resp.StatusCode)
	}
	
	var profile Profile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		app.logger.Error("Failed to decode profile response", "error", err)
		return 0, fmt.Errorf("failed to decode profile response: %w", err)
	}
	
	app.logger.Debug("Follower count for %s: %d", actor, profile.FollowersCount)
	return profile.FollowersCount, nil
}

// loadUsersFromDB loads users from the SQLite database
func (app *App) loadUsersFromDB() ([]TargetUser, error) {
	app.logger.Debug("Loading users from database")
	startTime := time.Now()
	
	rows, err := app.db.Query("SELECT handle, did, followers, saved_on, followed, last_checked, follow_date, priority, attempts FROM users")
	if err != nil {
		app.logger.Error("Failed to query users", "error", err)
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []TargetUser
	for rows.Next() {
		var user TargetUser
		var savedOn, lastChecked, followDate sql.NullString
		err := rows.Scan(&user.Handle, &user.DID, &user.Followers, &savedOn, &user.Followed, &lastChecked, &followDate, &user.Priority, &user.Attempts)
		if err != nil {
			app.logger.Error("Failed to scan user", "error", err)
			continue
		}

		// Parse timestamps
		if savedOn.Valid {
			user.SavedOn, _ = time.Parse(time.RFC3339, savedOn.String)
		}
		if lastChecked.Valid {
			user.LastChecked, _ = time.Parse(time.RFC3339, lastChecked.String)
		}
		if followDate.Valid {
			user.FollowDate, _ = time.Parse(time.RFC3339, followDate.String)
		}

		users = append(users, user)
		app.logger.Trace("Loaded user", "user", user)
	}

	if err = rows.Err(); err != nil {
		app.logger.Error("Error iterating rows", "error", err)
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	app.logger.Debug("Successfully loaded %d users from database in %v", len(users), time.Since(startTime))
	return users, nil
}

// saveUserToDB saves a user to the SQLite database
func (app *App) saveUserToDB(user TargetUser) error {
	app.logger.Debug("Saving user to database", "user", user)
	startTime := time.Now()
	
	// Use UPSERT to either insert or update the user
	result, err := app.db.Exec(`
		INSERT OR REPLACE INTO users (
			handle, did, followers, saved_on, followed, 
			last_checked, follow_date, priority, attempts
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.Handle, user.DID, user.Followers, user.SavedOn, user.Followed,
		user.LastChecked, user.FollowDate, user.Priority, user.Attempts)
	
	if err != nil {
		app.logger.Error("Failed to save user", "error", err)
		return fmt.Errorf("failed to save user: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	app.logger.Debug("Successfully saved user to database in %v (rows affected: %d)", time.Since(startTime), rowsAffected)
	app.logger.Audit("User saved/updated", "handle", user.Handle, "followers", user.Followers, "followed", user.Followed)
	return nil
}

// fetchTopHandlesFromBskyDirectory fetches top handles from multiple Bluesky sources
func (app *App) fetchTopHandlesFromBskyDirectory() ([]string, error) {
	app.logger.Info("Fetching top handles from multiple Bluesky sources")
	
	// First get a session to authenticate
	session, err := app.login()
	if err != nil {
		app.logger.Error("Failed to login", "error", err)
		return nil, fmt.Errorf("failed to login: %w", err)
	}

	// Use a map to deduplicate handles
	uniqueHandles := make(map[string]bool)
	var allHandles []string

	// Function to add handles to our collection
	addHandles := func(handles []string) {
		for _, handle := range handles {
			if !uniqueHandles[handle] {
				uniqueHandles[handle] = true
				allHandles = append(allHandles, handle)
			}
		}
	}

	// 1. Get trending users
	app.logger.Info("Fetching trending users")
	trendingURL := apiBase + "/app.bsky.unspecced.getPopular?limit=100"
	req, err := http.NewRequest("GET", trendingURL, nil)
	if err != nil {
		app.logger.Error("Failed to create trending request", "error", err)
	} else {
		req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
		resp, err := app.client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var result struct {
					Actors []struct {
						Handle string `json:"handle"`
					} `json:"actors"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
					var handles []string
					for _, actor := range result.Actors {
						handles = append(handles, actor.Handle)
					}
					addHandles(handles)
					app.logger.Info("Added %d trending users", len(handles))
				}
			}
		}
	}

	// 2. Get directory suggestions
	app.logger.Info("Fetching directory suggestions")
	dirURL := apiBase + "/app.bsky.actor.getSuggestions?limit=100"
	req, err = http.NewRequest("GET", dirURL, nil)
	if err != nil {
		app.logger.Error("Failed to create suggestions request", "error", err)
	} else {
		req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
		resp, err := app.client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var result struct {
					Actors []struct {
						Handle string `json:"handle"`
					} `json:"actors"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
					var handles []string
					for _, actor := range result.Actors {
						handles = append(handles, actor.Handle)
					}
					addHandles(handles)
					app.logger.Info("Added %d directory suggestions", len(handles))
				}
			}
		}
	}

	// 3. Get popular users by follower count
	app.logger.Info("Fetching popular users")
	popularURL := apiBase + "/app.bsky.actor.searchActors?limit=100&sort=followerCount"
	req, err = http.NewRequest("GET", popularURL, nil)
	if err != nil {
		app.logger.Error("Failed to create popular request", "error", err)
	} else {
		req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
		resp, err := app.client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var result struct {
					Actors []struct {
						Handle string `json:"handle"`
					} `json:"actors"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
					var handles []string
					for _, actor := range result.Actors {
						handles = append(handles, actor.Handle)
					}
					addHandles(handles)
					app.logger.Info("Added %d popular users", len(handles))
				}
			}
		}
	}

	if len(allHandles) == 0 {
		if len(app.config.FallbackHandles) == 0 {
			app.logger.Error("No handles found and no fallback handles configured")
			return nil, fmt.Errorf("no handles found and no fallback handles configured")
		}
		app.logger.Warn("No handles found from API, using configured fallback handles")
		return app.config.FallbackHandles, nil
	}

	app.logger.Info("Successfully fetched %d unique handles from multiple sources", len(allHandles))
	return allHandles, nil
}

// fetchAndSaveTopUsers fetches top users from bsky.directory and saves them to the database
func (app *App) fetchAndSaveTopUsers(simulate bool) error {
	app.logger.Info("Fetching top users from bsky.directory")
	
	handles, err := app.fetchTopHandlesFromBskyDirectory()
	if err != nil {
		return fmt.Errorf("failed to fetch top handles: %w", err)
	}

	session, err := app.login()
	if err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}

	now := time.Now()
	for _, handle := range handles {
		// Skip if we've already processed this handle recently
		var existingUser TargetUser
		err := app.db.QueryRow("SELECT * FROM users WHERE handle = ?", handle).Scan(
			&existingUser.Handle,
			&existingUser.DID,
			&existingUser.Followers,
			&existingUser.SavedOn,
			&existingUser.Followed,
			&existingUser.LastChecked,
			&existingUser.FollowDate,
			&existingUser.Priority,
			&existingUser.Attempts,
		)
		if err == nil && !existingUser.LastChecked.IsZero() && time.Since(existingUser.LastChecked) < 24*time.Hour {
			app.logger.Debug("Skipping recently checked user: %s", handle)
			continue
		}

		// Get user's DID
		did, err := app.getDID(session, handle)
		if err != nil {
			app.logger.Warn("Failed to get DID for %s: %v", handle, err)
			continue
		}

		// Get follower count
		followers, err := app.getFollowerCount(session, handle)
		if err != nil {
			app.logger.Warn("Failed to get follower count for %s: %v", handle, err)
			continue
		}

		// Calculate priority based on follower count
		priority := 1
		if followers > 10000 {
			priority = 3
		} else if followers > 1000 {
			priority = 2
		}

		user := TargetUser{
			Handle:      handle,
			DID:         did,
			Followers:   followers,
			SavedOn:     now,
			LastChecked: now,
			Priority:    priority,
		}

		// Save to database
		if err := app.saveUserToDB(user); err != nil {
			app.logger.Error("Failed to save user %s: %v", handle, err)
			continue
		}

		// Add to follow queue if not already followed
		if !user.Followed {
			app.addToFollowQueue(user, priority)
		}

		// Rate limiting
		time.Sleep(1 * time.Second)
	}

	return nil
}

// getDID retrieves the DID for a given handle
func (app *App) getDID(session *Session, handle string) (string, error) {
	app.logger.Debug("Getting DID for handle: %s", handle)
	
	url := apiBase + "/app.bsky.actor.getProfile?actor=" + handle
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		app.logger.Error("Failed to create profile request", "error", err)
		return "", fmt.Errorf("failed to create profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	
	resp, err := app.client.Do(req)
	if err != nil {
		app.logger.Error("Failed to fetch profile", "error", err)
		return "", fmt.Errorf("failed to fetch profile: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		app.logger.Error("Profile fetch failed with status: %d", resp.StatusCode)
		return "", fmt.Errorf("profile fetch failed with status: %d", resp.StatusCode)
	}
	
	var result struct {
		Did string `json:"did"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		app.logger.Error("Failed to decode profile response", "error", err)
		return "", fmt.Errorf("failed to decode profile response: %w", err)
	}
	
	app.logger.Debug("Got DID for %s: %s", handle, result.Did)
	return result.Did, nil
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
	var did string
	err := app.db.QueryRow(`
		SELECT followed, did 
		FROM users 
		WHERE handle = ? OR did = ? 
		LIMIT 1
	`, handleOrDid, handleOrDid).Scan(&followed, &did)
	
	if err == nil && followed {
		app.logger.Info("User %s is already marked as followed in database (skipped)", handleOrDid)
		app.followed[handleOrDid] = true
		return nil
	}

	if simulate {
		app.logger.Info("Simulation mode: would follow: %s", handleOrDid)
		return nil
	}

	// Get the DID if we only have a handle and don't already have it
	if !strings.HasPrefix(handleOrDid, "did:") && did == "" {
		var err error
		did, err = app.getDID(session, handleOrDid)
		if err != nil {
			app.logger.Error("Failed to get DID for %s: %v", handleOrDid, err)
			return fmt.Errorf("failed to get DID for %s: %w", handleOrDid, err)
		}
	} else if did == "" {
		did = handleOrDid
	}

	app.logger.Debug("Preparing follow request for %s (DID: %s)", handleOrDid, did)
	
	// Create the follow record with proper structure
	followRecord := map[string]interface{}{
		"$type": "app.bsky.graph.follow",
		"subject": did,
		"createdAt": time.Now().Format(time.RFC3339),
	}

	payload := map[string]interface{}{
		"collection": "app.bsky.graph.follow",
		"repo":       session.Did,
		"record":     followRecord,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		app.logger.Error("Failed to marshal follow request", "error", err)
		return fmt.Errorf("failed to marshal follow request: %w", err)
	}

	req, err := http.NewRequest("POST", apiBase+"/com.atproto.repo.createRecord", bytes.NewBuffer(jsonBody))
	if err != nil {
		app.logger.Error("Failed to create follow request", "error", err)
		return fmt.Errorf("failed to create follow request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	req.Header.Set("Content-Type", "application/json")

	app.logger.Debug("Sending follow request for %s (DID: %s)", handleOrDid, did)
	resp, err := app.client.Do(req)
	if err != nil {
		app.logger.Error("Failed to execute follow request", "error", err)
		return fmt.Errorf("failed to execute follow request: %w", err)
	}
	defer resp.Body.Close()

	// Read and log the response body for debugging
	body, _ := io.ReadAll(resp.Body)
	app.logger.Debug("Follow response for %s: Status=%d, Body=%s", handleOrDid, resp.StatusCode, string(body))

	if resp.StatusCode == http.StatusOK {
		app.logger.Info("Successfully followed: %s (took %v)", handleOrDid, time.Since(startTime))
		app.followed[handleOrDid] = true
		
		// Update the user's followed status in the database
		_, err := app.db.Exec(`
			UPDATE users 
			SET followed = 1, did = ? 
			WHERE handle = ? OR did = ?
		`, did, handleOrDid, handleOrDid)
		if err != nil {
			app.logger.Error("Failed to update user's followed status", "error", err)
			return fmt.Errorf("failed to update user's followed status: %w", err)
		}
		app.logger.Audit("User followed", "handle", handleOrDid, "DID", did)
	} else if strings.Contains(string(body), "already following") {
		app.logger.Info("Already following: %s (took %v)", handleOrDid, time.Since(startTime))
		app.followed[handleOrDid] = true
		
		// Update the database to reflect we're already following
		_, err := app.db.Exec(`
			UPDATE users 
			SET followed = 1, did = ? 
			WHERE handle = ? OR did = ?
		`, did, handleOrDid, handleOrDid)
		if err != nil {
			app.logger.Error("Failed to update user's followed status", "error", err)
		}
		return nil
	} else {
		app.logger.Error("Failed to follow %s. Status: %d, Response: %s (took %v)", handleOrDid, resp.StatusCode, string(body), time.Since(startTime))
		return fmt.Errorf("failed to follow %s. Status: %d, Response: %s", handleOrDid, resp.StatusCode, string(body))
	}
	return nil
}

// Heap interface implementation for FollowQueue
func (pq FollowQueue) Len() int { return len(pq) }

func (pq FollowQueue) Less(i, j int) bool {
	return pq[i].NextTry.Before(pq[j].NextTry)
}

func (pq FollowQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *FollowQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*FollowQueueItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *FollowQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

// update updates the priority and value of an Item in the queue.
func (pq *FollowQueue) update(item *FollowQueueItem, priority int, nextTry time.Time) {
	item.Priority = priority
	item.NextTry = nextTry
	heap.Fix(pq, item.index)
}

// Queue management functions
func (app *App) addToFollowQueue(user TargetUser, priority int) {
	app.mu.Lock()
	defer app.mu.Unlock()

	item := &FollowQueueItem{
		User:     user,
		Priority: priority,
		Attempts: 0,
		NextTry:  time.Now(),
	}
	heap.Push(app.queue, item)
	app.logger.Audit("Added user %s to follow queue with priority %d", user.Handle, priority)
}

func (app *App) processFollowQueue(session *Session) {
	app.mu.Lock()
	defer app.mu.Unlock()

	now := time.Now()
	
	// Reset follow count if an hour has passed
	if now.After(app.followReset) {
		app.followCount = 0
		app.followReset = now.Add(time.Hour)
	}

	// Check if we've hit the rate limit
	if app.followCount >= maxFollowsPerHour {
		app.logger.Warn("Rate limit reached, waiting for reset")
		return
	}

	// Process queue items that are ready
	for app.queue.Len() > 0 {
		item := (*app.queue)[0]
		if item.NextTry.After(now) {
			break
		}

		// Remove from queue
		heap.Pop(app.queue)

		// Attempt to follow
		err := app.followUser(session, item.User.DID, false)
		if err != nil {
			item.Attempts++
			if item.Attempts < maxRetries {
				// Add back to queue with exponential backoff
				item.NextTry = now.Add(retryDelay * time.Duration(item.Attempts))
				heap.Push(app.queue, item)
				app.logger.Warn("Failed to follow %s, retrying in %v", item.User.Handle, item.NextTry.Sub(now))
			} else {
				app.logger.Error("Failed to follow %s after %d attempts", item.User.Handle, maxRetries)
			}
			continue
		}

		// Update follow tracking
		app.followCount++
		app.lastFollow = now
		app.logger.Audit("Successfully followed %s", item.User.Handle)

		// Update database
		item.User.Followed = true
		if err := app.saveUserToDB(item.User); err != nil {
			app.logger.Error("Failed to update database for %s: %v", item.User.Handle, err)
		}
	}
}

func main() {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize application
	app, err := NewApp(config)
	if err != nil {
		fmt.Printf("Error initializing application: %v\n", err)
		os.Exit(1)
	}
	defer app.db.Close()

	var session *Session
	var isAuthenticated bool

	for {
		fmt.Println("\nBlueSky Follower Menu:")
		fmt.Println("1. Authenticate to BlueSky")
		fmt.Println("2. Fetch and Save Top Users")
		fmt.Println("3. Process Follow Queue")
		fmt.Println("4. Exit")

		if isAuthenticated {
			fmt.Println("\nCurrently authenticated as:", session.Handle)
			fmt.Println("5. Logout from BlueSky")
		}

		fmt.Print("\nSelect an option: ")
		var choice string
		fmt.Scanln(&choice)

		switch choice {
		case "1":
			if isAuthenticated {
				fmt.Println("Already authenticated as:", session.Handle)
				continue
			}
			session, err = app.login()
			if err != nil {
				fmt.Printf("Authentication failed: %v\n", err)
				continue
			}
			isAuthenticated = true
			fmt.Printf("Successfully authenticated as: %s\n", session.Handle)

		case "2":
			if !isAuthenticated {
				fmt.Println("Please authenticate first")
				continue
			}
			fmt.Println("Fetching and saving top users...")
			err = app.fetchAndSaveTopUsers(false)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Println("Successfully fetched and saved top users")
			}

		case "3":
			if !isAuthenticated {
				fmt.Println("Please authenticate first")
				continue
			}
			fmt.Println("Processing follow queue...")
			app.processFollowQueue(session)

		case "4":
			fmt.Println("Goodbye!")
			return

		case "5":
			if !isAuthenticated {
				fmt.Println("Not authenticated")
				continue
			}
			session = nil
			isAuthenticated = false
			fmt.Println("Successfully logged out")

		default:
			fmt.Println("Invalid option, please try again")
		}
	}
}
