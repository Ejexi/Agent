package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/SecDuckOps/agent/internal/config"
	"github.com/SecDuckOps/agent/internal/gui/setup"
	"github.com/SecDuckOps/shared/types"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup for DuckOps",
	Long:  `Launches a premium TUI to help you configure your DuckOps deployment.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadTOML()
		if err != nil {
			return types.Wrapf(err, types.ErrCodeInternal, "failed to load initial config")
		}

		// Run the interactive TUI
		results, err := setup.Run(cfg)
		if err != nil {
			return types.Wrapf(err, types.ErrCodeInternal, "setup failed")
		}

		if results == nil {
			fmt.Println("Setup cancelled.")
			os.Exit(0)
		}

		// Update profile
		profile := config.Profile{
			Provider:  results.Provider,
			Providers: make(map[string]config.Provider),
		}

		profile.Providers[results.Provider] = config.Provider{
			Type:   results.Provider,
			APIKey: results.APIKey,
		}

		if results.BaseURL != "" {
			p := profile.Providers[results.Provider]
			p.BaseURL = results.BaseURL
			profile.Providers[results.Provider] = p
		}

		cfg.SetProfile(results.ProfileName, profile)
		cfg.Settings.CollectTelemetry = results.Telemetry

		if err := cfg.SaveTOML(); err != nil {
			return types.Wrapf(err, types.ErrCodeInternal, "failed to save config")
		}

		fmt.Printf("\n✨ Profile '%s' configured successfully!\n", results.ProfileName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
