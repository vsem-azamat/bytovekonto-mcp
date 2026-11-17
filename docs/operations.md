[Docs](README.md) › Operations

# Operations

## Commands

| Command | Does |
| --- | --- |
| `bytovekonto-mcp` or `serve` | Runs the MCP server on stdin/stdout. One server per state directory. |
| `check` | Logs in (or uses the cached token) and prints the home accounts, ticket counts and the number of documents to pay. Next to a running server it uses the cached access token or a password login in memory, never refreshing or writing the cache. |
| `logout` | Deletes the cached token. Stop the server first. The refresh token stays valid on the server until it expires. |
| `version` | Prints the version. |

## Troubleshooting

- **`the Bytové konto login was refused`**: wrong e-mail or password, or the account was disabled. The server waits 5 minutes before the next password attempt; fix the password file and restart rather than retrying.
- **`another bytovekonto-mcp uses …`**: a second `serve` with the same state directory; stop the other one or use another directory.
- **`Bytové konto answered 4xx/5xx: …`**: the API's own message is passed on. A changed API shows up here first; compare with what the official web app sends.
