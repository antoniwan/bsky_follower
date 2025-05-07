package db

import (
	"database/sql"
	"fmt"

	"bsky_follower/internal/models"

	_ "modernc.org/sqlite"
)

// Store represents the database store
type Store struct {
	db     *sql.DB
	logger Logger
}

// Logger interface for logging
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewStore creates a new database store
func NewStore(dbPath string, logger Logger) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{
		db:     db,
		logger: logger,
	}

	if err := store.init(); err != nil {
		return nil, err
	}

	return store, nil
}

// init initializes the database schema
func (s *Store) init() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			handle TEXT PRIMARY KEY,
			did TEXT,
			followers INTEGER,
			saved_on TIMESTAMP,
			followed BOOLEAN,
			last_checked TIMESTAMP,
			follow_date TIMESTAMP,
			priority INTEGER DEFAULT 1,
			attempts INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		s.logger.Error("Failed to create table", "error", err)
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// LoadUsers loads all users from the database
func (s *Store) LoadUsers() ([]models.TargetUser, error) {
	rows, err := s.db.Query(`
		SELECT handle, did, followers, saved_on, followed, last_checked, follow_date, priority, attempts
		FROM users
	`)
	if err != nil {
		s.logger.Error("Failed to query users", "error", err)
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []models.TargetUser
	for rows.Next() {
		var user models.TargetUser
		var savedOn, lastChecked, followDate sql.NullTime
		
		err := rows.Scan(
			&user.Handle,
			&user.DID,
			&user.Followers,
			&savedOn,
			&user.Followed,
			&lastChecked,
			&followDate,
			&user.Priority,
			&user.Attempts,
		)
		if err != nil {
			s.logger.Error("Failed to scan user row", "error", err)
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}

		if savedOn.Valid {
			user.SavedOn = savedOn.Time
		}
		if lastChecked.Valid {
			user.LastChecked = lastChecked.Time
		}
		if followDate.Valid {
			user.FollowDate = followDate.Time
		}

		users = append(users, user)
	}

	return users, nil
}

// SaveUser saves a user to the database
func (s *Store) SaveUser(user models.TargetUser) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO users (
			handle, did, followers, saved_on, followed, last_checked, follow_date, priority, attempts
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		user.Handle,
		user.DID,
		user.Followers,
		user.SavedOn,
		user.Followed,
		user.LastChecked,
		user.FollowDate,
		user.Priority,
		user.Attempts,
	)
	if err != nil {
		s.logger.Error("Failed to save user", "error", err)
		return fmt.Errorf("failed to save user: %w", err)
	}

	return nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
} 