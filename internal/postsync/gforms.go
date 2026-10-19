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

// FormMapping is a source form and its target counterpart.
type FormMapping struct {
	SourceID int
	Title    string
	TargetID int
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

const gfFormsPHP = `if (!class_exists("GFAPI")) { echo "null"; return; }
$o = array(); foreach (GFAPI::get_forms(null, false) as $f) { $o[] = array("id" => (int) $f["id"], "title" => $f["title"], "active" => (bool) $f["is_active"]); }
echo json_encode($o);`

// planForms maps every embedded form to the target by title. A form that's
// missing (or ambiguous) on the target aborts the sync.
func planForms(p *Plan, content string) error {
	src, dst := p.f.src, p.f.dst
	ids := CollectFormIDs(content)
	if len(ids) == 0 {
		return nil
	}
	php := fmt.Sprintf(`if (!class_exists("GFAPI")) { echo "null"; return; } $o = array(); foreach (array(%s) as $i) { $f = GFAPI::get_form($i); $o[$i] = $f ? $f["title"] : ""; } echo json_encode((object) $o);`, intList(ids))
	out, err := src.run("eval", php)
	if err != nil {
		return fmt.Errorf("reading %s Gravity Forms: %w", src.name, err)
	}
	if lastLine(out) == "null" {
		p.Warnings = append(p.Warnings, fmt.Sprintf("the content embeds Gravity Forms but the plugin isn't active %s; form IDs are NOT remapped", src.where))
		return nil
	}
	var titles map[string]string
	if err := parseJSON(out, &titles); err != nil {
		return err
	}

	out, err = dst.run("eval", gfFormsPHP)
	if err != nil {
		return fmt.Errorf("reading %s Gravity Forms: %w", dst.name, err)
	}
	if lastLine(out) == "null" {
		return fmt.Errorf("the post embeds Gravity Forms, but Gravity Forms isn't active %s", dst.where)
	}
	var target []struct {
		ID     int    `json:"id"`
		Title  string `json:"title"`
		Active bool   `json:"active"`
	}
	if err := parseJSON(out, &target); err != nil {
		return err
	}

	var problems []string
	for _, id := range ids {
		title := strings.TrimSpace(titles[itoa(id)])
		if title == "" {
			problems = append(problems, fmt.Sprintf("form %d doesn't exist %s", id, src.where))
			continue
		}
		var matches []int
		inactive := false
		for _, f := range target {
			if strings.TrimSpace(f.Title) == title {
				matches = append(matches, f.ID)
				inactive = !f.Active
			}
		}
		switch len(matches) {
		case 1:
			p.Forms = append(p.Forms, FormMapping{SourceID: id, Title: title, TargetID: matches[0]})
			if inactive {
				p.Warnings = append(p.Warnings, fmt.Sprintf("%s form %q (ID %d) is inactive", dst.name, title, matches[0]))
			}
		case 0:
			problems = append(problems, fmt.Sprintf("form %q is missing %s; import it there first (Forms → Import/Export)", title, dst.where))
		default:
			problems = append(problems, fmt.Sprintf("form %q: several %s forms have this title", title, dst.name))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("the post embeds Gravity Forms %s doesn't have:\n  - %s", dst.site, strings.Join(problems, "\n  - "))
	}
	return nil
}

func (p *Plan) formMap() map[int]int {
	m := map[int]int{}
	for _, f := range p.Forms {
		m[f.SourceID] = f.TargetID
	}
	return m
}
