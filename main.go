package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

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

// App represents the application state
type App struct {
	config   *Config
	client   *http.Client
	followed map[string]bool
}

// NewApp creates a new application instance
func NewApp(config *Config) *App {
	return &App{
		config:   config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		followed: make(map[string]bool),
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
	body := map[string]string{
		"identifier": app.config.Identifier,
		"password":   app.config.Password,
	}
	
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal login request: %w", err)
	}
	
	req, err := http.NewRequest("POST", apiBase+"/com.atproto.server.createSession", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := app.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute login request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed with status: %d", resp.StatusCode)
	}
	
	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("failed to decode login response: %w", err)
	}
	
	return &session, nil
}

// getFollowerCount retrieves the follower count for a user
func (app *App) getFollowerCount(actor string) (int, error) {
	url := apiBase + "/app.bsky.actor.getProfile?actor=" + actor
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create profile request: %w", err)
	}
	
	resp, err := app.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch profile: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("profile fetch failed with status: %d", resp.StatusCode)
	}
	
	var profile Profile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return 0, fmt.Errorf("failed to decode profile response: %w", err)
	}
	
	return profile.FollowersCount, nil
}

// followUser follows a user on Bluesky
func (app *App) followUser(session *Session, handleOrDid string, simulate bool) error {
	if app.followed[handleOrDid] {
		fmt.Printf("Already followed %s (skipped)\n", handleOrDid)
		return nil
	}

	if simulate {
		fmt.Printf("[SIMULATION] Would follow: %s\n", handleOrDid)
		return nil
	}

	payload := map[string]interface{}{
		"collection": "app.bsky.graph.follow",
		"repo":       session.Did,
		"record":     FollowRecord{Subject: handleOrDid},
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal follow request: %w", err)
	}

	req, err := http.NewRequest("POST", apiBase+"/com.atproto.repo.createRecord", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create follow request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute follow request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Println("✅ Followed:", handleOrDid)
		app.followed[handleOrDid] = true
	} else if resp.StatusCode == http.StatusBadRequest {
		fmt.Printf("ℹ️ Already following: %s\n", handleOrDid)
		app.followed[handleOrDid] = true
		return nil
	} else {
		return fmt.Errorf("failed to follow %s. Status: %d", handleOrDid, resp.StatusCode)
	}
	return nil
}

// loadUsersFromJSON loads users from a JSON file
func (app *App) loadUsersFromJSON(filePath string) ([]TargetUser, error) {
	var users []TargetUser
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []TargetUser{}, nil
		}
		return nil, fmt.Errorf("failed to read users file: %w", err)
	}
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, fmt.Errorf("failed to unmarshal users: %w", err)
	}
	return users, nil
}

// saveUserToJSON saves users to a JSON file
func (app *App) saveUserToJSON(newUser TargetUser, filePath string) error {
	users, err := app.loadUsersFromJSON(filePath)
	if err != nil {
		return fmt.Errorf("failed to load existing users: %w", err)
	}
	
	// Check if user exists and update it, otherwise append
	found := false
	for i, u := range users {
		if u.Handle == newUser.Handle || u.DID == newUser.DID {
			users[i] = newUser
			found = true
			break
		}
	}
	
	if !found {
		users = append(users, newUser)
	}
	
	sort.Slice(users, func(i, j int) bool {
		return users[i].Followers > users[j].Followers
	})
	
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal users: %w", err)
	}
	
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write users file: %w", err)
	}
	
	return nil
}

// fetchTopHandlesFromBskyDirectory fetches top handles from Bluesky directory
func (app *App) fetchTopHandlesFromBskyDirectory() ([]string, error) {
	url := apiBase + "/app.bsky.actor.getSuggestions?limit=50"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create suggestions request: %w", err)
	}

	resp, err := app.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Bluesky API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("suggestions fetch failed with status: %d", resp.StatusCode)
	}

	var result struct {
		Actors []struct {
			Handle string `json:"handle"`
		} `json:"actors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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
			return nil, fmt.Errorf("no handles found and no fallback handles configured")
		}
		log.Printf("No handles found from API, using configured fallback handles")
		return app.config.FallbackHandles, nil
	}

	return handles, nil
}

// fetchAndSaveTopUsers fetches and saves top users
func (app *App) fetchAndSaveTopUsers(filePath string, simulate bool) error {
	session, err := app.login()
	if err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}

	handles, err := app.fetchTopHandlesFromBskyDirectory()
	if err != nil {
		return fmt.Errorf("failed to fetch handles: %w", err)
	}

	// Load existing users first
	existingUsers, err := app.loadUsersFromJSON(filePath)
	if err != nil {
		return fmt.Errorf("failed to load existing users: %w", err)
	}
	
	existingUsersMap := make(map[string]TargetUser)
	for _, user := range existingUsers {
		existingUsersMap[user.Handle] = user
	}

	// Collect all users
	var allUsers []TargetUser
	for _, handle := range handles {
		time.Sleep(1 * time.Second) // Rate limiting

		ctx, cancel := context.WithTimeout(context.Background(), app.config.Timeout)
		defer cancel()

		url := apiBase + "/app.bsky.actor.getProfile?actor=" + handle
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			log.Printf("Error creating request for %s: %v", handle, err)
			continue
		}
		req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

		resp, err := app.client.Do(req)
		if err != nil {
			log.Printf("Error fetching profile for %s: %v", handle, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Error fetching profile for %s: status %d", handle, resp.StatusCode)
			continue
		}

		var profile Profile
		if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
			log.Printf("Error decoding profile for %s: %v", handle, err)
			continue
		}

		user := TargetUser{
			Handle:    handle,
			Followers: profile.FollowersCount,
			SavedOn:   time.Now().UTC().Format(time.RFC3339),
		}

		allUsers = append(allUsers, user)
	}

	// Save all users
	for _, user := range allUsers {
		if err := app.saveUserToJSON(user, filePath); err != nil {
			log.Printf("Error saving user %s: %v", user.Handle, err)
		}
	}

	return nil
}

func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	app := NewApp(config)

	// Parse command line flags
	var (
		simulate      bool
		filePath      string
		updateTop     bool
		minFollowers  string
		realFollow    bool
	)

	flag.BoolVar(&simulate, "simulate", false, "Run in simulation mode (no actual follows)")
	flag.StringVar(&filePath, "file", "users.json", "Path to the users JSON file")
	flag.BoolVar(&updateTop, "update-top", false, "Fetch top users from bsky.directory and save to JSON")
	flag.StringVar(&minFollowers, "min-followers", "0", "Minimum followers required to follow")
	flag.BoolVar(&realFollow, "real", false, "Actually follow users (default is simulation only)")
	flag.Parse()

	if updateTop {
		if err := app.fetchAndSaveTopUsers(filePath, simulate); err != nil {
			log.Fatalf("Failed to fetch and save top users: %v", err)
		}
		return
	}

	// Load users from JSON
	users, err := app.loadUsersFromJSON(filePath)
	if err != nil {
		log.Fatalf("Failed to load users: %v", err)
	}

	// Parse minimum followers
	minFollowersCount, err := strconv.Atoi(minFollowers)
	if err != nil {
		log.Fatalf("Invalid minimum followers value: %v", err)
	}

	// Login to get session
	session, err := app.login()
	if err != nil {
		log.Fatalf("Failed to login: %v", err)
	}

	// Process each user
	for _, user := range users {
		time.Sleep(3 * time.Second) // Rate limiting

		actor := user.Handle
		if actor == "" {
			actor = user.DID
		}
		if actor == "" {
			log.Println("⚠️ Skipping user with no handle or DID")
			continue
		}

		// Update follower count if needed
		count := user.Followers
		if count == 0 {
			count, err = app.getFollowerCount(actor)
			if err != nil {
				log.Printf("Skipping %s (error getting follower count: %v)", actor, err)
				continue
			}
			user.Followers = count
			user.SavedOn = time.Now().UTC().Format(time.RFC3339)
			if err := app.saveUserToJSON(user, filePath); err != nil {
				log.Printf("Failed to save updated user %s: %v", actor, err)
			}
		}

		// Skip if below minimum followers
		if count < minFollowersCount {
			fmt.Printf("⏭️  Skipping %s (%d < min %d)\n", actor, count, minFollowersCount)
			continue
		}

		// Follow user
		if err := app.followUser(session, actor, !realFollow); err != nil {
			log.Printf("Follow error: %v", err)
		}
	}
}
