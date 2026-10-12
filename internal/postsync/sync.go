package postsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Options configure one sync run.
type Options struct {
	Ident        string // local post ID or slug
	PostType     string // narrows a slug lookup; "" for any
	Lang         string // Polylang language narrowing a slug lookup; "" for any
	Meta         bool   // also copy custom post meta
	As           string // remote user login to write as; "" picks an administrator
	LocalDomain  string
	RemoteDomain string
	RemotePath   string
	// LastPull is when the local database was last pulled from the remote
	// (zero if unknown). A remote post modified after it may contain
	// changes the local copy doesn't have.
	LastPull time.Time
	// ChooseRemote picks which remote post to update when several match
	// the local post's type and slug (and language), returning its ID. An
	// error aborts the sync. If nil, several matches abort the sync.
	ChooseRemote func(local *Post, candidates []RemoteCandidate) (int, error)
	// RemoteID, if set, is the remote post to update, instead of matching
	// one by type and slug.
	RemoteID int
}

// RemoteCandidate is one of several remote posts matching the local post.
type RemoteCandidate struct {
	ID       int
	Title    string
	Status   string
	Modified string // post_modified_gmt
}

// MetaItem is a custom meta value --meta will write to the remote.
type MetaItem struct {
	Key   string
	Value json.RawMessage // JSON, domain-rewritten (IDs not yet remapped)
	Kind  string          // ACF kind (acfMedia/acfPost) whose IDs get remapped; "" for none
}

// Plan is everything a sync would do, worked out without changing anything.
type Plan struct {
	Local          *Post
	LocalThumbnail int
	LocalTemplate  string

	Remote          *Post // nil: the post will be created
	RemoteMeta      []MetaEntry
	RemoteThumbnail int
	RemoteTemplate  string

	RemoteSiteURL string
	RemoteUser    string
	CreateParent  int // remote parent ID for a created post
	CreateAuthor  int // remote author ID for a created post (0: RemoteUser)

	Media []MediaItem
	Files []FileItem
	Meta  []MetaItem

	PLL           *PLLPlan    // nil unless the post has a Polylang language
	remotePLLInfo *PLLInfo    // remote post's language/translations, for the backup
	ACF           *ACFPlan    // nil unless ACF fields hold IDs
	PostMap       map[int]int // local → remote post IDs referenced by ACF fields
	Forms         []FormMapping

	site *Site

	// Drift lists reasons the remote post may have changed since the
	// local copy was taken.
	Drift    []string
	Warnings []string
	// InSync is set when the remote already matches and there's nothing to do.
	InSync bool
}

// Creating reports whether the plan creates a new remote post.
func (p *Plan) Creating() bool { return p.Remote == nil }

// Uploads returns the files that will be uploaded.
func (p *Plan) Uploads() []string {
	var out []string
	for _, f := range p.Files {
		if f.State == FileMissing {
			out = append(out, f.RelPath)
		}
	}
	return out
}

// Imports returns the attachments that will be added to the remote media library.
func (p *Plan) Imports() []MediaItem {
	var out []MediaItem
	for _, m := range p.Media {
		if m.RemoteID == 0 && m.RelPath != "" {
			out = append(out, m)
		}
	}
	return out
}

// Site is what preflight learned about the remote.
type Site struct {
	URL       string
	Warnings  []string
	LocalPLL  *PLLState // nil: Polylang isn't active locally
	RemotePLL *PLLState // nil: Polylang isn't active on the remote
}

// preflight checks that both sites are reachable single-site installs and
// reads the remote siteurl. It never writes.
func preflight(r Runner, opts Options) (*Site, error) {
	if _, err := r.Local("core", "is-installed"); err != nil {
		return nil, fmt.Errorf("local WordPress isn't reachable (is the project running? try `jcore start`): %w", err)
	}
	if _, err := r.Local("core", "is-installed", "--network"); err == nil {
		return nil, errors.New("the local site is a multisite network, which sync doesn't support yet")
	}

	site := &Site{}
	out, err := r.Remote("option", "get", "siteurl")
	if err != nil {
		return nil, fmt.Errorf("couldn't run wp-cli on the remote over SSH: %w", err)
	}
	site.URL = lastLine(out)
	if _, err := r.Remote("core", "is-installed", "--network"); err == nil {
		return nil, errors.New("the remote site is a multisite network, which sync doesn't support yet")
	}
	if u, err := url.Parse(site.URL); err == nil && opts.RemoteDomain != "" &&
		!strings.EqualFold(strings.TrimPrefix(u.Host, "www."), strings.TrimPrefix(opts.RemoteDomain, "www.")) {
		site.Warnings = append(site.Warnings, fmt.Sprintf("remote siteurl is %s but remoteDomain is %q; URLs will be rewritten to %q", site.URL, opts.RemoteDomain, opts.RemoteDomain))
	}
	if site.LocalPLL, err = detectPolylang(r.Local); err != nil {
		return nil, fmt.Errorf("checking for Polylang locally: %w", err)
	}
	if site.RemotePLL, err = detectPolylang(r.Remote); err != nil {
		return nil, fmt.Errorf("checking for Polylang on the remote: %w", err)
	}
	return site, nil
}

// Prepare inspects both sites and returns the sync plan. It never writes.
func Prepare(r Runner, opts Options) (*Plan, error) {
	site, err := preflight(r, opts)
	if err != nil {
		return nil, err
	}
	p := &Plan{RemoteSiteURL: site.URL, Warnings: site.Warnings, site: site, PostMap: map[int]int{}}
	mapper := newPostMapper(r, site)

	// Local post.
	local, err := resolveLocalPost(r, opts.Ident, opts.PostType, opts.Lang, site.LocalPLL)
	if err != nil {
		return nil, err
	}
	if !canSyncType(local.PostType) {
		return nil, fmt.Errorf("post %d is a %s, which can't be synced", local.ID, local.PostType)
	}
	if opts.PostType != "" && local.PostType != opts.PostType {
		return nil, fmt.Errorf("post %d is a %s, not a %s", local.ID, local.PostType, opts.PostType)
	}
	if local.PostStatus == "auto-draft" || local.PostStatus == "trash" {
		return nil, fmt.Errorf("post %d is %s; only real posts can be synced", local.ID, local.PostStatus)
	}
	if local.PostName == "" {
		return nil, fmt.Errorf("post %d has no slug yet (an unsaved draft?); give it a slug locally first, since the remote post is matched by slug", local.ID)
	}
	p.Local = local

	localMeta, err := getMeta(r.Local, int(local.ID))
	if err != nil {
		return nil, fmt.Errorf("reading local post meta: %w", err)
	}
	p.LocalThumbnail = metaInt(localMeta, "_thumbnail_id")
	p.LocalTemplate = metaString(localMeta, "_wp_page_template")

	// Polylang: the post's language decides which remote post it matches.
	var localTranslations map[string]int
	if site.LocalPLL != nil {
		info, err := pllInfo(r.Local, "post", []int{int(local.ID)})
		if err != nil {
			return nil, fmt.Errorf("reading the local post's Polylang language: %w", err)
		}
		li := info[int(local.ID)]
		switch {
		case !li.Translated:
		case li.Lang == "":
			p.Warnings = append(p.Warnings, "the local post has no Polylang language; it's synced without one")
		case site.RemotePLL == nil:
			return nil, fmt.Errorf("the local post is in language %q, but Polylang isn't active on the remote", li.Lang)
		case !site.RemotePLL.HasLanguage(li.Lang):
			return nil, fmt.Errorf("language %q isn't set up in Polylang on the remote (it has: %s)", li.Lang, strings.Join(site.RemotePLL.Languages, ", "))
		default:
			p.PLL = &PLLPlan{Lang: li.Lang}
			localTranslations = li.Translations
		}
	}

	// Remote counterpart: the one given by ID, or matched by type + slug
	// (+ language).
	if _, err := r.Remote("post-type", "get", local.PostType, "--field=name"); err != nil {
		return nil, fmt.Errorf("post type %q doesn't exist on the remote", local.PostType)
	}
	var remoteInfo map[int]PLLInfo
	if opts.RemoteID != 0 {
		p.Remote, remoteInfo, err = p.targetRemote(r, opts.RemoteID)
	} else {
		p.Remote, remoteInfo, err = p.matchRemote(r, opts)
	}
	if err != nil {
		return nil, err
	}
	if p.Remote != nil {
		if p.RemoteMeta, err = getMeta(r.Remote, int(p.Remote.ID)); err != nil {
			return nil, fmt.Errorf("reading remote post meta: %w", err)
		}
		p.RemoteThumbnail = metaInt(p.RemoteMeta, "_thumbnail_id")
		p.RemoteTemplate = metaString(p.RemoteMeta, "_wp_page_template")
		p.checkDrift(opts.LastPull)
	}

	if p.PLL != nil {
		var remoteGroup map[string]int
		if p.Remote != nil {
			info := remoteInfo[int(p.Remote.ID)]
			p.remotePLLInfo = &info
			remoteGroup = info.Translations
		}
		if err := p.planTranslations(r, localTranslations, remoteGroup); err != nil {
			return nil, err
		}
	}

	// Remote user to write as. Running without one would let kses strip
	// markup from the content.
	if p.RemoteUser, err = remoteUser(r, opts.As); err != nil {
		return nil, err
	}

	if p.Creating() {
		if err := p.resolveCreateRelations(r, mapper); err != nil {
			return nil, err
		}
	}

	// ACF fields holding attachment or post IDs.
	if p.ACF, err = planACF(r, p, local.PostContent, localMeta, opts.Meta); err != nil {
		return nil, err
	}
	var missing []string
	if p.ACF != nil {
		for _, id := range p.ACF.PostIDs {
			res, err := mapper.post(id)
			if err != nil {
				return nil, err
			}
			if res.problem != "" {
				missing = append(missing, res.problem)
				continue
			}
			p.PostMap[id] = res.id
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("ACF fields link to posts the remote doesn't have; sync these first:\n  - %s", strings.Join(missing, "\n  - "))
	}

	// Gravity Forms embedded in the content.
	if err := planForms(r, p, local.PostContent); err != nil {
		return nil, err
	}

	// Media.
	var extra []int
	if p.LocalThumbnail > 0 {
		extra = append(extra, p.LocalThumbnail)
	}
	if p.ACF != nil {
		extra = append(extra, p.ACF.MediaIDs...)
	}
	if err := planMedia(r, p, local.PostContent, extra, opts.LocalDomain); err != nil {
		return nil, err
	}

	if links := CollectLocalRefs(local.PostContent+" "+local.PostExcerpt, opts.LocalDomain).Links; len(links) > 0 {
		if len(links) > 10 {
			links = append(links[:10], fmt.Sprintf("… and %d more", len(links)-10))
		}
		p.Warnings = append(p.Warnings, fmt.Sprintf("internal links will be rewritten to %s; make sure these exist there: %s", opts.RemoteDomain, strings.Join(links, ", ")))
	}

	if opts.Meta {
		p.planMeta(localMeta, opts)
	}

	p.InSync = p.computeInSync(opts)
	return p, nil
}

// matchRemote finds the remote post with the local post's type and slug
// (and language), asking opts.ChooseRemote if several match. It returns nil
// if there is none, so the post is created.
func (p *Plan) matchRemote(r Runner, opts Options) (*Post, map[int]PLLInfo, error) {
	local := p.Local
	matches, err := listPosts(r.Remote, local.PostType, local.PostName, "any", langArgs(p.site.RemotePLL)...)
	if err != nil {
		return nil, nil, fmt.Errorf("looking up the remote post: %w", err)
	}
	var remoteInfo map[int]PLLInfo
	if p.PLL != nil && len(matches) > 0 {
		ids := make([]int, len(matches))
		for i, m := range matches {
			ids[i] = int(m.ID)
		}
		if remoteInfo, err = pllInfo(r.Remote, "post", ids); err != nil {
			return nil, nil, fmt.Errorf("reading remote Polylang languages: %w", err)
		}
		var same []postRef
		var noLang []string
		for _, m := range matches {
			switch remoteInfo[int(m.ID)].Lang {
			case p.PLL.Lang:
				same = append(same, m)
			case "":
				noLang = append(noLang, itoa(int(m.ID)))
			}
		}
		if len(same) == 0 && len(noLang) > 0 {
			return nil, nil, fmt.Errorf("remote %s %s has slug %q but no Polylang language; set its language in wp-admin first (refusing to guess)", local.PostType, strings.Join(noLang, ", "), local.PostName)
		}
		matches = same
	}
	if len(matches) == 0 {
		trashed, _ := listPosts(r.Remote, local.PostType, local.PostName, "trash", langArgs(p.site.RemotePLL)...)
		if len(trashed) > 0 {
			p.Warnings = append(p.Warnings, fmt.Sprintf("a trashed remote %s with slug %q exists (ID %d); it's left alone and a new post is created", local.PostType, local.PostName, trashed[0].ID))
		}
		return nil, remoteInfo, nil
	}
	remoteID := int(matches[0].ID)
	if len(matches) > 1 {
		if remoteID, err = chooseRemote(opts, local, matches); err != nil {
			return nil, nil, err
		}
		p.Warnings = append(p.Warnings, fmt.Sprintf("%d remote %s posts have slug %q; updating the one you chose (ID %d)", len(matches), local.PostType, local.PostName, remoteID))
	}
	remote, err := getPost(r.Remote, remoteID)
	if err != nil {
		return nil, nil, fmt.Errorf("reading remote post %d: %w", remoteID, err)
	}
	return remote, remoteInfo, nil
}

// targetRemote reads the remote post given by --remote-id, checking it can
// stand in for the local post: same type, not trashed and, with Polylang,
// the same language. Its slug may differ; it's kept as is.
func (p *Plan) targetRemote(r Runner, id int) (*Post, map[int]PLLInfo, error) {
	local := p.Local
	remote, err := getPost(r.Remote, id)
	if err != nil {
		return nil, nil, fmt.Errorf("remote post %d not found: %w", id, err)
	}
	if remote.PostType != local.PostType {
		return nil, nil, fmt.Errorf("remote post %d is a %s, but the local post is a %s", id, remote.PostType, local.PostType)
	}
	if remote.PostStatus == "auto-draft" || remote.PostStatus == "trash" {
		return nil, nil, fmt.Errorf("remote post %d is %s; restore it in wp-admin first", id, remote.PostStatus)
	}
	var remoteInfo map[int]PLLInfo
	if p.PLL != nil {
		if remoteInfo, err = pllInfo(r.Remote, "post", []int{id}); err != nil {
			return nil, nil, fmt.Errorf("reading the remote post's Polylang language: %w", err)
		}
		switch l := remoteInfo[id].Lang; l {
		case p.PLL.Lang:
		case "":
			return nil, nil, fmt.Errorf("remote post %d has no Polylang language; set it to %s in wp-admin first", id, p.PLL.Lang)
		default:
			return nil, nil, fmt.Errorf("remote post %d is in language %q, but the local post is in %q", id, l, p.PLL.Lang)
		}
	}
	if remote.PostName != local.PostName {
		p.Warnings = append(p.Warnings, fmt.Sprintf("remote post %d has slug %q, not %q; its slug is kept", id, remote.PostName, local.PostName))
	}
	return remote, remoteInfo, nil
}

// chooseRemote asks opts.ChooseRemote which of several matching remote
// posts to update.
func chooseRemote(opts Options, local *Post, matches []postRef) (int, error) {
	ids := make([]string, len(matches))
	candidates := make([]RemoteCandidate, len(matches))
	for i, m := range matches {
		ids[i] = itoa(int(m.ID))
		candidates[i] = RemoteCandidate{ID: int(m.ID), Title: m.Title, Status: m.Status, Modified: m.Modified}
	}
	if opts.ChooseRemote == nil {
		return 0, fmt.Errorf("%d remote %s posts have slug %q (IDs %s); refusing to guess which one to overwrite", len(matches), local.PostType, local.PostName, strings.Join(ids, ", "))
	}
	id, err := opts.ChooseRemote(local, candidates)
	if err != nil {
		return 0, err
	}
	if !slices.ContainsFunc(candidates, func(c RemoteCandidate) bool { return c.ID == id }) {
		return 0, fmt.Errorf("remote post %d isn't one of the matches (IDs %s)", id, strings.Join(ids, ", "))
	}
	return id, nil
}

func (p *Plan) checkDrift(lastPull time.Time) {
	rm, ok := parseGMT(p.Remote.PostModifiedGMT)
	if !ok {
		return
	}
	if lm, ok := parseGMT(p.Local.PostModifiedGMT); ok && rm.After(lm) {
		p.Drift = append(p.Drift, fmt.Sprintf("the remote post was modified %s UTC, after the local one (%s UTC)", p.Remote.PostModifiedGMT, p.Local.PostModifiedGMT))
	}
	if !lastPull.IsZero() && rm.After(lastPull.UTC()) {
		p.Drift = append(p.Drift, fmt.Sprintf("the remote post was modified %s UTC, after your last database pull (%s UTC)", p.Remote.PostModifiedGMT, lastPull.UTC().Format("2006-01-02 15:04:05")))
	}
}

// remoteUser returns the remote login to write as: as, if it exists, or the
// first administrator.
func remoteUser(r Runner, as string) (string, error) {
	if as != "" {
		if _, err := r.Remote("user", "get", as, "--field=ID"); err != nil {
			return "", fmt.Errorf("remote user %q not found", as)
		}
		return as, nil
	}
	out, err := r.Remote("user", "list", "--role=administrator", "--field=user_login", "--number=1", "--orderby=ID", "--order=ASC")
	if err != nil || strings.TrimSpace(out) == "" {
		return "", errors.New("couldn't find an administrator on the remote to write as; pass --as <login>")
	}
	return lastLine(out), nil
}

// resolveCreateRelations maps the local parent and author to the remote by
// slug and login. Failures only produce warnings.
func (p *Plan) resolveCreateRelations(r Runner, mapper *postMapper) error {
	if parent := int(p.Local.PostParent); parent > 0 {
		res, err := mapper.post(parent)
		if err != nil {
			return err
		}
		if res.problem == "" {
			p.CreateParent = res.id
		} else {
			p.Warnings = append(p.Warnings, fmt.Sprintf("local parent (ID %d) has no match on the remote (%s); the post is created without a parent", parent, res.problem))
		}
	}
	if author := int(p.Local.PostAuthor); author > 0 {
		if login, err := r.Local("user", "get", itoa(author), "--field=user_login"); err == nil {
			if out, err := r.Remote("user", "get", lastLine(login), "--field=ID"); err == nil {
				p.CreateAuthor, _ = parseID(out)
			}
		}
	}
	return nil
}

// planTranslations works out the remote translation group: the remote
// post's existing group, plus the remote counterparts of the local post's
// translations. Existing remote links are never removed or replaced, and a
// counterpart already linked elsewhere in a conflicting way is left alone.
func (p *Plan) planTranslations(r Runner, localTr, remoteGroup map[string]int) error {
	lang := p.PLL.Lang
	group := map[string]int{}
	for l, id := range remoteGroup {
		group[l] = id
	}
	self := 0
	if p.Remote != nil {
		self = int(p.Remote.ID)
	}
	group[lang] = self

	for _, l2 := range sortedKeys(localTr) {
		if l2 == lang {
			continue
		}
		if _, linked := group[l2]; linked {
			continue // the remote already has a linked translation in l2
		}
		lp, err := getPost(r.Local, localTr[l2])
		if err != nil || lp.PostName == "" {
			continue
		}
		refs, err := listPosts(r.Remote, lp.PostType, lp.PostName, "any", langArgs(p.site.RemotePLL)...)
		if err != nil {
			return fmt.Errorf("looking up the %s translation on the remote: %w", l2, err)
		}
		ids := make([]int, len(refs))
		for i, ref := range refs {
			ids[i] = int(ref.ID)
		}
		if ids, err = filterByLang(r.Remote, "post", ids, l2); err != nil {
			return err
		}
		if len(ids) != 1 {
			if len(ids) == 0 {
				p.Warnings = append(p.Warnings, fmt.Sprintf("the %s translation (%q) isn't on the remote yet; it'll be linked when you sync it", l2, lp.PostName))
			} else {
				p.Warnings = append(p.Warnings, fmt.Sprintf("several remote %s posts have slug %q; that translation isn't linked", l2, lp.PostName))
			}
			continue
		}
		info, err := pllInfo(r.Remote, "post", ids)
		if err != nil {
			return err
		}
		candidate := map[string]int{l2: ids[0]}
		for l, id := range info[ids[0]].Translations {
			candidate[l] = id
		}
		if mergeGroup(group, candidate) {
			p.PLL.NewLink = append(p.PLL.NewLink, l2)
		} else {
			p.Warnings = append(p.Warnings, fmt.Sprintf("the remote %s translation (ID %d) is already linked to other posts; not linked", l2, ids[0]))
		}
	}

	p.PLL.SetLang = p.Remote == nil
	if p.PLL.SetLang || len(p.PLL.NewLink) > 0 {
		p.PLL.Group = group
	}
	if len(group) > 1 && len(p.site.RemotePLL.Sync) > 0 {
		p.Warnings = append(p.Warnings, "Polylang on the remote copies these between translations, so linked translations may change too: "+strings.Join(p.site.RemotePLL.Sync, ", "))
	}
	return nil
}

func (p *Plan) planMeta(localMeta []MetaEntry, opts Options) {
	counts := map[string]int{}
	for _, m := range localMeta {
		counts[m.Key]++
	}
	remote := map[string]json.RawMessage{}
	for _, m := range p.RemoteMeta {
		remote[m.Key] = m.Value
	}
	warned := map[string]bool{}
	var numeric []string
	for _, m := range localMeta {
		if slices.Contains(internalMetaKeys, m.Key) || strings.HasPrefix(m.Key, "_wp_attachment") {
			continue
		}
		if counts[m.Key] > 1 {
			if !warned[m.Key] {
				p.Warnings = append(p.Warnings, fmt.Sprintf("meta %q has multiple values; skipped", m.Key))
				warned[m.Key] = true
			}
			continue
		}
		item := MetaItem{Key: m.Key, Value: json.RawMessage(ReplaceDomainJSON(string(m.Value), opts.LocalDomain, opts.RemoteDomain))}
		if p.ACF != nil {
			item.Kind = p.ACF.MetaKinds[m.Key]
		}
		if rv, ok := remote[m.Key]; ok && jsonEqual(rv, p.finalMeta(item)) {
			continue
		}
		p.Meta = append(p.Meta, item)
		if item.Kind == "" && !strings.HasPrefix(m.Key, "_") && looksLikeIDs(m.Value) {
			numeric = append(numeric, m.Key)
		}
	}
	if len(numeric) > 0 {
		p.Warnings = append(p.Warnings, "these meta values look like post/attachment IDs, which are copied as-is and NOT remapped: "+strings.Join(numeric, ", "))
	}
}

// finalMeta is a meta value with ACF attachment/post IDs remapped (as far
// as known).
func (p *Plan) finalMeta(m MetaItem) json.RawMessage {
	switch m.Kind {
	case acfMedia:
		return json.RawMessage(mapToken(string(m.Value), p.idMap()))
	case acfPost:
		return json.RawMessage(mapToken(string(m.Value), p.PostMap))
	}
	return m.Value
}

func jsonEqual(a, b json.RawMessage) bool {
	var va, vb any
	if json.Unmarshal(a, &va) != nil || json.Unmarshal(b, &vb) != nil {
		return false
	}
	return reflect.DeepEqual(va, vb)
}

// ReplaceDomainJSON rewrites the domain inside every string of a JSON value.
func ReplaceDomainJSON(raw, from, to string) string {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	v = replaceInValue(v, from, to)
	b, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return string(b)
}

func replaceInValue(v any, from, to string) any {
	switch t := v.(type) {
	case string:
		return ReplaceDomain(t, from, to)
	case []any:
		for i := range t {
			t[i] = replaceInValue(t[i], from, to)
		}
	case map[string]any:
		for k := range t {
			t[k] = replaceInValue(t[k], from, to)
		}
	}
	return v
}

func looksLikeIDs(raw json.RawMessage) bool {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		_, err := strconv.Atoi(s)
		return err == nil && s != "0" && s != "1"
	}
	var arr []any
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		for _, x := range arr {
			switch e := x.(type) {
			case string:
				if _, err := strconv.Atoi(e); err != nil {
					return false
				}
			case float64:
			default:
				return false
			}
		}
		return true
	}
	return false
}

// FinalContent is the local content with attachment, ACF and form IDs
// remapped (as far as known) and local URLs pointed at the remote.
func (p *Plan) FinalContent(opts Options) string {
	idMap := p.idMap()
	c := RemapAttachmentIDs(p.Local.PostContent, idMap)
	if p.ACF != nil {
		c = RemapACFContent(c, p.ACF.ContentKeys, idMap, p.PostMap)
	}
	c = RemapFormIDs(c, p.formMap())
	return ReplaceDomain(c, opts.LocalDomain, opts.RemoteDomain)
}

func (p *Plan) finalExcerpt(opts Options) string {
	return ReplaceDomain(p.Local.PostExcerpt, opts.LocalDomain, opts.RemoteDomain)
}

func (p *Plan) mappedThumbnail() int {
	if p.LocalThumbnail == 0 {
		return 0
	}
	return p.idMap()[p.LocalThumbnail]
}

func normalizeTemplate(t string) string {
	if t == "default" {
		return ""
	}
	return t
}

func (p *Plan) computeInSync(opts Options) bool {
	if p.Remote == nil || len(p.Uploads()) > 0 || len(p.Imports()) > 0 || len(p.Meta) > 0 {
		return false
	}
	if p.PLL != nil && p.PLL.Group != nil {
		return false
	}
	if strings.TrimSpace(p.FinalContent(opts)) != strings.TrimSpace(p.Remote.PostContent) ||
		p.Local.PostTitle != p.Remote.PostTitle ||
		p.finalExcerpt(opts) != p.Remote.PostExcerpt {
		return false
	}
	if t := p.mappedThumbnail(); t != 0 && t != p.RemoteThumbnail {
		return false
	}
	if normalizeTemplate(p.LocalTemplate) != normalizeTemplate(p.RemoteTemplate) {
		return false
	}
	return true
}

// Result describes what Apply actually did, including after a failure.
type Result struct {
	RemoteID   int
	Created    bool
	BackupPath string
	Uploaded   []string
	Imported   []string
	Done       []string
	// VerifyMismatch is set when the remote content read back after the
	// write differs from what was sent (e.g. markup stripped on save).
	VerifyMismatch bool
	// FinalSlug is the remote post's slug after the write, which WordPress
	// may have changed to keep it unique.
	FinalSlug string
}

// EditURL is the remote wp-admin edit screen for the synced post.
func (p *Plan) EditURL(id int) string {
	return strings.TrimRight(p.RemoteSiteURL, "/") + "/wp-admin/post.php?post=" + itoa(id) + "&action=edit"
}

// Apply carries out the plan. backupDir receives a snapshot of the remote
// post before it's overwritten. On error, the returned Result says what was
// already done.
func Apply(r Runner, p *Plan, opts Options, backupDir string) (*Result, error) {
	res := &Result{}

	if p.Remote != nil {
		path, err := WriteBackup(backupDir, p)
		if err != nil {
			return res, fmt.Errorf("backing up the remote post (nothing was changed): %w", err)
		}
		res.BackupPath = path
		res.RemoteID = int(p.Remote.ID)
		res.Done = append(res.Done, "backed up remote post to "+path)
	}

	for _, rel := range p.Uploads() {
		if err := r.Upload(rel); err != nil {
			return res, fmt.Errorf("uploading %s: %w", rel, err)
		}
		res.Uploaded = append(res.Uploaded, rel)
		res.Done = append(res.Done, "uploaded "+rel)
	}

	for i, m := range p.Media {
		if m.RemoteID != 0 || m.RelPath == "" {
			continue
		}
		id, err := importAttachment(r, opts.RemotePath, p.RemoteUser, m)
		if err != nil {
			return res, fmt.Errorf("adding %s to the remote media library: %w", m.RelPath, err)
		}
		p.Media[i].RemoteID = id
		if pll := p.site.RemotePLL; pll != nil && pll.Media && p.PLL != nil {
			if _, err := r.Remote("eval", fmt.Sprintf(`pll_set_post_language(%d, "%s"); echo "ok";`, id, p.PLL.Lang), "--user="+p.RemoteUser); err != nil {
				return res, fmt.Errorf("setting the language of %s: %w", m.RelPath, err)
			}
		}
		res.Imported = append(res.Imported, m.RelPath)
		res.Done = append(res.Done, fmt.Sprintf("added %s to the media library (ID %d)", m.RelPath, id))
	}

	content := p.FinalContent(opts)
	userArg := "--user=" + p.RemoteUser
	if p.Remote != nil {
		_, err := r.RemoteStdin(content, "post", "update", itoa(res.RemoteID), "-",
			"--post_title="+p.Local.PostTitle, "--post_excerpt="+p.finalExcerpt(opts), userArg)
		if err != nil {
			return res, fmt.Errorf("updating remote post %d: %w", res.RemoteID, err)
		}
		res.Done = append(res.Done, fmt.Sprintf("updated remote post %d", res.RemoteID))
	} else {
		args := []string{"post", "create", "-",
			"--post_type=" + p.Local.PostType, "--post_name=" + p.Local.PostName,
			"--post_title=" + p.Local.PostTitle, "--post_excerpt=" + p.finalExcerpt(opts),
			"--post_status=draft", "--porcelain", userArg}
		if p.CreateParent > 0 {
			args = append(args, "--post_parent="+itoa(p.CreateParent))
		}
		if p.CreateAuthor > 0 {
			args = append(args, "--post_author="+itoa(p.CreateAuthor))
		}
		out, err := r.RemoteStdin(content, args...)
		if err != nil {
			return res, fmt.Errorf("creating remote post: %w", err)
		}
		id, err := parseID(out)
		if err != nil {
			return res, fmt.Errorf("creating remote post: %w", err)
		}
		res.RemoteID, res.Created = id, true
		res.Done = append(res.Done, fmt.Sprintf("created remote draft %d", id))
	}

	// Language first, so Polylang treats the following meta updates (and
	// the slug) as belonging to the right language.
	if p.PLL != nil && p.PLL.Group != nil {
		if _, err := r.Remote("eval", pllSaveGroupPHP(res.RemoteID, p.PLL.Lang, p.Local.PostName, p.PLL.Group, p.PLL.SetLang), userArg); err != nil {
			return res, fmt.Errorf("saving the Polylang language/translations: %w", err)
		}
		if p.PLL.SetLang {
			res.Done = append(res.Done, "set language "+p.PLL.Lang)
		}
		if len(p.PLL.NewLink) > 0 {
			res.Done = append(res.Done, "linked translations: "+strings.Join(p.PLL.NewLink, ", "))
		}
	}

	id := itoa(res.RemoteID)
	if t := p.mappedThumbnail(); t != 0 && t != p.RemoteThumbnail {
		if _, err := r.Remote("post", "meta", "update", id, "_thumbnail_id", itoa(t), userArg); err != nil {
			return res, fmt.Errorf("setting featured image: %w", err)
		}
		res.Done = append(res.Done, "set featured image")
	}
	if tpl := normalizeTemplate(p.LocalTemplate); tpl != normalizeTemplate(p.RemoteTemplate) && tpl != "" {
		if _, err := r.Remote("post", "meta", "update", id, "_wp_page_template", tpl, userArg); err != nil {
			return res, fmt.Errorf("setting page template: %w", err)
		}
		res.Done = append(res.Done, "set page template "+tpl)
	}

	for _, m := range p.Meta {
		if _, err := r.RemoteStdin(string(p.finalMeta(m)), "post", "meta", "update", id, m.Key, "--format=json", userArg); err != nil {
			return res, fmt.Errorf("writing meta %q: %w", m.Key, err)
		}
	}
	if len(p.Meta) > 0 {
		res.Done = append(res.Done, fmt.Sprintf("wrote %d meta values", len(p.Meta)))
	}

	// Read the content back: WordPress may filter it on save.
	if out, err := r.Remote("post", "get", id, "--field=post_content"); err != nil ||
		strings.TrimSpace(out) != strings.TrimSpace(content) {
		res.VerifyMismatch = true
	}
	if out, err := r.Remote("post", "get", id, "--field=post_name"); err == nil {
		res.FinalSlug = lastLine(out)
	}
	return res, nil
}

// WriteBackup saves the remote post and its meta as JSON in dir.
func WriteBackup(dir string, p *Plan) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(map[string]any{
		"site":     p.RemoteSiteURL,
		"takenAt":  time.Now().UTC().Format(time.RFC3339),
		"post":     p.Remote,
		"postMeta": p.RemoteMeta,
		"polylang": p.remotePLLInfo,
	}, "", "  ")
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s-%s-remote%d.json", time.Now().Format("20060102-150405"), p.Remote.PostName, p.Remote.ID)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
