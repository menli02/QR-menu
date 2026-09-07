package venue

import (
	"testing"
	"time"

	"github.com/menli02/QR-menu/proto/catalog/v1"
)

func TestBusinessDateUsesVenueLocalCalendarDay(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo") // UTC+9, no DST
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	losAngeles, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}

	// 2026-03-01T23:30Z is already 2026-03-02 in Tokyo and still
	// 2026-03-01 in Los Angeles. The same instant therefore belongs to
	// two different business days depending on the venue — which is the
	// entire point of storing the date rather than the timestamp.
	instant := time.Date(2026, 3, 1, 23, 30, 0, 0, time.UTC)

	cases := []struct {
		name string
		loc  *time.Location
		want string
	}{
		{"utc", time.UTC, "2026-03-01"},
		{"tokyo is already tomorrow", tokyo, "2026-03-02"},
		{"los angeles is still today", losAngeles, "2026-03-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Settings{Location: tc.loc}
			got := s.BusinessDate(instant)
			if got.Format(time.DateOnly) != tc.want {
				t.Errorf("BusinessDate = %s, want %s", got.Format(time.DateOnly), tc.want)
			}
			// The result must be a bare UTC midnight so it compares equal
			// to a DATE column and to ParseBusinessDate's output.
			if h, m, sec := got.Clock(); h != 0 || m != 0 || sec != 0 {
				t.Errorf("BusinessDate carried a time-of-day: %s", got)
			}
			if got.Location() != time.UTC {
				t.Errorf("BusinessDate location = %s, want UTC", got.Location())
			}
		})
	}
}

// TestBusinessDateRoundTripsThroughParse is the property GetDayReport
// depends on: "today" computed from an instant and "today" parsed from
// the client's YYYY-MM-DD must be the same value, or a report for the
// current day silently returns nothing.
func TestBusinessDateRoundTripsThroughParse(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	s := &Settings{Location: tokyo}

	derived := s.BusinessDate(time.Date(2026, 3, 1, 23, 30, 0, 0, time.UTC))
	parsed, err := ParseBusinessDate(derived.Format(time.DateOnly))
	if err != nil {
		t.Fatalf("ParseBusinessDate: %v", err)
	}
	if !derived.Equal(parsed) {
		t.Errorf("round trip changed the value: %s -> %s", derived, parsed)
	}
}

func TestParseBusinessDateRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "2026-3-1", "01/03/2026", "2026-03-01T00:00:00Z", "yesterday"} {
		if _, err := ParseBusinessDate(in); err == nil {
			t.Errorf("ParseBusinessDate(%q) succeeded, want an error", in)
		}
	}
}

func TestFromProtoAppliesDefaults(t *testing.T) {
	// A venue that has never touched its settings sends zeros; those must
	// become the documented defaults, not a zero-length comment limit and
	// a zero-second cancel window.
	got := fromProto(&v1_catalogpb.VenueSettings{VenueId: "v1", Currency: "EUR"})

	if got.CommentMaxLen != DefaultCommentMaxLen {
		t.Errorf("CommentMaxLen = %d, want %d", got.CommentMaxLen, DefaultCommentMaxLen)
	}
	if got.CancelWindow != DefaultCancelWindow {
		t.Errorf("CancelWindow = %s, want %s", got.CancelWindow, DefaultCancelWindow)
	}
	if got.Location != time.UTC {
		t.Errorf("Location = %v, want UTC for an empty timezone", got.Location)
	}
	if got.Currency != "EUR" {
		t.Errorf("Currency = %q, want EUR", got.Currency)
	}
}

func TestFromProtoKeepsExplicitValues(t *testing.T) {
	got := fromProto(&v1_catalogpb.VenueSettings{
		VenueId:                "v1",
		Currency:               "JPY",
		DefaultLocale:          "ja",
		Timezone:               "Asia/Tokyo",
		ServiceChargeBps:       1000,
		OrderItemCommentMaxLen: 50,
		OrderTotalLimitMinor:   1_000_000,
		CancelWindowSeconds:    120,
	})

	if got.CommentMaxLen != 50 {
		t.Errorf("CommentMaxLen = %d, want 50", got.CommentMaxLen)
	}
	if got.CancelWindow != 2*time.Minute {
		t.Errorf("CancelWindow = %s, want 2m", got.CancelWindow)
	}
	if got.ServiceChargeBps != 1000 {
		t.Errorf("ServiceChargeBps = %d, want 1000", got.ServiceChargeBps)
	}
	if got.OrderTotalLimitMinor != 1_000_000 {
		t.Errorf("OrderTotalLimitMinor = %d, want 1000000", got.OrderTotalLimitMinor)
	}
	if got.Location.String() != "Asia/Tokyo" {
		t.Errorf("Location = %s, want Asia/Tokyo", got.Location)
	}
}

// TestLoadLocationFallsBackToUTC covers the deliberate choice not to fail
// an order over a misconfigured timezone.
func TestLoadLocationFallsBackToUTC(t *testing.T) {
	if got := loadLocation("Mars/Olympus_Mons"); got != time.UTC {
		t.Errorf("loadLocation(bogus) = %v, want UTC", got)
	}
	if got := loadLocation(""); got != time.UTC {
		t.Errorf("loadLocation(empty) = %v, want UTC", got)
	}
}
