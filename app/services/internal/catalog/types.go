package catalog

// FieldSpec describes one field in a credential schema (either the
// master-account auth for a vendor, or the per-product credential).
//
// FieldSpec is purely declarative metadata — it drives the admin UI
// (label / sensitive masking / validation hint) and the backend
// (which keys to expect in the auth payload before it is written to
// vault). The actual credential values never live on this struct.
type FieldSpec struct {
	// Key is the JSON key used in the credential payload (e.g.
	// "app_id", "secret_access_key"). Stable identifier, do not rename
	// without a migration.
	Key string `json:"key"`
	// Label is the Chinese display name shown in the admin UI form.
	Label string `json:"label"`
	// Sensitive marks fields whose values must be written to vault and
	// rendered as masked (•••) in any UI. Examples: tokens, secrets,
	// passwords. Non-sensitive fields (region, app_id) may be shown
	// in plain text in admin panels.
	Sensitive bool `json:"sensitive,omitempty"`
	// Required marks fields the user must fill before submit. Optional
	// fields default to empty / vendor default.
	Required bool `json:"required,omitempty"`
	// Pattern is an optional regex hint for client-side validation.
	// Empty = no check.
	Pattern string `json:"pattern,omitempty"`
}
