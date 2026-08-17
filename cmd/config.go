package cmd

import (
	"fmt"
	"sort"
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
Available fields: configfile, openai_base_url, gemini_base_url, anthropic_base_url, ollama_base_url, model, openai_token, gemini_token, anthropic_token, ollama_token, promptdirs, websearch, tools, execallowedcommands, execdeniedcommands, execunlisted, execenvmode, execenvpassthrough, writeallowedpaths, writedeniedpaths, writeunlisted, readdeniedpaths, sessionretentiondays

Examples:
  llmc config                      # Show all configuration
  llmc config model               # Show only model
  llmc config openai_base_url     # Show only OpenAI base URL
  llmc config gemini_base_url     # Show only Gemini base URL
  llmc config anthropic_base_url  # Show only Anthropic base URL
  llmc config ollama_base_url     # Show only Ollama base URL
  llmc config openai_token        # Show only OpenAI token
  llmc config gemini_token        # Show only Gemini token
  llmc config anthropic_token     # Show only Anthropic token
  llmc config ollama_token        # Show only Ollama token
  llmc config promptdirs          # Show only prompt directories
  llmc config websearch           # Show only web search setting
  llmc config tools               # Show only tool calling setting
  llmc config execunlisted        # Show only the action for unlisted commands
  llmc config execenvmode         # Show only exec_command environment mode
  llmc config writedeniedpaths    # Show only the write_file deny list
  llmc config readdeniedpaths     # Show only the read_file deny list
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
			case "ollama_base_url", "ollamabaseurl":
				fmt.Println(cfg.OllamaBaseURL)
			case "model":
				fmt.Println(cfg.Model)
			case "openai_token", "openaitoken":
				fmt.Println(resolveAndMaskToken(cfg, "openai"))
			case "gemini_token", "geminitoken":
				fmt.Println(resolveAndMaskToken(cfg, "gemini"))
			case "anthropic_token", "anthropictoken":
				fmt.Println(resolveAndMaskToken(cfg, "anthropic"))
			case "ollama_token", "ollamatoken":
				fmt.Println(resolveAndMaskToken(cfg, "ollama"))
			case "promptdirs":
				// PromptDirs are already absolute paths
				fmt.Println(strings.Join(cfg.PromptDirs, ","))
			case "websearch":
				fmt.Println(cfg.EnableWebSearch)
			case "tools":
				fmt.Println(cfg.EnableTools)
			case "execallowedcommands", "exec_allowed_commands":
				fmt.Println(formatCommandRules(cfg.ExecAllowedCommands))
			case "execdeniedcommands", "exec_denied_commands":
				fmt.Println(formatCommandRules(cfg.ExecDeniedCommands))
			case "execunlisted", "exec_unlisted":
				fmt.Println(cfg.ExecUnlisted)
			case "execenvmode", "exec_env_mode":
				fmt.Println(cfg.ExecEnvMode)
			case "execenvpassthrough", "exec_env_passthrough":
				fmt.Println(strings.Join(cfg.ExecEnvPassthrough, ","))
			case "writeallowedpaths", "write_allowed_paths":
				fmt.Println(formatPathRules(cfg.WriteAllowedPaths))
			case "writedeniedpaths", "write_denied_paths":
				fmt.Println(formatPathRules(cfg.WriteDeniedPaths))
			case "writeunlisted", "write_unlisted":
				fmt.Println(cfg.WriteUnlisted)
			case "readdeniedpaths", "read_denied_paths":
				fmt.Println(formatPathRules(cfg.ReadDeniedPaths))
			case "sessionretentiondays":
				fmt.Println(cfg.SessionRetentionDays)
			default:
				return fmt.Errorf("unknown field: %s\nAvailable fields: configfile, openai_base_url, gemini_base_url, anthropic_base_url, ollama_base_url, model, openai_token, gemini_token, anthropic_token, ollama_token, promptdirs, websearch, tools, execallowedcommands, execdeniedcommands, execunlisted, execenvmode, execenvpassthrough, writeallowedpaths, writedeniedpaths, writeunlisted, readdeniedpaths, sessionretentiondays", args[0])
			}
			return nil
		}

		// Display all configuration values
		fmt.Printf("%-20s: %s\n", "ConfigFile", viper.ConfigFileUsed())
		fmt.Printf("%-20s: %s\n", "OpenAIBaseURL", cfg.OpenAIBaseURL)
		fmt.Printf("%-20s: %s\n", "OpenAIToken", resolveAndMaskToken(cfg, "openai"))
		fmt.Printf("%-20s: %s\n", "GeminiBaseURL", cfg.GeminiBaseURL)
		fmt.Printf("%-20s: %s\n", "GeminiToken", resolveAndMaskToken(cfg, "gemini"))
		fmt.Printf("%-20s: %s\n", "AnthropicBaseURL", cfg.AnthropicBaseURL)
		fmt.Printf("%-20s: %s\n", "AnthropicToken", resolveAndMaskToken(cfg, "anthropic"))
		fmt.Printf("%-20s: %s\n", "OllamaBaseURL", cfg.OllamaBaseURL)
		fmt.Printf("%-20s: %s\n", "OllamaToken", resolveAndMaskToken(cfg, "ollama"))
		fmt.Printf("%-20s: %s\n", "Model", cfg.Model)
		// PromptDirs are already absolute paths
		fmt.Printf("%-20s: %s\n", "PromptDirectories", strings.Join(cfg.PromptDirs, ","))
		fmt.Printf("%-20s: %v\n", "WebSearch", cfg.EnableWebSearch)
		fmt.Printf("%-20s: %v\n", "Tools", cfg.EnableTools)
		fmt.Printf("%-20s: %s\n", "ExecAllowedCommands", formatCommandRules(cfg.ExecAllowedCommands))
		fmt.Printf("%-20s: %s\n", "ExecDeniedCommands", formatCommandRules(cfg.ExecDeniedCommands))
		fmt.Printf("%-20s: %s\n", "ExecUnlisted", cfg.ExecUnlisted)
		fmt.Printf("%-20s: %s\n", "ExecEnvMode", cfg.ExecEnvMode)
		fmt.Printf("%-20s: %s\n", "ExecEnvPassthrough", strings.Join(cfg.ExecEnvPassthrough, ","))
		fmt.Printf("%-20s: %s\n", "WriteAllowedPaths", formatPathRules(cfg.WriteAllowedPaths))
		fmt.Printf("%-20s: %s\n", "WriteDeniedPaths", formatPathRules(cfg.WriteDeniedPaths))
		fmt.Printf("%-20s: %s\n", "WriteUnlisted", cfg.WriteUnlisted)
		fmt.Printf("%-20s: %s\n", "ReadDeniedPaths", formatPathRules(cfg.ReadDeniedPaths))
		fmt.Printf("%-20s: %d\n", "SessionRetentionDays", cfg.SessionRetentionDays)
		return nil
	},
}

// formatCommandRules renders exec command rules as "git=[status diff], ls=*"
// with command names sorted for stable output.
func formatCommandRules(rules map[string][]string) string {
	if len(rules) == 0 {
		return "(none)"
	}

	names := make([]string, 0, len(rules))
	for name := range rules {
		names = append(names, name)
	}
	sort.Strings(names)

	entries := make([]string, 0, len(names))
	for _, name := range names {
		subcommands := rules[name]
		if len(subcommands) == 0 {
			entries = append(entries, name+"=*")
			continue
		}
		entries = append(entries, fmt.Sprintf("%s=[%s]", name, strings.Join(subcommands, " ")))
	}
	return strings.Join(entries, ", ")
}

// formatPathRules renders a path rule list for display.
func formatPathRules(rules []string) string {
	if len(rules) == 0 {
		return "(none)"
	}
	return strings.Join(rules, ", ")
}

// maskToken returns a masked version of the token for security
func maskToken(token string) string {
	if len(token) <= 8 {
		return "********"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// resolveAndMaskToken gets the token for the provider and masks it for display
func resolveAndMaskToken(cfg *config.Config, provider string) string {
	token, err := cfg.GetToken(provider)
	// GetToken returns an empty token without error for providers whose
	// token is optional (ollama), so check both cases
	if err != nil || token == "" {
		return "(not set)"
	}
	return maskToken(token)
}

func init() {
	rootCmd.AddCommand(configCmd)
}
