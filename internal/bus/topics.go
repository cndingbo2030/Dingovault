package bus

// Well-known topics for graph / vault lifecycle (extend freely for plugins).
const (
	TopicFileReindexed = "file.reindexed"
	TopicBlockUpdated  = "block.updated"
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
