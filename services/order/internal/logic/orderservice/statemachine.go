package orderservicelogic

import "github.com/menli02/QR-menu/services/order/internal/model"

// The order and order-item state machines. Kept as plain data in one
// place so the rules are reviewable at a glance and testable without a
// database.
//
// Both machines allow cancellation from every non-terminal state. That is
// deliberate: cancelling a ready dish is a real thing that happens (the
// party left, the guest refused it), and a machine that forbids it just
// pushes staff into marking food served that never was — which corrupts
// the day report far worse than an honest late cancellation does.

var orderTransitions = map[string]map[string]bool{
	model.OrderPlaced: {
		model.OrderAccepted:  true,
		model.OrderCancelled: true,
	},
	model.OrderAccepted: {
		// accepted -> ready skips in_progress on purpose: a drink poured
		// straight away never has a meaningful "cooking" phase, and FR-K3
		// lists "mark ready" as an action available on a ticket, not as a
		// step that must follow "start".
		model.OrderInProgress: true,
		model.OrderReady:      true,
		model.OrderCancelled:  true,
	},
	model.OrderInProgress: {
		model.OrderReady:     true,
		model.OrderCancelled: true,
	},
	model.OrderReady: {
		model.OrderServed:    true,
		model.OrderCancelled: true,
	},
	// served and cancelled are terminal. FR-K8 (recall a closed ticket
	// within 30 min) would reopen served -> ready; it is a Should, not a
	// Must, and no RPC carries the recall intent, so it is not wired here.
	model.OrderServed:    {},
	model.OrderCancelled: {},
}

var itemTransitions = map[string]map[string]bool{
	model.ItemPlaced: {
		model.ItemCooking:   true,
		model.ItemReady:     true,
		model.ItemCancelled: true,
	},
	model.ItemCooking: {
		model.ItemReady:     true,
		model.ItemCancelled: true,
	},
	model.ItemReady: {
		model.ItemCancelled: true,
	},
	model.ItemCancelled: {},
}

// canTransitionOrder reports whether from -> to is legal. A no-op
// transition (from == to) is not legal here; callers treat it as an
// idempotent success before reaching this function, because a retried
// request must not read as a client error.
func canTransitionOrder(from, to string) bool {
	return orderTransitions[from][to]
}

func canTransitionItem(from, to string) bool {
	return itemTransitions[from][to]
}

// isTerminalOrder reports whether an order can still change.
func isTerminalOrder(status string) bool {
	return len(orderTransitions[status]) == 0
}

// deriveOrderStatus implements FR-K5 and its mirror image: an order is
// ready once every non-cancelled line is ready, and cancelled once every
// line is cancelled. It returns "" when no automatic change applies.
//
// It only ever moves an order *forward* to ready — never back off ready
// if a line is later reopened — because the auto-rule exists to save the
// kitchen a tap, not to second-guess a status a human set.
func deriveOrderStatus(current string, counts map[string]int) string {
	total := 0
	for _, n := range counts {
		total += n
	}
	if total == 0 {
		return ""
	}
	if counts[model.ItemCancelled] == total {
		if current != model.OrderCancelled {
			return model.OrderCancelled
		}
		return ""
	}
	live := total - counts[model.ItemCancelled]
	if counts[model.ItemReady] == live && !isTerminalOrder(current) && current != model.OrderReady {
		return model.OrderReady
	}
	return ""
}
