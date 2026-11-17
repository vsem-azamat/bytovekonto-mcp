package watch_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vsem-azamat/bytovekonto-mcp/internal/api"
	"github.com/vsem-azamat/bytovekonto-mcp/internal/fakeapi"
	"github.com/vsem-azamat/bytovekonto-mcp/internal/watch"
)

type rig struct {
	fake *fakeapi.Server
	w    *watch.Watcher
	now  time.Time
}

func newRig(t *testing.T) *rig {
	t.Helper()
	f := fakeapi.New()
	t.Cleanup(f.Close)
	c, err := api.New(api.Config{BaseURL: f.URL, User: fakeapi.User, Password: fakeapi.Password})
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{fake: f, now: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)}
	r.w = &watch.Watcher{API: c, StateFile: filepath.Join(t.TempDir(), "watch.json"), Now: func() time.Time { return r.now }}
	return r
}

func (r *rig) run(t *testing.T) []watch.Event {
	t.Helper()
	var got []watch.Event
	n, err := r.w.Run(t.Context(), func(_ context.Context, e watch.Event) error { got = append(got, e); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if n != len(got) {
		t.Errorf("reported %d, delivered %d", n, len(got))
	}
	return got
}

func kinds(es []watch.Event) string {
	var ks []string
	for _, e := range es {
		ks = append(ks, e.Kind)
	}
	return strings.Join(ks, ",")
}

const (
	ownTicket = `{"id":3001,"createdAt":"2026-03-01T10:00:00Z","modifiedAt":"2026-03-11T10:00:00Z","subject":"Leaking tap","ticketState":"%s","tags":["USER_IS_CREATOR"],"lastCommentText":"x","lastCommentAuthor":"x","lastCommentCreatedAt":"%s"}`
	building  = `{"id":2001,"createdAt":"2026-03-11T08:00:00Z","subject":"Bins not collected","description":"Since Monday.","ticketState":"NEW","tags":["INVOLVES_COMMON_AREA"]}`
	lifecycle = `{"comments":[
		{"id":2,"createdAt":"2026-03-02T10:00:00Z","creatorSubjectId":21,"creatorSubjectName":"Tom Technician","text":"We will come on Friday."},
		{"id":4,"createdAt":"2026-03-11T09:00:00Z","creatorSubjectId":1,"creatorSubjectName":"Jana Example","text":"Is Friday still on?"},
		{"id":5,"createdAt":"2026-03-11T10:00:00Z","creatorSubjectId":21,"creatorSubjectName":"Tom Technician","text":"Yes, at 9."},
		{"id":6,"createdAt":"2026-03-11T10:30:00Z","creatorSubjectId":21,"creatorSubjectName":"Tom Technician","text":"gone","deletedAt":"2026-03-11T10:31:00Z"}],"history":[]}`
)

func ticketList(state, last string, extra ...string) string {
	return "[" + strings.Join(append([]string{strings.Replace(strings.Replace(ownTicket, "%s", state, 1), "%s", last, 1)}, extra...), ",") + "]"
}

func TestFirstRunReportsNothing(t *testing.T) {
	r := newRig(t)
	if got := r.run(t); len(got) != 0 {
		t.Fatalf("first run reported %v", kinds(got))
	}
	if got := r.run(t); len(got) != 0 {
		t.Fatalf("unchanged account reported %v", kinds(got))
	}
}

func TestChangesAreReportedOnce(t *testing.T) {
	r := newRig(t)
	r.fake.SetFixture("/tickets/list-with-last-comment", ticketList("IN_PROGRESS", "2026-03-02T10:00:00Z"))
	r.run(t)

	r.fake.SetFixture("/tickets/list-with-last-comment", ticketList("SOLVED", "2026-03-11T10:30:00Z", building))
	r.fake.SetFixture("/tickets/3001/lifecycle", lifecycle)
	r.fake.SetFixture("/project/infoboxes", `[{"id":-501,"htmlContent":"<p>Lift service on Friday</p>"}]`)
	got := r.run(t)
	if k := kinds(got); k != "ticket_state,ticket_message,building_ticket,announcement" {
		t.Fatalf("kinds = %s", k)
	}
	if e := got[0]; e.State != "SOLVED" || e.OldState != "IN_PROGRESS" || e.TicketID != 3001 {
		t.Errorf("state event = %+v", e)
	}
	if e := got[1]; e.Text != "Yes, at 9." || e.Author != "Tom Technician" || e.TicketID != 3001 {
		t.Errorf("message event = %+v, want only the new message from someone else", e)
	}
	if e := got[2]; e.TicketID != 2001 || e.Title != "Bins not collected" {
		t.Errorf("building event = %+v", e)
	}
	if e := got[3]; e.Text != "Lift service on Friday" {
		t.Errorf("announcement = %+v", e)
	}
	if got := r.run(t); len(got) != 0 {
		t.Fatalf("reported again: %v", kinds(got))
	}
}

func TestOverdueAndNewPayments(t *testing.T) {
	r := newRig(t)
	r.run(t) // the document due 2026-03-15 is known, not yet overdue
	r.now = time.Date(2026, 3, 20, 8, 0, 0, 0, time.UTC)
	got := r.run(t)
	if kinds(got) != "payment_overdue" || got[0].PaymentID != 7001 || *got[0].Amount != 25000 || got[0].Due != "2026-03-15" {
		t.Fatalf("events = %+v", got)
	}
	r.fake.SetFixture("/payments", `[{"id":7002,"remainingUnpaidAmount":24000,"documentCurrency":"CZK","documentDefinitionName":"Prescription","prescriptionName":"2026-04","formalCreatedDate":"2026-04-01","formalPaymentDueDate":"2026-04-15"}]`)
	if got := r.run(t); kinds(got) != "payment_new" || got[0].PaymentID != 7002 {
		t.Fatalf("events = %+v", got)
	}
}

func TestFailedDeliveryIsRetried(t *testing.T) {
	r := newRig(t)
	r.run(t)
	r.fake.SetFixture("/project/infoboxes", `[{"id":-501,"htmlContent":"<p>One</p>"},{"id":-502,"htmlContent":"<p>Two</p>"}]`)
	var got []string
	fail := true
	sink := func(_ context.Context, e watch.Event) error {
		if e.Text == "Two" && fail {
			return errors.New("telegram is down")
		}
		got = append(got, e.Text)
		return nil
	}
	if _, err := r.w.Run(t.Context(), sink); err == nil {
		t.Fatal("a failed delivery was not reported")
	}
	fail = false
	if _, err := r.w.Run(t.Context(), sink); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "One,Two" {
		t.Errorf("delivered %v, want each announcement once", got)
	}
}
