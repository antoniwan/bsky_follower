package queue

import (
	"container/heap"
	"time"

	"bsky_follower/internal/models"
)

// Queue represents a priority queue for follow operations
type Queue struct {
	items models.FollowQueue
}

// NewQueue creates a new follow queue
func NewQueue() *Queue {
	pq := make(models.FollowQueue, 0)
	heap.Init(&pq)
	return &Queue{
		items: pq,
	}
}

// Push adds a new item to the queue
func (q *Queue) Push(user models.TargetUser, priority int) {
	item := &models.FollowQueueItem{
		User:     user,
		Priority: priority,
		Attempts: user.Attempts,
		NextTry:  time.Now(),
	}
	heap.Push(&q.items, item)
}

// Pop removes and returns the highest priority item
func (q *Queue) Pop() *models.FollowQueueItem {
	if q.items.Len() == 0 {
		return nil
	}
	return heap.Pop(&q.items).(*models.FollowQueueItem)
}

// Update modifies the priority and next try time of an item
func (q *Queue) Update(item *models.FollowQueueItem, priority int, nextTry time.Time) {
	item.Priority = priority
	item.NextTry = nextTry
	heap.Fix(&q.items, item.Index)
}

// Len returns the number of items in the queue
func (q *Queue) Len() int {
	return q.items.Len()
}

// Peek returns the highest priority item without removing it
func (q *Queue) Peek() *models.FollowQueueItem {
	if q.items.Len() == 0 {
		return nil
	}
	return q.items[0]
} 