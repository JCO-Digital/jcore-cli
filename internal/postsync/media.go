package postsync

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// MediaItem is a source attachment the post references.
type MediaItem struct {
	SourceID int
	RelPath  string // _wp_attached_file, relative to wp-content/uploads
	Title    string
	Caption  string
	Desc     string
	Alt      string
	TargetID int // 0 until it exists on the target
}

// FileItem is an uploads file the post needs on the target.
type FileItem struct {
	RelPath string
	State   FileState
}

// readAttachment reads the details of attachment id through run (where
// says which site, e.g. "locally"), or returns nil (with a reason) if id
// isn't a usable attachment. The ID is returned as SourceID.
func readAttachment(run func(...string) (string, error), id int, where string) (*MediaItem, string) {
	p, err := getPost(run, id)
	if err != nil {
		return nil, fmt.Sprintf("attachment %d not found %s", id, where)
	}
	if p.PostType != "attachment" {
		return nil, fmt.Sprintf("ID %d is a %s, not an attachment", id, p.PostType)
	}
	meta, err := getMeta(run, id)
	if err != nil {
		return nil, fmt.Sprintf("could not read meta of attachment %d: %v", id, err)
	}
	rel := metaString(meta, "_wp_attached_file")
	if rel == "" || strings.HasPrefix(rel, "/") || strings.Contains(rel, "..") {
		return nil, fmt.Sprintf("attachment %d has no usable file path (%q)", id, rel)
	}
	return &MediaItem{
		SourceID: id,
		RelPath:  rel,
		Title:    p.PostTitle,
		Caption:  p.PostExcerpt,
		Desc:     p.PostContent,
		Alt:      metaString(meta, "_wp_attachment_image_alt"),
	}, ""
}

// findAttachment returns the ID of the attachment stored at rel on the site
// run talks to, or 0 if there is none. With Polylang media translation (pll
// is that site's state), one file can back an attachment per language; the
// one in lang is preferred.
func findAttachment(run func(...string) (string, error), rel string, pll *PLLState, lang string) (int, error) {
	args := []string{"post", "list", "--post_type=attachment", "--post_status=any", "--meta_key=_wp_attached_file", "--meta_value=" + rel, "--fields=ID", "--format=json"}
	out, err := run(append(args, langArgs(pll)...)...)
	if err != nil {
		return 0, err
	}
	var refs []postRef
	if strings.TrimSpace(out) != "" {
		if err := parseJSON(out, &refs); err != nil {
			return 0, err
		}
	}
	if len(refs) == 0 {
		return 0, nil
	}
	if len(refs) > 1 && pll != nil && pll.Media && lang != "" {
		ids := make([]int, len(refs))
		for i, ref := range refs {
			ids[i] = int(ref.ID)
		}
		if same, err := filterByLang(run, "post", ids, lang); err == nil && len(same) > 0 {
			return same[0], nil
		}
	}
	return int(refs[0].ID), nil
}

// planMedia resolves every attachment and uploads file the post needs.
// It only reads; nothing is changed on either side.
func planMedia(p *Plan, content string, extraIDs []int) error {
	f := p.f
	src, dst := f.src, f.dst
	ids := CollectAttachmentIDs(content)
	for _, id := range extraIDs {
		if id > 0 && !containsInt(ids, id) {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)

	lang := ""
	if p.PLL != nil {
		lang = p.PLL.Lang
	}
	for _, id := range ids {
		item, reason := readAttachment(src.run, id, src.where)
		if item == nil {
			p.Warnings = append(p.Warnings, reason+"; its references are left unchanged")
			continue
		}
		targetID, err := findAttachment(dst.run, item.RelPath, dst.pll, lang)
		if err != nil {
			return fmt.Errorf("looking up %s %s: %w", item.RelPath, dst.where, err)
		}
		item.TargetID = targetID
		p.Media = append(p.Media, *item)
	}

	// Files: originals of attachments that must be imported, plus every
	// uploads URL in the content (resized variants included).
	need := []string{}
	for _, m := range p.Media {
		if m.TargetID == 0 {
			need = appendUnique(need, m.RelPath)
		}
	}
	for _, rel := range CollectSiteRefs(content, src.domain).Uploads {
		need = appendUnique(need, rel)
	}

	for _, rel := range need {
		state, err := f.fileStatus(rel)
		if err != nil {
			return fmt.Errorf("checking %s %s: %w", rel, dst.where, err)
		}
		switch state {
		case FileMissingSource:
			p.Warnings = append(p.Warnings, fmt.Sprintf("uploads/%s is referenced but doesn't exist %s; skipped", rel, src.where))
			continue
		case FileDiffers:
			if p.belongsToTargetAttachment(rel) {
				// The target already has this attachment; its own
				// version of the file (or resized variant) is kept.
				state = FileIdentical
				break
			}
			if f.pull {
				return fmt.Errorf("you already have a DIFFERENT file at wp-content/uploads/%s locally; refusing to overwrite it. Move it aside (or run `jcore pull media`) first", rel)
			}
			return fmt.Errorf("remote already has a DIFFERENT file at wp-content/uploads/%s; refusing to overwrite it. Rename the file locally (re-upload it) or resolve it on the remote first", rel)
		}
		p.Files = append(p.Files, FileItem{RelPath: rel, State: state})
	}

	// An attachment to import whose original can't be copied can't be
	// imported either.
	for i, m := range p.Media {
		if m.TargetID != 0 {
			continue
		}
		if !p.hasFile(m.RelPath) {
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s attachment %d (%s) can't be imported: file unavailable", src.name, m.SourceID, m.RelPath))
			p.Media[i].RelPath = ""
		}
	}
	return nil
}

func (p *Plan) belongsToTargetAttachment(rel string) bool {
	base := baseUpload(rel)
	for _, m := range p.Media {
		if m.TargetID != 0 && baseUpload(m.RelPath) == base {
			return true
		}
	}
	return false
}

func (p *Plan) hasFile(rel string) bool {
	for _, f := range p.Files {
		if f.RelPath == rel {
			return true
		}
	}
	return false
}

// idMap returns the source→target attachment ID mapping known so far.
func (p *Plan) idMap() map[int]int {
	m := map[int]int{}
	for _, item := range p.Media {
		if item.TargetID != 0 {
			m[item.SourceID] = item.TargetID
		}
	}
	return m
}

// importAttachment registers an already-copied file in the media library of
// the site run talks to (WordPress root wpRoot) at its existing path (no
// copy, so the URL stays the same) and returns the new attachment ID.
func importAttachment(run func(...string) (string, error), wpRoot, user string, m MediaItem) (int, error) {
	args := []string{"media", "import", strings.TrimRight(wpRoot, "/") + "/wp-content/uploads/" + m.RelPath, "--skip-copy", "--porcelain", "--user=" + user}
	if m.Title != "" {
		args = append(args, "--title="+m.Title)
	}
	if m.Caption != "" {
		args = append(args, "--caption="+m.Caption)
	}
	if m.Alt != "" {
		args = append(args, "--alt="+m.Alt)
	}
	if m.Desc != "" {
		args = append(args, "--desc="+m.Desc)
	}
	out, err := run(args...)
	if err != nil {
		return 0, err
	}
	return parseID(out)
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

func itoa(n int) string { return strconv.Itoa(n) }
