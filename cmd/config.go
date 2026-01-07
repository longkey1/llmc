package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/longkey1/llmc/internal/llmc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config [field]",
	Short: "Display current configuration",
	Long: `Display the current configuration values.
This command shows all configuration values loaded from the config file and environment variables.

If a field name is specified, only that field's value is displayed.
Available fields: configfile, provider, baseurl, model, token, promptdirs, websearch

Examples:
  llmc config                    # Show all configuration
  llmc config provider          # Show only provider
  llmc config model             # Show only model
  llmc config promptdirs        # Show only prompt directories
  llmc config websearch         # Show only web search setting`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration from file
		config, err := llmc.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// If a field is specified, show only that field
		if len(args) > 0 {
			field := strings.ToLower(args[0])
			switch field {
			case "configfile":
				fmt.Println(viper.ConfigFileUsed())
			case "provider":
				fmt.Println(config.Provider)
			case "baseurl":
				fmt.Println(config.BaseURL)
			case "model":
				fmt.Println(config.Model)
			case "token":
				fmt.Println(maskToken(config.Token))
			case "promptdirs":
				// PromptDirs are already absolute paths
				fmt.Println(strings.Join(config.PromptDirs, ","))
			case "websearch":
				fmt.Println(config.EnableWebSearch)
			default:
				fmt.Fprintf(os.Stderr, "Unknown field: %s\n", args[0])
				fmt.Fprintf(os.Stderr, "Available fields: configfile, provider, baseurl, model, token, promptdirs, websearch\n")
				os.Exit(1)
			}
			return
		}

		// Display all configuration values
		fmt.Printf("ConfigFile: %s\n", viper.ConfigFileUsed())
		fmt.Printf("Provider: %s\n", config.Provider)
		fmt.Printf("BaseURL: %s\n", config.BaseURL)
		fmt.Printf("Model: %s\n", config.Model)
		fmt.Printf("Token: %s\n", maskToken(config.Token))
		// PromptDirs are already absolute paths
		fmt.Printf("PromptDirectories: %s\n", strings.Join(config.PromptDirs, ","))
		fmt.Printf("WebSearch: %v\n", config.EnableWebSearch)
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
