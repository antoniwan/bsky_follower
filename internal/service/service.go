package service

import (
	"fmt"
	"sync"
	"time"

	"bsky_follower/internal/api"
	"bsky_follower/internal/db"
	"bsky_follower/internal/models"
	"bsky_follower/internal/queue"
)

const (
	maxFollowsPerHour = 50
	maxRetries        = 3
	retryDelay        = 5 * time.Minute
	followCooldown    = 24 * time.Hour
)

// Service represents the main application service
type Service struct {
	config     *models.Config
	api        *api.Client
	db         *db.Store
	queue      *queue.Queue
	followed   map[string]bool
	mu         sync.Mutex
	lastFollow time.Time
	followCount int
	followReset time.Time
	logger     Logger
}

// Logger interface for logging
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewService creates a new service instance
func NewService(config *models.Config, apiClient *api.Client, dbStore *db.Store, logger Logger) *Service {
	return &Service{
		config:     config,
		api:        apiClient,
		db:         dbStore,
		queue:      queue.NewQueue(),
		followed:   make(map[string]bool),
		logger:     logger,
		followReset: time.Now(),
	}
}

// ProcessFollowQueue processes the follow queue
func (s *Service) ProcessFollowQueue(session *models.Session) {
	for {
		if s.queue.Len() == 0 {
			s.logger.Info("Queue is empty, waiting for new items")
			time.Sleep(time.Minute)
			continue
		}

		item := s.queue.Peek()
		if item == nil {
			continue
		}

		// Check if we need to wait for the next try
		if time.Now().Before(item.NextTry) {
			time.Sleep(time.Second)
			continue
		}

		// Check rate limits
		if s.followCount >= maxFollowsPerHour {
			if time.Since(s.followReset) < time.Hour {
				s.logger.Info("Rate limit reached, waiting for reset")
				time.Sleep(time.Minute)
				continue
			}
			s.followCount = 0
			s.followReset = time.Now()
		}

		// Check cooldown
		if time.Since(s.lastFollow) < followCooldown {
			s.logger.Info("Cooldown period active, waiting")
			time.Sleep(time.Minute)
			continue
		}

		// Process the item
		s.mu.Lock()
		item = s.queue.Pop()
		s.mu.Unlock()

		if err := s.processFollowItem(session, item); err != nil {
			s.logger.Error("Failed to process follow item", "error", err)
			if item.Attempts < maxRetries {
				item.Attempts++
				item.NextTry = time.Now().Add(retryDelay)
				s.mu.Lock()
				s.queue.Push(item.User, item.Priority)
				s.mu.Unlock()
			}
		}
	}
}

// processFollowItem processes a single follow queue item
func (s *Service) processFollowItem(session *models.Session, item *models.FollowQueueItem) error {
	s.logger.Info("Processing follow for user: %s", item.User.Handle)

	// Update user in database
	item.User.LastChecked = time.Now()
	if err := s.db.SaveUser(item.User); err != nil {
		return fmt.Errorf("failed to save user: %w", err)
	}

	// Follow the user
	if err := s.api.FollowUser(session, item.User.DID, false); err != nil {
		return fmt.Errorf("failed to follow user: %w", err)
	}

	// Update follow status
	s.mu.Lock()
	s.followed[item.User.Handle] = true
	s.lastFollow = time.Now()
	s.followCount++
	s.mu.Unlock()

	item.User.Followed = true
	item.User.FollowDate = time.Now()
	return s.db.SaveUser(item.User)
}

// AddToQueue adds a user to the follow queue
func (s *Service) AddToQueue(user models.TargetUser, priority int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.followed[user.Handle] {
		s.logger.Debug("User already followed: %s", user.Handle)
		return
	}

	s.queue.Push(user, priority)
	s.logger.Info("Added user to queue: %s (priority: %d)", user.Handle, priority)
}

// Close closes the service and its resources
func (s *Service) Close() error {
	return s.db.Close()
} 