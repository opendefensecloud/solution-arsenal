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

package etcd

import (
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOptions(t *testing.T) {
	t.Parallel()

	opts := NewOptions()

	assert.Equal(t, []string{"localhost:2379"}, opts.Endpoints)
	assert.Equal(t, "/registry/solar.odc.io", opts.Prefix)
	assert.Equal(t, 5*time.Second, opts.DialTimeout)
	assert.Equal(t, 10*time.Second, opts.RequestTimeout)
	assert.Equal(t, 3, opts.MaxRetries)
	assert.Equal(t, 5*time.Minute, opts.CompactionInterval)
}

func TestOptions_AddFlags(t *testing.T) {
	t.Parallel()

	opts := NewOptions()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	opts.AddFlags(fs)

	// Verify flags are registered
	assert.NotNil(t, fs.Lookup("etcd-servers"))
	assert.NotNil(t, fs.Lookup("etcd-prefix"))
	assert.NotNil(t, fs.Lookup("etcd-certfile"))
	assert.NotNil(t, fs.Lookup("etcd-keyfile"))
	assert.NotNil(t, fs.Lookup("etcd-cafile"))
	assert.NotNil(t, fs.Lookup("etcd-dial-timeout"))
	assert.NotNil(t, fs.Lookup("etcd-request-timeout"))
	assert.NotNil(t, fs.Lookup("etcd-max-retries"))
	assert.NotNil(t, fs.Lookup("etcd-compaction-interval"))
}

func TestOptions_Validate_Valid(t *testing.T) {
	t.Parallel()

	opts := NewOptions()
	errs := opts.Validate()

	assert.Empty(t, errs)
}

func TestOptions_Validate_NoEndpoints(t *testing.T) {
	t.Parallel()

	opts := NewOptions()
	opts.Endpoints = []string{}
	errs := opts.Validate()

	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "at least one etcd endpoint")
}

func TestOptions_Validate_EmptyPrefix(t *testing.T) {
	t.Parallel()

	opts := NewOptions()
	opts.Prefix = ""
	errs := opts.Validate()

	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "prefix cannot be empty")
}

func TestOptions_Validate_PartialTLS(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		certFile string
		keyFile  string
		caFile   string
		errCount int
	}{
		{
			name:     "only cert",
			certFile: "/path/to/cert",
			errCount: 2, // missing key and ca
		},
		{
			name:    "only key",
			keyFile: "/path/to/key",
			errCount: 2, // missing cert and ca
		},
		{
			name:     "only ca",
			caFile:   "/path/to/ca",
			errCount: 2, // missing cert and key
		},
		{
			name:     "cert and key",
			certFile: "/path/to/cert",
			keyFile:  "/path/to/key",
			errCount: 1, // missing ca
		},
		{
			name:     "all TLS",
			certFile: "/path/to/cert",
			keyFile:  "/path/to/key",
			caFile:   "/path/to/ca",
			errCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opts := NewOptions()
			opts.CertFile = tc.certFile
			opts.KeyFile = tc.keyFile
			opts.CAFile = tc.caFile
			errs := opts.Validate()
			assert.Len(t, errs, tc.errCount)
		})
	}
}

func TestOptions_Validate_InvalidTimeouts(t *testing.T) {
	t.Parallel()

	opts := NewOptions()
	opts.DialTimeout = 0
	opts.RequestTimeout = -1
	errs := opts.Validate()

	assert.Len(t, errs, 2)
}

func TestOptions_TLSEnabled(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		certFile string
		keyFile  string
		caFile   string
		expected bool
	}{
		{
			name:     "no TLS",
			expected: false,
		},
		{
			name:     "partial TLS",
			certFile: "/path/to/cert",
			expected: false,
		},
		{
			name:     "full TLS",
			certFile: "/path/to/cert",
			keyFile:  "/path/to/key",
			caFile:   "/path/to/ca",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opts := NewOptions()
			opts.CertFile = tc.certFile
			opts.KeyFile = tc.keyFile
			opts.CAFile = tc.caFile
			assert.Equal(t, tc.expected, opts.TLSEnabled())
		})
	}
}
