package postsync

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Gravity Forms embeds forms by ID: the gravityforms/form block's "formId"
// attribute and the [gravityform id="N"] shortcode. Form IDs differ between
// sites, so forms are matched by exact title.

var (
	gfAttrRe      = regexp.MustCompile(`("formId"\s*:\s*"?)(\d+)`)
	gfShortcodeRe = regexp.MustCompile(`(\[gravityforms?\b[^\]]*?\bid\s*=\s*["']?)(\d+)`)
)

// FormMapping is a local form and its remote counterpart.
type FormMapping struct {
	LocalID  int
	Title    string
	RemoteID int
}

// CollectFormIDs returns the Gravity Forms IDs embedded in content.
func CollectFormIDs(content string) []int {
	var ids []int
	add := func(s string) {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && !containsInt(ids, n) {
			ids = append(ids, n)
		}
	}
	for _, m := range blockRe.FindAllStringSubmatch(content, -1) {
		if normalizeBlockName(m[1]) == "gravityforms/form" {
			if sm := gfAttrRe.FindStringSubmatch(m[2]); sm != nil {
				add(sm[2])
			}
		}
	}
	for _, m := range gfShortcodeRe.FindAllStringSubmatch(content, -1) {
		add(m[2])
	}
	sort.Ints(ids)
	return ids
}

// RemapFormIDs rewrites embedded form IDs in content.
func RemapFormIDs(content string, m map[int]int) string {
	if len(m) == 0 {
		return content
	}
	mapID := func(re *regexp.Regexp) func(string) string {
		return func(s string) string {
			sm := re.FindStringSubmatch(s)
			n, _ := strconv.Atoi(sm[2])
			if to, ok := m[n]; ok {
				return sm[1] + itoa(to)
			}
			return s
		}
	}
	content = blockRe.ReplaceAllStringFunc(content, func(block string) string {
		bm := blockRe.FindStringSubmatch(block)
		if normalizeBlockName(bm[1]) != "gravityforms/form" {
			return block
		}
		attrs := gfAttrRe.ReplaceAllStringFunc(bm[2], mapID(gfAttrRe))
		return strings.Replace(block, bm[2], attrs, 1)
	})
	return gfShortcodeRe.ReplaceAllStringFunc(content, mapID(gfShortcodeRe))
}

const gfRemoteFormsPHP = `if (!class_exists("GFAPI")) { echo "null"; return; }
$o = array(); foreach (GFAPI::get_forms(null, false) as $f) { $o[] = array("id" => (int) $f["id"], "title" => $f["title"], "active" => (bool) $f["is_active"]); }
echo json_encode($o);`

// planForms maps every embedded form to the remote by title. A form that's
// missing (or ambiguous) on the remote aborts the sync.
func planForms(r Runner, p *Plan, content string) error {
	ids := CollectFormIDs(content)
	if len(ids) == 0 {
		return nil
	}
	php := fmt.Sprintf(`if (!class_exists("GFAPI")) { echo "null"; return; } $o = array(); foreach (array(%s) as $i) { $f = GFAPI::get_form($i); $o[$i] = $f ? $f["title"] : ""; } echo json_encode((object) $o);`, intList(ids))
	out, err := r.Local("eval", php)
	if err != nil {
		return fmt.Errorf("reading local Gravity Forms: %w", err)
	}
	if lastLine(out) == "null" {
		p.Warnings = append(p.Warnings, "the content embeds Gravity Forms but the plugin isn't active locally; form IDs are NOT remapped")
		return nil
	}
	var titles map[string]string
	if err := parseJSON(out, &titles); err != nil {
		return err
	}

	out, err = r.Remote("eval", gfRemoteFormsPHP)
	if err != nil {
		return fmt.Errorf("reading remote Gravity Forms: %w", err)
	}
	if lastLine(out) == "null" {
		return fmt.Errorf("the post embeds Gravity Forms, but Gravity Forms isn't active on the remote")
	}
	var remote []struct {
		ID     int    `json:"id"`
		Title  string `json:"title"`
		Active bool   `json:"active"`
	}
	if err := parseJSON(out, &remote); err != nil {
		return err
	}

	var problems []string
	for _, id := range ids {
		title := strings.TrimSpace(titles[itoa(id)])
		if title == "" {
			problems = append(problems, fmt.Sprintf("form %d doesn't exist locally", id))
			continue
		}
		var matches []int
		inactive := false
		for _, f := range remote {
			if strings.TrimSpace(f.Title) == title {
				matches = append(matches, f.ID)
				inactive = !f.Active
			}
		}
		switch len(matches) {
		case 1:
			p.Forms = append(p.Forms, FormMapping{LocalID: id, Title: title, RemoteID: matches[0]})
			if inactive {
				p.Warnings = append(p.Warnings, fmt.Sprintf("remote form %q (ID %d) is inactive", title, matches[0]))
			}
		case 0:
			problems = append(problems, fmt.Sprintf("form %q is missing on the remote; import it there first (Forms → Import/Export)", title))
		default:
			problems = append(problems, fmt.Sprintf("form %q: several remote forms have this title", title))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("the post embeds Gravity Forms the remote doesn't have:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

func (p *Plan) formMap() map[int]int {
	m := map[int]int{}
	for _, f := range p.Forms {
		m[f.LocalID] = f.RemoteID
	}
	return m
}
