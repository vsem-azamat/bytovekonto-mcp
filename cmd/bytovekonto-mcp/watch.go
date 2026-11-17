package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vsem-azamat/bytovekonto-mcp/internal/watch"
)

const (
	defaultWatchInterval = 30 * time.Minute
	minWatchInterval     = 5 * time.Minute
	execTimeout          = 2 * time.Minute
	watchRunTimeout      = 5 * time.Minute
	// watchTokenFile and watchStateFile live in the state directory next to the server's token.
	watchTokenFile = "watch-token.json"
	watchStateFile = "watch.json"
)

func watchCmd(ctx context.Context, cfg config, log *slog.Logger, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	once := fs.Bool("once", false, "check once and exit (for a timer)")
	interval := fs.Duration("interval", defaultWatchInterval, "time between checks (at least 5m)")
	execCmd := fs.String("exec", "", "run this shell command for each change, with the event as JSON on stdin")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "watch: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if *interval < minWatchInterval {
		fmt.Fprintf(stderr, "watch: the interval must be at least %s\n", minWatchInterval)
		return 2
	}
	unlock, err := lockStateDir(cfg.stateDir, watchLock)
	if err != nil {
		log.Error("cannot start", "err", err.Error())
		return 1
	}
	defer unlock()
	// the watcher keeps its own token: the server's refresh would invalidate a shared one
	cfg.tokenFile = filepath.Join(cfg.stateDir, watchTokenFile)
	client, err := newClient(cfg, log)
	if err != nil {
		log.Error("cannot start", "err", err.Error())
		return 1
	}
	w := &watch.Watcher{API: client, StateFile: filepath.Join(cfg.stateDir, watchStateFile), Log: log.With("component", "watch")}
	sink := printSink(stdout)
	if *execCmd != "" {
		sink = execSink(*execCmd, stderr)
	}
	for {
		rctx, cancel := context.WithTimeout(ctx, watchRunTimeout)
		n, err := w.Run(rctx, sink)
		cancel()
		switch {
		case err != nil:
			log.Warn("watch run failed", "reported", n, "err", err.Error())
			if *once {
				return 1
			}
		default:
			log.Info("watch run", "reported", n)
		}
		if *once {
			return 0
		}
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(*interval):
		}
	}
}

func printSink(out io.Writer) watch.Sink {
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	return func(_ context.Context, e watch.Event) error { return enc.Encode(e) }
}

// execSink runs the command for each event with the event as JSON on its stdin and its kind in
// BYTOVEKONTO_EVENT_KIND. The command does not see the password settings.
func execSink(command string, stderr io.Writer) watch.Sink {
	return func(ctx context.Context, e watch.Event) error {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		cctx, cancel := context.WithTimeout(ctx, execTimeout)
		defer cancel()
		cmd := exec.CommandContext(cctx, "/bin/sh", "-c", command) //nolint:gosec // the command is the operator's configuration
		cmd.Stdin = bytes.NewReader(b)
		cmd.Stdout, cmd.Stderr = stderr, stderr
		cmd.Env = append(childEnv(), "BYTOVEKONTO_EVENT_KIND="+e.Kind)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("exec: %w", err)
		}
		return nil
	}
}

func childEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "BYTOVEKONTO_PASSWORD") {
			continue
		}
		env = append(env, kv)
	}
	return env
}
