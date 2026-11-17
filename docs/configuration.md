[Docs](README.md) › Configuration

# Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `BYTOVEKONTO_USER` | – (required) | The login e-mail |
| `BYTOVEKONTO_PASSWORD` | – | The password; used only when no valid token is cached. Removed from the environment at start. |
| `BYTOVEKONTO_PASSWORD_FILE` | – | A file holding the password; when set, it wins over the variable above |
| `BYTOVEKONTO_URL` | `https://admin.bytovekonto.cz` | API base URL; must be https (plain http only for loopback addresses in tests) |
| `BYTOVEKONTO_STATE_DIR` | `$XDG_STATE_HOME/bytovekonto-mcp` (`~/.local/state/bytovekonto-mcp`) | Token cache (`token.json`) and the server lock (`serve.lock`) |
| `BYTOVEKONTO_DOWNLOAD_DIR` | `$XDG_DATA_HOME/bytovekonto-mcp/documents` (`~/.local/share/bytovekonto-mcp/documents`) | Where `download_document` saves files; `-` disables saving (files are then only returned inline, up to 4 MiB) |
| `BYTOVEKONTO_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`; logs are JSON on stderr |

Files written by the server: `token.json` (0600, a secret), downloaded documents (0600) named `<document id>-<file name>` with unsafe characters replaced.
