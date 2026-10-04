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
	Use:   "sync <id|slug> | --menu <name>",
	Short: "Push one local post or menu to the remote site",
	Long: `Pushes a single local post (content, title, excerpt, featured image and
page template) to the remote configured by remoteHost/remotePath, including any
[branch-<name>] override for the current git branch.

Media the post references is uploaded and added to the remote media library if
missing, attachment IDs are remapped, and localDomain URLs are rewritten to
remoteDomain. The remote post is matched by post type and slug; if there is
none, it is created as a draft.

Nothing on the remote is deleted or overwritten without confirmation: existing
remote files are never replaced, and the remote post is backed up to
.jcore/sync-backups/ before it's updated.

With --menu <slug|name|id>, a classic navigation menu is synced instead: items
are matched to the remote menu, linked pages and terms are mapped by slug (the
sync aborts if any are missing on the remote), and remote items no longer in the
local menu are removed only after a separate confirmation.`,
	Args: cobra.MaximumNArgs(1),
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
		menu, _ := cmd.Flags().GetString("menu")
		if err := validateSyncArgs(args, menu, postType, withMeta); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		ident := menu
		if len(args) == 1 {
			ident = args[0]
		}

		opts := postsync.Options{
			Ident:        ident,
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

		if menu != "" {
			runMenuSync(runner, opts, projectDir, remoteHost, dryRun)
			return
		}

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

// validateSyncArgs checks that exactly one of a post or --menu was given,
// and that post-only flags aren't combined with --menu.
func validateSyncArgs(args []string, menu, postType string, withMeta bool) error {
	switch {
	case menu != "" && len(args) > 0:
		return fmt.Errorf("give either a post or --menu, not both")
	case menu == "" && len(args) == 0:
		return fmt.Errorf("give a post ID or slug, or --menu <name>")
	case menu != "" && (postType != "" || withMeta):
		return fmt.Errorf("--type and --meta only apply to posts, not --menu")
	}
	return nil
}

func runMenuSync(runner postsync.Runner, opts postsync.Options, projectDir, remoteHost string, dryRun bool) {
	fmt.Printf("Inspecting local menu %q and %s...\n", opts.Ident, remoteHost)
	plan, err := postsync.PrepareMenu(runner, opts, opts.Ident)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println("Nothing was changed.")
		return
	}

	printMenuPlan(plan, opts, remoteHost, project.CurrentBranch(projectDir))

	if plan.InSync {
		fmt.Println(styleSyncOK.Render("The remote menu is already in sync. Nothing to do."))
		return
	}
	if dryRun {
		fmt.Println("Dry run: nothing was changed.")
		return
	}

	if len(plan.Drift) > 0 {
		fmt.Println(styleSyncDanger.Render("The remote menu may have changes your local copy doesn't have:"))
		for _, d := range plan.Drift {
			fmt.Println(styleSyncDanger.Render("  ! " + d))
		}
		fmt.Println("Syncing will overwrite them (a backup is taken first).")
		typed := ""
		prompt := &survey.Input{Message: fmt.Sprintf("Type the menu slug (%s) to overwrite anyway:", plan.Local.Slug)}
		if err := survey.AskOne(prompt, &typed); err != nil || strings.TrimSpace(typed) != plan.Local.Slug {
			fmt.Println("Aborted. Nothing was changed.")
			return
		}
	}

	ok := false
	prompt := &survey.Confirm{Message: fmt.Sprintf("Sync menu %q to %s?", plan.Local.Name, plan.SiteURL), Default: false}
	if err := survey.AskOne(prompt, &ok); err != nil || !ok {
		fmt.Println("Aborted. Nothing was changed.")
		return
	}

	remove := false
	if len(plan.Removals) > 0 {
		prompt := &survey.Confirm{Message: fmt.Sprintf("Also REMOVE the %d remote menu items listed above?", len(plan.Removals)), Default: false}
		if err := survey.AskOne(prompt, &remove); err != nil {
			remove = false
		}
		if !remove {
			fmt.Println("Keeping the old remote items.")
			if !plan.Pending() {
				fmt.Println("Nothing else to do. Nothing was changed.")
				return
			}
		}
	}

	res, err := postsync.ApplyMenu(runner, plan, remove, filepath.Join(projectDir, ".jcore", "sync-backups"))
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
			fmt.Printf("Remote menu backup: %s\n", res.BackupPath)
		}
		return
	}

	for _, d := range res.Done {
		fmt.Println(styleSyncOK.Render("  ✓ ") + d)
	}
	fmt.Printf("Edit: %s\n", plan.MenuEditURL(res.MenuID))
	if res.BackupPath != "" {
		fmt.Printf("Backup of the previous remote menu: %s\n", res.BackupPath)
	}
}

func printMenuPlan(p *postsync.MenuPlan, opts postsync.Options, remoteHost, branch string) {
	fmt.Println()
	fmt.Println(styleHeading.Render("Local menu"))
	fmt.Printf("  %q  (ID %d, slug %s, %d items)\n", p.Local.Name, p.Local.TermID, p.Local.Slug, len(p.Ops))

	fmt.Println(styleHeading.Render("Remote"))
	fmt.Printf("  site:   %s\n", p.SiteURL)
	fmt.Printf("  ssh:    %s:%s\n", remoteHost, opts.RemotePath)
	if branch != "" {
		fmt.Printf("  branch: %s\n", branch)
	}
	fmt.Printf("  user:   %s\n", p.RemoteUser)
	if p.Creating() {
		fmt.Println("  action: " + styleSyncWarn.Render("CREATE new menu"))
	} else {
		fmt.Printf("  action: "+styleSyncWarn.Render("UPDATE")+" remote menu %q (ID %d, %d items)\n", p.Remote.Name, p.Remote.TermID, len(p.RemoteItems))
	}

	fmt.Println(styleHeading.Render("Items"))
	depth := map[int]int{}
	for _, op := range p.Ops {
		d := 0
		if op.Local.Parent != 0 {
			d = depth[int(op.Local.Parent)] + 1
		}
		depth[int(op.Local.DBID)] = d
		status := styleDim.Render("keep")
		switch {
		case op.RemoteID == 0:
			status = styleSyncOK.Render("add")
		case len(op.Changes) > 0:
			status = styleSyncWarn.Render("update: " + strings.Join(op.Changes, ", "))
		}
		target := op.Local.Object
		if op.Local.Type == "custom" {
			target = op.URL
		}
		fmt.Printf("  %s%s  %s  [%s]\n", strings.Repeat("  ", d), op.Local.Label, styleDim.Render(target), status)
	}

	if len(p.Removals) > 0 {
		fmt.Println(styleSyncDanger.Render("Remote items not in the local menu (removed only if you confirm)"))
		for _, it := range p.Removals {
			fmt.Println(styleSyncDanger.Render("  - " + it.Label))
		}
	}
	if len(p.Locations) > 0 {
		fmt.Printf("Assign to theme locations: %s\n", strings.Join(p.Locations, ", "))
	}

	if len(p.Warnings) > 0 {
		fmt.Println(styleHeading.Render("Warnings"))
		for _, w := range p.Warnings {
			fmt.Println(styleSyncWarn.Render("  ! " + w))
		}
	}
	fmt.Println()
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
	syncCmd.Flags().String("menu", "", "sync a classic navigation menu (slug, name or ID) instead of a post")
	syncCmd.Flags().String("as", "", "remote user login to perform the write as (default: first administrator)")
}
