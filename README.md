# Bluesky Follower

A Go program to help manage following users on Bluesky social network.

## Features

- Login to Bluesky using your credentials
- Follow users from a predefined list
- Filter users by minimum follower count
- Simulation mode to preview actions
- Track already followed users
- Update follower counts automatically

## Requirements

- Go 1.16 or later
- Bluesky account credentials
- `users.json` file with target users

## Installation

1. Clone this repository
2. Install dependencies:

```bash
go mod tidy
```

## Configuration

Create a `users.json` file in the same directory as the program with the following structure:

```json
[
  {
    "handle": "username.bsky.social",
    "did": "did:plc:...",
    "followers": 123,
    "savedOn": "2024-03-14T12:00:00Z"
  }
]
```

Fields:

- `handle`: The user's Bluesky handle (e.g., "username.bsky.social")
- `did`: The user's Decentralized Identifier (DID)
- `followers`: Number of followers (optional, will be fetched if 0)
- `savedOn`: Timestamp when the user was added (optional)

## Usage

Basic usage:

```bash
go run main.go --id your@email.com --pw yourpassword
```

To actually follow users (not just simulate):

```bash
go run main.go --id your@email.com --pw yourpassword --real
```

Additional options:

- `--json`: Path to JSON file (default: "users.json")
- `--min-followers`: Minimum follower count to follow (default: 0)
- `--real`: Actually follow users (default: simulation only)

Example with all options:

```bash
go run main.go --id your@email.com --pw yourpassword --json users.json --min-followers 1000 --real
```

## Notes

- The program will skip users you're already following
- Follower counts are automatically updated if not provided
- A 3-second delay is added between operations to avoid rate limiting
- The program tracks followed users in memory to avoid duplicate follows

---

## 🌐 Auth Notes

Bluesky uses **App Passwords**, which you can generate [here](https://bsky.app/settings/app-passwords). You do **not** need API keys or tokens.

The script logs in using your handle/email and app password and fetches your DID + JWT token securely.

---

## 📈 Planned Features

- SQLite + JSON cache for followed users
- Dynamic trending user fetch
- Config file / ENV support
- Better error handling + logging

---

Built by [@antoniwan](https://github.com/antoniwan) 🛠️
