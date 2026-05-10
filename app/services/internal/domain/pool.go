package domain

import "time"

// AccountState is the lifecycle state of a pool account.
type AccountState string

const (
	AccountWarmup       AccountState = "warmup"
	AccountActive       AccountState = "active"
	AccountCooling      AccountState = "cooling"
	AccountRateLimited  AccountState = "rate_limited"
	AccountQuarantine   AccountState = "quarantine"
	AccountDepleted     AccountState = "depleted"
	AccountArchived     AccountState = "archived"
)

// PoolAccount is the domain-level view of an upstream account.
// Persistence details (Vault refs, raw DB rows) are hidden in the repo layer.
type PoolAccount struct {
	ID                     int64
	ProviderID             string
	Tier                   Tier
	State                  AccountState
	HealthScore            int
	SupportedCapabilities  []string
	QuotaTotalCents        int64
	QuotaUsedCents         int64
	QuotaResetAt           *time.Time
	QPSLimit               int
	DailyLimitCents        int64
	IsolationGroupID       int64
	RegisteredAt           *time.Time
	WarmupEndsAt           *time.Time
	LastUsedAt             *time.Time
	LastErrorAt            *time.Time
	ConsecutiveFailures    int
	Tags                   []string
}

// IsSchedulable returns true if the account can serve requests right now.
func (a *PoolAccount) IsSchedulable() bool {
	return a.State == AccountActive && a.HealthScore >= 40
}

// SupportsCapability returns true if the account declares support for c.
func (a *PoolAccount) SupportsCapability(c string) bool {
	for _, s := range a.SupportedCapabilities {
		if s == c {
			return true
		}
	}
	return false
}
