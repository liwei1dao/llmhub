package iam

import (
	"strings"
	"testing"
)

func TestNewAPIKeyShape(t *testing.T) {
	t.Parallel()
	k, err := NewAPIKey()
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if !strings.HasPrefix(k.Plaintext, APIKeyPrefix) {
		t.Fatalf("plaintext missing prefix: %s", k.Plaintext)
	}
	if len(k.Plaintext) != len(APIKeyPrefix)+64 {
		t.Fatalf("plaintext length wrong: %d", len(k.Plaintext))
	}
	if !strings.HasPrefix(k.PrefixVisible, APIKeyPrefix) {
		t.Fatalf("prefix missing llmh: %s", k.PrefixVisible)
	}
	if !strings.Contains(k.PrefixVisible, "****") {
		t.Fatalf("visible form should mask middle: %s", k.PrefixVisible)
	}
	if len(k.Hash) != 64 {
		t.Fatalf("hash should be hex sha-256 (64 chars), got %d", len(k.Hash))
	}
}

func TestHashAPIKeyRoundTrip(t *testing.T) {
	t.Parallel()
	k, _ := NewAPIKey()
	h, err := HashAPIKey(k.Plaintext)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if h != k.Hash {
		t.Fatalf("hash mismatch: got %s want %s", h, k.Hash)
	}
}

func TestHashAPIKeyRejectsForeign(t *testing.T) {
	t.Parallel()
	_, err := HashAPIKey("sk-openai-abc")
	if err == nil {
		t.Fatal("expected error for non-llmhub key")
	}
}
