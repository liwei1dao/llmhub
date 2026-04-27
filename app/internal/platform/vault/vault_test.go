package vault

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDevInlineKey(t *testing.T) {
	t.Parallel()
	m, err := DevInline{}.Resolve(context.Background(), "devkey://sk-upstream-abc")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if m["api_key"] != "sk-upstream-abc" {
		t.Fatalf("unexpected: %+v", m)
	}
}

func TestDevInlineAKSK(t *testing.T) {
	t.Parallel()
	m, err := DevInline{}.Resolve(context.Background(), "devaksk://AK:SK")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if m["access_key"] != "AK" || m["secret_key"] != "SK" {
		t.Fatalf("unexpected: %+v", m)
	}
}

func TestDevInlineUnknown(t *testing.T) {
	t.Parallel()
	if _, err := (DevInline{}).Resolve(context.Background(), "vault://real/path"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

type fake struct{ calls int }

func (f *fake) Resolve(_ context.Context, _ string) (map[string]string, error) {
	f.calls++
	return map[string]string{"api_key": "k"}, nil
}

func TestCachedHit(t *testing.T) {
	t.Parallel()
	f := &fake{}
	c := NewCached(f)
	c.TTL = 10 * time.Millisecond
	ctx := context.Background()
	if _, err := c.Resolve(ctx, "ref"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Resolve(ctx, "ref"); err != nil {
		t.Fatal(err)
	}
	if f.calls != 1 {
		t.Fatalf("expected single upstream call, got %d", f.calls)
	}
}
