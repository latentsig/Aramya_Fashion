package auth

import "testing"

func TestPasswordHashAndVerification(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}
	if hash[:10] != "$argon2id$" {
		t.Fatalf("unexpected password hash format: %q", hash[:10])
	}
	if !VerifyPassword(hash, "correct-horse-battery-staple") {
		t.Fatal("correct password was not verified")
	}
	if VerifyPassword(hash, "incorrect-password") {
		t.Fatal("incorrect password was verified")
	}
}

func TestPasswordPolicyAndMalformedHash(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected short password rejection")
	}
	if VerifyPassword("not-a-password-hash", "anything") {
		t.Fatal("malformed password hash was accepted")
	}
	if VerifyPassword("$argon2id$v=19$m=4294967295,t=3,p=2$c2FsdHNhbHQ$YWJjZGVmZ2hpamtsbW5vcA", "anything") {
		t.Fatal("unreasonable Argon2 memory settings were accepted")
	}
}
