package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// CreateCredentialWithBindings inserts a credential and N service
// bindings in a single transaction. Either every row commits or none
// does — exactly the contract the admin "新增凭据" wizard step 5
// promises in the UI.
//
// Caller is responsible for static validation (catalog constants):
//   - cred.VendorID == products[cred.ProductID].VendorID
//   - cred.VendorID == accounts[cred.AccountID].vendor_id
//   - every b.Capability ∈ products[cred.ProductID].AllowedCapabilities
//
// On success cred.ID and each b.ID are populated.
func (r *Repo) CreateCredentialWithBindings(
	ctx context.Context,
	cred *Credential,
	bindings []*ServiceBinding,
) error {
	credRepo := r.Credentials()
	bindRepo := r.Bindings()
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		credID, err := credRepo.createTx(ctx, tx, cred)
		if err != nil {
			return err
		}
		for _, b := range bindings {
			b.CredentialID = credID
			if _, err := bindRepo.createTx(ctx, tx, b); err != nil {
				return err
			}
		}
		return nil
	})
}
