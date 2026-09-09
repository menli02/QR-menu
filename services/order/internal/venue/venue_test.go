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

// TestBusinessDateHonoursTheCutoff is FR-A2: a venue that serves past
// midnight wants the whole night on one business day, not split across two
// reports with the ticket numbers restarting mid-shift.
func TestBusinessDateHonoursTheCutoff(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}

	// 04:00 rollover — the usual choice for a late-night venue.
	s := &Settings{Location: tokyo, CutoffMinute: 240}

	cases := []struct {
		name  string
		local string // venue-local wall clock
		want  string
	}{
		{"mid-service, before midnight", "2026-03-01T22:30:00", "2026-03-01"},
		{"just after midnight is still the same business day", "2026-03-02T00:30:00", "2026-03-01"},
		{"just before the cutoff", "2026-03-02T03:59:00", "2026-03-01"},
		{"exactly at the cutoff starts the new day", "2026-03-02T04:00:00", "2026-03-02"},
		{"lunchtime", "2026-03-02T12:00:00", "2026-03-02"},
		// Month and year ends are where a hand-rolled "decrement the date"
		// implementation goes wrong.
		{"across a month boundary", "2026-04-01T01:00:00", "2026-03-31"},
		{"across a year boundary", "2027-01-01T02:00:00", "2026-12-31"},
		{"across a leap day", "2028-03-01T01:00:00", "2028-02-29"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			local, err := time.ParseInLocation("2006-01-02T15:04:05", tc.local, tokyo)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := s.BusinessDate(local).Format(time.DateOnly); got != tc.want {
				t.Errorf("BusinessDate(%s local) = %s, want %s", tc.local, got, tc.want)
			}
		})
	}
}

// TestZeroCutoffIsUnchangedBehaviour guards the additive promise: a venue
// that never sets the field must bucket orders exactly as it did before
// the setting existed.
func TestZeroCutoffIsUnchangedBehaviour(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	s := &Settings{Location: tokyo, CutoffMinute: 0}

	for _, tc := range []struct{ local, want string }{
		{"2026-03-01T23:59:00", "2026-03-01"},
		{"2026-03-02T00:00:00", "2026-03-02"},
		{"2026-03-02T00:01:00", "2026-03-02"},
	} {
		local, _ := time.ParseInLocation("2006-01-02T15:04:05", tc.local, tokyo)
		if got := s.BusinessDate(local).Format(time.DateOnly); got != tc.want {
			t.Errorf("BusinessDate(%s) = %s, want %s (plain midnight)", tc.local, got, tc.want)
		}
	}
}

// TestCutoffSurvivesADSTTransition: the shift is applied in venue-local
// time, so a spring-forward night must still resolve to one business day
// rather than skipping or duplicating one.
func TestCutoffSurvivesADSTTransition(t *testing.T) {
	london, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	s := &Settings{Location: london, CutoffMinute: 240}

	// 2026-03-29 is the UK spring-forward: 01:00 GMT jumps to 02:00 BST.
	for _, tc := range []struct{ local, want string }{
		{"2026-03-29T00:30:00", "2026-03-28"}, // before the jump, before the cutoff
		{"2026-03-29T03:30:00", "2026-03-28"}, // after the jump, still before the cutoff
		{"2026-03-29T05:00:00", "2026-03-29"}, // past the cutoff
	} {
		local, err := time.ParseInLocation("2006-01-02T15:04:05", tc.local, london)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got := s.BusinessDate(local).Format(time.DateOnly); got != tc.want {
			t.Errorf("BusinessDate(%s) = %s, want %s", tc.local, got, tc.want)
		}
	}
}

func TestCutoffIsCarriedFromTheProto(t *testing.T) {
	got := fromProto(&v1_catalogpb.VenueSettings{VenueId: "v1", BusinessDayCutoffMinute: 240})
	if got.CutoffMinute != 240 {
		t.Errorf("CutoffMinute = %d, want 240", got.CutoffMinute)
	}
	// Unset must stay 0 rather than picking up a default, or a venue that
	// never configured it would silently start bucketing differently.
	if got := fromProto(&v1_catalogpb.VenueSettings{VenueId: "v1"}); got.CutoffMinute != 0 {
		t.Errorf("CutoffMinute = %d, want 0 when unset", got.CutoffMinute)
	}
}
