package bus

// Well-known topics for graph / vault lifecycle (extend freely for plugins).
const (
	TopicFileReindexed = "file.reindexed"
	TopicBlockUpdated  = "block.updated"
	// TopicAfterBlockIndexed fires after SQLite index replace for a file (payload: AfterBlockIndexedPayload).
	TopicAfterBlockIndexed = "after:block:indexed"
)

// FileReindexedPayload is emitted after a source file is parsed and written to SQLite.
type FileReindexedPayload struct {
	Path string `json:"path"`
}

// BlockUpdatedPayload is emitted after a surgical block edit and re-index.
type BlockUpdatedPayload struct {
	BlockID string `json:"blockId"`
	Path    string `json:"path"`
}

// AfterBlockIndexedPayload is emitted after blocks for a source file are written to the index.
type AfterBlockIndexedPayload struct {
	SourcePath string `json:"sourcePath"`
	BlockCount int    `json:"blockCount"`
}
