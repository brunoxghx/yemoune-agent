package cli

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/brunoxghx/yemoune-agent/internal/config"
	"github.com/spf13/cobra"
)

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check agent health and server connectivity",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		fmt.Printf("Agent ID: %s\n", cfg.Agent.ID)
		fmt.Printf("Server URL: %s\n", cfg.Agent.ServerURL)

		// Check server connectivity
		fmt.Printf("Checking server connectivity...\n")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", cfg.Agent.ServerURL+"/health", nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("❌ Server unreachable: %v\n", err)
			return nil
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			fmt.Printf("✓ Server is reachable (status: %d)\n", resp.StatusCode)
		} else {
			fmt.Printf("⚠ Server returned status: %d\n", resp.StatusCode)
		}

		fmt.Println("\nAgent health: OK")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}
