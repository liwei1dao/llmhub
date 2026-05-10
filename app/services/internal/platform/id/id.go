// Package id generates sortable unique identifiers for requests,
// holds, events, etc. ULIDs are used so IDs are time-ordered.
package id

import (
	"crypto/rand"
	"io"
	"time"

	"github.com/oklog/ulid/v2"
)

// Request returns a new ULID suitable for a request id.
// Format example: "01HX5E4PW8Q1ZMJP7A8X4V9T3Q".
func Request() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}

// Prefixed returns a ULID with a short prefix, e.g. "req_01HX5E..."
func Prefixed(prefix string) string {
	return prefix + "_" + Request()
}

var entropy io.Reader = rand.Reader
