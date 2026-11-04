package cmd

import (
	"fmt"
	"os"

	"github.com/JCO-Digital/jcore/internal/skills"
	"github.com/spf13/cobra"
)

var (
	skillsSyncForce bool
	skillsSyncRepo  string
)

// skillsSyncCmd pulls the shared JCORE agent skills and links them into
// Claude Code.
var skillsSyncCmd = &cobra.Command{
	Use:   "skills-sync",
	Short: "Pull the JCORE agent skills and install them into Claude Code",
	Long: `Clones (or updates) JCO-Digital/jcore-skills into a jcore-managed checkout
at ~/.local/share/jcore/jcore-skills, then symlinks every skill in it into
Claude Code's global skills directory (~/.claude/skills, or
$CLAUDE_CONFIG_DIR/skills). Run it again at any time to pick up new and
updated skills; links to skills removed upstream are cleaned up.

The managed checkout is reset to the remote on every run, so don't edit it —
work in your own clone of jcore-skills instead. A skill name that already
links to another location (such as your own clone) is left alone unless
--force is given. A real file or directory with the same name is never
touched.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		checkout, err := skills.CheckoutDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		target, err := skills.ClaudeSkillsDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Pulling %s...\n", skillsSyncRepo)
		if err := skills.Fetch(skillsSyncRepo, checkout); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating skills checkout: %v\n", err)
			os.Exit(1)
		}

		results, err := skills.Install(checkout, target, skillsSyncForce)
		for _, r := range results {
			line := fmt.Sprintf("  %-12s %s", r.Status, r.Name)
			if r.Detail != "" {
				line += " — " + r.Detail
			}
			fmt.Println(line)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error installing skills: %v\n", err)
			os.Exit(1)
		}
		if len(results) == 0 {
			fmt.Println("No skills found in the repository.")
			return
		}
		fmt.Printf("Skills installed in %s. New Claude Code sessions will pick them up.\n", target)
	},
}

func init() {
	skillsSyncCmd.Flags().BoolVarP(&skillsSyncForce, "force", "f", false, "replace existing symlinks that point somewhere else")
	skillsSyncCmd.Flags().StringVar(&skillsSyncRepo, "repo", skills.DefaultRepo, "git URL of the skills repository")
	rootCmd.AddCommand(skillsSyncCmd)
}
