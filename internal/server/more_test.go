package server_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vsem-azamat/bytovekonto-mcp/internal/fakeapi"
)

func TestPaymentQR(t *testing.T) {
	h := start(t)
	res, err := h.session.CallTool(t.Context(), &mcp.CallToolParams{Name: "payment_qr", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("payment_qr: %v %v", err, res)
	}
	if got := h.fake.QRRequestList(); len(got) != 1 || len(got[0]) != 1 || got[0][0].(float64) != 7001 {
		t.Errorf("QR requests = %v, want the document still to pay", got)
	}
	var img *mcp.ImageContent
	for _, c := range res.Content {
		if ic, ok := c.(*mcp.ImageContent); ok {
			img = ic
		}
	}
	png, _ := base64.StdEncoding.DecodeString(fakeapi.QRPNG)
	if img == nil || img.MIMEType != "image/png" || string(img.Data) != string(png) {
		t.Fatalf("no PNG image in %v", res.Content)
	}
	out := mustCall(t, h, "payment_qr", map[string]any{"ids": []int{7001}})
	code := out["codes"].([]any)[0].(map[string]any)
	if code["amount"].(float64) != 25000 || code["currency"] != "CZK" {
		t.Errorf("code = %v", code)
	}
	saved, _ := code["saved_to"].(string)
	if filepath.Dir(saved) != h.dir {
		t.Errorf("saved to %q, want the download directory", saved)
	}
	if _, errText := h.call(t, "payment_qr", map[string]any{"ids": []int{1}}); !strings.Contains(errText, "no QR code") {
		t.Errorf("unknown document = %q", errText)
	}
	if h.fake.WriteCount() != 0 {
		t.Error("a QR code request counted as a write")
	}
}

func TestCompareCosts(t *testing.T) {
	h := start(t)
	out := mustCall(t, h, "compare_costs", nil)
	if out["from"] != "2026-02-01" || out["to"] != "2026-02-28" {
		t.Errorf("range = %v – %v, want last month", out["from"], out["to"])
	}
	var heat map[string]any
	for _, d := range out["deliveries"].([]any) {
		if m := d.(map[string]any); m["category"] == "HEAT" {
			heat = m
		}
	}
	if heat == nil {
		t.Fatalf("no heat in %v", out["deliveries"])
	}
	cw := heat["compared_with"].(map[string]any)
	if prev := cw["previous_period"].(map[string]any); prev["cost"].(float64) != 40.5 {
		t.Errorf("previous period = %v", prev)
	}
	if ya := cw["year_ago"].(map[string]any); ya["available"] != false || ya["note"] == "" {
		t.Errorf("year ago = %v", ya)
	}
	if _, ok := cw["building_average"]; !ok {
		t.Errorf("no building average in %v", cw)
	}
	out = mustCall(t, h, "compare_costs", map[string]any{"from": "2026-01-01"})
	if out["to"] != "2026-01-31" {
		t.Errorf("to = %v, want the end of the month of from", out["to"])
	}
}

// pngBytes is a tiny valid PNG.
func pngBytes(t *testing.T) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(fakeapi.QRPNG)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAttachments(t *testing.T) {
	h := start(t)
	png := pngBytes(t)
	inline := map[string]any{"name": "../hob.png", "content_base64": base64.StdEncoding.EncodeToString(png)}
	out := mustCall(t, h, "comment_ticket", map[string]any{"ticket_id": 3001, "message": "Photo attached.", "attachments": []any{inline}})
	att := out["preview"].(map[string]any)["attachments"].([]any)
	if len(att) != 1 || att[0].(map[string]any)["name"] != "hob.png" || att[0].(map[string]any)["type"] != "image/png" {
		t.Fatalf("preview attachments = %v", att)
	}
	if h.fake.WriteCount() != 0 {
		t.Fatal("the preview sent something")
	}
	mustCall(t, h, "comment_ticket", map[string]any{"ticket_id": 3001, "message": "Photo attached.", "attachments": []any{inline}, "confirm": true})
	sent := h.fake.LastWrite().Body["attachments"].([]any)
	if len(sent) != 1 || sent[0].(map[string]any)["fileName"] != "hob.png" || sent[0].(map[string]any)["content"] != base64.StdEncoding.EncodeToString(png) {
		t.Errorf("sent attachments = %v", sent)
	}

	// by path, inside the allowed directory
	inside := filepath.Join(h.attach, "plate.png")
	if err := os.WriteFile(inside, png, 0o600); err != nil {
		t.Fatal(err)
	}
	mustCall(t, h, "create_ticket", map[string]any{"category": "FAULT", "subject": "Hob", "description": "d", "attachments": []any{map[string]any{"path": inside}}, "confirm": true})
	if sent := h.fake.LastWrite().Body["attachments"].([]any); len(sent) != 1 || sent[0].(map[string]any)["fileName"] != "plate.png" {
		t.Errorf("sent attachments = %v", sent)
	}

	outside := filepath.Join(t.TempDir(), "secret.png")
	if err := os.WriteFile(outside, png, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(h.attach, "link.png")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	text := filepath.Join(h.attach, "notes.txt")
	if err := os.WriteFile(text, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(h.attach, "pipe.png")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	writes := h.fake.WriteCount()
	notImage := base64.StdEncoding.EncodeToString([]byte("not an image at all"))
	bad := []struct {
		a    map[string]any
		want string
	}{
		{map[string]any{"path": outside}, "not inside"},
		{map[string]any{"path": link}, "cannot read"},
		{map[string]any{"path": "plate.png"}, "absolute"},
		{map[string]any{"path": text}, "only photos"},
		{map[string]any{"path": text, "name": "notes.heic"}, "only photos"},
		{map[string]any{"name": "fake.heic", "content_base64": notImage}, "only photos"},
		{map[string]any{"path": fifo}, "not a regular file"},
		{map[string]any{"content_base64": notImage}, "name is required"},
		{map[string]any{"name": "x.png", "content_base64": "%%%"}, "not standard base64"},
		{map[string]any{"path": inside, "name": "x.png", "content_base64": notImage}, "not both"},
	}
	for _, c := range bad {
		_, errText := h.call(t, "comment_ticket", map[string]any{"ticket_id": 3001, "message": "x", "attachments": []any{c.a}, "confirm": true})
		if !strings.Contains(errText, c.want) {
			t.Errorf("attachment %v: error %q, want %q", c.a, errText, c.want)
		}
	}
	heic := append([]byte{0, 0, 0, 24}, []byte("ftypheic\x00\x00\x00\x00mif1heic")...)
	out = mustCall(t, h, "comment_ticket", map[string]any{"ticket_id": 3001, "message": "HEIC", "attachments": []any{map[string]any{"name": "IMG_1.HEIC", "content_base64": base64.StdEncoding.EncodeToString(heic)}}})
	if typ := out["preview"].(map[string]any)["attachments"].([]any)[0].(map[string]any)["type"]; typ != "image/heic" {
		t.Errorf("HEIC type = %v", typ)
	}
	six := []any{inline, inline, inline, inline, inline, inline}
	if _, errText := h.call(t, "comment_ticket", map[string]any{"ticket_id": 3001, "message": "x", "attachments": six, "confirm": true}); !strings.Contains(errText, "at most") {
		t.Errorf("six attachments = %q", errText)
	}
	if h.fake.WriteCount() != writes {
		t.Error("a refused attachment was sent")
	}
}

func TestDuplicateTicketIsRefused(t *testing.T) {
	h := start(t)
	created := testNow.Add(-5 * time.Minute).Format(time.RFC3339)
	h.fake.SetFixture("/tickets/list-with-last-comment", `[{"id":3002,"createdAt":"`+created+`","subject":"Broken hob ","ticketState":"NEW","tags":["USER_IS_CREATOR"]}]`)
	args := map[string]any{"category": "FAULT", "subject": "broken hob", "description": "d"}
	out := mustCall(t, h, "create_ticket", args)
	if w, _ := out["preview"].(map[string]any)["warning"].(string); !strings.Contains(w, "3002") {
		t.Errorf("preview warning = %q", w)
	}
	args["confirm"] = true
	if _, errText := h.call(t, "create_ticket", args); !strings.Contains(errText, "3002") {
		t.Errorf("duplicate = %q, want it refused", errText)
	}
	if h.fake.WriteCount() != 0 {
		t.Fatal("the duplicate was sent")
	}
	// an older ticket with the same subject does not block
	old := testNow.Add(-2 * time.Hour).Format(time.RFC3339)
	h.fake.SetFixture("/tickets/list-with-last-comment", `[{"id":3002,"createdAt":"`+old+`","subject":"Broken hob","ticketState":"NEW","tags":["USER_IS_CREATOR"]}]`)
	mustCall(t, h, "create_ticket", args)
	if h.fake.WriteCount() != 1 {
		t.Error("a ticket like an old one was refused")
	}
}

func TestDuplicateCommentIsRefused(t *testing.T) {
	h := start(t)
	at := testNow.Add(-time.Minute).Format(time.RFC3339)
	h.fake.SetFixture("/tickets/3001/lifecycle", `{"comments":[{"id":9,"createdAt":"`+at+`","creatorSubjectId":1,"creatorSubjectName":"Jana Example","text":"Friday is fine.","deletedAt":null}],"history":[]}`)
	args := map[string]any{"ticket_id": 3001, "message": " Friday is fine.", "confirm": true}
	if _, errText := h.call(t, "comment_ticket", args); !strings.Contains(errText, "same message") {
		t.Errorf("duplicate = %q", errText)
	}
	args["message"] = "Friday is fine, thanks."
	mustCall(t, h, "comment_ticket", args)
	if h.fake.WriteCount() != 1 {
		t.Errorf("writes = %d, want only the different message", h.fake.WriteCount())
	}
	// the same words from someone else do not block the user's answer
	h.fake.SetFixture("/tickets/3001/lifecycle", `{"comments":[{"id":9,"createdAt":"`+at+`","creatorSubjectId":21,"creatorSubjectName":"Tom Technician","text":"Thanks.","deletedAt":null}],"history":[]}`)
	args["message"] = "Thanks."
	mustCall(t, h, "comment_ticket", args)
	if h.fake.WriteCount() != 2 {
		t.Error("the user's answer was refused because someone else wrote the same")
	}
}
