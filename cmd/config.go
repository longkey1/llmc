package cmd

import (
	"fmt"
	"strings"

	"github.com/longkey1/llmc/internal/llmc/config"
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
Available fields: configfile, openai_base_url, gemini_base_url, anthropic_base_url, model, openai_token, gemini_token, anthropic_token, promptdirs, websearch, ignorewebsearcherrors, sessionretentiondays

Examples:
  llmc config                      # Show all configuration
  llmc config model               # Show only model
  llmc config openai_base_url     # Show only OpenAI base URL
  llmc config gemini_base_url     # Show only Gemini base URL
  llmc config anthropic_base_url  # Show only Anthropic base URL
  llmc config openai_token        # Show only OpenAI token
  llmc config gemini_token        # Show only Gemini token
  llmc config anthropic_token     # Show only Anthropic token
  llmc config promptdirs          # Show only prompt directories
  llmc config websearch           # Show only web search setting
  llmc config ignorewebsearcherrors  # Show only ignore web search errors setting
  llmc config sessionretentiondays   # Show only session retention days setting`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration from file
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// If a field is specified, show only that field
		if len(args) > 0 {
			field := strings.ToLower(args[0])
			switch field {
			case "configfile":
				fmt.Println(viper.ConfigFileUsed())
			case "openai_base_url", "openaibaseurl":
				fmt.Println(cfg.OpenAIBaseURL)
			case "gemini_base_url", "geminibaseurl":
				fmt.Println(cfg.GeminiBaseURL)
			case "anthropic_base_url", "anthropicbaseurl":
				fmt.Println(cfg.AnthropicBaseURL)
			case "model":
				fmt.Println(cfg.Model)
			case "openai_token", "openaitoken":
				fmt.Println(maskToken(cfg.OpenAIToken))
			case "gemini_token", "geminitoken":
				fmt.Println(maskToken(cfg.GeminiToken))
			case "anthropic_token", "anthropictoken":
				fmt.Println(maskToken(cfg.AnthropicToken))
			case "promptdirs":
				// PromptDirs are already absolute paths
				fmt.Println(strings.Join(cfg.PromptDirs, ","))
			case "websearch":
				fmt.Println(cfg.EnableWebSearch)
			case "ignorewebsearcherrors":
				fmt.Println(cfg.IgnoreWebSearchErrors)
			case "sessionretentiondays":
				fmt.Println(cfg.SessionRetentionDays)
			default:
				return fmt.Errorf("unknown field: %s\nAvailable fields: configfile, openai_base_url, gemini_base_url, anthropic_base_url, model, openai_token, gemini_token, anthropic_token, promptdirs, websearch, ignorewebsearcherrors, sessionretentiondays", args[0])
			}
			return nil
		}

		// Display all configuration values
		fmt.Printf("ConfigFile: %s\n", viper.ConfigFileUsed())
		fmt.Printf("OpenAIBaseURL: %s\n", cfg.OpenAIBaseURL)
		fmt.Printf("OpenAIToken: %s\n", maskToken(cfg.OpenAIToken))
		fmt.Printf("GeminiBaseURL: %s\n", cfg.GeminiBaseURL)
		fmt.Printf("GeminiToken: %s\n", maskToken(cfg.GeminiToken))
		fmt.Printf("AnthropicBaseURL: %s\n", cfg.AnthropicBaseURL)
		fmt.Printf("AnthropicToken: %s\n", maskToken(cfg.AnthropicToken))
		fmt.Printf("Model: %s\n", cfg.Model)
		// PromptDirs are already absolute paths
		fmt.Printf("PromptDirectories: %s\n", strings.Join(cfg.PromptDirs, ","))
		fmt.Printf("WebSearch: %v\n", cfg.EnableWebSearch)
		fmt.Printf("IgnoreWebSearchErrors: %v\n", cfg.IgnoreWebSearchErrors)
		fmt.Printf("SessionRetentionDays: %d\n", cfg.SessionRetentionDays)
		return nil
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
