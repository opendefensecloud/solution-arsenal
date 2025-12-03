/*
Copyright 2024 Open Defense Cloud Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package oci

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// BasicAuthenticator provides basic authentication.
type BasicAuthenticator struct {
	username string
	password string
}

// NewBasicAuthenticator creates a new basic authenticator.
func NewBasicAuthenticator(username, password string) *BasicAuthenticator {
	return &BasicAuthenticator{
		username: username,
		password: password,
	}
}

// Authenticate adds basic auth to the request.
func (a *BasicAuthenticator) Authenticate(req *http.Request) error {
	auth := base64.StdEncoding.EncodeToString([]byte(a.username + ":" + a.password))
	req.Header.Set("Authorization", "Basic "+auth)
	return nil
}

// BearerAuthenticator provides bearer token authentication.
type BearerAuthenticator struct {
	token string
}

// NewBearerAuthenticator creates a new bearer authenticator.
func NewBearerAuthenticator(token string) *BearerAuthenticator {
	return &BearerAuthenticator{token: token}
}

// Authenticate adds bearer token to the request.
func (a *BearerAuthenticator) Authenticate(req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+a.token)
	return nil
}

// TokenAuthenticator handles OAuth2 token authentication with automatic refresh.
type TokenAuthenticator struct {
	mu           sync.RWMutex
	realm        string
	service      string
	scope        string
	username     string
	password     string
	token        string
	expiry       time.Time
	httpClient   *http.Client
}

// TokenAuthenticatorOption configures a TokenAuthenticator.
type TokenAuthenticatorOption func(*TokenAuthenticator)

// WithCredentials sets the username and password for token exchange.
func WithCredentials(username, password string) TokenAuthenticatorOption {
	return func(a *TokenAuthenticator) {
		a.username = username
		a.password = password
	}
}

// NewTokenAuthenticator creates a new token authenticator.
func NewTokenAuthenticator(realm, service, scope string, opts ...TokenAuthenticatorOption) *TokenAuthenticator {
	a := &TokenAuthenticator{
		realm:      realm,
		service:    service,
		scope:      scope,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Authenticate adds bearer token to the request, refreshing if needed.
func (a *TokenAuthenticator) Authenticate(req *http.Request) error {
	token, err := a.getToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func (a *TokenAuthenticator) getToken() (string, error) {
	a.mu.RLock()
	if a.token != "" && time.Now().Before(a.expiry) {
		token := a.token
		a.mu.RUnlock()
		return token, nil
	}
	a.mu.RUnlock()

	return a.refreshToken()
}

func (a *TokenAuthenticator) refreshToken() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Double-check after acquiring write lock
	if a.token != "" && time.Now().Before(a.expiry) {
		return a.token, nil
	}

	// Build token request URL
	url := fmt.Sprintf("%s?service=%s&scope=%s", a.realm, a.service, a.scope)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating token request: %w", err)
	}

	// Add basic auth if credentials are provided
	if a.username != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(a.username + ":" + a.password))
		req.Header.Set("Authorization", "Basic "+auth)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	var tokenResp struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}

	// Use token or access_token field
	token := tokenResp.Token
	if token == "" {
		token = tokenResp.AccessToken
	}

	if token == "" {
		return "", fmt.Errorf("no token in response")
	}

	a.token = token
	// Default expiry to 60 seconds before actual expiry, or 5 minutes if not specified
	expiresIn := tokenResp.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 300
	}
	a.expiry = time.Now().Add(time.Duration(expiresIn-60) * time.Second)

	return a.token, nil
}

// DockerConfigAuthenticator reads credentials from Docker config.
type DockerConfigAuthenticator struct {
	registry string
	config   *DockerConfig
}

// DockerConfig represents the Docker config.json structure.
type DockerConfig struct {
	Auths       map[string]DockerAuthEntry `json:"auths"`
	CredHelpers map[string]string          `json:"credHelpers"`
}

// DockerAuthEntry represents a single auth entry.
type DockerAuthEntry struct {
	Auth     string `json:"auth"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// NewDockerConfigAuthenticator creates an authenticator from Docker config.
func NewDockerConfigAuthenticator(registry string) (*DockerConfigAuthenticator, error) {
	config, err := loadDockerConfig()
	if err != nil {
		return nil, err
	}

	return &DockerConfigAuthenticator{
		registry: normalizeRegistryForAuth(registry),
		config:   config,
	}, nil
}

// Authenticate adds auth from Docker config to the request.
func (a *DockerConfigAuthenticator) Authenticate(req *http.Request) error {
	entry, ok := a.config.Auths[a.registry]
	if !ok {
		// Try without scheme
		for key, auth := range a.config.Auths {
			if normalizeRegistryForAuth(key) == a.registry {
				entry = auth
				ok = true
				break
			}
		}
	}

	if !ok {
		return nil // No auth configured for this registry
	}

	// Use auth field if present (base64 encoded username:password)
	if entry.Auth != "" {
		req.Header.Set("Authorization", "Basic "+entry.Auth)
		return nil
	}

	// Use username/password if present
	if entry.Username != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(entry.Username + ":" + entry.Password))
		req.Header.Set("Authorization", "Basic "+auth)
		return nil
	}

	return nil
}

func loadDockerConfig() (*DockerConfig, error) {
	// Try standard Docker config locations
	paths := []string{
		filepath.Join(os.Getenv("HOME"), ".docker", "config.json"),
		filepath.Join(os.Getenv("DOCKER_CONFIG"), "config.json"),
	}

	for _, path := range paths {
		if path == "" {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var config DockerConfig
		if err := json.Unmarshal(data, &config); err != nil {
			continue
		}

		return &config, nil
	}

	// Return empty config if no file found
	return &DockerConfig{
		Auths:       make(map[string]DockerAuthEntry),
		CredHelpers: make(map[string]string),
	}, nil
}

func normalizeRegistryForAuth(registry string) string {
	// Remove scheme
	registry = strings.TrimPrefix(registry, "https://")
	registry = strings.TrimPrefix(registry, "http://")

	// Handle docker.io special case
	if registry == "docker.io" || registry == "index.docker.io" {
		return "https://index.docker.io/v1/"
	}

	return registry
}

// ChainAuthenticator tries multiple authenticators in order.
type ChainAuthenticator struct {
	authenticators []Authenticator
}

// NewChainAuthenticator creates a chain of authenticators.
func NewChainAuthenticator(authenticators ...Authenticator) *ChainAuthenticator {
	return &ChainAuthenticator{authenticators: authenticators}
}

// Authenticate tries each authenticator until one succeeds.
func (a *ChainAuthenticator) Authenticate(req *http.Request) error {
	for _, auth := range a.authenticators {
		if err := auth.Authenticate(req); err == nil {
			return nil
		}
	}
	return nil // Allow unauthenticated request if all fail
}

// AnonymousAuthenticator provides no authentication (for public registries).
type AnonymousAuthenticator struct{}

// NewAnonymousAuthenticator creates an anonymous authenticator.
func NewAnonymousAuthenticator() *AnonymousAuthenticator {
	return &AnonymousAuthenticator{}
}

// Authenticate does nothing for anonymous access.
func (a *AnonymousAuthenticator) Authenticate(req *http.Request) error {
	return nil
}
