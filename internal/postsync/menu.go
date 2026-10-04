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

// MenuOp is what happens to one local menu item on the remote.
type MenuOp struct {
	Local          MenuItem
	RemoteObjectID int    // resolved remote post/term ID (post_type, taxonomy)
	URL            string // custom links: URL rewritten to the remote domain
	RemoteID       int    // paired remote item; 0 = add
	Changes        []string
}

// MenuPlan is everything a menu sync would do, worked out without changing anything.
type MenuPlan struct {
	SiteURL     string
	RemoteUser  string
	Local       MenuRef
	Remote      *MenuRef  // nil: the menu will be created
	RemoteMenus []MenuRef // every remote menu, with its locations (for the backup)
	RemoteItems []MenuItem
	Ops         []MenuOp // local position order
	Removals    []MenuItem
	Locations   []string // local locations to assign on the remote
	Drift       []string
	Warnings    []string
	InSync      bool
}

// Creating reports whether the plan creates a new remote menu.
func (p *MenuPlan) Creating() bool { return p.Remote == nil }

// Pending reports whether there's anything to add or change, apart from removals.
func (p *MenuPlan) Pending() bool {
	if p.Remote == nil || len(p.Locations) > 0 {
		return true
	}
	for _, op := range p.Ops {
		if op.RemoteID == 0 || len(op.Changes) > 0 {
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
	site, err := preflight(r, opts)
	if err != nil {
		return nil, err
	}
	p := &MenuPlan{SiteURL: site.URL, Warnings: site.Warnings}

	// Local menu.
	localMenus, err := listMenus(r.Local)
	if err != nil {
		return nil, fmt.Errorf("listing local menus: %w", err)
	}
	matches := findMenu(localMenus, ident)
	if len(matches) != 1 {
		var b strings.Builder
		if len(matches) == 0 {
			fmt.Fprintf(&b, "no local menu matches %q", ident)
		} else {
			fmt.Fprintf(&b, "%q matches %d local menus; pass the ID", ident, len(matches))
		}
		b.WriteString(". Local menus:")
		for _, m := range localMenus {
			fmt.Fprintf(&b, "\n  %d  %s (%s)", m.TermID, m.Name, m.Slug)
		}
		return nil, errors.New(b.String())
	}
	p.Local = matches[0]
	localItems, err := listMenuItems(r.Local, int(p.Local.TermID))
	if err != nil {
		return nil, fmt.Errorf("reading local menu items: %w", err)
	}
	if len(localItems) == 0 {
		return nil, fmt.Errorf("local menu %q has no items; nothing to sync", p.Local.Name)
	}
	p.warnItemMeta(r, localItems)

	// Resolve every item's target on the remote; abort if any is missing.
	if err := p.resolveTargets(r, localItems, opts); err != nil {
		return nil, err
	}

	// Remote menu: by slug, else by name.
	remoteMenus, err := listMenus(r.Remote)
	if err != nil {
		return nil, fmt.Errorf("listing remote menus: %w", err)
	}
	p.RemoteMenus = remoteMenus
	for i, m := range remoteMenus {
		if m.Slug == p.Local.Slug {
			p.Remote = &remoteMenus[i]
			break
		}
	}
	if p.Remote == nil {
		for i, m := range remoteMenus {
			if strings.EqualFold(m.Name, p.Local.Name) {
				p.Remote = &remoteMenus[i]
				p.Warnings = append(p.Warnings, fmt.Sprintf("matched remote menu %q by name (its slug is %q, local is %q)", m.Name, m.Slug, p.Local.Slug))
				break
			}
		}
	}

	if p.Remote != nil {
		if p.RemoteItems, err = listMenuItems(r.Remote, int(p.Remote.TermID)); err != nil {
			return nil, fmt.Errorf("reading remote menu items: %w", err)
		}
		p.diff()
		p.checkDrift(opts.LastPull)
	}

	if err := p.planLocations(r, remoteMenus); err != nil {
		return nil, err
	}

	if p.RemoteUser, err = remoteUser(r, opts.As); err != nil {
		return nil, err
	}

	p.InSync = !p.Pending() && len(p.Removals) == 0
	return p, nil
}

// coreMenuMeta are the meta keys WordPress itself stores on a menu item.
func isCoreMenuMeta(key string) bool {
	return strings.HasPrefix(key, "_menu_item_") || slices.Contains(internalMetaKeys, key)
}

func (p *MenuPlan) warnItemMeta(r Runner, items []MenuItem) {
	keys := []string{}
	for _, it := range items {
		meta, err := getMeta(r.Local, int(it.DBID))
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

// resolveTargets maps each local item's linked post/term to the remote.
func (p *MenuPlan) resolveTargets(r Runner, items []MenuItem, opts Options) error {
	var missing []string
	type key struct{ kind, a, b string }
	cache := map[key]int{}

	for _, it := range items {
		op := MenuOp{Local: it}
		switch it.Type {
		case "post_type":
			lp, err := getPost(r.Local, int(it.ObjectID))
			if err != nil || lp.PostName == "" {
				missing = append(missing, fmt.Sprintf("item %q links to local %s %d, which can't be read", it.Label, it.Object, it.ObjectID))
				break
			}
			k := key{"post", lp.PostType, lp.PostName}
			id, ok := cache[k]
			if !ok {
				refs, err := listPosts(r.Remote, lp.PostType, lp.PostName, "any")
				if err != nil {
					return fmt.Errorf("looking up %s %q on the remote: %w", lp.PostType, lp.PostName, err)
				}
				if len(refs) == 1 {
					id = int(refs[0].ID)
				} else if len(refs) > 1 {
					id = -1
				}
				cache[k] = id
			}
			switch {
			case id > 0:
				op.RemoteObjectID = id
			case id < 0:
				missing = append(missing, fmt.Sprintf("%s %q: several remote posts have this slug", lp.PostType, lp.PostName))
			default:
				missing = append(missing, fmt.Sprintf("%s %q is missing on the remote; run: jcore sync %d", lp.PostType, lp.PostName, lp.ID))
			}
		case "taxonomy":
			out, err := r.Local("term", "get", it.Object, itoa(int(it.ObjectID)), "--field=slug")
			if err != nil {
				missing = append(missing, fmt.Sprintf("item %q links to local %s term %d, which can't be read", it.Label, it.Object, it.ObjectID))
				break
			}
			slug := lastLine(out)
			k := key{"term", it.Object, slug}
			id, ok := cache[k]
			if !ok {
				out, err := r.Remote("term", "list", it.Object, "--slug="+slug, "--field=term_id")
				if err == nil {
					id, _ = parseID(out)
				}
				cache[k] = id
			}
			if id > 0 {
				op.RemoteObjectID = id
			} else {
				missing = append(missing, fmt.Sprintf("%s term %q is missing on the remote; create it there first", it.Object, slug))
			}
		case "post_type_archive":
			if _, err := r.Remote("post-type", "get", it.Object, "--field=name"); err != nil {
				missing = append(missing, fmt.Sprintf("post type %q (archive link %q) doesn't exist on the remote", it.Object, it.Label))
			}
		case "custom":
			op.URL = ReplaceDomain(it.Link, opts.LocalDomain, opts.RemoteDomain)
		default:
			missing = append(missing, fmt.Sprintf("item %q has unsupported type %q", it.Label, it.Type))
		}
		p.Ops = append(p.Ops, op)
	}

	if len(missing) > 0 {
		return fmt.Errorf("menu %q links to things the remote doesn't have; sync these first:\n  - %s", p.Local.Name, strings.Join(missing, "\n  - "))
	}
	return nil
}

func (op MenuOp) signature() string {
	switch op.Local.Type {
	case "custom":
		return "custom|" + op.URL
	case "post_type_archive":
		return "post_type_archive|" + op.Local.Object
	}
	return op.Local.Type + "|" + op.Local.Object + "|" + itoa(op.RemoteObjectID)
}

func remoteSignature(it MenuItem) string {
	switch it.Type {
	case "custom":
		return "custom|" + it.Link
	case "post_type_archive":
		return "post_type_archive|" + it.Object
	}
	return it.Type + "|" + it.Object + "|" + itoa(int(it.ObjectID))
}

// diff pairs local items with remote ones and works out field changes.
func (p *MenuPlan) diff() {
	used := map[int]bool{}
	for i := range p.Ops {
		sig := p.Ops[i].signature()
		for _, rit := range p.RemoteItems {
			if !used[int(rit.DBID)] && remoteSignature(rit) == sig {
				used[int(rit.DBID)] = true
				p.Ops[i].RemoteID = int(rit.DBID)
				break
			}
		}
	}
	for _, rit := range p.RemoteItems {
		if !used[int(rit.DBID)] {
			p.Removals = append(p.Removals, rit)
		}
	}

	byID := map[int]MenuItem{}
	for _, rit := range p.RemoteItems {
		byID[int(rit.DBID)] = rit
	}
	for i := range p.Ops {
		op := &p.Ops[i]
		if op.RemoteID == 0 {
			continue
		}
		rit := byID[op.RemoteID]
		l := op.Local
		if l.Title != rit.Title {
			op.Changes = append(op.Changes, "title")
		}
		if l.Position != rit.Position {
			op.Changes = append(op.Changes, "position")
		}
		if parent, known := p.remoteParent(l); !known || parent != int(rit.Parent) {
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

// remoteParent returns the remote item ID an item's parent maps to; known
// is false when the parent is still to be added (so its ID isn't known yet).
func (p *MenuPlan) remoteParent(it MenuItem) (int, bool) {
	if it.Parent == 0 {
		return 0, true
	}
	for _, op := range p.Ops {
		if op.Local.DBID == it.Parent {
			return op.RemoteID, op.RemoteID != 0
		}
	}
	return 0, true // parent not in the menu: top level
}

func (p *MenuPlan) checkDrift(lastPull time.Time) {
	if lastPull.IsZero() {
		return
	}
	for _, it := range p.RemoteItems {
		if t, ok := parseGMT(it.Modified); ok && t.After(lastPull.UTC()) {
			p.Drift = append(p.Drift, fmt.Sprintf("remote menu item %q was modified %s UTC, after your last database pull (%s UTC)", it.Label, it.Modified, lastPull.UTC().Format("2006-01-02 15:04:05")))
		}
	}
}

// planLocations assigns the local menu's theme locations on the remote,
// but only ones that exist there and are free.
func (p *MenuPlan) planLocations(r Runner, remoteMenus []MenuRef) error {
	if len(p.Local.Locations) == 0 {
		return nil
	}
	out, err := r.Remote("menu", "location", "list", "--format=json")
	if err != nil {
		return fmt.Errorf("listing remote menu locations: %w", err)
	}
	var locs []struct {
		Location string `json:"location"`
	}
	if strings.TrimSpace(out) != "" {
		if err := parseJSON(out, &locs); err != nil {
			return err
		}
	}
	registered := map[string]bool{}
	for _, l := range locs {
		registered[l.Location] = true
	}
	owner := map[string]MenuRef{}
	for _, m := range remoteMenus {
		for _, l := range m.Locations {
			owner[l] = m
		}
	}
	for _, loc := range p.Local.Locations {
		switch m, taken := owner[loc]; {
		case !registered[loc]:
			p.Warnings = append(p.Warnings, fmt.Sprintf("theme location %q isn't registered on the remote; not assigned", loc))
		case taken && p.Remote != nil && m.TermID == p.Remote.TermID:
			// Already assigned to this menu.
		case taken:
			p.Warnings = append(p.Warnings, fmt.Sprintf("remote location %q already shows menu %q; left unchanged", loc, m.Name))
		default:
			p.Locations = append(p.Locations, loc)
		}
	}
	return nil
}

// MenuResult describes what ApplyMenu actually did, including after a failure.
type MenuResult struct {
	MenuID     int
	Created    bool
	BackupPath string
	Done       []string
}

// MenuEditURL is the remote wp-admin screen for the menu.
func (p *MenuPlan) MenuEditURL(id int) string {
	return strings.TrimRight(p.SiteURL, "/") + "/wp-admin/nav-menus.php?action=edit&menu=" + itoa(id)
}

// ApplyMenu carries out the plan. Remote items missing locally are deleted
// only if removeLeftovers is set. On error, the result says what was
// already done.
func ApplyMenu(r Runner, p *MenuPlan, removeLeftovers bool, backupDir string) (*MenuResult, error) {
	res := &MenuResult{}
	user := "--user=" + p.RemoteUser

	if p.Remote != nil {
		path, err := WriteMenuBackup(backupDir, p)
		if err != nil {
			return res, fmt.Errorf("backing up the remote menu (nothing was changed): %w", err)
		}
		res.BackupPath = path
		res.MenuID = int(p.Remote.TermID)
		res.Done = append(res.Done, "backed up remote menu to "+path)
	} else {
		out, err := r.Remote("menu", "create", p.Local.Name, "--porcelain", user)
		if err != nil {
			return res, fmt.Errorf("creating remote menu: %w", err)
		}
		if res.MenuID, err = parseID(out); err != nil {
			return res, fmt.Errorf("creating remote menu: %w", err)
		}
		res.Created = true
		res.Done = append(res.Done, fmt.Sprintf("created remote menu %q (ID %d)", p.Local.Name, res.MenuID))
	}
	menu := itoa(res.MenuID)

	// Local item ID → remote item ID, filled in as items are paired or added.
	idMap := map[int]int{}
	for _, op := range p.Ops {
		if op.RemoteID != 0 {
			idMap[int(op.Local.DBID)] = op.RemoteID
		}
	}
	var parentFixes []MenuOp

	for _, op := range p.Ops {
		l := op.Local
		parent := 0
		if l.Parent != 0 {
			if id, ok := idMap[int(l.Parent)]; ok {
				parent = id
			} else if p.hasLocalItem(int(l.Parent)) {
				parentFixes = append(parentFixes, op) // parent comes later
			}
		}

		if op.RemoteID == 0 {
			id, err := addMenuItem(r, menu, op, parent, user)
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
		if err := updateMenuItem(r, op, parent, user); err != nil {
			return res, fmt.Errorf("updating menu item %q: %w", l.Label, err)
		}
		res.Done = append(res.Done, fmt.Sprintf("updated %q (%s)", l.Label, strings.Join(op.Changes, ", ")))
	}

	for _, op := range parentFixes {
		id := idMap[int(op.Local.DBID)]
		parent := idMap[int(op.Local.Parent)]
		if _, err := r.Remote("post", "meta", "update", itoa(id), "_menu_item_menu_item_parent", itoa(parent), user); err != nil {
			return res, fmt.Errorf("setting parent of %q: %w", op.Local.Label, err)
		}
	}

	if removeLeftovers && len(p.Removals) > 0 {
		args := []string{"menu", "item", "delete"}
		for _, it := range p.Removals {
			args = append(args, itoa(int(it.DBID)))
		}
		if _, err := r.Remote(append(args, user)...); err != nil {
			return res, fmt.Errorf("removing old menu items: %w", err)
		}
		res.Done = append(res.Done, fmt.Sprintf("removed %d old remote items", len(p.Removals)))
	}

	for _, loc := range p.Locations {
		if _, err := r.Remote("menu", "location", "assign", menu, loc, user); err != nil {
			return res, fmt.Errorf("assigning location %q: %w", loc, err)
		}
		res.Done = append(res.Done, "assigned to location "+loc)
	}
	return res, nil
}

func (p *MenuPlan) hasLocalItem(id int) bool {
	for _, op := range p.Ops {
		if int(op.Local.DBID) == id {
			return true
		}
	}
	return false
}

func addMenuItem(r Runner, menu string, op MenuOp, parent int, user string) (int, error) {
	l := op.Local
	var args []string
	switch l.Type {
	case "post_type":
		args = []string{"menu", "item", "add-post", menu, itoa(op.RemoteObjectID)}
	case "taxonomy":
		args = []string{"menu", "item", "add-term", menu, l.Object, itoa(op.RemoteObjectID)}
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
	out, err := r.Remote(append(args, "--porcelain", user)...)
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
			if _, err := r.Remote("post", "meta", "update", sid, k, v, user); err != nil {
				return id, err
			}
		}
		if l.Title == "" {
			if _, err := r.Remote("post", "update", sid, "--post_title=", user); err != nil {
				return id, err
			}
		}
	}
	if l.XFN != "" {
		if _, err := r.Remote("post", "meta", "update", sid, "_menu_item_xfn", l.XFN, user); err != nil {
			return id, err
		}
	}
	return id, nil
}

// updateMenuItem writes only the changed fields, straight to the item's
// post fields and meta (wp-cli's `menu item update` can't blank a title or
// move an item to another parent).
func updateMenuItem(r Runner, op MenuOp, parent int, user string) error {
	l := op.Local
	id := itoa(op.RemoteID)
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
			_, err = r.Remote("post", "meta", "update", id, "_menu_item_menu_item_parent", itoa(parent), user)
		case "target":
			_, err = r.Remote("post", "meta", "update", id, "_menu_item_target", l.Target, user)
		case "xfn":
			_, err = r.Remote("post", "meta", "update", id, "_menu_item_xfn", l.XFN, user)
		case "classes":
			classes, _ := json.Marshal(append([]string{}, l.Classes...))
			if len(l.Classes) == 0 {
				classes = []byte(`[""]`) // WordPress's own empty value
			}
			_, err = r.RemoteStdin(string(classes), "post", "meta", "update", id, "_menu_item_classes", "--format=json", user)
		}
		if err != nil {
			return err
		}
	}
	if len(post) > 3 {
		if _, err := r.Remote(append(post, user)...); err != nil {
			return err
		}
	}
	return nil
}

// WriteMenuBackup saves the remote menu, its items and every menu's
// theme locations as JSON in dir.
func WriteMenuBackup(dir string, p *MenuPlan) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(map[string]any{
		"site":    p.SiteURL,
		"takenAt": time.Now().UTC().Format(time.RFC3339),
		"menu":    p.Remote,
		// Every remote menu with its theme locations, i.e. nav_menu_locations.
		"allMenus": p.RemoteMenus,
		"items":    backupItems(p.RemoteItems),
	}, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-menu-%s.json", time.Now().Format("20060102-150405"), p.Remote.Slug))
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
