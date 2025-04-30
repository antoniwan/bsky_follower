package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
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

type TargetUser struct {
    Handle string `json:"handle"`
    DID    string `json:"did"`
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
    } else {
        fmt.Printf("❌ Failed to follow %s. Status: %d\n", handleOrDid, resp.StatusCode)
    }
    return nil
}

func loadUsersFromJSON(filePath string) ([]TargetUser, error) {
    data, err := ioutil.ReadFile(filePath)
    if err != nil {
        return nil, err
    }
    var users []TargetUser
    if err := json.Unmarshal(data, &users); err != nil {
        return nil, err
    }
    return users, nil
}

func main() {
    var identifier, password, jsonFile, minFollowersFlag string
    var realFollow bool

    flag.StringVar(&identifier, "id", "", "Bluesky handle or email")
    flag.StringVar(&password, "pw", "", "App password")
    flag.StringVar(&jsonFile, "json", "users.json", "Path to JSON file with users to follow")
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

    users, err := loadUsersFromJSON(jsonFile)
    if err != nil {
        log.Fatalf("Error loading users from %s: %v", jsonFile, err)
    }

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

    for _, user := range users {
        time.Sleep(4 * time.Second)

        actor := user.Handle
        if actor == "" {
            actor = user.DID
        }
        if actor == "" {
            log.Println("⚠️ Skipping user with no handle or DID")
            continue
        }

        followerCount, err := getFollowerCount(actor)
        if err != nil {
            log.Printf("Skipping %s (error getting follower count: %v)", actor, err)
            continue
        }

        if followerCount < minFollowers {
            fmt.Printf("⏭️  Skipping %s (has %d followers, needs at least %d)\n", actor, followerCount, minFollowers)
            continue
        }

        if err := followUser(session, actor, !realFollow); err != nil {
            log.Printf("Follow error: %v", err)
        }
    }
}
