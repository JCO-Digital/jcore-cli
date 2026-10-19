package postsync

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

const testRemoteContent = `<!-- wp:image {"id":77} --><img src="https://example.com/wp-content/uploads/2025/03/a.jpg" class="wp-image-77"/><!-- /wp:image -->`

// pullFake is a remote site with page 300 ("about") using attachment 77 and
// featured image 77, and a local site that has neither.
func pullFake() *fakeRunner {
	f := newFake()
	f.responses["L core is-installed"] = ""
	f.failing["L core is-installed --network"] = true
	f.responses["L option get siteurl"] = "https://site.localhost\n"
	f.responses["R option get siteurl"] = "https://example.com\n"
	f.failing["R core is-installed --network"] = true
	f.responses["L eval"] = "null\n"
	f.responses["R eval"] = "null\n"
	f.responses["R post get 300 --format=json"] = `{"ID":300,"post_type":"page","post_name":"about","post_title":"About","post_status":"publish","post_excerpt":"See https://example.com/x","post_content":` + jsonString(testRemoteContent) + `,"post_parent":0,"post_author":"3","post_date":"2025-03-01 12:00:00","post_modified_gmt":"2025-03-10 10:00:00"}`
	f.responses["R post meta list 300"] = `[{"meta_key":"_thumbnail_id","meta_value":"77"},{"meta_key":"_wp_page_template","meta_value":"wide.php"}]`
	f.responses["R post list --post_type=any --name=about"] = `[{"ID":300,"post_type":"page","post_title":"About","post_status":"publish"}]`
	f.responses["L post-type get page"] = "page"
	f.responses["L post list --post_type=page --name=about --post_status=any"] = `[]`
	f.responses["L post list --post_type=page --name=about --post_status=trash"] = `[]`
	f.responses["L user list --role=administrator"] = "admin\n"
	f.responses["R user get 3 --field=user_login"] = "linus\n"
	f.responses["L user get linus --field=ID"] = "4\n"
	f.responses["R post get 77 --format=json"] = `{"ID":77,"post_type":"attachment","post_name":"a","post_title":"A photo","post_status":"inherit","post_excerpt":"cap","post_content":""}`
	f.responses["R post meta list 77"] = `[{"meta_key":"_wp_attached_file","meta_value":"2025/03/a.jpg"},{"meta_key":"_wp_attachment_image_alt","meta_value":"alt text"}]`
	f.responses["L post list --post_type=attachment"] = `[]`
	f.responses["L media import"] = "12\n"
	f.responses["L post create"] = "Success\n50\n"
	f.responses["L post meta update"] = "Success\n"
	f.responses["L post get 50 --field=post_content"] = `<!-- wp:image {"id":12} --><img src="https://site.localhost/wp-content/uploads/2025/03/a.jpg" class="wp-image-12"/><!-- /wp:image -->` + "\n"
	f.responses["L post get 50 --field=post_name"] = "about\n"
	return f
}

func pullOpts(ident string) Options {
	o := testOpts(ident)
	o.Pull = true
	return o
}

func localWriteCalls(f *fakeRunner) []string {
	var out []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, "L post update") || strings.HasPrefix(c, "L post create") ||
			strings.HasPrefix(c, "L media import") || strings.HasPrefix(c, "L post meta update") ||
			strings.HasPrefix(c, "R post update") || strings.HasPrefix(c, "R post create") ||
			strings.HasPrefix(c, "R media import") || strings.HasPrefix(c, "R post meta update") {
			out = append(out, c)
		}
	}
	return out
}

func TestPreparePullMakesNoWrites(t *testing.T) {
	f := pullFake()
	p, err := Prepare(f, pullOpts("about"))
	if err != nil {
		t.Fatal(err)
	}
	if !p.Creating() || p.Source.ID != 300 {
		t.Fatalf("plan: creating=%v remote=%+v", p.Creating(), p.Source)
	}
	if w := localWriteCalls(f); len(w) > 0 || len(f.downloads) > 0 || len(f.uploads) > 0 {
		t.Fatalf("PreparePull wrote: %v downloads %v uploads %v", w, f.downloads, f.uploads)
	}
	if len(p.Imports()) != 1 || len(p.Copies()) != 1 || p.Copies()[0] != "2025/03/a.jpg" {
		t.Fatalf("imports %v downloads %v", p.Imports(), p.Copies())
	}
}

func TestPullCreateFlow(t *testing.T) {
	f := pullFake()
	opts := pullOpts("about")
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Apply(p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created || res.TargetID != 50 || res.BackupPath != "" || res.VerifyMismatch {
		t.Fatalf("result %+v", res)
	}
	if len(f.downloads) != 1 || f.downloads[0] != "2025/03/a.jpg" || len(f.uploads) != 0 {
		t.Fatalf("downloads %v uploads %v", f.downloads, f.uploads)
	}
	imp := lastCallWithPrefix(f, "L media import")
	if !strings.Contains(imp, "/var/www/html/wp-content/uploads/2025/03/a.jpg --skip-copy") || !strings.Contains(imp, "--alt=alt text") {
		t.Fatalf("import call: %s", imp)
	}
	create := lastCallWithPrefix(f, "L post create")
	for _, want := range []string{"--post_status=publish", "--post_date=2025-03-01 12:00:00", "--post_author=4", "--user=admin", "--post_excerpt=See https://site.localhost/x"} {
		if !strings.Contains(create, want) {
			t.Fatalf("create call lacks %q: %s", want, create)
		}
	}
	sent := f.stdin[create]
	if strings.Contains(sent, "example.com") || !strings.Contains(sent, `"id":12`) || !strings.Contains(sent, "wp-image-12") {
		t.Fatalf("content sent: %s", sent)
	}
	if !strings.Contains(lastCallWithPrefix(f, "L post meta update 50 _thumbnail_id"), "_thumbnail_id 12") {
		t.Fatalf("featured image not set: %v", f.calls)
	}
	if !strings.Contains(lastCallWithPrefix(f, "L post meta update 50 _wp_page_template"), "wide.php") {
		t.Fatalf("template not set: %v", f.calls)
	}
	for _, c := range f.calls {
		if strings.HasPrefix(c, "R ") && (strings.Contains(c, "update") || strings.Contains(c, "create") || strings.Contains(c, "import")) {
			t.Fatalf("pull wrote to the remote: %s", c)
		}
	}
}

func localExisting(f *fakeRunner, modified string) {
	f.responses["L post list --post_type=page --name=about --post_status=any"] = `[{"ID":5,"post_type":"page","post_title":"About","post_status":"publish"}]`
	f.responses["L post get 5 --format=json"] = `{"ID":5,"post_type":"page","post_name":"about","post_title":"About old","post_status":"publish","post_excerpt":"","post_content":"old","post_modified_gmt":"` + modified + `"}`
	f.responses["L post meta list 5"] = `[]`
	f.responses["L post list --post_type=attachment"] = `[{"ID":12}]`
	f.pullFiles["2025/03/a.jpg"] = FileIdentical
	f.responses["L post update 5"] = "Success\n"
}

func TestPullUpdateFlowBacksUpFirst(t *testing.T) {
	f := pullFake()
	localExisting(f, "2025-03-01 10:00:00")
	opts := pullOpts("300")
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Creating() || len(p.Drift) != 0 || len(p.Copies()) != 0 || len(p.Imports()) != 0 || p.InSync {
		t.Fatalf("plan: creating=%v drift=%v downloads=%v imports=%v insync=%v", p.Creating(), p.Drift, p.Copies(), p.Imports(), p.InSync)
	}
	dir := t.TempDir()
	res, err := Apply(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(res.BackupPath)
	if !strings.Contains(string(data), "About old") || !strings.Contains(res.BackupPath, "-about-local5.json") {
		t.Fatalf("backup %s: %s", res.BackupPath, data)
	}
	update := lastCallWithPrefix(f, "L post update 5")
	if !strings.Contains(update, "--post_title=About") || !strings.Contains(f.stdin[update], "wp-image-12") {
		t.Fatalf("update call: %s (stdin %q)", update, f.stdin[update])
	}
	if res.Created || len(f.downloads) != 0 || strings.Contains(strings.Join(f.calls, "\n"), "L media import") {
		t.Fatalf("unexpected create/download/import: %+v %v", res, f.downloads)
	}
}

func TestPullDriftDetected(t *testing.T) {
	f := pullFake()
	localExisting(f, "2025-03-12 10:00:00") // after remote's 2025-03-10
	opts := pullOpts("about")
	opts.LastPull = time.Date(2025, 3, 11, 0, 0, 0, 0, time.UTC)
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Drift) != 2 {
		t.Fatalf("drift: %v", p.Drift)
	}
}

func TestPullAlreadyInSync(t *testing.T) {
	f := pullFake()
	localExisting(f, "2025-03-01 10:00:00")
	f.responses["L post get 5 --format=json"] = `{"ID":5,"post_type":"page","post_name":"about","post_title":"About","post_status":"publish","post_excerpt":"See https://site.localhost/x","post_content":` +
		jsonString(`<!-- wp:image {"id":12} --><img src="https://site.localhost/wp-content/uploads/2025/03/a.jpg" class="wp-image-12"/><!-- /wp:image -->`) + `,"post_modified_gmt":"2025-03-01 10:00:00"}`
	f.responses["L post meta list 5"] = `[{"meta_key":"_thumbnail_id","meta_value":"12"},{"meta_key":"_wp_page_template","meta_value":"wide.php"}]`
	p, err := Prepare(f, pullOpts("about"))
	if err != nil {
		t.Fatal(err)
	}
	if !p.InSync {
		t.Fatal("expected in sync")
	}
}

func TestPullDifferentLocalFileAborts(t *testing.T) {
	f := pullFake()
	f.pullFiles["2025/03/a.jpg"] = FileDiffers
	if _, err := Prepare(f, pullOpts("about")); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("err = %v", err)
	}
}

func TestPullMissingRemoteFileSkipsImport(t *testing.T) {
	f := pullFake()
	f.pullFiles["2025/03/a.jpg"] = FileMissingSource
	p, err := Prepare(f, pullOpts("about"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Copies()) != 0 || len(p.Imports()) != 0 {
		t.Fatalf("downloads %v imports %v", p.Copies(), p.Imports())
	}
	if w := strings.Join(p.Warnings, "\n"); !strings.Contains(w, "doesn't exist on the remote") || !strings.Contains(w, "can't be imported") {
		t.Fatalf("warnings: %s", w)
	}
}

func TestPullMultipleLocalMatches(t *testing.T) {
	f := pullFake()
	f.responses["L post list --post_type=page --name=about --post_status=any"] = `[{"ID":5},{"ID":6}]`
	if _, err := Prepare(f, pullOpts("about")); err == nil || !strings.Contains(err.Error(), "2 local page posts") {
		t.Fatalf("err = %v", err)
	}

	f.responses["L post get 6 --format=json"] = `{"ID":6,"post_type":"page","post_name":"about","post_title":"About","post_status":"draft","post_content":"x","post_modified_gmt":"2025-03-01 10:00:00"}`
	f.responses["L post meta list 6"] = `[]`
	opts := pullOpts("about")
	opts.ChooseTarget = func(remote *Post, c []Candidate) (int, error) {
		if remote.ID != 300 || len(c) != 2 {
			t.Fatalf("ChooseLocal(%+v, %+v)", remote, c)
		}
		return 6, nil
	}
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Target == nil || p.Target.ID != 6 {
		t.Fatalf("local = %+v", p.Target)
	}
}

func TestItemizeStateReceiving(t *testing.T) {
	if s := itemizeState("cd+++++++++ 2025/\n>f+++++++++ 2025/a.jpg\n", "2025/a.jpg", ">"); s != FileMissing {
		t.Fatalf("new: %v", s)
	}
	if s := itemizeState(">fcs........ 2025/a.jpg\n", "2025/a.jpg", ">"); s != FileDiffers {
		t.Fatalf("differs: %v", s)
	}
	if s := itemizeState("<f+++++++++ 2025/a.jpg\n", "2025/a.jpg", ">"); s != FileIdentical {
		t.Fatalf("wrong direction: %v", s)
	}
}

// Remote "about" (300) is Finnish with an English translation (301, also
// "about"); locally, "about" exists only in English (5).
func pullPLLFake() *fakeRunner {
	f := pullFake()
	f.responses["L "+pllDetectKey] = pllLangsJSON
	f.responses["R "+pllDetectKey] = pllLangsJSON
	f.responses[pllKey("R", "post", "300")] = `{"300":{"lang":"fi","tr":{"fi":300,"en":301},"translated":true}}`
	f.responses["R post get 301 --format=json"] = `{"ID":301,"post_type":"page","post_name":"about","post_status":"publish"}`
	f.responses["L post list --post_type=page --name=about --post_status=any"] = `[{"ID":5,"post_type":"page"}]`
	f.responses[pllKey("L", "post", "5")] = `{"5":{"lang":"en","tr":{"en":5},"translated":true}}`
	f.responses["L eval pll_"] = "about\n"
	return f
}

func TestPullPolylangMatchesLanguage(t *testing.T) {
	f := pullPLLFake()
	opts := pullOpts("about")
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Creating() || p.PLL == nil || p.PLL.Lang != "fi" || !p.PLL.SetLang {
		t.Fatalf("expected a Finnish create, got local=%+v pll=%+v", p.Target, p.PLL)
	}
	if strings.Join(p.PLL.NewLink, ",") != "en" {
		t.Fatalf("expected the local English post to be linked: %+v", p.PLL)
	}
	if _, err := Apply(p, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	php := lastCallWithPrefix(f, "L eval pll_set_post_language(50")
	for _, want := range []string{`pll_set_post_language(50, "fi")`, `pll_save_post_translations(array("en" => 5, "fi" => 50))`, "--user=admin"} {
		if !strings.Contains(php, want) {
			t.Fatalf("polylang write lacks %s: %s", want, php)
		}
	}
}

func TestPullPolylangNeedsLanguageLocally(t *testing.T) {
	f := pullPLLFake()
	f.responses["L "+pllDetectKey] = `{"languages":["en"],"default":"en","media":false,"sync":[]}`
	if _, err := Prepare(f, pullOpts("about")); err == nil || !strings.Contains(err.Error(), `"fi" isn't set up in Polylang locally`) {
		t.Fatalf("err = %v", err)
	}
}

// mirror swaps the local and remote sides of a push fake, so the same
// scenario can be pulled with mirrorOpts (which swaps the domains too).
func mirror(f *fakeRunner) *fakeRunner {
	swap := func(k string) string {
		switch {
		case strings.HasPrefix(k, "L "):
			return "R " + k[2:]
		case strings.HasPrefix(k, "R "):
			return "L " + k[2:]
		}
		return k
	}
	m := newFake()
	for k, v := range f.responses {
		m.responses[swap(k)] = v
	}
	for k, v := range f.failing {
		m.failing[swap(k)] = v
	}
	for k, v := range f.files {
		m.pullFiles[k] = v
	}
	m.responses["L core is-installed"] = "" // reachability is only checked locally
	return m
}

func mirrorOpts(ident string) Options {
	o := pullOpts(ident)
	o.LocalDomain, o.RemoteDomain = o.RemoteDomain, o.LocalDomain
	return o
}

func remoteWrites(f *fakeRunner) []string {
	var out []string
	for _, c := range f.calls {
		for _, w := range []string{"R post update", "R post create", "R post meta update", "R media import", "R menu create", "R menu item add", "R menu item delete", "R menu location assign"} {
			if strings.HasPrefix(c, w) {
				out = append(out, c)
			}
		}
	}
	return out
}

func TestPullACFBlockIDsRemapped(t *testing.T) {
	f := mirror(acfFake())
	p, err := Prepare(f, mirrorOpts("about"))
	if err != nil {
		t.Fatal(err)
	}
	want := `<!-- wp:acf/hero {"name":"acf/hero","data":{"title":"Hi 12","_title":"field_t","image":77,"_image":"field_i","gallery":["77","78"],"_gallery":"field_g","link_page":"350","_link_page":"field_p"},"mode":"preview"} /-->`
	if got := p.FinalContent(); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPullACFMissingLinkedPostAborts(t *testing.T) {
	f := mirror(acfFake())
	f.responses["L post list --post_type=page --name=contact --post_status=any"] = `[]`
	_, err := Prepare(f, mirrorOpts("about"))
	if err == nil || !strings.Contains(err.Error(), "missing locally; run: jcore sync --pull 7") || !strings.Contains(err.Error(), "pull these first") {
		t.Fatalf("err = %v", err)
	}
}

func TestPullMetaWithACF(t *testing.T) {
	f := acfFake()
	f.responses["L post meta list 5"] = `[{"meta_key":"hero_image","meta_value":"13"},{"meta_key":"_hero_image","meta_value":"field_i"},{"meta_key":"related","meta_value":["7"]},{"meta_key":"_related","meta_value":"field_r"},{"meta_key":"note","meta_value":"https://site.localhost/x"}]`
	f.responses[acfTypesKey] = `{"field_g":"gallery","field_i":"image","field_p":"post_object","field_r":"relationship","field_t":"text"}`
	f.responses["R post create"] = "Success\n200\n"
	f.responses["R post meta update"] = "Success\n"
	f = mirror(f)
	opts := mirrorOpts("about")
	opts.Meta = true
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, m := range p.Meta {
		got[m.Key] = string(p.finalMeta(m))
	}
	if got["hero_image"] != `"78"` || got["related"] != `["350"]` || got["note"] != `"https://example.com/x"` {
		t.Fatalf("meta: %v", got)
	}
	if _, err := Apply(p, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if call := lastCallWithPrefix(f, "L post meta update 200 hero_image"); f.stdin[call] != `"78"` {
		t.Fatalf("meta write %q: %q", call, f.stdin[call])
	}
	if w := remoteWrites(f); len(w) > 0 {
		t.Fatalf("pull wrote to the remote: %v", w)
	}
}

func TestPullGravityFormsRemapped(t *testing.T) {
	content := `<!-- wp:gravityforms/form {"formId":"3","title":false} /--><p>[gravityform id="3" title="false"]</p>`
	f := mirror(gfFake(content))
	p, err := Prepare(f, mirrorOpts("about"))
	if err != nil {
		t.Fatal(err)
	}
	want := `<!-- wp:gravityforms/form {"formId":"9","title":false} /--><p>[gravityform id="9" title="false"]</p>`
	if got := p.FinalContent(); got != want {
		t.Fatalf("got %s", got)
	}

	f = mirror(gfFake(content))
	f.responses["L"+gfRemoteKey[1:]] = `[{"id":10,"title":"Newsletter","active":true}]`
	if _, err := Prepare(f, mirrorOpts("about")); err == nil || !strings.Contains(err.Error(), `form "Contact" is missing locally`) {
		t.Fatalf("err = %v", err)
	}
}

func TestPullLocalIDTargetsPostWithOtherSlug(t *testing.T) {
	f := pullFake()
	f.responses["L post get 9 --format=json"] = `{"ID":9,"post_type":"page","post_name":"about-us","post_title":"About us","post_status":"publish","post_content":"old","post_modified_gmt":"2025-03-01 10:00:00"}`
	f.responses["L post meta list 9"] = `[]`
	opts := pullOpts("about")
	opts.TargetID = 9
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Target == nil || p.Target.ID != 9 {
		t.Fatalf("target = %+v", p.Target)
	}
	if !strings.Contains(strings.Join(p.Warnings, "\n"), `local post 9 has slug "about-us", not "about"`) {
		t.Fatalf("warnings: %v", p.Warnings)
	}
}

func TestPullMenu(t *testing.T) {
	f := mirror(menuFake())
	p, err := PrepareMenu(f, mirrorOpts(""), "main")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Creating() || len(p.Ops) != 3 || !slices.Equal(p.Locations, []string{"primary"}) {
		t.Fatalf("plan: creating=%v ops=%d locations=%v", p.Creating(), len(p.Ops), p.Locations)
	}
	if p.Ops[0].TargetObjectID != 300 || p.Ops[1].URL != "https://example.com/contact/" {
		t.Fatalf("ops: %+v", p.Ops)
	}
	res, err := ApplyMenu(p, false, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created || res.MenuID != 9 {
		t.Fatalf("result %+v", res)
	}
	if c := callsWithPrefix(f, "L menu item add-post 9 300"); len(c) != 1 {
		t.Fatalf("calls: %v", f.calls)
	}
	if c := callsWithPrefix(f, "L menu location assign 9 primary"); len(c) != 1 {
		t.Fatalf("location not assigned: %v", f.calls)
	}
	if w := remoteWrites(f); len(w) > 0 {
		t.Fatalf("pull wrote to the remote: %v", w)
	}
}

func TestPullMenuUpdateBacksUpLocal(t *testing.T) {
	f := menuFake()
	existingRemoteMenu(f, `["primary"]`)
	f = mirror(f)
	p, err := PrepareMenu(f, mirrorOpts(""), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Removals) != 1 || p.Removals[0].DBID != 201 {
		t.Fatalf("removals: %+v", p.Removals)
	}
	res, err := ApplyMenu(p, true, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(res.BackupPath, "-menu-main-local.json") {
		t.Fatalf("backup: %s", res.BackupPath)
	}
	if c := callsWithPrefix(f, "L menu item delete 201"); len(c) != 1 {
		t.Fatalf("calls: %v", f.calls)
	}
	if w := remoteWrites(f); len(w) > 0 {
		t.Fatalf("pull wrote to the remote: %v", w)
	}
}
