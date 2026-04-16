---
id: 021-680e
status: complete
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T20:57:49.600Z"
---
# defer writer.Flush() in CSV NewClient runs when constructor returns, not on close

## Description
`defer writer.Flush()` in `NewClient` runs when `NewClient` returns, not when the CSV client is closed. This is misleading and there is no `Close()` method on `CSVClient`. The header row happens to flush correctly due to this timing, but the pattern is confusing and fragile.

## Acceptance Criteria
- [ ] Remove the `defer`; call `writer.Flush()` explicitly after writing the header
- [ ] Check and handle the Flush error

## Context Files
- `internal/output/csv/client.go:32-44` — constructor with misplaced defer

## Work Log

### 2026-04-16T20:57:49.555Z - Fixed: removed defer writer.Flush() from constructor; added immediate writer.Flush() after writing header row

