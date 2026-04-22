package crtd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	canModels "github.com/robbiebyrd/cantou/internal/models"
)

// WriteHeader writes the standard CRTD file header to w, flushing after the
// last line. Each write error is returned immediately so the caller knows
// exactly which line failed.
func WriteHeader(w *bufio.Writer, cfg *canModels.Config) error {
	if _, err := fmt.Fprintln(w, "0.000000 CXX CRTD file created by cantou"); err != nil {
		return fmt.Errorf("writing CRTD header line: %w", err)
	}
	for index, canInterface := range cfg.CanInterfaces {
		_, err := fmt.Fprintf(
			w,
			"0.000000 CXX Info Type:'interface'; ID:'%d'; Name:'%s'; URI:'%s'; Network:'%s'; DBC:'%s';\n",
			index,
			canInterface.Name,
			canInterface.URI,
			canInterface.Network,
			strings.Join(canInterface.DBCFiles, ","),
		)
		if err != nil {
			return fmt.Errorf("writing CRTD interface header: %w", err)
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("flushing CRTD header: %w", err)
	}
	return nil
}

// CANWriter writes CAN message records in CRTD format.
type CANWriter struct {
	w    *bufio.Writer
	file *os.File
}

// NewCANWriter opens path for writing, writes the CRTD file header, and
// returns a writer ready to accept CAN records. File-open and header-write
// failures are both returned as errors.
func NewCANWriter(path string, cfg *canModels.Config) (*CANWriter, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening CRTD output file: %w", err)
	}
	w := bufio.NewWriter(f)
	if err := WriteHeader(w, cfg); err != nil {
		f.Close()
		return nil, err
	}
	return &CANWriter{w: w, file: f}, nil
}

// Append writes one CRTD CAN record. The line format is:
//
//	<seconds>.<microseconds> <interfaceID><recordType> <hexID> <hex bytes...>
func (cw *CANWriter) Append(timestamp int64, interfaceID int, id uint32, transmit bool, data []byte) error {
	seconds := timestamp / 1e9
	microseconds := (timestamp % 1e9) / 1e3

	recordType := "R"
	if transmit {
		recordType = "T"
	}
	if id > 0x7FF {
		recordType += "29"
	} else {
		recordType += "11"
	}

	dataBytes := make([]string, len(data))
	for i, b := range data {
		dataBytes[i] = fmt.Sprintf("%02X", b)
	}

	line := fmt.Sprintf("%d.%06d %d%s %X %s",
		seconds, microseconds,
		interfaceID, recordType, id,
		strings.Join(dataBytes, " "))
	_, err := fmt.Fprintln(cw.w, line)
	return err
}

// Flush flushes buffered data to the underlying file.
func (cw *CANWriter) Flush() error {
	return cw.w.Flush()
}

// Close flushes and closes the underlying file. Safe to call more than once.
func (cw *CANWriter) Close() error {
	if cw.file == nil {
		return nil
	}
	defer func() { cw.file = nil }()
	if err := cw.w.Flush(); err != nil {
		cw.file.Close()
		return err
	}
	return cw.file.Close()
}

// SignalWriter writes decoded signal records in CRTD format.
type SignalWriter struct {
	w    *bufio.Writer
	file *os.File
}

// NewSignalWriter opens path for writing and returns a writer ready to accept
// signal records. No header is written to the signal file.
func NewSignalWriter(path string) (*SignalWriter, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening CRTD signal output file: %w", err)
	}
	return &SignalWriter{w: bufio.NewWriter(f), file: f}, nil
}

// Append writes one CRTD signal record. The line format is:
//
//	<seconds>.<microseconds> <interfaceID>SIG <message>/<signal> <value> <unit>
func (sw *SignalWriter) Append(timestamp int64, interfaceID int, message, signal string, value float64, unit string) error {
	seconds := timestamp / 1e9
	microseconds := (timestamp % 1e9) / 1e3

	line := fmt.Sprintf("%d.%06d %dSIG %s/%s %s %s",
		seconds, microseconds,
		interfaceID, message, signal,
		strconv.FormatFloat(value, 'f', -1, 64),
		unit,
	)
	_, err := fmt.Fprintln(sw.w, line)
	return err
}

// Flush flushes buffered data to the underlying file.
func (sw *SignalWriter) Flush() error {
	return sw.w.Flush()
}

// Close flushes and closes the underlying file. Safe to call more than once.
func (sw *SignalWriter) Close() error {
	if sw.file == nil {
		return nil
	}
	defer func() { sw.file = nil }()
	if err := sw.w.Flush(); err != nil {
		sw.file.Close()
		return err
	}
	return sw.file.Close()
}
