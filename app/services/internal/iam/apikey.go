package iam

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// APIKeyPrefix is the public human-readable prefix for all LLMHub
// user-facing API keys. Use it to detect obvious misconfigurations
// (e.g. user pasted an upstream OpenAI key into LLMHub).
const APIKeyPrefix = "sk-llmh-"

// GeneratedAPIKey is the full key returned to the caller on creation.
// The plaintext is returned exactly once — callers must persist only
// the PrefixVisible + Hash.
type GeneratedAPIKey struct {
	Plaintext      string // returned to user once
	PrefixVisible  string // shown on listings, e.g. "sk-llmh-1a2b****f8e9"
	Hash           string // hex SHA-256 of Plaintext, stored in DB
}

// NewAPIKey generates a fresh API key with 256 bits of entropy.
func NewAPIKey() (*GeneratedAPIKey, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, fmt.Errorf("api key entropy: %w", err)
	}
	body := hex.EncodeToString(raw[:])
	pt := APIKeyPrefix + body

	sum := sha256.Sum256([]byte(pt))
	hash := hex.EncodeToString(sum[:])

	// "sk-llmh-<first 4>****<last 4>"
	head := body[:4]
	tail := body[len(body)-4:]
	visible := APIKeyPrefix + head + "****" + tail

	return &GeneratedAPIKey{
		Plaintext:     pt,
		PrefixVisible: visible,
		Hash:          hash,
	}, nil
}

// HashAPIKey returns the storage hash of a plaintext key.
// Use this on the auth hot path to look up by key_hash.
func HashAPIKey(plaintext string) (string, error) {
	if !isLikelyAPIKey(plaintext) {
		return "", errors.New("iam: not an LLMHub API key")
	}
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:]), nil
}

func isLikelyAPIKey(s string) bool {
	return len(s) == len(APIKeyPrefix)+64 && s[:len(APIKeyPrefix)] == APIKeyPrefix
}
