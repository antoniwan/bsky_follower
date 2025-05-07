# Bluesky Follower

A Go application for automatically following users on Bluesky (AT Protocol) with rate limiting and queue management.

## Features

- Automatic user following with rate limiting
- Priority queue for follow operations
- SQLite database for user tracking
- Configurable follow limits and cooldowns
- Logging with rotation
- Graceful shutdown handling

## Project Structure

```
.
├── cmd/
│   └── bsky_follower/    # Application entry point
├── internal/
│   ├── api/             # Bluesky API client
│   ├── config/          # Configuration management
│   ├── db/              # Database operations
│   ├── models/          # Data models
│   ├── queue/           # Priority queue implementation
│   └── service/         # Main service logic
├── pkg/
│   └── logger/          # Logging package
├── .env-example         # Example environment configuration
├── go.mod              # Go module file
└── README.md           # This file
```

## Configuration

Create a `.env` file based on `.env-example` with your Bluesky credentials:

```env
BSKY_IDENTIFIER=your.handle.bsky.social
BSKY_PASSWORD=your_password
BSKY_TIMEOUT=10
BSKY_FALLBACK_HANDLES=handle1.bsky.social,handle2.bsky.social
DEBUG_MODE=false
```

## Building

```bash
go build -o bsky_follower ./cmd/bsky_follower
```

## Running

```bash
./bsky_follower
```

## Rate Limits

- Maximum 50 follows per hour
- 24-hour cooldown between follows
- Maximum 3 retry attempts with 5-minute delay

## Logging

Logs are written to `logs/bsky_follower.log` with the following features:

- Automatic rotation at 100MB
- Keeps 3 backup files
- Logs are kept for 7 days
- Automatic compression of old logs

## Database

The application uses SQLite to store user information:

- User handles and DIDs
- Follower counts
- Follow status and dates
- Priority and attempt tracking

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
