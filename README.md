# bytovekonto-mcp

An [MCP](https://modelcontextprotocol.io) server for a resident's [Bytové konto](https://www.bytovekonto.cz) account, the resident app of the Czech property-management platform BeiT. It lets an AI agent read the tenancy, the building's contacts and announcements, tickets and their conversations, payments, costs, utility meters and shared documents, and send tickets and comments (with photos or PDFs) to the building's management in the resident's name, always after showing a preview. It also makes QR codes to pay, compares costs with earlier periods and the building's average, and has a `watch` command that reports new messages from the management, announcements and payments due, for notifications.

It is not affiliated with BeiT. It uses the same API as the official apps, with the resident's own login.

## Install

Download a release archive for your platform from [Releases](https://github.com/vsem-azamat/bytovekonto-mcp/releases), check it against `checksums.txt`, and put `bytovekonto-mcp` on your `PATH`. Or build it with Go:

```bash
go install github.com/vsem-azamat/bytovekonto-mcp/cmd/bytovekonto-mcp@latest
```

## Use

```bash
export BYTOVEKONTO_USER=resident@example.com
export BYTOVEKONTO_PASSWORD_FILE=~/.config/bytovekonto-mcp/password   # a 0600 file holding the password
bytovekonto-mcp check    # log in and print a short summary
```

Add it to an MCP client as a stdio server, for example Claude Code:

```bash
claude mcp add bytovekonto -e BYTOVEKONTO_USER=resident@example.com \
  -e BYTOVEKONTO_PASSWORD_FILE=$HOME/.config/bytovekonto-mcp/password -- bytovekonto-mcp
```

Then ask things like "do I owe anything?", "what did the technician answer?", or "report that the kitchen tap leaks".

## Documentation

- [docs/tools.md](docs/tools.md): the tools
- [docs/safety.md](docs/safety.md): how writes are kept under the user's control
- [docs/configuration.md](docs/configuration.md): environment and files
- [docs/architecture.md](docs/architecture.md): how it works
- [docs/operations.md](docs/operations.md): running, watching for changes, checking, logging out

## License

[Apache-2.0](LICENSE)
