/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/longkey1/llmc/internal/llmc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

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

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in config directory with name "config" (without extension).
		configDir := filepath.Join(home, ".config", "llmc")
		viper.AddConfigPath(configDir)
		viper.SetConfigType("toml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// Set default values
	defaultConfig := llmc.NewDefaultConfig(filepath.Join(filepath.Dir(viper.ConfigFileUsed()), "prompts"))
	viper.SetDefault("provider", defaultConfig.Provider)
	viper.SetDefault("base_url", defaultConfig.BaseURL)
	viper.SetDefault("model", defaultConfig.Model)
	viper.SetDefault("token", defaultConfig.Token)
	viper.SetDefault("prompt_dir", defaultConfig.PromptDir)

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
