package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gmcnicol/worldseed/internal/daemon"
	"github.com/gmcnicol/worldseed/internal/universe"
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
		Use:   "worldseed",
		Short: "Maintain local universe archive nodes",
	}
	cmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "worldseed data directory")
	cmd.AddCommand(universeCmd(&dataDir), connectCmd())
	return cmd
}

func universeCmd(dataDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "universe",
		Short: "Manage local universe shards",
	}
	var seed int64
	create := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a deterministic universe archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := universe.Create(cmd.Context(), universe.CreateOptions{
				DataDir: *dataDir,
				Name:    args[0],
				Seed:    seed,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created universe %q at %s\n", args[0], path)
			return nil
		},
	}
	create.Flags().Int64Var(&seed, "seed", 0, "explicit deterministic universe seed")
	cmd.AddCommand(create)
	return cmd
}

func connectCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect to a local worldseedd SSH archive session",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			return daemon.Connect(ctx, daemon.ConnectOptions{Addr: addr})
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:27411", "worldseedd SSH address")
	return cmd
}
