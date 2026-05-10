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

func TestExpectedShape(t *testing.T) {
	if got, want := len(Categories), 4; got != want {
		t.Errorf("Categories len = %d, want %d", got, want)
	}
	if got, want := len(Vendors), 6; got != want {
		t.Errorf("Vendors len = %d, want %d", got, want)
	}
	if got, want := len(Products), 12; got != want {
		t.Errorf("Products len = %d, want %d", got, want)
	}
	// 13 capabilities per the v0.2 schema doc.
	if got, want := len(Capabilities), 13; got != want {
		t.Errorf("Capabilities len = %d, want %d", got, want)
	}
}

func TestProductsByVendorPartition(t *testing.T) {
	groups := ProductsByVendor()
	cases := map[string]int{
		"volc":      3,
		"aliyun":    3,
		"tencent":   3,
		"openai":    1,
		"anthropic": 1,
		"deepseek":  1,
	}
	for vendor, want := range cases {
		if got := len(groups[vendor]); got != want {
			t.Errorf("vendor %q: got %d products, want %d", vendor, got, want)
		}
	}
}

func TestProductAllowsCapability(t *testing.T) {
	if !ProductAllowsCapability("volc.ark", "chat") {
		t.Errorf("volc.ark should allow chat")
	}
	if ProductAllowsCapability("volc.ark", "asr_realtime") {
		t.Errorf("volc.ark should not allow asr_realtime (that's volc.speech)")
	}
	if ProductAllowsCapability("nonexistent", "chat") {
		t.Errorf("unknown product should not allow anything")
	}
}

func TestCapabilityCategories(t *testing.T) {
	// Spot-check a few representative mappings.
	cases := map[string]string{
		"chat":            "llm",
		"vision":          "llm",
		"asr_realtime":    "asr",
		"tts_voice_clone": "tts",
		"mt_document":     "mt",
	}
	for capID, wantCat := range cases {
		c, ok := Capabilities[capID]
		if !ok {
			t.Errorf("capability %q missing", capID)
			continue
		}
		if c.CategoryID != wantCat {
			t.Errorf("capability %q category = %q, want %q", capID, c.CategoryID, wantCat)
		}
	}
}
