package postsync

import (
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// blockRe matches a block comment delimiter carrying JSON attributes, e.g.
// `<!-- wp:image {"id":12,"sizeSlug":"large"} -->`. Gutenberg escapes "--"
// inside attributes, so the first "} -->" (or "} /-->") always ends them.
var blockRe = regexp.MustCompile(`<!--\s+wp:([a-z][a-z0-9_-]*(?:/[a-z][a-z0-9_-]*)?)\s+(\{[\s\S]*?\})\s+(/)?-->`)

// mediaIDAttrs lists, per block, the top-level attributes holding a single
// attachment ID.
var mediaIDAttrs = map[string][]string{
	"core/image":      {"id"},
	"core/cover":      {"id"},
	"core/media-text": {"mediaId"},
	"core/video":      {"id"},
	"core/audio":      {"id"},
	"core/file":       {"id"},
}

// galleryIDsRe matches core/gallery's legacy `"ids":[1,2,3]` attribute.
var galleryIDsRe = regexp.MustCompile(`"ids"\s*:\s*\[([\d,\s]*)\]`)

var wpImageClassRe = regexp.MustCompile(`\bwp-image-(\d+)\b`)

var numberRe = regexp.MustCompile(`\d+`)

func normalizeBlockName(name string) string {
	if !strings.Contains(name, "/") {
		return "core/" + name
	}
	return name
}

// CollectAttachmentIDs returns every attachment ID referenced by content:
// media block attributes, gallery "ids" and wp-image-N classes. Sorted and
// de-duplicated.
func CollectAttachmentIDs(content string) []int {
	seen := map[int]bool{}
	for _, m := range blockRe.FindAllStringSubmatch(content, -1) {
		name := normalizeBlockName(m[1])
		var attrs map[string]any
		if json.Unmarshal([]byte(m[2]), &attrs) != nil {
			continue
		}
		for _, key := range mediaIDAttrs[name] {
			if id, ok := attrs[key].(float64); ok && id > 0 {
				seen[int(id)] = true
			}
		}
		if name == "core/gallery" {
			if ids, ok := attrs["ids"].([]any); ok {
				for _, v := range ids {
					if id, ok := v.(float64); ok && id > 0 {
						seen[int(id)] = true
					}
				}
			}
		}
	}
	for _, m := range wpImageClassRe.FindAllStringSubmatch(content, -1) {
		if id, err := strconv.Atoi(m[1]); err == nil && id > 0 {
			seen[id] = true
		}
	}
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// RemapAttachmentIDs rewrites local attachment IDs to remote ones in media
// block attributes, gallery "ids" and wp-image-N classes. IDs missing from
// idMap are left untouched. Every replacement is a single pass, so a
// mapping like 12→34 alongside 34→56 can't chain.
func RemapAttachmentIDs(content string, idMap map[int]int) string {
	if len(idMap) == 0 {
		return content
	}
	mapNum := func(s string) string {
		n, err := strconv.Atoi(s)
		if err != nil {
			return s
		}
		if to, ok := idMap[n]; ok {
			return strconv.Itoa(to)
		}
		return s
	}

	content = blockRe.ReplaceAllStringFunc(content, func(block string) string {
		m := blockRe.FindStringSubmatch(block)
		name := normalizeBlockName(m[1])
		attrs := m[2]
		newAttrs := attrs
		for _, key := range mediaIDAttrs[name] {
			re := regexp.MustCompile(`("` + regexp.QuoteMeta(key) + `"\s*:\s*)(\d+)`)
			// Only the first occurrence: the top-level attribute is
			// serialized before any nested object could repeat the key.
			replaced := false
			newAttrs = re.ReplaceAllStringFunc(newAttrs, func(s string) string {
				if replaced {
					return s
				}
				replaced = true
				sm := re.FindStringSubmatch(s)
				return sm[1] + mapNum(sm[2])
			})
		}
		if name == "core/gallery" {
			newAttrs = galleryIDsRe.ReplaceAllStringFunc(newAttrs, func(s string) string {
				return numberRe.ReplaceAllStringFunc(s, mapNum)
			})
		}
		if newAttrs == attrs {
			return block
		}
		return strings.Replace(block, attrs, newAttrs, 1)
	})

	return wpImageClassRe.ReplaceAllStringFunc(content, func(s string) string {
		return "wp-image-" + mapNum(strings.TrimPrefix(s, "wp-image-"))
	})
}

// domainRe matches "//<domain>" (optionally JSON-escaped as "\/\/<domain>")
// followed by a character that can't continue a hostname, so
// "site.localhost" never matches inside "site.localhost.example".
func domainRe(domain string) *regexp.Regexp {
	return regexp.MustCompile(`(\\?/\\?/)` + regexp.QuoteMeta(domain) + `([^A-Za-z0-9.-]|$)`)
}

// ReplaceDomain rewrites every //from URL to //to, keeping the scheme.
func ReplaceDomain(s, from, to string) string {
	if from == "" || to == "" || from == to {
		return s
	}
	re := domainRe(from)
	return re.ReplaceAllStringFunc(s, func(m string) string {
		sm := re.FindStringSubmatch(m)
		return sm[1] + to + sm[2]
	})
}

// LocalRefs are URLs on the local domain found in content.
type LocalRefs struct {
	// Uploads are wp-content/uploads-relative file paths, e.g.
	// "2025/03/photo-1024x768.jpg".
	Uploads []string
	// Links are every other local URL path (internal links, which may
	// point at pages that don't exist on the remote).
	Links []string
}

// CollectLocalRefs finds URLs on localDomain in content.
func CollectLocalRefs(content, localDomain string) LocalRefs {
	var refs LocalRefs
	if localDomain == "" {
		return refs
	}
	plain := strings.ReplaceAll(content, `\/`, "/")
	re := regexp.MustCompile(`//` + regexp.QuoteMeta(localDomain) + `(?::\d+)?(/[^"'\s<>()]*)?`)
	seenU, seenL := map[string]bool{}, map[string]bool{}
	for _, m := range re.FindAllStringSubmatchIndex(plain, -1) {
		// Same hostname boundary as ReplaceDomain.
		if end := m[1]; m[2] < 0 && end < len(plain) && isHostChar(plain[end]) {
			continue
		}
		p := ""
		if m[2] >= 0 {
			p = plain[m[2]:m[3]]
		}
		if i := strings.IndexAny(p, "?#"); i >= 0 {
			p = p[:i]
		}
		if rel, ok := strings.CutPrefix(p, "/wp-content/uploads/"); ok {
			if dec, err := url.PathUnescape(rel); err == nil {
				rel = dec
			}
			rel = path.Clean(rel)
			if rel == "." || strings.HasPrefix(rel, "..") || seenU[rel] {
				continue
			}
			seenU[rel] = true
			refs.Uploads = append(refs.Uploads, rel)
			continue
		}
		if p == "" {
			p = "/"
		}
		if !seenL[p] {
			seenL[p] = true
			refs.Links = append(refs.Links, p)
		}
	}
	return refs
}

func isHostChar(c byte) bool {
	return c == '.' || c == '-' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

var sizeSuffixRe = regexp.MustCompile(`(?:-\d+x\d+|-scaled|-rotated)+(\.[A-Za-z0-9]+)$`)

// baseUpload strips WordPress's generated-size suffixes ("-1024x768",
// "-scaled") so a resized variant can be matched to its attachment.
func baseUpload(rel string) string {
	return sizeSuffixRe.ReplaceAllString(rel, "$1")
}
