package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vsem-azamat/bytovekonto-mcp/internal/watch"
)

func TestExecSink(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	t.Setenv("BYTOVEKONTO_PASSWORD_FILE", "/secret/password")
	sink := execSink(`{ cat; echo; echo "kind=$BYTOVEKONTO_EVENT_KIND pw=${BYTOVEKONTO_PASSWORD_FILE:-none}"; } > `+out, io.Discard)
	if err := sink(t.Context(), watch.Event{Kind: watch.KindAnnouncement, Title: "Lift"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); !strings.Contains(s, `"title":"Lift"`) || !strings.Contains(s, "kind=announcement pw=none") {
		t.Errorf("the command saw %q", s)
	}
	if err := execSink("exit 3", io.Discard)(t.Context(), watch.Event{Kind: "x"}); err == nil {
		t.Error("a failing command was not reported")
	}
}
