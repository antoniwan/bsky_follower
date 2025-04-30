package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"
)

const apiBase = "https://bsky.social/xrpc"

type Session struct {
	AccessJwt string `json:"accessJwt"`
	Did       string `json:"did"`
	Handle    string `json:"handle"`
}

type Profile struct {
	FollowersCount int `json:"followersCount"`
}

type FollowRecord struct {
	Subject string `json:"subject"`
}

type TargetUser struct {
	Handle    string `json:"handle"`
	DID       string `json:"did"`
	Followers int    `json:"followers"`
	SavedOn   string `json:"savedOn"`
}

var followed = map[string]bool{}

func login(identifier, password string) (*Session, error) {
	body := map[string]string{
		"identifier": identifier,
		"password":   password,
	}
	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post(apiBase+"/com.atproto.server.createSession", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func getFollowerCount(actor string) (int, error) {
	url := apiBase + "/app.bsky.actor.getProfile?actor=" + actor
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var profile Profile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return 0, err
	}
	return profile.FollowersCount, nil
}

func followUser(session *Session, handleOrDid string, simulate bool) error {
	if followed[handleOrDid] {
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

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", apiBase+"/com.atproto.repo.createRecord", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Println("✅ Followed:", handleOrDid)
		followed[handleOrDid] = true
	} else if resp.StatusCode == 400 {
		fmt.Printf("ℹ️ Already following: %s\n", handleOrDid)
		followed[handleOrDid] = true
		return nil
	} else {
		fmt.Printf("❌ Failed to follow %s. Status: %d\n", handleOrDid, resp.StatusCode)
	}
	return nil
}

func loadUsersFromJSON(filePath string) ([]TargetUser, error) {
	var users []TargetUser
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []TargetUser{}, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func saveUserToJSON(newUser TargetUser, filePath string) error {
	users, _ := loadUsersFromJSON(filePath)
	
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
		return err
	}
	return ioutil.WriteFile(filePath, data, 0644)
}

func fetchTopHandlesFromBskyDirectory() ([]string, error) {
	url := apiBase + "/app.bsky.actor.getSuggestions?limit=50"
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Bluesky API: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Actors []struct {
			Handle string `json:"handle"`
		} `json:"actors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
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
		// Fallback to team members if no suggestions found
		teamHandles := []string{
			"jay.bsky.social",
			"dwr.bsky.social",
			"mikko.bsky.social",
			"maggieappleton.bsky.social",
			"theblaze.bsky.social",
		}
		return teamHandles, nil
	}

	return handles, nil
}

func fetchAndSaveTopUsers(filePath string) error {
	// First, we need to authenticate
	identifier := os.Getenv("BSKY_IDENTIFIER")
	password := os.Getenv("BSKY_PASSWORD")
	if identifier == "" || password == "" {
		return fmt.Errorf("BSKY_IDENTIFIER and BSKY_PASSWORD environment variables must be set")
	}

	session, err := login(identifier, password)
	if err != nil {
		return fmt.Errorf("failed to login: %v", err)
	}

	handles, err := fetchTopHandlesFromBskyDirectory()
	if err != nil {
		return fmt.Errorf("failed to fetch handles: %v", err)
	}

	// Load existing users first
	existingUsers, _ := loadUsersFromJSON(filePath)
	existingUsersMap := make(map[string]TargetUser)
	for _, user := range existingUsers {
		existingUsersMap[user.Handle] = user
	}

	// Collect all users
	var allUsers []TargetUser
	for _, handle := range handles {
		time.Sleep(1 * time.Second) // Rate limiting

		url := apiBase + "/app.bsky.actor.getProfile?actor=" + handle
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Printf("Error creating request for %s: %v", handle, err)
			continue
		}
		req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Error fetching profile for %s: %v", handle, err)
			continue
		}
		defer resp.Body.Close()

		// Read the raw response body for debugging
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading response body for %s: %v", handle, err)
			continue
		}
		log.Printf("Raw response for %s: %s", handle, string(body))

		// Create a new reader from the body for decoding
		reader := bytes.NewReader(body)
		var profile struct {
			DID            string `json:"did"`
			FollowersCount int    `json:"followersCount"`
			Handle         string `json:"handle"`
		}
		if err := json.NewDecoder(reader).Decode(&profile); err != nil {
			log.Printf("Decode error for %s: %v", handle, err)
			continue
		}

		user := TargetUser{
			Handle:    profile.Handle,
			DID:       profile.DID,
			Followers: profile.FollowersCount,
			SavedOn:   time.Now().UTC().Format(time.RFC3339),
		}
		allUsers = append(allUsers, user)
		log.Printf("Collected: %s (%d followers)", handle, profile.FollowersCount)
	}

	// Merge with existing users
	for _, existingUser := range existingUsers {
		if _, exists := existingUsersMap[existingUser.Handle]; exists {
			// Only keep existing users that weren't updated in this run
			keep := true
			for _, newUser := range allUsers {
				if newUser.Handle == existingUser.Handle {
					keep = false
					break
				}
			}
			if keep {
				allUsers = append(allUsers, existingUser)
			}
		}
	}

	// Sort all users by follower count
	sort.Slice(allUsers, func(i, j int) bool {
		return allUsers[i].Followers > allUsers[j].Followers
	})

	// Save all users at once
	data, err := json.MarshalIndent(allUsers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal users: %v", err)
	}
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write users file: %v", err)
	}

	log.Printf("Successfully saved %d users to %s", len(allUsers), filePath)
	return nil
}

func main() {
	var identifier, password, jsonFile, minFollowersFlag string
	var realFollow, pullTopUsers bool

	flag.StringVar(&identifier, "id", "", "Bluesky handle or email")
	flag.StringVar(&password, "pw", "", "App password")
	flag.StringVar(&jsonFile, "json", "users.json", "Path to JSON file with users to follow")
	flag.StringVar(&minFollowersFlag, "min-followers", "0", "Minimum followers")
	flag.BoolVar(&realFollow, "real", false, "Actually follow users (default is simulation only)")
	flag.BoolVar(&pullTopUsers, "update-top", false, "Fetch top users from bsky.directory and save to JSON")
	flag.Parse()

	if pullTopUsers {
		if err := fetchAndSaveTopUsers(jsonFile); err != nil {
			log.Fatalf("Failed to fetch top users: %v", err)
		}
		return
	}

	if identifier == "" || password == "" {
		log.Fatal("Must pass --id and --pw")
	}

	session, err := login(identifier, password)
	if err != nil {
		log.Fatal("Login failed:", err)
	}

	users, err := loadUsersFromJSON(jsonFile)
	if err != nil {
		log.Fatalf("Error loading users: %v", err)
	}

	minFollowers, err := strconv.Atoi(minFollowersFlag)
	if err != nil {
		log.Fatalf("Invalid --min-followers value: %v", err)
	}

	for _, user := range users {
		time.Sleep(3 * time.Second)

		actor := user.Handle
		if actor == "" {
			actor = user.DID
		}
		if actor == "" {
			log.Println("⚠️ Skipping user with no handle or DID")
			continue
		}

		count := user.Followers
		if count == 0 {
			count, err = getFollowerCount(actor)
			if err != nil {
				log.Printf("Skipping %s (error getting follower count: %v)", actor, err)
				continue
			}
			user.Followers = count
			user.SavedOn = time.Now().UTC().Format(time.RFC3339)
			saveUserToJSON(user, jsonFile)
		}

		if count < minFollowers {
			fmt.Printf("⏭️  Skipping %s (%d < min %d)\n", actor, count, minFollowers)
			continue
		}

		if err := followUser(session, actor, !realFollow); err != nil {
			log.Printf("Follow error: %v", err)
		}
	}
}
