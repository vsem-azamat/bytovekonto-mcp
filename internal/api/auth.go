package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// token is the cached OAuth2 token pair. The access token lives minutes, the refresh token about
// a year; the password is needed only when both are gone.
type token struct {
	User           string    `json:"user"`
	Access         string    `json:"access_token,omitempty"`
	AccessExpires  time.Time `json:"access_expires,omitzero"`
	Refresh        string    `json:"refresh_token,omitempty"`
	RefreshExpires time.Time `json:"refresh_expires,omitzero"`
}

// accessToken returns a valid access token: the cached one, a refreshed one, or one from a
// password login. refused is an access token the API just refused: it is dropped if it is
// still the cached one (another call may already have replaced it).
func (c *Client) accessToken(ctx context.Context, refused string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded {
		c.tok = c.store.load(c.cfg.User)
		c.loaded = true
	}
	now := c.cfg.Now()
	if refused != "" && refused == c.tok.Access {
		c.tok.Access, c.tok.AccessExpires = "", time.Time{}
	}
	if c.tok.Access != "" && now.Add(accessMargin).Before(c.tok.AccessExpires) {
		return c.tok.Access, nil
	}
	// a read-only client never refreshes: the API may rotate the refresh token, which would
	// invalidate the one the running server has cached
	if !c.cfg.TokenReadOnly && c.tok.Refresh != "" && (c.tok.RefreshExpires.IsZero() || now.Before(c.tok.RefreshExpires)) {
		err := c.grant(ctx, url.Values{"grant_type": {"refresh_token"}}, url.Values{"refresh_token": {c.tok.Refresh}})
		if err == nil {
			return c.tok.Access, nil
		}
		if !isAuthRefusal(err) {
			return "", err
		}
		c.log.Info("refresh token refused; logging in with the password")
		c.tok.Refresh, c.tok.RefreshExpires = "", time.Time{}
	}
	if !c.refusedAt.IsZero() && now.Sub(c.refusedAt) < c.cfg.LoginBackoff {
		return "", fmt.Errorf("%w (the next login attempt is allowed at %s)", c.refusedErr, c.refusedAt.Add(c.cfg.LoginBackoff).Format(time.RFC3339))
	}
	if c.cfg.Password == "" {
		return "", errors.New("not logged in and no password configured")
	}
	err := c.grant(ctx, url.Values{"grant_type": {"password"}}, url.Values{"username": {c.cfg.User}, "password": {c.cfg.Password}})
	if err != nil {
		if isAuthRefusal(err) {
			c.refusedAt = now
			c.refusedErr = fmt.Errorf("the Bytové konto login was refused: %w", err)
			c.log.Warn("password login refused", "err", err.Error())
			return "", c.refusedErr
		}
		return "", err
	}
	c.refusedAt, c.refusedErr = time.Time{}, nil
	c.log.Info("logged in with the password")
	return c.tok.Access, nil
}

// isAuthRefusal reports whether the token endpoint refused the credentials (rather than failing).
func isAuthRefusal(err error) bool {
	return IsStatus(err, http.StatusBadRequest) || IsStatus(err, http.StatusUnauthorized) || IsStatus(err, http.StatusForbidden)
}

type grantAnswer struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// grant calls the token endpoint. The grant type and client go in the query, as the apps send
// them; credentials go in the form body so they never appear in a URL. Called with c.mu held.
func (c *Client) grant(ctx context.Context, query, form url.Values) error {
	query.Set("client_id", c.cfg.ClientID)
	query.Set("response_type", "token")
	u := c.base.JoinPath(tokenPath)
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("token request: %w", redactURLError(err))
	}
	if resp.StatusCode != http.StatusOK {
		return readError(resp)
	}
	defer resp.Body.Close()
	var a grantAnswer
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		return fmt.Errorf("token answer: %w", err)
	}
	if a.AccessToken == "" || !strings.EqualFold(a.TokenType, "bearer") {
		return errors.New("token answer: no bearer access token")
	}
	now := c.cfg.Now()
	t := token{User: c.cfg.User, Access: a.AccessToken, AccessExpires: now.Add(time.Duration(a.ExpiresIn) * time.Second)}
	if exp, ok := jwtExpiry(a.AccessToken); ok {
		t.AccessExpires = exp
	}
	t.Refresh = a.RefreshToken
	if t.Refresh == "" {
		t.Refresh = c.tok.Refresh // a refresh may keep the old refresh token
		t.RefreshExpires = c.tok.RefreshExpires
	} else if exp, ok := jwtExpiry(t.Refresh); ok {
		t.RefreshExpires = exp
	}
	c.tok = t
	if err := c.store.save(t); err != nil {
		c.log.Warn("token not cached", "err", err.Error())
	}
	return nil
}

// jwtExpiry reads the exp claim of a JSON Web Token without verifying it: the client only uses
// it to know when to refresh.
func jwtExpiry(tok string) (time.Time, bool) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(b, &claims) != nil || claims.Exp <= 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}

// Login makes sure the client has a valid access token, logging in if needed.
func (c *Client) Login(ctx context.Context) error {
	_, err := c.accessToken(ctx, "")
	return err
}

// Forget deletes the cached token pair. The API has no logout endpoint for the apps; the refresh
// token stays valid on the server until it expires.
func (c *Client) Forget() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tok = token{User: c.cfg.User}
	c.loaded = true
	return c.store.remove()
}
