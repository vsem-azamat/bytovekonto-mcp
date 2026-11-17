package server_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vsem-azamat/bytovekonto-mcp/internal/api"
	"github.com/vsem-azamat/bytovekonto-mcp/internal/fakeapi"
	"github.com/vsem-azamat/bytovekonto-mcp/internal/server"
)

type harness struct {
	fake    *fakeapi.Server
	session *mcp.ClientSession
	dir     string
	attach  string
}

func start(t *testing.T) *harness {
	t.Helper()
	f := fakeapi.New()
	t.Cleanup(f.Close)
	c, err := api.New(api.Config{BaseURL: f.URL, User: fakeapi.User, Password: fakeapi.Password})
	if err != nil {
		t.Fatal(err)
	}
	dir, attach := t.TempDir(), t.TempDir()
	srv := server.New(c, server.Options{Now: func() time.Time { return testNow }, DownloadDir: dir, AttachDirs: []string{attach}})
	st, ct := mcp.NewInMemoryTransports()
	ss, err := srv.MCP().Connect(t.Context(), st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	cs, err := client.Connect(t.Context(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return &harness{fake: f, session: cs, dir: dir, attach: attach}
}

var testNow = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

// call runs a tool and returns its structured result, or the error text when the tool failed.
func (h *harness) call(t *testing.T, name string, args map[string]any) (map[string]any, string) {
	t.Helper()
	res, err := h.session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		var b strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				b.WriteString(tc.Text)
			}
		}
		return nil, b.String()
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out, ""
}

func mustCall(t *testing.T, h *harness, name string, args map[string]any) map[string]any {
	t.Helper()
	out, errText := h.call(t, name, args)
	if errText != "" {
		t.Fatalf("%s failed: %s", name, errText)
	}
	return out
}

func TestToolsAreListed(t *testing.T) {
	h := start(t)
	res, err := h.session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"home_overview": true, "building_info": true, "list_tickets": true, "get_ticket": true, "ticket_options": true,
		"list_payments": true, "get_payment": true, "payments_overview": true, "costs": true, "list_meters": true, "get_meter": true,
		"list_documents": true, "download_document": true, "notifications": true, "create_ticket": true, "comment_ticket": true,
		"payment_qr": true, "compare_costs": true,
	}
	for _, tool := range res.Tools {
		if !want[tool.Name] {
			t.Errorf("unexpected tool %s", tool.Name)
		}
		delete(want, tool.Name)
		writes := tool.Name == "create_ticket" || tool.Name == "comment_ticket"
		if tool.Annotations == nil || tool.Annotations.ReadOnlyHint == writes {
			t.Errorf("%s: read-only hint wrong", tool.Name)
		}
	}
	if len(want) > 0 {
		t.Errorf("missing tools: %v", want)
	}
}

func TestOverview(t *testing.T) {
	h := start(t)
	out := mustCall(t, h, "home_overview", nil)
	ten := out["tenancy"].(map[string]any)
	if ten["building"] != "Example House" || ten["from"] != "2026-01-01" {
		t.Errorf("tenancy = %v", ten)
	}
	if units := ten["units"].([]any); len(units) != 1 || !strings.HasPrefix(units[0].(string), "A101") {
		t.Errorf("units = %v", units)
	}
	open := out["open_tickets"].([]any)
	if len(open) != 1 || open[0].(map[string]any)["id"].(float64) != 3001 {
		t.Errorf("open tickets = %v, want only the own open ticket 3001", open)
	}
	if out["unread_notifications"].(float64) != 2 {
		t.Errorf("unread = %v", out["unread_notifications"])
	}
	ann := out["announcements"].([]any)
	if len(ann) != 1 || ann[0] != "Water off on Monday 8–12\nManaged by Example Manager" {
		t.Errorf("announcements = %q", ann)
	}
	money := out["money"].(map[string]any)
	if money["past_due"].(float64) != 25000 || len(money["to_pay"].([]any)) != 1 {
		t.Errorf("money = %v", money)
	}
}

func ids(t *testing.T, out map[string]any) []int {
	t.Helper()
	var got []int
	for _, x := range out["tickets"].([]any) {
		got = append(got, int(x.(map[string]any)["id"].(float64)))
	}
	return got
}

func TestListTicketsScopes(t *testing.T) {
	h := start(t)
	cases := []struct {
		args map[string]any
		want []int
	}{
		{nil, []int{3001, 3000}},
		{map[string]any{"scope": "building"}, []int{2000}},
		{map[string]any{"scope": "all", "state": "open"}, []int{3001, 2000}},
		{map[string]any{"state": "closed"}, []int{3000}},
		{map[string]any{"query": "friday"}, []int{3001}},
		{map[string]any{"scope": "all", "limit": 1}, []int{3001}},
	}
	for _, c := range cases {
		got := ids(t, mustCall(t, h, "list_tickets", c.args))
		if jsonString(got) != jsonString(c.want) {
			t.Errorf("list_tickets %v = %v, want %v", c.args, got, c.want)
		}
	}
	if _, errText := h.call(t, "list_tickets", map[string]any{"scope": "everyone"}); errText == "" {
		t.Error("an unknown scope was accepted")
	}
}

func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestGetTicketConversation(t *testing.T) {
	h := start(t)
	out := mustCall(t, h, "get_ticket", map[string]any{"id": 3001})
	conv := out["conversation"].([]any)
	if len(conv) != 2 {
		t.Fatalf("conversation = %v, want 2 messages without the deleted one", conv)
	}
	if first := conv[0].(map[string]any); first["author"] != "Jana Example" {
		t.Errorf("first message = %v, want the oldest first", first)
	}
	if len(out["history"].([]any)) != 1 {
		t.Errorf("history = %v", out["history"])
	}
	if _, errText := h.call(t, "get_ticket", map[string]any{"id": 42}); !strings.Contains(errText, "no ticket 42") {
		t.Errorf("missing ticket error = %q", errText)
	}
}

func TestPaymentsAndCosts(t *testing.T) {
	h := start(t)
	p := mustCall(t, h, "list_payments", nil)["payments"].([]any)
	if len(p) != 1 || p[0].(map[string]any)["due"] != "2026-03-15" {
		t.Errorf("payments = %v", p)
	}
	d := mustCall(t, h, "get_payment", map[string]any{"id": 7001})
	if d["variable_symbol"] != "1234567" || len(d["items"].([]any)) != 2 {
		t.Errorf("payment = %v", d)
	}
	if _, errText := h.call(t, "list_payments", map[string]any{"filter": "late"}); errText == "" {
		t.Error("an unknown filter was accepted")
	}
	c := mustCall(t, h, "costs", nil)
	if c["from"] != "2026-01-01" || c["to"] != "2026-03-10" || c["settled"] != false {
		t.Errorf("costs range = %v %v %v", c["from"], c["to"], c["settled"])
	}
	heat := c["deliveries"].([]any)[1].(map[string]any)
	if heat["name"] != "Heat" || heat["unit"] != "kWh" {
		t.Errorf("heat = %v", heat)
	}
	if c["rent"].(map[string]any)["amount"].(float64) != 24000 {
		t.Errorf("rent = %v", c["rent"])
	}
	if _, errText := h.call(t, "costs", map[string]any{"from": "2024-01-01"}); !strings.Contains(errText, "limited") {
		t.Errorf("long range error = %q", errText)
	}
}

func TestMeter(t *testing.T) {
	h := start(t)
	m := mustCall(t, h, "get_meter", map[string]any{"id": 801})
	last := m["last_readings"].([]any)
	if len(last) != 1 || last[0].(map[string]any)["value"].(float64) != 1234.5 {
		t.Errorf("last readings = %v, want only the channel with a value", last)
	}
	ch := m["consumption"].(map[string]any)["channels"].(map[string]any)["P+ T1"].(map[string]any)
	if ch["total"].(float64) != 200 {
		t.Errorf("chart = %v", ch)
	}
}

func TestDownloadDocumentStaysInDirectory(t *testing.T) {
	h := start(t)
	res, err := h.session.CallTool(t.Context(), &mcp.CallToolParams{Name: "download_document", Arguments: map[string]any{"id": "doc-1"}})
	if err != nil || res.IsError {
		t.Fatalf("download: %v %v", err, res)
	}
	out := res.StructuredContent.(map[string]any)
	path := out["saved_to"].(string)
	if filepath.Dir(path) != h.dir {
		t.Fatalf("saved to %s, outside %s", path, h.dir)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != fakeapi.DocumentContent {
		t.Fatalf("saved file = %q, %v", b, err)
	}
	var inline bool
	for _, c := range res.Content {
		if er, ok := c.(*mcp.EmbeddedResource); ok && string(er.Resource.Blob) == fakeapi.DocumentContent {
			inline = true
		}
	}
	if !inline {
		t.Error("the document was not returned inline")
	}
}

func TestCreateTicketPreviewSendsNothing(t *testing.T) {
	h := start(t)
	args := map[string]any{"category": "fault", "subject": "Broken hob", "description": "Dobrý den, the hob does not heat."}
	out := mustCall(t, h, "create_ticket", args)
	if out["sent"] != false {
		t.Fatalf("preview = %v", out)
	}
	prev := out["preview"].(map[string]any)
	if prev["unit"] != "A101" || prev["area"] != "Other" || prev["category"] != "Fault" {
		t.Errorf("preview = %v, want the own flat, area Other, category Fault", prev)
	}
	if n := h.fake.WriteCount(); n != 0 {
		t.Fatalf("%d writes sent by a preview", n)
	}
	args["confirm"] = true
	out = mustCall(t, h, "create_ticket", args)
	if out["sent"] != true {
		t.Fatalf("confirmed = %v", out)
	}
	w := h.fake.LastWrite()
	if w.Path != "/tickets" || w.Body["categoryId"].(float64) != 2 || w.Body["areaId"].(float64) != 28 || w.Body["privateTicket"] != false {
		t.Errorf("sent %v %v", w.Path, w.Body)
	}
	rel := w.Body["relatedEntities"].([]any)
	if len(rel) != 1 || rel[0].(map[string]any)["entityType"] != "OBJECT" || rel[0].(map[string]any)["entityId"] != "701" {
		t.Errorf("related entities = %v, want only the flat", rel)
	}
	if w.Body["ticketPriority"] != nil {
		t.Errorf("priority = %v, want null as the app sends it", w.Body["ticketPriority"])
	}
}

func TestCreateTicketValidation(t *testing.T) {
	h := start(t)
	cases := []map[string]any{
		{"category": "ANNOUNCEMENT", "subject": "a", "description": "b"},
		{"category": "FAULT", "subject": " ", "description": "b"},
		{"category": "FAULT", "subject": "a", "description": "b", "unit": "Z999"},
		{"category": "FAULT", "subject": "a", "description": "b", "area": "nowhere"},
		{"category": "FAULT", "subject": strings.Repeat("x", 201), "description": "b"},
	}
	for _, c := range cases {
		c["confirm"] = true
		if _, errText := h.call(t, "create_ticket", c); errText == "" {
			t.Errorf("accepted %v", c)
		}
	}
	if n := h.fake.WriteCount(); n != 0 {
		t.Fatalf("%d writes sent for invalid tickets", n)
	}
	out := mustCall(t, h, "create_ticket", map[string]any{"category": "Question", "subject": "Cellar", "description": "Hi", "unit": "C01", "area": "equipment"})
	if p := out["preview"].(map[string]any); p["unit"] != "C01" || p["area"] != "Equipment" {
		t.Errorf("preview = %v", p)
	}
}

func TestCommentTicket(t *testing.T) {
	h := start(t)
	out := mustCall(t, h, "comment_ticket", map[string]any{"ticket_id": 3001, "message": "Friday is fine."})
	if out["sent"] != false || h.fake.WriteCount() != 0 {
		t.Fatalf("preview sent something: %v", out)
	}
	out = mustCall(t, h, "comment_ticket", map[string]any{"ticket_id": 3001, "message": "Friday is fine.", "confirm": true})
	if out["sent"] != true {
		t.Fatalf("confirmed = %v", out)
	}
	if w := h.fake.LastWrite(); w.Path != "/tickets/3001/comment" || w.Body["message"] != "Friday is fine." {
		t.Errorf("sent %v %v", w.Path, w.Body)
	}
	if _, errText := h.call(t, "comment_ticket", map[string]any{"ticket_id": 2000, "message": "x", "confirm": true}); !strings.Contains(errText, "does not take comments") {
		t.Errorf("comment on a closed conversation = %q", errText)
	}
}

func keys(m map[string]any) string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return strings.Join(ks, ",")
}

func TestWriteBodiesHaveTheAppsFields(t *testing.T) {
	h := start(t)
	mustCall(t, h, "create_ticket", map[string]any{"category": "FAULT", "subject": "s", "description": "d", "confirm": true})
	body := h.fake.LastWrite().Body
	if got := keys(body); got != "actions,areaId,attachments,categoryId,description,dueDate,privateTicket,relatedEntities,subject,ticketPriority" {
		t.Errorf("ticket body keys = %s", got)
	}
	if got := keys(body["relatedEntities"].([]any)[0].(map[string]any)); got != "entityId,entityType,label" {
		t.Errorf("related entity keys = %s", got)
	}
	mustCall(t, h, "comment_ticket", map[string]any{"ticket_id": 3001, "message": "m", "confirm": true})
	if got := keys(h.fake.LastWrite().Body); got != "attachments,message,setIgnoreMemberCountWarning" {
		t.Errorf("comment body keys = %s", got)
	}
}

func TestFailedWriteOutcome(t *testing.T) {
	h := start(t)
	args := map[string]any{"category": "FAULT", "subject": "s", "description": "d", "confirm": true}
	h.fake.Configure(func(s *fakeapi.Server) { s.FailWrites = 400 })
	if _, errText := h.call(t, "create_ticket", args); !strings.Contains(errText, "was not created") {
		t.Errorf("refused write = %q", errText)
	}
	h.fake.Configure(func(s *fakeapi.Server) { s.FailWrites = 502 })
	if _, errText := h.call(t, "create_ticket", args); !strings.Contains(errText, "outcome is unknown") {
		t.Errorf("failed write = %q, want an unknown outcome", errText)
	}
}

func TestAmbiguousUnitIsRefused(t *testing.T) {
	h := start(t)
	args := map[string]any{"category": "FAULT", "subject": "s", "description": "d", "unit": "P1"}
	if _, errText := h.call(t, "create_ticket", args); !strings.Contains(errText, "several units") {
		t.Errorf("ambiguous unit = %q", errText)
	}
	args["unit"] = "704"
	if p := mustCall(t, h, "create_ticket", args)["preview"].(map[string]any); p["unit_id"].(float64) != 704 {
		t.Errorf("preview = %v", p)
	}
}

func TestDownloadTextContent(t *testing.T) {
	h := start(t)
	res, err := h.session.CallTool(t.Context(), &mcp.CallToolParams{Name: "download_document", Arguments: map[string]any{"id": "doc-1"}})
	if err != nil || res.IsError || len(res.Content) == 0 {
		t.Fatalf("download: %v %v", err, res)
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(tc.Text, "saved_to") {
		t.Errorf("first content = %#v, want the result as text", res.Content[0])
	}
}

func TestCostsRangeFollowsTo(t *testing.T) {
	h := start(t)
	c := mustCall(t, h, "costs", map[string]any{"to": "2025-06-30"})
	if c["from"] != "2025-01-01" {
		t.Errorf("from = %v, want 1 January of the year of to", c["from"])
	}
}
