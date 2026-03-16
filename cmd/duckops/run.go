package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/SecDuckOps/agent/internal/adapters/bootstrap"
	"github.com/SecDuckOps/agent/internal/config"
	shared_ports "github.com/SecDuckOps/shared/ports"
	"github.com/SecDuckOps/shared/types"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the interactive agent (without server)",
	Long:  `Start DuckOps in interactive mode with a REPL only (no HTTP server).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		tomlCfg, err := config.LoadTOML()
		if err != nil {
			return types.Wrap(err, types.ErrCodeInternal, "failed to load config")
		}

		app := bootstrap.FromTOML(context.Background(), tomlCfg)
		defer app.Shutdown()

		app.Logger.Info(context.Background(), "Starting interactive mode", shared_ports.Field{Key: "provider", Value: app.Provider})
		runInteractive(app.Kernel, app.Provider, "Stand Duck ")

		return nil
	},
}
