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
	Meta         bool   // also copy custom post meta
	As           string // remote user login to write as; "" picks an administrator
	LocalDomain  string
	RemoteDomain string
	RemotePath   string
	// LastPull is when the local database was last pulled from the remote
	// (zero if unknown). A remote post modified after it may contain
	// changes the local copy doesn't have.
	LastPull time.Time
}

// MetaItem is a custom meta value --meta will write to the remote.
type MetaItem struct {
	Key   string
	Value json.RawMessage // JSON, domain-rewritten
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

// Prepare inspects both sites and returns the sync plan. It never writes.
func Prepare(r Runner, opts Options) (*Plan, error) {
	if _, err := r.Local("core", "is-installed"); err != nil {
		return nil, fmt.Errorf("local WordPress isn't reachable (is the project running? try `jcore start`): %w", err)
	}
	if _, err := r.Local("core", "is-installed", "--network"); err == nil {
		return nil, errors.New("the local site is a multisite network, which sync doesn't support yet")
	}

	p := &Plan{}

	out, err := r.Remote("option", "get", "siteurl")
	if err != nil {
		return nil, fmt.Errorf("couldn't run wp-cli on the remote over SSH: %w", err)
	}
	p.RemoteSiteURL = lastLine(out)
	if _, err := r.Remote("core", "is-installed", "--network"); err == nil {
		return nil, errors.New("the remote site is a multisite network, which sync doesn't support yet")
	}
	if u, err := url.Parse(p.RemoteSiteURL); err == nil && opts.RemoteDomain != "" &&
		!strings.EqualFold(strings.TrimPrefix(u.Host, "www."), strings.TrimPrefix(opts.RemoteDomain, "www.")) {
		p.Warnings = append(p.Warnings, fmt.Sprintf("remote siteurl is %s but remoteDomain is %q; URLs will be rewritten to %q", p.RemoteSiteURL, opts.RemoteDomain, opts.RemoteDomain))
	}

	// Local post.
	local, err := resolveLocalPost(r, opts.Ident, opts.PostType)
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

	// Remote counterpart, matched by type + slug.
	if _, err := r.Remote("post-type", "get", local.PostType, "--field=name"); err != nil {
		return nil, fmt.Errorf("post type %q doesn't exist on the remote", local.PostType)
	}
	matches, err := listPosts(r.Remote, local.PostType, local.PostName, "any")
	if err != nil {
		return nil, fmt.Errorf("looking up the remote post: %w", err)
	}
	switch len(matches) {
	case 0:
		trashed, _ := listPosts(r.Remote, local.PostType, local.PostName, "trash")
		if len(trashed) > 0 {
			p.Warnings = append(p.Warnings, fmt.Sprintf("a trashed remote %s with slug %q exists (ID %d); it's left alone and a new post is created", local.PostType, local.PostName, trashed[0].ID))
		}
	case 1:
		remote, err := getPost(r.Remote, int(matches[0].ID))
		if err != nil {
			return nil, fmt.Errorf("reading remote post %d: %w", matches[0].ID, err)
		}
		p.Remote = remote
		if p.RemoteMeta, err = getMeta(r.Remote, int(remote.ID)); err != nil {
			return nil, fmt.Errorf("reading remote post meta: %w", err)
		}
		p.RemoteThumbnail = metaInt(p.RemoteMeta, "_thumbnail_id")
		p.RemoteTemplate = metaString(p.RemoteMeta, "_wp_page_template")
	default:
		ids := []string{}
		for _, m := range matches {
			ids = append(ids, itoa(int(m.ID)))
		}
		return nil, fmt.Errorf("%d remote %s posts have slug %q (IDs %s); refusing to guess which one to overwrite", len(matches), local.PostType, local.PostName, strings.Join(ids, ", "))
	}

	if p.Remote != nil {
		p.checkDrift(opts.LastPull)
	}

	// Remote user to write as. Running without one would let kses strip
	// markup from the content.
	if err := p.resolveRemoteUser(r, opts.As); err != nil {
		return nil, err
	}

	if p.Creating() {
		p.resolveCreateRelations(r)
	}

	// Media.
	var extra []int
	if p.LocalThumbnail > 0 {
		extra = append(extra, p.LocalThumbnail)
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

func (p *Plan) resolveRemoteUser(r Runner, as string) error {
	if as != "" {
		if _, err := r.Remote("user", "get", as, "--field=ID"); err != nil {
			return fmt.Errorf("remote user %q not found", as)
		}
		p.RemoteUser = as
		return nil
	}
	out, err := r.Remote("user", "list", "--role=administrator", "--field=user_login", "--number=1", "--orderby=ID", "--order=ASC")
	if err != nil || strings.TrimSpace(out) == "" {
		return errors.New("couldn't find an administrator on the remote to write as; pass --as <login>")
	}
	p.RemoteUser = lastLine(out)
	return nil
}

// resolveCreateRelations maps the local parent and author to the remote by
// slug and login. Failures only produce warnings.
func (p *Plan) resolveCreateRelations(r Runner) {
	if parent := int(p.Local.PostParent); parent > 0 {
		lp, err := getPost(r.Local, parent)
		if err == nil && lp.PostName != "" {
			if refs, err := listPosts(r.Remote, lp.PostType, lp.PostName, "any"); err == nil && len(refs) == 1 {
				p.CreateParent = int(refs[0].ID)
			}
		}
		if p.CreateParent == 0 {
			p.Warnings = append(p.Warnings, fmt.Sprintf("local parent (ID %d) has no match on the remote; the post is created without a parent", parent))
		}
	}
	if author := int(p.Local.PostAuthor); author > 0 {
		if login, err := r.Local("user", "get", itoa(author), "--field=user_login"); err == nil {
			if out, err := r.Remote("user", "get", lastLine(login), "--field=ID"); err == nil {
				p.CreateAuthor, _ = parseID(out)
			}
		}
	}
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
		value := json.RawMessage(ReplaceDomainJSON(string(m.Value), opts.LocalDomain, opts.RemoteDomain))
		if rv, ok := remote[m.Key]; ok && jsonEqual(rv, value) {
			continue
		}
		p.Meta = append(p.Meta, MetaItem{Key: m.Key, Value: value})
		if !strings.HasPrefix(m.Key, "_") && looksLikeIDs(m.Value) {
			numeric = append(numeric, m.Key)
		}
	}
	if len(numeric) > 0 {
		p.Warnings = append(p.Warnings, "these meta values look like post/attachment IDs, which are copied as-is and NOT remapped (ACF image/relationship fields will point at the wrong items): "+strings.Join(numeric, ", "))
	}
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

// FinalContent is the local content with attachment IDs remapped (as far as
// known) and local URLs pointed at the remote.
func (p *Plan) FinalContent(opts Options) string {
	return ReplaceDomain(RemapAttachmentIDs(p.Local.PostContent, p.idMap()), opts.LocalDomain, opts.RemoteDomain)
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
		if _, err := r.RemoteStdin(string(m.Value), "post", "meta", "update", id, m.Key, "--format=json", userArg); err != nil {
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
