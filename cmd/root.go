/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/longkey1/llmc/internal/llmc/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "llmc",
	Short: "A CLI tool for interacting with LLM APIs",
	Long: `llmc is a command-line tool that allows you to interact with various LLM APIs.
It supports multiple providers.
You can configure the tool using a TOML configuration file.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/llmc/config.toml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Set environment variable prefix and automatic env
	viper.SetEnvPrefix("LLMC") // Set prefix for environment variables
	viper.AutomaticEnv()       // read in environment variables that match

	// Determine config directory for user config
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)
	userConfigDir := filepath.Join(home, ".config", "llmc")

	// Create default config with multiple prompts directories
	// Note: Later directories in the array take precedence over earlier ones
	defaultPromptDirs := []string{
		"/usr/share/llmc/prompts",              // System package prompts (lowest priority)
		"/usr/local/share/llmc/prompts",        // Local install prompts (low priority)
		filepath.Join(userConfigDir, "prompts"), // User-specific prompts (highest priority)
	}
	defaultConfig := config.NewDefaultConfig(filepath.Join(userConfigDir, "prompts"))

	// Set default values from llmc package
	viper.SetDefault("model", defaultConfig.Model)
	viper.SetDefault("openai_base_url", defaultConfig.OpenAIBaseURL)
	viper.SetDefault("openai_token", defaultConfig.OpenAIToken)
	viper.SetDefault("gemini_base_url", defaultConfig.GeminiBaseURL)
	viper.SetDefault("gemini_token", defaultConfig.GeminiToken)
	viper.SetDefault("prompt_dirs", defaultPromptDirs)
	viper.SetDefault("enable_web_search", defaultConfig.EnableWebSearch)
	viper.SetDefault("session_message_threshold", defaultConfig.SessionMessageThreshold)

	// Bind environment variables
	viper.BindEnv("openai_base_url", "LLMC_OPENAI_BASE_URL")
	viper.BindEnv("openai_token", "LLMC_OPENAI_TOKEN")
	viper.BindEnv("gemini_base_url", "LLMC_GEMINI_BASE_URL")
	viper.BindEnv("gemini_token", "LLMC_GEMINI_TOKEN")
	viper.BindEnv("session_message_threshold", "LLMC_SESSION_MESSAGE_THRESHOLD")

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Load system-wide config first (lower priority)
		systemConfigPaths := []string{
			"/etc/llmc",
			"/usr/local/etc/llmc",
		}

		systemConfigLoaded := false
		for _, path := range systemConfigPaths {
			viper.AddConfigPath(path)
		}
		viper.SetConfigType("toml")
		viper.SetConfigName("config")

		// Try to read system-wide config
		if err := viper.ReadInConfig(); err == nil {
			systemConfigLoaded = true
			if verbose {
				fmt.Fprintln(os.Stderr, "Loaded system-wide config:", viper.ConfigFileUsed())
			}
		}

		// Load user config (higher priority) - merge with system config
		viper.AddConfigPath(userConfigDir)
		if systemConfigLoaded {
			// Merge user config on top of system config
			if err := viper.MergeInConfig(); err != nil {
				if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
					fmt.Fprintf(os.Stderr, "Error merging user config file: %v\n", err)
				}
			} else if verbose {
				fmt.Fprintln(os.Stderr, "Merged user config:", viper.ConfigFileUsed())
			}
		} else {
			// No system config, just read user config
			if err := viper.ReadInConfig(); err != nil {
				if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
					fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
				}
			}
		}
	}

	if verbose {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		fmt.Fprintln(os.Stderr, "Environment variables:")
		fmt.Fprintln(os.Stderr, "  LLMC_MODEL:", viper.GetString("model"))
		fmt.Fprintln(os.Stderr, "  LLMC_OPENAI_BASE_URL:", viper.GetString("openai_base_url"))
		fmt.Fprintln(os.Stderr, "  LLMC_GEMINI_BASE_URL:", viper.GetString("gemini_base_url"))
		fmt.Fprintln(os.Stderr, "  LLMC_PROMPT_DIRS:", viper.GetStringSlice("prompt_dirs"))
		fmt.Fprintln(os.Stderr, "  LLMC_ENABLE_WEB_SEARCH:", viper.GetBool("enable_web_search"))
	}
}
