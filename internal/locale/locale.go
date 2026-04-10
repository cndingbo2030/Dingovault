package locale

import "strings"

// Well-known message keys (bridge and UI-adjacent errors).
const (
	ErrStoreNotInit      = "err.store_not_init"
	ErrGraphNotInit      = "err.graph_not_init"
	ErrNotesRootNotSet   = "err.notes_root_not_set"
	ErrThemeInvalid      = "err.theme_invalid"
	ErrResolvePath       = "err.resolve_path"
	ErrNotMarkdown       = "err.not_markdown"
	ErrListBlocks        = "err.list_blocks"
	ErrReadPage          = "err.read_page"
	ErrWriteExport       = "err.write_export"
	ErrLocaleUnsupported = "err.locale_unsupported"
	ErrAIKeyRequired     = "err.ai_key_required"
	ErrAIEmptyMessage       = "err.ai_empty_message"
	ErrAIEmptyInstruction   = "err.ai_empty_instruction"
)

// Normalize maps BCP 47 and legacy tags to supported UI locales.
func Normalize(tag string) string {
	t := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(tag, "_", "-")))
	switch {
	case t == "":
		return "en"
	case strings.HasPrefix(t, "zh"):
		return "zh-CN"
	default:
		return "en"
	}
}

// Supported reports whether the normalized tag is persisted as its own catalog.
func Supported(normalized string) bool {
	return normalized == "en" || normalized == "zh-CN"
}

// Message returns a localized string for tag (e.g. en, zh-CN) and key, falling back to English then the key.
func Message(tag, key string) string {
	tag = Normalize(tag)
	if m, ok := catalog[tag]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	if s, ok := catalog["en"][key]; ok {
		return s
	}
	return key
}
