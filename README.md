# bsky_follower

🌀 A simple Go app to follow top Bluesky accounts by topic.

You control the topic, follower threshold, and whether the app simulates or actually follows. Designed for natural API pacing.

---

## 🚀 Usage

```bash
go run main.go --id you@me.com --pw your-apppassword --topic tech [--real] [--min-followers 1000]
```

### Required Flags:

- `--id` → your Bluesky username or email
- `--pw` → [your Bluesky app password](https://bsky.app/settings/app-passwords)

### Optional Flags:

- `--topic` → one of: `tech`, `music`, `art`
- `--real` → actually follow (otherwise it just simulates)
- `--min-followers` → only follow users with at least this many followers, or use `'my'` to follow users with more followers than you

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
