package postsync

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// menuFake extends baseFake with a local menu "Main" (slug main, location
// primary): About (page 5) > Contact (custom link), then News (category 8).
// The remote has page "about" as 300 and category "news" as 44, but no menus.
func menuFake() *fakeRunner {
	f := baseFake()
	f.responses["L menu list"] = `[{"term_id":3,"name":"Main","slug":"main","locations":["primary"]}]`
	f.responses["L menu item list 3"] = `[` +
		`{"db_id":10,"type":"post_type","object":"page","object_id":"5","menu_item_parent":"0","position":1,"title":"About","link":"https://site.localhost/about/","target":"","attr_title":"","description":"","classes":"","xfn":""},` +
		`{"db_id":11,"type":"custom","object":"custom","object_id":"11","menu_item_parent":"10","position":2,"title":"Contact","link":"https://site.localhost/contact/","target":"_blank","attr_title":"","description":"","classes":"btn cta","xfn":""},` +
		`{"db_id":12,"type":"taxonomy","object":"category","object_id":"8","menu_item_parent":"0","position":3,"title":"News","link":"https://site.localhost/category/news/","target":"","attr_title":"","description":"","classes":"","xfn":""}]`
	f.responses["L post list --post_type=nav_menu_item"] = `[{"ID":10,"post_title":""},{"ID":11,"post_title":"Contact"},{"ID":12,"post_title":""}]`
	f.responses["L post meta list 1"] = `[{"meta_key":"_menu_item_type","meta_value":"post_type"}]` // 10, 11, 12
	f.responses["L term get category 8 --field=slug"] = "news\n"

	f.responses["R post list --post_type=page --name=about --post_status=any"] = `[{"ID":300,"post_type":"page"}]`
	f.responses["R term list category --slug=news --field=term_id"] = "44\n"
	f.responses["R menu list"] = `[]`
	f.responses["R menu location list"] = `[{"location":"primary"},{"location":"footer"}]`
	f.responses["R menu create"] = "9\n"
	f.responses["R menu item add-post"] = "100\n"
	f.responses["R menu item add-custom"] = "101\n"
	f.responses["R menu item add-term"] = "102\n"
	f.responses["R menu location assign"] = "Success\n"
	f.responses["R menu item delete"] = "Success\n"
	f.responses["R post update"] = "Success\n"
	f.responses["R post meta update"] = "Success\n"
	return f
}

// existingRemoteMenu gives the remote a "Main" menu (ID 20): About (keep),
// an old custom link (to be removed) and News at the wrong position.
func existingRemoteMenu(f *fakeRunner, locations string) {
	f.responses["R menu list"] = `[{"term_id":20,"name":"Main","slug":"main","locations":` + locations + `}]`
	f.responses["R menu item list 20"] = `[` +
		`{"db_id":200,"type":"post_type","object":"page","object_id":"300","menu_item_parent":"0","position":1,"title":"About","link":"https://example.com/about/","target":"","attr_title":"","description":"","classes":"","xfn":""},` +
		`{"db_id":201,"type":"custom","object":"custom","object_id":"201","menu_item_parent":"0","position":2,"title":"Old","link":"https://example.com/old/","target":"","attr_title":"","description":"","classes":"","xfn":""},` +
		`{"db_id":202,"type":"taxonomy","object":"category","object_id":"44","menu_item_parent":"0","position":5,"title":"News","link":"https://example.com/category/news/","target":"","attr_title":"","description":"","classes":"","xfn":""}]`
	f.responses["R post list --post_type=nav_menu_item"] = `[{"ID":200,"post_title":"","post_modified_gmt":"2025-03-01 10:00:00"},{"ID":201,"post_title":"Old","post_modified_gmt":"2025-03-01 10:00:00"},{"ID":202,"post_title":"","post_modified_gmt":"2025-03-01 10:00:00"}]`
}

func callsWithPrefix(f *fakeRunner, prefix string) []string {
	var out []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

func menuWrites(f *fakeRunner) []string {
	var out []string
	for _, c := range f.calls {
		for _, p := range []string{"R menu create", "R menu item add", "R menu item delete", "R menu location assign", "R post update", "R post meta update"} {
			if strings.HasPrefix(c, p) {
				out = append(out, c)
			}
		}
	}
	return out
}

func TestMenuCreateFlow(t *testing.T) {
	f := menuFake()
	p, err := PrepareMenu(f, testOpts(""), "main")
	if err != nil {
		t.Fatal(err)
	}
	if w := menuWrites(f); len(w) > 0 {
		t.Fatalf("PrepareMenu wrote: %v", w)
	}
	if !p.Creating() || len(p.Ops) != 3 || !slices.Equal(p.Locations, []string{"primary"}) {
		t.Fatalf("plan: creating=%v ops=%d locations=%v", p.Creating(), len(p.Ops), p.Locations)
	}

	res, err := ApplyMenu(f, p, false, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created || res.MenuID != 9 || res.BackupPath != "" {
		t.Fatalf("result %+v", res)
	}
	want := []string{
		"R menu create Main --porcelain --user=admin",
		"R menu item add-post 9 300 --position=1 --porcelain --user=admin",
		"R menu item add-custom 9 Contact https://example.com/contact/ --position=2 --parent-id=100 --target=_blank --classes=btn cta --porcelain --user=admin",
		"R menu item add-term 9 category 44 --position=3 --porcelain --user=admin",
		"R menu location assign 9 primary --user=admin",
	}
	if got := menuWrites(f); !slices.Equal(got, want) {
		t.Fatalf("writes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestMenuUpdateFlow(t *testing.T) {
	f := menuFake()
	existingRemoteMenu(f, `[]`)
	p, err := PrepareMenu(f, testOpts(""), "Main")
	if err != nil {
		t.Fatal(err)
	}
	if p.Creating() || p.InSync {
		t.Fatal("expected an update")
	}
	if p.Ops[0].RemoteID != 200 || len(p.Ops[0].Changes) != 0 {
		t.Errorf("About: %+v", p.Ops[0])
	}
	if p.Ops[1].RemoteID != 0 {
		t.Errorf("Contact should be added: %+v", p.Ops[1])
	}
	if p.Ops[2].RemoteID != 202 || !slices.Equal(p.Ops[2].Changes, []string{"position"}) {
		t.Errorf("News: %+v", p.Ops[2])
	}
	if len(p.Removals) != 1 || p.Removals[0].DBID != 201 {
		t.Fatalf("removals: %+v", p.Removals)
	}

	res, err := ApplyMenu(f, p, true, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(res.BackupPath)
	if err != nil || !strings.Contains(string(data), `"raw_title": "Old"`) {
		t.Fatalf("backup: %s %v", data, err)
	}
	want := []string{
		"R menu item add-custom 20 Contact https://example.com/contact/ --position=2 --parent-id=200 --target=_blank --classes=btn cta --porcelain --user=admin",
		"R post update 202 --menu_order=3 --user=admin",
		"R menu item delete 201 --user=admin",
		"R menu location assign 20 primary --user=admin",
	}
	if got := menuWrites(f); !slices.Equal(got, want) {
		t.Fatalf("writes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestMenuRemovalDeclinedKeepsItems(t *testing.T) {
	f := menuFake()
	existingRemoteMenu(f, `[]`)
	p, err := PrepareMenu(f, testOpts(""), "main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyMenu(f, p, false, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if d := callsWithPrefix(f, "R menu item delete"); len(d) > 0 {
		t.Fatalf("deleted despite no confirmation: %v", d)
	}
}

func TestMenuMissingPageAborts(t *testing.T) {
	f := menuFake()
	f.responses["R post list --post_type=page --name=about --post_status=any"] = `[]`
	_, err := PrepareMenu(f, testOpts(""), "main")
	if err == nil || !strings.Contains(err.Error(), "jcore sync 5") {
		t.Fatalf("err = %v", err)
	}
	if w := menuWrites(f); len(w) > 0 {
		t.Fatalf("wrote: %v", w)
	}
}

func TestMenuAlreadyInSync(t *testing.T) {
	f := menuFake()
	existingRemoteMenu(f, `["primary"]`)
	f.responses["R menu item list 20"] = `[` +
		`{"db_id":200,"type":"post_type","object":"page","object_id":"300","menu_item_parent":"0","position":1,"title":"About","link":"","target":"","classes":""},` +
		`{"db_id":201,"type":"custom","object":"custom","object_id":"201","menu_item_parent":"200","position":2,"title":"Contact","link":"https://example.com/contact/","target":"_blank","classes":["btn","cta"]},` +
		`{"db_id":202,"type":"taxonomy","object":"category","object_id":"44","menu_item_parent":"0","position":3,"title":"News","link":"","target":"","classes":""}]`
	f.responses["R post list --post_type=nav_menu_item"] = `[{"ID":200,"post_title":""},{"ID":201,"post_title":"Contact"},{"ID":202,"post_title":""}]`
	p, err := PrepareMenu(f, testOpts(""), "main")
	if err != nil {
		t.Fatal(err)
	}
	if !p.InSync {
		t.Fatalf("expected in sync: ops=%+v removals=%v locations=%v", p.Ops, p.Removals, p.Locations)
	}
}

func TestMenuOccupiedLocationLeftAlone(t *testing.T) {
	f := menuFake()
	f.responses["R menu list"] = `[{"term_id":50,"name":"Footer","slug":"footer","locations":["primary"]}]`
	p, err := PrepareMenu(f, testOpts(""), "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Locations) != 0 || !strings.Contains(strings.Join(p.Warnings, "\n"), `already shows menu "Footer"`) {
		t.Fatalf("locations=%v warnings=%v", p.Locations, p.Warnings)
	}
}

func TestMenuDrift(t *testing.T) {
	f := menuFake()
	existingRemoteMenu(f, `[]`)
	opts := testOpts("")
	opts.LastPull = time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	p, err := PrepareMenu(f, opts, "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Drift) != 3 {
		t.Fatalf("drift: %v", p.Drift)
	}
}

func TestMenuNotFoundListsMenus(t *testing.T) {
	f := menuFake()
	_, err := PrepareMenu(f, testOpts(""), "nope")
	if err == nil || !strings.Contains(err.Error(), "3  Main (main)") {
		t.Fatalf("err = %v", err)
	}
}
