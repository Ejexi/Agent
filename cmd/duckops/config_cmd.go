package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/SecDuckOps/agent/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current configuration",
	Long:  `Display the current DuckOps configuration loaded from ~/.duckops/config.toml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadTOML()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		dir, _ := config.DuckOpsDir()
		dbPath, _ := config.DatabasePath()

		fmt.Println("🦆 DuckOps Configuration")
		fmt.Println("════════════════════════════════")
		fmt.Printf("  Config:   %s/config.toml\n", dir)
		fmt.Printf("  Auth:     %s/auth.toml\n", dir)
		fmt.Printf("  Database: %s\n", dbPath)

		fmt.Printf("\n  Settings:\n")
		fmt.Printf("    editor:           %s\n", cfg.Settings.Editor)
		fmt.Printf("    server_addr:      %s\n", cfg.Settings.ServerAddr)
		fmt.Printf("    telemetry:        %v\n", cfg.Settings.CollectTelemetry)

		for name, profile := range cfg.Profiles {
			fmt.Printf("\n  Profile [%s]:\n", name)
			fmt.Printf("    api_endpoint: %s\n", profile.APIEndpoint)
			fmt.Printf("    provider:     %s\n", profile.Provider)

			if len(profile.Providers) > 0 {
				fmt.Println("    providers:")
				for pName, prov := range profile.Providers {
					keyDisplay := "(not set)"
					if prov.APIKey != "" {
						if len(prov.APIKey) > 8 {
							keyDisplay = prov.APIKey[:8] + "****"
						} else {
							keyDisplay = "****"
						}
					}
					fmt.Printf("      %s [%s]: key=%s\n", pName, prov.Type, keyDisplay)
					if prov.BaseURL != "" {
						fmt.Printf("        base_url: %s\n", prov.BaseURL)
					}
				}
			}

			if profile.Warden != nil {
				fmt.Printf("    warden: enabled=%v, volumes=%d\n", profile.Warden.Enabled, len(profile.Warden.Volumes))
			}
		}

		return nil
	},
}
