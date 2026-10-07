package postsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// flexInt decodes a JSON number or numeric string (wp-cli returns e.g.
// post_author as "1" but ID as 1).
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	*f = flexInt(n)
	return nil
}

// Post is the subset of WP_Post fields the sync uses.
type Post struct {
	ID              flexInt `json:"ID"`
	PostType        string  `json:"post_type"`
	PostName        string  `json:"post_name"`
	PostTitle       string  `json:"post_title"`
	PostStatus      string  `json:"post_status"`
	PostExcerpt     string  `json:"post_excerpt"`
	PostContent     string  `json:"post_content"`
	PostParent      flexInt `json:"post_parent"`
	PostAuthor      flexInt `json:"post_author"`
	PostModifiedGMT string  `json:"post_modified_gmt"`
}

// MetaEntry is one row of `wp post meta list --format=json`.
type MetaEntry struct {
	Key   string          `json:"meta_key"`
	Value json.RawMessage `json:"meta_value"`
}

// unsyncablePostTypes can never be synced as a post.
var unsyncablePostTypes = []string{"attachment", "revision", "nav_menu_item", "customize_changeset", "oembed_cache", "user_request"}

// internalMetaKeys are never copied by --meta: they're WordPress
// bookkeeping, or handled explicitly (_thumbnail_id, _wp_page_template).
var internalMetaKeys = []string{"_edit_lock", "_edit_last", "_wp_old_slug", "_wp_old_date", "_thumbnail_id", "_wp_page_template", "_encloseme", "_pingme", "_wp_trash_meta_status", "_wp_trash_meta_time", "_wp_desired_post_slug"}

// parseJSON decodes the last line of out that parses as JSON into v.
// wp-cli prints its JSON on one line, but PHP notices or plugin output can
// precede it on stdout.
func parseJSON(out string, v any) error {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(l, "{") && !strings.HasPrefix(l, "[") {
			continue
		}
		if err := json.Unmarshal([]byte(l), v); err == nil {
			return nil
		}
	}
	return fmt.Errorf("could not parse wp-cli JSON output: %q", truncate(out, 200))
}

// parseID returns the last all-digit line of out (wp-cli --porcelain / --field=ID).
func parseID(out string) (int, error) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if n, err := strconv.Atoi(strings.TrimSpace(lines[i])); err == nil {
			return n, nil
		}
	}
	return 0, fmt.Errorf("expected an ID from wp-cli, got %q", truncate(out, 200))
}

// lastLine returns the last non-empty line of out.
func lastLine(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func getPost(run func(...string) (string, error), id int) (*Post, error) {
	out, err := run("post", "get", strconv.Itoa(id), "--format=json")
	if err != nil {
		return nil, err
	}
	var p Post
	if err := parseJSON(out, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func getMeta(run func(...string) (string, error), id int) ([]MetaEntry, error) {
	// --unserialize returns arrays as JSON rather than PHP-serialized
	// strings, so they round-trip through `meta update --format=json`
	// without being serialized twice.
	out, err := run("post", "meta", "list", strconv.Itoa(id), "--unserialize", "--format=json")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	var meta []MetaEntry
	if err := parseJSON(out, &meta); err != nil {
		return nil, err
	}
	return meta, nil
}

// metaString returns the string value of the first meta entry with key.
func metaString(meta []MetaEntry, key string) string {
	for _, m := range meta {
		if m.Key != key {
			continue
		}
		var s string
		if json.Unmarshal(m.Value, &s) == nil {
			return s
		}
		var n json.Number
		if json.Unmarshal(m.Value, &n) == nil {
			return n.String()
		}
	}
	return ""
}

func metaInt(meta []MetaEntry, key string) int {
	n, _ := strconv.Atoi(metaString(meta, key))
	return n
}

type postRef struct {
	ID       flexInt `json:"ID"`
	PostType string  `json:"post_type"`
	Title    string  `json:"post_title"`
	Status   string  `json:"post_status"`
}

func listPosts(run func(...string) (string, error), postType, slug, status string, extra ...string) ([]postRef, error) {
	args := append([]string{"post", "list", "--post_type=" + postType, "--name=" + slug, "--post_status=" + status, "--fields=ID,post_type,post_title,post_status", "--format=json"}, extra...)
	out, err := run(args...)
	if err != nil {
		return nil, err
	}
	var refs []postRef
	if strings.TrimSpace(out) == "" {
		return refs, nil
	}
	if err := parseJSON(out, &refs); err != nil {
		return nil, err
	}
	return refs, nil
}

// resolveLocalPost finds exactly one local post by numeric ID or slug. With
// Polylang, lang (if set) narrows a slug shared by several languages.
func resolveLocalPost(r Runner, ident, postType, lang string, pll *PLLState) (*Post, error) {
	if lang != "" && pll == nil {
		return nil, errors.New("--lang needs Polylang, which isn't active on the local site")
	}
	if id, err := strconv.Atoi(ident); err == nil {
		p, err := getPost(r.Local, id)
		if err != nil {
			return nil, fmt.Errorf("local post %d not found: %w", id, err)
		}
		if lang != "" {
			if info, err := pllInfo(r.Local, "post", []int{id}); err == nil && info[id].Lang != lang {
				return nil, fmt.Errorf("local post %d is in language %q, not %q", id, info[id].Lang, lang)
			}
		}
		return p, nil
	}
	if postType == "" {
		postType = "any"
	}
	refs, err := listPosts(r.Local, postType, ident, "any", langArgs(pll)...)
	if err != nil {
		return nil, fmt.Errorf("looking up local post %q: %w", ident, err)
	}
	if lang != "" && len(refs) > 0 {
		ids := make([]int, len(refs))
		for i, ref := range refs {
			ids[i] = int(ref.ID)
		}
		keep, err := filterByLang(r.Local, "post", ids, lang)
		if err != nil {
			return nil, err
		}
		var filtered []postRef
		for _, ref := range refs {
			if containsInt(keep, int(ref.ID)) {
				filtered = append(filtered, ref)
			}
		}
		refs = filtered
	}
	switch len(refs) {
	case 0:
		if lang != "" {
			return nil, fmt.Errorf("no local post with slug %q in language %q (type %s)", ident, lang, postType)
		}
		return nil, fmt.Errorf("no local post with slug %q (type %s)", ident, postType)
	case 1:
		return getPost(r.Local, int(refs[0].ID))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "slug %q matches %d local posts; pass the ID, --type or --lang:\n", ident, len(refs))
	var langs map[int]PLLInfo
	if pll != nil {
		ids := make([]int, len(refs))
		for i, ref := range refs {
			ids[i] = int(ref.ID)
		}
		langs, _ = pllInfo(r.Local, "post", ids)
	}
	for _, p := range refs {
		l := ""
		if langs != nil && langs[int(p.ID)].Lang != "" {
			l = " [" + langs[int(p.ID)].Lang + "]"
		}
		fmt.Fprintf(&b, "  %d  %s  %s (%s)%s\n", p.ID, p.PostType, p.Title, p.Status, l)
	}
	return nil, errors.New(strings.TrimRight(b.String(), "\n"))
}

func parseGMT(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil || t.Year() < 1971 {
		return time.Time{}, false
	}
	return t, true
}

// canSyncType reports whether postType may be synced.
func canSyncType(postType string) bool {
	return !slices.Contains(unsyncablePostTypes, postType)
}
