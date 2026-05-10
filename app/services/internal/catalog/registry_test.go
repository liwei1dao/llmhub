package catalog

import "testing"

// TestValidate is a guard against silent drift in the static
// dictionaries — any inconsistency must fail CI before it reaches
// the binary's init().
func TestValidate(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatalf("static catalog invariant failed: %v", err)
	}
}

// TestExpectedShape pins down the MVP catalog size: only volc/ark/chat.
// 后续接入新 vendor / product / capability 时同步更新这里的数字，避免静默
// 走样。
func TestExpectedShape(t *testing.T) {
	if got, want := len(Categories), 1; got != want {
		t.Errorf("Categories len = %d, want %d", got, want)
	}
	if got, want := len(Vendors), 1; got != want {
		t.Errorf("Vendors len = %d, want %d", got, want)
	}
	if got, want := len(Products), 1; got != want {
		t.Errorf("Products len = %d, want %d", got, want)
	}
	if got, want := len(Capabilities), 1; got != want {
		t.Errorf("Capabilities len = %d, want %d", got, want)
	}
}

func TestProductsByVendorPartition(t *testing.T) {
	groups := ProductsByVendor()
	if got := len(groups["volc"]); got != 1 {
		t.Errorf("vendor volc: got %d products, want 1", got)
	}
}

func TestProductAllowsCapability(t *testing.T) {
	if !ProductAllowsCapability("volc.ark", "chat") {
		t.Errorf("volc.ark should allow chat")
	}
	if ProductAllowsCapability("volc.ark", "asr_realtime") {
		t.Errorf("volc.ark should not allow asr_realtime (that capability is not yet activated)")
	}
	if ProductAllowsCapability("nonexistent", "chat") {
		t.Errorf("unknown product should not allow anything")
	}
}

func TestCapabilityCategories(t *testing.T) {
	c, ok := Capabilities["chat"]
	if !ok {
		t.Fatalf("capability chat missing")
	}
	if c.CategoryID != "llm" {
		t.Errorf("chat.CategoryID = %q, want llm", c.CategoryID)
	}
}
