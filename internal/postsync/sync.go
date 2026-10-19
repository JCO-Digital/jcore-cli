package postsync

import (
	"encoding/json"
	"fmt"
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
	Ident        string // source post ID or slug (local when pushing, remote when pulling)
	Pull         bool   // copy from the remote to the local site instead of pushing
	PostType     string // narrows a slug lookup; "" for any
	Lang         string // Polylang language narrowing a slug lookup; "" for any
	Meta         bool   // also copy custom post meta
	As           string // user login to write as on the target; "" picks an administrator
	LocalDomain  string
	RemoteDomain string
	RemotePath   string
	// LastPull is when the local database was last pulled from the remote
	// (zero if unknown). A target post modified after it may contain
	// changes the source doesn't have.
	LastPull time.Time
	// ChooseTarget picks which target post to update when several match
	// the source post's type and slug (and language), returning its ID. An
	// error aborts the sync. If nil, several matches abort the sync.
	ChooseTarget func(source *Post, candidates []Candidate) (int, error)
	// TargetID, if set, is the target post to update, instead of matching
	// one by type and slug.
	TargetID int
}

// Candidate is one of several posts on the target site matching the post
// being synced.
type Candidate struct {
	ID       int
	Title    string
	Status   string
	Modified string // post_modified_gmt
}

// MetaItem is a custom meta value --meta will write to the target.
type MetaItem struct {
	Key   string
	Value json.RawMessage // JSON, domain-rewritten (IDs not yet remapped)
	Kind  string          // ACF kind (acfMedia/acfPost) whose IDs get remapped; "" for none
}

// Plan is everything a sync would do, worked out without changing anything.
type Plan struct {
	Pull bool // the source is the remote and the target the local site

	Source          *Post
	SourceThumbnail int
	SourceTemplate  string

	Target          *Post // nil: the post will be created
	TargetMeta      []MetaEntry
	TargetThumbnail int
	TargetTemplate  string

	SourceSiteURL string
	TargetSiteURL string
	TargetUser    string
	CreateParent  int // target parent ID for a created post
	CreateAuthor  int // target author ID for a created post (0: TargetUser)

	Media []MediaItem
	Files []FileItem
	Meta  []MetaItem

	PLL           *PLLPlan    // nil unless the post has a Polylang language
	targetPLLInfo *PLLInfo    // target post's language/translations, for the backup
	ACF           *ACFPlan    // nil unless ACF fields hold IDs
	PostMap       map[int]int // source → target post IDs referenced by ACF fields
	Forms         []FormMapping

	f *flow

	// Drift lists reasons the target post may have changes the source
	// doesn't have.
	Drift    []string
	Warnings []string
	// InSync is set when the target already matches and there's nothing to do.
	InSync bool
}

// Creating reports whether the plan creates a new target post.
func (p *Plan) Creating() bool { return p.Target == nil }

// Copies returns the files that will be copied (uploaded or downloaded).
func (p *Plan) Copies() []string {
	var out []string
	for _, f := range p.Files {
		if f.State == FileMissing {
			out = append(out, f.RelPath)
		}
	}
	return out
}

// Imports returns the attachments that will be added to the target media library.
func (p *Plan) Imports() []MediaItem {
	var out []MediaItem
	for _, m := range p.Media {
		if m.TargetID == 0 && m.RelPath != "" {
			out = append(out, m)
		}
	}
	return out
}

// Prepare inspects both sites and returns the sync plan. It never writes.
func Prepare(r Runner, opts Options) (*Plan, error) {
	f, err := preflight(r, opts)
	if err != nil {
		return nil, err
	}
	src, dst := f.src, f.dst
	p := &Plan{Pull: f.pull, SourceSiteURL: src.url, TargetSiteURL: dst.url, Warnings: f.warnings, f: f, PostMap: map[int]int{}}
	mapper := newPostMapper(f)

	// Source post.
	source, err := resolvePost(src.run, src.name, opts.Ident, opts.PostType, opts.Lang, src.pll)
	if err != nil {
		return nil, err
	}
	if !canSyncType(source.PostType) {
		return nil, fmt.Errorf("%s post %d is a %s, which can't be synced", src.name, source.ID, source.PostType)
	}
	if opts.PostType != "" && source.PostType != opts.PostType {
		return nil, fmt.Errorf("%s post %d is a %s, not a %s", src.name, source.ID, source.PostType, opts.PostType)
	}
	if source.PostStatus == "auto-draft" || source.PostStatus == "trash" {
		return nil, fmt.Errorf("%s post %d is %s; only real posts can be synced", src.name, source.ID, source.PostStatus)
	}
	if source.PostName == "" {
		return nil, fmt.Errorf("%s post %d has no slug yet (an unsaved draft?); give it a slug %s first, since the %s post is matched by slug", src.name, source.ID, src.where, dst.name)
	}
	p.Source = source

	sourceMeta, err := getMeta(src.run, int(source.ID))
	if err != nil {
		return nil, fmt.Errorf("reading %s post meta: %w", src.name, err)
	}
	p.SourceThumbnail = metaInt(sourceMeta, "_thumbnail_id")
	p.SourceTemplate = metaString(sourceMeta, "_wp_page_template")

	// Polylang: the post's language decides which target post it matches.
	var sourceTranslations map[string]int
	if src.pll != nil {
		info, err := pllInfo(src.run, "post", []int{int(source.ID)})
		if err != nil {
			return nil, fmt.Errorf("reading the %s post's Polylang language: %w", src.name, err)
		}
		si := info[int(source.ID)]
		switch {
		case !si.Translated:
		case si.Lang == "":
			p.Warnings = append(p.Warnings, fmt.Sprintf("the %s post has no Polylang language; it's synced without one", src.name))
		case dst.pll == nil:
			return nil, fmt.Errorf("the %s post is in language %q, but Polylang isn't active %s", src.name, si.Lang, dst.where)
		case !dst.pll.HasLanguage(si.Lang):
			return nil, fmt.Errorf("language %q isn't set up in Polylang %s (it has: %s)", si.Lang, dst.where, strings.Join(dst.pll.Languages, ", "))
		default:
			p.PLL = &PLLPlan{Lang: si.Lang}
			sourceTranslations = si.Translations
		}
	}

	// Target counterpart: the one given by ID, or matched by type + slug
	// (+ language).
	if _, err := dst.run("post-type", "get", source.PostType, "--field=name"); err != nil {
		return nil, fmt.Errorf("post type %q doesn't exist %s", source.PostType, dst.where)
	}
	var targetInfo map[int]PLLInfo
	if opts.TargetID != 0 {
		p.Target, targetInfo, err = p.targetPost(opts.TargetID)
	} else {
		p.Target, targetInfo, err = p.matchTarget(opts)
	}
	if err != nil {
		return nil, err
	}
	if p.Target != nil {
		if p.TargetMeta, err = getMeta(dst.run, int(p.Target.ID)); err != nil {
			return nil, fmt.Errorf("reading %s post meta: %w", dst.name, err)
		}
		p.TargetThumbnail = metaInt(p.TargetMeta, "_thumbnail_id")
		p.TargetTemplate = metaString(p.TargetMeta, "_wp_page_template")
		p.checkDrift(opts.LastPull)
	}

	if p.PLL != nil {
		var targetGroup map[string]int
		if p.Target != nil {
			info := targetInfo[int(p.Target.ID)]
			p.targetPLLInfo = &info
			targetGroup = info.Translations
		}
		if err := p.planTranslations(sourceTranslations, targetGroup); err != nil {
			return nil, err
		}
	}

	// Target user to write as. Running without one would let kses strip
	// markup from the content.
	if p.TargetUser, err = writeUser(dst.run, dst.name, opts.As); err != nil {
		return nil, err
	}

	if p.Creating() {
		if err := p.resolveCreateRelations(mapper); err != nil {
			return nil, err
		}
	}

	// ACF fields holding attachment or post IDs.
	if p.ACF, err = planACF(p, source.PostContent, sourceMeta, opts.Meta); err != nil {
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
		return nil, fmt.Errorf("ACF fields link to posts %s doesn't have; %s these first:\n  - %s", dst.site, f.verb(), strings.Join(missing, "\n  - "))
	}

	// Gravity Forms embedded in the content.
	if err := planForms(p, source.PostContent); err != nil {
		return nil, err
	}

	// Media.
	var extra []int
	if p.SourceThumbnail > 0 {
		extra = append(extra, p.SourceThumbnail)
	}
	if p.ACF != nil {
		extra = append(extra, p.ACF.MediaIDs...)
	}
	if err := planMedia(p, source.PostContent, extra); err != nil {
		return nil, err
	}

	if links := CollectSiteRefs(source.PostContent+" "+source.PostExcerpt, src.domain).Links; len(links) > 0 {
		if len(links) > 10 {
			links = append(links[:10], fmt.Sprintf("… and %d more", len(links)-10))
		}
		p.Warnings = append(p.Warnings, fmt.Sprintf("internal links will be rewritten to %s; make sure these exist there: %s", dst.domain, strings.Join(links, ", ")))
	}

	if opts.Meta {
		p.planMeta(sourceMeta)
	}

	p.InSync = p.computeInSync()
	return p, nil
}

// matchTarget finds the target post with the source post's type and slug
// (and language), asking opts.ChooseTarget if several match. It returns
// nil if there is none, so the post is created.
func (p *Plan) matchTarget(opts Options) (*Post, map[int]PLLInfo, error) {
	source, dst := p.Source, p.f.dst
	matches, err := listPosts(dst.run, source.PostType, source.PostName, "any", langArgs(dst.pll)...)
	if err != nil {
		return nil, nil, fmt.Errorf("looking up the %s post: %w", dst.name, err)
	}
	var targetInfo map[int]PLLInfo
	if p.PLL != nil && len(matches) > 0 {
		ids := make([]int, len(matches))
		for i, m := range matches {
			ids[i] = int(m.ID)
		}
		if targetInfo, err = pllInfo(dst.run, "post", ids); err != nil {
			return nil, nil, fmt.Errorf("reading %s Polylang languages: %w", dst.name, err)
		}
		var same []postRef
		var noLang []string
		for _, m := range matches {
			switch targetInfo[int(m.ID)].Lang {
			case p.PLL.Lang:
				same = append(same, m)
			case "":
				noLang = append(noLang, itoa(int(m.ID)))
			}
		}
		if len(same) == 0 && len(noLang) > 0 {
			return nil, nil, fmt.Errorf("%s %s %s has slug %q but no Polylang language; set its language in wp-admin first (refusing to guess)", dst.name, source.PostType, strings.Join(noLang, ", "), source.PostName)
		}
		matches = same
	}
	if len(matches) == 0 {
		trashed, _ := listPosts(dst.run, source.PostType, source.PostName, "trash", langArgs(dst.pll)...)
		if len(trashed) > 0 {
			p.Warnings = append(p.Warnings, fmt.Sprintf("a trashed %s %s with slug %q exists (ID %d); it's left alone and a new post is created", dst.name, source.PostType, source.PostName, trashed[0].ID))
		}
		return nil, targetInfo, nil
	}
	targetID := int(matches[0].ID)
	if len(matches) > 1 {
		if targetID, err = choosePost(opts.ChooseTarget, dst.name, source, matches); err != nil {
			return nil, nil, err
		}
		p.Warnings = append(p.Warnings, fmt.Sprintf("%d %s %s posts have slug %q; updating the one you chose (ID %d)", len(matches), dst.name, source.PostType, source.PostName, targetID))
	}
	target, err := getPost(dst.run, targetID)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s post %d: %w", dst.name, targetID, err)
	}
	return target, targetInfo, nil
}

// targetPost reads the target post given by --remote-id/--local-id,
// checking it can stand in for the source post: same type, not trashed
// and, with Polylang, the same language. Its slug may differ; it's kept as
// is.
func (p *Plan) targetPost(id int) (*Post, map[int]PLLInfo, error) {
	source, src, dst := p.Source, p.f.src, p.f.dst
	target, err := getPost(dst.run, id)
	if err != nil {
		return nil, nil, fmt.Errorf("%s post %d not found: %w", dst.name, id, err)
	}
	if target.PostType != source.PostType {
		return nil, nil, fmt.Errorf("%s post %d is a %s, but the %s post is a %s", dst.name, id, target.PostType, src.name, source.PostType)
	}
	if target.PostStatus == "auto-draft" || target.PostStatus == "trash" {
		return nil, nil, fmt.Errorf("%s post %d is %s; restore it in wp-admin first", dst.name, id, target.PostStatus)
	}
	var targetInfo map[int]PLLInfo
	if p.PLL != nil {
		if targetInfo, err = pllInfo(dst.run, "post", []int{id}); err != nil {
			return nil, nil, fmt.Errorf("reading the %s post's Polylang language: %w", dst.name, err)
		}
		switch l := targetInfo[id].Lang; l {
		case p.PLL.Lang:
		case "":
			return nil, nil, fmt.Errorf("%s post %d has no Polylang language; set it to %s in wp-admin first", dst.name, id, p.PLL.Lang)
		default:
			return nil, nil, fmt.Errorf("%s post %d is in language %q, but the %s post is in %q", dst.name, id, l, src.name, p.PLL.Lang)
		}
	}
	if target.PostName != source.PostName {
		p.Warnings = append(p.Warnings, fmt.Sprintf("%s post %d has slug %q, not %q; its slug is kept", dst.name, id, target.PostName, source.PostName))
	}
	return target, targetInfo, nil
}

// choosePost asks choose which of several matching posts on the target site
// (side "remote" or "local") to update for post.
func choosePost(choose func(*Post, []Candidate) (int, error), side string, post *Post, matches []postRef) (int, error) {
	ids := make([]string, len(matches))
	candidates := make([]Candidate, len(matches))
	for i, m := range matches {
		ids[i] = itoa(int(m.ID))
		candidates[i] = Candidate{ID: int(m.ID), Title: m.Title, Status: m.Status, Modified: m.Modified}
	}
	if choose == nil {
		return 0, fmt.Errorf("%d %s %s posts have slug %q (IDs %s); refusing to guess which one to overwrite", len(matches), side, post.PostType, post.PostName, strings.Join(ids, ", "))
	}
	id, err := choose(post, candidates)
	if err != nil {
		return 0, err
	}
	if !slices.ContainsFunc(candidates, func(c Candidate) bool { return c.ID == id }) {
		return 0, fmt.Errorf("%s post %d isn't one of the matches (IDs %s)", side, id, strings.Join(ids, ", "))
	}
	return id, nil
}

func (p *Plan) checkDrift(lastPull time.Time) {
	tm, ok := parseGMT(p.Target.PostModifiedGMT)
	if !ok {
		return
	}
	src, dst := p.f.src, p.f.dst
	if sm, ok := parseGMT(p.Source.PostModifiedGMT); ok && tm.After(sm) {
		p.Drift = append(p.Drift, fmt.Sprintf("the %s post was modified %s UTC, after the %s one (%s UTC)", dst.name, p.Target.PostModifiedGMT, src.name, p.Source.PostModifiedGMT))
	}
	if !lastPull.IsZero() && tm.After(lastPull.UTC()) {
		p.Drift = append(p.Drift, fmt.Sprintf("the %s post was modified %s UTC, after your last database pull (%s UTC)", dst.name, p.Target.PostModifiedGMT, lastPull.UTC().Format("2006-01-02 15:04:05")))
	}
}

// writeUser returns the login to write as on the site run talks to (side
// "remote" or "local"): as, if it exists, or the first administrator.
func writeUser(run func(...string) (string, error), side, as string) (string, error) {
	if as != "" {
		if _, err := run("user", "get", as, "--field=ID"); err != nil {
			return "", fmt.Errorf("%s user %q not found", side, as)
		}
		return as, nil
	}
	out, err := run("user", "list", "--role=administrator", "--field=user_login", "--number=1", "--orderby=ID", "--order=ASC")
	if err != nil || strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("couldn't find a %s administrator to write as; pass --as <login>", side)
	}
	return lastLine(out), nil
}

// resolveCreateRelations maps the source parent and author to the target
// by slug and login. Failures only produce warnings.
func (p *Plan) resolveCreateRelations(mapper *postMapper) error {
	src, dst := p.f.src, p.f.dst
	if parent := int(p.Source.PostParent); parent > 0 {
		res, err := mapper.post(parent)
		if err != nil {
			return err
		}
		if res.problem == "" {
			p.CreateParent = res.id
		} else {
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s parent (ID %d) has no match %s (%s); the post is created without a parent", src.name, parent, dst.where, res.problem))
		}
	}
	if author := int(p.Source.PostAuthor); author > 0 {
		if login, err := src.run("user", "get", itoa(author), "--field=user_login"); err == nil {
			if out, err := dst.run("user", "get", lastLine(login), "--field=ID"); err == nil {
				p.CreateAuthor, _ = parseID(out)
			}
		}
	}
	return nil
}

// planTranslations works out the target translation group: the target
// post's existing group, plus the target counterparts of the source post's
// translations. Existing target links are never removed or replaced, and a
// counterpart already linked elsewhere in a conflicting way is left alone.
func (p *Plan) planTranslations(sourceTr, targetGroup map[string]int) error {
	src, dst := p.f.src, p.f.dst
	lang := p.PLL.Lang
	group := map[string]int{}
	for l, id := range targetGroup {
		group[l] = id
	}
	self := 0
	if p.Target != nil {
		self = int(p.Target.ID)
	}
	group[lang] = self

	for _, l2 := range sortedKeys(sourceTr) {
		if l2 == lang {
			continue
		}
		if _, linked := group[l2]; linked {
			continue // the target already has a linked translation in l2
		}
		sp, err := getPost(src.run, sourceTr[l2])
		if err != nil || sp.PostName == "" {
			continue
		}
		refs, err := listPosts(dst.run, sp.PostType, sp.PostName, "any", langArgs(dst.pll)...)
		if err != nil {
			return fmt.Errorf("looking up the %s translation %s: %w", l2, dst.where, err)
		}
		ids := make([]int, len(refs))
		for i, ref := range refs {
			ids[i] = int(ref.ID)
		}
		if ids, err = filterByLang(dst.run, "post", ids, l2); err != nil {
			return err
		}
		if len(ids) != 1 {
			if len(ids) == 0 {
				p.Warnings = append(p.Warnings, fmt.Sprintf("the %s translation (%q) doesn't exist %s yet; it'll be linked when you %s it", l2, sp.PostName, dst.where, p.f.verb()))
			} else {
				p.Warnings = append(p.Warnings, fmt.Sprintf("several %s %s posts have slug %q; that translation isn't linked", dst.name, l2, sp.PostName))
			}
			continue
		}
		info, err := pllInfo(dst.run, "post", ids)
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
			p.Warnings = append(p.Warnings, fmt.Sprintf("the %s %s translation (ID %d) is already linked to other posts; not linked", dst.name, l2, ids[0]))
		}
	}

	p.PLL.SetLang = p.Target == nil
	if p.PLL.SetLang || len(p.PLL.NewLink) > 0 {
		p.PLL.Group = group
	}
	if len(group) > 1 && len(dst.pll.Sync) > 0 {
		p.Warnings = append(p.Warnings, fmt.Sprintf("Polylang on %s copies these between translations, so linked translations may change too: %s", dst.site, strings.Join(dst.pll.Sync, ", ")))
	}
	return nil
}

func (p *Plan) planMeta(sourceMeta []MetaEntry) {
	counts := map[string]int{}
	for _, m := range sourceMeta {
		counts[m.Key]++
	}
	target := map[string]json.RawMessage{}
	for _, m := range p.TargetMeta {
		target[m.Key] = m.Value
	}
	warned := map[string]bool{}
	var numeric []string
	for _, m := range sourceMeta {
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
		item := MetaItem{Key: m.Key, Value: json.RawMessage(ReplaceDomainJSON(string(m.Value), p.f.src.domain, p.f.dst.domain))}
		if p.ACF != nil {
			item.Kind = p.ACF.MetaKinds[m.Key]
		}
		if tv, ok := target[m.Key]; ok && jsonEqual(tv, p.finalMeta(item)) {
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

// FinalContent is the source content with attachment, ACF and form IDs
// remapped (as far as known) and source URLs pointed at the target.
func (p *Plan) FinalContent() string {
	idMap := p.idMap()
	c := RemapAttachmentIDs(p.Source.PostContent, idMap)
	if p.ACF != nil {
		c = RemapACFContent(c, p.ACF.ContentKeys, idMap, p.PostMap)
	}
	c = RemapFormIDs(c, p.formMap())
	return ReplaceDomain(c, p.f.src.domain, p.f.dst.domain)
}

func (p *Plan) finalExcerpt() string {
	return ReplaceDomain(p.Source.PostExcerpt, p.f.src.domain, p.f.dst.domain)
}

func (p *Plan) mappedThumbnail() int {
	if p.SourceThumbnail == 0 {
		return 0
	}
	return p.idMap()[p.SourceThumbnail]
}

func normalizeTemplate(t string) string {
	if t == "default" {
		return ""
	}
	return t
}

func (p *Plan) computeInSync() bool {
	if p.Target == nil || len(p.Copies()) > 0 || len(p.Imports()) > 0 || len(p.Meta) > 0 {
		return false
	}
	if p.PLL != nil && p.PLL.Group != nil {
		return false
	}
	if strings.TrimSpace(p.FinalContent()) != strings.TrimSpace(p.Target.PostContent) ||
		p.Source.PostTitle != p.Target.PostTitle ||
		p.finalExcerpt() != p.Target.PostExcerpt {
		return false
	}
	if t := p.mappedThumbnail(); t != 0 && t != p.TargetThumbnail {
		return false
	}
	if normalizeTemplate(p.SourceTemplate) != normalizeTemplate(p.TargetTemplate) {
		return false
	}
	return true
}

// Result describes what Apply actually did, including after a failure.
type Result struct {
	TargetID   int
	Created    bool
	BackupPath string
	Done       []string
	// VerifyMismatch is set when the target content read back after the
	// write differs from what was sent (e.g. markup stripped on save).
	VerifyMismatch bool
	// FinalSlug is the target post's slug after the write, which WordPress
	// may have changed to keep it unique.
	FinalSlug string
}

// EditURL is the target's wp-admin edit screen for the synced post.
func (p *Plan) EditURL(id int) string {
	return strings.TrimRight(p.TargetSiteURL, "/") + "/wp-admin/post.php?post=" + itoa(id) + "&action=edit"
}

// Apply carries out the plan. backupDir receives a snapshot of the target
// post before it's overwritten. On error, the returned Result says what was
// already done.
func Apply(p *Plan, backupDir string) (*Result, error) {
	f := p.f
	dst := f.dst
	res := &Result{}

	if p.Target != nil {
		path, err := backupPost(backupDir, dst.name, dst.url, p.Target, p.TargetMeta, p.targetPLLInfo)
		if err != nil {
			return res, fmt.Errorf("backing up the %s post (nothing was changed): %w", dst.name, err)
		}
		res.BackupPath = path
		res.TargetID = int(p.Target.ID)
		res.Done = append(res.Done, fmt.Sprintf("backed up %s post to %s", dst.name, path))
	}

	for _, rel := range p.Copies() {
		if err := f.copyFile(rel); err != nil {
			return res, fmt.Errorf("%sing %s: %w", f.copyVerb(), rel, err)
		}
		res.Done = append(res.Done, fmt.Sprintf("%sed %s", f.copyVerb(), rel))
	}

	userArg := "--user=" + p.TargetUser
	for i, m := range p.Media {
		if m.TargetID != 0 || m.RelPath == "" {
			continue
		}
		id, err := importAttachment(dst.run, dst.wpRoot, p.TargetUser, m)
		if err != nil {
			return res, fmt.Errorf("adding %s to the %s media library: %w", m.RelPath, dst.name, err)
		}
		p.Media[i].TargetID = id
		if dst.pll != nil && dst.pll.Media && p.PLL != nil {
			if _, err := dst.run("eval", fmt.Sprintf(`pll_set_post_language(%d, "%s"); echo "ok";`, id, p.PLL.Lang), userArg); err != nil {
				return res, fmt.Errorf("setting the language of %s: %w", m.RelPath, err)
			}
		}
		res.Done = append(res.Done, fmt.Sprintf("added %s to the media library (ID %d)", m.RelPath, id))
	}

	content := p.FinalContent()
	if p.Target != nil {
		_, err := dst.stdin(content, "post", "update", itoa(res.TargetID), "-",
			"--post_title="+p.Source.PostTitle, "--post_excerpt="+p.finalExcerpt(), userArg)
		if err != nil {
			return res, fmt.Errorf("updating %s post %d: %w", dst.name, res.TargetID, err)
		}
		res.Done = append(res.Done, fmt.Sprintf("updated %s post %d", dst.name, res.TargetID))
	} else {
		// A push creates a draft on the live site to review. A pull keeps
		// the remote status and date, so the local copy shows up where it
		// does on the remote.
		status := "draft"
		if f.pull {
			status = p.Source.PostStatus
		}
		args := []string{"post", "create", "-",
			"--post_type=" + p.Source.PostType, "--post_name=" + p.Source.PostName,
			"--post_title=" + p.Source.PostTitle, "--post_excerpt=" + p.finalExcerpt(),
			"--post_status=" + status, "--porcelain", userArg}
		if d := p.Source.PostDate; f.pull && d != "" && !strings.HasPrefix(d, "0000") {
			args = append(args, "--post_date="+d)
		}
		if p.CreateParent > 0 {
			args = append(args, "--post_parent="+itoa(p.CreateParent))
		}
		if p.CreateAuthor > 0 {
			args = append(args, "--post_author="+itoa(p.CreateAuthor))
		}
		out, err := dst.stdin(content, args...)
		if err != nil {
			return res, fmt.Errorf("creating %s post: %w", dst.name, err)
		}
		id, err := parseID(out)
		if err != nil {
			return res, fmt.Errorf("creating %s post: %w", dst.name, err)
		}
		res.TargetID, res.Created = id, true
		if f.pull {
			res.Done = append(res.Done, fmt.Sprintf("created local post %d (%s)", id, status))
		} else {
			res.Done = append(res.Done, fmt.Sprintf("created remote draft %d", id))
		}
	}

	// Language first, so Polylang treats the following meta updates (and
	// the slug) as belonging to the right language.
	if p.PLL != nil && p.PLL.Group != nil {
		if _, err := dst.run("eval", pllSaveGroupPHP(res.TargetID, p.PLL.Lang, p.Source.PostName, p.PLL.Group, p.PLL.SetLang), userArg); err != nil {
			return res, fmt.Errorf("saving the Polylang language/translations: %w", err)
		}
		if p.PLL.SetLang {
			res.Done = append(res.Done, "set language "+p.PLL.Lang)
		}
		if len(p.PLL.NewLink) > 0 {
			res.Done = append(res.Done, "linked translations: "+strings.Join(p.PLL.NewLink, ", "))
		}
	}

	id := itoa(res.TargetID)
	if t := p.mappedThumbnail(); t != 0 && t != p.TargetThumbnail {
		if _, err := dst.run("post", "meta", "update", id, "_thumbnail_id", itoa(t), userArg); err != nil {
			return res, fmt.Errorf("setting featured image: %w", err)
		}
		res.Done = append(res.Done, "set featured image")
	}
	if tpl := normalizeTemplate(p.SourceTemplate); tpl != normalizeTemplate(p.TargetTemplate) && tpl != "" {
		if _, err := dst.run("post", "meta", "update", id, "_wp_page_template", tpl, userArg); err != nil {
			return res, fmt.Errorf("setting page template: %w", err)
		}
		res.Done = append(res.Done, "set page template "+tpl)
	}

	for _, m := range p.Meta {
		if _, err := dst.stdin(string(p.finalMeta(m)), "post", "meta", "update", id, m.Key, "--format=json", userArg); err != nil {
			return res, fmt.Errorf("writing meta %q: %w", m.Key, err)
		}
	}
	if len(p.Meta) > 0 {
		res.Done = append(res.Done, fmt.Sprintf("wrote %d meta values", len(p.Meta)))
	}

	// Read the content back: WordPress may filter it on save.
	if out, err := dst.run("post", "get", id, "--field=post_content"); err != nil ||
		strings.TrimSpace(out) != strings.TrimSpace(content) {
		res.VerifyMismatch = true
	}
	if out, err := dst.run("post", "get", id, "--field=post_name"); err == nil {
		res.FinalSlug = lastLine(out)
	}
	return res, nil
}

// backupPost saves a post about to be overwritten, with its meta and
// Polylang info, as JSON in dir. side ("remote" or "local") goes into the
// file name.
func backupPost(dir, side, site string, post *Post, meta []MetaEntry, pll *PLLInfo) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(map[string]any{
		"site":     site,
		"takenAt":  time.Now().UTC().Format(time.RFC3339),
		"post":     post,
		"postMeta": meta,
		"polylang": pll,
	}, "", "  ")
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s-%s-%s%d.json", time.Now().Format("20060102-150405"), post.PostName, side, post.ID)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
