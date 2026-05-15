package catalog

import (
	"fmt"
	"strings"
)

// init enforces cross-dictionary consistency at process start. Any
// failure here is a programmer error in the static catalog tables —
// the binary refuses to come up rather than masking a misconfiguration.
//
// Concretely it rejects:
//   - duplicate map key vs ID drift,
//   - product → vendor that doesn't exist,
//   - capability → category that doesn't exist,
//   - product.AllowedCapabilities referencing unknown capability ids,
//   - empty required identifiers.
func init() {
	if err := Validate(); err != nil {
		panic("catalog: invariant violated at startup: " + err.Error())
	}
}

// Validate runs the consistency checks performed by init() in a way
// that tests can call without triggering a process-level panic.
func Validate() error {
	// ── categories ────────────────────────────────────────────
	for k, v := range Categories {
		if k != v.ID {
			return fmt.Errorf("category map key %q != ID %q", k, v.ID)
		}
		if v.ID == "" || v.Name == "" {
			return fmt.Errorf("category %q has empty id or name", k)
		}
	}
	// ── vendors ───────────────────────────────────────────────
	for k, v := range Vendors {
		if k != v.ID {
			return fmt.Errorf("vendor map key %q != ID %q", k, v.ID)
		}
		if v.ID == "" || v.Name == "" {
			return fmt.Errorf("vendor %q has empty id or name", k)
		}
		if strings.ContainsRune(v.ID, '.') {
			return fmt.Errorf("vendor id %q must not contain '.'", v.ID)
		}
		if len(v.MasterAuthSchema) == 0 {
			return fmt.Errorf("vendor %q must declare at least one master-auth field", v.ID)
		}
	}
	// ── capabilities ──────────────────────────────────────────
	for k, c := range Capabilities {
		if k != c.ID {
			return fmt.Errorf("capability map key %q != ID %q", k, c.ID)
		}
		if c.BillingUnit == "" {
			return fmt.Errorf("capability %q has empty billing_unit", c.ID)
		}
		if _, ok := Categories[c.CategoryID]; !ok {
			return fmt.Errorf("capability %q references unknown category %q", c.ID, c.CategoryID)
		}
	}
	// ── products ──────────────────────────────────────────────
	for k, p := range Products {
		if k != p.ID {
			return fmt.Errorf("product map key %q != ID %q", k, p.ID)
		}
		// Composite IDs must use the "<vendor>.<board>" form so the
		// vendor prefix is recoverable from the id alone.
		if !strings.HasPrefix(p.ID, p.VendorID+".") {
			return fmt.Errorf("product %q must be prefixed with vendor id %q", p.ID, p.VendorID)
		}
		if _, ok := Vendors[p.VendorID]; !ok {
			return fmt.Errorf("product %q references unknown vendor %q", p.ID, p.VendorID)
		}
		if len(p.CredentialSchema) == 0 {
			return fmt.Errorf("product %q must declare at least one credential field", p.ID)
		}
		if len(p.AllowedCapabilities) == 0 {
			return fmt.Errorf("product %q must allow at least one capability", p.ID)
		}
		seen := make(map[string]struct{}, len(p.AllowedCapabilities))
		for _, capID := range p.AllowedCapabilities {
			if _, ok := Capabilities[capID]; !ok {
				return fmt.Errorf("product %q references unknown capability %q", p.ID, capID)
			}
			if _, dup := seen[capID]; dup {
				return fmt.Errorf("product %q lists capability %q twice", p.ID, capID)
			}
			seen[capID] = struct{}{}
		}
	}
	// ── service modules ───────────────────────────────────────
	for k, m := range Modules {
		if k != m.ID {
			return fmt.Errorf("module map key %q != ID %q", k, m.ID)
		}
		// ID 必须 = "<vendor_product>.<capability>"，否则反推 (product,
		// capability) 就要查表，违背 ID 命名约定的初衷。
		want := m.VendorProductID + "." + m.Capability
		if m.ID != want {
			return fmt.Errorf("module %q ID must equal %q", m.ID, want)
		}
		if _, ok := Products[m.VendorProductID]; !ok {
			return fmt.Errorf("module %q references unknown product %q", m.ID, m.VendorProductID)
		}
		if _, ok := Capabilities[m.Capability]; !ok {
			return fmt.Errorf("module %q references unknown capability %q", m.ID, m.Capability)
		}
		if !ProductAllowsCapability(m.VendorProductID, m.Capability) {
			return fmt.Errorf("module %q: capability %q not allowed by product %q",
				m.ID, m.Capability, m.VendorProductID)
		}
		// 模块的 DefaultBillingUnit 必须和 Capability.BillingUnit 对齐，
		// 否则 admin 创建的 SKU 计费单位会和能力声明冲突。
		if cap, ok := Capabilities[m.Capability]; ok && m.DefaultBillingUnit != cap.BillingUnit {
			return fmt.Errorf("module %q DefaultBillingUnit %q != capability %q's %q",
				m.ID, m.DefaultBillingUnit, m.Capability, cap.BillingUnit)
		}
		// 模型 ID 在同一模块内必须唯一。
		seenModels := make(map[string]struct{}, len(m.AvailableModels))
		for _, mo := range m.AvailableModels {
			if mo.ID == "" || mo.UpstreamModel == "" || mo.DisplayName == "" {
				return fmt.Errorf("module %q has model with empty id / upstream / display_name", m.ID)
			}
			if _, dup := seenModels[mo.ID]; dup {
				return fmt.Errorf("module %q lists model %q twice", m.ID, mo.ID)
			}
			seenModels[mo.ID] = struct{}{}
		}
		seenRegions := make(map[string]struct{}, len(m.AvailableRegions))
		defaultCount := 0
		for _, ro := range m.AvailableRegions {
			if ro.ID == "" || ro.Endpoint == "" {
				return fmt.Errorf("module %q has region with empty id / endpoint", m.ID)
			}
			if _, dup := seenRegions[ro.ID]; dup {
				return fmt.Errorf("module %q lists region %q twice", m.ID, ro.ID)
			}
			seenRegions[ro.ID] = struct{}{}
			if ro.Default {
				defaultCount++
			}
		}
		if defaultCount > 1 {
			return fmt.Errorf("module %q has more than one default region", m.ID)
		}
	}
	return nil
}

// LookupVendor returns the vendor by id along with a found-flag.
func LookupVendor(id string) (Vendor, bool) { v, ok := Vendors[id]; return v, ok }

// LookupProduct returns the product by id along with a found-flag.
func LookupProduct(id string) (VendorProduct, bool) { p, ok := Products[id]; return p, ok }

// LookupCapability returns the capability by id along with a found-flag.
func LookupCapability(id string) (Capability, bool) { c, ok := Capabilities[id]; return c, ok }

// LookupCategory returns the category by id along with a found-flag.
func LookupCategory(id string) (PlatformCategory, bool) { c, ok := Categories[id]; return c, ok }

// ProductAllowsCapability reports whether capability capID may be
// bound on product productID. False is returned both for unknown
// products and for capabilities outside the product's allow-list.
func ProductAllowsCapability(productID, capID string) bool {
	p, ok := Products[productID]
	if !ok {
		return false
	}
	for _, c := range p.AllowedCapabilities {
		if c == capID {
			return true
		}
	}
	return false
}

// ProductsByVendor groups all known products by their vendor id. Each
// known vendor always appears as a key (possibly with an empty slice)
// so callers don't have to nil-check vendor lookup. Order within a
// vendor's slice follows Go map iteration (i.e. unstable) — callers
// should sort if they care.
func ProductsByVendor() map[string][]VendorProduct {
	out := make(map[string][]VendorProduct, len(Vendors))
	for vid := range Vendors {
		out[vid] = nil
	}
	for _, p := range Products {
		out[p.VendorID] = append(out[p.VendorID], p)
	}
	return out
}
