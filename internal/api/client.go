// Package api is a client of the Bytové konto resident API (BeiT): OAuth2 password login with a
// cached, refreshed token pair, and typed calls of the /app endpoints the MCP tools use.
//
// The API is the one the official mobile and web apps use; it is not documented publicly. The
// client sends only requests those apps send.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBaseURL is the API host of the Bytové konto apps.
	DefaultBaseURL = "https://admin.bytovekonto.cz"
	// DefaultClientID is the OAuth2 client of the mobile app.
	DefaultClientID = "io.beit.consumer.mobile"

	tokenPath = "/api/oauth/token" //nolint:gosec // an endpoint path, not a credential
	appPrefix = "/app"

	// accessMargin is how long before its expiry an access token counts as expired.
	accessMargin = 30 * time.Second
	// defaultLoginBackoff is how long a refused password login blocks the next attempt, so a
	// wrong password does not hammer the login endpoint.
	defaultLoginBackoff = 5 * time.Minute
	defaultTimeout      = 30 * time.Second
	// maxErrorBody is how much of an error answer is read for its message.
	maxErrorBody = 64 << 10
	maxRedirects = 5
)

// Config configures a Client.
type Config struct {
	BaseURL  string // default DefaultBaseURL
	ClientID string // default DefaultClientID
	User     string // the login e-mail
	Password string
	// TokenFile caches the token pair between runs; "" keeps it in memory only.
	TokenFile string
	// TokenReadOnly uses a cached token but never writes the cache (a second process next to a
	// running server).
	TokenReadOnly bool
	HTTPClient    *http.Client
	Logger        *slog.Logger
	Now           func() time.Time
	LoginBackoff  time.Duration // default 5 minutes
}

// Client calls the API as one resident.
type Client struct {
	cfg   Config
	base  *url.URL
	http  *http.Client
	log   *slog.Logger
	store *tokenStore

	mu         sync.Mutex
	tok        token
	loaded     bool
	refusedAt  time.Time // last refused password login
	refusedErr error
}

// New returns a client. It does not contact the API.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.ClientID == "" {
		cfg.ClientID = DefaultClientID
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultTimeout, CheckRedirect: secureRedirect}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.LoginBackoff <= 0 {
		cfg.LoginBackoff = defaultLoginBackoff
	}
	base, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("base URL %q: not an absolute URL", cfg.BaseURL)
	}
	if base.Scheme != "https" && !isLoopback(base.Hostname()) {
		return nil, fmt.Errorf("base URL %q: https is required", cfg.BaseURL)
	}
	return &Client{
		cfg:   cfg,
		base:  base,
		http:  cfg.HTTPClient,
		log:   cfg.Logger,
		store: &tokenStore{path: cfg.TokenFile, readOnly: cfg.TokenReadOnly},
	}, nil
}

// secureRedirect refuses redirects to plain http, which would carry the bearer token or the login
// form in clear text, and long redirect chains.
func secureRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return errors.New("too many redirects")
	}
	if req.URL.Scheme != "https" && !isLoopback(req.URL.Hostname()) {
		return errors.New("refused a redirect to a non-https address")
	}
	return nil
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// Error is an error answer of the API.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("Bytové konto answered %d %s", e.Status, http.StatusText(e.Status))
	}
	return fmt.Sprintf("Bytové konto answered %d: %s", e.Status, e.Message)
}

// IsStatus reports whether err is an API answer with the status.
func IsStatus(err error, status int) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == status
}

// get calls GET /app<path> and decodes the JSON answer into out.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.call(ctx, http.MethodGet, path, query, nil, out)
}

// post calls POST /app<path> with a JSON body and decodes the JSON answer into out (may be nil).
func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return c.call(ctx, http.MethodPost, path, nil, body, out)
}

func (c *Client) call(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		payload = b
	}
	resp, err := c.send(ctx, method, path, query, payload, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		if errors.Is(err, io.EOF) {
			return nil // an empty answer leaves out as it is
		}
		return fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return nil
}

// send makes an authorised request and returns a 2xx response; the caller closes its body. A 401
// answer drops the access token and retries once with a new one.
func (c *Client) send(ctx context.Context, method, path string, query url.Values, payload []byte, contentType string) (*http.Response, error) {
	u := c.base.JoinPath(appPrefix, path)
	u.RawQuery = query.Encode()
	refused := ""
	for attempt := 0; ; attempt++ {
		access, err := c.accessToken(ctx, refused)
		if err != nil {
			return nil, err
		}
		var rd io.Reader
		if payload != nil {
			rd = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
		if err != nil {
			return nil, fmt.Errorf("request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", contentType)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", method, path, redactURLError(err))
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		apiErr := readError(resp)
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			c.log.Info("access token refused; getting a new one")
			refused = access
			continue
		}
		return nil, apiErr
	}
}

// jwtRE matches JSON Web Tokens: some OAuth2 servers echo the refused token in their message.
var jwtRE = regexp.MustCompile(`[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]*`)

func redactTokens(s string) string { return jwtRE.ReplaceAllString(s, "<token>") }

// redactURLError keeps the request URL out of transport errors: it may carry query values.
func redactURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

func readError(resp *http.Response) error {
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	e := &Error{Status: resp.StatusCode}
	defer func() { e.Message = redactTokens(e.Message) }()
	var body struct {
		Message          string `json:"message"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(b, &body) == nil {
		switch {
		case body.ErrorDescription != "":
			e.Message = body.ErrorDescription
		case body.Message != "":
			e.Message = body.Message
		default:
			e.Message = body.Error
		}
	}
	return e
}
