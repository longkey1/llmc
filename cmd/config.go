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
Available fields: configfile, openai_base_url, gemini_base_url, model, openai_token, gemini_token, promptdirs, websearch, ignorewebsearcherrors

Examples:
  llmc config                    # Show all configuration
  llmc config model             # Show only model
  llmc config openai_base_url   # Show only OpenAI base URL
  llmc config gemini_base_url   # Show only Gemini base URL
  llmc config openai_token      # Show only OpenAI token
  llmc config gemini_token      # Show only Gemini token
  llmc config promptdirs        # Show only prompt directories
  llmc config websearch         # Show only web search setting
  llmc config ignorewebsearcherrors  # Show only ignore web search errors setting`,
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
			case "openai_base_url", "openaibaseurl":
				fmt.Println(config.OpenAIBaseURL)
			case "gemini_base_url", "geminibaseurl":
				fmt.Println(config.GeminiBaseURL)
			case "model":
				fmt.Println(config.Model)
			case "openai_token", "openaitoken":
				fmt.Println(maskToken(config.OpenAIToken))
			case "gemini_token", "geminitoken":
				fmt.Println(maskToken(config.GeminiToken))
			case "promptdirs":
				// PromptDirs are already absolute paths
				fmt.Println(strings.Join(config.PromptDirs, ","))
			case "websearch":
				fmt.Println(config.EnableWebSearch)
			case "ignorewebsearcherrors":
				fmt.Println(config.IgnoreWebSearchErrors)
			default:
				fmt.Fprintf(os.Stderr, "Unknown field: %s\n", args[0])
				fmt.Fprintf(os.Stderr, "Available fields: configfile, openai_base_url, gemini_base_url, model, openai_token, gemini_token, promptdirs, websearch, ignorewebsearcherrors\n")
				os.Exit(1)
			}
			return
		}

		// Display all configuration values
		fmt.Printf("ConfigFile: %s\n", viper.ConfigFileUsed())
		fmt.Printf("OpenAIBaseURL: %s\n", config.OpenAIBaseURL)
		fmt.Printf("OpenAIToken: %s\n", maskToken(config.OpenAIToken))
		fmt.Printf("GeminiBaseURL: %s\n", config.GeminiBaseURL)
		fmt.Printf("GeminiToken: %s\n", maskToken(config.GeminiToken))
		fmt.Printf("Model: %s\n", config.Model)
		// PromptDirs are already absolute paths
		fmt.Printf("PromptDirectories: %s\n", strings.Join(config.PromptDirs, ","))
		fmt.Printf("WebSearch: %v\n", config.EnableWebSearch)
		fmt.Printf("IgnoreWebSearchErrors: %v\n", config.IgnoreWebSearchErrors)
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
