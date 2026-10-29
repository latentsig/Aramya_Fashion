package admin

import (
	"errors"
	"testing"
)

func TestAdminOrderTransitionMatrix(t *testing.T) {
	tests := []struct {
		from, to string
		allowed  bool
	}{
		{"pending_payment", "cancelled", true},
		{"pending_payment", "paid", false},
		{"paid", "confirmed", true},
		{"paid", "processing", false},
		{"confirmed", "processing", true},
		{"processing", "ready_to_ship", true},
		{"ready_to_ship", "shipped", true},
		{"shipped", "delivered", true},
		{"refund_pending", "refunded", false},
		{"delivered", "cancelled", false},
		{"paid", "refunded", false},
	}
	for _, test := range tests {
		t.Run(test.from+"_to_"+test.to, func(t *testing.T) {
			if actual := ValidAdminOrderTransition(test.from, test.to); actual != test.allowed {
				t.Fatalf("transition allowed=%v, want %v", actual, test.allowed)
			}
		})
	}
}

func TestValidateAdminOrderUpdate(t *testing.T) {
	base := OrderUpdate{ExpectedVersion: 2, Status: "shipped", Reason: "Carrier collected parcel", ShipmentProvider: "Aramya Logistics", TrackingNumber: "AR-TRACK-123", TrackingURL: "https://track.example.test/AR-TRACK-123"}
	tests := []struct {
		name  string
		edit  func(*OrderUpdate)
		valid bool
	}{
		{name: "valid shipment", valid: true},
		{name: "requires current version", edit: func(input *OrderUpdate) { input.ExpectedVersion = 0 }},
		{name: "requires reason", edit: func(input *OrderUpdate) { input.Reason = "  " }},
		{name: "requires carrier", edit: func(input *OrderUpdate) { input.ShipmentProvider = "" }},
		{name: "requires tracking id", edit: func(input *OrderUpdate) { input.TrackingNumber = "" }},
		{name: "requires https tracking", edit: func(input *OrderUpdate) { input.TrackingURL = "http://track.example.test/a" }},
		{name: "no carrier for non-shipped transition", edit: func(input *OrderUpdate) { input.Status = "confirmed" }},
		{name: "never allow paid as staff target", edit: func(input *OrderUpdate) { input.Status = "paid" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := base
			if test.edit != nil {
				test.edit(&input)
			}
			err := ValidateOrderUpdate(input)
			if (err == nil) != test.valid {
				t.Fatalf("error=%v, valid=%v", err, test.valid)
			}
			if !test.valid && !errors.Is(err, ErrInvalidCatalog) {
				t.Fatalf("error=%v, want ErrInvalidCatalog", err)
			}
		})
	}
}
