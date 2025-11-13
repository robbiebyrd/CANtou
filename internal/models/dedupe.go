package models

type Deduper interface {
	Enabled() bool
	Enable()
	Disable()
	Filter(canMsg CanMessageTimestamped) bool
}
