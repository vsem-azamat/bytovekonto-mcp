[Docs](README.md) › Architecture

# Architecture

```
MCP client ──stdio──▶ cmd/bytovekonto-mcp ──▶ internal/server (tools) ──▶ internal/api (client) ──HTTPS──▶ Bytové konto API
timer/loop ─────────▶ cmd/bytovekonto-mcp watch ──▶ internal/watch ───────▶ internal/api
```

## The API

Bytové konto's apps call a REST API at `https://admin.bytovekonto.cz`: an OAuth2 token endpoint at `/api/oauth/token` and resident endpoints under `/app/`. The API is not documented publicly; `internal/api` calls only endpoints and bodies the official apps send, with the client id of the mobile app (`io.beit.consumer.mobile`).

## Authentication

- **Login.** A password grant (`grant_type=password`) returns an access token (a JWT valid about 10 minutes) and a refresh token (valid about a year). The grant type and client id go in the query, as the apps send them; the user name, password and refresh token go in the form body, so they never appear in a URL.
- **Cache.** The token pair is cached in `token.json` in the state directory (mode 0600, directory 0700). A restart uses the cache and needs no password while the refresh token is valid.
- **Refresh.** An access token within 30 seconds of its expiry is refreshed with the refresh token; the answer may replace the refresh token. A refused refresh falls back to a password login. A call answered 401 drops that access token (unless another call already replaced it) and retries once with a new one; token fetching is single-flight, so parallel calls refused together refresh once.
- **Back-off.** A refused password login blocks further password logins for 5 minutes, so a wrong password does not hammer the login endpoint.
- **One process.** `serve` holds a lock on the state directory: two servers would share and rotate one refresh token. `watch` has its own lock and its own token cache, so it logs in separately and runs next to the server. `check` next to a running server uses the cached access token while it is valid and otherwise logs in with the password in memory; it never refreshes (that could rotate the server's refresh token) and never writes the cache.

There is no logout endpoint for the apps; `logout` deletes the cache.

## Packages

- `internal/api`: the HTTP client, the token logic and typed calls. Types hold the fields the tools use; fields whose shape the apps do not show (attachments, history changes, notifications) are passed on as raw JSON.
- `internal/server`: the tools. Reads fetch in parallel and return compact views; the two writes share the preview-then-confirm flow, the attachment checks and the repeated-send guard described in [safety.md](safety.md).
- `internal/watch`: the change watcher. Each run lists the account's items (own tickets with their state and new messages from others, the building's tickets, announcements, documents to pay and overdue, shared documents) under stable keys, reports the keys it has not seen (or whose value changed) to a sink in order, and saves what it reported.
- `internal/text`: announcement HTML to plain text, shared by the tools and the watcher.
- `internal/fakeapi`: an `httptest` server with the token endpoint, the `/app` endpoints and made-up data, recording write requests. All tests run against it.

## Tickets the account sees

`/app/tickets/list-with-last-comment` returns the user's own tickets (tagged `USER_IS_CREATOR`) and the building's public tickets about common areas (tagged `INVOLVES_COMMON_AREA`). The tools show only the user's own by default.

A new ticket is sent with the unit as its only related entity (`OBJECT`); the API adds the building (`PROJECT`). Residents cannot change a ticket's state (`possibleTicketStates` is empty for them).

Attachments of a new ticket or comment are sent inline, as the apps send them: `{"content": <standard base64>, "fileName": …}` in the body's `attachments` list.

The building's income and expense overviews (`/app/payments/project/…`) answer 401 for tenants; the server does not call them.
