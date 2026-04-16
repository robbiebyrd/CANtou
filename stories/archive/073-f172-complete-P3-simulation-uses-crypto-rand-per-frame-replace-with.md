---
id: 073-f172
title: Simulation uses crypto/rand per frame — replace with math/rand
status: complete
priority: P3
created: "2026-04-16T20:32:52.685Z"
updated: "2026-04-16T20:59:35.637Z"
dependencies: []
---

# Simulation uses crypto/rand per frame — replace with math/rand

## Problem Statement

The simulation client calls cryptoRand.Read(randomBytes) and allocates a fresh byte slice on every frame. Cryptographic randomness is not needed for simulation.

## Acceptance Criteria

- [ ] Byte slice allocated once outside the loop and reused
- [ ] cryptoRand replaced with mathRand

## Files

- internal/connection/simulate/connection.go

## Work Log

### 2026-04-16T20:59:35.594Z - Fixed: see implementation

