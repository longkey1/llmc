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

	"github.com/longkey1/llmc/internal/llmc/config"
	"github.com/spf13/cobra"
)

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
model = "optional-model-name"  # Optional: overrides the default model for this prompt

Prompt names are displayed in a table format with the relative path from the prompt directory root and the full file path.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration from file
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Debug output
		if verbose {
			fmt.Fprintf(os.Stderr, "Prompt directories: %v\n", cfg.PromptDirs)
		}

		// Collect all prompt files from all directories
		var allPrompts []string
		promptMap := make(map[string]string)     // prompt name -> directory path
		promptPathMap := make(map[string]string) // prompt name -> full file path

		for _, promptDir := range cfg.PromptDirs {
			// promptDir is already an absolute path
			// Check if directory exists
			if _, err := os.Stat(promptDir); os.IsNotExist(err) {
				if verbose {
					fmt.Fprintf(os.Stderr, "Prompt directory does not exist: %s\n", promptDir)
				}
				continue
			}

			// Recursively find all .toml files
			err = filepath.Walk(promptDir, func(path string, info os.FileInfo, err error) error {
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
				existingDir, exists := promptMap[promptName]
				if exists {
					if verbose {
						fmt.Fprintf(os.Stderr, "Warning: Prompt '%s' found in multiple directories: %s and %s (using %s)\n",
							promptName, existingDir, promptDir, promptDir)
					}
				}
				// Always update with the current directory (later directories take precedence)
				promptMap[promptName] = promptDir
				promptPathMap[promptName] = path
				// Only add to allPrompts if this is the first time we've seen this prompt
				if !exists {
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
			for _, promptDir := range cfg.PromptDirs {
				// Show the original path (relative or absolute) in the message
				fmt.Printf("  - %s\n", promptDir)
			}
			return nil
		}

		fmt.Printf("Available prompt templates (%d found):\n\n", len(allPrompts))

		// Calculate maximum width for prompt names (minimum 15 characters)
		maxNameWidth := 15
		for _, promptName := range allPrompts {
			if len(promptName) > maxNameWidth {
				maxNameWidth = len(promptName)
			}
		}

		// Display in table format
		fmt.Printf("%-*s  %s\n", maxNameWidth, "PROMPT", "FILE PATH")
		fmt.Printf("%s  %s\n", strings.Repeat("-", maxNameWidth), strings.Repeat("-", 80))

		for _, promptName := range allPrompts {
			filePath := promptPathMap[promptName]
			fmt.Printf("%-*s  %s\n", maxNameWidth, promptName, filePath)
		}

		fmt.Printf("\nUse a prompt template with: llmc chat --prompt <name> [message]\n")
		fmt.Printf("Example: llmc chat --prompt foo/bar [message]\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(promptCmd)
}
