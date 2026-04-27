// Package wallet owns user balances and the pre-authorization
// (freeze / settle / release) machinery used by the gateway before
// each AI call.
package wallet

import "errors"

// Sentinel errors returned by wallet.Service. Callers map these to
// domain errors at the HTTP boundary.
var (
	ErrAccountNotFound    = errors.New("wallet: account not found")
	ErrHoldNotFound       = errors.New("wallet: hold not found")
	ErrHoldAlreadyClosed  = errors.New("wallet: hold already closed")
	ErrInsufficientFunds  = errors.New("wallet: insufficient funds")
	ErrNegativeAmount     = errors.New("wallet: non-positive amount")
	ErrConcurrentUpdate   = errors.New("wallet: concurrent update; retry")
)
