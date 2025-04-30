package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
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

func getMyFollowerCount(session *Session) (int, error) {
	url := apiBase + "/app.bsky.actor.getProfile?actor=" + session.Handle
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

	client := &http.Client{}
	resp, err := client.Do(req)
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

func getFollowerCount(did string) (int, error) {
	url := apiBase + "/app.bsky.actor.getProfile?actor=" + did
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

func followUser(session *Session, handle string, simulate bool) error {
	if followed[handle] {
		fmt.Printf("Already followed %s (skipped)\n", handle)
		return nil
	}

	if simulate {
		fmt.Printf("[SIMULATION] Would follow: %s\n", handle)
		return nil
	}

	payload := map[string]interface{}{
		"collection": "app.bsky.graph.follow",
		"repo":       session.Did,
		"record":     FollowRecord{Subject: handle},
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
		fmt.Println("✅ Followed:", handle)
		followed[handle] = true
	} else {
		fmt.Printf("❌ Failed to follow %s. Status: %d\n", handle, resp.StatusCode)
	}
	return nil
}

func main() {
	var identifier, password, topic, minFollowersFlag string
	var realFollow bool

	flag.StringVar(&identifier, "id", "", "Bluesky handle or email")
	flag.StringVar(&password, "pw", "", "App password")
	flag.StringVar(&topic, "topic", "tech", "Topic to follow (tech, art, music)")
	flag.StringVar(&minFollowersFlag, "min-followers", "0", "Minimum followers (or 'my' to follow only users with more followers than you)")
	flag.BoolVar(&realFollow, "real", false, "Actually follow users (default is simulation only)")
	flag.Parse()

	if identifier == "" || password == "" {
		log.Fatal("Must pass --id and --pw")
	}

	session, err := login(identifier, password)
	if err != nil {
		log.Fatal("Login failed:", err)
	}

	// Optional: Load your own follower count if needed
	var minFollowers int
	if minFollowersFlag == "my" {
		minFollowers, err = getMyFollowerCount(session)
		if err != nil {
			log.Fatal("Couldn't fetch your follower count:", err)
		}
		fmt.Printf("📊 You have %d followers\n", minFollowers)
	} else {
		minFollowers, err = strconv.Atoi(minFollowersFlag)
		if err != nil {
			log.Fatalf("Invalid --min-followers value: %v", err)
		}
	}

	topics := map[string][]string{
		"tech":   {"did:plc:4o5xk64qu6oaykni5ayh7usg", "did:plc:7ybsnib3zp5hnpejp4prtyqh"},
		"art":    {"did:plc:xnp7kqp3y3jxul5h3dijv2f2"},
		"music":  {"did:plc:zwrxyjx43fymf3jifwlqkbym"},
	}

	handles, ok := topics[topic]
	if !ok {
		log.Fatalf("Unknown topic: %s", topic)
	}

	for _, handle := range handles {
		time.Sleep(4 * time.Second)

		followerCount, err := getFollowerCount(handle)
		if err != nil {
			log.Printf("Skipping %s (error getting follower count: %v)", handle, err)
			continue
		}

		if followerCount < minFollowers {
			fmt.Printf("⏭️  Skipping %s (has %d followers, needs at least %d)\n", handle, followerCount, minFollowers)
			continue
		}

		if err := followUser(session, handle, !realFollow); err != nil {
			log.Printf("Follow error: %v", err)
		}
	}
}
