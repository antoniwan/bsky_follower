package ui

import (
	"container/heap"
	"time"

	"bsky_follower/internal/api"
	"bsky_follower/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

// QueueMsg represents a message about queue processing
type QueueMsg struct {
	Message string
	Error   error
}

// QueueCmd represents a command to process the follow queue
func QueueCmd(client *api.Client, session *models.Session, queue *models.FollowQueue) tea.Cmd {
	return func() tea.Msg {
		if queue.Len() == 0 {
			return QueueMsg{
				Message: "Queue is empty",
			}
		}

		// Get the highest priority item
		item := heap.Pop(queue).(*models.FollowQueueItem)
		if item.NextTry.After(time.Now()) {
			// Put the item back in the queue
			heap.Push(queue, item)
			return QueueMsg{
				Message: "No items ready to process",
			}
		}

		// Try to follow the user
		err := client.FollowUser(session, item.User.DID, false)
		if err != nil {
			// Increment attempts and update next try time
			item.Attempts++
			item.NextTry = time.Now().Add(time.Duration(item.Attempts) * 5 * time.Minute)
			heap.Push(queue, item)
			return QueueMsg{
				Message: "Failed to follow user",
				Error:   err,
			}
		}

		// Update user status
		item.User.Followed = true
		item.User.FollowDate = time.Now()
		return QueueMsg{
			Message: "Successfully followed user",
		}
	}
} 