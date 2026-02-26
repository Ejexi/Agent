package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/SecDuckOps/agent/internal/adapters/auth"
	"github.com/SecDuckOps/shared/types"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Store API key for DuckOps",
	Long:  `Save your API key to ~/.duckops/auth.toml for authenticated requests.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		apiKey, _ := cmd.Flags().GetString("api-key")
		if apiKey == "" {
			return types.New(types.ErrCodeInvalidInput, "--api-key is required")
		}

		store, err := auth.NewStore("")
		if err != nil {
			return types.Wrap(err, types.ErrCodeInternal, "failed to initialize auth store")
		}

		if err := store.SaveAPIKey(apiKey); err != nil {
			return types.Wrap(err, types.ErrCodeInternal, "failed to save API key")
		}

		fmt.Println("✅ API key saved successfully to ~/.duckops/auth.toml")
		return nil
	},
}

func init() {
	loginCmd.Flags().String("api-key", "", "API key to store")
}
