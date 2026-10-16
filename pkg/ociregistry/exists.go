// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ociregistry

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	ociname "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

// ManifestExists reports whether the manifest identified by rawRef (tag or digest
// reference) resolves. Only an HTTP 404 on the manifest itself yields (false, nil); every other failure,
// including a cancelled context, is returned as an error so callers can tell
// "missing" from "could not check".
func ManifestExists(ctx context.Context, rawRef string, auth authn.Authenticator, insecure bool) (bool, error) {
	parseOpts := []ociname.Option{}
	if insecure {
		parseOpts = append(parseOpts, ociname.Insecure)
	}

	ref, err := ociname.ParseReference(rawRef, parseOpts...)
	if err != nil {
		return false, fmt.Errorf("invalid OCI reference %q: %w", rawRef, err)
	}

	opts := []remote.Option{remote.WithContext(ctx)}
	if auth != nil && auth != authn.Anonymous {
		opts = append(opts, remote.WithAuth(auth))
	}

	if _, err := remote.Head(ref, opts...); err != nil {
		// Only a 404 on the manifest HEAD itself means "missing". A 404 from the
		// /v2/ ping or a token endpoint (e.g. a misrouted ingress) is an error.
		var terr *transport.Error
		if errors.As(err, &terr) && terr.StatusCode == http.StatusNotFound &&
			terr.Request != nil && terr.Request.Method == http.MethodHead {
			return false, nil
		}

		return false, fmt.Errorf("HEAD %s: %w", ref.String(), err)
	}

	return true, nil
}
