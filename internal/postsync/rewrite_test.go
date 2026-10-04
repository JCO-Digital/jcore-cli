package postsync

import (
	"reflect"
	"testing"
)

func TestCollectAttachmentIDs(t *testing.T) {
	content := `<!-- wp:image {"id":12,"sizeSlug":"large"} -->
<figure class="wp-block-image"><img src="x.jpg" class="wp-image-12"/></figure>
<!-- /wp:image -->
<!-- wp:core/cover {"url":"y.jpg","id":7} -->x<!-- /wp:core/cover -->
<!-- wp:media-text {"mediaId":30,"mediaType":"image"} -->m<!-- /wp:media-text -->
<!-- wp:gallery {"ids":[40,41]} /-->
<!-- wp:paragraph {"id":99} --><p class="wp-image-5">p</p><!-- /wp:paragraph -->
<!-- wp:acme/thing {"id":123} /-->`
	got := CollectAttachmentIDs(content)
	want := []int{5, 7, 12, 30, 40, 41}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRemapAttachmentIDs(t *testing.T) {
	content := `<!-- wp:image {"id":12,"sizeSlug":"large"} -->
<img class="wp-image-12 size-large"/>
<!-- /wp:image -->
<!-- wp:media-text {"mediaId":34} --><img class="wp-image-34"/><!-- /wp:media-text -->
<!-- wp:gallery {"ids":[12, 34,99]} /-->
<!-- wp:paragraph {"id":12} --><p>wp-image-120</p><!-- /wp:paragraph -->`
	got := RemapAttachmentIDs(content, map[int]int{12: 34, 34: 56})
	want := `<!-- wp:image {"id":34,"sizeSlug":"large"} -->
<img class="wp-image-34 size-large"/>
<!-- /wp:image -->
<!-- wp:media-text {"mediaId":56} --><img class="wp-image-56"/><!-- /wp:media-text -->
<!-- wp:gallery {"ids":[34, 56,99]} /-->
<!-- wp:paragraph {"id":12} --><p>wp-image-120</p><!-- /wp:paragraph -->`
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemapAttachmentIDs_NoChainAndUnmappedUntouched(t *testing.T) {
	in := `<!-- wp:image {"id":1} --><img class="wp-image-1"/><!-- /wp:image --><!-- wp:image {"id":3} /-->`
	got := RemapAttachmentIDs(in, map[int]int{1: 2, 2: 9})
	want := `<!-- wp:image {"id":2} --><img class="wp-image-2"/><!-- /wp:image --><!-- wp:image {"id":3} /-->`
	if got != want {
		t.Fatalf("got %s", got)
	}
}

func TestReplaceDomain(t *testing.T) {
	cases := map[string]string{
		`<a href="https://site.localhost/about/">`:          `<a href="https://example.com/about/">`,
		`{"url":"https:\/\/site.localhost\/wp-content\/x"}`: `{"url":"https:\/\/example.com\/wp-content\/x"}`,
		`https://site.localhost`:                            `https://example.com`,
		`https://site.localhost.other.com/x`:                `https://site.localhost.other.com/x`,
		`https://other-site.localhost/x`:                    `https://other-site.localhost/x`,
		`mail me at site.localhost`:                         `mail me at site.localhost`,
	}
	for in, want := range cases {
		if got := ReplaceDomain(in, "site.localhost", "example.com"); got != want {
			t.Errorf("ReplaceDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCollectLocalRefs(t *testing.T) {
	content := `<img src="https://site.localhost/wp-content/uploads/2025/03/a-1024x768.jpg" srcset="https://site.localhost/wp-content/uploads/2025/03/a.jpg 2000w, https://site.localhost/wp-content/uploads/2025/03/a-300x200.jpg 300w"/>
{"url":"https:\/\/site.localhost\/wp-content\/uploads\/2025\/03\/my%20file.pdf"}
<a href="https://site.localhost/contact/?x=1">c</a> <a href="https://site.localhost">home</a>
<a href="https://cdn.example.com/wp-content/uploads/z.jpg">ext</a> https://site.localhost.evil.com/wp-content/uploads/no.jpg`
	got := CollectLocalRefs(content, "site.localhost")
	wantU := []string{"2025/03/a-1024x768.jpg", "2025/03/a.jpg", "2025/03/a-300x200.jpg", "2025/03/my file.pdf"}
	wantL := []string{"/contact/", "/"}
	if !reflect.DeepEqual(got.Uploads, wantU) {
		t.Errorf("uploads = %v, want %v", got.Uploads, wantU)
	}
	if !reflect.DeepEqual(got.Links, wantL) {
		t.Errorf("links = %v, want %v", got.Links, wantL)
	}
}

func TestBaseUpload(t *testing.T) {
	cases := map[string]string{
		"2025/03/a-1024x768.jpg":       "2025/03/a.jpg",
		"2025/03/a-scaled.jpg":         "2025/03/a.jpg",
		"2025/03/a-scaled-300x200.jpg": "2025/03/a.jpg",
		"2025/03/a.jpg":                "2025/03/a.jpg",
	}
	for in, want := range cases {
		if got := baseUpload(in); got != want {
			t.Errorf("baseUpload(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitSSHHost(t *testing.T) {
	cases := []struct{ in, host, port string }{
		{"user@host.com", "user@host.com", ""},
		{"user@host.com:2222", "user@host.com", "2222"},
		{"[2001:db8::1]:22", "[2001:db8::1]", "22"},
		{"host", "host", ""},
	}
	for _, c := range cases {
		h, p := SplitSSHHost(c.in)
		if h != c.host || p != c.port {
			t.Errorf("SplitSSHHost(%q) = %q, %q", c.in, h, p)
		}
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote(`it's $HOME`); got != `'it'\''s $HOME'` {
		t.Fatalf("got %s", got)
	}
}

func TestParseItemize(t *testing.T) {
	if s := parseItemize("cd+++++++++ 2025/\n<f+++++++++ 2025/a.jpg\n", "2025/a.jpg"); s != FileMissing {
		t.Errorf("new file: %v", s)
	}
	if s := parseItemize("<fcs........ 2025/a.jpg\n", "2025/a.jpg"); s != FileDiffers {
		t.Errorf("differs: %v", s)
	}
	if s := parseItemize(".f..t...... 2025/a.jpg\n", "2025/a.jpg"); s != FileIdentical {
		t.Errorf("attr only: %v", s)
	}
	if s := parseItemize("", "2025/a.jpg"); s != FileIdentical {
		t.Errorf("no output: %v", s)
	}
}

func TestReplaceDomainJSON(t *testing.T) {
	got := ReplaceDomainJSON(`{"a":["https://site.localhost/x",3],"b":"plain"}`, "site.localhost", "example.com")
	if got != `{"a":["https://example.com/x",3],"b":"plain"}` {
		t.Fatalf("got %s", got)
	}
}
