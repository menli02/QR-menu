package guest

import (
	"testing"
)

// TestMenuVersionFromETag covers the parsing that decides whether a
// conditional request becomes a cheap 304 or a full menu build. Getting it
// wrong is invisible — the endpoint still works, it just never hits the
// cache — so it is worth pinning.
func TestMenuVersionFromETag(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
	}{
		{"absent", "", ""},
		{"weak validator, as we emit it", `W/"7"`, "7"},
		{"strong validator from a proxy that rewrote ours", `"7"`, "7"},
		// A client echoing the body's `menuVersion` instead of the header
		// is doing something reasonable; there is no reason to make that a
		// guaranteed cache miss.
		{"bare version", "7", "7"},
		// `*` means "if any representation exists", which this route has
		// no useful answer to. Serve a full response.
		{"wildcard", "*", ""},
		// Several validators: picking one arbitrarily could serve a 304
		// for a version the client does not actually hold.
		{"multiple validators", `W/"7", W/"8"`, ""},
		{"multiple bare", "7,8", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := menuVersionFromETag(tc.header); got != tc.want {
				t.Errorf("menuVersionFromETag(%q) = %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

// TestETagRoundTrip is the property the whole scheme rests on: what we emit
// must be what we can parse back. If these two ever disagree, every
// conditional request silently misses and nobody notices except the bill.
func TestETagRoundTrip(t *testing.T) {
	for _, version := range []string{"1", "7", "12345", "999999"} {
		etag := etagForMenu(version)
		if got := menuVersionFromETag(etag); got != version {
			t.Errorf("round trip lost the version: %q -> %q -> %q", version, etag, got)
		}
	}
}

func TestETagIsAWeakValidator(t *testing.T) {
	// Weak on purpose: the same menu_version renders differently per
	// locale, so the responses are semantically equivalent but not
	// byte-identical. Claiming a strong validator would be untrue.
	if got := etagForMenu("7"); got != `W/"7"` {
		t.Errorf("etagForMenu = %q, want a weak validator", got)
	}
	// An unset version must not produce `W/""`, which a client would
	// happily cache and send back forever.
	if got := etagForMenu(""); got != "" {
		t.Errorf("etagForMenu(\"\") = %q, want empty", got)
	}
}
