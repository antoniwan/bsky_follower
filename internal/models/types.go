package models

import "time"

// Config holds application configuration
type Config struct {
	Identifier       string
	Password         string
	Timeout          time.Duration
	FallbackHandles  []string
}

// Session represents an authenticated Bluesky session
type Session struct {
	AccessJwt string    `json:"accessJwt"`
	Did       string    `json:"did"`
	Handle    string    `json:"handle"`
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
	Index     int // for heap implementation
}

// FollowQueue implements heap.Interface for priority queue
type FollowQueue []*FollowQueueItem

// Len returns the length of the queue
func (pq FollowQueue) Len() int { return len(pq) }

// Less compares two items in the queue
func (pq FollowQueue) Less(i, j int) bool {
	// First compare by priority (higher priority first)
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	// Then compare by next try time (earlier time first)
	return pq[i].NextTry.Before(pq[j].NextTry)
}

// Swap swaps two items in the queue
func (pq FollowQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

// Push adds an item to the queue
func (pq *FollowQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*FollowQueueItem)
	item.Index = n
	*pq = append(*pq, item)
}

// Pop removes and returns the highest priority item
func (pq *FollowQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.Index = -1 // for safety
	*pq = old[0 : n-1]
	return item
} 