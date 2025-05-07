package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bsky_follower/internal/models"
)

const apiBase = "https://bsky.social/xrpc"

// Client represents a Bluesky API client
type Client struct {
	httpClient *http.Client
	logger     Logger
}

// Logger interface for logging
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewClient creates a new Bluesky API client
func NewClient(timeout time.Duration, logger Logger) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}
}

// Login authenticates with the Bluesky API
func (c *Client) Login(identifier, password string) (*models.Session, error) {
	c.logger.Info("Attempting to login with identifier: %s", identifier)
	
	payload := map[string]string{
		"identifier": identifier,
		"password":   password,
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		c.logger.Error("Failed to marshal login payload", "error", err)
		return nil, fmt.Errorf("failed to marshal login payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiBase+"/com.atproto.server.createSession", bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Error("Failed to create login request", "error", err)
		return nil, fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to execute login request", "error", err)
		return nil, fmt.Errorf("failed to execute login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Login failed with status code: %d", resp.StatusCode)
		return nil, fmt.Errorf("login failed with status code: %d", resp.StatusCode)
	}

	var session models.Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		c.logger.Error("Failed to decode login response", "error", err)
		return nil, fmt.Errorf("failed to decode login response: %w", err)
	}

	session.CreatedAt = time.Now()
	c.logger.Info("Successfully logged in as: %s (DID: %s)", session.Handle, session.Did)
	return &session, nil
}

// GetFollowerCount retrieves the follower count for a user
func (c *Client) GetFollowerCount(session *models.Session, actor string) (int, error) {
	c.logger.Debug("Getting follower count for actor: %s", actor)
	
	url := apiBase + "/app.bsky.actor.getProfile?actor=" + actor
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.logger.Error("Failed to create profile request", "error", err)
		return 0, fmt.Errorf("failed to create profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to fetch profile", "error", err)
		return 0, fmt.Errorf("failed to fetch profile: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Profile fetch failed with status: %d", resp.StatusCode)
		return 0, fmt.Errorf("profile fetch failed with status: %d", resp.StatusCode)
	}

	var profile models.Profile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		c.logger.Error("Failed to decode profile response", "error", err)
		return 0, fmt.Errorf("failed to decode profile response: %w", err)
	}

	return profile.FollowersCount, nil
}

// GetDID retrieves the DID for a handle
func (c *Client) GetDID(session *models.Session, handle string) (string, error) {
	c.logger.Debug("Getting DID for handle: %s", handle)
	
	url := apiBase + "/com.atproto.identity.resolveHandle?handle=" + handle
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.logger.Error("Failed to create resolve handle request", "error", err)
		return "", fmt.Errorf("failed to create resolve handle request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to resolve handle", "error", err)
		return "", fmt.Errorf("failed to resolve handle: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Handle resolution failed with status: %d", resp.StatusCode)
		return "", fmt.Errorf("handle resolution failed with status: %d", resp.StatusCode)
	}

	var result struct {
		Did string `json:"did"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.logger.Error("Failed to decode handle resolution response", "error", err)
		return "", fmt.Errorf("failed to decode handle resolution response: %w", err)
	}

	return result.Did, nil
}

// FollowUser follows a user on Bluesky
func (c *Client) FollowUser(session *models.Session, handleOrDid string, simulate bool) error {
	if simulate {
		c.logger.Info("Simulating follow for: %s", handleOrDid)
		return nil
	}

	c.logger.Info("Following user: %s", handleOrDid)
	
	payload := map[string]interface{}{
		"collection": "app.bsky.graph.follow",
		"repo":       session.Did,
		"record": models.FollowRecord{
			Subject: handleOrDid,
		},
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		c.logger.Error("Failed to marshal follow payload", "error", err)
		return fmt.Errorf("failed to marshal follow payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiBase+"/com.atproto.repo.createRecord", bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Error("Failed to create follow request", "error", err)
		return fmt.Errorf("failed to create follow request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to execute follow request", "error", err)
		return fmt.Errorf("failed to execute follow request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Follow failed with status: %d", resp.StatusCode)
		return fmt.Errorf("follow failed with status: %d", resp.StatusCode)
	}

	c.logger.Info("Successfully followed user: %s", handleOrDid)
	return nil
} 