package wallet

import (
	"strings"
	"testing"
)

func TestNewOrderNoShape(t *testing.T) {
	t.Parallel()
	on, err := newOrderNo()
	if err != nil {
		t.Fatalf("newOrderNo: %v", err)
	}
	if !strings.HasPrefix(on, "RC-") {
		t.Fatalf("missing RC- prefix: %s", on)
	}
	parts := strings.Split(on, "-")
	if len(parts) != 3 {
		t.Fatalf("unexpected segment count: %v", parts)
	}
	if len(parts[1]) != 8 {
		t.Fatalf("date segment must be 8 chars: %s", parts[1])
	}
	if len(parts[2]) != 8 {
		t.Fatalf("hex segment must be 8 chars: %s", parts[2])
	}
}

func TestNewOrderNoUnique(t *testing.T) {
	t.Parallel()
	a, _ := newOrderNo()
	b, _ := newOrderNo()
	if a == b {
		t.Fatalf("expected unique order numbers, got %s twice", a)
	}
}
