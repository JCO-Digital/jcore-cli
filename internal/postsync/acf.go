package postsync

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ACF stores field values by name, each paired with a "_<name>" entry
// holding the field key ("field_abc123"): in block attributes under "data",
// and in post meta. Repeater/group subfields are flattened the same way
// ("slides_0_image" / "_slides_0_image"). Values may also be keyed by the
// field key directly. The field key tells us the field type, and therefore
// whether a value is an attachment ID or a post ID.

var fieldKeyRe = regexp.MustCompile(`^field_[A-Za-z0-9_]+$`)

// ACF field kinds that hold IDs needing remapping.
const (
	acfMedia = "media"
	acfPost  = "post"
)

var acfKinds = map[string]string{
	"image": acfMedia, "file": acfMedia, "gallery": acfMedia,
	"post_object": acfPost, "relationship": acfPost, "page_link": acfPost,
}

// acfUnmapped are ID-holding field types that aren't remapped.
var acfUnmapped = map[string]bool{"taxonomy": true, "user": true}

// acfFieldRefs collects value key → field key from an ACF data object.
func acfFieldRefs(data map[string]any, out map[string]string) {
	for k, v := range data {
		if strings.HasPrefix(k, "_") {
			continue
		}
		if fk, ok := data["_"+k].(string); ok && fieldKeyRe.MatchString(fk) {
			out[k] = fk
		} else if fieldKeyRe.MatchString(k) {
			out[k] = k
		}
		switch t := v.(type) {
		case map[string]any:
			acfFieldRefs(t, out)
		case []any:
			for _, e := range t {
				if m, ok := e.(map[string]any); ok {
					acfFieldRefs(m, out)
				}
			}
		}
	}
}

// acfBlockData yields every block's raw attributes and decoded "data"
// object, for blocks that have one.
func acfBlockData(content string, fn func(attrs string, data map[string]any)) {
	for _, m := range blockRe.FindAllStringSubmatch(content, -1) {
		var attrs map[string]any
		if json.Unmarshal([]byte(m[2]), &attrs) != nil {
			continue
		}
		if data, ok := attrs["data"].(map[string]any); ok {
			fn(m[2], data)
		}
	}
}

// acfFieldTypes looks up field types by key on the local site. nil when
// ACF isn't active.
func acfFieldTypes(r Runner, keys []string) (map[string]string, error) {
	valid := []string{}
	for _, k := range keys {
		if fieldKeyRe.MatchString(k) {
			valid = append(valid, `"`+k+`"`)
		}
	}
	if len(valid) == 0 {
		return map[string]string{}, nil
	}
	php := fmt.Sprintf(`if (!function_exists("acf_get_field")) { echo "null"; return; } $o = array(); foreach (array(%s) as $k) { $f = acf_get_field($k); $o[$k] = $f ? $f["type"] : ""; } echo json_encode((object) $o);`, strings.Join(valid, ","))
	out, err := r.Local("eval", php)
	if err != nil {
		return nil, err
	}
	if lastLine(out) == "null" {
		return nil, nil
	}
	types := map[string]string{}
	if err := parseJSON(out, &types); err != nil {
		return nil, err
	}
	return types, nil
}

// ACFPlan is what ACF fields in the post reference.
type ACFPlan struct {
	ContentKeys map[string]string // block data key → kind
	MetaKinds   map[string]string // meta key → kind
	MediaIDs    []int
	PostIDs     []int
}

func acfValueRe(key string) *regexp.Regexp {
	return regexp.MustCompile(`("` + regexp.QuoteMeta(key) + `"\s*:\s*)(\[[^\]]*\]|"[^"\\]*"|-?\d+)`)
}

var dataStartRe = regexp.MustCompile(`"data"\s*:\s*\{`)

// tokenIDs returns the IDs in a JSON token that is a number, a numeric
// string, or an array of those. Anything else (e.g. a URL) yields nothing.
func tokenIDs(tok string) []int {
	tok = strings.TrimSpace(tok)
	if strings.HasPrefix(tok, "[") {
		var arr []any
		if json.Unmarshal([]byte(tok), &arr) != nil {
			return nil
		}
		var ids []int
		for _, e := range arr {
			ids = append(ids, scalarIDs(e)...)
		}
		return ids
	}
	var v any
	if json.Unmarshal([]byte(tok), &v) != nil {
		return nil
	}
	return scalarIDs(v)
}

func scalarIDs(v any) []int {
	switch t := v.(type) {
	case float64:
		if t > 0 && t == float64(int(t)) {
			return []int{int(t)}
		}
	case string:
		if n, err := strconv.Atoi(t); err == nil && n > 0 && itoa(n) == t {
			return []int{n}
		}
	}
	return nil
}

// mapToken remaps the IDs in a token, preserving its exact formatting.
func mapToken(tok string, m map[int]int) string {
	if len(tokenIDs(tok)) == 0 {
		return tok
	}
	return numberRe.ReplaceAllStringFunc(tok, func(s string) string {
		n, _ := strconv.Atoi(s)
		if to, ok := m[n]; ok {
			return itoa(to)
		}
		return s
	})
}

// planACF finds ACF fields in the content (and in meta when withMeta) that
// hold attachment or post IDs. Returns nil when there are none.
func planACF(r Runner, p *Plan, content string, meta []MetaEntry, withMeta bool) (*ACFPlan, error) {
	contentRefs := map[string]string{}
	acfBlockData(content, func(_ string, data map[string]any) { acfFieldRefs(data, contentRefs) })

	metaRefs := map[string]string{}
	if withMeta {
		for _, m := range meta {
			var fk string
			if json.Unmarshal(m.Value, &fk) == nil && fieldKeyRe.MatchString(fk) && strings.HasPrefix(m.Key, "_") {
				metaRefs[strings.TrimPrefix(m.Key, "_")] = fk
			}
		}
	}
	if len(contentRefs) == 0 && len(metaRefs) == 0 {
		return nil, nil
	}

	keySet := map[string]bool{}
	for _, fk := range contentRefs {
		keySet[fk] = true
	}
	for _, fk := range metaRefs {
		keySet[fk] = true
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	types, err := acfFieldTypes(r, keys)
	if err != nil {
		return nil, fmt.Errorf("reading ACF field types: %w", err)
	}
	if types == nil {
		p.Warnings = append(p.Warnings, "the content has ACF block data but ACF isn't active locally; IDs in it are NOT remapped")
		return nil, nil
	}

	a := &ACFPlan{ContentKeys: map[string]string{}, MetaKinds: map[string]string{}}
	unmapped := []string{}
	for k, fk := range contentRefs {
		if kind := acfKinds[types[fk]]; kind != "" {
			a.ContentKeys[k] = kind
		} else if acfUnmapped[types[fk]] {
			unmapped = appendUnique(unmapped, k)
		}
	}
	for k, fk := range metaRefs {
		if kind := acfKinds[types[fk]]; kind != "" {
			a.MetaKinds[k] = kind
		} else if acfUnmapped[types[fk]] {
			unmapped = appendUnique(unmapped, k)
		}
	}
	if len(unmapped) > 0 {
		sort.Strings(unmapped)
		p.Warnings = append(p.Warnings, "ACF taxonomy/user fields are copied as-is, NOT remapped: "+strings.Join(unmapped, ", "))
	}

	addIDs := func(kind string, ids []int) {
		for _, id := range ids {
			if kind == acfMedia && !containsInt(a.MediaIDs, id) {
				a.MediaIDs = append(a.MediaIDs, id)
			}
			if kind == acfPost && !containsInt(a.PostIDs, id) {
				a.PostIDs = append(a.PostIDs, id)
			}
		}
	}
	acfBlockData(content, func(attrs string, _ map[string]any) {
		loc := dataStartRe.FindStringIndex(attrs)
		if loc == nil {
			return
		}
		region := attrs[loc[0]:]
		for k, kind := range a.ContentKeys {
			for _, m := range acfValueRe(k).FindAllStringSubmatch(region, -1) {
				addIDs(kind, tokenIDs(m[2]))
			}
		}
	})
	for _, m := range meta {
		if kind, ok := a.MetaKinds[m.Key]; ok {
			addIDs(kind, tokenIDs(string(m.Value)))
		}
	}
	sort.Ints(a.MediaIDs)
	sort.Ints(a.PostIDs)
	return a, nil
}

// RemapACFContent rewrites attachment and post IDs inside ACF block data.
func RemapACFContent(content string, keys map[string]string, mediaMap, postMap map[int]int) string {
	if len(keys) == 0 {
		return content
	}
	return blockRe.ReplaceAllStringFunc(content, func(block string) string {
		m := blockRe.FindStringSubmatch(block)
		attrs := m[2]
		loc := dataStartRe.FindStringIndex(attrs)
		if loc == nil {
			return block
		}
		head, region := attrs[:loc[0]], attrs[loc[0]:]
		for k, kind := range keys {
			idMap := mediaMap
			if kind == acfPost {
				idMap = postMap
			}
			re := acfValueRe(k)
			region = re.ReplaceAllStringFunc(region, func(s string) string {
				sm := re.FindStringSubmatch(s)
				return sm[1] + mapToken(sm[2], idMap)
			})
		}
		if head+region == attrs {
			return block
		}
		return strings.Replace(block, attrs, head+region, 1)
	})
}
