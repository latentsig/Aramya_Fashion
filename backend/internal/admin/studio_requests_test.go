package admin

import "testing"

func TestAppointmentTransitionMatrix(t *testing.T) {
	tests := []struct {
		from, to string
		allowed  bool
	}{
		{"requested", "confirmed", true}, {"requested", "cancelled", true}, {"requested", "completed", false},
		{"confirmed", "completed", true}, {"confirmed", "cancelled", true}, {"confirmed", "requested", false},
		{"completed", "cancelled", false}, {"cancelled", "confirmed", false},
	}
	for _, test := range tests {
		t.Run(test.from+"_to_"+test.to, func(t *testing.T) {
			if got := CanTransitionAppointment(test.from, test.to); got != test.allowed {
				t.Fatalf("transition allowed=%v, want %v", got, test.allowed)
			}
		})
	}
}

func TestCustomOrderTransitionMatrix(t *testing.T) {
	tests := []struct {
		from, to string
		allowed  bool
	}{
		{"requested", "reviewing", true}, {"requested", "declined", true}, {"requested", "approved", false},
		{"reviewing", "fitting", true}, {"reviewing", "quoted", true}, {"fitting", "quoted", true},
		{"quoted", "awaiting_approval", true}, {"awaiting_approval", "approved", false},
		{"approved", "in_progress", false}, {"in_progress", "completed", false}, {"completed", "cancelled", false},
	}
	for _, test := range tests {
		t.Run(test.from+"_to_"+test.to, func(t *testing.T) {
			if got := CanTransitionCustomOrder(test.from, test.to); got != test.allowed {
				t.Fatalf("transition allowed=%v, want %v", got, test.allowed)
			}
		})
	}
}
