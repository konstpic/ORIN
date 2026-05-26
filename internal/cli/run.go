package cli

import (
	"context"

	"github.com/orin/orin/internal/config"
	"github.com/orin/orin/internal/runtime"
)

// RunAPIServer loads config and starts the API server.
func RunAPIServer(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return runtime.RunAPIServer(ctx, cfg)
}

// RunController loads config and starts the controller.
func RunController(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return runtime.RunController(ctx, cfg)
}

// RunRepoServer loads config and starts the gRPC repo server.
func RunRepoServer(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return runtime.RunRepoServer(ctx, cfg)
}

// RunAllInOne loads config and runs apiserver, controller, and reposerver in-process.
func RunAllInOne(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return runtime.RunAllInOne(ctx, cfg)
}
