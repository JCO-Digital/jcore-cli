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
	Short: "Push one local post or menu to the remote site, or pull one with --pull",
	Long: `Pushes a single local post (content, title, excerpt, featured image and
page template) to the remote configured by remoteHost/remotePath, including any
[branch-<name>] override for the current git branch.

Media the post references is uploaded and added to the remote media library if
missing, attachment IDs are remapped, and localDomain URLs are rewritten to
remoteDomain. The remote post is matched by post type and slug; if there is
none, it is created as a draft, and if there are several, you pick which one to
update. --remote-id <id> updates that remote post instead, whatever its slug.

Nothing on the remote is deleted or overwritten without confirmation: existing
remote files are never replaced, and the remote post is backed up to
.jcore/sync-backups/ before it's updated.

With Polylang, the remote post is matched in the same language, a new post gets
the local post's language, and it's linked to translations that already exist on
the remote (existing links are never removed). IDs in ACF image/file/gallery and
post-object/relationship fields, and embedded Gravity Forms (matched by title),
are remapped too.

With --menu <slug|name|id>, a classic navigation menu is synced instead: items
are matched to the remote menu, linked pages and terms are mapped by slug (the
sync aborts if any are missing on the remote), and remote items no longer in the
local menu are removed only after a separate confirmation.

With --pull, everything goes the other way: the remote post (or --menu) given by
ID or slug is copied to the local site, with the same matching, remapping,
backups and confirmations. Missing media files are downloaded (never
overwriting local ones), remoteDomain URLs are rewritten to localDomain, and
--local-id <id> picks the local post to update. A post created locally keeps
the remote post's status and date.`,
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
		lang, _ := cmd.Flags().GetString("lang")
		remoteID, _ := cmd.Flags().GetInt("remote-id")
		localID, _ := cmd.Flags().GetInt("local-id")
		pull, _ := cmd.Flags().GetBool("pull")
		if err := validateSyncArgs(args, menu, postType, lang, withMeta, remoteID, localID, pull); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		ident := menu
		if len(args) == 1 {
			ident = args[0]
		}
		targetID := remoteID
		if pull {
			targetID = localID
		}

		opts := postsync.Options{
			Ident:        ident,
			Pull:         pull,
			PostType:     postType,
			Lang:         lang,
			Meta:         withMeta,
			TargetID:     targetID,
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
		src, dst := syncSides(pull)
		opts.ChooseTarget = chooseTargetPost(dst)

		if withMeta {
			fmt.Println(styleSyncDanger.Render(fmt.Sprintf("WARNING: --meta copies ALL custom post meta (ACF fields etc.) to the %s post.", dst)))
			fmt.Println(styleSyncWarn.Render(fmt.Sprintf("Only IDs in ACF image/file/gallery/post fields are remapped; other IDs are copied as-is, and existing %s values for the same keys are overwritten.", dst)))
		}

		project.KnockIfNeeded()
		runner := postsync.NewDockerRunner(projectDir, remoteHost, opts.RemotePath)

		if menu != "" {
			runMenuSync(runner, opts, projectDir, remoteHost, dryRun)
			return
		}

		printInspecting("post", opts, remoteHost)
		plan, err := postsync.Prepare(runner, opts)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			fmt.Println("Nothing was changed.")
			return
		}

		printSyncPlan(plan, opts, remoteHost, project.CurrentBranch(projectDir))

		if plan.InSync {
			fmt.Println(styleSyncOK.Render(fmt.Sprintf("The %s post is already in sync. Nothing to do.", dst)))
			return
		}
		if dryRun {
			fmt.Println("Dry run: nothing was changed.")
			return
		}

		if len(plan.Drift) > 0 {
			fmt.Println(styleSyncDanger.Render(fmt.Sprintf("The %s post may have changes the %s one doesn't have:", dst, src)))
			for _, d := range plan.Drift {
				fmt.Println(styleSyncDanger.Render("  ! " + d))
			}
			fmt.Println("Syncing will overwrite them (a backup is taken first).")
			typed := ""
			prompt := &survey.Input{Message: fmt.Sprintf("Type the slug (%s) to overwrite anyway:", plan.Source.PostName)}
			if err := survey.AskOne(prompt, &typed); err != nil || strings.TrimSpace(typed) != plan.Source.PostName {
				fmt.Println("Aborted. Nothing was changed.")
				return
			}
		}

		if withMeta && len(plan.Meta) > 0 {
			ok := false
			prompt := &survey.Confirm{Message: fmt.Sprintf("Really write %d meta values to the %s post?", len(plan.Meta), dst), Default: false}
			if err := survey.AskOne(prompt, &ok); err != nil || !ok {
				fmt.Println("Aborted. Nothing was changed.")
				return
			}
		}

		verb := "Sync"
		if pull {
			verb = "Pull"
		}
		ok := false
		prompt := &survey.Confirm{Message: fmt.Sprintf("%s %q to %s?", verb, plan.Source.PostTitle, plan.TargetSiteURL), Default: false}
		if err := survey.AskOne(prompt, &ok); err != nil || !ok {
			fmt.Println("Aborted. Nothing was changed.")
			return
		}

		res, err := postsync.Apply(plan, filepath.Join(projectDir, ".jcore", "sync-backups"))
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
				fmt.Printf("%s post backup: %s\n", capitalize(dst), res.BackupPath)
			}
			return
		}

		for _, d := range res.Done {
			fmt.Println(styleSyncOK.Render("  ✓ ") + d)
		}
		if res.VerifyMismatch {
			fmt.Println(styleSyncDanger.Render(fmt.Sprintf("WARNING: the content read back from the %s post differs from what was sent.", dst)))
			fmt.Println(styleSyncWarn.Render("WordPress may have filtered markup on save. Check the post in the editor."))
		}
		if res.FinalSlug != "" && res.FinalSlug != plan.Source.PostName && (plan.Creating() || res.FinalSlug != plan.Target.PostName) {
			fmt.Println(styleSyncWarn.Render(fmt.Sprintf("Note: WordPress changed the %s slug to %q (%s is %q), probably because it was already taken.", dst, res.FinalSlug, src, plan.Source.PostName)))
		}
		if res.Created && !pull {
			fmt.Println("Created as a draft. Review and publish it in wp-admin.")
		}
		fmt.Printf("Edit: %s\n", plan.EditURL(res.TargetID))
		if res.BackupPath != "" {
			fmt.Printf("Backup of the previous %s version: %s\n", dst, res.BackupPath)
		}
	},
}

// validateSyncArgs checks that exactly one of a post or --menu was given,
// that post-only flags aren't combined with --menu, and that the target ID
// flag matches the direction.
func validateSyncArgs(args []string, menu, postType, lang string, withMeta bool, remoteID, localID int, pull bool) error {
	switch {
	case menu != "" && len(args) > 0:
		return fmt.Errorf("give either a post or --menu, not both")
	case menu == "" && len(args) == 0:
		return fmt.Errorf("give a post ID or slug, or --menu <name>")
	case menu != "" && (postType != "" || lang != "" || withMeta || remoteID != 0 || localID != 0):
		return fmt.Errorf("--type, --lang, --meta, --remote-id and --local-id only apply to posts, not --menu")
	case pull && remoteID != 0:
		return fmt.Errorf("--remote-id picks the post a push updates; with --pull, use --local-id")
	case !pull && localID != 0:
		return fmt.Errorf("--local-id picks the post a pull updates; it needs --pull")
	case remoteID < 0:
		return fmt.Errorf("--remote-id must be a post ID")
	case localID < 0:
		return fmt.Errorf("--local-id must be a post ID")
	}
	return nil
}

// syncSides names the source and target sites of a sync.
func syncSides(pull bool) (src, dst string) {
	if pull {
		return "remote", "local"
	}
	return "local", "remote"
}

func printInspecting(what string, opts postsync.Options, remoteHost string) {
	if opts.Pull {
		fmt.Printf("Inspecting remote %s %q on %s and the local site...\n", what, opts.Ident, remoteHost)
		return
	}
	fmt.Printf("Inspecting local %s %q and %s...\n", what, opts.Ident, remoteHost)
}

// printRemoteSite prints where the remote is.
func printRemoteSite(siteURL string, opts postsync.Options, remoteHost, branch string) {
	fmt.Printf("  site:   %s\n", siteURL)
	fmt.Printf("  ssh:    %s:%s\n", remoteHost, opts.RemotePath)
	if branch != "" {
		fmt.Printf("  branch: %s\n", branch)
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// chooseTargetPost returns a prompt asking which of several target (side
// dst) posts sharing the source post's slug should be updated.
func chooseTargetPost(dst string) func(*postsync.Post, []postsync.Candidate) (int, error) {
	return func(source *postsync.Post, candidates []postsync.Candidate) (int, error) {
		options := make([]string, len(candidates))
		for i, c := range candidates {
			options[i] = fmt.Sprintf("ID %d  %q  (%s, modified %s UTC)", c.ID, c.Title, c.Status, c.Modified)
		}
		fmt.Println(styleSyncWarn.Render(fmt.Sprintf("%d %s %s posts have slug %q.", len(candidates), dst, source.PostType, source.PostName)))
		idx := 0
		prompt := &survey.Select{Message: fmt.Sprintf("Which %s post should be updated?", dst), Options: options}
		if err := survey.AskOne(prompt, &idx); err != nil {
			return 0, fmt.Errorf("no %s post chosen: %w", dst, err)
		}
		return candidates[idx].ID, nil
	}
}

func runMenuSync(runner postsync.Runner, opts postsync.Options, projectDir, remoteHost string, dryRun bool) {
	src, dst := syncSides(opts.Pull)
	printInspecting("menu", opts, remoteHost)
	plan, err := postsync.PrepareMenu(runner, opts, opts.Ident)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println("Nothing was changed.")
		return
	}

	printMenuPlan(plan, opts, remoteHost, project.CurrentBranch(projectDir))

	if plan.InSync {
		fmt.Println(styleSyncOK.Render(fmt.Sprintf("The %s menu is already in sync. Nothing to do.", dst)))
		return
	}
	if dryRun {
		fmt.Println("Dry run: nothing was changed.")
		return
	}

	if len(plan.Drift) > 0 {
		fmt.Println(styleSyncDanger.Render(fmt.Sprintf("The %s menu may have changes the %s one doesn't have:", dst, src)))
		for _, d := range plan.Drift {
			fmt.Println(styleSyncDanger.Render("  ! " + d))
		}
		fmt.Println("Syncing will overwrite them (a backup is taken first).")
		typed := ""
		prompt := &survey.Input{Message: fmt.Sprintf("Type the menu slug (%s) to overwrite anyway:", plan.Source.Slug)}
		if err := survey.AskOne(prompt, &typed); err != nil || strings.TrimSpace(typed) != plan.Source.Slug {
			fmt.Println("Aborted. Nothing was changed.")
			return
		}
	}

	verb := "Sync"
	if opts.Pull {
		verb = "Pull"
	}
	ok := false
	prompt := &survey.Confirm{Message: fmt.Sprintf("%s menu %q to %s?", verb, plan.Source.Name, plan.TargetSiteURL), Default: false}
	if err := survey.AskOne(prompt, &ok); err != nil || !ok {
		fmt.Println("Aborted. Nothing was changed.")
		return
	}

	remove := false
	if len(plan.Removals) > 0 {
		prompt := &survey.Confirm{Message: fmt.Sprintf("Also REMOVE the %d %s menu items listed above?", len(plan.Removals), dst), Default: false}
		if err := survey.AskOne(prompt, &remove); err != nil {
			remove = false
		}
		if !remove {
			fmt.Printf("Keeping the old %s items.\n", dst)
			if !plan.Pending() {
				fmt.Println("Nothing else to do. Nothing was changed.")
				return
			}
		}
	}

	res, err := postsync.ApplyMenu(plan, remove, filepath.Join(projectDir, ".jcore", "sync-backups"))
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
			fmt.Printf("%s menu backup: %s\n", capitalize(dst), res.BackupPath)
		}
		return
	}

	for _, d := range res.Done {
		fmt.Println(styleSyncOK.Render("  ✓ ") + d)
	}
	fmt.Printf("Edit: %s\n", plan.MenuEditURL(res.MenuID))
	if res.BackupPath != "" {
		fmt.Printf("Backup of the previous %s menu: %s\n", dst, res.BackupPath)
	}
}

func printMenuPlan(p *postsync.MenuPlan, opts postsync.Options, remoteHost, branch string) {
	src, dst := syncSides(p.Pull)
	fmt.Println()
	fmt.Println(styleHeading.Render(capitalize(src) + " menu"))
	fmt.Printf("  %q  (ID %d, slug %s, %d items)\n", p.Source.Name, p.Source.TermID, p.Source.Slug, len(p.Ops))
	if p.Pull {
		printRemoteSite(p.SourceSiteURL, opts, remoteHost, branch)
	}

	fmt.Println(styleHeading.Render(capitalize(dst)))
	if p.Pull {
		fmt.Printf("  site:   %s\n", p.TargetSiteURL)
	} else {
		printRemoteSite(p.TargetSiteURL, opts, remoteHost, branch)
	}
	fmt.Printf("  user:   %s\n", p.TargetUser)
	if p.Creating() {
		fmt.Println("  action: " + styleSyncWarn.Render("CREATE new menu"))
	} else {
		fmt.Printf("  action: "+styleSyncWarn.Render("UPDATE")+" %s menu %q (ID %d, %d items)\n", dst, p.Target.Name, p.Target.TermID, len(p.TargetItems))
	}

	fmt.Println(styleHeading.Render("Items"))
	depth := map[int]int{}
	for _, op := range p.Ops {
		d := 0
		if op.Source.Parent != 0 {
			d = depth[int(op.Source.Parent)] + 1
		}
		depth[int(op.Source.DBID)] = d
		status := styleDim.Render("keep")
		switch {
		case op.TargetID == 0:
			status = styleSyncOK.Render("add")
		case len(op.Changes) > 0:
			status = styleSyncWarn.Render("update: " + strings.Join(op.Changes, ", "))
		}
		target := op.Source.Object
		if op.Source.Type == "custom" {
			target = op.URL
		}
		fmt.Printf("  %s%s  %s  [%s]\n", strings.Repeat("  ", d), op.Source.Label, styleDim.Render(target), status)
	}

	if len(p.Removals) > 0 {
		fmt.Println(styleSyncDanger.Render(fmt.Sprintf("%s items not in the %s menu (removed only if you confirm)", capitalize(dst), src)))
		for _, it := range p.Removals {
			fmt.Println(styleSyncDanger.Render("  - " + it.Label))
		}
	}
	if len(p.Locations) > 0 {
		fmt.Printf("Assign to theme locations: %s\n", strings.Join(p.Locations, ", "))
	}
	for loc, langs := range p.PLLLocations {
		fmt.Printf("Assign to theme location %s for: %s\n", loc, strings.Join(langs, ", "))
	}

	printWarnings(p.Warnings)
}

func printSyncPlan(p *postsync.Plan, opts postsync.Options, remoteHost, branch string) {
	src, dst := syncSides(p.Pull)
	fmt.Println()
	fmt.Println(styleHeading.Render(capitalize(src) + " post"))
	fmt.Printf("  %s %q  (ID %d, slug %s, %s)\n", p.Source.PostType, p.Source.PostTitle, p.Source.ID, p.Source.PostName, p.Source.PostStatus)
	if p.Pull {
		printRemoteSite(p.SourceSiteURL, opts, remoteHost, branch)
	}

	fmt.Println(styleHeading.Render(capitalize(dst)))
	if p.Pull {
		fmt.Printf("  site:   %s\n", p.TargetSiteURL)
	} else {
		printRemoteSite(p.TargetSiteURL, opts, remoteHost, branch)
	}
	fmt.Printf("  user:   %s\n", p.TargetUser)
	if p.Creating() {
		status := "draft"
		if p.Pull {
			status = p.Source.PostStatus
		}
		fmt.Println("  action: " + styleSyncWarn.Render("CREATE new "+status+" "+p.Source.PostType))
	} else {
		fmt.Printf("  action: "+styleSyncWarn.Render("UPDATE")+" %s ID %d %q (%s, modified %s UTC)\n", dst, p.Target.ID, p.Target.PostTitle, p.Target.PostStatus, p.Target.PostModifiedGMT)
		fmt.Printf("  content: %d → %d bytes\n", len(p.Target.PostContent), len(p.FinalContent()))
	}

	if p.PLL != nil {
		fmt.Println(styleHeading.Render("Polylang"))
		fmt.Printf("  language: %s\n", p.PLL.Lang)
		if p.PLL.SetLang {
			fmt.Printf("  the new post is assigned language %s\n", p.PLL.Lang)
		}
		if len(p.PLL.NewLink) > 0 {
			fmt.Printf("  link translations: %s\n", strings.Join(p.PLL.NewLink, ", "))
		}
	}

	if len(p.Forms) > 0 {
		fmt.Println(styleHeading.Render("Gravity Forms"))
		for _, f := range p.Forms {
			fmt.Printf("  %d → %s %d  %q\n", f.SourceID, dst, f.TargetID, f.Title)
		}
	}

	if len(p.PostMap) > 0 {
		fmt.Println(styleHeading.Render("Linked posts (ACF)"))
		for from, to := range p.PostMap {
			fmt.Printf("  %d → %s %d\n", from, dst, to)
		}
	}

	if len(p.Media) > 0 || len(p.Files) > 0 {
		fmt.Println(styleHeading.Render("Media"))
		for _, m := range p.Media {
			switch {
			case m.TargetID != 0:
				fmt.Printf("  %d → %s %d  %s\n", m.SourceID, dst, m.TargetID, m.RelPath)
			case m.RelPath != "":
				fmt.Printf("  %d → new attachment  %s\n", m.SourceID, m.RelPath)
			}
		}
		if files := p.Copies(); len(files) > 0 {
			verb := "upload"
			if p.Pull {
				verb = "download"
			}
			fmt.Printf("  files to %s (%d):\n", verb, len(files))
			for _, f := range files {
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

	printWarnings(p.Warnings)
}

func printWarnings(warnings []string) {
	if len(warnings) > 0 {
		fmt.Println(styleHeading.Render("Warnings"))
		for _, w := range warnings {
			fmt.Println(styleSyncWarn.Render("  ! " + w))
		}
	}
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().String("type", "", "post type to match a slug against (default: any)")
	syncCmd.Flags().Bool("meta", false, "also copy custom post meta (ACF etc.); only IDs in ACF media/post fields are remapped")
	syncCmd.Flags().Bool("dry-run", false, "show what would be synced without changing anything")
	syncCmd.Flags().String("lang", "", "Polylang language narrowing a slug shared by several languages (e.g. fi)")
	syncCmd.Flags().String("menu", "", "sync a classic navigation menu (slug, name or ID) instead of a post")
	syncCmd.Flags().Int("remote-id", 0, "remote post ID to update, instead of matching one by slug")
	syncCmd.Flags().Int("local-id", 0, "with --pull: local post ID to update, instead of matching one by slug")
	syncCmd.Flags().String("as", "", "user login to perform the write as on the target site (default: first administrator)")
	syncCmd.Flags().Bool("pull", false, "copy the given remote post or --menu to the local site instead of pushing")
}
