package postsync

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeRunner answers wp-cli calls from canned responses keyed by command
// prefix ("L " local / "R " remote + space-joined args) and records every
// call, so tests can assert which commands ran.
type fakeRunner struct {
	responses map[string]string // prefix → stdout
	failing   map[string]bool   // prefix → returns an error
	files     map[string]FileState
	calls     []string
	uploads   []string
	stdin     map[string]string // call → stdin
}

func newFake() *fakeRunner {
	return &fakeRunner{responses: map[string]string{}, failing: map[string]bool{}, files: map[string]FileState{}, stdin: map[string]string{}}
}

func (f *fakeRunner) answer(call string) (string, error) {
	f.calls = append(f.calls, call)
	best := ""
	for prefix := range f.failing {
		if strings.HasPrefix(call, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	for prefix := range f.responses {
		if strings.HasPrefix(call, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	if best == "" || f.failing[best] {
		return "", errors.New("fake: no response for " + call)
	}
	return f.responses[best], nil
}

func (f *fakeRunner) Local(args ...string) (string, error) {
	return f.answer("L " + strings.Join(args, " "))
}
func (f *fakeRunner) Remote(args ...string) (string, error) {
	return f.answer("R " + strings.Join(args, " "))
}
func (f *fakeRunner) RemoteStdin(stdin string, args ...string) (string, error) {
	call := "R " + strings.Join(args, " ")
	f.stdin[call] = stdin
	return f.answer(call)
}
func (f *fakeRunner) FileStatus(rel string) (FileState, error) {
	if s, ok := f.files[rel]; ok {
		return s, nil
	}
	return FileMissing, nil
}
func (f *fakeRunner) Upload(rel string) error {
	f.uploads = append(f.uploads, rel)
	return nil
}

func (f *fakeRunner) writeCalls() []string {
	var out []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, "R post update") || strings.HasPrefix(c, "R post create") ||
			strings.HasPrefix(c, "R media import") || strings.HasPrefix(c, "R post meta update") {
			out = append(out, c)
		}
	}
	return out
}

const testContent = `<!-- wp:image {"id":12} --><img src="https://site.localhost/wp-content/uploads/2025/03/a.jpg" class="wp-image-12"/><!-- /wp:image -->`

// baseFake is a local site with page 5 ("about") using attachment 12, and a
// remote that's reachable and single-site.
func baseFake() *fakeRunner {
	f := newFake()
	f.responses["L core is-installed"] = ""
	f.failing["L core is-installed --network"] = true
	f.responses["R option get siteurl"] = "https://example.com\n"
	f.failing["R core is-installed --network"] = true
	f.responses["L post get 5 --format=json"] = `{"ID":5,"post_type":"page","post_name":"about","post_title":"About","post_status":"publish","post_excerpt":"","post_content":` + jsonString(testContent) + `,"post_parent":0,"post_author":"1","post_modified_gmt":"2025-03-10 10:00:00"}`
	f.responses["L post meta list 5"] = `[{"meta_key":"_wp_page_template","meta_value":"default"}]`
	f.responses["L post list --post_type=any --name=about"] = `[{"ID":5,"post_type":"page","post_title":"About","post_status":"publish"}]`
	f.responses["R post-type get page"] = "page"
	f.responses["R post list --post_type=page --name=about --post_status=any"] = `[]`
	f.responses["R post list --post_type=page --name=about --post_status=trash"] = `[]`
	f.responses["R user list --role=administrator"] = "admin\n"
	f.responses["L user get 1 --field=user_login"] = "linus\n"
	f.failing["R user get linus"] = true
	f.responses["L post get 12 --format=json"] = `{"ID":12,"post_type":"attachment","post_name":"a","post_title":"A photo","post_status":"inherit","post_excerpt":"cap","post_content":""}`
	f.responses["L post meta list 12"] = `[{"meta_key":"_wp_attached_file","meta_value":"2025/03/a.jpg"},{"meta_key":"_wp_attachment_image_alt","meta_value":"alt text"}]`
	f.responses["R post list --post_type=attachment"] = `[]`
	f.responses["R media import"] = "77\n"
	f.responses["R post create"] = "Success\n200\n"
	f.responses["R post get 200 --field=post_content"] = `<!-- wp:image {"id":77} --><img src="https://example.com/wp-content/uploads/2025/03/a.jpg" class="wp-image-77"/><!-- /wp:image -->` + "\n"
	return f
}

func jsonString(s string) string {
	b := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s)
	return `"` + b + `"`
}

func testOpts(ident string) Options {
	return Options{Ident: ident, LocalDomain: "site.localhost", RemoteDomain: "example.com", RemotePath: "/sites/example"}
}

func TestPrepareMakesNoWrites(t *testing.T) {
	f := baseFake()
	p, err := Prepare(f, testOpts("about"))
	if err != nil {
		t.Fatal(err)
	}
	if !p.Creating() {
		t.Fatal("expected create")
	}
	if w := f.writeCalls(); len(w) > 0 || len(f.uploads) > 0 {
		t.Fatalf("Prepare wrote: %v uploads %v", w, f.uploads)
	}
	if len(p.Imports()) != 1 || len(p.Uploads()) != 1 {
		t.Fatalf("imports %v uploads %v", p.Imports(), p.Uploads())
	}
}

func TestCreateFlow(t *testing.T) {
	f := baseFake()
	opts := testOpts("5")
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Apply(f, p, opts, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created || res.RemoteID != 200 || res.BackupPath != "" {
		t.Fatalf("result %+v", res)
	}
	if len(f.uploads) != 1 || f.uploads[0] != "2025/03/a.jpg" {
		t.Fatalf("uploads %v", f.uploads)
	}
	var create string
	for _, c := range f.calls {
		if strings.HasPrefix(c, "R post create") {
			create = c
		}
	}
	if !strings.Contains(create, "--post_status=draft") || !strings.Contains(create, "--user=admin") || strings.Contains(create, "--post_author") {
		t.Fatalf("create call: %s", create)
	}
	sent := f.stdin[create]
	if strings.Contains(sent, "localhost") || !strings.Contains(sent, `"id":77`) || !strings.Contains(sent, "wp-image-77") {
		t.Fatalf("content sent: %s", sent)
	}
	if res.VerifyMismatch {
		t.Fatal("unexpected verify mismatch")
	}
}

func remoteExisting(f *fakeRunner, modified string) {
	f.responses["R post list --post_type=page --name=about --post_status=any"] = `[{"ID":300,"post_type":"page","post_title":"About","post_status":"publish"}]`
	f.responses["R post get 300 --format=json"] = `{"ID":300,"post_type":"page","post_name":"about","post_title":"About old","post_status":"publish","post_excerpt":"","post_content":"old","post_modified_gmt":"` + modified + `"}`
	f.responses["R post meta list 300"] = `[]`
	f.responses["R post list --post_type=attachment"] = `[{"ID":77}]`
	f.files["2025/03/a.jpg"] = FileIdentical
	f.responses["R post update 300"] = "Success\n"
}

func TestUpdateFlowBacksUpFirst(t *testing.T) {
	f := baseFake()
	remoteExisting(f, "2025-03-01 10:00:00")
	opts := testOpts("about")
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Creating() || len(p.Drift) != 0 || len(p.Uploads()) != 0 || len(p.Imports()) != 0 {
		t.Fatalf("plan: creating=%v drift=%v uploads=%v imports=%v", p.Creating(), p.Drift, p.Uploads(), p.Imports())
	}
	dir := t.TempDir()
	res, err := Apply(f, p, opts, dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.BackupPath == "" {
		t.Fatal("no backup")
	}
	data, _ := os.ReadFile(res.BackupPath)
	if !strings.Contains(string(data), "About old") || filepath.Dir(res.BackupPath) != dir {
		t.Fatalf("backup: %s", data)
	}
	if res.Created || len(f.uploads) != 0 {
		t.Fatalf("unexpected create/upload: %+v %v", res, f.uploads)
	}
	// The read-back isn't stubbed for 300, so the mismatch must be flagged.
	if !res.VerifyMismatch {
		t.Fatal("expected verify mismatch to be reported")
	}
}

func TestDriftDetected(t *testing.T) {
	f := baseFake()
	remoteExisting(f, "2025-03-12 10:00:00") // after local's 2025-03-10
	opts := testOpts("about")
	opts.LastPull = time.Date(2025, 3, 11, 0, 0, 0, 0, time.UTC)
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Drift) != 2 {
		t.Fatalf("drift: %v", p.Drift)
	}
}

func TestAlreadyInSync(t *testing.T) {
	f := baseFake()
	remoteExisting(f, "2025-03-01 10:00:00")
	f.responses["R post get 300 --format=json"] = `{"ID":300,"post_type":"page","post_name":"about","post_title":"About","post_status":"publish","post_excerpt":"","post_content":` +
		jsonString(`<!-- wp:image {"id":77} --><img src="https://example.com/wp-content/uploads/2025/03/a.jpg" class="wp-image-77"/><!-- /wp:image -->`) + `,"post_modified_gmt":"2025-03-01 10:00:00"}`
	p, err := Prepare(f, testOpts("about"))
	if err != nil {
		t.Fatal(err)
	}
	if !p.InSync {
		t.Fatal("expected in sync")
	}
}

func TestMultipleRemoteMatchesAbort(t *testing.T) {
	f := baseFake()
	f.responses["R post list --post_type=page --name=about --post_status=any"] = `[{"ID":1},{"ID":2}]`
	if _, err := Prepare(f, testOpts("about")); err == nil || !strings.Contains(err.Error(), "refusing to guess") {
		t.Fatalf("err = %v", err)
	}
}

func TestDifferentRemoteFileAborts(t *testing.T) {
	f := baseFake()
	f.files["2025/03/a.jpg"] = FileDiffers
	if _, err := Prepare(f, testOpts("about")); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("err = %v", err)
	}
	if len(f.uploads) != 0 {
		t.Fatal("uploaded despite conflict")
	}
}

func TestAmbiguousLocalSlug(t *testing.T) {
	f := baseFake()
	f.responses["L post list --post_type=any --name=about"] = `[{"ID":5,"post_type":"page"},{"ID":6,"post_type":"post"}]`
	if _, err := Prepare(f, testOpts("about")); err == nil || !strings.Contains(err.Error(), "matches 2 local posts") {
		t.Fatalf("err = %v", err)
	}
}

func TestMultisiteRefused(t *testing.T) {
	f := baseFake()
	delete(f.failing, "R core is-installed --network")
	f.responses["R core is-installed --network"] = ""
	if _, err := Prepare(f, testOpts("about")); err == nil || !strings.Contains(err.Error(), "multisite") {
		t.Fatalf("err = %v", err)
	}
}

func TestMetaOptIn(t *testing.T) {
	f := baseFake()
	f.responses["L post meta list 5"] = `[{"meta_key":"hero_title","meta_value":"See https://site.localhost/x"},{"meta_key":"hero_image","meta_value":"12"},{"meta_key":"_edit_lock","meta_value":"1:1"},{"meta_key":"dup","meta_value":"a"},{"meta_key":"dup","meta_value":"b"},{"meta_key":"list","meta_value":{"a":["https://site.localhost/y"]}}]`
	opts := testOpts("about")
	opts.Meta = true
	p, err := Prepare(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, m := range p.Meta {
		keys = append(keys, m.Key+"="+string(m.Value))
	}
	got := strings.Join(keys, " ")
	want := `hero_title="See https://example.com/x" hero_image="12" list={"a":["https://example.com/y"]}`
	if got != want {
		t.Fatalf("meta = %s", got)
	}
	warn := strings.Join(p.Warnings, "\n")
	if !strings.Contains(warn, `"dup" has multiple values`) || !strings.Contains(warn, "hero_image") {
		t.Fatalf("warnings: %s", warn)
	}
}
