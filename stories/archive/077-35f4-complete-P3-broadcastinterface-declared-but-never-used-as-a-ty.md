---
id: 077-35f4
title: BroadcastInterface declared but never used as a type — YAGNI
status: complete
priority: P3
created: "2026-04-16T20:32:52.687Z"
updated: "2026-04-16T20:57:25.233Z"
dependencies: []
---

# BroadcastInterface declared but never used as a type — YAGNI

## Problem Statement

BroadcastInterface is defined but BroadcastClient is always used as *BroadcastClient directly. One implementation, zero usage as a type. Premature abstraction.

## Acceptance Criteria

- [ ] BroadcastInterface deleted from broadcast.go

## Files

- internal/client/broadcast/broadcast.go

## Work Log

### 2026-04-16T20:57:25.186Z - Fixed: see implementation

