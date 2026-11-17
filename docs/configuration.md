[Docs](README.md) › Configuration

# Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `BYTOVEKONTO_USER` | – (required) | The login e-mail |
| `BYTOVEKONTO_PASSWORD` | – | The password; used only when no valid token is cached. Removed from the environment at start. |
| `BYTOVEKONTO_PASSWORD_FILE` | – | A file holding the password; when set, it wins over the variable above |
| `BYTOVEKONTO_URL` | `https://admin.bytovekonto.cz` | API base URL; must be https (plain http only for loopback addresses in tests) |
| `BYTOVEKONTO_STATE_DIR` | `$XDG_STATE_HOME/bytovekonto-mcp` (`~/.local/state/bytovekonto-mcp`) | The server's token cache (`token.json`) and lock (`serve.lock`); the watcher's token cache (`watch-token.json`), lock (`watch.lock`) and state (`watch.json`) |
| `BYTOVEKONTO_DOWNLOAD_DIR` | `$XDG_DATA_HOME/bytovekonto-mcp/documents` (`~/.local/share/bytovekonto-mcp/documents`) | Where `download_document` and `payment_qr` save files; `-` disables saving (files are then only returned inline, documents up to 4 MiB) |
| `BYTOVEKONTO_ATTACH_DIRS` | – | Absolute directories, separated by `:`, the write tools may attach files from by path; unset, files can only be attached inline |
| `BYTOVEKONTO_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`; logs are JSON on stderr |

Files written: `token.json` and `watch-token.json` (0600, secrets), `watch.json` (what the watcher has reported), downloaded documents (0600) named `<document id>-<file name>` and QR codes named `qr-<document ids>.png`, with unsafe characters replaced.
