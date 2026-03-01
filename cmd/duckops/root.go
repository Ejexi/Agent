package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/SecDuckOps/agent/internal/adapters/bootstrap"
	"github.com/SecDuckOps/agent/internal/adapters/server"
	"github.com/SecDuckOps/agent/internal/config"
	"github.com/SecDuckOps/shared/types"
)

// Build-time variables (set via -ldflags)
var (
	version = "Dev"
	commit  = "DuckOps - Team"
	date    = "25/02/2026"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "duckops",
	Short: "DuckOps — DevSecOps AI Agent",
	Long: `DuckOps is a standalone AI agent for DevSecOps.
It lives on your machine, keeps your apps running, 
and only pings when it needs a human.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		tomlCfg, err := config.LoadTOML()
		if err != nil {
			return types.Wrap(err, types.ErrCodeInternal, "failed to load config")
		}

		app := bootstrap.FromTOML(tomlCfg)

		// Start HTTP/SSE server in background
		addr := tomlCfg.Settings.ServerAddr
		if addr == "" {
			addr = ":8090" //api_endpoint
		}
		srv := server.NewAgentServer(app.Sessions, addr, app.Logger)
		go func() {
			app.Logger.Info(context.Background(), "system_event", fmt.Sprintf("Server listening on %s", addr))
			if err := srv.Start(); err != nil {
				app.Logger.ErrorErr(context.Background(), "operation_failed", err, "Server error")
			}
		}()

		// Run REPL in foreground
		runInteractive(app.Kernel, app.Provider, "standalone")
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.duckops/config.toml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(NewLogCmd())
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("duckops %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
	},
}
