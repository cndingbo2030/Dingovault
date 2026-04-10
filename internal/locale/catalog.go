package locale

// catalog holds backend-facing user messages (Wails errors, toasts originating from Go).
var catalog = map[string]map[string]string{
	"en": {
		ErrStoreNotInit:       "Store is not initialized.",
		ErrGraphNotInit:       "Graph engine is not initialized.",
		ErrNotesRootNotSet:    "Notes root is not set.",
		ErrThemeInvalid:       "Theme must be light or dark.",
		ErrResolvePath:        "Could not resolve path.",
		ErrNotMarkdown:        "Not a Markdown file.",
		ErrListBlocks:         "Could not load blocks for this page.",
		ErrReadPage:           "Could not read page file.",
		ErrWriteExport:        "Could not write export file.",
		ErrLocaleUnsupported:  "Language must be en or zh-CN.",
		ErrAIKeyRequired:      "OpenAI provider requires an API key.",
		ErrAIEmptyMessage:     "Message is empty.",
		ErrAIEmptyInstruction: "Instruction is empty.",
	},
	"zh-CN": {
		ErrStoreNotInit:       "存储未初始化。",
		ErrGraphNotInit:       "图谱引擎未初始化。",
		ErrNotesRootNotSet:    "未设置笔记根目录。",
		ErrThemeInvalid:       "主题只能是浅色或深色。",
		ErrResolvePath:        "无法解析路径。",
		ErrNotMarkdown:        "不是 Markdown 文件。",
		ErrListBlocks:         "无法加载此页面的块。",
		ErrReadPage:           "无法读取页面文件。",
		ErrWriteExport:        "无法写入导出文件。",
		ErrLocaleUnsupported:  "语言只能是 en 或 zh-CN。",
		ErrAIKeyRequired:      "使用 OpenAI 时需要填写 API 密钥。",
		ErrAIEmptyMessage:     "消息为空。",
		ErrAIEmptyInstruction: "指令为空。",
	},
}
