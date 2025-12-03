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

// Package main is the entrypoint for solar-index, the Kubernetes extension
// apiserver that stores catalog data, cluster registrations, and release
// desired state.
package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/component-base/cli"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"

	"github.com/opendefensecloud/solution-arsenal/internal/index/apiserver"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	cmd := NewSolarIndexCommand()
	code := cli.Run(cmd)
	os.Exit(code)
}

// NewSolarIndexCommand creates the solar-index command.
func NewSolarIndexCommand() *cobra.Command {
	opts := apiserver.NewOptions()

	cmd := &cobra.Command{
		Use:   "solar-index",
		Short: "Launch the Solar Index API server",
		Long: `solar-index is a Kubernetes extension API server that provides
the Solar catalog, cluster registration, and release management APIs.

Version: ` + version + `
Commit: ` + commit + `
Build Time: ` + buildTime,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(); err != nil {
				return err
			}
			if errs := opts.Validate(); len(errs) > 0 {
				return errs[0]
			}
			return runServer(opts)
		},
		SilenceUsage: true,
	}

	// Add flags.
	fs := cmd.Flags()
	opts.AddFlags(fs)

	// Add logging flags.
	logsapi.AddFlags(logsapi.NewLoggingConfiguration(), fs)

	// Normalize all flags.
	pflag.CommandLine.SetNormalizeFunc(pflag.CommandLine.GetNormalizeFunc())

	return cmd
}

func runServer(opts *apiserver.Options) error {
	config, err := opts.Config()
	if err != nil {
		return err
	}

	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	return server.GenericAPIServer.PrepareRun().Run(wait.NeverStop)
}
