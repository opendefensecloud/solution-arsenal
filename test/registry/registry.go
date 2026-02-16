// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"encoding/base64"
	"net/http"

	"github.com/google/go-containerregistry/pkg/registry"
)

// Registry represents a Docker registry.
type Registry struct {
	wantedAuthHeader      string
	dockerRegistryHandler http.Handler
}

// New returns a new Registry.
func New(opts ...registry.Option) *Registry {
	r := &Registry{}
	r.dockerRegistryHandler = registry.New(opts...)

	return r
}

// HandleFunc returns a http.Handler that handles requests to the registry.
func (r *Registry) HandleFunc() http.Handler {
	return http.HandlerFunc(r.handle)
}

// handle handles requests to the registry.
func (r *Registry) handle(w http.ResponseWriter, req *http.Request) {
	if r.wantedAuthHeader != "" && req.Header.Get("Authorization") != r.wantedAuthHeader {
		w.Header().Set("Www-Authenticate", `Basic realm="Test Server"`)
		w.WriteHeader(http.StatusUnauthorized)
	}
	r.dockerRegistryHandler.ServeHTTP(w, req)
}

// WithAuth sets the wanted auth header for the registry.
func (r *Registry) WithAuth(username string, password string) *Registry {
	r.wantedAuthHeader = "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	return r
}
