package gateway

import (
	"context"

	"github.com/llmhub/llmhub/internal/iam"
	"github.com/llmhub/llmhub/internal/wallet"
)

// WalletBilling adapts wallet.Service to the chat.BillingClient interface.
type WalletBilling struct{ S *wallet.Service }

// Freeze pre-authorizes cents against the user's wallet.
func (b WalletBilling) Freeze(ctx context.Context, requestID string, userID, cents int64) error {
	return b.S.Freeze(ctx, requestID, userID, cents)
}

// Settle finalizes the hold with the actual cost.
func (b WalletBilling) Settle(ctx context.Context, requestID string, cents int64) error {
	return b.S.Settle(ctx, requestID, cents)
}

// Release returns the hold's amount to the user.
func (b WalletBilling) Release(ctx context.Context, requestID string) error {
	return b.S.Release(ctx, requestID)
}

// IAMAuth adapts iam.Service to chat.AuthResolver.
type IAMAuth struct{ S *iam.Service }

// AuthenticateAPIKey resolves a bearer token to a user id.
func (a IAMAuth) AuthenticateAPIKey(ctx context.Context, plaintext string) (int64, int64, error) {
	k, err := a.S.AuthenticateAPIKey(ctx, plaintext)
	if err != nil {
		return 0, 0, err
	}
	return k.UserID, k.ID, nil
}
