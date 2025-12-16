package registry

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
)

type ZotRegistry struct {
	cmd        *exec.Cmd
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// Start starts the registry process in the background.
func NewRegistry(ctx context.Context) (*ZotRegistry, error) {
	zotPath := os.Getenv("ZOT")
	if zotPath == "" {
		log.Fatal("ZOT environment variable is not set.")
		return nil, errors.New("ZOT must be set")
	}

	zotConfigPath := os.Getenv("ZOT_CONFIG")
	if zotConfigPath == "" {
		log.Fatal("ZOT_CONFIG environment variable is not set.")
	}

	cctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, zotPath, "serve", zotConfigPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run process in the background
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
		return nil, err
	}

	// Wait for the command to finish in the background
	go func() {
		err := cmd.Wait()
		if err != nil {
			panic(fmt.Sprintf("command finished with error: %v", err))
		}
	}()
	return &ZotRegistry{ctx: cctx, cmd: cmd, cancelFunc: cancel}, nil
}

// Stop stops the registry process.
func (r *ZotRegistry) Stop() error {
	if r.ctx.Err() == nil {
		r.cancelFunc()
	}
	return nil
}
