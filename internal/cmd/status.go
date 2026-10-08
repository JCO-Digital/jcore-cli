package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the running projects and their URLs",
	Run: func(cmd *cobra.Command, args []string) {
		printProxyStatus()

		running, err := runningProjects()
		if err != nil {
			fmt.Printf("Error checking Docker projects: %v\n", err)
			return
		}
		proxied, err := project.ProxiedProjects()
		if err != nil {
			fmt.Printf("Error checking Docker projects: %v\n", err)
			return
		}

		if len(running) == 0 {
			fmt.Println("\nNo running projects.")
			return
		}

		domainsByName := make(map[string][]string, len(proxied))
		for _, p := range proxied {
			domainsByName[p.Name] = p.Domains
		}

		for _, p := range running {
			fmt.Printf("\n%s  %s\n", p.Name, shortenHome(p.Path))
			domains, ok := domainsByName[p.Name]
			if !ok || len(domains) == 0 {
				fmt.Println("  not behind the proxy (started by an older jcore?) - restart it with \"jcore start\"")
				continue
			}
			sites := make([]string, len(domains))
			for i, d := range domains {
				sites[i] = "https://" + d
			}
			fmt.Printf("  Site:    %s\n", strings.Join(sites, ", "))
			fmt.Printf("  Adminer: http://adminer.%s\n", domains[0])
			fmt.Printf("  Mail:    http://mail.%s\n", domains[0])
		}
	},
}

// printProjectURLs prints where the current project is served.
func printProjectURLs() {
	adminer, mail := project.ToolURLs()
	fmt.Printf("  Site:    https://%s\n  Adminer: %s\n  Mail:    %s\n", project.LocalDomain(), adminer, mail)
}

// shortenHome replaces the user's home directory prefix of path with "~".
func shortenHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if rel, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.Join("~", rel)
	}
	return path
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
