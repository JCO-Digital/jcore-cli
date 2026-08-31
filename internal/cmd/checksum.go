package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
)

// checksumCmd represents the checksum command
var checksumCmd = &cobra.Command{
	Use:   "checksum",
	Short: "Manage file checksums",
	Long: `This is used to check which files have been changed manually,
and should not be overwritten automatically during updates.`,
}

// listChecksumsCmd represents the checksum list command
var listChecksumsCmd = &cobra.Command{
	Use:   "list",
	Short: "List all checksums and their status",
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil || projectDir == "" {
			fmt.Println("Error: not in a JCore project (no jcore.toml found)")
			return
		}

		checksums, err := project.LoadChecksums(projectDir)
		if err != nil {
			fmt.Printf("Error loading checksums: %v\n", err)
			return
		}

		if len(checksums) == 0 {
			fmt.Println("No checksums found.")
			return
		}

		keys := make([]string, 0, len(checksums))
		for k := range checksums {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		fmt.Printf("%-35s %s\n", "File", "Status")
		fmt.Printf("%-35s %s\n", "----", "------")

		for _, file := range keys {
			fullPath := filepath.Join(projectDir, file)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				fmt.Printf("%-35s Missing\n", file)
				continue
			}

			current, err := project.CalculateChecksum(fullPath)
			if err != nil {
				fmt.Printf("%-35s Error calculating checksum\n", file)
				continue
			}

			status := "OK"
			if current != checksums[file] {
				status = "Changed"
			}
			fmt.Printf("%-35s %s\n", file, status)
		}
	},
}

// setChecksumCmd represents the checksum set command
var setChecksumCmd = &cobra.Command{
	Use:   "set [files...]",
	Short: "Set checksums for specific files",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil || projectDir == "" {
			fmt.Println("Error: not in a JCore project (no jcore.toml found)")
			return
		}

		checksums, err := project.LoadChecksums(projectDir)
		if err != nil {
			fmt.Printf("Error loading checksums: %v\n", err)
			return
		}

		for _, file := range args {
			fullPath := filepath.Join(projectDir, file)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				fmt.Printf("Error: File %s doesn't exist.\n", file)
				continue
			}

			checksum, err := project.CalculateChecksum(fullPath)
			if err != nil {
				fmt.Printf("Error calculating checksum for %s: %v\n", file, err)
				continue
			}

			if current, exists := checksums[file]; exists && current == checksum {
				fmt.Printf("File %s already has the correct checksum.\n", file)
			} else {
				checksums[file] = checksum
				fmt.Printf("Checksum for %s updated.\n", file)
			}
		}

		if err := project.SaveChecksums(projectDir, checksums); err != nil {
			fmt.Printf("Error saving checksums: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(checksumCmd)
	checksumCmd.AddCommand(listChecksumsCmd)
	checksumCmd.AddCommand(setChecksumCmd)
}
