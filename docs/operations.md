[Docs](README.md) › Operations

# Operations

## Commands

| Command | Does |
| --- | --- |
| `bytovekonto-mcp` or `serve` | Runs the MCP server on stdin/stdout. One server per state directory. |
| `check` | Logs in (or uses the cached token) and prints the home accounts, ticket counts and the number of documents to pay. Next to a running server it uses the cached access token or a password login in memory, never refreshing or writing the cache. |
| `watch [-once] [-interval 30m] [-exec CMD]` | Reports what changed since its last check; see below. One watcher per state directory, next to the server. |
| `logout` | Deletes the server's cached token. Stop the server first. The refresh token stays valid on the server until it expires. |
| `version` | Prints the version. |

## Watching for changes

`watch` checks the account every `-interval` (default 30 minutes, at least 5), or once with `-once` (for a systemd timer or cron), and reports each change once:

| `kind` | When |
| --- | --- |
| `ticket_message` | Someone else wrote in one of the user's tickets (the user's own messages are skipped) |
| `ticket_state` | One of the user's tickets changed its state (`old_state` → `state`) |
| `building_ticket` | A new public ticket of the building appeared |
| `announcement` | A new or changed announcement |
| `payment_new` | A new document to pay (`amount`, `currency`, `due`) |
| `payment_overdue` | A document to pay passed its due date |
| `document` | A new or updated shared document |

Each event is a JSON object with `kind`, a stable `key`, `title`, `text` and the fields that apply (`at`, `author`, `ticket_id`, `state`, `payment_id`, `document_id`, …). Without `-exec`, events are printed as JSON lines on stdout. With `-exec CMD`, `/bin/sh -c CMD` runs once per event with the event on stdin and its kind in `BYTOVEKONTO_EVENT_KIND` (the password variables are not passed on); a command that fails or runs over 2 minutes stops the run, and that event and the ones after it are reported again next time.

The first run only remembers what is there and reports nothing. A ticket whose conversation the API refuses (4xx) is skipped with a warning and tried again next run; other errors fail the run. What has been reported is kept in `watch.json` in the state directory; deleting it starts over. The watcher keeps its own token (`watch-token.json`), so it runs next to the server without disturbing its login.

Example: a systemd user timer that hands each change to a notification script.

```ini
# ~/.config/systemd/user/bytovekonto-watch.service
[Service]
Type=oneshot
Environment=BYTOVEKONTO_USER=resident@example.com
Environment=BYTOVEKONTO_PASSWORD_FILE=%h/.config/bytovekonto-mcp/password
ExecStart=%h/.local/bin/bytovekonto-mcp watch -once -exec %h/.local/bin/notify-bytovekonto

# ~/.config/systemd/user/bytovekonto-watch.timer
[Timer]
OnCalendar=*:0/30
Persistent=true

[Install]
WantedBy=timers.target
```

## Troubleshooting

- **`the Bytové konto login was refused`**: wrong e-mail or password, or the account was disabled. The server waits 5 minutes before the next password attempt; fix the password file and restart rather than retrying.
- **`another bytovekonto-mcp serve (watch) uses …`**: a second server (watcher) with the same state directory; stop the other one or use another directory.
- **`watch state … is unreadable`**: `watch.json` is damaged or from another version; delete it (the next run reports nothing and starts over).
- **`Bytové konto answered 4xx/5xx: …`**: the API's own message is passed on. A changed API shows up here first; compare with what the official web app sends.
