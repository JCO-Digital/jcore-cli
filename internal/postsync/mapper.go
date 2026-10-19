package postsync

import (
	"fmt"
	"strings"
)

// postMapper finds the target counterpart of source posts and terms by
// type and slug, narrowed to the same Polylang language when both sites use
// it. Results are cached per source ID.
type postMapper struct {
	f     *flow
	posts map[int]mapResult
	terms map[string]mapResult // taxonomy|sourceID
}

type mapResult struct {
	id      int
	problem string // why there's no unique target counterpart
}

func newPostMapper(f *flow) *postMapper {
	return &postMapper{f: f, posts: map[int]mapResult{}, terms: map[string]mapResult{}}
}

// langArgs makes post queries span every language (Polylang otherwise
// filters by the admin language preference under WP-CLI).
func langArgs(s *PLLState) []string {
	if s == nil {
		return nil
	}
	return []string{"--lang=all"}
}

// sourceLang returns the Polylang language of a source post or term (""
// when not applicable).
func (m *postMapper) sourceLang(kind string, id int) string {
	if m.f.src.pll == nil || m.f.dst.pll == nil {
		return ""
	}
	info, err := pllInfo(m.f.src.run, kind, []int{id})
	if err != nil {
		return ""
	}
	return info[id].Lang
}

// post maps a source post ID to the target one.
func (m *postMapper) post(sourceID int) (mapResult, error) {
	if res, ok := m.posts[sourceID]; ok {
		return res, nil
	}
	res, err := m.lookupPost(sourceID)
	if err == nil {
		m.posts[sourceID] = res
	}
	return res, err
}

func (m *postMapper) lookupPost(sourceID int) (mapResult, error) {
	src, dst := m.f.src, m.f.dst
	sp, err := getPost(src.run, sourceID)
	if err != nil || sp.PostName == "" {
		return mapResult{problem: fmt.Sprintf("%s post %d can't be read or has no slug", src.name, sourceID)}, nil
	}
	refs, err := listPosts(dst.run, sp.PostType, sp.PostName, "any", langArgs(dst.pll)...)
	if err != nil {
		return mapResult{}, fmt.Errorf("looking up %s %q %s: %w", sp.PostType, sp.PostName, dst.where, err)
	}
	ids := make([]int, len(refs))
	for i, ref := range refs {
		ids[i] = int(ref.ID)
	}
	label := fmt.Sprintf("%s %q", sp.PostType, sp.PostName)
	if lang := m.sourceLang("post", sourceID); lang != "" {
		label += " [" + lang + "]"
		if ids, err = filterByLang(dst.run, "post", ids, lang); err != nil {
			return mapResult{}, err
		}
	}
	switch len(ids) {
	case 1:
		return mapResult{id: ids[0]}, nil
	case 0:
		return mapResult{problem: fmt.Sprintf("%s is missing %s; run: %s", label, dst.where, m.f.syncHint(sourceID))}, nil
	}
	return mapResult{problem: fmt.Sprintf("%s: several %s posts match", label, dst.name)}, nil
}

// term maps a source term ID to the target one.
func (m *postMapper) term(taxonomy string, sourceID int) (mapResult, error) {
	src, dst := m.f.src, m.f.dst
	key := taxonomy + "|" + itoa(sourceID)
	if res, ok := m.terms[key]; ok {
		return res, nil
	}
	out, err := src.run("term", "get", taxonomy, itoa(sourceID), "--field=slug")
	if err != nil {
		res := mapResult{problem: fmt.Sprintf("%s %s term %d can't be read", src.name, taxonomy, sourceID)}
		m.terms[key] = res
		return res, nil
	}
	slug := lastLine(out)
	label := fmt.Sprintf("%s term %q", taxonomy, slug)
	out, err = dst.run("term", "list", taxonomy, "--slug="+slug, "--field=term_id")
	var ids []int
	if err == nil {
		ids = parseIDs(out)
	}
	if lang := m.sourceLang("term", sourceID); lang != "" && len(ids) > 0 {
		label += " [" + lang + "]"
		if ids, err = filterByLang(dst.run, "term", ids, lang); err != nil {
			return mapResult{}, err
		}
	}
	var res mapResult
	switch len(ids) {
	case 1:
		res = mapResult{id: ids[0]}
	case 0:
		res = mapResult{problem: fmt.Sprintf("%s is missing %s; create it there first", label, dst.where)}
	default:
		res = mapResult{problem: fmt.Sprintf("%s: several %s terms match", label, dst.name)}
	}
	m.terms[key] = res
	return res, nil
}

// parseIDs returns every all-digit line of out.
func parseIDs(out string) []int {
	var ids []int
	for _, l := range strings.Split(out, "\n") {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(l), "%d", &n); err == nil && itoa(n) == strings.TrimSpace(l) && n > 0 {
			ids = append(ids, n)
		}
	}
	return ids
}
