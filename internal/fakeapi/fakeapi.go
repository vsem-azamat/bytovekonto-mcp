// Package fakeapi is a fake Bytové konto API for tests: the OAuth2 token endpoint and the /app
// endpoints the client uses, serving made-up data and recording what the client sends.
package fakeapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Test credentials.
const (
	User     = "resident@example.com"
	Password = "correct horse battery staple"
)

// Created is a request the fake received on a write endpoint.
type Created struct {
	Path string
	Body map[string]any
}

// Server is a running fake API.
type Server struct {
	*httptest.Server

	mu sync.Mutex
	// AccessTTL is the lifetime of issued access tokens (default 10 minutes).
	AccessTTL time.Duration
	// RefuseRefresh makes the token endpoint refuse refresh grants.
	RefuseRefresh bool
	// Revoked makes the next /app call answer 401 once.
	Revoked bool
	// AlwaysUnauthorized makes every /app call answer 401, echoing the token like some OAuth2
	// servers do.
	AlwaysUnauthorized bool
	// FailWrites makes write requests answer this status (after recording them).
	FailWrites int
	// Now is the fake's clock for token expiry (default time.Now).
	Now func() time.Time

	PasswordLogins int
	FailedLogins   int
	Refreshes      int
	Calls          map[string]int
	Writes         []Created

	access  map[string]bool
	refresh map[string]bool
	seq     int
	nextID  int64
}

// New starts a fake API; close it with Close.
func New() *Server {
	s := &Server{
		AccessTTL: 10 * time.Minute, Now: time.Now, Calls: map[string]int{},
		access: map[string]bool{}, refresh: map[string]bool{}, nextID: 9000,
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.URL.Path == "/api/oauth/token" {
		s.token(w, r)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/app/") {
		http.NotFound(w, r)
		return
	}
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !s.access[tok] || s.Revoked || s.AlwaysUnauthorized {
		s.Revoked = false
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid_token", "error_description": "Invalid access token: " + tok})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/app")
	s.Calls[r.Method+" "+path]++
	if r.Method == http.MethodPost {
		s.write(w, r, path)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"message": "method not allowed"})
		return
	}
	if path == "/payments" {
		switch f := r.URL.Query().Get("filter"); f {
		case "", "TO_PAY", "OVERPAID":
		default:
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "Method parameter 'filter': Failed to convert value " + f})
			return
		}
	}
	if strings.HasPrefix(path, "/documents/myProject/download/") {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = io.WriteString(w, DocumentContent)
		return
	}
	body, ok := fixtures[path]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"status": 404, "error": "Not Found", "message": "No static resource " + path})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if err := r.ParseForm(); err != nil || q.Get("client_id") != "io.beit.consumer.mobile" || q.Get("response_type") != "token" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}
	switch q.Get("grant_type") {
	case "password":
		if q.Has("password") || q.Has("username") {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request", "error_description": "credentials in the URL"})
			return
		}
		if r.PostForm.Get("username") != User || r.PostForm.Get("password") != Password {
			s.FailedLogins++
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": "Bad credentials"})
			return
		}
		s.PasswordLogins++
	case "refresh_token":
		rt := r.PostForm.Get("refresh_token")
		if s.RefuseRefresh || !s.refresh[rt] {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": "Invalid refresh token"})
			return
		}
		delete(s.refresh, rt)
		s.Refreshes++
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported_grant_type"})
		return
	}
	now := s.Now()
	s.seq++
	access := jwt(fmt.Sprintf("access-%d", s.seq), now.Add(s.AccessTTL))
	refresh := jwt(fmt.Sprintf("refresh-%d", s.seq), now.Add(365*24*time.Hour))
	s.access[access], s.refresh[refresh] = true, true
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": access, "token_type": "bearer", "refresh_token": refresh,
		"expires_in": int(s.AccessTTL / time.Second), "scope": "consumer",
	})
}

// jwt makes an unsigned token with the claims the client reads.
func jwt(id string, exp time.Time) string {
	enc := base64.RawURLEncoding
	claims, _ := json.Marshal(map[string]any{"jti": id, "exp": exp.Unix()})
	return enc.EncodeToString([]byte(`{"alg":"none"}`)) + "." + enc.EncodeToString(claims) + ".sig"
}

func (s *Server) write(w http.ResponseWriter, r *http.Request, path string) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "malformed JSON"})
		return
	}
	s.Writes = append(s.Writes, Created{Path: path, Body: body})
	if s.FailWrites != 0 {
		writeJSON(w, s.FailWrites, map[string]any{"message": "write failed"})
		return
	}
	s.nextID++
	now := s.Now().UTC().Format(time.RFC3339Nano)
	switch {
	case path == "/tickets":
		writeJSON(w, http.StatusOK, map[string]any{
			"id": s.nextID, "createdAt": now, "modifiedAt": now, "subject": body["subject"], "description": body["description"],
			"ticketState": "NEW", "ticketPriority": "NORMAL", "ticketCategory": "Fault", "ticketArea": "Other",
			"tags":      []string{"USER_IS_CREATOR", "STATE_OPEN"},
			"relations": []map[string]any{{"entityType": "PROJECT", "entityId": "500", "label": "Example House"}, {"entityType": "OBJECT", "entityId": "701", "label": "A101"}},
		})
	case strings.HasPrefix(path, "/tickets/") && strings.HasSuffix(path, "/comment"):
		id, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(path, "/tickets/"), "/comment"), 10, 64)
		if err != nil || fixtures["/tickets/"+strconv.FormatInt(id, 10)] == "" {
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "ticket not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": s.nextID, "createdAt": now, "creatorSubjectId": 1, "creatorSubjectName": "Jana Example", "text": body["message"]})
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "not found"})
	}
}

// Configure changes the fake's settings (AccessTTL, RefuseRefresh, Revoked, Now) safely.
func (s *Server) Configure(f func(s *Server)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(s)
}

// Expire makes every issued access token unknown, as if they had expired on the server.
func (s *Server) Expire() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.access = map[string]bool{}
}

// WriteCount is the number of write requests received.
func (s *Server) WriteCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Writes)
}

// LastWrite is the last write request received.
func (s *Server) LastWrite() Created {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Writes[len(s.Writes)-1]
}

// Counts returns the login counters.
func (s *Server) Counts() (passwordLogins, refreshes int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.PasswordLogins, s.Refreshes
}

// FailedLoginCount is the number of refused password logins.
func (s *Server) FailedLoginCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.FailedLogins
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
