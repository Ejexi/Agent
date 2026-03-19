package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/SecDuckOps/agent/internal/adapters/bootstrap"
	"github.com/SecDuckOps/agent/internal/adapters/server"
	"github.com/SecDuckOps/agent/internal/config"
	shared_ports "github.com/SecDuckOps/shared/ports"
	"github.com/SecDuckOps/shared/types"
)

var serveAddr string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP/SSE API server only (no REPL)",
	Long:  `Start the DuckOps agent server for managing subagent sessions via REST API and SSE streaming.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		tomlCfg, err := config.LoadTOML()
		if err != nil {
			return types.Wrap(err, types.ErrCodeInternal, "failed to load config")
		}

		app, err := bootstrap.FromTOML(context.Background(), tomlCfg)
		if err != nil {
			return fmt.Errorf("failed to initialize agent: %w", err)
		}
		defer app.Shutdown()

		addr := serveAddr
		if addr == "" {
			addr = tomlCfg.Settings.ServerAddr
		}
		if addr == "" {
			addr = ":8090"
		}

		fmt.Printf("🦆 DuckOps Agent Server starting on %s\n", addr)

		srv := server.NewAgentServer(app.Sessions, addr, app.Logger)

		// Graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		errCh := make(chan error, 1)
		go func() {
			errCh <- srv.Start()
		}()

		select {
		case sig := <-sigChan:
			app.Logger.Info(context.Background(), "Shutting down server...", shared_ports.Field{Key: "signal", Value: sig})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return srv.Stop(ctx)
		case err := <-errCh:
			return err
		}
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", "", "server listen address (default: from config.toml)")
}
