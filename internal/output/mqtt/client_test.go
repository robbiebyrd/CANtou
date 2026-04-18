package mqtt

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

// stubFilter is a minimal FilterInterface implementation that never filters any message.
type stubFilter struct{}

func (s *stubFilter) Add(_ canModels.CanMessageFilter) error        { return nil }
func (s *stubFilter) Filter(_ canModels.CanMessageTimestamped) bool { return false }
func (s *stubFilter) Mode(_ canModels.CanFilterGroupOperator)       {}

// TestHandleCanMessageChannel_DrainAndReturn verifies the channel handler
// returns nil once the incoming channel is closed with no pending messages.
func TestHandleCanMessageChannel_DrainAndReturn(t *testing.T) {
	c := &MQTTClient{
		l:               slog.Default(),
		ctx:             context.Background(),
		incomingChannel: make(chan canModels.CanMessageTimestamped),
		filters:         make(map[string]canModels.FilterInterface),
		resolver:        &mockResolver{conns: map[int]*mockCanConn{}},
	}

	close(c.incomingChannel)

	err := c.HandleCanMessageChannel()
	assert.NoError(t, err)
}

// TestHandleSignalChannel_DrainAndReturn verifies the signal channel handler
// returns nil once the signal channel is closed with no pending signals.
func TestHandleSignalChannel_DrainAndReturn(t *testing.T) {
	c := &MQTTClient{
		l:             slog.Default(),
		ctx:           context.Background(),
		signalChannel: make(chan canModels.CanSignalTimestamped),
		filters:       make(map[string]canModels.FilterInterface),
		resolver:      &mockResolver{conns: map[int]*mockCanConn{}},
	}

	close(c.signalChannel)

	err := c.HandleSignalChannel()
	assert.NoError(t, err)
}

// TestAddFilter_HappyPath verifies a new filter is accepted and stored.
func TestAddFilter_HappyPath(t *testing.T) {
	c := &MQTTClient{
		l:       slog.Default(),
		filters: make(map[string]canModels.FilterInterface),
	}

	err := c.AddFilter("my-filter", &stubFilter{})
	require.NoError(t, err)
	assert.Contains(t, c.filters, "my-filter")
}

// TestAddFilter_DuplicateRejected verifies registering the same filter name twice returns an error.
func TestAddFilter_DuplicateRejected(t *testing.T) {
	c := &MQTTClient{
		l:       slog.Default(),
		filters: make(map[string]canModels.FilterInterface),
	}

	require.NoError(t, c.AddFilter("dup", &stubFilter{}))

	err := c.AddFilter("dup", &stubFilter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dup")
}
