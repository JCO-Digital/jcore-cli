package postsync

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Polylang has no WP-CLI commands for post languages or translations, so
// its public PHP API (pll_* functions, present in both Polylang and
// Polylang Pro) is called through `wp eval`. Only integers and validated
// language/field slugs are ever interpolated into that PHP.

var langSlugRe = regexp.MustCompile(`^[a-z0-9_-]+$`)

// PLLState describes Polylang on one site; nil means it isn't active.
type PLLState struct {
	Languages []string `json:"languages"`
	Default   string   `json:"default"`
	Media     bool     `json:"media"` // media translation enabled
	Sync      []string `json:"sync"`  // data Polylang copies between translations
}

// HasLanguage reports whether lang is configured on the site.
func (s *PLLState) HasLanguage(lang string) bool {
	if s == nil {
		return false
	}
	for _, l := range s.Languages {
		if l == lang {
			return true
		}
	}
	return false
}

const pllDetectPHP = `if (!function_exists("pll_languages_list") || !function_exists("PLL")) { echo "null"; return; }
$o = PLL()->options;
echo json_encode(array(
  "languages" => pll_languages_list(array("fields" => "slug")),
  "default" => (string) pll_default_language(),
  "media" => !empty($o["media_support"]),
  "sync" => array_values((array) $o["sync"]),
));`

func detectPolylang(run func(...string) (string, error)) (*PLLState, error) {
	out, err := run("eval", pllDetectPHP)
	if err != nil {
		return nil, err
	}
	if lastLine(out) == "null" {
		return nil, nil
	}
	var s PLLState
	if err := parseJSON(out, &s); err != nil {
		return nil, err
	}
	for _, l := range s.Languages {
		if !langSlugRe.MatchString(l) {
			return nil, fmt.Errorf("unexpected Polylang language slug %q", l)
		}
	}
	return &s, nil
}

// PLLInfo is an object's language and translation group.
type PLLInfo struct {
	Lang         string         `json:"lang"`
	Translations map[string]int `json:"tr"`
	Translated   bool           `json:"translated"` // its post type/taxonomy is translatable
}

func intList(ids []int) string {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = itoa(id)
	}
	return strings.Join(s, ",")
}

// pllInfo returns the language and translations of posts (kind "post") or
// terms (kind "term").
func pllInfo(run func(...string) (string, error), kind string, ids []int) (map[int]PLLInfo, error) {
	res := map[int]PLLInfo{}
	if len(ids) == 0 {
		return res, nil
	}
	translated := `pll_is_translated_post_type(get_post_type($id))`
	if kind == "term" {
		translated = `(($t = get_term($id)) && !is_wp_error($t) && pll_is_translated_taxonomy($t->taxonomy))`
	}
	php := fmt.Sprintf(`$o = array(); foreach (array(%s) as $id) { $o[$id] = array("lang" => (string) pll_get_%[2]s_language($id, "slug"), "tr" => (object) pll_get_%[2]s_translations($id), "translated" => (bool) %[3]s); } echo json_encode((object) $o);`, intList(ids), kind, translated)
	out, err := run("eval", php)
	if err != nil {
		return nil, err
	}
	var raw map[string]PLLInfo
	if err := parseJSON(out, &raw); err != nil {
		return nil, err
	}
	for k, v := range raw {
		var id int
		fmt.Sscan(k, &id)
		res[id] = v
	}
	return res, nil
}

// filterByLang keeps the IDs whose language is lang.
func filterByLang(run func(...string) (string, error), kind string, ids []int, lang string) ([]int, error) {
	info, err := pllInfo(run, kind, ids)
	if err != nil {
		return nil, err
	}
	var out []int
	for _, id := range ids {
		if info[id].Lang == lang {
			out = append(out, id)
		}
	}
	return out, nil
}

// PLLPlan is the Polylang part of a post sync.
type PLLPlan struct {
	Lang string // the post's language
	// Group is the remote translation group to save (language → remote
	// post ID), with 0 standing in for the synced post when it's created.
	// nil: nothing to save.
	Group   map[string]int
	NewLink []string // languages newly linked, for the summary
	SetLang bool     // set the language on the remote post (always on create)
}

// mergeGroup adds candidate's translation group into group unless that
// would put two posts in the same language; returns whether it merged.
func mergeGroup(group map[string]int, candidate map[string]int) bool {
	for lang, id := range candidate {
		if cur, ok := group[lang]; ok && cur != id {
			return false
		}
	}
	for lang, id := range candidate {
		group[lang] = id
	}
	return true
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// pllSaveGroupPHP returns PHP that sets the post's language (if setLang)
// and saves its translation group, with 0 in group replaced by postID.
//
// Setting the language first, then re-saving the slug, lets Polylang Pro's
// shared slugs apply: on insert it didn't know the language yet, so a slug
// used by another language got a "-2" suffix. The final slug is echoed.
func pllSaveGroupPHP(postID int, lang, slug string, group map[string]int, setLang bool) string {
	var b strings.Builder
	if setLang {
		fmt.Fprintf(&b, `pll_set_post_language(%d, "%s"); `, postID, lang)
		fmt.Fprintf(&b, `if (get_post_field("post_name", %[1]d) !== %[2]s) { wp_update_post(array("ID" => %[1]d, "post_name" => %[2]s)); } `, postID, phpString(slug))
	}
	if len(group) > 1 {
		parts := []string{}
		for _, l := range sortedKeys(group) {
			id := group[l]
			if id == 0 {
				id = postID
			}
			parts = append(parts, fmt.Sprintf(`"%s" => %d`, l, id))
		}
		fmt.Fprintf(&b, `pll_save_post_translations(array(%s)); `, strings.Join(parts, ", "))
	}
	fmt.Fprintf(&b, `echo get_post_field("post_name", %d);`, postID)
	return b.String()
}

// phpString quotes s as a single-quoted PHP string literal (no
// interpolation; only backslash and quote need escaping).
func phpString(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

var locationRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// pllNavMenusPHP reads Polylang's per-language menu locations for the
// active theme: location → language → menu ID.
const pllNavMenusPHP = `$o = PLL()->options;
$nm = is_array($o) ? (isset($o["nav_menus"]) ? $o["nav_menus"] : array()) : $o->get("nav_menus");
$t = get_stylesheet();
echo json_encode(isset($nm[$t]) ? (object) $nm[$t] : new stdClass);`

func pllNavMenus(run func(...string) (string, error)) (map[string]map[string]int, error) {
	out, err := run("eval", pllNavMenusPHP)
	if err != nil {
		return nil, err
	}
	var raw map[string]map[string]flexInt
	if err := parseJSON(out, &raw); err != nil {
		// An empty location can be stored as a JSON array.
		return map[string]map[string]int{}, nil
	}
	res := map[string]map[string]int{}
	for loc, langs := range raw {
		res[loc] = map[string]int{}
		for l, id := range langs {
			res[loc][l] = int(id)
		}
	}
	return res, nil
}

// pllAssignMenuPHP assigns menuID to location for lang through Polylang's
// own options API (3.7+ refuses direct update_option writes).
func pllAssignMenuPHP(menuID int, location, lang string) string {
	return fmt.Sprintf(`$o = PLL()->options; $t = get_stylesheet();
if (is_array($o)) {
  $nm = isset($o["nav_menus"]) ? $o["nav_menus"] : array();
  $nm[$t]["%[2]s"]["%[3]s"] = %[1]d;
  $o["nav_menus"] = $nm; PLL()->options = $o; update_option("polylang", $o);
} else {
  $nm = $o->get("nav_menus"); $nm[$t]["%[2]s"]["%[3]s"] = %[1]d;
  $e = $o->set("nav_menus", $nm);
  if ($e->has_errors()) { WP_CLI::error($e->get_error_message()); }
  $o->save();
}
echo "ok";`, menuID, location, lang)
}
