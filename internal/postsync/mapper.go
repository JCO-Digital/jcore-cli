package postsync

import (
	"fmt"
	"strings"
)

// postMapper finds the remote counterpart of local posts and terms by type
// and slug, narrowed to the same Polylang language when both sites use it.
// Results are cached per local ID.
type postMapper struct {
	r         Runner
	localPLL  *PLLState
	remotePLL *PLLState
	posts     map[int]mapResult
	terms     map[string]mapResult // taxonomy|localID
}

type mapResult struct {
	id      int
	problem string // why there's no unique remote counterpart
}

func newPostMapper(r Runner, site *Site) *postMapper {
	return &postMapper{r: r, localPLL: site.LocalPLL, remotePLL: site.RemotePLL, posts: map[int]mapResult{}, terms: map[string]mapResult{}}
}

// langArgs makes post queries span every language (Polylang otherwise
// filters by the admin language preference under WP-CLI).
func langArgs(s *PLLState) []string {
	if s == nil {
		return nil
	}
	return []string{"--lang=all"}
}

// localLang returns the Polylang language of a local post or term ("" when
// not applicable).
func (m *postMapper) localLang(kind string, id int) string {
	if m.localPLL == nil || m.remotePLL == nil {
		return ""
	}
	info, err := pllInfo(m.r.Local, kind, []int{id})
	if err != nil {
		return ""
	}
	return info[id].Lang
}

// post maps a local post ID to the remote one.
func (m *postMapper) post(localID int) (mapResult, error) {
	if res, ok := m.posts[localID]; ok {
		return res, nil
	}
	res, err := m.lookupPost(localID)
	if err == nil {
		m.posts[localID] = res
	}
	return res, err
}

func (m *postMapper) lookupPost(localID int) (mapResult, error) {
	lp, err := getPost(m.r.Local, localID)
	if err != nil || lp.PostName == "" {
		return mapResult{problem: fmt.Sprintf("local post %d can't be read or has no slug", localID)}, nil
	}
	refs, err := listPosts(m.r.Remote, lp.PostType, lp.PostName, "any", langArgs(m.remotePLL)...)
	if err != nil {
		return mapResult{}, fmt.Errorf("looking up %s %q on the remote: %w", lp.PostType, lp.PostName, err)
	}
	ids := make([]int, len(refs))
	for i, ref := range refs {
		ids[i] = int(ref.ID)
	}
	label := fmt.Sprintf("%s %q", lp.PostType, lp.PostName)
	if lang := m.localLang("post", localID); lang != "" {
		label += " [" + lang + "]"
		if ids, err = filterByLang(m.r.Remote, "post", ids, lang); err != nil {
			return mapResult{}, err
		}
	}
	switch len(ids) {
	case 1:
		return mapResult{id: ids[0]}, nil
	case 0:
		return mapResult{problem: fmt.Sprintf("%s is missing on the remote; run: jcore sync %d", label, localID)}, nil
	}
	return mapResult{problem: fmt.Sprintf("%s: several remote posts match", label)}, nil
}

// term maps a local term ID to the remote one.
func (m *postMapper) term(taxonomy string, localID int) (mapResult, error) {
	key := taxonomy + "|" + itoa(localID)
	if res, ok := m.terms[key]; ok {
		return res, nil
	}
	out, err := m.r.Local("term", "get", taxonomy, itoa(localID), "--field=slug")
	if err != nil {
		res := mapResult{problem: fmt.Sprintf("local %s term %d can't be read", taxonomy, localID)}
		m.terms[key] = res
		return res, nil
	}
	slug := lastLine(out)
	label := fmt.Sprintf("%s term %q", taxonomy, slug)
	out, err = m.r.Remote("term", "list", taxonomy, "--slug="+slug, "--field=term_id")
	var ids []int
	if err == nil {
		ids = parseIDs(out)
	}
	if lang := m.localLang("term", localID); lang != "" && len(ids) > 0 {
		label += " [" + lang + "]"
		if ids, err = filterByLang(m.r.Remote, "term", ids, lang); err != nil {
			return mapResult{}, err
		}
	}
	var res mapResult
	switch len(ids) {
	case 1:
		res = mapResult{id: ids[0]}
	case 0:
		res = mapResult{problem: label + " is missing on the remote; create it there first"}
	default:
		res = mapResult{problem: label + ": several remote terms match"}
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
