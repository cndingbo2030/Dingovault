# Changelog

All notable user-facing changes for Dingovault are listed here. We describe what you gain in daily use, not internal implementation details.

## v1.3.0 — AI writing & smart links (2026-04-10)

### Real-time AI writing

See the assistant’s words appear as they are composed when you use inline AI on a bullet. The experience feels like a collaborator typing beside you instead of waiting for a single block of text at the end.

### Smarter answers from your vault

Ask questions in the AI chat sidebar and get answers that take the **current page** and **related notes** into account. Dingovault surfaces passages from elsewhere in your vault that are genuinely similar in meaning—even when you never linked them—so follow-ups and research stay grounded in what you already wrote.

### Instant brain link

The **Semantically related** panel suggests other blocks that match the spirit of what you are reading. It helps you rediscover past notes, connect ideas, and avoid duplicate work without manually hunting through filenames.

### Tag suggestions that understand content

When you edit a block, suggested **#tags** reflect how your note reads, not just spelling. Picking one is a fast way to keep tagging consistent across the vault.

### Graph: meaning, not only links

The page graph can show **semantic connections** between notes—visual hints for “these pages belong together” based on content similarity, alongside classic wikilink edges.

### Calmer when the AI server drops

If the local model or API stops mid-stream, the app shows a clear **connection lost** message, restores your text, and stops spinning—instead of leaving the editor stuck.

### Under the hood (for the curious)

- Stress-tested concurrent search while hundreds of embeddings are written, to keep the database responsive during heavy indexing.
- Tighter code structure across AI, search, and graph paths for long-term stability.

---

## Earlier releases

See [GitHub Releases](https://github.com/cndingbo2030/dingovault/releases) for prior binaries and notes.
