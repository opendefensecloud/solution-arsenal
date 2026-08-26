// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"bytes"
	"crypto"
	"encoding/base64"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/oci/mutate"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"

	testregistry "go.opendefense.cloud/solar/test/registry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// signTestKey is a cosign keypair written to disk for a single test.
type signTestKey struct {
	path     string
	password []byte
	public   []byte
}

func newSignTestKey(password string) signTestKey {
	GinkgoHelper()

	pass := []byte(password)
	keys, err := cosign.GenerateKeyPair(func(bool) ([]byte, error) { return pass, nil })
	Expect(err).NotTo(HaveOccurred())

	path := filepath.Join(GinkgoT().TempDir(), "cosign.key")
	Expect(os.WriteFile(path, keys.PrivateBytes, 0o600)).To(Succeed())

	return signTestKey{path: path, password: pass, public: keys.PublicBytes}
}

// pushTestImage pushes an empty image and returns its "oci://"-prefixed tag ref.
func pushTestImage(host, repo string) string {
	GinkgoHelper()

	ref, err := name.ParseReference(host+"/"+repo, name.Insecure)
	Expect(err).NotTo(HaveOccurred())
	Expect(remote.Write(ref, empty.Image, remote.WithContext(GinkgoT().Context()))).To(Succeed())

	return "oci://" + ref.String()
}

var _ = Describe("SignChart", func() {
	var (
		host string
		key  signTestKey
	)

	BeforeEach(func() {
		srv := httptest.NewServer(testregistry.New().HandleFunc())
		DeferCleanup(srv.Close)
		host = srv.Listener.Addr().String()
		key = newSignTestKey("passw0rd")
	})

	It("pushes a verifiable signature to the same repository", func() {
		ref := pushTestImage(host, "charts/signed:v1.0.0")

		Expect(SignChart(SignOptions{
			Reference:     ref,
			KeyPath:       key.path,
			KeyPassword:   key.password,
			NameOptions:   []name.Option{name.Insecure},
			RemoteOptions: []remote.Option{remote.WithContext(GinkgoT().Context())},
		})).To(Succeed())

		// The signature must live in the same repository, tagged sha256-<digest>.sig.
		digest := resolveDigest(ref)
		remoteOpts := ociremote.WithRemoteOptions(remote.WithContext(GinkgoT().Context()))

		sigTag, err := ociremote.SignatureTag(digest, remoteOpts)
		Expect(err).NotTo(HaveOccurred())
		Expect(sigTag.Context().String()).To(Equal(digest.Context().String()))

		sigs, err := ociremote.Signatures(sigTag, remoteOpts)
		Expect(err).NotTo(HaveOccurred())

		list, err := sigs.Get()
		Expect(err).NotTo(HaveOccurred())
		Expect(list).To(HaveLen(1))

		// The signature must verify against the public half of the keypair.
		payload, err := list[0].Payload()
		Expect(err).NotTo(HaveOccurred())

		b64sig, err := list[0].Base64Signature()
		Expect(err).NotTo(HaveOccurred())

		raw, err := base64.StdEncoding.DecodeString(b64sig)
		Expect(err).NotTo(HaveOccurred())

		pub, err := cryptoutils.UnmarshalPEMToPublicKey(key.public)
		Expect(err).NotTo(HaveOccurred())

		verifier, err := signature.LoadVerifier(pub, crypto.SHA256)
		Expect(err).NotTo(HaveOccurred())
		Expect(verifier.VerifySignature(bytes.NewReader(raw), bytes.NewReader(payload))).To(Succeed())
	})

	It("fails when the key file does not exist", func() {
		ref := pushTestImage(host, "charts/missing-key:v1.0.0")

		err := SignChart(SignOptions{
			Reference:     ref,
			KeyPath:       filepath.Join(GinkgoT().TempDir(), "absent.key"),
			KeyPassword:   key.password,
			NameOptions:   []name.Option{name.Insecure},
			RemoteOptions: []remote.Option{remote.WithContext(GinkgoT().Context())},
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to read signing key"))
	})

	It("fails when the key file is not a cosign key", func() {
		ref := pushTestImage(host, "charts/invalid-key:v1.0.0")

		invalid := filepath.Join(GinkgoT().TempDir(), "invalid.key")
		Expect(os.WriteFile(invalid, []byte("not a pem encoded key"), 0o600)).To(Succeed())

		err := SignChart(SignOptions{
			Reference:     ref,
			KeyPath:       invalid,
			KeyPassword:   key.password,
			NameOptions:   []name.Option{name.Insecure},
			RemoteOptions: []remote.Option{remote.WithContext(GinkgoT().Context())},
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to load signing key"))
	})

	It("fails when the key password is wrong", func() {
		ref := pushTestImage(host, "charts/wrong-pass:v1.0.0")

		err := SignChart(SignOptions{
			Reference:     ref,
			KeyPath:       key.path,
			KeyPassword:   []byte("wrong"),
			NameOptions:   []name.Option{name.Insecure},
			RemoteOptions: []remote.Option{remote.WithContext(GinkgoT().Context())},
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to load signing key"))
	})

	It("fails when the artifact is not present in the registry", func() {
		err := SignChart(SignOptions{
			Reference:     "oci://" + host + "/charts/never-pushed:v1.0.0",
			KeyPath:       key.path,
			KeyPassword:   key.password,
			NameOptions:   []name.Option{name.Insecure},
			RemoteOptions: []remote.Option{remote.WithContext(GinkgoT().Context())},
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to resolve"))
	})

	It("fails when the key path is empty", func() {
		err := SignChart(SignOptions{Reference: "oci://" + host + "/charts/no-key:v1.0.0"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("signing key path is required"))
	})

	It("fails when the reference is empty", func() {
		err := SignChart(SignOptions{KeyPath: key.path, KeyPassword: key.password})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("registry reference is required"))
	})
})

// resolveDigest turns an "oci://host/repo:tag" reference into its digest reference.
func resolveDigest(ref string) name.Digest {
	GinkgoHelper()

	parsed, err := name.ParseReference(strings.TrimPrefix(ref, "oci://"), name.Insecure)
	Expect(err).NotTo(HaveOccurred())

	desc, err := remote.Get(parsed, remote.WithContext(GinkgoT().Context()))
	Expect(err).NotTo(HaveOccurred())

	return parsed.Context().Digest(desc.Digest.String())
}

var _ = Describe("SignatureExists", func() {
	var (
		host string
		key  signTestKey
	)

	BeforeEach(func() {
		srv := httptest.NewServer(testregistry.New().HandleFunc())
		DeferCleanup(srv.Close)
		host = srv.Listener.Addr().String()
		key = newSignTestKey("passw0rd")
	})

	optsFor := func(ref string, k signTestKey) SignOptions {
		return SignOptions{
			Reference:     ref,
			KeyPath:       k.path,
			KeyPassword:   k.password,
			NameOptions:   []name.Option{name.Insecure},
			RemoteOptions: []remote.Option{remote.WithContext(GinkgoT().Context())},
		}
	}

	It("reports true for an artifact signed with the same key", func() {
		ref := pushTestImage(host, "charts/dedup:v1.0.0")
		Expect(SignChart(optsFor(ref, key))).To(Succeed())

		Expect(SignatureExists(optsFor(ref, key))).To(BeTrue())
	})

	It("reports false for an artifact signed with a different key", func() {
		ref := pushTestImage(host, "charts/dedup:v1.0.0")
		Expect(SignChart(optsFor(ref, key))).To(Succeed())

		other := newSignTestKey("passw0rd")
		Expect(SignatureExists(optsFor(ref, other))).To(BeFalse())
	})

	It("reports false for an unsigned artifact", func() {
		ref := pushTestImage(host, "charts/unsigned:v1.0.0")

		Expect(SignatureExists(optsFor(ref, key))).To(BeFalse())
	})

	It("reports false when the artifact is not in the registry", func() {
		Expect(SignatureExists(optsFor("oci://"+host+"/charts/absent:v1.0.0", key))).To(BeFalse())
	})

	It("reports false when the signature payload claims a different artifact", func() {
		// A signature this key made for another artifact, re-attached to this
		// artifact's .sig tag, must not count as this artifact being signed.
		signed := pushTestImage(host, "charts/mixed:v1.0.0")
		Expect(SignChart(optsFor(signed, key))).To(Succeed())

		victim := pushRandomImage(host, "charts/mixed:v2.0.0")
		relocateSignature(resolveDigest(signed), resolveDigest(victim))

		Expect(SignatureExists(optsFor(victim, key))).To(BeFalse())
	})
})

// pushRandomImage pushes an image with content distinct from the empty image.
func pushRandomImage(host, repo string) string {
	GinkgoHelper()

	img, err := random.Image(256, 1)
	Expect(err).NotTo(HaveOccurred())

	ref, err := name.ParseReference(host+"/"+repo, name.Insecure)
	Expect(err).NotTo(HaveOccurred())
	Expect(remote.Write(ref, img, remote.WithContext(GinkgoT().Context()))).To(Succeed())

	return "oci://" + ref.String()
}

// relocateSignature copies the signature attached to from onto to's .sig tag,
// leaving the payload pointing at the original artifact.
func relocateSignature(from, to name.Digest) {
	GinkgoHelper()

	opts := ociremote.WithRemoteOptions(remote.WithContext(GinkgoT().Context()))

	sigTag, err := ociremote.SignatureTag(from, opts)
	Expect(err).NotTo(HaveOccurred())

	sigs, err := ociremote.Signatures(sigTag, opts)
	Expect(err).NotTo(HaveOccurred())

	list, err := sigs.Get()
	Expect(err).NotTo(HaveOccurred())
	Expect(list).To(HaveLen(1))

	entity, err := ociremote.SignedEntity(to, opts)
	Expect(err).NotTo(HaveOccurred())

	relocated, err := mutate.AttachSignatureToEntity(entity, list[0])
	Expect(err).NotTo(HaveOccurred())

	Expect(ociremote.WriteSignatures(to.Repository, relocated, opts)).To(Succeed())
}
