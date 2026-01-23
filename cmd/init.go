package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/longkey1/llmc/internal/llmc/config"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the configuration file",
	Long: `Initialize the configuration file with default settings.
The config file will be created at $HOME/.config/llmc/config.toml by default.
You can specify a different location using the --config option.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get home directory
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %v", err)
		}

		// Set config file path
		configFile := filepath.Join(home, ".config", "llmc", "config.toml")
		if cfgFile != "" {
			configFile = cfgFile
		}

		// Create config directory
		configDir := filepath.Dir(configFile)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %v", err)
		}

		// Check if config file already exists
		if _, err := os.Stat(configFile); err == nil {
			return fmt.Errorf("config file already exists at: %s", configFile)
		}

		// Create default config
		cfg := config.NewDefaultConfig(filepath.Join(configDir, "prompts"))

		// Create config file
		f, err := os.Create(configFile)
		if err != nil {
			return fmt.Errorf("failed to create config file: %v", err)
		}
		defer f.Close()

		// Encode config to TOML
		encoder := toml.NewEncoder(f)
		if err := encoder.Encode(cfg); err != nil {
			return fmt.Errorf("failed to encode config: %v", err)
		}

		// Create prompts directory
		promptsDir := filepath.Join(configDir, "prompts")
		if err := os.MkdirAll(promptsDir, 0755); err != nil {
			return fmt.Errorf("failed to create prompts directory: %v", err)
		}

		fmt.Printf("Configuration file created at: %s\n", configFile)
		fmt.Printf("Prompts directory created at: %s\n", promptsDir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
