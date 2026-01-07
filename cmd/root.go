/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/longkey1/llmc/internal/gemini"
	"github.com/longkey1/llmc/internal/llmc"
	"github.com/longkey1/llmc/internal/openai"
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

	// Determine config directory
	configDir := ""
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
		configDir = filepath.Dir(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in config directory with name "config" (without extension).
		configDir = filepath.Join(home, ".config", "llmc")
		viper.AddConfigPath(configDir)
		viper.SetConfigType("toml")
		viper.SetConfigName("config")
	}

	// Create default config with prompts directory
	defaultConfig := llmc.NewDefaultConfig(filepath.Join(configDir, "prompts"))

	// Set default values from llmc package
	viper.SetDefault("provider", defaultConfig.Provider)
	viper.SetDefault("model", defaultConfig.Model)
	viper.SetDefault("token", defaultConfig.Token)
	viper.SetDefault("prompt_dirs", defaultConfig.PromptDirs)
	viper.SetDefault("enable_web_search", defaultConfig.EnableWebSearch)

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
		}
	}

	// Dynamically set base_url based on provider if not explicitly set
	if viper.GetString("base_url") == "" {
		provider := viper.GetString("provider")
		switch provider {
		case gemini.ProviderName:
			viper.Set("base_url", gemini.DefaultBaseURL)
		case openai.ProviderName:
			viper.Set("base_url", openai.DefaultBaseURL)
		}
	}

	if verbose {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		fmt.Fprintln(os.Stderr, "Environment variables:")
		fmt.Fprintln(os.Stderr, "  LLMC_PROVIDER:", viper.GetString("provider"))
		fmt.Fprintln(os.Stderr, "  LLMC_MODEL:", viper.GetString("model"))
		fmt.Fprintln(os.Stderr, "  LLMC_BASE_URL:", viper.GetString("base_url"))
		fmt.Fprintln(os.Stderr, "  LLMC_PROMPT_DIRS:", viper.GetStringSlice("prompt_dirs"))
		fmt.Fprintln(os.Stderr, "  LLMC_ENABLE_WEB_SEARCH:", viper.GetBool("enable_web_search"))
	}
}
