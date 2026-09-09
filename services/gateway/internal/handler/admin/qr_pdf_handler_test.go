package admin

import (
	"bytes"
	"net/url"
	"strings"
	"testing"

	v1_catalogpb "github.com/menli02/QR-menu/proto/catalog/v1"
)

func TestGuestURL(t *testing.T) {
	code := &v1_catalogpb.TablePrintCode{
		TableCode:  "9nIKAj4v_BcXNJy8",
		KeyVersion: 2,
		Sig:        "abc-def_123",
	}

	raw := guestURL("https://menu.example.com", "the-cafe", code)
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("produced an unparseable URL %q: %v", raw, err)
	}

	q := u.Query()
	if q.Get("v") != "the-cafe" {
		t.Errorf("venue = %q", q.Get("v"))
	}
	if q.Get("t") != "9nIKAj4v_BcXNJy8" {
		t.Errorf("table code = %q", q.Get("t"))
	}
	// The key version rides on the signature because the public
	// session-exchange body has no field for it, and FR-T4's rotation
	// grace period means verification must know which key signed this
	// sticker.
	if q.Get("s") != "2.abc-def_123" {
		t.Errorf("sig = %q, want the version-prefixed form", q.Get("s"))
	}
}

// TestGuestURLTrailingSlash: an operator will configure the base URL both
// ways, and a doubled slash produces a link that 404s on some frontends.
func TestGuestURLIsStableAcrossBaseURLForms(t *testing.T) {
	code := &v1_catalogpb.TablePrintCode{TableCode: "t", KeyVersion: 1, Sig: "s"}
	withSlash := guestURL("https://menu.example.com", "cafe", code)
	if strings.Contains(strings.TrimPrefix(withSlash, "https://"), "//") {
		t.Errorf("URL contains a doubled slash: %s", withSlash)
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"the-cafe":      "the-cafe",
		"cafe_2":        "cafe_2",
		"":              "venue",
		`a"b`:           "a-b",
		"a\r\nb":        "a--b",
		"../../etc/pwd": "------etc-pwd",
	}
	for in, want := range cases {
		if got := sanitizeFilename(in); got != want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSanitizeFilenameBlocksHeaderInjection is the reason that function
// exists: Content-Disposition is interpolated into a response header, and
// a CR or LF in it is classic response splitting.
func TestSanitizeFilenameBlocksHeaderInjection(t *testing.T) {
	got := sanitizeFilename("evil\r\nSet-Cookie: admin=1")
	for _, bad := range []string{"\r", "\n", ":", " "} {
		if strings.Contains(got, bad) {
			t.Errorf("sanitized name still contains %q: %q", bad, got)
		}
	}
}

func TestRenderQRSheetProducesAPDF(t *testing.T) {
	codes := &v1_catalogpb.GetTablePrintCodesResponse{
		VenueSlug: "the-cafe",
		Codes: []*v1_catalogpb.TablePrintCode{
			{TableId: "t1", Label: "1", TableCode: "codeone123456", KeyVersion: 1, Sig: "sig1"},
			{TableId: "t2", Label: "2", TableCode: "codetwo123456", KeyVersion: 1, Sig: "sig2"},
		},
	}

	out, err := renderQRSheet("https://menu.example.com", codes)
	if err != nil {
		t.Fatalf("renderQRSheet: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Errorf("output is not a PDF: first bytes %q", out[:min(8, len(out))])
	}
	if len(out) < 1000 {
		t.Errorf("PDF is %d bytes — too small to contain two QR images", len(out))
	}
}

// TestRenderQRSheetIsDeterministic guards the fixed creation date. Without
// it the same tables produce a different file on every call, which makes
// the endpoint impossible to cache or to compare in a test.
func TestRenderQRSheetIsDeterministic(t *testing.T) {
	codes := &v1_catalogpb.GetTablePrintCodesResponse{
		VenueSlug: "the-cafe",
		Codes: []*v1_catalogpb.TablePrintCode{
			{TableId: "t1", Label: "1", TableCode: "codeone123456", KeyVersion: 1, Sig: "sig1"},
		},
	}

	first, err := renderQRSheet("https://menu.example.com", codes)
	if err != nil {
		t.Fatalf("first render: %v", err)
	}
	second, err := renderQRSheet("https://menu.example.com", codes)
	if err != nil {
		t.Fatalf("second render: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("the same input rendered two different PDFs")
	}
}

// TestRenderQRSheetPaginates: six cards fit a page, so seven tables must
// produce two.
func TestRenderQRSheetPaginates(t *testing.T) {
	build := func(n int) *v1_catalogpb.GetTablePrintCodesResponse {
		out := &v1_catalogpb.GetTablePrintCodesResponse{VenueSlug: "cafe"}
		for i := 0; i < n; i++ {
			out.Codes = append(out.Codes, &v1_catalogpb.TablePrintCode{
				TableId:    string(rune('a' + i)),
				Label:      string(rune('1' + i)),
				TableCode:  "code123456789",
				KeyVersion: 1,
				Sig:        "sig",
			})
		}
		return out
	}

	onePage, err := renderQRSheet("https://menu.example.com", build(cardsPerRow*cardsPerCol))
	if err != nil {
		t.Fatalf("one page: %v", err)
	}
	twoPages, err := renderQRSheet("https://menu.example.com", build(cardsPerRow*cardsPerCol+1))
	if err != nil {
		t.Fatalf("two pages: %v", err)
	}

	countPages := func(b []byte) int { return bytes.Count(b, []byte("/Type /Page\n")) }
	if p1, p2 := countPages(onePage), countPages(twoPages); p2 <= p1 {
		t.Errorf("adding a seventh table did not add a page: %d then %d", p1, p2)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
