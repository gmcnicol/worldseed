package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/worldseed/worldseed/internal/migrations"
	"github.com/worldseed/worldseed/internal/storage"
	"github.com/worldseed/worldseed/internal/tui"
	"github.com/worldseed/worldseed/internal/universe"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var rootDir string
	cmd := &cobra.Command{Use: "worldseed"}
	cmd.PersistentFlags().StringVar(&rootDir, "root-dir", "/var/lib/worldseed/universes", "Universe root directory")

	cmd.AddCommand(&cobra.Command{
		Use:   "init [universe-name]",
		Args:  cobra.ExactArgs(1),
		Short: "Create a seeded universe",
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := os.MkdirAll(filepath.Join(rootDir, name), 0o755); err != nil {
				return err
			}
			db, path, err := storage.OpenUniverseDB(rootDir, name)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := migrations.Apply(db); err != nil {
				return err
			}
			svc := universe.NewService(db)
			if err := svc.Create(context.Background(), universe.CreateInput{ID: name, Name: name, Seed: 42, EntropyProfile: "baseline"}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Universe %q initialized at %s\n", name, path)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "connect [universe-name]",
		Args:  cobra.ExactArgs(1),
		Short: "Open archive dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(args[0])
		},
	})

	return cmd
}
