// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"encoding/pem"
	"strings"
	"testing"

	"github.com/sigstore/cosign/v3/pkg/cosign"
	corev1 "k8s.io/api/core/v1"
)

const testKeyPassword = "s3cret-pw"

// genKey returns a cosign key pair, so the PEM types are checked against cosign itself.
func genKey(t *testing.T, password string) *cosign.KeysBytes {
	t.Helper()
	kb, err := cosign.GenerateKeyPair(func(bool) ([]byte, error) { return []byte(password), nil })
	if err != nil {
		t.Fatal(err)
	}

	return kb
}

func keySecret(key []byte, password string) *corev1.Secret {
	data := map[string][]byte{signingKeyField: key}
	if password != "" {
		data[signingPasswordField] = []byte(password)
	}

	return &corev1.Secret{Name: "key", Namespace: "ns", Data: data}
}

func TestValidateSigningKey(t *testing.T) {
	t.Parallel()
	withPw := genKey(t, testKeyPassword)

	if err := validateSigningKey(keySecret(withPw.PrivateBytes, testKeyPassword)); err != nil {
		t.Errorf("sigstore PEM type: %v", err)
	}

	// A key under the older cosign PEM type has to be accepted as well.
	blk, _ := pem.Decode(withPw.PrivateBytes)
	blk.Type = cosign.CosignPrivateKeyPemType
	if err := validateSigningKey(keySecret(pem.EncodeToMemory(blk), testKeyPassword)); err != nil {
		t.Errorf("cosign PEM type: %v", err)
	}

	for name, tc := range map[string]struct {
		secret *corev1.Secret
		leak   string // material that must not show up in the error
	}{
		"missing key": {secret: &corev1.Secret{}},
		"empty key":   {secret: keySecret([]byte{}, "")},
		"garbage":     {secret: keySecret([]byte("garbage-bytes"), testKeyPassword), leak: "garbage-bytes"},
		"public key":  {secret: keySecret(withPw.PublicBytes, testKeyPassword), leak: string(withPw.PublicBytes)},
	} {
		err := validateSigningKey(tc.secret)
		if err == nil {
			t.Errorf("%s: want error", name)

			continue
		}
		for _, leak := range []string{tc.leak, testKeyPassword, string(withPw.PrivateBytes)} {
			if leak != "" && strings.Contains(err.Error(), leak) {
				t.Errorf("%s: error leaks %q", name, leak)
			}
		}
	}
}

func TestSigningKeyTag(t *testing.T) {
	t.Parallel()
	a := keySecret(genKey(t, "").PrivateBytes, "pw")
	b := keySecret(genKey(t, "").PrivateBytes, "pw")

	if got := signingKeyTag(a); len(got) != 8 || got != signingKeyTag(a) {
		t.Errorf("tag %q must be 8 chars and stable", got)
	}
	if signingKeyTag(a) == signingKeyTag(b) {
		t.Error("different keys must have different tags")
	}
	if signingKeyTag(a) == signingKeyTag(keySecret(a.Data[signingKeyField], "other-pw")) {
		t.Error("a changed password must change the tag")
	}
}
