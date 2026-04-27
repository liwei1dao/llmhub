package domain

// TranslateRequest is the unified text translation request.
type TranslateRequest struct {
	Texts      []string
	SourceLang string // "auto" for detection
	TargetLang string
	Engine     string // "mt" / "llm"
	GlossaryID string
	PreserveFormatting bool
}

// TranslateResponse is the unified translation response.
type TranslateResponse struct {
	Translations []TranslateItem
	Engine       string
	CharsBilled  int
}

// TranslateItem is a single translated result.
type TranslateItem struct {
	Text           string
	DetectedSource string
}
