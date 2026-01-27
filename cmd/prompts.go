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
	promptpkg "github.com/longkey1/llmc/internal/llmc/prompt"
	"github.com/spf13/cobra"
)

// promptCmd represents the prompts command
var promptCmd = &cobra.Command{
	Use:   "prompts",
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

		// promptInfo holds information about each prompt
		type promptInfo struct {
			path      string
			model     string
			webSearch string
		}

		// Collect all prompt files from all directories
		var allPrompts []string
		promptInfoMap := make(map[string]*promptInfo) // prompt name -> prompt info

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

				// Load prompt file to get model and web_search settings
				promptData, err := promptpkg.LoadPrompt(path)
				if err != nil {
					if verbose {
						fmt.Fprintf(os.Stderr, "Warning: Failed to load prompt '%s': %v\n", promptName, err)
					}
					// Continue even if we can't load the prompt
				}

				// Extract model and web_search info
				// Use default values in parentheses if not set in prompt
				modelStr := ""
				webSearchStr := ""
				if promptData != nil {
					if promptData.Model != nil {
						modelStr = *promptData.Model
					} else {
						modelStr = fmt.Sprintf("(%s)", cfg.Model)
					}
					if promptData.WebSearch != nil {
						if *promptData.WebSearch {
							webSearchStr = "enabled"
						} else {
							webSearchStr = "disabled"
						}
					} else {
						if cfg.EnableWebSearch {
							webSearchStr = "(enabled)"
						} else {
							webSearchStr = "(disabled)"
						}
					}
				} else {
					// If prompt failed to load, show defaults
					modelStr = fmt.Sprintf("(%s)", cfg.Model)
					if cfg.EnableWebSearch {
						webSearchStr = "(enabled)"
					} else {
						webSearchStr = "(disabled)"
					}
				}

				// Check if we already found this prompt in another directory
				_, exists := promptInfoMap[promptName]
				if exists {
					if verbose {
						existingPath := promptInfoMap[promptName].path
						fmt.Fprintf(os.Stderr, "Warning: Prompt '%s' found in multiple directories: %s and %s (using %s)\n",
							promptName, filepath.Dir(existingPath), promptDir, promptDir)
					}
				}
				// Always update with the current directory (later directories take precedence)
				promptInfoMap[promptName] = &promptInfo{
					path:      path,
					model:     modelStr,
					webSearch: webSearchStr,
				}
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

		// Calculate maximum widths for columns (with minimum values)
		maxNameWidth := 15
		maxModelWidth := 10
		maxWebSearchWidth := 10
		for _, promptName := range allPrompts {
			if len(promptName) > maxNameWidth {
				maxNameWidth = len(promptName)
			}
			info := promptInfoMap[promptName]
			if len(info.model) > maxModelWidth {
				maxModelWidth = len(info.model)
			}
			if len(info.webSearch) > maxWebSearchWidth {
				maxWebSearchWidth = len(info.webSearch)
			}
		}

		// Display in table format
		fmt.Printf("%-*s  %-*s  %-*s  %s\n",
			maxNameWidth, "PROMPT",
			maxModelWidth, "MODEL",
			maxWebSearchWidth, "WEB SEARCH",
			"FILE PATH")
		fmt.Printf("%s  %s  %s  %s\n",
			strings.Repeat("-", maxNameWidth),
			strings.Repeat("-", maxModelWidth),
			strings.Repeat("-", maxWebSearchWidth),
			strings.Repeat("-", 60))

		for _, promptName := range allPrompts {
			info := promptInfoMap[promptName]
			fmt.Printf("%-*s  %-*s  %-*s  %s\n",
				maxNameWidth, promptName,
				maxModelWidth, info.model,
				maxWebSearchWidth, info.webSearch,
				info.path)
		}

		fmt.Printf("\nUse a prompt template with: llmc chat --prompt <name> [message]\n")
		fmt.Printf("Example: llmc chat --prompt foo/bar [message]\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(promptCmd)
}
