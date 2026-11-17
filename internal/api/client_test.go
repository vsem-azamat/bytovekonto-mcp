package api_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vsem-azamat/bytovekonto-mcp/internal/api"
	"github.com/vsem-azamat/bytovekonto-mcp/internal/fakeapi"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newClient(t *testing.T, f *fakeapi.Server, mod func(*api.Config)) *api.Client {
	t.Helper()
	cfg := api.Config{BaseURL: f.URL, User: fakeapi.User, Password: fakeapi.Password, TokenFile: filepath.Join(t.TempDir(), "state", "token.json")}
	if mod != nil {
		mod(&cfg)
	}
	c, err := api.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestLoginCachesTokenPair(t *testing.T) {
	f := fakeapi.New()
	defer f.Close()
	file := filepath.Join(t.TempDir(), "state", "token.json")
	c := newClient(t, f, func(c *api.Config) { c.TokenFile = file })
	if _, err := c.HomeAccounts(t.Context()); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("token file mode %v, want 0600", st.Mode().Perm())
	}
	// a new process with the cache needs no password
	c2 := newClient(t, f, func(c *api.Config) { c.TokenFile = file; c.Password = "" })
	if _, err := c2.Tickets(t.Context()); err != nil {
		t.Fatal(err)
	}
	if logins, _ := f.Counts(); logins != 1 {
		t.Errorf("password logins = %d, want 1", logins)
	}
}

func TestExpiredAccessTokenIsRefreshed(t *testing.T) {
	f := fakeapi.New()
	defer f.Close()
	clk := &clock{t: time.Now()}
	c := newClient(t, f, func(c *api.Config) { c.Now = clk.now })
	if err := c.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	clk.t = clk.t.Add(11 * time.Minute)
	if _, err := c.HomeAccounts(t.Context()); err != nil {
		t.Fatal(err)
	}
	if logins, refreshes := f.Counts(); logins != 1 || refreshes != 1 {
		t.Errorf("logins %d refreshes %d, want 1 and 1", logins, refreshes)
	}
}

func TestRefusedRefreshFallsBackToPassword(t *testing.T) {
	f := fakeapi.New()
	defer f.Close()
	clk := &clock{t: time.Now()}
	c := newClient(t, f, func(c *api.Config) { c.Now = clk.now })
	if err := c.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	f.Configure(func(s *fakeapi.Server) { s.RefuseRefresh = true })
	clk.t = clk.t.Add(11 * time.Minute)
	if _, err := c.HomeAccounts(t.Context()); err != nil {
		t.Fatal(err)
	}
	if logins, _ := f.Counts(); logins != 2 {
		t.Errorf("password logins = %d, want 2", logins)
	}
}

func TestUnauthorizedCallRetriesOnce(t *testing.T) {
	f := fakeapi.New()
	defer f.Close()
	c := newClient(t, f, nil)
	if err := c.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	f.Expire()
	if _, err := c.HomeAccounts(t.Context()); err != nil {
		t.Fatalf("after a revoked access token: %v", err)
	}
	if _, refreshes := f.Counts(); refreshes != 1 {
		t.Errorf("refreshes = %d, want 1", refreshes)
	}
}

func TestRefusedPasswordBacksOff(t *testing.T) {
	f := fakeapi.New()
	defer f.Close()
	clk := &clock{t: time.Now()}
	c := newClient(t, f, func(c *api.Config) { c.Password = "wrong"; c.Now = clk.now })
	err := c.Login(t.Context())
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("login error = %v, want a refusal", err)
	}
	if strings.Contains(err.Error(), "wrong") {
		t.Fatal("the error contains the password")
	}
	if err := c.Login(t.Context()); err == nil || !strings.Contains(err.Error(), "next login attempt") {
		t.Fatalf("second login = %v, want the back-off", err)
	}
	clk.t = clk.t.Add(6 * time.Minute)
	_ = c.Login(t.Context())
	if logins, _ := f.Counts(); logins != 0 {
		t.Errorf("successful logins = %d, want 0", logins)
	}
}

func TestReadOnlyTokenStoreWritesNothing(t *testing.T) {
	f := fakeapi.New()
	defer f.Close()
	file := filepath.Join(t.TempDir(), "token.json")
	c := newClient(t, f, func(c *api.Config) { c.TokenFile = file; c.TokenReadOnly = true })
	if err := c.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("token file written in read-only mode: %v", err)
	}
}

func TestAPIErrorMessage(t *testing.T) {
	f := fakeapi.New()
	defer f.Close()
	c := newClient(t, f, nil)
	_, err := c.Payments(t.Context(), api.PaymentQuery{Filter: "BOGUS"})
	if !api.IsStatus(err, 400) || !strings.Contains(err.Error(), "filter") {
		t.Fatalf("error = %v, want a 400 naming the filter", err)
	}
	if _, err := c.Ticket(t.Context(), 1); !api.IsStatus(err, 404) {
		t.Fatalf("missing ticket error = %v, want 404", err)
	}
}

func TestPlainHTTPIsRefused(t *testing.T) {
	if _, err := api.New(api.Config{BaseURL: "http://example.com"}); err == nil {
		t.Fatal("plain http to a remote host was accepted")
	}
}

func TestForgetDeletesCache(t *testing.T) {
	f := fakeapi.New()
	defer f.Close()
	file := filepath.Join(t.TempDir(), "token.json")
	c := newClient(t, f, func(c *api.Config) { c.TokenFile = file })
	if err := c.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := c.Forget(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("token file still there: %v", err)
	}
}

func TestReadOnlyClientNeverRefreshes(t *testing.T) {
	f := fakeapi.New()
	defer f.Close()
	file := filepath.Join(t.TempDir(), "token.json")
	clk := &clock{t: time.Now()}
	srv := newClient(t, f, func(c *api.Config) { c.TokenFile = file; c.Now = clk.now })
	if err := srv.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	clk.t = clk.t.Add(11 * time.Minute)
	ro := newClient(t, f, func(c *api.Config) { c.TokenFile = file; c.TokenReadOnly = true; c.Now = clk.now })
	if _, err := ro.HomeAccounts(t.Context()); err != nil {
		t.Fatal(err)
	}
	// the server's cached refresh token still works
	srv2 := newClient(t, f, func(c *api.Config) { c.TokenFile = file; c.Password = ""; c.Now = clk.now })
	if _, err := srv2.HomeAccounts(t.Context()); err != nil {
		t.Fatalf("the server's refresh token was invalidated: %v", err)
	}
	if logins, refreshes := f.Counts(); logins != 2 || refreshes != 1 {
		t.Errorf("logins %d refreshes %d, want 2 (server, read-only) and 1 (server)", logins, refreshes)
	}
}

func TestParallelUnauthorizedCallsRefreshOnce(t *testing.T) {
	f := fakeapi.New()
	defer f.Close()
	c := newClient(t, f, nil)
	if err := c.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	f.Expire()
	errs := make(chan error, 8)
	for range 8 {
		go func() { _, err := c.HomeAccounts(t.Context()); errs <- err }()
	}
	for range 8 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if _, refreshes := f.Counts(); refreshes != 1 {
		t.Errorf("refreshes = %d, want 1", refreshes)
	}
}

func TestSecondUnauthorizedIsReturnedWithoutToken(t *testing.T) {
	f := fakeapi.New()
	defer f.Close()
	c := newClient(t, f, nil)
	f.Configure(func(s *fakeapi.Server) { s.AlwaysUnauthorized = true })
	_, err := c.HomeAccounts(t.Context())
	if !api.IsStatus(err, 401) {
		t.Fatalf("error = %v, want 401", err)
	}
	if strings.Contains(err.Error(), "eyJ") || !strings.Contains(err.Error(), "<token>") {
		t.Errorf("the token was not redacted: %v", err)
	}
	if _, refreshes := f.Counts(); refreshes != 1 {
		t.Errorf("refreshes = %d, want exactly one retry", refreshes)
	}
}

func TestBackOffEnds(t *testing.T) {
	f := fakeapi.New()
	defer f.Close()
	clk := &clock{t: time.Now()}
	c := newClient(t, f, func(c *api.Config) { c.Password = "wrong"; c.Now = clk.now })
	_ = c.Login(t.Context())
	_ = c.Login(t.Context())
	if n := f.FailedLoginCount(); n != 1 {
		t.Fatalf("attempts during the back-off: %d, want 1", n)
	}
	clk.t = clk.t.Add(6 * time.Minute)
	_ = c.Login(t.Context())
	if n := f.FailedLoginCount(); n != 2 {
		t.Errorf("attempts after the back-off: %d, want 2", n)
	}
}

func TestRedirectToPlainHTTPIsRefused(t *testing.T) {
	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.invalid/steal", http.StatusTemporaryRedirect)
	}))
	defer redir.Close()
	c, err := api.New(api.Config{BaseURL: redir.URL, User: fakeapi.User, Password: fakeapi.Password})
	if err != nil {
		t.Fatal(err)
	}
	err = c.Login(t.Context())
	if err == nil || !strings.Contains(err.Error(), "non-https") {
		t.Fatalf("login error = %v, want the redirect refused", err)
	}
}
