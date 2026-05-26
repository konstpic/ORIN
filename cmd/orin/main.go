// Command orin is the dev/MVP entry point (all-in-one + migrate). Production
// scaled deploys use dedicated binaries: orin-apiserver, orin-controller,
// orin-reposerver. Subcommands:
//
//	apiserver    HTTP + WebSocket gateway
//	controller   reconciliation loop
//	reposerver   git + manifest rendering
//	all-in-one   run all three in-process (MVP default)
//	migrate      run database migrations (up/down)
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/orin/orin/internal/cli"
	"github.com/orin/orin/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	root := &cobra.Command{
		Use:     "orin",
		Short:   "GitOps dashboard for Kubernetes",
		Version: config.Version,
		SilenceUsage: true,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	root.AddCommand(cli.NewAPIServerCmd(ctx))
	root.AddCommand(cli.NewControllerCmd(ctx))
	root.AddCommand(cli.NewRepoServerCmd(ctx))
	root.AddCommand(cli.NewAllInOneCmd(ctx))
	root.AddCommand(cli.NewMigrateCmd(ctx))

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
