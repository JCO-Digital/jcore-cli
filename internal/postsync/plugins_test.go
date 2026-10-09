package postsync

import (
	"fmt"
	"strings"
	"testing"
)

const (
	pllDetectKey = `eval if (!function_exists("pll_languages_list")`
	acfTypesKey  = `L eval if (!function_exists("acf_get_field"))`
	gfLocalKey   = `L eval if (!class_exists("GFAPI")) { echo "null"; return; } $o = array(); foreach (array(`
	gfRemoteKey  = "R eval if (!class_exists(\"GFAPI\")) { echo \"null\"; return; }\n$o = array(); foreach (GFAPI::get_forms"
	pllLangsJSON = `{"languages":["fi","en"],"default":"fi","media":false,"sync":[]}`
)

// pllKey is the fake-runner prefix of a pllInfo call.
func pllKey(side, kind, ids string) string {
	return fmt.Sprintf(`%s eval $o = array(); foreach (array(%s) as $id) { $o[$id] = array("lang" => (string) pll_get_%s_language`, side, ids, kind)
}

func lastCallWithPrefix(f *fakeRunner, prefix string) string {
	c := callsWithPrefix(f, prefix)
	if len(c) == 0 {
		return ""
	}
	return c[len(c)-1]
}

// pllFake: local English page "about" (5) whose Finnish translation is
// "meista" (6). On the remote, "about" exists in Finnish (300) and English
// (301, unlinked), and "meista" exists in Finnish (400).
func pllFake() *fakeRunner {
	f := baseFake()
	f.responses["L "+pllDetectKey] = pllLangsJSON
	f.responses["R "+pllDetectKey] = pllLangsJSON
	f.responses[pllKey("L", "post", "5")] = `{"5":{"lang":"en","tr":{"en":5,"fi":6},"translated":true}}`
	f.responses["L post get 6 --format=json"] = `{"ID":6,"post_type":"page","post_name":"meista","post_title":"Meistä","post_status":"publish"}`

	f.responses["R post list --post_type=page --name=about --post_status=any"] = `[{"ID":300,"post_type":"page"},{"ID":301,"post_type":"page"}]`
	f.responses[pllKey("R", "post", "300,301")] = `{"300":{"lang":"fi","tr":{"fi":300},"translated":true},"301":{"lang":"en","tr":{"en":301},"translated":true}}`
	f.responses["R post get 301 --format=json"] = `{"ID":301,"post_type":"page","post_name":"about","post_title":"About old","post_status":"publish","post_content":"old","post_modified_gmt":"2025-03-01 10:00:00"}`
	f.responses["R post meta list 301"] = `[]`
	f.responses["R post update 301"] = "Success\n"

	f.responses["R post list --post_type=page --name=meista --post_status=any"] = `[{"ID":400,"post_type":"page"}]`
	f.responses[pllKey("R", "post", "400")] = `{"400":{"lang":"fi","tr":{"fi":400},"translated":true}}`

	f.responses["R post list --post_type=attachment"] = `[{"ID":77}]`
	f.files["2025/03/a.jpg"] = FileIdentical
	f.responses["R eval pll_"] = "about\n"
	return f
}

func TestPolylangUpdateMatchesLanguageAndLinksTranslation(t *testing.T) {
	f := pllFake()
	opts := testOpts("about")
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Remote == nil || p.Remote.ID != 301 {
		t.Fatalf("matched %+v, want the English remote post 301", p.Remote)
	}
	if p.PLL == nil || p.PLL.SetLang || strings.Join(p.PLL.NewLink, ",") != "fi" {
		t.Fatalf("pll plan: %+v", p.PLL)
	}
	if w := f.writeCalls(); len(w) > 0 {
		t.Fatalf("Prepare wrote: %v", w)
	}
	if _, err := Apply(f, p, opts, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	php := lastCallWithPrefix(f, "R eval pll_")
	if !strings.Contains(php, `pll_save_post_translations(array("en" => 301, "fi" => 400))`) || strings.Contains(php, "pll_set_post_language") {
		t.Fatalf("polylang write: %s", php)
	}
	// The language step must run before the read-back, after the update.
	if !strings.Contains(php, "--user=admin") {
		t.Fatalf("polylang write not run as a user: %s", php)
	}
}

func TestPolylangCreateSetsLanguageAndSlug(t *testing.T) {
	f := pllFake()
	f.responses["R post list --post_type=page --name=about --post_status=any"] = `[{"ID":300,"post_type":"page"}]`
	f.responses[pllKey("R", "post", "300")] = `{"300":{"lang":"fi","tr":{"fi":300},"translated":true}}`
	f.responses["R post list --post_type=attachment"] = `[]`
	f.files["2025/03/a.jpg"] = FileMissing
	opts := testOpts("about")
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Creating() || !p.PLL.SetLang {
		t.Fatalf("expected a create with language: %+v", p.PLL)
	}
	res, err := Apply(f, p, opts, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	php := lastCallWithPrefix(f, "R eval pll_set_post_language(200")
	for _, want := range []string{`pll_set_post_language(200, "en")`, `"post_name" => 'about'`, `pll_save_post_translations(array("en" => 200, "fi" => 400))`} {
		if !strings.Contains(php, want) {
			t.Errorf("polylang write missing %s:\n%s", want, php)
		}
	}
	if !strings.Contains(strings.Join(res.Done, "\n"), "set language en") {
		t.Errorf("done: %v", res.Done)
	}
}

func TestPolylangConflictingGroupNotLinked(t *testing.T) {
	f := pllFake()
	// The remote Finnish page is already linked to a different English page.
	f.responses[pllKey("R", "post", "400")] = `{"400":{"lang":"fi","tr":{"fi":400,"en":999},"translated":true}}`
	p, err := Prepare(f, testOpts("about"))
	if err != nil {
		t.Fatal(err)
	}
	if p.PLL.Group != nil || len(p.PLL.NewLink) != 0 {
		t.Fatalf("expected no linking: %+v", p.PLL)
	}
	if !strings.Contains(strings.Join(p.Warnings, "\n"), "already linked to other posts") {
		t.Fatalf("warnings: %v", p.Warnings)
	}
}

func TestPolylangRemoteWithoutLanguageAborts(t *testing.T) {
	f := pllFake()
	f.responses[pllKey("R", "post", "300,301")] = `{"300":{"lang":"fi","tr":{},"translated":true},"301":{"lang":"","tr":{},"translated":true}}`
	_, err := Prepare(f, testOpts("about"))
	if err == nil || !strings.Contains(err.Error(), "no Polylang language") {
		t.Fatalf("err = %v", err)
	}
}

func TestPolylangRemoteIDLanguage(t *testing.T) {
	f := pllFake()
	f.responses[pllKey("R", "post", "301")] = `{"301":{"lang":"en","tr":{"en":301},"translated":true}}`
	f.responses["R post get 300 --format=json"] = `{"ID":300,"post_type":"page","post_name":"about","post_status":"publish"}`
	f.responses[pllKey("R", "post", "300")] = `{"300":{"lang":"fi","tr":{"fi":300},"translated":true}}`
	opts := testOpts("about")

	opts.RemoteID = 301
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Remote == nil || p.Remote.ID != 301 || p.remotePLLInfo == nil || p.remotePLLInfo.Lang != "en" {
		t.Fatalf("remote = %+v, pll = %+v", p.Remote, p.remotePLLInfo)
	}

	opts.RemoteID = 300
	if _, err := Prepare(f, opts); err == nil || !strings.Contains(err.Error(), `in language "fi", but the local post is in "en"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestPolylangMissingOnRemoteAborts(t *testing.T) {
	f := pllFake()
	f.responses["R "+pllDetectKey] = "null\n"
	_, err := Prepare(f, testOpts("about"))
	if err == nil || !strings.Contains(err.Error(), "Polylang isn't active on the remote") {
		t.Fatalf("err = %v", err)
	}
}

func TestPolylangLangFlagNarrowsSlug(t *testing.T) {
	f := pllFake()
	f.responses["L post list --post_type=any --name=about"] = `[{"ID":5,"post_type":"page"},{"ID":8,"post_type":"page"}]`
	f.responses[pllKey("L", "post", "5,8")] = `{"5":{"lang":"en"},"8":{"lang":"fi"}}`
	if _, err := Prepare(f, testOpts("about")); err == nil || !strings.Contains(err.Error(), "[en]") {
		t.Fatalf("expected an ambiguity error listing languages, got %v", err)
	}
	opts := testOpts("about")
	opts.Lang = "en"
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Local.ID != 5 {
		t.Fatalf("picked %d", p.Local.ID)
	}
}

const acfContent = `<!-- wp:acf/hero {"name":"acf/hero","data":{"title":"Hi 12","_title":"field_t","image":12,"_image":"field_i","gallery":["12","13"],"_gallery":"field_g","link_page":"7","_link_page":"field_p"},"mode":"preview"} /-->`

func acfFake() *fakeRunner {
	f := baseFake()
	f.responses["L post get 5 --format=json"] = `{"ID":5,"post_type":"page","post_name":"about","post_title":"About","post_status":"publish","post_content":` + jsonString(acfContent) + `,"post_modified_gmt":"2025-03-10 10:00:00"}`
	f.responses[acfTypesKey] = `{"field_g":"gallery","field_i":"image","field_p":"post_object","field_t":"text"}`
	f.responses["L post get 13 --format=json"] = `{"ID":13,"post_type":"attachment","post_title":"B"}`
	f.responses["L post meta list 13"] = `[{"meta_key":"_wp_attached_file","meta_value":"2025/03/b.jpg"}]`
	f.responses["R post list --post_type=attachment --post_status=any --meta_key=_wp_attached_file --meta_value=2025/03/a.jpg"] = `[{"ID":77}]`
	f.responses["R post list --post_type=attachment --post_status=any --meta_key=_wp_attached_file --meta_value=2025/03/b.jpg"] = `[{"ID":78}]`
	f.responses["L post get 7 --format=json"] = `{"ID":7,"post_type":"page","post_name":"contact","post_status":"publish"}`
	f.responses["R post list --post_type=page --name=contact --post_status=any"] = `[{"ID":350}]`
	return f
}

func TestACFBlockIDsRemapped(t *testing.T) {
	f := acfFake()
	opts := testOpts("about")
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	got := p.FinalContent(opts)
	want := `<!-- wp:acf/hero {"name":"acf/hero","data":{"title":"Hi 12","_title":"field_t","image":77,"_image":"field_i","gallery":["77","78"],"_gallery":"field_g","link_page":"350","_link_page":"field_p"},"mode":"preview"} /-->`
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestACFMissingLinkedPostAborts(t *testing.T) {
	f := acfFake()
	f.responses["R post list --post_type=page --name=contact --post_status=any"] = `[]`
	_, err := Prepare(f, testOpts("about"))
	if err == nil || !strings.Contains(err.Error(), "jcore sync 7") {
		t.Fatalf("err = %v", err)
	}
}

func TestACFMetaRemappedWithMetaFlag(t *testing.T) {
	f := acfFake()
	f.responses["L post meta list 5"] = `[{"meta_key":"hero_image","meta_value":"13"},{"meta_key":"_hero_image","meta_value":"field_i"},{"meta_key":"related","meta_value":["7"]},{"meta_key":"_related","meta_value":"field_r"}]`
	f.responses[acfTypesKey] = `{"field_g":"gallery","field_i":"image","field_p":"post_object","field_r":"relationship","field_t":"text"}`
	opts := testOpts("about")
	opts.Meta = true
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, m := range p.Meta {
		got[m.Key] = string(p.finalMeta(m))
	}
	if got["hero_image"] != `"78"` || got["related"] != `["350"]` || got["_hero_image"] != `"field_i"` {
		t.Fatalf("meta: %v", got)
	}
	if strings.Contains(strings.Join(p.Warnings, "\n"), "look like post/attachment IDs") {
		t.Fatalf("ACF-typed meta shouldn't get the generic ID warning: %v", p.Warnings)
	}
}

func gfFake(content string) *fakeRunner {
	f := baseFake()
	f.responses["L post get 5 --format=json"] = `{"ID":5,"post_type":"page","post_name":"about","post_title":"About","post_status":"publish","post_content":` + jsonString(content) + `,"post_modified_gmt":"2025-03-10 10:00:00"}`
	f.responses[gfLocalKey] = `{"3":"Contact"}`
	f.responses[gfRemoteKey] = `[{"id":9,"title":"Contact","active":true},{"id":10,"title":"Newsletter","active":true}]`
	return f
}

func TestGravityFormsRemappedByTitle(t *testing.T) {
	content := `<!-- wp:gravityforms/form {"formId":"3","title":false} /--><p>[gravityform id="3" title="false"]</p>`
	f := gfFake(content)
	opts := testOpts("about")
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := `<!-- wp:gravityforms/form {"formId":"9","title":false} /--><p>[gravityform id="9" title="false"]</p>`
	if got := p.FinalContent(opts); got != want {
		t.Fatalf("got %s", got)
	}
}

func TestGravityFormMissingOnRemoteAborts(t *testing.T) {
	f := gfFake(`[gravityform id=3]`)
	f.responses[gfRemoteKey] = `[{"id":10,"title":"Newsletter","active":true}]`
	_, err := Prepare(f, testOpts("about"))
	if err == nil || !strings.Contains(err.Error(), `form "Contact" is missing on the remote`) {
		t.Fatalf("err = %v", err)
	}
}

func TestPolylangMenuLocations(t *testing.T) {
	f := menuFake()
	f.responses["L "+pllDetectKey] = pllLangsJSON
	f.responses["R "+pllDetectKey] = pllLangsJSON
	f.responses["L eval $o = PLL()->options;"] = `{"primary":{"fi":3,"en":4}}`
	f.responses["R eval $o = PLL()->options;"] = `{"primary":{"en":55}}`
	f.responses[pllKey("L", "post", "5")] = `{"5":{"lang":"fi","tr":{},"translated":true}}`
	f.responses[pllKey("R", "post", "300")] = `{"300":{"lang":"fi","tr":{},"translated":true}}`
	f.responses[pllKey("L", "term", "8")] = `{"8":{"lang":"fi","tr":{},"translated":true}}`
	f.responses[pllKey("R", "term", "44")] = `{"44":{"lang":"fi","tr":{},"translated":true}}`
	f.responses["R eval $o = PLL()->options; $t = get_stylesheet();"] = "ok\n"

	p, err := PrepareMenu(f, testOpts(""), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Locations) != 0 || strings.Join(p.PLLLocations["primary"], ",") != "fi" {
		t.Fatalf("locations=%v pll=%v", p.Locations, p.PLLLocations)
	}
	if _, err := ApplyMenu(f, p, false, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	php := lastCallWithPrefix(f, "R eval $o = PLL()->options; $t = get_stylesheet();")
	if !strings.Contains(php, `["primary"]["fi"] = 9`) {
		t.Fatalf("assign: %s", php)
	}
	if c := callsWithPrefix(f, "R menu location assign"); len(c) > 0 {
		t.Fatalf("plain location assign used with Polylang: %v", c)
	}
}

func TestPolylangMenuPicksSameLanguagePage(t *testing.T) {
	f := menuFake()
	f.responses["L "+pllDetectKey] = pllLangsJSON
	f.responses["R "+pllDetectKey] = pllLangsJSON
	f.responses["L eval $o = PLL()->options;"] = `{}`
	f.responses[pllKey("L", "post", "5")] = `{"5":{"lang":"en","translated":true}}`
	f.responses["R post list --post_type=page --name=about --post_status=any"] = `[{"ID":300},{"ID":301}]`
	f.responses[pllKey("R", "post", "300,301")] = `{"300":{"lang":"fi"},"301":{"lang":"en"}}`
	f.responses[pllKey("L", "term", "8")] = `{"8":{"lang":"en"}}`
	f.responses[pllKey("R", "term", "44")] = `{"44":{"lang":"en"}}`
	p, err := PrepareMenu(f, testOpts(""), "main")
	if err != nil {
		t.Fatal(err)
	}
	if p.Ops[0].RemoteObjectID != 301 {
		t.Fatalf("About mapped to %d, want the English 301", p.Ops[0].RemoteObjectID)
	}
}

func TestPHPHelpers(t *testing.T) {
	if got := phpString(`it's a\b`); got != `'it\'s a\\b'` {
		t.Errorf("phpString = %s", got)
	}
	php := pllSaveGroupPHP(12, "en", "about", map[string]int{"en": 0, "fi": 4}, true)
	for _, want := range []string{`pll_set_post_language(12, "en")`, `"en" => 12, "fi" => 4`, `echo get_post_field("post_name", 12);`} {
		if !strings.Contains(php, want) {
			t.Errorf("missing %s in %s", want, php)
		}
	}
	if php := pllSaveGroupPHP(12, "en", "about", map[string]int{"en": 12}, false); strings.Contains(php, "pll_save_post_translations") || strings.Contains(php, "pll_set_post_language") {
		t.Errorf("nothing to save, got %s", php)
	}
}

func TestTokenHelpers(t *testing.T) {
	m := map[int]int{12: 77, 13: 78}
	cases := map[string]string{
		`12`:             `77`,
		`"12"`:           `"77"`,
		`["12", 13]`:     `["77", 78]`,
		`"photo-12.jpg"`: `"photo-12.jpg"`,
		`"https://x/12"`: `"https://x/12"`,
		`99`:             `99`,
	}
	for in, want := range cases {
		if got := mapToken(in, m); got != want {
			t.Errorf("mapToken(%s) = %s, want %s", in, got, want)
		}
	}
}
