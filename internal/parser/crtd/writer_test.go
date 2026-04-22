package crtd

import (
	"bufio"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/cantou/internal/models"
)

func tempPath(t *testing.T, suffix string) string {
	t.Helper()
	f, err := os.CreateTemp("", "crtd_writer_test_*"+suffix)
	require.NoError(t, err)
	name := f.Name()
	require.NoError(t, f.Close())
	t.Cleanup(func() { os.Remove(name) })
	return name
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

// alwaysFailWriter rejects every write.
type alwaysFailWriter struct{}

func (alwaysFailWriter) Write(_ []byte) (int, error) { return 0, io.ErrClosedPipe }

func TestWriteHeader_ContentsAndFormat(t *testing.T) {
	path := tempPath(t, ".crtd")
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	require.NoError(t, err)
	defer f.Close()

	cfg := &canModels.Config{
		CanInterfaces: []canModels.CanInterfaceOption{
			{Name: "can0", URI: "vcan0", Network: "can", DBCFiles: []string{"a.dbc"}},
		},
	}
	w := bufio.NewWriter(f)
	require.NoError(t, WriteHeader(w, cfg))

	contents := readFile(t, path)
	assert.Contains(t, contents, "0.000000 CXX CRTD file created by cantou")
	assert.Contains(t, contents, "Name:'can0'")
	assert.Contains(t, contents, "URI:'vcan0'")
	assert.Contains(t, contents, "Network:'can'")
	assert.Contains(t, contents, "DBC:'a.dbc'")
}

func TestWriteHeader_ReturnsErrorOnWriteFailure(t *testing.T) {
	// A 1-byte bufio.Writer backed by an always-failing writer forces the first
	// multi-byte write to overflow into the underlying writer, surfacing the error.
	w := bufio.NewWriterSize(alwaysFailWriter{}, 1)
	cfg := &canModels.Config{
		CanInterfaces: []canModels.CanInterfaceOption{{Name: "can0"}},
	}
	err := WriteHeader(w, cfg)
	assert.Error(t, err, "WriteHeader must return an error when the underlying writer fails")
}

func TestNewCANWriter_WritesHeader(t *testing.T) {
	path := tempPath(t, ".crtd")
	cfg := &canModels.Config{MessageBufferSize: 16}
	w, err := NewCANWriter(path, cfg)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	contents := readFile(t, path)
	assert.Contains(t, contents, "0.000000 CXX CRTD file created by cantou")
}

func TestNewCANWriter_BadPath(t *testing.T) {
	_, err := NewCANWriter("/no/such/dir/out.crtd", &canModels.Config{})
	assert.Error(t, err)
}

func TestCANWriter_Append_StandardMessage(t *testing.T) {
	path := tempPath(t, ".crtd")
	cfg := &canModels.Config{}
	w, err := NewCANWriter(path, cfg)
	require.NoError(t, err)

	// 1 second in nanoseconds, interface 0, 11-bit ID, RX
	require.NoError(t, w.Append(1_000_000_000, 0, 0x123, false, []byte{0xDE, 0xAD, 0xBE, 0xEF}))
	require.NoError(t, w.Close())

	contents := readFile(t, path)
	assert.Contains(t, contents, "1.000000 0R11 123 DE AD BE EF")
}

func TestCANWriter_Append_TransmitMessage(t *testing.T) {
	path := tempPath(t, ".crtd")
	w, err := NewCANWriter(path, &canModels.Config{})
	require.NoError(t, err)
	require.NoError(t, w.Append(2_000_000_000, 0, 0x100, true, []byte{0x01}))
	require.NoError(t, w.Close())

	assert.Contains(t, readFile(t, path), "T11")
}

func TestCANWriter_Append_Extended29BitID(t *testing.T) {
	path := tempPath(t, ".crtd")
	w, err := NewCANWriter(path, &canModels.Config{})
	require.NoError(t, err)
	require.NoError(t, w.Append(3_000_000_000, 0, 0x800, false, []byte{0xAA}))
	require.NoError(t, w.Close())

	assert.Contains(t, readFile(t, path), "R29")
}

func TestCANWriter_Append_TimestampConversion(t *testing.T) {
	path := tempPath(t, ".crtd")
	w, err := NewCANWriter(path, &canModels.Config{})
	require.NoError(t, err)
	// 5 seconds + 123456 microseconds = 5_000_123_456_000 nanoseconds
	require.NoError(t, w.Append(5_000_123_456_000, 0, 0x01, false, []byte{0x00}))
	require.NoError(t, w.Close())

	assert.Contains(t, readFile(t, path), "5000.123456")
}

func TestCANWriter_Append_EmptyData(t *testing.T) {
	path := tempPath(t, ".crtd")
	w, err := NewCANWriter(path, &canModels.Config{})
	require.NoError(t, err)
	require.NoError(t, w.Append(1_000_000_000, 0, 0x7FF, false, []byte{}))
	require.NoError(t, w.Close())

	contents := readFile(t, path)
	lines := strings.Split(strings.TrimSpace(contents), "\n")
	// Last line should be the CAN record (first line is the header).
	last := lines[len(lines)-1]
	assert.Contains(t, last, "0R11 7FF")
}

func TestCANWriter_Close_Idempotent(t *testing.T) {
	path := tempPath(t, ".crtd")
	w, err := NewCANWriter(path, &canModels.Config{})
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, w.Close(), "second Close must be a no-op")
}

func TestNewSignalWriter_BadPath(t *testing.T) {
	_, err := NewSignalWriter("/no/such/dir/out.crtd")
	assert.Error(t, err)
}

func TestSignalWriter_Append_Format(t *testing.T) {
	path := tempPath(t, ".crtd")
	w, err := NewSignalWriter(path)
	require.NoError(t, err)

	require.NoError(t, w.Append(1_000_000_000, 0, "ENGINE", "RPM", 1500.5, "rpm"))
	require.NoError(t, w.Close())

	contents := readFile(t, path)
	assert.Contains(t, contents, "1.000000")
	assert.Contains(t, contents, "0SIG")
	assert.Contains(t, contents, "ENGINE/RPM")
	assert.Contains(t, contents, "1500.5")
	assert.Contains(t, contents, "rpm")
}

func TestSignalWriter_Close_Idempotent(t *testing.T) {
	path := tempPath(t, ".crtd")
	w, err := NewSignalWriter(path)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, w.Close(), "second Close must be a no-op")
}
