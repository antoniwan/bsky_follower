# 🦋 Bluesky Follower

A Go program to help manage following users on Bluesky social network.

## ✨ Features

- 🔐 Login to Bluesky using your credentials
- 👥 Follow users from a database
- 🔍 Filter users by minimum follower count
- 🎮 Simulation mode to preview actions
- 📊 Track already followed users
- 🔄 Update follower counts automatically
- 📥 Fetch and save top users from Bluesky directory
- 📝 Enhanced logging with debug mode
- 💾 SQLite database for reliable data storage
- 🆔 Proper DID (Decentralized Identifier) handling
- ⏱️ Rate limiting and error handling

## 📋 Requirements

- Go 1.16 or later
- Bluesky account credentials

## 🛠️ Setup

1. Create a `.env` file in the project root with the following variables:

```bash
# BlueSky API Configuration
BSKY_IDENTIFIER=your.bsky.social
BSKY_PASSWORD=your_app_password

# Application Settings
DEBUG_MODE=false
REQUEST_TIMEOUT=30s

# Database Configuration
DB_PATH=users.db

# Fallback Handles (comma-separated list of handles to use if directory fetch fails)
FALLBACK_HANDLES=handle1.bsky.social,handle2.bsky.social

# Rate Limiting
RATE_LIMIT_DELAY=1s
```

2. Install dependencies:

```bash
go mod download
```

3. Run the application:

```bash
go run main.go
```

## 🚀 Usage

The application supports the following command-line flags:

- `-simulate`: Run in simulation mode (no actual follows)
- `-update-top`: Fetch top users from bsky.directory and save to database
- `-min-followers`: Minimum followers required to follow (default: 0)
- `-real`: Actually follow users (default is simulation only)

### 💡 Examples

1. Fetch and save top users:

```bash
go run main.go --update-top
```

2. Follow users with minimum followers:

```bash
go run main.go --min-followers 1000 --real
```

3. Simulate following users:

```bash
go run main.go --simulate
```

## 💾 Data Storage

The application uses SQLite for data storage. All user data is stored in `users.db` with the following schema:

```sql
CREATE TABLE users (
    handle TEXT PRIMARY KEY,
    did TEXT,
    followers INTEGER,
    saved_on TEXT,
    followed BOOLEAN
)
```

Fields:

- `handle`: The user's Bluesky handle (e.g., "username.bsky.social")
- `did`: The user's Decentralized Identifier (DID)
- `followers`: Number of followers
- `saved_on`: Timestamp when the user was added
- `followed`: Whether the user has been followed

## 📝 Logging

The application provides detailed logging with multiple levels:

- `TRACE`: Most detailed logging (only in debug mode)
- `DEBUG`: Detailed logging (only in debug mode)
- `INFO`: General information
- `WARN`: Warning messages
- `ERROR`: Error messages
- `AUDIT`: Important state changes

To enable debug mode, set `DEBUG_MODE=true` in your environment variables.

## ⚙️ Environment Variables

- `BSKY_IDENTIFIER`: Your Bluesky handle or email
- `BSKY_PASSWORD`: Your Bluesky app password
- `BSKY_FALLBACK_HANDLES`: Comma-separated list of fallback handles (optional)
- `BSKY_TIMEOUT`: Request timeout in seconds (optional, default: 10)
- `DEBUG_MODE`: Enable detailed logging (optional, default: false)

## 📌 Notes

- ⏭️ The program will skip users you're already following
- 🔄 Follower counts are automatically updated if not provided
- ⏱️ A 3-second delay is added between operations to avoid rate limiting
- 📊 The program tracks followed users in both memory and database
- 💾 SQLite database provides reliable data storage and atomic updates
- 🆔 Proper DID handling ensures successful follows
- ❌ Error handling for common Bluesky API issues

## 📅 Version History

### v1.1.0 (2024-05-15)

- ✨ Added enhanced user filtering capabilities
- 🚀 Improved performance with optimized database queries
- 🛠️ Fixed rate limiting issues
- 📊 Added detailed statistics tracking
- 🔄 Improved error recovery mechanisms

### v1.0.0 (2024-04-30)

- 🎉 Initial stable release
- 🛠️ Fixed DID handling for follow operations
- 🚨 Improved error handling and logging
- ⏱️ Added proper rate limiting
- 💾 Enhanced database operations

---

## 🔐 Auth Notes

Bluesky uses **App Passwords**, which you can generate [here](https://bsky.app/settings/app-passwords). You do **not** need API keys or tokens.

The script logs in using your handle/email and app password and fetches your DID + JWT token securely.

---

## 🚀 Planned Features

- 📈 Dynamic trending user fetch with customizable filters
- 📤 Export/import database functionality with JSON/CSV support
- 💾 Automated backup and restore functionality
- 📦 Batch processing for large user lists with progress tracking
- ⚙️ Customizable rate limiting based on API response headers
- 🔍 Advanced user search and filtering capabilities
- 📊 Enhanced analytics dashboard
- 🔄 Real-time follower count updates
- 🎯 Smart following recommendations based on user interests

---

Built by [@antoniwan](https://github.com/antoniwan) 🛠️

## 🔒 Security Notes

- 🚫 Never commit your `.env` file to version control
- 🔐 Keep your app password secure
- 💾 The application uses SQLite for data storage - ensure proper file permissions
- 🐛 Debug mode should be disabled in production
