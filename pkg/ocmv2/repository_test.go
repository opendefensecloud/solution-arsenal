// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ocmv2

import (
	"context"
	"testing"

	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestAuthClientScopesBasicCredentials(t *testing.T) {
	const host = "registry.example.com:5000"
	client, err := authClient(host, &Credentials{Username: "user", Password: "password"})
	if err != nil {
		t.Fatal(err)
	}

	for _, target := range []string{host, "other.example.com:5000", "registry.example.com", "registry.example.com:5001"} {
		t.Run(target, func(t *testing.T) {
			got, err := client.Credential(context.Background(), target)
			if err != nil {
				t.Fatal(err)
			}
			want := auth.EmptyCredential
			if target == host {
				want = auth.Credential{Username: "user", Password: "password"}
			}
			if got != want {
				t.Fatal("credentials did not match the expected host scope")
			}
		})
	}
}
