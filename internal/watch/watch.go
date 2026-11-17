// Package watch finds what changed in a resident's Bytové konto since the last check: messages
// from the management in the user's tickets, state changes of those tickets, new public tickets
// of the building, announcements, documents to pay, overdue documents and shared documents. It
// remembers what it has reported in a state file, so each change is reported once.
package watch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/vsem-azamat/bytovekonto-mcp/internal/api"
	"github.com/vsem-azamat/bytovekonto-mcp/internal/text"
)

// Event kinds.
const (
	KindTicketMessage  = "ticket_message"  // a message from someone else in the user's ticket
	KindTicketState    = "ticket_state"    // the user's ticket changed its state
	KindBuildingTicket = "building_ticket" // a new public ticket of the building
	KindAnnouncement   = "announcement"    // a new or changed announcement
	KindPaymentNew     = "payment_new"     // a new document to pay
	KindPaymentOverdue = "payment_overdue" // a document to pay passed its due date
	KindDocument       = "document"        // a new or updated shared document
)

// Event is one change, as the watcher reports it.
type Event struct {
	Kind       string   `json:"kind"`
	Key        string   `json:"key"`
	At         string   `json:"at,omitempty"`
	Title      string   `json:"title"`
	Text       string   `json:"text,omitempty"`
	Author     string   `json:"author,omitempty"`
	TicketID   int64    `json:"ticket_id,omitempty"`
	State      string   `json:"state,omitempty"`
	OldState   string   `json:"old_state,omitempty"`
	PaymentID  int64    `json:"payment_id,omitempty"`
	Amount     *float64 `json:"amount,omitempty"`
	Currency   string   `json:"currency,omitempty"`
	Due        string   `json:"due,omitempty"`
	DocumentID string   `json:"document_id,omitempty"`
}

// Sink delivers one event; an error stops the run and the event is reported again next time.
type Sink func(context.Context, Event) error

// Watcher checks one account.
type Watcher struct {
	API       *api.Client
	StateFile string
	Now       func() time.Time
}

// state is what the watcher has seen: keys of reported or known items, and per key a value
// (a ticket's state, the time of its last comment).
type state struct {
	Version int               `json:"version"`
	Seen    map[string]string `json:"seen"`
}

const stateVersion = 1

// item is something found in the account: a key to remember with its value, and the event to
// report when the key is new or its value changed (nil for items only remembered).
type item struct {
	key, value string
	event      *Event
}

// Run reports the changes since the last run to sink and returns how many it reported. The
// first run (no state file) only remembers what is there and reports nothing.
func (w *Watcher) Run(ctx context.Context, sink Sink) (int, error) {
	if w.Now == nil {
		w.Now = time.Now
	}
	st, first, err := w.load()
	if err != nil {
		return 0, err
	}
	items, err := w.collect(ctx, st)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, it := range items {
		if it.event != nil && !first {
			if old, seen := st.Seen[it.key]; !seen || old != it.value {
				if err := sink(ctx, *it.event); err != nil {
					if serr := w.save(st); serr != nil {
						return sent, errors.Join(err, serr)
					}
					return sent, fmt.Errorf("deliver %s: %w", it.event.Kind, err)
				}
				sent++
			}
		}
		st.Seen[it.key] = it.value
	}
	return sent, w.save(st)
}

// collect lists the account's items in the order their events should be reported.
func (w *Watcher) collect(ctx context.Context, st *state) ([]item, error) {
	me, err := w.API.Me(ctx)
	if err != nil {
		return nil, fmt.Errorf("user: %w", err)
	}
	tickets, err := w.API.Tickets(ctx)
	if err != nil {
		return nil, fmt.Errorf("tickets: %w", err)
	}
	slices.SortStableFunc(tickets, func(a, b api.Ticket) int { return strings.Compare(a.CreatedAt, b.CreatedAt) })
	var items []item
	for _, t := range tickets {
		subject := strings.TrimSpace(t.Subject)
		if !t.HasTag(api.TagUserIsCreator) {
			items = append(items, item{key: "ticket:" + id(t.ID), event: &Event{
				Kind: KindBuildingTicket, At: t.CreatedAt, Title: subject, Text: strings.TrimSpace(t.Description), TicketID: t.ID, State: t.State,
			}})
			continue
		}
		// a new own ticket is the user's doing: only its later state changes are reported
		stateKey := "state:" + id(t.ID)
		var ev *Event
		if old, ok := st.Seen[stateKey]; ok && old != t.State {
			ev = &Event{Kind: KindTicketState, At: t.ModifiedAt, Title: subject, TicketID: t.ID, State: t.State, OldState: old}
		}
		items = append(items, item{key: stateKey, value: t.State, event: ev})
		last := ""
		if t.LastCommentCreatedAt != nil {
			last = *t.LastCommentCreatedAt
		}
		lcKey := "comments:" + id(t.ID)
		if last == "" || st.Seen[lcKey] == last {
			continue
		}
		msgs, err := w.messages(ctx, t, me.ID, st.Seen[lcKey])
		if err != nil {
			return nil, err
		}
		items = append(items, msgs...)
		items = append(items, item{key: lcKey, value: last})
	}

	boxes, err := w.API.Infoboxes(ctx)
	if err != nil {
		return nil, fmt.Errorf("announcements: %w", err)
	}
	for _, b := range boxes {
		body := text.HTML(b.HTMLContent)
		if body == "" {
			continue
		}
		sum := sha256.Sum256([]byte(b.HTMLContent))
		items = append(items, item{key: "announcement:" + id(b.ID) + ":" + hex.EncodeToString(sum[:8]), event: &Event{
			Kind: KindAnnouncement, Title: firstLine(body), Text: body,
		}})
	}

	pay, err := w.API.Payments(ctx, api.PaymentQuery{Filter: api.PaymentsToPay})
	if err != nil {
		return nil, fmt.Errorf("payments: %w", err)
	}
	today := w.Now().Format(time.DateOnly)
	for _, p := range pay {
		if p.RemainingUnpaid <= 0 {
			continue
		}
		amount := p.RemainingUnpaid
		due := ""
		if p.DueDate != nil {
			due = datePart(*p.DueDate)
		}
		name := strings.TrimSpace(p.DocumentName + " " + p.PrescriptionName)
		items = append(items, item{key: "payment:" + id(p.ID), event: &Event{
			Kind: KindPaymentNew, At: p.CreatedDate, Title: name, PaymentID: p.ID, Amount: &amount, Currency: p.Currency, Due: due, Text: p.Number,
		}})
		if due != "" && due < today {
			items = append(items, item{key: "overdue:" + id(p.ID), event: &Event{
				Kind: KindPaymentOverdue, Title: name, PaymentID: p.ID, Amount: &amount, Currency: p.Currency, Due: due, Text: p.Number,
			}})
		}
	}

	docs, err := w.API.Documents(ctx)
	if err != nil {
		return nil, fmt.Errorf("documents: %w", err)
	}
	slices.SortStableFunc(docs, func(a, b api.Document) int { return strings.Compare(a.ModifiedAt, b.ModifiedAt) })
	for _, d := range docs {
		items = append(items, item{key: "document:" + d.ID + ":" + d.ModifiedAt, event: &Event{
			Kind: KindDocument, At: d.ModifiedAt, Title: d.Name, Text: d.FileMeta.Filename, DocumentID: d.ID,
		}})
	}
	return items, nil
}

// messages are the comments of the user's ticket posted by others after the last one seen.
func (w *Watcher) messages(ctx context.Context, t api.Ticket, me int64, after string) ([]item, error) {
	lc, err := w.API.TicketLifecycle(ctx, t.ID)
	if err != nil {
		return nil, fmt.Errorf("ticket %d: %w", t.ID, err)
	}
	cs := slices.Clone(lc.Comments)
	slices.SortStableFunc(cs, func(a, b api.Comment) int { return compareTimes(a.CreatedAt, b.CreatedAt) })
	var items []item
	for _, c := range cs {
		if c.DeletedAt != nil || c.CreatorID == me || (after != "" && compareTimes(c.CreatedAt, after) <= 0) {
			continue
		}
		items = append(items, item{key: "comment:" + id(c.ID), event: &Event{
			Kind: KindTicketMessage, At: c.CreatedAt, Title: strings.TrimSpace(t.Subject), Text: strings.TrimSpace(c.Text),
			Author: c.CreatorName, TicketID: t.ID, State: t.State,
		}})
	}
	return items, nil
}

// compareTimes orders two API times; unreadable ones compare as text.
func compareTimes(a, b string) int {
	ta, ea := time.Parse(time.RFC3339Nano, a)
	tb, eb := time.Parse(time.RFC3339Nano, b)
	if ea != nil || eb != nil {
		return strings.Compare(a, b)
	}
	return ta.Compare(tb)
}

func id(n int64) string { return strconv.FormatInt(n, 10) }

func datePart(s string) string {
	if len(s) >= len(time.DateOnly) {
		return s[:len(time.DateOnly)]
	}
	return s
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func (w *Watcher) load() (*state, bool, error) {
	b, err := os.ReadFile(w.StateFile)
	if errors.Is(err, fs.ErrNotExist) {
		return &state{Version: stateVersion, Seen: map[string]string{}}, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("watch state: %w", err)
	}
	var st state
	if err := json.Unmarshal(b, &st); err != nil || st.Version != stateVersion {
		return nil, false, fmt.Errorf("watch state %s is unreadable; delete it to start over", w.StateFile)
	}
	if st.Seen == nil {
		st.Seen = map[string]string{}
	}
	return &st, false, nil
}

func (w *Watcher) save(st *state) error {
	b, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("watch state: %w", err)
	}
	dir := filepath.Dir(w.StateFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("watch state: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".watch-*")
	if err != nil {
		return fmt.Errorf("watch state: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return fmt.Errorf("watch state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("watch state: %w", err)
	}
	if err := os.Rename(tmp.Name(), w.StateFile); err != nil {
		return fmt.Errorf("watch state: %w", err)
	}
	return nil
}
