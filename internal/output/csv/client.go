package csv

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/robbiebyrd/cantou/internal/client/common"
	canModels "github.com/robbiebyrd/cantou/internal/models"
)

// csvFlushInterval caps how long a buffered row can sit in memory before the
// underlying csv.Writer is flushed to disk. Short enough to bound data loss
// on abrupt termination, long enough that high-rate bursts benefit from
// buffered writes.
const csvFlushInterval = 1 * time.Second

type CSVClient struct {
	w              *csv.Writer
	file           *os.File
	signalWriter   *csv.Writer
	signalFile     *os.File
	includeHeaders bool
	canChannel     chan canModels.CanMessageTimestamped
	signalChannel  chan canModels.CanSignalTimestamped
	filters        *common.FilterSet
	l              *slog.Logger
	resolver       canModels.InterfaceResolver
	canMsgCount    atomic.Uint64
	signalMsgCount atomic.Uint64
}

func NewClient(
	ctx context.Context,
	cfg *canModels.Config,
	logger *slog.Logger,
	resolver canModels.InterfaceResolver,
) (canModels.OutputClient, error) {
	var (
		canFile      *os.File
		canWriter    *csv.Writer
		signalFile   *os.File
		signalWriter *csv.Writer
	)

	if cfg.CSVLog.CanOutputFile != "" {
		f, err := os.OpenFile(cfg.CSVLog.CanOutputFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return nil, fmt.Errorf("opening CSV CAN output file: %w", err)
		}
		canFile = f
		canWriter = csv.NewWriter(f)
		if cfg.CSVLog.IncludeHeaders {
			header := []string{"timestamp", "id", "interface", "remote", "transmit", "length", "data"}
			if err = canWriter.Write(header); err != nil {
				logger.Error("csv write CAN header error", "error", err)
			}
		}
	}

	if cfg.CSVLog.SignalOutputFile != "" {
		f, err := os.OpenFile(cfg.CSVLog.SignalOutputFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return nil, fmt.Errorf("opening CSV signal output file: %w", err)
		}
		signalFile = f
		signalWriter = csv.NewWriter(f)
		if cfg.CSVLog.IncludeHeaders {
			header := []string{"timestamp", "interface", "message", "signal", "value", "unit"}
			if err = signalWriter.Write(header); err != nil {
				logger.Error("csv write signal header error", "error", err)
			}
		}
	}

	return &CSVClient{
		w:              canWriter,
		file:           canFile,
		signalWriter:   signalWriter,
		signalFile:     signalFile,
		includeHeaders: cfg.CSVLog.IncludeHeaders,
		canChannel:     make(chan canModels.CanMessageTimestamped, cfg.MessageBufferSize),
		signalChannel:  make(chan canModels.CanSignalTimestamped, cfg.MessageBufferSize),
		filters:        common.NewFilterSet(),
		l:              logger,
		resolver:       resolver,
	}, nil
}

func (c *CSVClient) AddFilter(name string, filter canModels.FilterInterface) error {
	c.l.Debug("creating new filter group", "filterName", name)
	return c.filters.Add(name, filter)
}

func (c *CSVClient) HandleCanMessage(canMsg canModels.CanMessageTimestamped) {
	if c.w == nil {
		return
	}
	if shouldFilter, _ := c.filters.ShouldFilter(canMsg); shouldFilter {
		return
	}

	interfaceName := ""
	if conn := c.resolver.ConnectionByID(canMsg.Interface); conn != nil {
		interfaceName = conn.GetInterfaceName()
	}
	row := []string{
		strconv.FormatInt(canMsg.Timestamp, 10),
		strconv.FormatUint(uint64(canMsg.ID), 10),
		interfaceName,
		strconv.FormatBool(canMsg.Remote),
		strconv.FormatBool(canMsg.Transmit),
		strconv.Itoa(int(canMsg.Length)),
		hex.EncodeToString(canMsg.Data)}
	if err := c.w.Write(row); err != nil {
		c.l.Error("csv write error", "error", err)
	}
}

func (c *CSVClient) HandleCanMessageChannel() error {
	if c.file != nil {
		defer c.file.Close()
	}
	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "can", &c.canMsgCount, func() int { return len(c.canChannel) }, 5*time.Second)

	ticker := time.NewTicker(csvFlushInterval)
	defer ticker.Stop()

	flush := func() {
		if c.w == nil {
			return
		}
		c.w.Flush()
		if err := c.w.Error(); err != nil {
			c.l.Error("csv flush error", "error", err)
		}
	}

	for {
		select {
		case canMsg, ok := <-c.canChannel:
			if !ok {
				flush()
				return nil
			}
			c.canMsgCount.Add(1)
			c.HandleCanMessage(canMsg)
		case <-ticker.C:
			flush()
		}
	}
}

func (c *CSVClient) GetChannel() chan canModels.CanMessageTimestamped {
	return c.canChannel
}

func (c *CSVClient) GetSignalChannel() chan canModels.CanSignalTimestamped {
	return c.signalChannel
}

func (c *CSVClient) HandleSignal(sig canModels.CanSignalTimestamped) {
	if c.signalWriter == nil {
		return
	}

	interfaceName := ""
	if conn := c.resolver.ConnectionByID(sig.Interface); conn != nil {
		interfaceName = conn.GetInterfaceName()
	}
	row := []string{
		strconv.FormatInt(sig.Timestamp, 10),
		interfaceName,
		sig.Message,
		sig.Signal,
		strconv.FormatFloat(sig.Value, 'f', -1, 64),
		sig.Unit,
	}
	if err := c.signalWriter.Write(row); err != nil {
		c.l.Error("csv signal write error", "error", err)
	}
}

func (c *CSVClient) HandleSignalChannel() error {
	if c.signalFile != nil {
		defer c.signalFile.Close()
	}
	done := make(chan struct{})
	defer close(done)
	common.StartThroughputReporter(done, c.l, c.GetName(), "signal", &c.signalMsgCount, func() int { return len(c.signalChannel) }, 5*time.Second)

	ticker := time.NewTicker(csvFlushInterval)
	defer ticker.Stop()

	flush := func() {
		if c.signalWriter == nil {
			return
		}
		c.signalWriter.Flush()
		if err := c.signalWriter.Error(); err != nil {
			c.l.Error("csv signal flush error", "error", err)
		}
	}

	for {
		select {
		case sig, ok := <-c.signalChannel:
			if !ok {
				flush()
				return nil
			}
			c.signalMsgCount.Add(1)
			c.HandleSignal(sig)
		case <-ticker.C:
			flush()
		}
	}
}

func (c *CSVClient) GetName() string {
	return "output-csv"
}
