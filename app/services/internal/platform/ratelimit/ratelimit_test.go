package ratelimit

import (
	"context"
	"testing"
)

func TestMemoryAllowsUpToPerSecond(t *testing.T) {
	t.Parallel()
	m := NewMemory()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		ok, err := m.Allow(ctx, "user-1", 5)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if !ok {
			t.Fatalf("call %d denied early", i)
		}
	}
	ok, _ := m.Allow(ctx, "user-1", 5)
	if ok {
		t.Fatal("6th call should be denied")
	}
}

func TestMemoryIsolatesKeys(t *testing.T) {
	t.Parallel()
	m := NewMemory()
	ctx := context.Background()
	_, _ = m.Allow(ctx, "a", 1)
	if ok, _ := m.Allow(ctx, "b", 1); !ok {
		t.Fatal("different key should have its own bucket")
	}
}

func TestMemoryUnlimitedWhenZero(t *testing.T) {
	t.Parallel()
	m := NewMemory()
	for i := 0; i < 100; i++ {
		if ok, _ := m.Allow(context.Background(), "any", 0); !ok {
			t.Fatal("zero perSecond should mean unlimited")
		}
	}
}
