package ai

import "strings"

// BuildInlineRefactorMessages builds chat messages for a single-block edit: output should be
// only the replacement bullet text (no markdown fences, no preamble).
func BuildInlineRefactorMessages(blockContent, instruction string) []ChatMessage {
	inst := strings.TrimSpace(instruction)
	body := strings.TrimSpace(blockContent)
	sys := "You are Dingovault inline editor. The user has one outline bullet (Markdown list line). " +
		"Follow their instruction precisely. Reply with ONLY the new bullet text: one line or a short multi-line block " +
		"that fits the outline. Do not wrap in code fences. Do not add 'Here is' or explanations."
	user := "Instruction:\n" + inst + "\n\nCurrent bullet:\n" + body
	return []ChatMessage{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	}
}
