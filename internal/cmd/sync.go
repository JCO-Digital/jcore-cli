package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JCO-Digital/jcore/internal/postsync"
	"github.com/JCO-Digital/jcore/internal/project"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	styleSyncWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleSyncDanger = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	styleSyncOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync <id|slug>",
	Short: "Push one local post to the remote site",
	Long: `Pushes a single local post (content, title, excerpt, featured image and
page template) to the remote configured by remoteHost/remotePath, including any
[branch-<name>] override for the current git branch.

Media the post references is uploaded and added to the remote media library if
missing, attachment IDs are remapped, and localDomain URLs are rewritten to
remoteDomain. The remote post is matched by post type and slug; if there is
none, it is created as a draft.

Nothing on the remote is deleted or overwritten without confirmation: existing
remote files are never replaced, and the remote post is backed up to
.jcore/sync-backups/ before it's updated.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectDir, err := project.FindProjectRoot()
		if err != nil || projectDir == "" {
			fmt.Println("Error: not in a JCore project (no jcore.toml found)")
			return
		}

		postType, _ := cmd.Flags().GetString("type")
		withMeta, _ := cmd.Flags().GetBool("meta")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		as, _ := cmd.Flags().GetString("as")

		opts := postsync.Options{
			Ident:        args[0],
			PostType:     postType,
			Meta:         withMeta,
			As:           as,
			LocalDomain:  viper.GetString("localDomain"),
			RemoteDomain: viper.GetString("remoteDomain"),
			RemotePath:   viper.GetString("remotePath"),
		}
		remoteHost := viper.GetString("remoteHost")
		for key, val := range map[string]string{"remoteHost": remoteHost, "remotePath": opts.RemotePath, "remoteDomain": opts.RemoteDomain, "localDomain": opts.LocalDomain} {
			if val == "" {
				fmt.Printf("Error: %s is not set; configure it with `jcore config set %s <value>`\n", key, key)
				return
			}
		}
		if info, err := os.Stat(filepath.Join(projectDir, ".jcore", "sql", "db.sql")); err == nil {
			opts.LastPull = info.ModTime()
		}

		if withMeta {
			fmt.Println(styleSyncDanger.Render("WARNING: --meta copies ALL custom post meta (ACF fields etc.) to the remote."))
			fmt.Println(styleSyncWarn.Render("Meta holding post or attachment IDs is not remapped, and existing remote values for the same keys are overwritten."))
		}

		project.KnockIfNeeded()
		runner := postsync.NewDockerRunner(projectDir, remoteHost, opts.RemotePath)

		fmt.Printf("Inspecting local post %q and %s...\n", opts.Ident, remoteHost)
		plan, err := postsync.Prepare(runner, opts)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			fmt.Println("Nothing was changed.")
			return
		}

		printSyncPlan(plan, opts, remoteHost, project.CurrentBranch(projectDir))

		if plan.InSync {
			fmt.Println(styleSyncOK.Render("The remote post is already in sync. Nothing to do."))
			return
		}
		if dryRun {
			fmt.Println("Dry run: nothing was changed.")
			return
		}

		if len(plan.Drift) > 0 {
			fmt.Println(styleSyncDanger.Render("The remote post may have changes your local copy doesn't have:"))
			for _, d := range plan.Drift {
				fmt.Println(styleSyncDanger.Render("  ! " + d))
			}
			fmt.Println("Syncing will overwrite them (a backup is taken first).")
			typed := ""
			prompt := &survey.Input{Message: fmt.Sprintf("Type the slug (%s) to overwrite anyway:", plan.Local.PostName)}
			if err := survey.AskOne(prompt, &typed); err != nil || strings.TrimSpace(typed) != plan.Local.PostName {
				fmt.Println("Aborted. Nothing was changed.")
				return
			}
		}

		if withMeta && len(plan.Meta) > 0 {
			ok := false
			prompt := &survey.Confirm{Message: fmt.Sprintf("Really write %d meta values to the remote post?", len(plan.Meta)), Default: false}
			if err := survey.AskOne(prompt, &ok); err != nil || !ok {
				fmt.Println("Aborted. Nothing was changed.")
				return
			}
		}

		ok := false
		prompt := &survey.Confirm{Message: fmt.Sprintf("Sync %q to %s?", plan.Local.PostTitle, plan.RemoteSiteURL), Default: false}
		if err := survey.AskOne(prompt, &ok); err != nil || !ok {
			fmt.Println("Aborted. Nothing was changed.")
			return
		}

		res, err := postsync.Apply(runner, plan, opts, filepath.Join(projectDir, ".jcore", "sync-backups"))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			if len(res.Done) == 0 {
				fmt.Println("Nothing was changed.")
			} else {
				fmt.Println("Already done before the failure:")
				for _, d := range res.Done {
					fmt.Println("  - " + d)
				}
			}
			if res.BackupPath != "" {
				fmt.Printf("Remote post backup: %s\n", res.BackupPath)
			}
			return
		}

		for _, d := range res.Done {
			fmt.Println(styleSyncOK.Render("  ✓ ") + d)
		}
		if res.VerifyMismatch {
			fmt.Println(styleSyncDanger.Render("WARNING: the content read back from the remote differs from what was sent."))
			fmt.Println(styleSyncWarn.Render("WordPress may have filtered markup on save. Check the post in the editor."))
		}
		if res.Created {
			fmt.Println("Created as a draft. Review and publish it in wp-admin.")
		}
		fmt.Printf("Edit: %s\n", plan.EditURL(res.RemoteID))
		if res.BackupPath != "" {
			fmt.Printf("Backup of the previous remote version: %s\n", res.BackupPath)
		}
	},
}

func printSyncPlan(p *postsync.Plan, opts postsync.Options, remoteHost, branch string) {
	fmt.Println()
	fmt.Println(styleHeading.Render("Local post"))
	fmt.Printf("  %s %q  (ID %d, slug %s, %s)\n", p.Local.PostType, p.Local.PostTitle, p.Local.ID, p.Local.PostName, p.Local.PostStatus)

	fmt.Println(styleHeading.Render("Remote"))
	fmt.Printf("  site:   %s\n", p.RemoteSiteURL)
	fmt.Printf("  ssh:    %s:%s\n", remoteHost, opts.RemotePath)
	if branch != "" {
		fmt.Printf("  branch: %s\n", branch)
	}
	fmt.Printf("  user:   %s\n", p.RemoteUser)
	if p.Creating() {
		fmt.Println("  action: " + styleSyncWarn.Render("CREATE new draft "+p.Local.PostType))
	} else {
		fmt.Printf("  action: "+styleSyncWarn.Render("UPDATE")+" remote ID %d %q (%s, modified %s UTC)\n", p.Remote.ID, p.Remote.PostTitle, p.Remote.PostStatus, p.Remote.PostModifiedGMT)
		fmt.Printf("  content: %d → %d bytes\n", len(p.Remote.PostContent), len(p.FinalContent(opts)))
	}

	if len(p.Media) > 0 || len(p.Files) > 0 {
		fmt.Println(styleHeading.Render("Media"))
		for _, m := range p.Media {
			switch {
			case m.RemoteID != 0:
				fmt.Printf("  %d → remote %d  %s\n", m.LocalID, m.RemoteID, m.RelPath)
			case m.RelPath != "":
				fmt.Printf("  %d → new attachment  %s\n", m.LocalID, m.RelPath)
			}
		}
		if up := p.Uploads(); len(up) > 0 {
			fmt.Printf("  files to upload (%d):\n", len(up))
			for _, f := range up {
				fmt.Println("    " + f)
			}
		}
	}

	if len(p.Meta) > 0 {
		fmt.Println(styleHeading.Render("Meta to write"))
		for _, m := range p.Meta {
			fmt.Println("  " + m.Key)
		}
	}

	if len(p.Warnings) > 0 {
		fmt.Println(styleHeading.Render("Warnings"))
		for _, w := range p.Warnings {
			fmt.Println(styleSyncWarn.Render("  ! " + w))
		}
	}
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().String("type", "", "post type to match a slug against (default: any)")
	syncCmd.Flags().Bool("meta", false, "also copy custom post meta (ACF etc.); IDs in meta are NOT remapped")
	syncCmd.Flags().Bool("dry-run", false, "show what would be synced without changing anything")
	syncCmd.Flags().String("as", "", "remote user login to perform the write as (default: first administrator)")
}
