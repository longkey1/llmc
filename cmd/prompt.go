/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/longkey1/llmc/internal/llmc"
	"github.com/spf13/cobra"
)

var withDir bool

// promptCmd represents the prompt command
var promptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "List available prompt templates",
	Long: `List all available prompt templates from the configured prompt directories.
This command recursively scans all prompt directories specified in the configuration and displays
the names of available .toml prompt files, including those in subdirectories.

The prompt files should be in TOML format with the following structure:
system = "System prompt with optional {{input}} placeholder"
user = "User prompt with optional {{input}} placeholder"

Prompt names are displayed as relative paths from the prompt directory root.
For example, a file at ${prompt_dir}/foo/bar.toml will be displayed as "foo/bar".

If you want to see which directory each prompt comes from, use the --with-dir option.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration from file
		config, err := llmc.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Debug output
		if verbose {
			fmt.Fprintf(os.Stderr, "Prompt directories: %v\n", config.PromptDirs)
		}

		// Collect all prompt files from all directories
		var allPrompts []string
		promptMap := make(map[string]string) // prompt name -> directory path

		for _, promptDir := range config.PromptDirs {
			// Check if directory exists
			if _, err := os.Stat(promptDir); os.IsNotExist(err) {
				if verbose {
					fmt.Fprintf(os.Stderr, "Prompt directory does not exist: %s\n", promptDir)
				}
				continue
			}

			// Recursively find all .toml files
			err := filepath.Walk(promptDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Skip directories
				if info.IsDir() {
					return nil
				}

				// Check if it's a .toml file
				if !strings.HasSuffix(info.Name(), ".toml") {
					return nil
				}

				// Calculate relative path from prompt directory
				relPath, err := filepath.Rel(promptDir, path)
				if err != nil {
					if verbose {
						fmt.Fprintf(os.Stderr, "Error calculating relative path for %s: %v\n", path, err)
					}
					return nil
				}

				// Remove .toml extension to get prompt name
				promptName := strings.TrimSuffix(relPath, ".toml")

				// Convert Windows path separators to forward slashes for consistency
				promptName = filepath.ToSlash(promptName)

				// Check if we already found this prompt in another directory
				if existingDir, exists := promptMap[promptName]; exists {
					if verbose {
						fmt.Fprintf(os.Stderr, "Warning: Prompt '%s' found in multiple directories: %s and %s\n",
							promptName, existingDir, promptDir)
					}
				} else {
					promptMap[promptName] = promptDir
					allPrompts = append(allPrompts, promptName)
				}

				return nil
			})

			if err != nil {
				fmt.Fprintf(os.Stderr, "Error walking prompt directory %s: %v\n", promptDir, err)
				continue
			}
		}

		// Sort prompts alphabetically
		sort.Strings(allPrompts)

		// Display results
		if len(allPrompts) == 0 {
			fmt.Println("No prompt templates found.")
			fmt.Println("Create .toml files in the following directories:")
			for _, promptDir := range config.PromptDirs {
				fmt.Printf("  - %s\n", promptDir)
			}
			return
		}

		fmt.Printf("Available prompt templates (%d found):\n\n", len(allPrompts))
		for _, promptName := range allPrompts {
			dir := promptMap[promptName]
			if withDir {
				fmt.Printf("  %s (from %s)\n", promptName, dir)
			} else {
				fmt.Printf("  %s\n", promptName)
			}
		}

		fmt.Printf("\nUse a prompt template with: llmc chat --prompt <name> [message]\n")
		fmt.Printf("Example: llmc chat --prompt foo/bar [message]\n")
	},
}

func init() {
	rootCmd.AddCommand(promptCmd)
	promptCmd.Flags().BoolVar(&withDir, "with-dir", false, "Show the directory each prompt was found in")
}
