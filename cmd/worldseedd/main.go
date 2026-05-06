package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/worldseed/worldseed/internal/daemon"
	"github.com/worldseed/worldseed/internal/migrations"
	"github.com/worldseed/worldseed/internal/storage"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var rootDir, universeName string
	var tick time.Duration
	cmd := &cobra.Command{
		Use:   "worldseedd",
		Short: "Universe daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, _, err := storage.OpenUniverseDB(rootDir, universeName)
			if err != nil {
				return err
			}
			if err := migrations.Apply(db); err != nil {
				_ = db.Close()
				return err
			}
			_ = db.Close()
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return daemon.Run(ctx, daemon.Config{RootDir: rootDir, UniverseName: universeName, TickInterval: tick})
		},
	}
	cmd.Flags().StringVar(&rootDir, "root-dir", "/var/lib/worldseed/universes", "Universe root directory")
	cmd.Flags().StringVar(&universeName, "universe", "veyr-node", "Universe name")
	cmd.Flags().DurationVar(&tick, "tick", 5*time.Second, "Simulation tick interval")
	return cmd
}
