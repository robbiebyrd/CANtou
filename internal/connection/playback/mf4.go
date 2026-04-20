package playback

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"strings"
)

// MF4Parser reads ASAM MDF4 (.mf4) log files containing CAN bus data.
//
// It supports both finalized and unfinalized (streaming) files as produced
// by CANedge and similar loggers. Only CAN_DataFrame channel groups are
// extracted; LIN and other bus types are skipped.
//
// The parser handles unsorted files where records from multiple channel
// groups are interleaved with record-ID prefixes, and where payload data
// is stored in a separate VLSD (Variable Length Signal Data) channel group.
type MF4Parser struct {
	l *slog.Logger
}

// mf4 block header: [4]ID [4]reserved uint64(length) uint64(linkCount) = 24 bytes.
const mf4HeaderSize = 24

// cgInfo describes one channel group in an MF4 data group.
type cgInfo struct {
	recordID  uint64
	dataBytes uint32
	acqName   string
	isVLSD    bool
}

func (p *MF4Parser) Parse(path string) ([]LogEntry, error) {
	logger := p.l
	if logger == nil {
		logger = slog.Default()
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening MF4 file: %w", err)
	}
	defer f.Close()

	// Read the 64-byte ID block.
	idBuf := make([]byte, 64)
	if _, err := io.ReadFull(f, idBuf); err != nil {
		return nil, fmt.Errorf("reading MF4 ID block: %w", err)
	}

	// Read HD block at fixed offset 64.
	hdLinks, _, err := readMF4Block(f, 64)
	if err != nil {
		return nil, fmt.Errorf("reading HD block: %w", err)
	}
	dgAddr := hdLinks[0] // first data group

	if dgAddr == 0 {
		return nil, nil
	}

	// Walk the DG chain — typically one DG for CAN bus logging.
	var allEntries []LogEntry
	for dgAddr != 0 {
		entries, nextDG, err := p.parseDG(f, dgAddr, logger)
		if err != nil {
			return nil, fmt.Errorf("parsing data group at %d: %w", dgAddr, err)
		}
		allEntries = append(allEntries, entries...)
		dgAddr = nextDG
	}

	// Convert absolute timestamps to offsets from the first frame.
	if len(allEntries) > 0 {
		baseNs := allEntries[0].OffsetNs
		for i := range allEntries {
			allEntries[i].OffsetNs -= baseNs
		}
	}

	return allEntries, nil
}

// parseDG parses a single Data Group and returns CAN log entries plus the
// address of the next DG (0 if none).
func (p *MF4Parser) parseDG(f *os.File, addr int64, logger *slog.Logger) ([]LogEntry, int64, error) {
	dgLinks, dgData, err := readMF4Block(f, addr)
	if err != nil {
		return nil, 0, fmt.Errorf("reading DG block: %w", err)
	}
	nextDG := dgLinks[0]
	cgFirstAddr := dgLinks[1]
	dtAddr := dgLinks[2]
	recIDSize := int(dgData[0])

	if dtAddr == 0 || cgFirstAddr == 0 {
		return nil, nextDG, nil
	}

	// Build a map of channel groups by record ID.
	cgMap := make(map[uint64]*cgInfo)
	var canCG *cgInfo

	cgAddr := cgFirstAddr
	for cgAddr != 0 {
		cgLinks, cgData, err := readMF4Block(f, cgAddr)
		if err != nil {
			return nil, nextDG, fmt.Errorf("reading CG block: %w", err)
		}
		recordID := binary.LittleEndian.Uint64(cgData[0:8])
		flags := binary.LittleEndian.Uint16(cgData[16:18])
		dataBytes := binary.LittleEndian.Uint32(cgData[24:28])
		acqName := readMF4Text(f, cgLinks[2])
		isVLSD := flags&1 != 0

		info := &cgInfo{
			recordID:  recordID,
			dataBytes: dataBytes,
			acqName:   acqName,
			isVLSD:    isVLSD,
		}
		cgMap[recordID] = info

		if acqName == "CAN_DataFrame" {
			canCG = info
		}

		cgAddr = cgLinks[0] // next CG
	}

	if canCG == nil {
		logger.Debug("playback: MF4 data group has no CAN_DataFrame channel group")
		return nil, nextDG, nil
	}

	// Read the DT block data.
	dtData, err := readMF4DTData(f, dtAddr)
	if err != nil {
		return nil, nextDG, fmt.Errorf("reading DT data: %w", err)
	}

	// Two-pass parsing: first collect all VLSD payload data, then decode
	// CAN records. This is necessary because in unsorted files, the VLSD
	// offset in a CAN record references accumulated VLSD data that may
	// appear anywhere in the interleaved stream.
	vlsd := collectVLSD(dtData, recIDSize, cgMap)

	// Second pass: extract CAN_DataFrame records.
	var entries []LogEntry
	pos := 0
	for pos < len(dtData) {
		if recIDSize > 0 {
			if pos+recIDSize > len(dtData) {
				break
			}
			recID := readRecordID(dtData[pos:], recIDSize)
			pos += recIDSize

			cg, ok := cgMap[recID]
			if !ok {
				logger.Debug("playback: MF4 unknown record ID", "recID", recID)
				break
			}

			if cg.isVLSD {
				if pos+4 > len(dtData) {
					break
				}
				vlsdLen := int(binary.LittleEndian.Uint32(dtData[pos : pos+4]))
				pos += 4 + vlsdLen
				continue
			}

			recSize := int(cg.dataBytes)
			if pos+recSize > len(dtData) {
				break
			}

			if cg.acqName != "CAN_DataFrame" {
				pos += recSize
				continue
			}

			rec := dtData[pos : pos+recSize]
			pos += recSize

			entry, err := parseMF4CANRecord(rec, vlsd)
			if err != nil {
				logger.Debug("playback: MF4 skipping unparseable CAN record", "error", err)
				continue
			}
			entries = append(entries, *entry)
		} else {
			// Sorted file: only one CG, no record ID prefix.
			recSize := int(canCG.dataBytes)
			if pos+recSize > len(dtData) {
				break
			}
			rec := dtData[pos : pos+recSize]
			pos += recSize

			entry, err := parseMF4CANRecord(rec, vlsd)
			if err != nil {
				logger.Debug("playback: MF4 skipping unparseable CAN record", "error", err)
				continue
			}
			entries = append(entries, *entry)
		}
	}

	return entries, nextDG, nil
}

// parseMF4CANRecord decodes a single CAN_DataFrame record (22 bytes for
// standard CAN). The record layout is:
//
//	[0:8]   float64 LE  timestamp (ns since measurement start)
//	[8:22]  CAN composite:
//	  byte 0, bits 0-1:   BusChannel
//	  byte 0, bits 2-30:  CAN ID (29 bits, LE uint32)
//	  byte 3, bit 7:      IDE
//	  byte 4, bit 0:      Dir (0=Rx, 1=Tx)
//	  byte 4, bits 1-7:   DataLength
//	  byte 5, bit 0:      EDL
//	  byte 5, bit 1:      BRS
//	  byte 5, bits 2-5:   DLC
//	  bytes 6-13:         VLSD offset (uint64 LE) into vlsdBuf
func parseMF4CANRecord(rec []byte, vlsd *vlsdIndex) (*LogEntry, error) {
	if len(rec) < 22 {
		return nil, fmt.Errorf("CAN record too short: %d bytes", len(rec))
	}

	tsNs := math.Float64frombits(binary.LittleEndian.Uint64(rec[0:8]))
	can := rec[8:22]

	idRaw := binary.LittleEndian.Uint32(can[0:4])
	canID := (idRaw >> 2) & 0x1FFFFFFF

	dir := can[4] & 1
	dataLength := int((can[4] >> 1) & 0x7F)

	vlsdOffset := binary.LittleEndian.Uint64(can[6:14])

	data := vlsd.lookup(vlsdOffset, dataLength)

	return &LogEntry{
		OffsetNs: int64(tsNs),
		ID:       canID,
		Transmit: dir == 1,
		Length:   uint8(dataLength),
		Data:     data,
	}, nil
}

// vlsdIndex maps raw byte offsets (as stored in CAN_DataFrame records) to
// positions within the concatenated payload buffer.
type vlsdIndex struct {
	buf     []byte          // concatenated VLSD payloads (no length prefixes)
	offsets map[uint64]int  // raw stream offset -> index into buf
}

// collectVLSD scans the data stream and builds a VLSD index. In MDF4, each
// VLSD record in the unsorted stream is [4-byte length][payload]. The offset
// stored in a CAN_DataFrame record is the cumulative byte position in the raw
// VLSD stream (including length prefixes). We track both the raw offset and
// the corresponding position in our payload-only buffer.
func collectVLSD(dtData []byte, recIDSize int, cgMap map[uint64]*cgInfo) *vlsdIndex {
	idx := &vlsdIndex{offsets: make(map[uint64]int)}
	var rawOffset uint64
	pos := 0
	for pos < len(dtData) {
		if recIDSize <= 0 {
			break
		}
		if pos+recIDSize > len(dtData) {
			break
		}
		recID := readRecordID(dtData[pos:], recIDSize)
		pos += recIDSize

		cg, ok := cgMap[recID]
		if !ok {
			break
		}

		if cg.isVLSD {
			if pos+4 > len(dtData) {
				break
			}
			vlsdLen := int(binary.LittleEndian.Uint32(dtData[pos : pos+4]))
			// Map the raw offset (including length prefix) to payload position.
			idx.offsets[rawOffset] = len(idx.buf)
			rawOffset += uint64(4 + vlsdLen)
			pos += 4
			if pos+vlsdLen > len(dtData) {
				break
			}
			idx.buf = append(idx.buf, dtData[pos:pos+vlsdLen]...)
			pos += vlsdLen
		} else {
			pos += int(cg.dataBytes)
		}
	}
	return idx
}

// lookup retrieves dataLength bytes starting at the given raw VLSD offset.
func (v *vlsdIndex) lookup(rawOffset uint64, dataLength int) []byte {
	payloadPos, ok := v.offsets[rawOffset]
	if !ok || payloadPos+dataLength > len(v.buf) {
		return nil
	}
	out := make([]byte, dataLength)
	copy(out, v.buf[payloadPos:payloadPos+dataLength])
	return out
}

// readMF4Block reads a generic MF4 block at the given address and returns
// its link array and data section.
func readMF4Block(f *os.File, addr int64) ([]int64, []byte, error) {
	if _, err := f.Seek(addr, io.SeekStart); err != nil {
		return nil, nil, err
	}
	var hdr [mf4HeaderSize]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return nil, nil, err
	}
	linkCount := binary.LittleEndian.Uint64(hdr[16:24])
	blockLen := binary.LittleEndian.Uint64(hdr[8:16])

	links := make([]int64, linkCount)
	for i := uint64(0); i < linkCount; i++ {
		if err := binary.Read(f, binary.LittleEndian, &links[i]); err != nil {
			return nil, nil, err
		}
	}

	dataSize := blockLen - mf4HeaderSize - linkCount*8
	data := make([]byte, dataSize)
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, nil, err
	}
	return links, data, nil
}

// readMF4DTData reads the raw data bytes from a DT block. For unfinalized
// files the DT header reports length=24 (no data), so we read from after
// the header to EOF.
func readMF4DTData(f *os.File, addr int64) ([]byte, error) {
	if _, err := f.Seek(addr, io.SeekStart); err != nil {
		return nil, err
	}
	var hdr [mf4HeaderSize]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return nil, err
	}

	id := string(hdr[0:4])
	blockLen := binary.LittleEndian.Uint64(hdr[8:16])

	if id != "##DT" {
		return nil, fmt.Errorf("expected ##DT block, got %q", id)
	}

	dataSize := blockLen - mf4HeaderSize
	if dataSize > 0 {
		// Finalized file: data size is known.
		data := make([]byte, dataSize)
		if _, err := io.ReadFull(f, data); err != nil {
			return nil, err
		}
		return data, nil
	}

	// Unfinalized file: length=24 means no declared data. Read to EOF.
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// readMF4Text reads a TX or MD block and returns the text content.
func readMF4Text(f *os.File, addr int64) string {
	if addr == 0 {
		return ""
	}
	links, data, err := readMF4Block(f, addr)
	_ = links
	if err != nil || len(data) == 0 {
		return ""
	}
	return strings.TrimRight(string(data), "\x00")
}

// readRecordID reads 1, 2, 4, or 8 byte record IDs from an unsorted data stream.
func readRecordID(buf []byte, size int) uint64 {
	switch size {
	case 1:
		return uint64(buf[0])
	case 2:
		return uint64(binary.LittleEndian.Uint16(buf[:2]))
	case 4:
		return uint64(binary.LittleEndian.Uint32(buf[:4]))
	case 8:
		return binary.LittleEndian.Uint64(buf[:8])
	default:
		return 0
	}
}

// isMF4File checks the first 8 bytes of a file for MF4 magic bytes.
func isMF4File(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var magic [8]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return false
	}
	s := string(magic[:])
	return strings.HasPrefix(s, "MDF") || strings.HasPrefix(s, "UnFinMF")
}
