package bringup

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// SQLite evidence is a bounded, transactionally exported cursor database.
// This reader never opens a writer, recovers a WAL or interprets SQL.
type sqliteEvidence struct {
	data     []byte
	pageSize int
	visiting map[uint32]bool
}

func sqliteVarint(data []byte) (uint64, int, error) {
	var n uint64
	for i := 0; i < 9 && i < len(data); i++ {
		b := data[i]
		if i == 8 {
			return n<<8 | uint64(b), 9, nil
		}
		n = n<<7 | uint64(b&127)
		if b < 128 {
			return n, i + 1, nil
		}
	}
	return 0, 0, errors.New("SQLite varint truncated")
}

func sqliteRecord(data []byte) ([]any, error) {
	size, n, err := sqliteVarint(data)
	if err != nil || size < uint64(n) || size > uint64(len(data)) {
		return nil, errors.New("SQLite record header malformed")
	}
	header, body := data[n:int(size)], data[int(size):]
	var row []any
	for len(header) > 0 {
		serial, n, err := sqliteVarint(header)
		if err != nil {
			return nil, err
		}
		header = header[n:]
		var width uint64
		switch serial {
		case 0, 8, 9:
			width = 0
		case 1, 2, 3, 4:
			width = serial
		case 5:
			width = 6
		case 6, 7:
			width = 8
		case 10, 11:
			return nil, errors.New("SQLite reserved serial type")
		default:
			width = (serial - 12) / 2
		}
		if width > uint64(len(body)) {
			return nil, errors.New("SQLite field truncated")
		}
		value := body[:int(width)]
		body = body[int(width):]
		switch {
		case serial == 0:
			row = append(row, nil)
		case serial == 8:
			row = append(row, int64(0))
		case serial == 9:
			row = append(row, int64(1))
		case serial >= 1 && serial <= 6:
			var v uint64
			for _, b := range value {
				v = v<<8 | uint64(b)
			}
			if width < 8 && value[0]&128 != 0 {
				v |= ^uint64(0) << (width * 8)
			}
			row = append(row, int64(v))
		case serial == 7:
			row = append(row, append([]byte(nil), value...))
		case serial >= 13 && serial%2 == 1:
			row = append(row, string(value))
		default:
			row = append(row, append([]byte(nil), value...))
		}
	}
	if len(body) != 0 {
		return nil, errors.New("SQLite record has trailing payload")
	}
	return row, nil
}

func (s *sqliteEvidence) table(pageNumber uint32) ([][]any, error) {
	if pageNumber == 0 || int64(pageNumber)*int64(s.pageSize) > int64(len(s.data)) || s.visiting[pageNumber] {
		return nil, errors.New("SQLite table page invalid or cyclic")
	}
	s.visiting[pageNumber] = true
	page := s.data[(int(pageNumber)-1)*s.pageSize : int(pageNumber)*s.pageSize]
	base := 0
	if pageNumber == 1 {
		base = 100
	}
	if len(page) < base+8 {
		return nil, io.ErrUnexpectedEOF
	}
	kind := page[base]
	header := 8
	if kind == 5 {
		header = 12
	} else if kind != 13 {
		return nil, errors.New("SQLite evidence requires table btree")
	}
	count := int(binary.BigEndian.Uint16(page[base+3 : base+5]))
	if base+header+count*2 > len(page) {
		return nil, errors.New("SQLite cell pointers out of range")
	}
	var rows [][]any
	for i := 0; i < count; i++ {
		offset := int(binary.BigEndian.Uint16(page[base+header+i*2 : base+header+i*2+2]))
		if offset < base+header+count*2 || offset >= len(page) {
			return nil, errors.New("SQLite cell offset invalid")
		}
		cell := page[offset:]
		if kind == 5 {
			if len(cell) < 4 {
				return nil, io.ErrUnexpectedEOF
			}
			child := binary.BigEndian.Uint32(cell[:4])
			if _, _, err := sqliteVarint(cell[4:]); err != nil {
				return nil, err
			}
			part, err := s.table(child)
			if err != nil {
				return nil, err
			}
			rows = append(rows, part...)
			continue
		}
		size, n, err := sqliteVarint(cell)
		if err != nil {
			return nil, err
		}
		cell = cell[n:]
		_, n, err = sqliteVarint(cell)
		if err != nil {
			return nil, err
		}
		cell = cell[n:]
		// Cursor-only snapshots have no overflow payload. Refuse oversized or
		// damaged snapshots instead of silently consuming a partial row.
		if size > uint64(len(cell)) || size > uint64(s.pageSize-35) {
			return nil, errors.New("SQLite cursor payload exceeds local page")
		}
		row, err := sqliteRecord(cell[:int(size)])
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	if kind == 5 {
		part, err := s.table(binary.BigEndian.Uint32(page[base+8 : base+12]))
		if err != nil {
			return nil, err
		}
		rows = append(rows, part...)
	}
	return rows, nil
}

type SQLiteCursor struct {
	Position uint64
	Hash     string
}

func ReadSQLiteCursors(data []byte) (map[string]SQLiteCursor, error) {
	if len(data) < 100 || len(data) > 1<<20 || string(data[:16]) != "SQLite format 3\x00" {
		return nil, errors.New("bounded SQLite cursor snapshot required")
	}
	pageSize := int(binary.BigEndian.Uint16(data[16:18]))
	if pageSize == 1 {
		pageSize = 65536
	}
	if pageSize < 512 || pageSize > 65536 || pageSize&(pageSize-1) != 0 || len(data)%pageSize != 0 || data[20] != 0 || binary.BigEndian.Uint32(data[56:60]) != 1 {
		return nil, errors.New("unsupported SQLite snapshot encoding")
	}
	s := sqliteEvidence{data, pageSize, map[uint32]bool{}}
	schema, err := s.table(1)
	if err != nil {
		return nil, err
	}
	roots := map[string]uint32{}
	for _, row := range schema {
		if len(row) != 5 {
			return nil, errors.New("SQLite schema row malformed")
		}
		name, _ := row[1].(string)
		if name != "cursors" && name != "backfill_cursors" {
			continue
		}
		kind, _ := row[0].(string)
		root, ok := row[3].(int64)
		if !ok || kind != "table" || root <= 1 || root > 1<<32-1 {
			return nil, errors.New("SQLite cursor schema malformed")
		}
		if _, ok := roots[name]; ok {
			return nil, errors.New("duplicate SQLite cursor table")
		}
		roots[name] = uint32(root)
	}
	if len(roots) != 2 {
		return nil, errors.New("SQLite cursor tables missing")
	}
	out := map[string]SQLiteCursor{}
	for name, root := range roots {
		rows, err := s.table(root)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if len(row) < 3 {
				return nil, errors.New("SQLite cursor row malformed")
			}
			chain, _ := row[0].(string)
			if chain != "paxeer" {
				continue
			}
			position, ok := row[1].(int64)
			hash, text := row[2].(string)
			if !ok || position < 0 || !text || hash == "" {
				return nil, errors.New("SQLite cursor value malformed")
			}
			if _, ok := out[name]; ok {
				return nil, fmt.Errorf("duplicate SQLite %s cursor", name)
			}
			out[name] = SQLiteCursor{uint64(position), hash}
		}
	}
	if len(out) != 2 {
		return nil, errors.New("SQLite paxeer cursor missing")
	}
	return out, nil
}
