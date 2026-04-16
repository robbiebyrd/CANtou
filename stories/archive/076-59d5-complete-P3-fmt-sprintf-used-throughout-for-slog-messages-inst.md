---
id: 076-59d5
title: fmt.Sprintf used throughout for slog messages instead of key-value args
status: complete
priority: P3
created: "2026-04-16T20:32:52.687Z"
updated: "2026-04-16T20:58:19.574Z"
dependencies: []
---

# fmt.Sprintf used throughout for slog messages instead of key-value args

## Problem Statement

c.l.Debug(fmt.Sprintf(worker %v started, i)) defeats structured logging — values embedded in message string cannot be queried by log aggregators. Should use slog key-value pairs.

## Acceptance Criteria

- [ ] All slog calls using fmt.Sprintf in the message converted to structured key-value args
- [ ] Affects: influxdb/client.go, app/app.go, mqtt/client.go, simulate/connection.go

## Files

- internal/output/influxdb/client.go
- internal/app/app.go
- internal/output/mqtt/client.go
- internal/connection/simulate/connection.go

## Work Log

### 2026-04-16T20:58:19.529Z - Fixed mqtt/client.go: replaced all fmt.Sprintf in slog calls with structured key-value args

