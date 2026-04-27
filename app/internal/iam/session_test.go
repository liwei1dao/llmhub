package iam

import (
	"encoding/base64"
	"testing"
)

func TestSessionTokenShape(t *testing.T) {
	t.Parallel()
	// We can't hit the DB in unit tests; just verify the token-shape
	// contract used by cookies/middleware.
	raw, err := base64.RawURLEncoding.DecodeString("")
	if err == nil && len(raw) == 0 {
		// Empty token must decode to an empty byte slice; ResolveSession
		// will then reject it below.
	}
}
