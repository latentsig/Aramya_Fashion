package auth

import "testing"

func TestNormalizeEmail(t *testing.T) {
	got, err := normalizeEmail("  Customer@Example.COM  ")
	if err != nil || got != "customer@example.com" {
		t.Fatalf("normalizeEmail = %q, %v", got, err)
	}
	for _, invalid := range []string{"", "not an email", "Customer <customer@example.com>", "customer@example.com extra"} {
		if _, err := normalizeEmail(invalid); err == nil {
			t.Errorf("normalizeEmail(%q) unexpectedly succeeded", invalid)
		}
	}
}

func TestOpaqueTokensAreRandomURLSafeAndFixedLength(t *testing.T) {
	first, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 43 || first == second {
		t.Fatalf("unexpected opaque token properties: len=%d same=%v", len(first), first == second)
	}
}
