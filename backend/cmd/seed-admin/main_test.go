package main

import "testing"

func TestValidateSetupInput(t *testing.T) {
	tests := []struct {
		name, email, displayName, password string
		valid                              bool
	}{
		{name: "valid", email: "owner@example.com", displayName: "Studio Owner", password: "Strong-Dev-Password-2026!", valid: true},
		{name: "normalizes email and name", email: " Owner@Example.com ", displayName: "  Studio Owner  ", password: "Strong-Dev-Password-2026!", valid: true},
		{name: "rejects malformed email", email: "not-an-email", displayName: "Studio Owner", password: "Strong-Dev-Password-2026!"},
		{name: "rejects short password", email: "owner@example.com", displayName: "Studio Owner", password: "short"},
		{name: "rejects empty name", email: "owner@example.com", displayName: " ", password: "Strong-Dev-Password-2026!"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			email, name, err := validateSetupInput(test.email, test.displayName, test.password)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, err=%v", err == nil, err)
			}
			if test.valid && (email != "owner@example.com" || name != "Studio Owner") {
				t.Fatalf("normalized values = %q, %q", email, name)
			}
		})
	}
}
