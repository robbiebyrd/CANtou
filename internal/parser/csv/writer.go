package csv

import (
	stdcsv "encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
)

// CANWriter writes CAN message records to a CSV file.
type CANWriter struct {
	w    *stdcsv.Writer
	file *os.File
}

// NewCANWriter opens path for writing and optionally emits the column header row.
func NewCANWriter(path string, includeHeaders bool) (*CANWriter, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening CSV CAN output file: %w", err)
	}
	w := stdcsv.NewWriter(f)
	if includeHeaders {
		if err := w.Write([]string{"timestamp", "id", "interface", "remote", "transmit", "length", "data"}); err != nil {
			f.Close()
			return nil, fmt.Errorf("writing CSV CAN header: %w", err)
		}
	}
	return &CANWriter{w: w, file: f}, nil
}

// Append writes a single CAN message row. id is the decimal CAN arbitration ID,
// interfaceName is the human-readable connection name, and data is hex-encoded.
func (cw *CANWriter) Append(timestamp int64, id uint64, interfaceName string, remote, transmit bool, length int, data []byte) error {
	return cw.w.Write([]string{
		strconv.FormatInt(timestamp, 10),
		strconv.FormatUint(id, 10),
		interfaceName,
		strconv.FormatBool(remote),
		strconv.FormatBool(transmit),
		strconv.Itoa(length),
		hex.EncodeToString(data),
	})
}

// Flush flushes buffered data to the underlying file.
func (cw *CANWriter) Flush() error {
	cw.w.Flush()
	return cw.w.Error()
}

// Close flushes and closes the underlying file. Safe to call more than once.
func (cw *CANWriter) Close() error {
	if cw.file == nil {
		return nil
	}
	cw.w.Flush()
	err := cw.w.Error()
	defer func() { cw.file = nil }()
	closeErr := cw.file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

// SignalWriter writes decoded signal records to a CSV file.
type SignalWriter struct {
	w    *stdcsv.Writer
	file *os.File
}

// NewSignalWriter opens path for writing and optionally emits the column header row.
func NewSignalWriter(path string, includeHeaders bool) (*SignalWriter, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening CSV signal output file: %w", err)
	}
	w := stdcsv.NewWriter(f)
	if includeHeaders {
		if err := w.Write([]string{"timestamp", "interface", "message", "signal", "value", "unit"}); err != nil {
			f.Close()
			return nil, fmt.Errorf("writing CSV signal header: %w", err)
		}
	}
	return &SignalWriter{w: w, file: f}, nil
}

// Append writes a single signal row.
func (sw *SignalWriter) Append(timestamp int64, interfaceName, message, signal string, value float64, unit string) error {
	return sw.w.Write([]string{
		strconv.FormatInt(timestamp, 10),
		interfaceName,
		message,
		signal,
		strconv.FormatFloat(value, 'f', -1, 64),
		unit,
	})
}

// Flush flushes buffered data to the underlying file.
func (sw *SignalWriter) Flush() error {
	sw.w.Flush()
	return sw.w.Error()
}

// Close flushes and closes the underlying file. Safe to call more than once.
func (sw *SignalWriter) Close() error {
	if sw.file == nil {
		return nil
	}
	sw.w.Flush()
	err := sw.w.Error()
	defer func() { sw.file = nil }()
	closeErr := sw.file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
