# bytovekonto-mcp Agent Instructions

The working contract for AI agents and contributors. Facts about how the server works live in [`docs/`](docs/README.md).

## What this is

An MCP server (stdio, Go, `github.com/modelcontextprotocol/go-sdk`) that lets agents read one resident's Bytové konto account (the BeiT resident app) and send tickets and comments to the building's management in the resident's name. The API is the undocumented one the official apps use: the client sends only requests those apps send. The write rules in [docs/safety.md](docs/safety.md) are the core of the project: a change that weakens them needs an explicit reason in the PR.

## Rules

- Everything in the repository is written in English and describes the current state: no plans, decision logs, status notes or scratch files.
- The repository is public. Never commit anything that identifies a real account or building: names, e-mails, phone numbers, addresses, unit names, ids, tokens, captured API answers. Tests use the made-up data in `internal/fakeapi/`; examples use made-up data. `make repo` checks the obvious cases.
- Tests never call the real API. Live tests are manual; a live write sends a real message to a real building's management and needs the account owner's consent first.
- The password and the tokens never reach logs, error messages or URLs.
- `AGENTS.md` is the only agent instruction file; do not add `CLAUDE.md` or tool-specific copies.
- No AI attribution in commits or PRs.

## Order of work

1. Read the docs of the area you change.
2. Write a failing test against the fake API, then the change.
3. Update `docs/` when behaviour, tools, configuration or the write rules change.
4. `make check` (and `make release-check` when the build or the release config changes) before pushing. `make hooks` installs a pre-commit hook that runs the fast checks.

Branch from `main`, conventional commit messages (`feat:`, `fix:`, `docs:`, `chore:`, `test:`, `refactor:`), PR with green CI and an independent review (a fresh session or subagent), squash merge. Releases are `v*` tags on `main`.

## Code review

Reviews fix real defects (behaviour, security, broken contracts, code/docs drift) and drop taste-only nitpicks. Check: typed contracts across boundaries, no magic values, consistency with existing code, docs and code agree, tests prove the behaviour, errors handled.

## Map

| Path | Contents |
| --- | --- |
| `cmd/bytovekonto-mcp/` | The binary: subcommands, environment, state-directory lock |
| `internal/api/` | API client: OAuth2 login, token cache and refresh, typed endpoint calls |
| `internal/server/` | The MCP tools: reads, document download, the two writes with preview and audit |
| `internal/fakeapi/` | Fake API with made-up data for tests |
| `docs/` | How it works, configuration, tools, write rules, operations |
| `scripts/` | Repository policy (`repo-check.sh`) and the pre-commit hook (`hooks/`) |
| `tools/`, `tools/lint/` | Pinned development tools, run with `go tool` |
| `.github/workflows/` | CI (`make check` jobs, release snapshot) and the tag-driven release |
