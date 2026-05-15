// 邮箱验证码：Redis 存 6 位数字，TTL 10 分钟；同一邮箱 60s 内只能再发
// 一次；同一验证码最多被尝试校验 5 次后强制失效（防爆破）。
//
// Key 设计（统一前缀 `email_verify:`）：
//
//   email_verify:code:<email>      → 6 位数字, TTL 600s
//   email_verify:cooldown:<email>  → "1",    TTL 60s   (节流)
//   email_verify:attempts:<email>  → 计数,    TTL 600s (与 code 同寿命)
//
// 不在数据库里建 pending 用户行，账号要等到 /auth/register 携带正确
// 验证码 + 密码时才落库。

package account

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	verifyCodeTTL     = 10 * time.Minute
	verifyCooldownTTL = 60 * time.Second
	verifyMaxAttempts = 5
)

// VerifySender is the subset of Mailer the verify flow needs.
type VerifySender interface {
	Send(ctx context.Context, to, subject, body string) error
	Enabled() bool
}

// Verifier wraps Redis-backed verification-code state for registration.
type Verifier struct {
	rds    *redis.Client
	mailer VerifySender
}

// NewVerifier — nil rds disables the feature entirely; callers check
// Enabled() before exposing send-code/register-with-code paths.
func NewVerifier(rds *redis.Client, mailer VerifySender) *Verifier {
	return &Verifier{rds: rds, mailer: mailer}
}

// Enabled reports whether the verifier has the dependencies it needs.
// When false, the public flow falls back to "no email verification" so
// dev environments without Redis/SMTP keep working.
func (v *Verifier) Enabled() bool { return v != nil && v.rds != nil }

// MailerWorks reports whether real outbound email is wired. False means
// codes can still be issued but they only land in server logs (dev).
func (v *Verifier) MailerWorks() bool {
	return v != nil && v.mailer != nil && v.mailer.Enabled()
}

// Issue generates a fresh code, stores it (resetting attempts), enforces
// the per-email send cooldown, then asks the mailer to deliver it.
//
// Errors are surfaced verbatim to callers — handler maps them onto HTTP
// codes. ErrTooFrequent is returned when the cooldown is active.
var ErrTooFrequent = errors.New("verification code already sent; please wait before retrying")

func (v *Verifier) Issue(ctx context.Context, email string) error {
	if !v.Enabled() {
		return errors.New("verifier disabled")
	}
	email = normalizeEmail(email)
	if email == "" {
		return errors.New("invalid email")
	}

	// SetNX cooldown — atomic check-and-set: first sender wins.
	ok, err := v.rds.SetNX(ctx, cooldownKey(email), "1", verifyCooldownTTL).Result()
	if err != nil {
		return fmt.Errorf("redis cooldown setnx: %w", err)
	}
	if !ok {
		return ErrTooFrequent
	}

	code, err := randomCode6()
	if err != nil {
		return err
	}

	// Pipeline: set code + reset attempts in one round trip.
	pipe := v.rds.TxPipeline()
	pipe.Set(ctx, codeKey(email), code, verifyCodeTTL)
	pipe.Del(ctx, attemptsKey(email))
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis store code: %w", err)
	}

	subject := "【LLMHub】邮箱验证码"
	body := fmt.Sprintf(
		"您正在注册 LLMHub 账户。\n\n本次验证码为：%s\n\n验证码 10 分钟内有效，请勿向他人泄露。\n如非本人操作，请忽略此邮件。\n",
		code,
	)
	if err := v.mailer.Send(ctx, email, subject, body); err != nil {
		// Roll back the cooldown so the user can retry — otherwise a
		// transient SMTP hiccup locks them out for a full minute.
		_ = v.rds.Del(ctx, cooldownKey(email)).Err()
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
}

// Check validates a user-submitted code. On match the code is deleted
// (single-use). On mismatch the attempt counter increments; after
// verifyMaxAttempts misses the code is force-invalidated so callers
// can't brute-force in 10 minutes.
var (
	ErrCodeMissing  = errors.New("verification code not found or expired")
	ErrCodeMismatch = errors.New("verification code does not match")
)

func (v *Verifier) Check(ctx context.Context, email, input string) error {
	if !v.Enabled() {
		return errors.New("verifier disabled")
	}
	email = normalizeEmail(email)
	input = strings.TrimSpace(input)
	if email == "" || input == "" {
		return ErrCodeMismatch
	}

	expected, err := v.rds.Get(ctx, codeKey(email)).Result()
	if errors.Is(err, redis.Nil) {
		return ErrCodeMissing
	}
	if err != nil {
		return fmt.Errorf("redis get code: %w", err)
	}

	if expected == input {
		// Single-use: nuke both the code and the attempts counter so a
		// reused code can never re-authenticate. Cooldown is left to
		// expire naturally — re-issuing immediately gives no benefit.
		pipe := v.rds.TxPipeline()
		pipe.Del(ctx, codeKey(email))
		pipe.Del(ctx, attemptsKey(email))
		_, _ = pipe.Exec(ctx)
		return nil
	}

	// Mismatch path: bump attempts; if over limit, hard-delete code.
	n, err := v.rds.Incr(ctx, attemptsKey(email)).Result()
	if err == nil && n == 1 {
		_ = v.rds.Expire(ctx, attemptsKey(email), verifyCodeTTL).Err()
	}
	if n >= verifyMaxAttempts {
		pipe := v.rds.TxPipeline()
		pipe.Del(ctx, codeKey(email))
		pipe.Del(ctx, attemptsKey(email))
		_, _ = pipe.Exec(ctx)
	}
	return ErrCodeMismatch
}

// -------- helpers --------

func codeKey(email string) string     { return "email_verify:code:" + email }
func cooldownKey(email string) string { return "email_verify:cooldown:" + email }
func attemptsKey(email string) string { return "email_verify:attempts:" + email }

func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func randomCode6() (string, error) {
	// crypto/rand to avoid predictability — guessing space is only 1e6
	// so weak entropy materially helps an attacker.
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
