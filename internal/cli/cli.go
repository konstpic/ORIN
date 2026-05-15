// Package cli wires Cobra subcommands to subsystem entry points.
// Keeping wiring out of cmd/ keeps main.go small and makes it easy to
// test command construction.
package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/k8s-ui/k8s-ui/internal/config"
	"github.com/k8s-ui/k8s-ui/internal/runtime"
)

// NewAPIServerCmd returns the apiserver subcommand.
func NewAPIServerCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "apiserver",
		Short: "Run the API server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			return runtime.RunAPIServer(ctx, cfg)
		},
	}
}

// NewControllerCmd returns the controller subcommand.
func NewControllerCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "controller",
		Short: "Run the application controller (reconciler)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			return runtime.RunController(ctx, cfg)
		},
	}
}

// NewRepoServerCmd returns the reposerver subcommand.
func NewRepoServerCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "reposerver",
		Short: "Run the repo server (standalone HTTP, MVP usually in-process)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			return runtime.RunRepoServer(ctx, cfg)
		},
	}
}

// NewAllInOneCmd is the default MVP entry: apiserver+controller+reposerver
// in a single process.
func NewAllInOneCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "all-in-one",
		Short: "Run apiserver, controller, and reposerver in one process",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			return runtime.RunAllInOne(ctx, cfg)
		},
	}
}

// NewMigrateCmd handles golang-migrate up/down.
func NewMigrateCmd(ctx context.Context) *cobra.Command {
	c := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
	}
	c.AddCommand(&cobra.Command{
		Use: "up",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			slog.Info("running migrations up")
			return runtime.MigrateUp(ctx, cfg)
		},
	})
	c.AddCommand(&cobra.Command{
		Use: "down",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			slog.Info("running migrations down (one step)")
			return runtime.MigrateDown(ctx, cfg)
		},
	})
	c.AddCommand(&cobra.Command{
		Use: "status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			version, dirty, err := runtime.MigrateStatus(ctx, cfg)
			if err != nil {
				return err
			}
			fmt.Printf("version=%d dirty=%v\n", version, dirty)
			return nil
		},
	})
	return c
}
