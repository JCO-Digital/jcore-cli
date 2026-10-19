package postsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// classList decodes wp-cli's "classes" field, which may be a
// space-separated string or an array.
type classList []string

func (c *classList) UnmarshalJSON(b []byte) error {
	var arr []string
	if json.Unmarshal(b, &arr) == nil {
		*c = cleanClasses(arr)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*c = cleanClasses(strings.Fields(s))
	return nil
}

func cleanClasses(in []string) []string {
	out := []string{}
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// MenuRef is one row of `wp menu list --format=json`.
type MenuRef struct {
	TermID    flexInt  `json:"term_id"`
	Name      string   `json:"name"`
	Slug      string   `json:"slug"`
	Locations []string `json:"locations"`
}

// MenuItem is a nav_menu_item: wp-cli's `menu item list` fields plus the
// raw post fields (a blank stored title means "use the linked object's").
type MenuItem struct {
	DBID        flexInt   `json:"db_id"`
	Type        string    `json:"type"`
	Object      string    `json:"object"`
	ObjectID    flexInt   `json:"object_id"`
	Parent      flexInt   `json:"menu_item_parent"`
	Position    flexInt   `json:"position"`
	Label       string    `json:"title"` // displayed title (computed if blank)
	Link        string    `json:"link"`
	Target      string    `json:"target"`
	AttrTitle   string    `json:"attr_title"`
	Description string    `json:"description"`
	Classes     classList `json:"classes"`
	XFN         string    `json:"xfn"`

	Title    string `json:"-"` // raw stored post_title
	Modified string `json:"-"` // post_modified_gmt
}

// MenuOp is what happens to one source menu item on the target.
type MenuOp struct {
	Source         MenuItem
	TargetObjectID int    // resolved target post/term ID (post_type, taxonomy)
	URL            string // custom links: URL rewritten to the target domain
	TargetID       int    // paired target item; 0 = add
	Changes        []string
}

// MenuPlan is everything a menu sync would do, worked out without changing anything.
type MenuPlan struct {
	Pull          bool // the source is the remote and the target the local site
	SourceSiteURL string
	TargetSiteURL string
	TargetUser    string
	Source        MenuRef
	Target        *MenuRef  // nil: the menu will be created
	TargetMenus   []MenuRef // every target menu, with its locations (for the backup)
	TargetItems   []MenuItem
	Ops           []MenuOp // source position order
	Removals      []MenuItem
	Locations     []string // source locations to assign on the target
	// PLLLocations are Polylang per-language locations to assign on the
	// target (location → languages), used instead of Locations when the
	// source site runs Polylang.
	PLLLocations map[string][]string
	pllNavMenus  map[string]map[string]int // target, for the backup
	Drift        []string
	Warnings     []string
	InSync       bool

	f *flow
}

// Creating reports whether the plan creates a new target menu.
func (p *MenuPlan) Creating() bool { return p.Target == nil }

// Pending reports whether there's anything to add or change, apart from removals.
func (p *MenuPlan) Pending() bool {
	if p.Target == nil || len(p.Locations) > 0 || len(p.PLLLocations) > 0 {
		return true
	}
	for _, op := range p.Ops {
		if op.TargetID == 0 || len(op.Changes) > 0 {
			return true
		}
	}
	return false
}

func listMenus(run func(...string) (string, error)) ([]MenuRef, error) {
	out, err := run("menu", "list", "--fields=term_id,name,slug,locations", "--format=json")
	if err != nil {
		return nil, err
	}
	var menus []MenuRef
	if strings.TrimSpace(out) == "" {
		return menus, nil
	}
	if err := parseJSON(out, &menus); err != nil {
		return nil, err
	}
	return menus, nil
}

// listMenuItems returns a menu's items in position order, with raw post fields.
func listMenuItems(run func(...string) (string, error), menuID int) ([]MenuItem, error) {
	out, err := run("menu", "item", "list", itoa(menuID), "--fields=db_id,type,object,object_id,menu_item_parent,position,title,link,target,attr_title,description,classes,xfn", "--format=json")
	if err != nil {
		return nil, err
	}
	var items []MenuItem
	if strings.TrimSpace(out) != "" {
		if err := parseJSON(out, &items); err != nil {
			return nil, err
		}
	}
	if len(items) == 0 {
		return items, nil
	}

	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = itoa(int(it.DBID))
	}
	out, err = run("post", "list", "--post_type=nav_menu_item", "--post_status=any", "--post__in="+strings.Join(ids, ","), "--posts_per_page=-1", "--fields=ID,post_title,post_modified_gmt", "--format=json")
	if err != nil {
		return nil, err
	}
	var raw []struct {
		ID       flexInt `json:"ID"`
		Title    string  `json:"post_title"`
		Modified string  `json:"post_modified_gmt"`
	}
	if err := parseJSON(out, &raw); err != nil {
		return nil, err
	}
	byID := map[int]int{}
	for i, r := range raw {
		byID[int(r.ID)] = i
	}
	for i := range items {
		if j, ok := byID[int(items[i].DBID)]; ok {
			items[i].Title = raw[j].Title
			items[i].Modified = raw[j].Modified
		}
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].Position < items[b].Position })
	return items, nil
}

// findMenu matches ident against menus by term ID, slug or name.
func findMenu(menus []MenuRef, ident string) []MenuRef {
	var out []MenuRef
	id, err := strconv.Atoi(ident)
	for _, m := range menus {
		if (err == nil && int(m.TermID) == id) || m.Slug == ident || strings.EqualFold(m.Name, ident) {
			out = append(out, m)
		}
	}
	return out
}

// PrepareMenu inspects both sites and returns the menu sync plan. It never writes.
func PrepareMenu(r Runner, opts Options, ident string) (*MenuPlan, error) {
	f, err := preflight(r, opts)
	if err != nil {
		return nil, err
	}
	src, dst := f.src, f.dst
	p := &MenuPlan{Pull: f.pull, SourceSiteURL: src.url, TargetSiteURL: dst.url, Warnings: f.warnings, f: f}

	// Source menu.
	sourceMenus, err := listMenus(src.run)
	if err != nil {
		return nil, fmt.Errorf("listing %s menus: %w", src.name, err)
	}
	matches := findMenu(sourceMenus, ident)
	if len(matches) != 1 {
		var b strings.Builder
		if len(matches) == 0 {
			fmt.Fprintf(&b, "no %s menu matches %q", src.name, ident)
		} else {
			fmt.Fprintf(&b, "%q matches %d %s menus; pass the ID", ident, len(matches), src.name)
		}
		fmt.Fprintf(&b, ". %s menus:", src.title)
		for _, m := range sourceMenus {
			fmt.Fprintf(&b, "\n  %d  %s (%s)", m.TermID, m.Name, m.Slug)
		}
		return nil, errors.New(b.String())
	}
	p.Source = matches[0]
	sourceItems, err := listMenuItems(src.run, int(p.Source.TermID))
	if err != nil {
		return nil, fmt.Errorf("reading %s menu items: %w", src.name, err)
	}
	if len(sourceItems) == 0 {
		return nil, fmt.Errorf("%s menu %q has no items; nothing to sync", src.name, p.Source.Name)
	}
	p.warnItemMeta(sourceItems)

	// Resolve every item's target; abort if any is missing.
	if err := p.resolveTargets(newPostMapper(f), sourceItems); err != nil {
		return nil, err
	}

	// Target menu: by slug, else by name.
	targetMenus, err := listMenus(dst.run)
	if err != nil {
		return nil, fmt.Errorf("listing %s menus: %w", dst.name, err)
	}
	p.TargetMenus = targetMenus
	for i, m := range targetMenus {
		if m.Slug == p.Source.Slug {
			p.Target = &targetMenus[i]
			break
		}
	}
	if p.Target == nil {
		for i, m := range targetMenus {
			if strings.EqualFold(m.Name, p.Source.Name) {
				p.Target = &targetMenus[i]
				p.Warnings = append(p.Warnings, fmt.Sprintf("matched %s menu %q by name (its slug is %q, %s is %q)", dst.name, m.Name, m.Slug, src.name, p.Source.Slug))
				break
			}
		}
	}

	if p.Target != nil {
		if p.TargetItems, err = listMenuItems(dst.run, int(p.Target.TermID)); err != nil {
			return nil, fmt.Errorf("reading %s menu items: %w", dst.name, err)
		}
		p.diff()
		p.checkDrift(opts.LastPull)
	}

	if src.pll != nil {
		err = p.planPLLLocations()
	} else {
		err = p.planLocations(targetMenus)
	}
	if err != nil {
		return nil, err
	}

	if p.TargetUser, err = writeUser(dst.run, dst.name, opts.As); err != nil {
		return nil, err
	}

	p.InSync = !p.Pending() && len(p.Removals) == 0
	return p, nil
}

// coreMenuMeta are the meta keys WordPress itself stores on a menu item.
func isCoreMenuMeta(key string) bool {
	return strings.HasPrefix(key, "_menu_item_") || slices.Contains(internalMetaKeys, key)
}

func (p *MenuPlan) warnItemMeta(items []MenuItem) {
	keys := []string{}
	for _, it := range items {
		meta, err := getMeta(p.f.src.run, int(it.DBID))
		if err != nil {
			continue
		}
		for _, m := range meta {
			if !isCoreMenuMeta(m.Key) && !slices.Contains(keys, m.Key) {
				keys = append(keys, m.Key)
			}
		}
	}
	if len(keys) > 0 {
		p.Warnings = append(p.Warnings, "menu items have custom meta that is NOT synced: "+strings.Join(keys, ", "))
	}
}

// resolveTargets maps each source item's linked post/term to the target.
func (p *MenuPlan) resolveTargets(mapper *postMapper, items []MenuItem) error {
	src, dst := p.f.src, p.f.dst
	var missing []string
	for _, it := range items {
		op := MenuOp{Source: it}
		switch it.Type {
		case "post_type", "taxonomy":
			var res mapResult
			var err error
			if it.Type == "post_type" {
				res, err = mapper.post(int(it.ObjectID))
			} else {
				res, err = mapper.term(it.Object, int(it.ObjectID))
			}
			if err != nil {
				return err
			}
			if res.problem != "" {
				missing = appendUnique(missing, res.problem)
				break
			}
			op.TargetObjectID = res.id
		case "post_type_archive":
			if _, err := dst.run("post-type", "get", it.Object, "--field=name"); err != nil {
				missing = append(missing, fmt.Sprintf("post type %q (archive link %q) doesn't exist %s", it.Object, it.Label, dst.where))
			}
		case "custom":
			op.URL = ReplaceDomain(it.Link, src.domain, dst.domain)
		default:
			missing = append(missing, fmt.Sprintf("item %q has unsupported type %q", it.Label, it.Type))
		}
		p.Ops = append(p.Ops, op)
	}

	if len(missing) > 0 {
		return fmt.Errorf("menu %q links to things %s doesn't have; %s these first:\n  - %s", p.Source.Name, dst.site, p.f.verb(), strings.Join(missing, "\n  - "))
	}
	return nil
}

func (op MenuOp) signature() string {
	switch op.Source.Type {
	case "custom":
		return "custom|" + op.URL
	case "post_type_archive":
		return "post_type_archive|" + op.Source.Object
	}
	return op.Source.Type + "|" + op.Source.Object + "|" + itoa(op.TargetObjectID)
}

func targetSignature(it MenuItem) string {
	switch it.Type {
	case "custom":
		return "custom|" + it.Link
	case "post_type_archive":
		return "post_type_archive|" + it.Object
	}
	return it.Type + "|" + it.Object + "|" + itoa(int(it.ObjectID))
}

// diff pairs source items with target ones and works out field changes.
func (p *MenuPlan) diff() {
	used := map[int]bool{}
	for i := range p.Ops {
		sig := p.Ops[i].signature()
		for _, rit := range p.TargetItems {
			if !used[int(rit.DBID)] && targetSignature(rit) == sig {
				used[int(rit.DBID)] = true
				p.Ops[i].TargetID = int(rit.DBID)
				break
			}
		}
	}
	for _, rit := range p.TargetItems {
		if !used[int(rit.DBID)] {
			p.Removals = append(p.Removals, rit)
		}
	}

	byID := map[int]MenuItem{}
	for _, rit := range p.TargetItems {
		byID[int(rit.DBID)] = rit
	}
	for i := range p.Ops {
		op := &p.Ops[i]
		if op.TargetID == 0 {
			continue
		}
		rit := byID[op.TargetID]
		l := op.Source
		if l.Title != rit.Title {
			op.Changes = append(op.Changes, "title")
		}
		if l.Position != rit.Position {
			op.Changes = append(op.Changes, "position")
		}
		if parent, known := p.targetParent(l); !known || parent != int(rit.Parent) {
			op.Changes = append(op.Changes, "parent")
		}
		if l.Target != rit.Target {
			op.Changes = append(op.Changes, "target")
		}
		if l.AttrTitle != rit.AttrTitle {
			op.Changes = append(op.Changes, "attr_title")
		}
		if l.Description != rit.Description {
			op.Changes = append(op.Changes, "description")
		}
		if strings.Join(l.Classes, " ") != strings.Join(rit.Classes, " ") {
			op.Changes = append(op.Changes, "classes")
		}
		if l.XFN != rit.XFN {
			op.Changes = append(op.Changes, "xfn")
		}
	}
}

// targetParent returns the target item ID an item's parent maps to; known
// is false when the parent is still to be added (so its ID isn't known yet).
func (p *MenuPlan) targetParent(it MenuItem) (int, bool) {
	if it.Parent == 0 {
		return 0, true
	}
	for _, op := range p.Ops {
		if op.Source.DBID == it.Parent {
			return op.TargetID, op.TargetID != 0
		}
	}
	return 0, true // parent not in the menu: top level
}

func (p *MenuPlan) checkDrift(lastPull time.Time) {
	if lastPull.IsZero() {
		return
	}
	for _, it := range p.TargetItems {
		if t, ok := parseGMT(it.Modified); ok && t.After(lastPull.UTC()) {
			p.Drift = append(p.Drift, fmt.Sprintf("%s menu item %q was modified %s UTC, after your last database pull (%s UTC)", p.f.dst.name, it.Label, it.Modified, lastPull.UTC().Format("2006-01-02 15:04:05")))
		}
	}
}

// planLocations assigns the source menu's theme locations on the target,
// but only ones that exist there and are free.
func (p *MenuPlan) planLocations(targetMenus []MenuRef) error {
	if len(p.Source.Locations) == 0 {
		return nil
	}
	dst := p.f.dst
	registered, err := menuLocations(dst)
	if err != nil {
		return err
	}
	owner := map[string]MenuRef{}
	for _, m := range targetMenus {
		for _, l := range m.Locations {
			owner[l] = m
		}
	}
	for _, loc := range p.Source.Locations {
		switch m, taken := owner[loc]; {
		case !registered[loc]:
			p.Warnings = append(p.Warnings, fmt.Sprintf("theme location %q isn't registered %s; not assigned", loc, dst.where))
		case taken && p.Target != nil && m.TermID == p.Target.TermID:
			// Already assigned to this menu.
		case taken:
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s location %q already shows menu %q; left unchanged", dst.name, loc, m.Name))
		default:
			p.Locations = append(p.Locations, loc)
		}
	}
	return nil
}

// planPLLLocations assigns the source menu's per-language theme locations
// (Polylang stores these in its own settings, not the theme mods) on the
// target, but only ones that exist there and are free.
func (p *MenuPlan) planPLLLocations() error {
	src, dst := p.f.src, p.f.dst
	source, err := pllNavMenus(src.run)
	if err != nil {
		return fmt.Errorf("reading %s Polylang menu locations: %w", src.name, err)
	}
	type slot struct{ loc, lang string }
	var wanted []slot
	for _, loc := range sortedLocKeys(source) {
		for _, lang := range sortedKeys(source[loc]) {
			if source[loc][lang] == int(p.Source.TermID) {
				wanted = append(wanted, slot{loc, lang})
			}
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	if dst.pll == nil {
		p.Warnings = append(p.Warnings, fmt.Sprintf("the %s menu is assigned to Polylang language locations, but Polylang isn't active %s; locations not assigned", src.name, dst.where))
		return nil
	}
	target, err := pllNavMenus(dst.run)
	if err != nil {
		return fmt.Errorf("reading %s Polylang menu locations: %w", dst.name, err)
	}
	p.pllNavMenus = target
	registered, err := menuLocations(dst)
	if err != nil {
		return err
	}
	p.PLLLocations = map[string][]string{}
	for _, w := range wanted {
		cur := target[w.loc][w.lang]
		switch {
		case !locationRe.MatchString(w.loc) || !registered[w.loc]:
			p.Warnings = append(p.Warnings, fmt.Sprintf("theme location %q isn't registered %s; not assigned", w.loc, dst.where))
		case !dst.pll.HasLanguage(w.lang):
			p.Warnings = append(p.Warnings, fmt.Sprintf("language %q isn't set up %s; location %s not assigned for it", w.lang, dst.where, w.loc))
		case cur != 0 && p.Target != nil && cur == int(p.Target.TermID):
			// Already assigned to this menu.
		case cur != 0:
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s location %q (%s) already shows another menu (ID %d); left unchanged", dst.name, w.loc, w.lang, cur))
		default:
			p.PLLLocations[w.loc] = append(p.PLLLocations[w.loc], w.lang)
		}
	}
	if len(p.PLLLocations) == 0 {
		p.PLLLocations = nil
	}
	return nil
}

func sortedLocKeys(m map[string]map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// menuLocations returns the theme locations registered on e.
func menuLocations(e *end) (map[string]bool, error) {
	out, err := e.run("menu", "location", "list", "--format=json")
	if err != nil {
		return nil, fmt.Errorf("listing %s menu locations: %w", e.name, err)
	}
	var locs []struct {
		Location string `json:"location"`
	}
	if strings.TrimSpace(out) != "" {
		if err := parseJSON(out, &locs); err != nil {
			return nil, err
		}
	}
	registered := map[string]bool{}
	for _, l := range locs {
		registered[l.Location] = true
	}
	return registered, nil
}

// MenuResult describes what ApplyMenu actually did, including after a failure.
type MenuResult struct {
	MenuID     int
	Created    bool
	BackupPath string
	Done       []string
}

// MenuEditURL is the target's wp-admin screen for the menu.
func (p *MenuPlan) MenuEditURL(id int) string {
	return strings.TrimRight(p.TargetSiteURL, "/") + "/wp-admin/nav-menus.php?action=edit&menu=" + itoa(id)
}

// ApplyMenu carries out the plan. Target items missing from the source are
// deleted only if removeLeftovers is set. On error, the result says what
// was already done.
func ApplyMenu(p *MenuPlan, removeLeftovers bool, backupDir string) (*MenuResult, error) {
	dst := p.f.dst
	res := &MenuResult{}
	user := "--user=" + p.TargetUser

	if p.Target != nil {
		path, err := WriteMenuBackup(backupDir, p)
		if err != nil {
			return res, fmt.Errorf("backing up the %s menu (nothing was changed): %w", dst.name, err)
		}
		res.BackupPath = path
		res.MenuID = int(p.Target.TermID)
		res.Done = append(res.Done, fmt.Sprintf("backed up %s menu to %s", dst.name, path))
	} else {
		out, err := dst.run("menu", "create", p.Source.Name, "--porcelain", user)
		if err != nil {
			return res, fmt.Errorf("creating %s menu: %w", dst.name, err)
		}
		if res.MenuID, err = parseID(out); err != nil {
			return res, fmt.Errorf("creating %s menu: %w", dst.name, err)
		}
		res.Created = true
		res.Done = append(res.Done, fmt.Sprintf("created %s menu %q (ID %d)", dst.name, p.Source.Name, res.MenuID))
	}
	menu := itoa(res.MenuID)

	// Source item ID → target item ID, filled in as items are paired or added.
	idMap := map[int]int{}
	for _, op := range p.Ops {
		if op.TargetID != 0 {
			idMap[int(op.Source.DBID)] = op.TargetID
		}
	}
	var parentFixes []MenuOp

	for _, op := range p.Ops {
		l := op.Source
		parent := 0
		if l.Parent != 0 {
			if id, ok := idMap[int(l.Parent)]; ok {
				parent = id
			} else if p.hasSourceItem(int(l.Parent)) {
				parentFixes = append(parentFixes, op) // parent comes later
			}
		}

		if op.TargetID == 0 {
			id, err := addMenuItem(dst, menu, op, parent, user)
			if err != nil {
				return res, fmt.Errorf("adding menu item %q: %w", l.Label, err)
			}
			idMap[int(l.DBID)] = id
			res.Done = append(res.Done, fmt.Sprintf("added %q", l.Label))
			continue
		}
		if len(op.Changes) == 0 {
			continue
		}
		if err := updateMenuItem(dst, op, parent, user); err != nil {
			return res, fmt.Errorf("updating menu item %q: %w", l.Label, err)
		}
		res.Done = append(res.Done, fmt.Sprintf("updated %q (%s)", l.Label, strings.Join(op.Changes, ", ")))
	}

	for _, op := range parentFixes {
		id := idMap[int(op.Source.DBID)]
		parent := idMap[int(op.Source.Parent)]
		if _, err := dst.run("post", "meta", "update", itoa(id), "_menu_item_menu_item_parent", itoa(parent), user); err != nil {
			return res, fmt.Errorf("setting parent of %q: %w", op.Source.Label, err)
		}
	}

	if removeLeftovers && len(p.Removals) > 0 {
		args := []string{"menu", "item", "delete"}
		for _, it := range p.Removals {
			args = append(args, itoa(int(it.DBID)))
		}
		if _, err := dst.run(append(args, user)...); err != nil {
			return res, fmt.Errorf("removing old menu items: %w", err)
		}
		res.Done = append(res.Done, fmt.Sprintf("removed %d old %s items", len(p.Removals), dst.name))
	}

	for _, loc := range p.Locations {
		if _, err := dst.run("menu", "location", "assign", menu, loc, user); err != nil {
			return res, fmt.Errorf("assigning location %q: %w", loc, err)
		}
		res.Done = append(res.Done, "assigned to location "+loc)
	}
	for _, loc := range sortedLangKeys(p.PLLLocations) {
		for _, lang := range p.PLLLocations[loc] {
			if _, err := dst.run("eval", pllAssignMenuPHP(res.MenuID, loc, lang), user); err != nil {
				return res, fmt.Errorf("assigning location %q (%s): %w", loc, lang, err)
			}
			res.Done = append(res.Done, fmt.Sprintf("assigned to location %s (%s)", loc, lang))
		}
	}
	return res, nil
}

func sortedLangKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (p *MenuPlan) hasSourceItem(id int) bool {
	for _, op := range p.Ops {
		if int(op.Source.DBID) == id {
			return true
		}
	}
	return false
}

func addMenuItem(dst *end, menu string, op MenuOp, parent int, user string) (int, error) {
	l := op.Source
	var args []string
	switch l.Type {
	case "post_type":
		args = []string{"menu", "item", "add-post", menu, itoa(op.TargetObjectID)}
	case "taxonomy":
		args = []string{"menu", "item", "add-term", menu, l.Object, itoa(op.TargetObjectID)}
	case "custom":
		args = []string{"menu", "item", "add-custom", menu, l.Label, op.URL}
	case "post_type_archive":
		// wp-cli can't add archive links; add a custom link and convert it.
		args = []string{"menu", "item", "add-custom", menu, l.Label, "#"}
	}
	if l.Title != "" && l.Type != "custom" && l.Type != "post_type_archive" {
		args = append(args, "--title="+l.Title)
	}
	args = append(args, "--position="+itoa(int(l.Position)))
	if parent > 0 {
		args = append(args, "--parent-id="+itoa(parent))
	}
	if l.Target != "" {
		args = append(args, "--target="+l.Target)
	}
	if len(l.Classes) > 0 {
		args = append(args, "--classes="+strings.Join(l.Classes, " "))
	}
	if l.Description != "" {
		args = append(args, "--description="+l.Description)
	}
	if l.AttrTitle != "" {
		args = append(args, "--attr-title="+l.AttrTitle)
	}
	out, err := dst.run(append(args, "--porcelain", user)...)
	if err != nil {
		return 0, err
	}
	id, err := parseID(out)
	if err != nil {
		return 0, err
	}

	sid := itoa(id)
	if l.Type == "post_type_archive" {
		for k, v := range map[string]string{"_menu_item_type": "post_type_archive", "_menu_item_object": l.Object, "_menu_item_url": ""} {
			if _, err := dst.run("post", "meta", "update", sid, k, v, user); err != nil {
				return id, err
			}
		}
		if l.Title == "" {
			if _, err := dst.run("post", "update", sid, "--post_title=", user); err != nil {
				return id, err
			}
		}
	}
	if l.XFN != "" {
		if _, err := dst.run("post", "meta", "update", sid, "_menu_item_xfn", l.XFN, user); err != nil {
			return id, err
		}
	}
	return id, nil
}

// updateMenuItem writes only the changed fields, straight to the item's
// post fields and meta (wp-cli's `menu item update` can't blank a title or
// move an item to another parent).
func updateMenuItem(dst *end, op MenuOp, parent int, user string) error {
	l := op.Source
	id := itoa(op.TargetID)
	post := []string{"post", "update", id}
	for _, c := range op.Changes {
		var err error
		switch c {
		case "title":
			post = append(post, "--post_title="+l.Title)
		case "description":
			post = append(post, "--post_content="+l.Description)
		case "attr_title":
			post = append(post, "--post_excerpt="+l.AttrTitle)
		case "position":
			post = append(post, "--menu_order="+itoa(int(l.Position)))
		case "parent":
			_, err = dst.run("post", "meta", "update", id, "_menu_item_menu_item_parent", itoa(parent), user)
		case "target":
			_, err = dst.run("post", "meta", "update", id, "_menu_item_target", l.Target, user)
		case "xfn":
			_, err = dst.run("post", "meta", "update", id, "_menu_item_xfn", l.XFN, user)
		case "classes":
			classes, _ := json.Marshal(append([]string{}, l.Classes...))
			if len(l.Classes) == 0 {
				classes = []byte(`[""]`) // WordPress's own empty value
			}
			_, err = dst.stdin(string(classes), "post", "meta", "update", id, "_menu_item_classes", "--format=json", user)
		}
		if err != nil {
			return err
		}
	}
	if len(post) > 3 {
		if _, err := dst.run(append(post, user)...); err != nil {
			return err
		}
	}
	return nil
}

// WriteMenuBackup saves the target menu, its items and every menu's
// theme locations as JSON in dir.
func WriteMenuBackup(dir string, p *MenuPlan) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(map[string]any{
		"site":    p.TargetSiteURL,
		"takenAt": time.Now().UTC().Format(time.RFC3339),
		"menu":    p.Target,
		// Every target menu with its theme locations, i.e. nav_menu_locations.
		"allMenus": p.TargetMenus,
		// Polylang's per-language locations (location → language → menu ID).
		"polylangNavMenus": p.pllNavMenus,
		"items":            backupItems(p.TargetItems),
	}, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-menu-%s-%s.json", time.Now().Format("20060102-150405"), p.Target.Slug, p.f.dst.name))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// backupItems includes the raw fields that MenuItem's JSON tags skip.
func backupItems(items []MenuItem) []map[string]any {
	out := []map[string]any{}
	for _, it := range items {
		out = append(out, map[string]any{
			"db_id": it.DBID, "type": it.Type, "object": it.Object, "object_id": it.ObjectID,
			"menu_item_parent": it.Parent, "position": it.Position, "title": it.Label,
			"raw_title": it.Title, "link": it.Link, "target": it.Target, "attr_title": it.AttrTitle,
			"description": it.Description, "classes": it.Classes, "xfn": it.XFN, "modified_gmt": it.Modified,
		})
	}
	return out
}
