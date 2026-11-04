package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	v1 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The transactional pgwire peer below is the committed repair_32_test.go
// fixture. Application tests use real pgx, canonical reader, publication store
// and stream writer against its persisted state, with canonical query transcripts.
type repairM934Stored struct {
	Artifact []byte
	Header   ranker.Header
	Items    []ranker.RankedItem
}
type repairM934Intent struct {
	Key                        string
	Sequence, CategorySequence int64
	Published                  bool
}
type repairM934State struct {
	Snapshots      map[string]*repairM934Stored
	Heads          map[string]string
	Outbox         []repairM934Intent
	Frames         map[string]json.RawMessage
	RejectCategory string
	Cutoff         int64
}
type repairM934Database struct {
	mu    sync.Mutex
	path  string
	state repairM934State
	t     *testing.T
}

func repairM934OpenPool(t *testing.T, path string) *pgxpool.Pool {
	t.Helper()
	db := &repairM934Database{path: path, t: t, state: repairM934State{Snapshots: map[string]*repairM934Stored{}, Heads: map[string]string{}, Frames: map[string]json.RawMessage{}, Cutoff: time.Now().UTC().Truncate(time.Second).Unix()}}
	raw, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(raw, &db.state); err != nil {
			t.Fatal(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig("postgres://repair:repair@localhost:1/repair?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	cfg.ConnConfig.LookupFunc = func(context.Context, string) ([]string, error) { return []string{"127.0.0.1"}, nil }
	cfg.ConnConfig.DialFunc = func(context.Context, string, string) (net.Conn, error) {
		client, server := net.Pipe()
		go db.serve(server)
		return client, nil
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}

type repairM934Result struct {
	oids []uint32
	rows [][]string
	tag  string
}

func repairM934Columns(sql string) []uint32 {
	switch {
	case strings.HasPrefix(sql, "SELECT w.raw_complete_height"):
		return []uint32{20, 20, 16}
	case strings.HasPrefix(sql, "SELECT block_hash,completed_at"):
		return []uint32{25, 1184, 16}
	case strings.HasPrefix(sql, "SELECT NOT EXISTS"):
		return []uint32{16}
	case strings.HasPrefix(sql, "WITH facts AS"):
		return []uint32{25, 20, 23, 23, 20, 25, 25, 25, 20, 20, 20, 25, 25, 25, 25, 25, 20, 25, 25, 23, 25, 25, 23}
	case strings.HasPrefix(sql, "SELECT category,cutoff_time"):
		return []uint32{25, 20}
	case strings.HasPrefix(sql, "SELECT count(*),count(*) FILTER"):
		return []uint32{20, 20, 20}
	case strings.HasPrefix(sql, "select ") && strings.Contains(sql, "snapshot_entries"):
		return []uint32{pgtype.TextOID, pgtype.TextOID, pgtype.Int8OID, pgtype.TextOID, pgtype.TextOID}
	case strings.HasPrefix(sql, "SELECT s.run_id,s.category"):
		return []uint32{25, 25, 25, 25, 25, 20, 1184, 20, 20, 114}
	case strings.HasPrefix(sql, "SELECT run_id,cutoff_time,revision"):
		return []uint32{25, 20, 20}
	case strings.HasPrefix(sql, "SELECT o.sequence,o.run_id"):
		return []uint32{20, 25, 25, 20}
	case strings.HasPrefix(sql, "SELECT run_id FROM"):
		return []uint32{25}
	case strings.HasPrefix(sql, "SELECT artifact="):
		return []uint32{16}
	case strings.HasPrefix(sql, "SELECT pool_address,score,rank"):
		return []uint32{25, 25, 20}
	case strings.HasPrefix(sql, "SELECT count(*)"):
		return []uint32{20}
	case strings.HasPrefix(sql, "SELECT sequence,frame"):
		return []uint32{20, 17}
	default:
		return nil
	}
}
func repairM934Description(oids []uint32) *pgproto3.RowDescription {
	desc := &pgproto3.RowDescription{}
	for i, oid := range oids {
		desc.Fields = append(desc.Fields, pgproto3.FieldDescription{Name: []byte(fmt.Sprint(i)), DataTypeOID: oid, DataTypeSize: -1, TypeModifier: -1, Format: 0})
	}
	return desc
}

// A fail-closed, transactional storage double behind the actual pgx pool.
// It accepts only the SQL exercised here; unknown statements fail the test.
// Committed state is written to disk, and every reopen constructs a new server.
func (db *repairM934Database) serve(conn net.Conn) {
	defer conn.Close()
	b := pgproto3.NewBackend(conn, conn)
	if _, err := b.ReceiveStartupMessage(); err != nil {
		return
	}
	b.Send(&pgproto3.AuthenticationOk{})
	b.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
	b.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	b.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"})
	b.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
	b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if err := b.Flush(); err != nil {
		return
	}
	statements := map[string]*pgproto3.Parse{}
	var sql string
	var args []string
	var tx *repairM934State
	var copyData []byte
	status := byte('I')
	sendResult := func(r repairM934Result, err error) {
		if err != nil {
			b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "P0001", Message: err.Error()})
			return
		}
		if len(r.oids) > 0 {
			b.Send(repairM934Description(r.oids))
		}
		for _, row := range r.rows {
			values := make([][]byte, len(row))
			for i := range row {
				values[i] = []byte(row[i])
			}
			b.Send(&pgproto3.DataRow{Values: values})
		}
		tag := r.tag
		if tag == "" {
			tag = fmt.Sprintf("SELECT %d", len(r.rows))
		}
		b.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
	}
	for {
		message, err := b.Receive()
		if err != nil {
			return
		}
		switch m := message.(type) {
		case *pgproto3.Terminate:
			return
		case *pgproto3.Parse:
			p := *m
			p.ParameterOIDs = append([]uint32(nil), m.ParameterOIDs...)
			statements[p.Name] = &p
			b.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Bind:
			p := statements[m.PreparedStatement]
			sql = p.Query
			args = make([]string, len(m.Parameters))
			for i, value := range m.Parameters {
				format := int16(0)
				if len(m.ParameterFormatCodes) == 1 {
					format = m.ParameterFormatCodes[0]
				} else if len(m.ParameterFormatCodes) > i {
					format = m.ParameterFormatCodes[i]
				}
				oid := uint32(0)
				if i < len(p.ParameterOIDs) {
					oid = p.ParameterOIDs[i]
				}
				switch {
				case format == 0 && bytes.HasPrefix(value, []byte(`\x`)):
					decoded, decodeErr := hex.DecodeString(string(value[2:]))
					if decodeErr != nil {
						db.t.Errorf("invalid bytea parameter: %v", decodeErr)
						return
					}
					args[i] = string(decoded)
				case format == 1 && oid == pgtype.Int8OID:
					args[i] = strconv.FormatInt(int64(binary.BigEndian.Uint64(value)), 10)
				case format == 1 && oid == pgtype.Int4OID:
					args[i] = strconv.FormatInt(int64(int32(binary.BigEndian.Uint32(value))), 10)
				default:
					args[i] = string(value)
				}
			}
			b.Send(&pgproto3.BindComplete{})
		case *pgproto3.Describe:
			query := sql
			if m.ObjectType == 'S' {
				p := statements[m.Name]
				query = p.Query
				b.Send(&pgproto3.ParameterDescription{ParameterOIDs: p.ParameterOIDs})
			}
			if oids := repairM934Columns(query); len(oids) > 0 {
				b.Send(repairM934Description(oids))
			} else {
				b.Send(&pgproto3.NoData{})
			}
		case *pgproto3.Execute:
			// The portal description was already sent by Describe.
			r, e := db.query(sql, args, &tx)
			r.oids = nil
			sendResult(r, e)
		case *pgproto3.Query:
			query := strings.TrimSpace(m.String)
			switch {
			case strings.HasPrefix(query, "begin"):
				db.mu.Lock()
				raw, e := json.Marshal(db.state)
				if e == nil {
					tx = &repairM934State{}
					e = json.Unmarshal(raw, tx)
				}
				db.mu.Unlock()
				status = 'T'
				sendResult(repairM934Result{tag: "BEGIN"}, e)
			case query == "commit":
				db.mu.Lock()
				raw, e := json.Marshal(tx)
				if e == nil {
					e = os.WriteFile(db.path, raw, 0600)
				}
				if e == nil {
					db.state = *tx
				}
				db.mu.Unlock()
				tx = nil
				status = 'I'
				sendResult(repairM934Result{tag: "COMMIT"}, e)
			case query == "rollback":
				tx = nil
				status = 'I'
				sendResult(repairM934Result{tag: "ROLLBACK"}, nil)
			case strings.HasPrefix(query, "copy "):
				copyData = nil
				b.Send(&pgproto3.CopyInResponse{OverallFormat: 1, ColumnFormatCodes: []uint16{1, 1, 1, 1, 1}})
				if e := b.Flush(); e != nil {
					return
				}
				continue
			default:
				r, e := db.query(query, nil, &tx)
				sendResult(r, e)
			}
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.CopyData:
			copyData = append(copyData, m.Data...)
			continue
		case *pgproto3.CopyDone:
			n, e := repairM934Copy(tx, copyData)
			sendResult(repairM934Result{tag: fmt.Sprintf("COPY %d", n)}, e)
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.Sync:
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.Flush:
		case *pgproto3.Close:
			b.Send(&pgproto3.CloseComplete{})
		default:
			db.t.Errorf("unsupported PostgreSQL message %T", m)
			return
		}
		// Extended-protocol requests are pipelined on net.Pipe. Flush at Sync,
		// Flush, Query and CopyDone so neither endpoint blocks writing to the other.
		switch message.(type) {
		case *pgproto3.Sync, *pgproto3.Flush, *pgproto3.Query, *pgproto3.CopyDone:
			if err = b.Flush(); err != nil {
				return
			}
		}
	}
}

func repairM934Copy(tx *repairM934State, raw []byte) (int, error) {
	if tx == nil || len(raw) < 19 || !bytes.Equal(raw[:11], []byte("PGCOPY\n\xff\r\n\x00")) {
		return 0, fmt.Errorf("invalid binary COPY")
	}
	r := bytes.NewReader(raw[19:])
	n := 0
	for r.Len() > 0 {
		var columns int16
		if err := binary.Read(r, binary.BigEndian, &columns); err != nil {
			return n, err
		}
		if columns == -1 {
			return n, nil
		}
		if columns != 5 {
			return n, fmt.Errorf("COPY columns %d", columns)
		}
		fields := make([][]byte, 5)
		for i := range fields {
			var size int32
			if err := binary.Read(r, binary.BigEndian, &size); err != nil {
				return n, err
			}
			if size < 0 || int64(size) > int64(r.Len()) {
				return n, fmt.Errorf("invalid COPY field size")
			}
			fields[i] = make([]byte, size)
			if _, err := io.ReadFull(r, fields[i]); err != nil {
				return n, err
			}
		}
		key := string(fields[0]) + "/" + string(fields[1])
		stored := tx.Snapshots[key]
		if stored == nil || len(fields[2]) != 8 {
			return n, fmt.Errorf("COPY without snapshot")
		}
		rank := int64(binary.BigEndian.Uint64(fields[2]))
		if rank != int64(len(stored.Items)+1) {
			return n, fmt.Errorf("COPY rank gap")
		}
		stored.Items = append(stored.Items, ranker.RankedItem{Address: string(fields[3]), Score: string(fields[4]), Rank: rank})
		n++
	}
	return n, nil
}

func (db *repairM934Database) query(sql string, args []string, tx **repairM934State) (repairM934Result, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	s := *tx
	if s == nil {
		s = &db.state
	}
	result := repairM934Result{oids: repairM934Columns(sql)}
	integer := func(i int) int64 { n, _ := strconv.ParseInt(args[i], 10, 64); return n }
	key := func() string { return args[0] + "/" + args[1] }
	switch {
	case strings.HasPrefix(sql, "SELECT w.raw_complete_height"):
		result.rows = [][]string{{"200", "200", "t"}}
	case strings.HasPrefix(sql, "SELECT block_hash,completed_at"):
		result.rows = [][]string{{"0x" + strings.Repeat("c8", 32), time.Unix(s.Cutoff, 0).UTC().Format("2006-01-02 15:04:05.999999999Z07:00"), "t"}}
	case strings.HasPrefix(sql, "SELECT NOT EXISTS"):
		result.rows = [][]string{{"t"}}
	case strings.HasPrefix(sql, "WITH facts AS"):
		// Canonical projection transcript: positive and negative execution-price changes,
		// sixty-one creations and complete observed fill metrics; pool 61 is hidden.
		for i := 1; i <= 61; i++ {
			last := "1250000"
			if i%2 == 0 {
				last = "750000"
			}
			result.rows = append(result.rows, []string{fmt.Sprintf("0x%040x", i), fmt.Sprint(i), "0", "0", fmt.Sprint(s.Cutoff - 600), "2400000000", "1000000000", "100000000", "5", "2", "3", "0", "1000000000", "0", "1000000000", "0", "4", last, "1000000", "6", "1000000", "1000000", "6"})
		}
	case strings.HasPrefix(sql, "SELECT category,cutoff_time"):
		categories := append([]string(nil), ranker.Categories[:]...)
		sort.Strings(categories)
		for _, category := range categories {
			if stored := s.Snapshots[s.Heads[category]]; stored != nil {
				result.rows = append(result.rows, []string{category, fmt.Sprint(stored.Header.CutoffTime)})
			}
			if len(result.rows) == 6 {
				break
			}
		}
	case strings.HasPrefix(sql, "SELECT count(*),count(*) FILTER"):
		pending := 0
		for _, intent := range s.Outbox {
			if !intent.Published {
				pending++
			}
		}
		result.rows = [][]string{{fmt.Sprint(pending), "0", "0"}}
	case strings.HasPrefix(sql, "SET LOCAL "), strings.Contains(sql, "pg_advisory_xact_lock"), strings.Contains(sql, ".assert_epoch("):
		result.tag = "SELECT 1"
	case strings.HasPrefix(sql, "SELECT run_id,cutoff_time,revision"):
		if stored := s.Snapshots[s.Heads[args[0]]]; stored != nil {
			h := stored.Header
			result.rows = [][]string{{h.RunID, fmt.Sprint(h.CutoffTime), fmt.Sprint(h.Revision)}}
		}
	case strings.HasPrefix(sql, "INSERT INTO ") && strings.Contains(sql, `"snapshots"`):
		if s.Snapshots[key()] != nil {
			result.tag = "INSERT 0 0"
			break
		}
		var public struct {
			AlgorithmVersion string            `json:"algorithmVersion"`
			Cutoff           int64             `json:"cutoffTime"`
			Provenance       ranker.Provenance `json:"provenance"`
			Total            int64             `json:"total"`
		}
		if err := json.Unmarshal([]byte(args[5]), &public); err != nil {
			return result, err
		}
		h := ranker.Header{RunID: args[0], Category: args[1], InputDigest: args[2], OutputDigest: args[3], AlgorithmVersion: public.AlgorithmVersion, CutoffTime: public.Cutoff, CompletedAt: time.Now().UTC().Truncate(time.Millisecond), Total: public.Total, Provenance: public.Provenance}
		s.Snapshots[key()] = &repairM934Stored{Artifact: []byte(args[4]), Header: h, Items: []ranker.RankedItem{}}
		result.tag = "INSERT 0 1"
	case strings.HasPrefix(sql, "SELECT artifact="):
		stored := s.Snapshots[key()]
		if stored == nil {
			break
		}
		result.rows = [][]string{{strconv.FormatBool(bytes.Equal(stored.Artifact, []byte(args[2])))}}
	case strings.HasPrefix(sql, "INSERT INTO ") && strings.Contains(sql, `"category_heads"`):
		k := args[1] + "/" + args[0]
		s.Heads[args[0]] = k
		s.Snapshots[k].Header.Revision = integer(3)
		result.tag = "INSERT 0 1"
	case strings.HasPrefix(sql, "INSERT INTO ") && strings.Contains(sql, `"publication_outbox"`):
		if args[1] == s.RejectCategory {
			return result, fmt.Errorf("publication outbox constraint rejected %s", args[1])
		}
		s.Outbox = append(s.Outbox, repairM934Intent{Key: key(), Sequence: int64(len(s.Outbox) + 1), CategorySequence: integer(3)})
		result.tag = "INSERT 0 1"
	case strings.HasPrefix(sql, "SELECT run_id FROM"):
		if stored := s.Snapshots[s.Heads[args[0]]]; stored != nil {
			result.rows = [][]string{{stored.Header.RunID}}
		}
	case strings.HasPrefix(sql, "SELECT s.run_id,s.category"):
		if stored := s.Snapshots[key()]; stored != nil {
			h := stored.Header
			provenance, err := json.Marshal(h.Provenance)
			if err != nil {
				return result, err
			}
			result.rows = [][]string{{h.RunID, h.Category, h.InputDigest, h.OutputDigest, h.AlgorithmVersion, fmt.Sprint(h.CutoffTime), h.CompletedAt.Format("2006-01-02 15:04:05.999999999Z07:00"), fmt.Sprint(h.Revision), fmt.Sprint(h.Total), string(provenance)}}
		}
	case strings.HasPrefix(sql, "SELECT pool_address,score,rank"):
		if stored := s.Snapshots[key()]; stored != nil {
			offset, limit := int64(0), int64(100)
			if len(args) == 4 {
				offset = integer(2)
				limit = integer(3)
			}
			for _, item := range stored.Items {
				if item.Rank > offset && int64(len(result.rows)) < limit {
					result.rows = append(result.rows, []string{item.Address, item.Score, fmt.Sprint(item.Rank)})
				}
			}
		}
	case strings.HasPrefix(sql, "SELECT o.sequence,o.run_id"):
		for _, intent := range s.Outbox {
			if !intent.Published {
				h := s.Snapshots[intent.Key].Header
				result.rows = [][]string{{fmt.Sprint(intent.Sequence), h.RunID, h.Category, fmt.Sprint(intent.CategorySequence)}}
				break
			}
		}
	case strings.HasPrefix(sql, `SELECT "api_stream"."append_ranking"`):
		if *tx == nil {
			return result, fmt.Errorf("stream append outside transaction")
		}
		var update delivery.StreamUpdate
		var proof delivery.StreamSourceProof
		if err := json.Unmarshal([]byte(args[0]), &update); err != nil {
			return result, err
		}
		if err := json.Unmarshal([]byte(args[1]), &proof); err != nil {
			return result, err
		}
		category := strings.TrimPrefix(update.Topic, "market.rankings.")
		stored := s.Snapshots[proof.SourceID+"/"+category]
		if stored == nil || proof.Kind != "ranking_commit" || stored.Header.OutputDigest != proof.SourceDigest || fmt.Sprint(stored.Header.Revision) != proof.Sequence {
			return result, fmt.Errorf("uncommitted stream source")
		}
		expected, err := store.WireSnapshot(stored.Header, stored.Items)
		if err != nil {
			return result, err
		}
		var payload v1.RankingSnapshot
		if err = json.Unmarshal(update.Payload, &payload); err != nil {
			return result, err
		}
		if !reflect.DeepEqual(expected, payload) {
			return result, fmt.Errorf("stream payload differs from committed source")
		}
		frame, err := json.Marshal(map[string]any{"eventId": update.EventID, "topic": update.Topic, "sequence": proof.Sequence, "revision": update.Revision, "occurredAt": update.OccurredAt, "watermark": update.Watermark, "payload": update.Payload, "coalesced": false, "sourceSequenceFirst": nil, "sourceSequenceLast": nil})
		if err != nil {
			return result, err
		}
		if _, exists := s.Frames[update.Topic]; exists {
			return result, fmt.Errorf("duplicate stream append")
		}
		s.Frames[update.Topic] = frame
		result.tag = "SELECT 1"
	case strings.HasPrefix(sql, "UPDATE ") && strings.Contains(sql, " SET published_at="):
		for i := range s.Outbox {
			if s.Outbox[i].Sequence == integer(0) {
				s.Outbox[i].Published = true
			}
		}
		result.tag = "UPDATE 1"
	case strings.HasPrefix(sql, "SELECT count(*)"):
		n := 0
		switch {
		case strings.Contains(sql, "api_stream."):
			n = len(s.Frames)
		case strings.Contains(sql, "ranking.snapshots"):
			n = len(s.Snapshots)
		case strings.Contains(sql, "ranking.category_heads"):
			n = len(s.Heads)
		case strings.Contains(sql, "ranking.publication_outbox"):
			for _, intent := range s.Outbox {
				if !strings.Contains(sql, "published_at IS NULL") || !intent.Published {
					n++
				}
			}
		default:
			return result, fmt.Errorf("unsupported count query: %s", sql)
		}
		result.rows = [][]string{{fmt.Sprint(n)}}
	case strings.HasPrefix(sql, "SELECT sequence,frame"):
		if frame, ok := s.Frames[args[0]]; ok {
			result.rows = [][]string{{"1", `\x` + fmt.Sprintf("%x", frame)}}
		}
	default:
		db.t.Errorf("unsupported storage query: %s", sql)
		return result, fmt.Errorf("unsupported storage query: %s", sql)
	}
	return result, nil
}
