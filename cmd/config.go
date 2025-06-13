package cmd

import (
	"fmt"
	"os"

	"github.com/longkey1/llmc/internal/llmc"
	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Display current configuration",
	Long: `Display the current configuration values.
This command shows all configuration values loaded from the config file and environment variables.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration from file
		config, err := llmc.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Display configuration values
		fmt.Printf("Provider: %s\n", config.Provider)
		fmt.Printf("BaseURL: %s\n", config.BaseURL)
		fmt.Printf("Model: %s\n", config.Model)
		fmt.Printf("Token: %s\n", maskToken(config.Token))
		fmt.Printf("PromptDirectory: %s\n", config.PromptDir)
	},
}

// maskToken returns a masked version of the token for security
func maskToken(token string) string {
	if len(token) <= 8 {
		return "********"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func init() {
	rootCmd.AddCommand(configCmd)
}
