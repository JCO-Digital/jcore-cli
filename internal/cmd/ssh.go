package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/spf13/cobra"
)

// sshCmd represents the ssh command
var sshCmd = &cobra.Command{
	Use:   "ssh [-- command...]",
	Short: "Open an ssh shell on the project's remote server",
	Long: `Sends the configured port knock (knockdPorts) and connects to remoteHost over ssh.
Without arguments it opens a login shell, starting in remotePath when set.
Anything after "--" is run on the remote instead, e.g. jcore ssh -- wp plugin list`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil || projectDir == "" {
			fmt.Println("Error: not in a JCore project (no jcore.toml found)")
			return
		}

		if err := project.RunSSH(args); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				os.Exit(exitErr.ExitCode())
			}
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(sshCmd)
}
