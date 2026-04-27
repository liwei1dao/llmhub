// Package golden_test is the entry point for protocol conformance tests.
// Real harness lands in M4 alongside the first provider adapter.
package golden

import "testing"

// TestGoldenPlaceholder ensures `make test-golden` has at least one test
// to execute until the real harness ships. Remove once the harness lands.
func TestGoldenPlaceholder(t *testing.T) {
	t.Log("golden harness placeholder; real tests land in M4")
}
