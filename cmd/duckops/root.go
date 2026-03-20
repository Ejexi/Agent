package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/SecDuckOps/agent/internal/adapters/bootstrap"
	"github.com/SecDuckOps/agent/internal/adapters/server"
	"github.com/SecDuckOps/agent/internal/config"
)

// Build-time variables (set via -ldflags)
var (
	version = "Dev"
	commit  = "DuckOps - Team"
	date    = "25/02/2026"
)

var (
	cfgFile       string
	verbose       bool
	cliMode       bool
	scanMode      bool
	resumeSession string // --resume <session-id or "latest">
	outputFormat  string // --format text|json|sarif
)

// Exit codes per Phase 3 spec
const (
	exitOK          = 0 // clean — no findings at/above fail_on severity
	exitFindings    = 1 // findings found at/above fail_on severity
	exitDockerError = 2 // Docker unavailable or scan crashed
	exitConfigError = 3 // config error
)

var rootCmd = &cobra.Command{
	Use:          "duckops",
	Short:        "DuckOps — DevSecOps AI Agent",
	Long:         "DuckOps is a DevSecOps AI agent. Just type what you want — it figures out the rest.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Load config — exit(3) on failure
		tomlCfg, err := config.LoadTOML()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
			os.Exit(exitConfigError)
		}

		// 2. Bootstrap — exit(2) if init fails
		app, err := bootstrap.FromTOML(context.Background(), tomlCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Initialization error: %v\n", err)
			os.Exit(exitDockerError)
		}
		defer app.Shutdown()

		// 3. Background warm-up (non-blocking — does not delay the prompt)
		if app.DuckOpsAgent != nil {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				defer cancel()
				app.DuckOpsAgent.WarmupAll(ctx)
			}()
		}

		// 4. Decide mode
		if scanMode {
			// Conversational scan agent REPL — apply --format flag
			app.DuckOpsAgent.WithFormat(outputFormat).Run(context.Background())
			return nil
		}

		if cliMode {
			// Legacy LLM REPL
			runInteractive(app.Kernel, app.Provider, "DuckOps")
			return nil
		}

		// Default: HTTP server in background + TUI in foreground
		addr := tomlCfg.Settings.ServerAddr
		if addr == "" {
			addr = ":8090"
		}
		srv := server.NewAgentServer(app.Sessions, addr, app.Logger)
		if app.CheckpointStore != nil {
			srv.WithCheckpointStore(app.CheckpointStore)
		}
		go func() {
			app.Logger.Info(context.Background(), fmt.Sprintf("Server listening on %s", addr))
			if err := srv.Start(); err != nil {
				app.Logger.ErrorErr(context.Background(), err, "Server error")
			}
		}()

		actualModel := app.Model
		if actualModel == "" && app.Kernel != nil && app.Kernel.Deps.LLM != nil {
			if defaultLLM := app.Kernel.Deps.LLM.Get(app.Provider); defaultLLM != nil {
				actualModel = defaultLLM.Model()
			} else if defaultLLM = app.Kernel.Deps.LLM.Default(); defaultLLM != nil {
				actualModel = defaultLLM.Model()
			}
		}

		// If --resume was passed, open the session browser or resume directly
		if resumeSession != "" && app.CheckpointStore != nil {
			if resumeSession == "latest" {
				// Resolve to the most recent session ID
				sessions, err := app.CheckpointStore.ListSessions()
				if err == nil && len(sessions) > 0 {
					resumeSession = sessions[0].SessionID
				}
			}
			// Pass the resume target via the TUI — it will load on start
			_ = resumeSession // picked up by openSessionBrowser / resumeSession flow in TUI
		}

		runTUI(app.Kernel, actualModel, app.AppSessions, app.Sessions, app.EventBus, app.SkillRegistry, app.CheckpointStore)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.duckops/config.toml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.Flags().BoolVar(&cliMode, "cli", false, "launch the LLM REPL")
	rootCmd.Flags().BoolVar(&scanMode, "scan", false, "launch the conversational scan agent")
	rootCmd.Flags().StringVarP(&resumeSession, "resume", "r", "", "resume a session by ID, or 'latest' for the most recent")
	rootCmd.Flags().StringVarP(&outputFormat, "format", "f", "text", "output format: text, json, sarif")

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
