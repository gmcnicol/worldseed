package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gmcnicol/worldseed/internal/daemon"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var dataDir string
	cmd := &cobra.Command{
		Use:   "worldseedd",
		Short: "Run a local universe daemon",
	}
	cmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "worldseed data directory")
	cmd.AddCommand(startCmd(&dataDir))
	return cmd
}

func startCmd(dataDir *string) *cobra.Command {
	var universeName string
	var addr string
	var tickInterval time.Duration
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the daemon for one universe shard",
		RunE: func(cmd *cobra.Command, args []string) error {
			if universeName == "" {
				return fmt.Errorf("--universe is required")
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
			return daemon.Start(ctx, daemon.StartOptions{
				DataDir:      *dataDir,
				UniverseName: universeName,
				Addr:         addr,
				TickInterval: tickInterval,
				Logger:       logger,
			})
		},
	}
	cmd.Flags().StringVar(&universeName, "universe", "", "universe name to load")
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:27411", "SSH listen address")
	cmd.Flags().DurationVar(&tickInterval, "tick", 5*time.Second, "simulation tick interval")
	return cmd
}
