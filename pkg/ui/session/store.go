// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	cookieName      = "solar-session"
	stateCookieName = "solar-oidc-state" //nolint:gosec // not a credential
)

// Data holds session data.
type Data struct {
	Username    string   `json:"username"`
	Groups      []string `json:"groups"`
	IDToken     string   `json:"id_token,omitempty"`     //nolint:gosec // not a hardcoded credential
	AccessToken string   `json:"access_token,omitempty"` //nolint:gosec // not a hardcoded credential

	// ImpersonatingAs is set when an admin is previewing as another user.
	// The BE will forward K8s requests with Impersonate-User headers.
	ImpersonatingAs     string   `json:"impersonating_as,omitempty"`
	ImpersonatingGroups []string `json:"impersonating_groups,omitempty"`
}

// Store manages encrypted cookie-based sessions.
// For the MVP / spike, this uses a simple in-memory map keyed by session ID.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Data
	key      []byte // unused for now, reserved for cookie encryption
}

// NewStore creates a new session store.
func NewStore(hexKey string) (*Store, error) {
	var key []byte
	if hexKey != "" {
		var err error
		key, err = hex.DecodeString(hexKey)
		if err != nil {
			return nil, fmt.Errorf("invalid session key: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("session key must be 32 bytes (64 hex chars), got %d", len(key))
		}
	} else {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("failed to generate session key: %w", err)
		}
	}

	return &Store{
		sessions: make(map[string]*Data),
		key:      key,
	}, nil
}

// Get retrieves session data from the request.
func (s *Store) Get(r *http.Request) *Data {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.sessions[cookie.Value]
}

// Set stores session data and sets the cookie.
func (s *Store) Set(w http.ResponseWriter, data *Data) {
	id := generateSessionID()

	s.mu.Lock()
	s.sessions[id] = data
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(24 * time.Hour / time.Second),
	})
}

// SetImpersonation updates ImpersonatingAs/Groups in the existing session in-place.
func (s *Store) SetImpersonation(r *http.Request, username string, groups []string) bool {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if sess, ok := s.sessions[cookie.Value]; ok {
		sess.ImpersonatingAs = username
		sess.ImpersonatingGroups = groups

		return true
	}

	return false
}

// ClearImpersonation removes the impersonation override from the existing session.
func (s *Store) ClearImpersonation(r *http.Request) bool {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if sess, ok := s.sessions[cookie.Value]; ok {
		sess.ImpersonatingAs = ""
		sess.ImpersonatingGroups = nil

		return true
	}

	return false
}

// Clear deletes the session from the server-side store and removes the cookie.
func (s *Store) Clear(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(cookieName); err == nil {
		s.mu.Lock()
		delete(s.sessions, cookie.Value)
		s.mu.Unlock()
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1,
	})
}

// SetState stores the OIDC state parameter in a short-lived cookie.
func (s *Store) SetState(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/api/auth/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300, // 5 minutes
	})
}

// GetState retrieves the OIDC state parameter from the cookie.
func (s *Store) GetState(r *http.Request) string {
	cookie, err := r.Cookie(stateCookieName)
	if err != nil {
		return ""
	}

	return cookie.Value
}

// ClearState removes the OIDC state cookie.
func (s *Store) ClearState(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/api/auth/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1,
	})
}

// GetJSON writes session data as JSON to the writer.
func (s *Store) GetJSON(w http.ResponseWriter, r *http.Request) {
	data := s.Get(r)
	if data == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"authenticated":false}`))

		return
	}

	w.Header().Set("Content-Type", "application/json")

	resp := map[string]any{
		"authenticated": true,
		"username":      data.Username,
		"groups":        data.Groups,
	}
	if data.ImpersonatingAs != "" {
		resp["impersonating"] = map[string]any{
			"username": data.ImpersonatingAs,
			"groups":   data.ImpersonatingGroups,
		}
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("failed to encode session JSON: %v", err)
	}
}

func generateSessionID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	return hex.EncodeToString(b)
}
