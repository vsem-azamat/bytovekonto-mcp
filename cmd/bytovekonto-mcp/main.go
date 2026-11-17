// Command bytovekonto-mcp is an MCP server (stdio) for a resident's Bytové konto account.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vsem-azamat/bytovekonto-mcp/internal/api"
	"github.com/vsem-azamat/bytovekonto-mcp/internal/server"
)

// version is set at release builds.
var version = "dev"

const usage = `bytovekonto-mcp: an MCP server (stdio) for a resident's Bytové konto (BeiT) account.

Usage:
  bytovekonto-mcp [serve]  run the MCP server on stdin/stdout
  bytovekonto-mcp check    log in and print a short summary of the account (no changes)
  bytovekonto-mcp logout   delete the cached token
  bytovekonto-mcp version  print the version

Environment:
  BYTOVEKONTO_USER           login e-mail (required)
  BYTOVEKONTO_PASSWORD       its password, used only when no valid token is cached
                             (or BYTOVEKONTO_PASSWORD_FILE: a file holding it)
  BYTOVEKONTO_URL            API base URL (default https://admin.bytovekonto.cz)
  BYTOVEKONTO_STATE_DIR      where the token is cached (default $XDG_STATE_HOME/bytovekonto-mcp)
  BYTOVEKONTO_DOWNLOAD_DIR   where download_document saves files
                             (default $XDG_DATA_HOME/bytovekonto-mcp/documents; "-" disables saving)
  BYTOVEKONTO_LOG_LEVEL      debug, info (default), warn or error
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "version", "-version", "--version":
		fmt.Fprintln(stdout, version)
		return 0
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "serve", "check", "logout":
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", cmd, usage)
		return 2
	}
	cfg, err := loadConfig(cmd != "logout")
	if err != nil {
		fmt.Fprintln(stderr, "bytovekonto-mcp:", err)
		return 2
	}
	log := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: cfg.level}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "check":
		// next to a running server, use its token without replacing it
		if unlock, err := lockStateDir(cfg.stateDir); err == nil {
			defer unlock()
		} else {
			cfg.tokenReadOnly = true
		}
		return check(ctx, cfg, log, stdout)
	case "logout":
		unlock, err := lockStateDir(cfg.stateDir)
		if err != nil {
			fmt.Fprintln(stderr, "logout:", err, "(stop the server first)")
			return 1
		}
		defer unlock()
		c, err := newClient(cfg, log)
		if err == nil {
			err = c.Forget()
		}
		if err != nil {
			fmt.Fprintln(stderr, "logout:", err)
			return 1
		}
		fmt.Fprintln(stdout, "cached token deleted")
		return 0
	default:
		return serve(ctx, cfg, log, stdin, stdout)
	}
}

type config struct {
	url, user, password string
	stateDir            string
	tokenFile           string
	downloadDir         string
	tokenReadOnly       bool
	level               slog.Level
}

func loadConfig(needUser bool) (config, error) {
	c := config{
		url:      strings.TrimSpace(os.Getenv("BYTOVEKONTO_URL")),
		user:     strings.TrimSpace(os.Getenv("BYTOVEKONTO_USER")),
		password: os.Getenv("BYTOVEKONTO_PASSWORD"),
	}
	if f := os.Getenv("BYTOVEKONTO_PASSWORD_FILE"); f != "" {
		b, err := os.ReadFile(f) //nolint:gosec // the password file is configuration
		if err != nil {
			return c, fmt.Errorf("BYTOVEKONTO_PASSWORD_FILE: %w", err)
		}
		c.password = strings.TrimRight(string(b), "\r\n")
	}
	// not passed on to anything this process starts
	_ = os.Unsetenv("BYTOVEKONTO_PASSWORD")
	if needUser && c.user == "" {
		return c, errors.New("missing environment: BYTOVEKONTO_USER")
	}
	home := os.Getenv("HOME")
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	c.stateDir = os.Getenv("BYTOVEKONTO_STATE_DIR")
	if c.stateDir == "" {
		c.stateDir = filepath.Join(stateHome, "bytovekonto-mcp")
	}
	c.tokenFile = filepath.Join(c.stateDir, "token.json")
	switch d := os.Getenv("BYTOVEKONTO_DOWNLOAD_DIR"); d {
	case "-":
	case "":
		c.downloadDir = filepath.Join(dataHome, "bytovekonto-mcp", "documents")
	default:
		c.downloadDir = d
	}
	lvl := os.Getenv("BYTOVEKONTO_LOG_LEVEL")
	if lvl == "" || c.level.UnmarshalText([]byte(lvl)) != nil {
		c.level = slog.LevelInfo
	}
	return c, nil
}

func newClient(cfg config, log *slog.Logger) (*api.Client, error) {
	return api.New(api.Config{
		BaseURL:       cfg.url,
		User:          cfg.user,
		Password:      cfg.password,
		TokenFile:     cfg.tokenFile,
		TokenReadOnly: cfg.tokenReadOnly,
		Logger:        log.With("component", "api"),
	})
}

func serve(ctx context.Context, cfg config, log *slog.Logger, stdin io.Reader, stdout io.Writer) int {
	unlock, err := lockStateDir(cfg.stateDir)
	if err != nil {
		log.Error("cannot start", "err", err.Error())
		return 1
	}
	defer unlock()
	client, err := newClient(cfg, log)
	if err != nil {
		log.Error("cannot start", "err", err.Error())
		return 1
	}
	srv := server.New(client, server.Options{Logger: log.With("component", "server"), Version: version, DownloadDir: cfg.downloadDir})
	log.Info("serving", "version", version)
	err = srv.MCP().Run(ctx, &mcp.IOTransport{Reader: io.NopCloser(stdin), Writer: nopWriteCloser{stdout}})
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
		log.Error("server stopped", "err", err.Error())
		return 1
	}
	return 0
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func check(ctx context.Context, cfg config, log *slog.Logger, stdout io.Writer) int {
	client, err := newClient(cfg, log)
	if err != nil {
		fmt.Fprintln(stdout, "config:", err)
		return 1
	}
	cctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := client.Login(cctx); err != nil {
		fmt.Fprintln(stdout, "login:", err)
		return 1
	}
	accounts, err := client.HomeAccounts(cctx)
	if err != nil {
		fmt.Fprintln(stdout, "home accounts:", err)
		return 1
	}
	for _, a := range accounts {
		fmt.Fprintf(stdout, "home account %d: %s (%s, %s), %s – %s, active %v\n", a.ID, a.Name, a.SubType, a.ProjectName, a.ActiveFromDate, a.ActiveToDate, a.Active)
	}
	tickets, err := client.Tickets(cctx)
	if err != nil {
		fmt.Fprintln(stdout, "tickets:", err)
		return 1
	}
	own, open := 0, 0
	for _, t := range tickets {
		if t.HasTag(api.TagUserIsCreator) {
			own++
			if t.Open() {
				open++
			}
		}
	}
	fmt.Fprintf(stdout, "tickets: %d visible, %d own, %d own open\n", len(tickets), own, open)
	pay, err := client.Payments(cctx, api.PaymentQuery{Filter: api.PaymentsToPay})
	if err != nil {
		fmt.Fprintln(stdout, "payments:", err)
		return 1
	}
	fmt.Fprintf(stdout, "payments to pay: %d\n", len(pay))
	return 0
}
