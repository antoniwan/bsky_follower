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
	for _, u := range users {
		if u.Handle == newUser.Handle || u.DID == newUser.DID {
			return nil
		}
	}
	users = append(users, newUser)
	sort.Slice(users, func(i, j int) bool {
		return users[i].Followers > users[j].Followers
	})
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, data, 0644)
}

func fetchTopHandlesFromBskyDirectory() []string {
	return []string{
		"jay.bsky.social",
		"dwr.bsky.social",
		"mikko.bsky.social",
		"maggieappleton.bsky.social",
		"theblaze.bsky.social",
	}
}

func fetchAndSaveTopUsers(filePath string) error {
	handles := fetchTopHandlesFromBskyDirectory()
	for _, handle := range handles {
		url := apiBase + "/app.bsky.actor.getProfile?actor=" + handle
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("Error fetching profile for %s: %v", handle, err)
			continue
		}
		defer resp.Body.Close()

		var profile struct {
			DID            string `json:"did"`
			FollowersCount int    `json:"followersCount"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
			log.Printf("Decode error for %s: %v", handle, err)
			continue
		}

		user := TargetUser{
			Handle:    handle,
			DID:       profile.DID,
			Followers: profile.FollowersCount,
			SavedOn:   time.Now().UTC().Format(time.RFC3339),
		}

		if err := saveUserToJSON(user, filePath); err != nil {
			log.Printf("Error saving user %s: %v", handle, err)
		} else {
			log.Printf("Saved: %s (%d followers)", handle, profile.FollowersCount)
		}
	}
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
