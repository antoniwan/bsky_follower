# Bluesky Follower

An automated tool for following users on Bluesky (AT Protocol) with intelligent queue management and rate limiting.

## Features

- Automatic user discovery from multiple Bluesky sources
- Priority-based following queue (prioritizes users with more followers)
- Rate limiting to avoid API restrictions
- Retry mechanism with exponential backoff
- Comprehensive structured logging with rotation
- SQLite database for persistent storage
- Session management and automatic refresh

## Prerequisites

- Go 1.16 or higher
- SQLite3
- A Bluesky account

## Installation

1. Clone the repository:

```bash
git clone https://github.com/yourusername/bsky_follower.git
cd bsky_follower
```

2. Install dependencies:

```bash
go mod download
```

3. Create a `.env` file based on `.env-example`:

```bash
cp .env-example .env
```

4. Edit the `.env` file with your Bluesky credentials and logging configuration:

```env
# Bluesky Configuration
BSKY_IDENTIFIER=your.handle.bsky.social
BSKY_PASSWORD=your_password
BSKY_TIMEOUT=10
BSKY_FALLBACK_HANDLES=user1.bsky.social,user2.bsky.social

# Logging Configuration
DEBUG_MODE=true
LOG_LEVEL=debug  # Options: debug, info, warn, error
```

5. Initialize the database with the proper schema:

```bash
go run main.go -init-db
```

## Usage

### Basic Usage

Run the application in normal mode:

```bash
go run main.go
```

The application will:

1. Load existing users from the database
2. Start the follow queue processor
3. Periodically fetch new users from Bluesky
4. Follow users based on their priority (higher follower count = higher priority)

### Command Line Flags

- `-simulate`: Run in simulation mode (no actual follows)
- `-init-db`: Initialize database with proper schema (WARNING: This will delete existing data)

### Simulation Mode

To test without actually following users:

```bash
go run main.go -simulate
```

### Rate Limits

The application enforces the following rate limits:

- Maximum 50 follows per hour
- 1-second delay between API calls
- Session refresh every 24 hours

### Priority Levels

Users are prioritized based on their follower count:

- Priority 3: >10,000 followers
- Priority 2: >1,000 followers
- Priority 1: All other users

### Database

The application uses SQLite to store user information. The database file (`users.db`) contains:

- User handles and DIDs
- Follower counts
- Follow status
- Last check time
- Follow date
- Priority level
- Follow attempts

### Logging

The application uses a robust logging system with the following features:

#### Log Levels

- DEBUG: Detailed debugging information (when DEBUG_MODE=true)
- INFO: General information
- WARN: Warning messages
- ERROR: Error messages with stack traces
- AUDIT: Important actions with timestamps

#### Log Output

- Console output with colored formatting
- File output in JSON format (`logs/bsky_follower.log`)
- Automatic log rotation:
  - Max file size: 100MB
  - Max backups: 3
  - Max age: 7 days
  - Compression enabled

#### Log Format

- Timestamps in ISO8601 format
- Structured fields for better parsing
- Stack traces for errors
- Caller information
- Audit trail with additional metadata

## Configuration

### Environment Variables

#### Bluesky Configuration

- `BSKY_IDENTIFIER`: Your Bluesky handle
- `BSKY_PASSWORD`: Your Bluesky password
- `BSKY_TIMEOUT`: API request timeout in seconds
- `BSKY_FALLBACK_HANDLES`: Comma-separated list of fallback handles

#### Logging Configuration

- `DEBUG_MODE`: Enable debug logging (true/false)
- `LOG_LEVEL`: Minimum log level (debug, info, warn, error)

### Database Schema

```sql
CREATE TABLE users (
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
```

## Safety Features

1. **Rate Limiting**: Prevents hitting Bluesky API limits
2. **Retry Mechanism**: Automatically retries failed follows
3. **Session Management**: Refreshes session token periodically
4. **Duplicate Prevention**: Checks both memory and database for already followed users
5. **Error Handling**: Comprehensive error handling and logging
6. **Log Rotation**: Prevents disk space issues from log files

## Troubleshooting

### Database Issues

If you encounter database-related errors, you can reinitialize the database:

1. Backup your existing database (optional):

```bash
cp users.db users.db.backup
```

2. Initialize a fresh database:

```bash
go run main.go -init-db
```

This will create a new database with the correct schema. Note that this will delete all existing data.

### Logging Issues

If you need to adjust logging behavior:

1. Check the log files in the `logs` directory
2. Adjust log level in `.env` file
3. Monitor log rotation settings if disk space is a concern

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a new Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Disclaimer

This tool is for educational purposes only. Use responsibly and in accordance with Bluesky's terms of service. Excessive following may result in account restrictions.
