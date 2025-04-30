# Bluesky Follower

A Go program to help manage following users on Bluesky social network.

## Features

- Login to Bluesky using your credentials
- Follow users from a predefined list
- Filter users by minimum follower count
- Simulation mode to preview actions
- Track already followed users
- Update follower counts automatically
- Fetch and save top users from Bluesky directory

## Requirements

- Go 1.16 or later
- Bluesky account credentials
- `users.json` file with target users (optional)

## Setup

1. Create a `.env` file in the project root with the following variables:

```bash
# Required: Bluesky API credentials
BSKY_IDENTIFIER=your.handle.bsky.social
BSKY_PASSWORD=your-app-password

# Optional: Fallback handles to use if API returns no suggestions
# Comma-separated list of handles
BSKY_FALLBACK_HANDLES=user1.bsky.social,user2.bsky.social,user3.bsky.social

# Optional: Request timeout in seconds (default: 10)
BSKY_TIMEOUT=10
```

2. Install dependencies:

```bash
go mod download
```

3. Run the application:

```bash
go run main.go
```

## Usage

The application supports the following command-line flags:

- `-simulate`: Run in simulation mode (no actual follows)
- `-file`: Path to the users JSON file (default: "users.json")
- `-update-top`: Fetch top users from bsky.directory and save to JSON
- `-min-followers`: Minimum followers required to follow (default: 0)
- `-real`: Actually follow users (default is simulation only)

### Examples

1. Fetch and save top users:

```bash
go run main.go --update-top
```

2. Follow users from a JSON file with minimum followers:

```bash
go run main.go --file users.json --min-followers 1000 --real
```

3. Simulate following users:

```bash
go run main.go --simulate --file users.json
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

## Environment Variables

- `BSKY_IDENTIFIER`: Your Bluesky handle or email
- `BSKY_PASSWORD`: Your Bluesky app password
- `BSKY_FALLBACK_HANDLES`: Comma-separated list of fallback handles (optional)
- `BSKY_TIMEOUT`: Request timeout in seconds (optional, default: 10)

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
- Better error handling + logging

---

Built by [@antoniwan](https://github.com/antoniwan) 🛠️

## Security Notes

- Never commit your `.env` file to version control
- Keep your app password secure
- The application stores user data in a JSON file - ensure proper file permissions
