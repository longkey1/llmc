/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/longkey1/llmc/internal/version"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long: `Show detailed version information including:
- Version number
- Git commit SHA
- Build time
- Go version`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check for short flag
		short, _ := cmd.Flags().GetBool("short")
		if short {
			fmt.Println(version.Short())
		} else {
			fmt.Println(version.Info())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	// Add short flag for brief version output
	versionCmd.Flags().BoolP("short", "s", false, "Show only version number")
}
