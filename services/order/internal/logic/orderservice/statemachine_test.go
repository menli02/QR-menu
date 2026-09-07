package orderservicelogic

import (
	"testing"

	"github.com/menli02/QR-menu/services/order/internal/model"
)

func TestCanTransitionOrder(t *testing.T) {
	cases := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{"placed to accepted", model.OrderPlaced, model.OrderAccepted, true},
		{"placed to cancelled", model.OrderPlaced, model.OrderCancelled, true},
		{"placed skips straight to ready", model.OrderPlaced, model.OrderReady, false},
		{"placed skips straight to served", model.OrderPlaced, model.OrderServed, false},
		{"accepted to in_progress", model.OrderAccepted, model.OrderInProgress, true},
		{"accepted may skip in_progress", model.OrderAccepted, model.OrderReady, true},
		{"in_progress to ready", model.OrderInProgress, model.OrderReady, true},
		{"ready to served", model.OrderReady, model.OrderServed, true},
		{"ready to cancelled", model.OrderReady, model.OrderCancelled, true},
		{"served is terminal", model.OrderServed, model.OrderReady, false},
		{"served cannot be cancelled", model.OrderServed, model.OrderCancelled, false},
		{"cancelled is terminal", model.OrderCancelled, model.OrderPlaced, false},
		{"no-op is not a transition", model.OrderAccepted, model.OrderAccepted, false},
		{"backwards is rejected", model.OrderReady, model.OrderAccepted, false},
		{"unknown source status", "nonsense", model.OrderAccepted, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canTransitionOrder(tc.from, tc.to); got != tc.want {
				t.Errorf("canTransitionOrder(%q, %q) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestCanTransitionItem(t *testing.T) {
	cases := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{"placed to cooking", model.ItemPlaced, model.ItemCooking, true},
		{"placed may skip cooking", model.ItemPlaced, model.ItemReady, true},
		{"cooking to ready", model.ItemCooking, model.ItemReady, true},
		{"ready may still be cancelled", model.ItemReady, model.ItemCancelled, true},
		{"ready cannot go back to cooking", model.ItemReady, model.ItemCooking, false},
		{"cancelled is terminal", model.ItemCancelled, model.ItemReady, false},
		{"no-op is not a transition", model.ItemCooking, model.ItemCooking, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canTransitionItem(tc.from, tc.to); got != tc.want {
				t.Errorf("canTransitionItem(%q, %q) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestIsTerminalOrder(t *testing.T) {
	for _, s := range []string{model.OrderServed, model.OrderCancelled} {
		if !isTerminalOrder(s) {
			t.Errorf("isTerminalOrder(%q) = false, want true", s)
		}
	}
	for _, s := range []string{model.OrderPlaced, model.OrderAccepted, model.OrderInProgress, model.OrderReady} {
		if isTerminalOrder(s) {
			t.Errorf("isTerminalOrder(%q) = true, want false", s)
		}
	}
}

// TestDeriveOrderStatus covers FR-K5 ("an order becomes ready
// automatically when all non-cancelled items are ready") and the
// all-cancelled mirror.
func TestDeriveOrderStatus(t *testing.T) {
	cases := []struct {
		name    string
		current string
		counts  map[string]int
		want    string
	}{
		{
			name:    "all items ready promotes the order",
			current: model.OrderInProgress,
			counts:  map[string]int{model.ItemReady: 3},
			want:    model.OrderReady,
		},
		{
			name:    "cancelled items don't block readiness",
			current: model.OrderInProgress,
			counts:  map[string]int{model.ItemReady: 2, model.ItemCancelled: 1},
			want:    model.OrderReady,
		},
		{
			name:    "one line still cooking holds the order",
			current: model.OrderInProgress,
			counts:  map[string]int{model.ItemReady: 2, model.ItemCooking: 1},
			want:    "",
		},
		{
			name:    "every line cancelled cancels the order",
			current: model.OrderAccepted,
			counts:  map[string]int{model.ItemCancelled: 2},
			want:    model.OrderCancelled,
		},
		{
			name:    "an already-cancelled order is left alone",
			current: model.OrderCancelled,
			counts:  map[string]int{model.ItemCancelled: 2},
			want:    "",
		},
		{
			name:    "an already-ready order is not re-promoted",
			current: model.OrderReady,
			counts:  map[string]int{model.ItemReady: 2},
			want:    "",
		},
		{
			name:    "a served order is never moved back to ready",
			current: model.OrderServed,
			counts:  map[string]int{model.ItemReady: 2},
			want:    "",
		},
		{
			name:    "no items means nothing to derive",
			current: model.OrderPlaced,
			counts:  map[string]int{},
			want:    "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveOrderStatus(tc.current, tc.counts); got != tc.want {
				t.Errorf("deriveOrderStatus(%q, %v) = %q, want %q", tc.current, tc.counts, got, tc.want)
			}
		})
	}
}

func TestServiceRequestTransitions(t *testing.T) {
	cases := []struct {
		from string
		to   string
		want bool
	}{
		{model.RequestOpen, model.RequestAcknowledged, true},
		{model.RequestOpen, model.RequestResolved, true},
		{model.RequestAcknowledged, model.RequestResolved, true},
		{model.RequestAcknowledged, model.RequestOpen, false},
		{model.RequestResolved, model.RequestAcknowledged, false},
		{model.RequestExpired, model.RequestResolved, true},
		{model.RequestExpired, model.RequestAcknowledged, false},
	}
	for _, tc := range cases {
		if got := serviceRequestTransitions[tc.from][tc.to]; got != tc.want {
			t.Errorf("serviceRequestTransitions[%q][%q] = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}
