package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

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
		// Catch interrupts for graceful shutdown
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		tomlCfg, err := config.LoadTOML()
		if err != nil {
			return types.Wrap(err, types.ErrCodeInternal, "failed to load config")
		}

		app, err := bootstrap.FromTOML(ctx, tomlCfg)
		if err != nil {
			return types.Newf(types.ErrCodeInternal, "failed to bootstrap application: %s", err.Error())
		}
		
		// Run in a goroutine so we can block on ctx.Done to run cleanup
		defer app.Shutdown()

		go func() {
			app.Logger.Info(ctx, "Starting interactive mode", shared_ports.Field{Key: "provider", Value: app.Provider})
			runInteractive(app.Kernel, app.Provider, "Stand Duck ")
			stop() // Triggers shutdown when loop exits normally
		}()

		<-ctx.Done()
		return nil

	},
}
