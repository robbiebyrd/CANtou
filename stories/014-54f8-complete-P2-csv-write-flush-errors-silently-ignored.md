---
id: 014-54f8
status: complete
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T20:57:46.261Z"
---
# CSV Write and Flush errors silently ignored on every message

## Description
Both `c.w.Write(row)` and `c.w.Flush()` return errors that are never checked. Disk-full or broken file-descriptor conditions cause silent data loss with no indication in logs.

## Acceptance Criteria
- [ ] `Write` error is checked and logged
- [ ] `w.Error()` is checked after `Flush` and logged
- [ ] The header write in `NewClient` also checks and logs its error

## Context Files
- `internal/output/csv/client.go:56-66` — `Handle()` with unchecked Write/Flush

## Work Log

### 2026-04-16T20:57:46.214Z - Fixed: log Write/Flush errors in Handle() using c.l.Error with structured key-value args

