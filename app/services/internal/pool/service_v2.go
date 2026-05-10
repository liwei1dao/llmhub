package pool

import (
	"context"
	"errors"
	"fmt"

	"github.com/llmhub/llmhub/internal/catalog"
	"github.com/llmhub/llmhub/internal/pool/repo"
)

// Errors returned by the v0.2 service layer. They wrap repo errors and
// add the static-catalog invariant violations admin code paths must
// distinguish from generic DB failures.
var (
	ErrUnknownVendor       = errors.New("pool: unknown vendor")
	ErrUnknownProduct      = errors.New("pool: unknown product")
	ErrUnknownCapability   = errors.New("pool: unknown capability")
	ErrVendorMismatch      = errors.New("pool: vendor mismatch between account and product")
	ErrCapabilityNotAllowed = errors.New("pool: capability not allowed by product")
)

// CreateVendorAccountInput is the externally-facing parameter shape for
// creating a master account row. The auth payload is referenced by
// vault path; the secret itself is written by the caller.
type CreateVendorAccountInput struct {
	VendorID      string
	Name          string
	Entity        string
	ConsoleURL    string
	MasterAuthRef string
}

// CreateVendorAccount validates the vendor against the static catalog
// dictionary and inserts a row.
func (s *Service) CreateVendorAccount(ctx context.Context, in CreateVendorAccountInput) (*repo.VendorAccount, error) {
	if _, ok := catalog.LookupVendor(in.VendorID); !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownVendor, in.VendorID)
	}
	a := &repo.VendorAccount{
		VendorID:      in.VendorID,
		Name:          in.Name,
		Entity:        in.Entity,
		ConsoleURL:    in.ConsoleURL,
		MasterAuthRef: in.MasterAuthRef,
		Status:        "active",
	}
	if _, err := s.repo.VendorAccounts().Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// CredentialBindingInput is one row's worth of binding data passed
// alongside a credential at creation time.
type CredentialBindingInput struct {
	Capability       string
	Tier             string
	QPSLimit         *int32
	DailyLimitCents  *int64
	QuotaTotalCents  *int64
	CostBasisCents   float64
}

// CreateCredentialInput is the parameter shape for the wizard's final
// "submit" step.
type CreateCredentialInput struct {
	AccountID         int64
	ProductID         string
	Name              string
	Env               string
	AuthPayloadRef    string
	AuthPayloadDigest string
	IsolationGroupID  *int64
	Bindings          []CredentialBindingInput
}

// CreateCredential validates static-catalog invariants and writes the
// credential + N bindings in a single transaction.
//
// Static invariants enforced here (DB cannot express them since the
// catalog dictionary is in code):
//   - product exists in catalog.Products
//   - account.VendorID matches product.VendorID
//   - every binding's capability is in product.AllowedCapabilities
func (s *Service) CreateCredential(ctx context.Context, in CreateCredentialInput) (*repo.Credential, []*repo.ServiceBinding, error) {
	prod, ok := catalog.LookupProduct(in.ProductID)
	if !ok {
		return nil, nil, fmt.Errorf("%w: %q", ErrUnknownProduct, in.ProductID)
	}

	acct, err := s.repo.VendorAccounts().Get(ctx, in.AccountID)
	if err != nil {
		return nil, nil, err
	}
	if acct.VendorID != prod.VendorID {
		return nil, nil, fmt.Errorf("%w: account vendor=%s product vendor=%s",
			ErrVendorMismatch, acct.VendorID, prod.VendorID)
	}

	for _, b := range in.Bindings {
		if !catalog.ProductAllowsCapability(in.ProductID, b.Capability) {
			return nil, nil, fmt.Errorf("%w: %s on %s", ErrCapabilityNotAllowed, b.Capability, in.ProductID)
		}
	}

	cred := &repo.Credential{
		VendorID:          prod.VendorID,
		AccountID:         in.AccountID,
		ProductID:         in.ProductID,
		Name:              in.Name,
		Env:               in.Env,
		AuthPayloadRef:    in.AuthPayloadRef,
		AuthPayloadDigest: in.AuthPayloadDigest,
		IsolationGroupID:  in.IsolationGroupID,
	}
	bindings := make([]*repo.ServiceBinding, 0, len(in.Bindings))
	for _, b := range in.Bindings {
		bindings = append(bindings, &repo.ServiceBinding{
			Capability:      b.Capability,
			Tier:            b.Tier,
			QPSLimit:        b.QPSLimit,
			DailyLimitCents: b.DailyLimitCents,
			QuotaTotalCents: b.QuotaTotalCents,
			CostBasisCents:  b.CostBasisCents,
		})
	}

	if err := s.repo.CreateCredentialWithBindings(ctx, cred, bindings); err != nil {
		return nil, nil, err
	}
	return cred, bindings, nil
}

// AddBindingInput attaches a new service to an existing credential
// (the "+ 加服务" action on the credential management page).
type AddBindingInput struct {
	CredentialID    int64
	Capability      string
	Tier            string
	QPSLimit        *int32
	DailyLimitCents *int64
	QuotaTotalCents *int64
	CostBasisCents  float64
}

// AddBinding adds one binding to an existing credential. Validates
// the capability against the credential's product allow-list.
func (s *Service) AddBinding(ctx context.Context, in AddBindingInput) (*repo.ServiceBinding, error) {
	cred, err := s.repo.Credentials().Get(ctx, in.CredentialID)
	if err != nil {
		return nil, err
	}
	if !catalog.ProductAllowsCapability(cred.ProductID, in.Capability) {
		return nil, fmt.Errorf("%w: %s on %s", ErrCapabilityNotAllowed, in.Capability, cred.ProductID)
	}
	b := &repo.ServiceBinding{
		CredentialID:    in.CredentialID,
		Capability:      in.Capability,
		Tier:            in.Tier,
		QPSLimit:        in.QPSLimit,
		DailyLimitCents: in.DailyLimitCents,
		QuotaTotalCents: in.QuotaTotalCents,
		CostBasisCents:  in.CostBasisCents,
	}
	if _, err := s.repo.Bindings().Create(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

// PickBinding is the scheduler hot-path entry. Returns up to N
// candidate bindings for (product, capability). The scheduler layer
// filters further by isolation/sticky/etc.
func (s *Service) PickBinding(ctx context.Context, productID, capability string, minHealth int16) ([]repo.PickedBinding, error) {
	if _, ok := catalog.LookupProduct(productID); !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownProduct, productID)
	}
	if _, ok := catalog.LookupCapability(capability); !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownCapability, capability)
	}
	return s.repo.Bindings().Pick(ctx, repo.PickQuery{
		ProductID: productID, Capability: capability, MinHealth: minHealth,
	})
}
