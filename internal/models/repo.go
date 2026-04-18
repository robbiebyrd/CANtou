package models

type OutputClient interface {
	HandleCanMessage(canMsg CanMessageTimestamped)
	HandleCanMessageChannel() error
	GetChannel() chan CanMessageTimestamped
	GetName() string
	AddFilter(name string, filter FilterInterface) error
}

// RunnerClient is satisfied by output clients that have a background Run()
// goroutine (e.g. InfluxDB's worker pool). app.go detects this via type
// assertion; clients without a Run() loop need not implement it.
type RunnerClient interface {
	Run() error
}

// SignalOutputClient extends OutputClient for clients that consume decoded DBC
// signals. app.go detects this interface via type assertion at wiring time;
// clients that only process raw CAN frames need not implement it.
type SignalOutputClient interface {
	OutputClient
	HandleSignal(signal CanSignalTimestamped)
	HandleSignalChannel() error
	GetSignalChannel() chan CanSignalTimestamped
}
