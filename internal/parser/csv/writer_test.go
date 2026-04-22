package csv

import (
	stdcsv "encoding/csv"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempPath(t *testing.T, suffix string) string {
	t.Helper()
	f, err := os.CreateTemp("", "csv_writer_test_*"+suffix)
	require.NoError(t, err)
	name := f.Name()
	require.NoError(t, f.Close())
	t.Cleanup(func() { os.Remove(name) })
	return name
}

func readRows(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	rows, err := stdcsv.NewReader(f).ReadAll()
	require.NoError(t, err)
	return rows
}

func TestNewCANWriter_NoHeaders(t *testing.T) {
	path := tempPath(t, ".csv")
	w, err := NewCANWriter(path, false)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	rows := readRows(t, path)
	assert.Empty(t, rows, "no header row expected when includeHeaders is false")
}

func TestNewCANWriter_WithHeaders(t *testing.T) {
	path := tempPath(t, ".csv")
	w, err := NewCANWriter(path, true)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	rows := readRows(t, path)
	require.Len(t, rows, 1)
	assert.Equal(t, []string{"timestamp", "id", "interface", "remote", "transmit", "length", "data"}, rows[0])
}

func TestCANWriter_Append_RowFormat(t *testing.T) {
	path := tempPath(t, ".csv")
	w, err := NewCANWriter(path, false)
	require.NoError(t, err)

	require.NoError(t, w.Append(1000000000, 0x1AB, "can0-can-vcan0", false, true, 4, []byte{0xDE, 0xAD, 0xBE, 0xEF}))
	require.NoError(t, w.Close())

	rows := readRows(t, path)
	require.Len(t, rows, 1)
	row := rows[0]
	assert.Equal(t, "1000000000", row[0], "timestamp")
	assert.Equal(t, "427", row[1], "id (decimal of 0x1AB)")
	assert.Equal(t, "can0-can-vcan0", row[2], "interface name")
	assert.Equal(t, "false", row[3], "remote")
	assert.Equal(t, "true", row[4], "transmit")
	assert.Equal(t, "4", row[5], "length")
	assert.Equal(t, "deadbeef", row[6], "data hex")
}

func TestCANWriter_Append_EmptyData(t *testing.T) {
	path := tempPath(t, ".csv")
	w, err := NewCANWriter(path, false)
	require.NoError(t, err)
	require.NoError(t, w.Append(0, 0x001, "", false, false, 0, []byte{}))
	require.NoError(t, w.Close())

	rows := readRows(t, path)
	require.Len(t, rows, 1)
	assert.Equal(t, "", rows[0][6], "empty data should produce empty hex string")
}

func TestCANWriter_Close_Idempotent(t *testing.T) {
	path := tempPath(t, ".csv")
	w, err := NewCANWriter(path, false)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, w.Close(), "second Close must be a no-op")
}

func TestCANWriter_BadPath(t *testing.T) {
	_, err := NewCANWriter("/no/such/dir/out.csv", false)
	assert.Error(t, err)
}

func TestNewSignalWriter_NoHeaders(t *testing.T) {
	path := tempPath(t, ".csv")
	w, err := NewSignalWriter(path, false)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	rows := readRows(t, path)
	assert.Empty(t, rows)
}

func TestNewSignalWriter_WithHeaders(t *testing.T) {
	path := tempPath(t, ".csv")
	w, err := NewSignalWriter(path, true)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	rows := readRows(t, path)
	require.Len(t, rows, 1)
	assert.Equal(t, []string{"timestamp", "interface", "message", "signal", "value", "unit"}, rows[0])
}

func TestSignalWriter_Append_RowFormat(t *testing.T) {
	path := tempPath(t, ".csv")
	w, err := NewSignalWriter(path, false)
	require.NoError(t, err)

	require.NoError(t, w.Append(1000000000, "can0-can-vcan0", "ENGINE", "RPM", 1500.5, "rpm"))
	require.NoError(t, w.Close())

	rows := readRows(t, path)
	require.Len(t, rows, 1)
	row := rows[0]
	assert.Equal(t, "1000000000", row[0], "timestamp")
	assert.Equal(t, "can0-can-vcan0", row[1], "interface")
	assert.Equal(t, "ENGINE", row[2], "message")
	assert.Equal(t, "RPM", row[3], "signal")
	assert.Equal(t, "1500.5", row[4], "value")
	assert.Equal(t, "rpm", row[5], "unit")
}

func TestSignalWriter_Close_Idempotent(t *testing.T) {
	path := tempPath(t, ".csv")
	w, err := NewSignalWriter(path, false)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, w.Close(), "second Close must be a no-op")
}

func TestSignalWriter_BadPath(t *testing.T) {
	_, err := NewSignalWriter("/no/such/dir/out.csv", false)
	assert.Error(t, err)
}

func TestCANWriter_Flush(t *testing.T) {
	path := tempPath(t, ".csv")
	w, err := NewCANWriter(path, false)
	require.NoError(t, err)
	defer w.Close()

	require.NoError(t, w.Append(0, 1, "iface", false, false, 1, []byte{0x01}))
	require.NoError(t, w.Flush())

	// Data must be visible after Flush without Close.
	rows := readRows(t, path)
	require.Len(t, rows, 1)
	assert.Equal(t, "iface", rows[0][2])
}

// alwaysFailWriter rejects every write.
type alwaysFailWriter struct{}

func (alwaysFailWriter) Write(_ []byte) (int, error) { return 0, io.ErrClosedPipe }
