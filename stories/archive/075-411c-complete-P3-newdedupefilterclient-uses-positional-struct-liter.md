---
id: 075-411c
title: NewDedupeFilterClient uses positional struct literal — fragile
status: complete
priority: P3
created: "2026-04-16T20:32:52.686Z"
updated: "2026-04-16T20:57:32.774Z"
dependencies: []
---

# NewDedupeFilterClient uses positional struct literal — fragile

## Problem Statement

return &DedupeFilterClient{make(map[uint64]time.Time), l, timeout, ids} uses positional initialization. Any field reorder silently initializes wrong fields.

## Acceptance Criteria

- [ ] Changed to named field initialization

## Files

- internal/client/dedupe/dedupe.go

## Work Log

### 2026-04-16T20:57:32.726Z - Fixed: see implementation

