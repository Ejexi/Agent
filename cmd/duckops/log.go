package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/SecDuckOps/agent/internal/adapters/audit"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/spf13/cobra"
)

func NewLogCmd() *cobra.Command {
	var sessionID string
	var limit int

	cmd := &cobra.Command{
		Use:   "log",
		Short: "View agent audit logs",
		Long:  "Displays the audit logs recorded by the agent. You can filter by session ID.",
		Run: func(cmd *cobra.Command, args []string) {
			logger, err := audit.New("", "")
			if err != nil {
				fmt.Printf("Failed to initialize audit logger: %v\n", err)
				os.Exit(1)
			}
			defer logger.Close()

			filter := ports.AuditFilter{
				SessionID: sessionID,
				Limit:     limit,
			}

			entries, err := logger.Query(context.Background(), filter)
			if err != nil {
				fmt.Printf("Failed to query logs: %v\n", err)
				os.Exit(1)
			}

			if len(entries) == 0 {
				fmt.Println("No audit logs found.")
				return
			}

			for _, entry := range entries {
				details, _ := json.Marshal(entry.Details)
				fmt.Printf("[%s] %s | Session: %s | Action: %s | Target: %s | Details: %s\n",
					entry.Timestamp.Format("2006-01-02 15:04:05"),
					entry.Actor,
					entry.SessionID,
					entry.Action,
					entry.Target,
					string(details),
				)
			}
		},
	}

	cmd.Flags().StringVarP(&sessionID, "session", "s", "", "Filter logs by session ID")
	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "Limit the number of log entries returned")

	return cmd
}
