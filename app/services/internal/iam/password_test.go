package iam

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	t.Parallel()
	h, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := VerifyPassword(h, "correct-horse-battery-staple"); err != nil {
		t.Fatalf("verify correct: %v", err)
	}
	if err := VerifyPassword(h, "wrong-password"); err == nil {
		t.Fatal("verify wrong: expected mismatch, got nil")
	}
}

func TestHashPasswordEmpty(t *testing.T) {
	t.Parallel()
	if _, err := HashPassword(""); err == nil {
		t.Fatal("expected error on empty password")
	}
}

func TestHashPasswordUniqueness(t *testing.T) {
	t.Parallel()
	// Two hashes of the same password must differ thanks to the per-hash salt.
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Fatal("expected different hashes for the same input")
	}
}
