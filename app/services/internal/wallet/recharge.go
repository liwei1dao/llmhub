package wallet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/llmhub/llmhub/internal/wallet/repo"
)

// Recharge is exported for handler types.
type Recharge = repo.Recharge

// CreateRechargeRequest is the input for opening a recharge order.
// AmountCents must be positive; channel is "alipay" / "wechat" /
// "stripe" / "manual". The PSP-side payment URL is allocated by the
// caller (handler) — this layer only persists the order.
type CreateRechargeRequest struct {
	UserID      int64
	AmountCents int64
	Channel     string
}

// CreateRecharge mints an order_no and inserts a pending row.
func (s *Service) CreateRecharge(ctx context.Context, req CreateRechargeRequest) (*repo.Recharge, error) {
	if req.AmountCents <= 0 {
		return nil, ErrNegativeAmount
	}
	if req.Channel == "" {
		return nil, fmt.Errorf("wallet: channel is required")
	}
	orderNo, err := newOrderNo()
	if err != nil {
		return nil, err
	}
	return s.repo.CreateRecharge(ctx, req.UserID, orderNo, req.Channel, req.AmountCents)
}

// ConfirmRecharge marks the order paid and credits the wallet
// atomically. Idempotent against duplicate webhook deliveries.
func (s *Service) ConfirmRecharge(ctx context.Context, orderNo, channelOrderID string) error {
	return s.repo.ConfirmRecharge(ctx, orderNo, channelOrderID)
}

// GetRecharge returns one recharge by order_no.
func (s *Service) GetRecharge(ctx context.Context, orderNo string) (*repo.Recharge, error) {
	return s.repo.GetRechargeByOrderNo(ctx, orderNo)
}

// ListRecharges returns the user's recent recharge orders.
func (s *Service) ListRecharges(ctx context.Context, userID int64, limit int) ([]repo.Recharge, error) {
	return s.repo.ListRechargesByUser(ctx, userID, limit)
}

// newOrderNo returns a recharge order number of the form
// "RC-YYYYMMDD-<8 hex>". Short enough to print on receipts; entropy
// dominated by the random suffix so collisions on the same day are
// vanishingly unlikely.
func newOrderNo() (string, error) {
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("RC-%s-%s",
		time.Now().UTC().Format("20060102"),
		hex.EncodeToString(raw[:])), nil
}
