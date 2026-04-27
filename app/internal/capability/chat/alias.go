package chat

import (
	catalogrepo "github.com/llmhub/llmhub/internal/catalog/repo"
)

// pricingAlias avoids spreading catalog/repo imports across the
// handler's method signatures. It is intentionally unexported.
type pricingAlias = catalogrepo.Pricing
