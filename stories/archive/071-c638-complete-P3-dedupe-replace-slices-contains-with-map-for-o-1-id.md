---
id: 071-c638
title: "Dedupe: replace slices.Contains with map for O(1) ID lookup"
status: complete
priority: P3
created: "2026-04-16T20:32:52.682Z"
updated: "2026-04-16T20:57:32.649Z"
dependencies: []
---

# Dedupe: replace slices.Contains with map for O(1) ID lookup

## Problem Statement

slices.Contains(dc.ids, canMsg.ID) performs a linear scan on every message. With many watched IDs this scales poorly.

## Acceptance Criteria

- [ ] ids []uint32 replaced with ids map[uint32]struct{}
- [ ] NewDedupeFilterClient constructs the map from the input slice
- [ ] Filter() uses map lookup

## Files

- internal/client/dedupe/dedupe.go

## Work Log

### 2026-04-16T20:57:32.605Z - Fixed: see implementation

