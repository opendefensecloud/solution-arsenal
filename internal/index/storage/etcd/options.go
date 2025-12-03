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

// Package etcd provides etcd-based storage for the solar-index API server.
package etcd

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
)

// Options contains configuration for etcd storage.
type Options struct {
	// Endpoints is a list of etcd server endpoints.
	Endpoints []string
	// Prefix is the key prefix for all solar resources.
	Prefix string
	// CertFile is the path to the client TLS certificate.
	CertFile string
	// KeyFile is the path to the client TLS key.
	KeyFile string
	// CAFile is the path to the CA certificate.
	CAFile string
	// DialTimeout is the timeout for establishing connections.
	DialTimeout time.Duration
	// RequestTimeout is the timeout for individual requests.
	RequestTimeout time.Duration
	// MaxRetries is the maximum number of retries for failed operations.
	MaxRetries int
	// CompactionInterval is the interval for etcd compaction.
	CompactionInterval time.Duration
}

// NewOptions creates new etcd options with defaults.
func NewOptions() *Options {
	return &Options{
		Endpoints:          []string{"localhost:2379"},
		Prefix:             "/registry/solar.odc.io",
		DialTimeout:        5 * time.Second,
		RequestTimeout:     10 * time.Second,
		MaxRetries:         3,
		CompactionInterval: 5 * time.Minute,
	}
}

// AddFlags adds etcd configuration flags to the given FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringSliceVar(&o.Endpoints, "etcd-servers", o.Endpoints,
		"List of etcd server endpoints (comma-separated)")
	fs.StringVar(&o.Prefix, "etcd-prefix", o.Prefix,
		"Prefix for all keys stored in etcd")
	fs.StringVar(&o.CertFile, "etcd-certfile", o.CertFile,
		"Path to the client TLS certificate file")
	fs.StringVar(&o.KeyFile, "etcd-keyfile", o.KeyFile,
		"Path to the client TLS key file")
	fs.StringVar(&o.CAFile, "etcd-cafile", o.CAFile,
		"Path to the CA certificate file")
	fs.DurationVar(&o.DialTimeout, "etcd-dial-timeout", o.DialTimeout,
		"Timeout for establishing etcd connections")
	fs.DurationVar(&o.RequestTimeout, "etcd-request-timeout", o.RequestTimeout,
		"Timeout for etcd requests")
	fs.IntVar(&o.MaxRetries, "etcd-max-retries", o.MaxRetries,
		"Maximum number of retries for failed etcd operations")
	fs.DurationVar(&o.CompactionInterval, "etcd-compaction-interval", o.CompactionInterval,
		"Interval between etcd compactions")
}

// Validate checks that the etcd options are valid.
func (o *Options) Validate() []error {
	var errs []error

	if len(o.Endpoints) == 0 {
		errs = append(errs, fmt.Errorf("at least one etcd endpoint is required"))
	}

	if o.Prefix == "" {
		errs = append(errs, fmt.Errorf("etcd prefix cannot be empty"))
	}

	// If TLS is partially configured, require all TLS settings
	hasTLS := o.CertFile != "" || o.KeyFile != "" || o.CAFile != ""
	if hasTLS {
		if o.CertFile == "" {
			errs = append(errs, fmt.Errorf("etcd-certfile is required when using TLS"))
		}
		if o.KeyFile == "" {
			errs = append(errs, fmt.Errorf("etcd-keyfile is required when using TLS"))
		}
		if o.CAFile == "" {
			errs = append(errs, fmt.Errorf("etcd-cafile is required when using TLS"))
		}
	}

	if o.DialTimeout <= 0 {
		errs = append(errs, fmt.Errorf("etcd-dial-timeout must be positive"))
	}

	if o.RequestTimeout <= 0 {
		errs = append(errs, fmt.Errorf("etcd-request-timeout must be positive"))
	}

	return errs
}

// TLSEnabled returns true if TLS is configured.
func (o *Options) TLSEnabled() bool {
	return o.CertFile != "" && o.KeyFile != "" && o.CAFile != ""
}
