package projectionstats

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This fixture is a persistent in-memory SQL peer on a loopback pgwire socket.
// Production pgx transactions, repository methods, projectors, stream writers
// and consumer codecs run unchanged. Unknown SQL fails closed. COMMIT and
// savepoint rollback change the stored state, rather than returning canned
// query results; reopening the peer reloads its committed JSON snapshot.
type repairM1043Raw struct {
	Pool, Emitter string
	Record        syncRecord
}
type repairM1043Checkpoint struct {
	Key    string
	Events json.RawMessage
}
type repairM1043State struct {
	Raw          []repairM1043Raw
	Commits      []accounting.PublicationV1
	Pools        []string
	Frames       map[string]json.RawMessage
	Checkpoints  map[string]repairM1043Checkpoint
	ResetQueries int
}
type repairM1043DB struct {
	mu       sync.Mutex
	state    repairM1043State
	path     string
	listener net.Listener
	wg       sync.WaitGroup
	pool     *pgxpool.Pool
}

func repairM1043Clone(s repairM1043State) repairM1043State {
	wire, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	var out repairM1043State
	if err = json.Unmarshal(wire, &out); err != nil {
		panic(err)
	}
	return out
}

func repairM1043Open(t *testing.T, path string, initial *repairM1043State) *repairM1043DB {
	t.Helper()
	d := &repairM1043DB{path: path}
	if initial != nil {
		d.state = repairM1043Clone(*initial)
		wire, err := json.Marshal(d.state)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, wire, 0600); err != nil {
			t.Fatal(err)
		}
	} else {
		wire, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(wire, &d.state); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	d.listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		for {
			c, err := d.listener.Accept()
			if err != nil {
				return
			}
			d.wg.Add(1)
			go func() { defer d.wg.Done(); defer c.Close(); d.serve(c) }()
		}
	}()
	cfg, err := pgxpool.ParseConfig("postgres://fixture@" + d.listener.Addr().String() + "/fixture?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	cfg.ConnConfig.ConnectTimeout = 3 * time.Second
	d.pool, err = pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.close)
	return d
}
func (d *repairM1043DB) close() {
	if d.pool != nil {
		d.pool.Close()
	}
	if d.listener != nil {
		_ = d.listener.Close()
	}
	d.wg.Wait()
}
func (d *repairM1043DB) snapshot() repairM1043State {
	d.mu.Lock()
	defer d.mu.Unlock()
	return repairM1043Clone(d.state)
}

type repairM1043Cell struct {
	oid  uint32
	text []byte
}

func repairM1043Text(s string) repairM1043Cell { return repairM1043Cell{pgtype.TextOID, []byte(s)} }
func repairM1043Bytes(b []byte) repairM1043Cell {
	if b == nil {
		return repairM1043Cell{pgtype.ByteaOID, nil}
	}
	return repairM1043Cell{pgtype.ByteaOID, []byte("\\x" + hex.EncodeToString(b))}
}
func repairM1043Bool(b bool) repairM1043Cell {
	return repairM1043Cell{pgtype.BoolOID, []byte(strconv.FormatBool(b))}
}
func repairM1043Int(n int64) repairM1043Cell {
	return repairM1043Cell{pgtype.Int8OID, []byte(strconv.FormatInt(n, 10))}
}
func repairM1043JSON(v any) repairM1043Cell {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return repairM1043Cell{pgtype.JSONBOID, b}
}
func repairM1043ByteArg(s string) ([]byte, error) {
	return hex.DecodeString(strings.TrimPrefix(s, "\\x"))
}

func (d *repairM1043DB) serve(c net.Conn) {
	b := pgproto3.NewBackend(c, c)
	if _, err := b.ReceiveStartupMessage(); err != nil {
		return
	}
	b.Send(&pgproto3.AuthenticationOk{})
	b.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
	b.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if b.Flush() != nil {
		return
	}
	var query string
	var args []string
	var tx *repairM1043State
	var savepoints []repairM1043State
	respond := func(q string, a []string) {
		var rows [][]repairM1043Cell
		var err error
		command := strings.ToLower(q)
		switch {
		case strings.HasPrefix(command, "begin"):
			s := d.snapshot()
			tx = &s
			savepoints = nil
		case command == "commit":
			if tx == nil {
				err = errors.New("commit without transaction")
				break
			}
			wire, e := json.Marshal(tx)
			if e == nil {
				e = os.WriteFile(d.path, wire, 0600)
			}
			if e != nil {
				err = e
				break
			}
			d.mu.Lock()
			d.state = repairM1043Clone(*tx)
			d.mu.Unlock()
			tx = nil
		case command == "rollback":
			tx = nil
		case strings.HasPrefix(command, "savepoint "):
			if tx == nil {
				err = errors.New("savepoint without transaction")
				break
			}
			savepoints = append(savepoints, repairM1043Clone(*tx))
		case strings.HasPrefix(command, "rollback to savepoint "):
			if len(savepoints) == 0 {
				err = errors.New("missing savepoint")
				break
			}
			s := savepoints[len(savepoints)-1]
			*tx = repairM1043Clone(s)
			savepoints = savepoints[:len(savepoints)-1]
		case strings.HasPrefix(command, "release savepoint "):
			if len(savepoints) == 0 {
				err = errors.New("missing savepoint")
				break
			}
			savepoints = savepoints[:len(savepoints)-1]
		default:
			if tx == nil {
				err = errors.New("query outside transaction")
			} else {
				rows, err = repairM1043Query(tx, q, a)
			}
		}
		if err != nil {
			b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: err.Error()})
			return
		}
		if len(rows) > 0 {
			fields := make([]pgproto3.FieldDescription, len(rows[0]))
			for i, v := range rows[0] {
				fields[i] = pgproto3.FieldDescription{Name: []byte(fmt.Sprintf("c%d", i)), DataTypeOID: v.oid, DataTypeSize: -1, TypeModifier: -1}
			}
			b.Send(&pgproto3.RowDescription{Fields: fields})
			for _, row := range rows {
				values := make([][]byte, len(row))
				for i, v := range row {
					values[i] = v.text
				}
				b.Send(&pgproto3.DataRow{Values: values})
			}
		}
		tag := "SELECT 0"
		if strings.HasPrefix(command, "insert") {
			tag = "INSERT 0 1"
		}
		if command == "commit" {
			tag = "COMMIT"
		}
		if command == "rollback" {
			tag = "ROLLBACK"
		}
		b.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
	}
	for {
		msg, err := b.Receive()
		if err != nil {
			return
		}
		switch m := msg.(type) {
		case *pgproto3.Parse:
			query = m.Query
			b.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Bind:
			args = make([]string, len(m.Parameters))
			for i, v := range m.Parameters {
				args[i] = string(v)
			}
			b.Send(&pgproto3.BindComplete{})
		case *pgproto3.Describe:
			// Exec mode describes the bound portal immediately before Execute.
			// RowDescription is sent with the computed result in Execute.

		case *pgproto3.Execute:
			respond(query, args)
		case *pgproto3.Query:
			respond(m.String, nil)
			status := byte('I')
			if tx != nil {
				status = 'T'
			}
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.Sync:
			status := byte('I')
			if tx != nil {
				status = 'T'
			}
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.Terminate:
			return
		case *pgproto3.Close:
			b.Send(&pgproto3.CloseComplete{})
		default:
			return
		}
		if b.Flush() != nil {
			return
		}
	}
}

func repairM1043PositionCompare(h, t, l string, p accounting.SourcePosition) int {
	for i, s := range []string{h, t, l} {
		a, _ := new(big.Int).SetString(s, 10)
		b, _ := new(big.Int).SetString([]string{p.Height, p.TransactionIndex, p.LogIndex}[i], 10)
		if c := a.Cmp(b); c != 0 {
			return c
		}
	}
	return 0
}
func repairM1043Anchor(s *repairM1043State) accounting.FillV1 {
	var last accounting.FillV1
	for _, p := range s.Commits {
		if last.SourceFillID == "" || repairM1043PositionCompare(p.Fill.Position.Height, p.Fill.Position.TransactionIndex, p.Fill.Position.LogIndex, last.Position) > 0 {
			last = *p.Fill
		}
	}
	return last
}

func repairM1043Query(s *repairM1043State, q string, a []string) ([][]repairM1043Cell, error) {
	one := func(c ...repairM1043Cell) ([][]repairM1043Cell, error) { return [][]repairM1043Cell{c}, nil }
	switch {
	case strings.HasPrefix(q, "SELECT pg_advisory_xact_lock"):
		return nil, nil
	case q == "SELECT clock_timestamp()":
		return one(repairM1043Cell{pgtype.TimestamptzOID, []byte("2026-01-01 00:00:00+00")})
	case strings.HasPrefix(q, "INSERT INTO") && strings.Contains(q, ".accounting_market_commits("):
		var f accounting.FillV1
		raw, e := repairM1043ByteArg(a[7])
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &f); e != nil {
			return nil, e
		}
		var pos accounting.PartitionPosition
		raw, e = repairM1043ByteArg(a[6])
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &pos); e != nil {
			return nil, e
		}
		p := accounting.PublicationV1{Version: accounting.PublicationVersion, PublicationID: a[0], SourceFillID: a[1], SourceFactDigest: accounting.Digest(a[2]), Facet: "market", Fill: &f, Positions: []accounting.PartitionPosition{pos}}
		if e = accounting.ValidatePublicationV1(p); e != nil {
			return nil, e
		}
		s.Commits = append(s.Commits, p)
		return nil, nil
	case strings.HasPrefix(q, "SELECT checkpoint_key,events FROM "):
		c, ok := s.Checkpoints[a[1]]
		if !ok {
			return nil, nil
		}
		return one(repairM1043Text(c.Key), repairM1043Cell{pgtype.JSONBOID, c.Events})
	case strings.HasPrefix(q, "INSERT INTO") && strings.Contains(q, ".public_stats_sync_baselines("):
		raw, e := repairM1043ByteArg(a[3])
		if e != nil {
			return nil, e
		}
		s.Checkpoints[a[1]] = repairM1043Checkpoint{a[2], raw}
		return nil, nil
	case strings.HasPrefix(q, "SELECT height::text,transaction_index::text,log_index::text,topics,data FROM "):
		// Apply the production lookup's emitter, pool, anchor and ABI-word
		// timestamp predicates over the persisted raw-log fixture.
		var emitters []string
		if e := pgtype.NewMap().Scan(pgtype.TextArrayOID, pgtype.TextFormatCode, []byte(a[0]), &emitters); e != nil {
			return nil, e
		}
		cutoff, e := repairM1043ByteArg(a[6])
		if e != nil {
			return nil, e
		}
		candidates := []repairM1043Raw{}
		for _, r := range s.Raw {
			allowed := false
			for _, emitter := range emitters {
				allowed = allowed || r.Emitter == emitter
			}
			var topics []string
			_ = json.Unmarshal(r.Record.Topics, &topics)
			if !allowed || len(topics) < 2 || topics[0] != a[1] || topics[1] != a[2] {
				continue
			}
			if repairM1043PositionCompare(a[3], a[4], a[5], accounting.SourcePosition{Height: r.Record.Height, TransactionIndex: r.Record.TransactionIndex, LogIndex: r.Record.LogIndex}) < 0 {
				continue
			}
			stamp := []byte{}
			if len(r.Record.Data) >= 160 {
				stamp = r.Record.Data[128:160]
			}
			if len(cutoff) > 0 && bytes.Compare(stamp, cutoff) >= 0 {
				continue
			}
			candidates = append(candidates, r)
		}
		sort.Slice(candidates, func(i, j int) bool {
			return repairM1043PositionCompare(candidates[i].Record.Height, candidates[i].Record.TransactionIndex, candidates[i].Record.LogIndex, accounting.SourcePosition{Height: candidates[j].Record.Height, TransactionIndex: candidates[j].Record.TransactionIndex, LogIndex: candidates[j].Record.LogIndex}) > 0
		})
		s.ResetQueries++
		if !strings.Contains(q, "ORDER BY height DESC,transaction_index DESC,log_index DESC LIMIT 1") {
			return nil, errors.New("unbounded or oldest-first reset query")
		}
		if len(candidates) == 0 {
			return nil, nil
		}
		r := candidates[0].Record
		return one(repairM1043Text(r.Height), repairM1043Text(r.TransactionIndex), repairM1043Text(r.LogIndex), repairM1043Bytes(r.Topics), repairM1043Bytes(r.Data))
	case strings.HasPrefix(q, "SELECT COALESCE(SUM(CASE WHEN fill"):
		delta := new(big.Int)
		any := false
		for _, p := range s.Commits {
			f := p.Fill
			if f.PoolAddress != a[1] || repairM1043PositionCompare(a[2], a[3], a[4], f.Position) >= 0 || repairM1043PositionCompare(a[5], a[6], a[7], f.Position) < 0 {
				continue
			}
			any = true
			paid, _ := new(big.Int).SetString(f.PaidInputRaw, 10)
			fee, _ := new(big.Int).SetString(f.FeeRaw, 10)
			usdl, _ := new(big.Int).SetString(f.USDLAmountRaw, 10)
			if f.Side == "buy" {
				delta.Add(delta, new(big.Int).Sub(paid, fee))
			} else {
				delta.Sub(delta, usdl)
			}
		}
		sides := repairM1043Cell{pgtype.BoolOID, nil}
		if any {
			sides = repairM1043Bool(true)
		}
		return one(repairM1043Text(delta.String()), sides)
	case strings.HasPrefix(q, "SELECT c.fill,t.block_timestamp,c.sequence,c.source_fact_digest"):
		before, _ := strconv.ParseInt(a[5], 10, 64)
		var last *accounting.PublicationV1
		for i := range s.Commits {
			p := &s.Commits[i]
			ts, _ := strconv.ParseInt(p.Fill.BlockTimestampSeconds, 10, 64)
			if p.Fill.PoolAddress != a[1] || repairM1043PositionCompare(a[2], a[3], a[4], p.Fill.Position) < 0 || (before >= 0 && ts >= before) {
				continue
			}
			if last == nil || repairM1043PositionCompare(p.Fill.Position.Height, p.Fill.Position.TransactionIndex, p.Fill.Position.LogIndex, last.Fill.Position) > 0 {
				last = p
			}
		}
		if last == nil {
			return nil, nil
		}
		ts, _ := strconv.ParseInt(last.Fill.BlockTimestampSeconds, 10, 64)
		seq, _ := strconv.ParseInt(last.Positions[0].Sequence, 10, 64)
		return one(repairM1043JSON(last.Fill), repairM1043Int(ts), repairM1043Int(seq), repairM1043Text(string(last.SourceFactDigest)))
	case strings.HasPrefix(q, "SELECT emitter,block_hash,topics,data FROM "):
		for _, p := range s.Commits {
			f := p.Fill
			if f.Position.TransactionHash != a[0] || f.Position.LogIndex != a[1] || f.Position.Height != a[2] || f.Position.TransactionIndex != a[3] {
				continue
			}
			for _, r := range s.Raw {
				if r.Record.Height == a[2] && r.Record.TransactionIndex == a[3] && r.Record.LogIndex == a[1] {
					return one(repairM1043Text(r.Emitter), repairM1043Text(f.Position.BlockHash), repairM1043Bytes(r.Record.Topics), repairM1043Bytes(r.Record.Data))
				}
			}
		}
		return nil, nil
	case strings.HasPrefix(q, "SELECT COALESCE(SUM(t.volume)"):
		sum := new(big.Int)
		buys, sells := int64(0), int64(0)
		actors := map[string]bool{}
		start, _ := strconv.ParseInt(a[5], 10, 64)
		end, _ := strconv.ParseInt(a[6], 10, 64)
		for _, p := range s.Commits {
			f := p.Fill
			ts, _ := strconv.ParseInt(f.BlockTimestampSeconds, 10, 64)
			if f.PoolAddress != a[1] || ts < start || ts >= end || repairM1043PositionCompare(a[2], a[3], a[4], f.Position) < 0 {
				continue
			}
			n, _ := new(big.Int).SetString(f.USDLAmountRaw, 10)
			sum.Add(sum, n)
			actors[f.Attribution.TransactionSender] = true
			if f.Side == "buy" {
				buys++
			} else {
				sells++
			}
		}
		return one(repairM1043Text(sum.String()), repairM1043Text(strconv.FormatInt(buys, 10)), repairM1043Text(strconv.FormatInt(sells, 10)), repairM1043Text(strconv.Itoa(len(actors))), repairM1043Text(strconv.FormatInt(buys+sells, 10)))
	case strings.HasPrefix(q, "SELECT coverage,last_event,supply::text,decimals FROM "):
		return nil, nil
	case strings.Contains(q, `"market_projection_head"($1)`):
		return one(repairM1043Bytes(s.Frames[a[0]]))
	case strings.Contains(q, `"append_market_`):
		if !strings.HasPrefix(q, `SELECT "api_stream".`) {
			return nil, fmt.Errorf("wrong stream sink: %s", q)
		}
		raw, e := repairM1043ByteArg(a[0])
		if e != nil {
			return nil, e
		}
		proofRaw, e := repairM1043ByteArg(a[1])
		if e != nil {
			return nil, e
		}
		var u delivery.StreamUpdate
		var proof delivery.StreamSourceProof
		if e = json.Unmarshal(raw, &u); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(proofRaw, &proof); e != nil {
			return nil, e
		}
		found := false
		for _, p := range s.Commits {
			if p.Fill.PoolAddress == proof.Pool && p.Positions[0].ProjectionVersion == proof.ProjectionVersion {
				if proof.Kind == "stats_commit" || p.SourceFillID == proof.SourceFillID && string(p.SourceFactDigest) == proof.SourceFactDigest && p.Positions[0].Sequence == proof.Sequence {
					found = true
				}
			}
		}
		if !found {
			return nil, errors.New("stream proof has no committed source")
		}
		var frame map[string]any
		if e = json.Unmarshal(raw, &frame); e != nil {
			return nil, e
		}
		frame["sequence"] = "1"
		frame["coalesced"] = false
		frame["sourceSequenceFirst"] = nil
		frame["sourceSequenceLast"] = nil
		if prior := s.Frames[u.Topic]; len(prior) > 0 {
			var old struct{ Sequence string }
			if e = json.Unmarshal(prior, &old); e != nil {
				return nil, e
			}
			n, e := strconv.Atoi(old.Sequence)
			if e != nil {
				return nil, e
			}
			frame["sequence"] = strconv.Itoa(n + 1)
		}
		wire, e := json.Marshal(frame)
		if e != nil {
			return nil, e
		}
		if _, e = delivery.ParseCommittedStreamFrame(wire); e != nil {
			return nil, e
		}
		s.Frames[u.Topic] = wire
		return nil, nil
	case strings.HasPrefix(q, "SELECT w.raw_complete_height,w.typed_complete_height"):
		f := repairM1043Anchor(s)
		h, _ := strconv.ParseInt(f.Position.Height, 10, 64)
		return one(repairM1043Int(h), repairM1043Int(h), repairM1043Bool(h > 0))
	case strings.HasPrefix(q, "SELECT block_hash,fetch_complete AND required_decode_complete"):
		return one(repairM1043Text(repairM1043Anchor(s).Position.BlockHash), repairM1043Bool(true))
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM ") && strings.Contains(q, ".accounting_pool_fences"):
		return one(repairM1043Bool(false))
	case strings.HasPrefix(q, "SELECT NOT EXISTS(SELECT 1 FROM "):
		return one(repairM1043Bool(len(s.Commits) > 0))
	case strings.HasPrefix(q, "SELECT height::text,transaction_index::text,log_index::text,transaction_hash,block_hash,block_timestamp FROM ("):
		f := repairM1043Anchor(s)
		ts, _ := strconv.ParseInt(f.BlockTimestampSeconds, 10, 64)
		return one(repairM1043Text(f.Position.Height), repairM1043Text(f.Position.TransactionIndex), repairM1043Text(f.Position.LogIndex), repairM1043Text(f.Position.TransactionHash), repairM1043Text(f.Position.BlockHash), repairM1043Int(ts))
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM ") && strings.Contains(q, "indexer_block_completeness_v1"):
		return one(repairM1043Bool(a[1] == repairM1043Anchor(s).Position.Height && a[2] == repairM1043Anchor(s).Position.BlockHash))
	case strings.HasPrefix(q, "SELECT pool,token,lower(creator),block_timestamp FROM "):
		limit, _ := strconv.Atoi(a[2])
		rows := [][]repairM1043Cell{}
		pools := append([]string(nil), s.Pools...)
		sort.Strings(pools)
		for _, pool := range pools {
			if pool > a[1] && len(rows) < limit {
				rows = append(rows, []repairM1043Cell{repairM1043Text(pool), repairM1043Text("0x" + strings.Repeat("a", 40)), repairM1043Text("0x" + strings.Repeat("b", 40)), repairM1043Int(1)})
			}
		}
		return rows, nil
	case strings.HasPrefix(q, "SELECT DISTINCT fill->>'actualEmitter'"):
		emitters := map[string]bool{}
		for _, p := range s.Commits {
			emitters[p.Fill.ActualEmitter] = true
		}
		rows := [][]repairM1043Cell{}
		for e := range emitters {
			rows = append(rows, []repairM1043Cell{repairM1043Text(e)})
		}
		return rows, nil
	}
	return nil, fmt.Errorf("unsupported fixture query: %s", q)
}

func repairM1043Receipt(t *testing.T) (accounting.PublicationV1, repairM1043Raw) {
	t.Helper()
	wire, err := os.ReadFile("testdata/real_pool_125.json")
	if err != nil {
		t.Fatal(err)
	}
	var tape realPool125
	if err = json.Unmarshal(wire, &tape); err != nil {
		t.Fatal(err)
	}
	for _, tx := range tape.Transactions {
		for _, log := range tx.Logs {
			if len(log.Topics) != 3 || log.Topics[0] != swapEventTopic {
				continue
			}
			data, err := hex.DecodeString(strings.TrimPrefix(log.Data, "0x"))
			if err != nil {
				t.Fatal(err)
			}
			words, err := abiWords(data, 7)
			if err != nil {
				t.Fatal(err)
			}
			if words[0].String() != "1" {
				continue
			}
			timestamp := tape.Blocks[strconv.FormatInt(tx.BlockNumber, 10)].Timestamp
			f := accounting.FillV1{Version: accounting.FillVersion,
				Position:      accounting.SourcePosition{ChainID: "125", Height: strconv.FormatInt(tx.BlockNumber, 10), TransactionIndex: strconv.FormatInt(tx.TransactionIndex, 10), LogIndex: strconv.FormatInt(log.LogIndex, 10), TransactionHash: tx.Hash, BlockHash: tx.BlockHash},
				ActualEmitter: log.Address, PoolAddress: tape.Pool, Token: accounting.Asset{Address: tape.Token, Decimals: tape.TokenDecimals}, Settlement: accounting.Asset{Address: tape.USDL, Decimals: 6},
				Side: "buy", TokenAmountRaw: words[2].String(), USDLAmountRaw: words[1].String(), PaidInputRaw: words[1].String(), FeeRaw: words[3].String(), FeeDisposition: "distributed_usdl", InputDerivationQuality: "receipt_incomplete",
				Attribution: accounting.Attribution{TransactionSender: tx.From, Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Execution: accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
				Provenance: accounting.Provenance{DeploymentID: publicTestDeployment, ABIRevision: "sidiora-v1", EmitterCodeEpoch: "event-emitter-1", PoolCodeEpoch: "pool-1", ReceiptSHA256: strings.Repeat("a", 64), HeaderSHA256: strings.Repeat("b", 64)}, BlockTimestampSeconds: strconv.FormatInt(timestamp, 10)}
			f.SourceFillID = accounting.SourceFillIdentity(f.Position, f.Provenance.DeploymentID)
			digest, err := accounting.FillDigest(f)
			if err != nil {
				t.Fatal(err)
			}
			p := accounting.PublicationV1{Version: accounting.PublicationVersion, Facet: "market", SourceFillID: f.SourceFillID, SourceFactDigest: digest, Fill: &f,
				Positions: []accounting.PartitionPosition{{ProjectionVersion: publicTestVersion, PartitionKind: "market", PartitionKey: "125:" + f.PoolAddress, Sequence: "1", SourceEventID: f.SourceFillID, PayloadDigest: digest, SourceOrder: f.Position}}}
			p.PublicationID = accounting.PublicationIdentity(f.SourceFillID, digest, "market", publicTestVersion, "initial", "")
			p.Positions[0].EffectIdentity = accounting.EffectIdentity(f.SourceFillID, "market", publicTestVersion, "", "market", p.Positions[0].PartitionKey)
			// Drive the real producer codec and consumer decoder before repository
			// persistence, preserving the source receipt's exact amounts/position.
			wire, err = accounting.CanonicalPublicationV1(p)
			if err != nil {
				t.Fatal(err)
			}
			p, err = accounting.DecodePublicationV1(wire)
			if err != nil {
				t.Fatal(err)
			}
			topics, err := json.Marshal(log.Topics)
			if err != nil {
				t.Fatal(err)
			}
			return p, repairM1043Raw{Pool: f.PoolAddress, Emitter: f.ActualEmitter, Record: syncRecord{Height: f.Position.Height, TransactionIndex: f.Position.TransactionIndex, LogIndex: f.Position.LogIndex, Topics: topics, Data: data}}
		}
	}
	t.Fatal("recorded receipt contains no canonical buy")
	return accounting.PublicationV1{}, repairM1043Raw{}
}

func repairM1043StateFor(t *testing.T, resets int) (repairM1043State, accounting.PublicationV1) {
	t.Helper()
	pub, swap := repairM1043Receipt(t)
	f := pub.Fill
	s := repairM1043State{Raw: []repairM1043Raw{swap}, Pools: []string{f.PoolAddress}, Frames: map[string]json.RawMessage{}, Checkpoints: map[string]repairM1043Checkpoint{}}
	height, _ := strconv.ParseInt(f.Position.Height, 10, 64)
	timestamp, _ := strconv.ParseInt(f.BlockTimestampSeconds, 10, 64)
	topics, err := json.Marshal([]string{poolStateUpdatedTopic, poolTopic(f.PoolAddress)})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < resets; i++ {
		h := height - int64(resets-i)
		// 400 resets span 33 hours and leave 288 within the last 24 hours.
		ts := timestamp - int64(resets-i)*300
		data := make([]byte, 192)
		for j, n := range []int64{100000, 1000000 + int64(i)*100, 2000000, 100 + int64(i), ts, h} {
			big.NewInt(n).FillBytes(data[j*32 : (j+1)*32])
		}
		s.Raw = append(s.Raw, repairM1043Raw{Pool: f.PoolAddress, Emitter: f.ActualEmitter, Record: syncRecord{Height: strconv.FormatInt(h, 10), TransactionIndex: "0", LogIndex: "0", Topics: topics, Data: data}})
	}
	return s, pub
}
func repairM1043Projector(t *testing.T, d *repairM1043DB, batch int) (*Repository, *PublicStatsProjector) {
	t.Helper()
	r, err := NewRepository(d.pool, "stats")
	if err != nil {
		t.Fatal(err)
	}
	reads, err := NewReadService(r, ReadConfig{SourceSchema: "indexer", SourceStream: publicTestStream, ProjectionVersion: publicTestVersion, CursorKey: bytes.Repeat([]byte{7}, 32), CursorTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewPublicStatsProjector(PublicStatsConfig{Reads: reads, HolderSchema: publicTestHolders, StreamSchema: "api_stream", Batch: batch})
	if err != nil {
		t.Fatal(err)
	}
	return r, p
}
func repairM1043Commit(t *testing.T, r *Repository, p accounting.PublicationV1) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = r.RecordMarketPublicationTx(ctx, tx, p, ApplyResult{Accepted: true, Revision: 1}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}
func repairM1043Stats(t *testing.T, s repairM1043State, pool string) publicStatsFrame {
	t.Helper()
	raw := s.Frames["market.stats."+pool]
	if err := ValidatePublicStatsFrame(raw); err != nil {
		t.Fatalf("stats frame: %v %s", err, raw)
	}
	consumed, err := delivery.ParseCommittedStreamFrame(raw)
	if err != nil {
		t.Fatal(err)
	}
	if consumed.Topic != "market.stats."+pool || consumed.Sequence != "1" {
		t.Fatal(consumed)
	}
	var frame publicStatsFrame
	if err = json.Unmarshal(raw, &frame); err != nil {
		t.Fatal(err)
	}
	return frame
}

func TestRepairM_1043_StreamSchemaSplit(t *testing.T) {
	state, pub := repairM1043StateFor(t, 0)
	d := repairM1043Open(t, filepath.Join(t.TempDir(), "committed.json"), &state)
	repo, p := repairM1043Projector(t, d, 2)
	for _, schema := range []string{"other_stream", "API_STREAM", "api_stream;drop schema stats", ""} {
		if _, err := NewPublicStatsProjector(PublicStatsConfig{Reads: p.reads, HolderSchema: publicTestHolders, StreamSchema: schema, Batch: 2}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("override %q accepted: %v", schema, err)
		}
	}
	repairM1043Commit(t, repo, pub)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := p.RefreshOnce(ctx, time.Unix(1800000000, 0))
	if err != nil || result.Appended != 1 || result.Prices != 1 || result.Pools != 1 {
		t.Fatal(result, err)
	}
	stored := d.snapshot()
	trade, err := delivery.ParseCommittedStreamFrame(stored.Frames["market.trades."+pub.Fill.PoolAddress])
	if err != nil {
		t.Fatal(err)
	}
	stats := repairM1043Stats(t, stored, pub.Fill.PoolAddress)
	var payload struct {
		USDL  string `json:"usdlAmountRaw"`
		Token string `json:"tokenAmountRaw"`
	}
	if err = json.Unmarshal(trade.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if len(stored.Frames) != 3 || payload.USDL != pub.Fill.USDLAmountRaw || payload.Token != pub.Fill.TokenAmountRaw || stats.Payload.Volume24hRaw != pub.Fill.USDLAmountRaw || stats.Payload.SourceUpperPosition != Position(pub.Fill.Position) {
		t.Fatal(trade, stats)
	}
	if err = ValidatePublicPriceFrame(stored.Frames["market.price."+pub.Fill.PoolAddress]); err != nil {
		t.Fatal(err)
	}
}

func TestRepairM_1043_SyncLifetimeCapRestart(t *testing.T) {
	state, pub := repairM1043StateFor(t, 400)
	path := filepath.Join(t.TempDir(), "committed.json")
	d := repairM1043Open(t, path, &state)
	repo, p := repairM1043Projector(t, d, 2)
	repairM1043Commit(t, repo, pub)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := p.RefreshOnce(ctx, time.Unix(1800000000, 0))
	if err != nil || result.Appended != 1 || result.Pools != 1 {
		t.Fatal(result, err)
	}
	stored := d.snapshot()
	frame := repairM1043Stats(t, stored, pub.Fill.PoolAddress)
	reserve := big.NewInt(1000000 + 399*100)
	paid, _ := new(big.Int).SetString(pub.Fill.PaidInputRaw, 10)
	fee, _ := new(big.Int).SetString(pub.Fill.FeeRaw, 10)
	reserve.Add(reserve, new(big.Int).Sub(paid, fee))
	if frame.Payload.RealUsdlBalanceRaw != reserve.String() {
		t.Fatal(frame.Payload.RealUsdlBalanceRaw, reserve)
	}
	swapWords, err := abiWords(state.Raw[0].Record.Data, 7)
	if err != nil {
		t.Fatal(err)
	}
	timestamp, _ := strconv.ParseInt(pub.Fill.BlockTimestampSeconds, 10, 64)
	changes := []*string{frame.Payload.PriceChangeBps24h, frame.Payload.PriceChangeBps5m, frame.Payload.PriceChangeBps1h, frame.Payload.PriceChangeBps6h}
	qualities := []string{frame.Payload.ChangeBaselineQuality, frame.Payload.ChangeBaselineQuality5m, frame.Payload.ChangeBaselineQuality1h, frame.Payload.ChangeBaselineQuality6h}
	for i, seconds := range []int64{86400, 300, 3600, 21600} {
		var baseline *big.Int
		for _, r := range state.Raw[1:] {
			words, err := abiWords(r.Record.Data, 6)
			if err != nil {
				t.Fatal(err)
			}
			if words[4].Int64() < timestamp+1-seconds {
				baseline = words[3]
			}
		}
		if baseline == nil {
			t.Fatal("missing fixture baseline")
		}
		expected := new(big.Int).Sub(swapWords[4], baseline)
		expected.Mul(expected, big.NewInt(10000))
		expected.Quo(expected, baseline)
		if changes[i] == nil || *changes[i] != expected.String() || qualities[i] != "complete" {
			t.Fatalf("window %d: %v %s expected %s", seconds, changes[i], qualities[i], expected)
		}
	}
	checkpoint := stored.Checkpoints[pub.Fill.PoolAddress]
	var records []syncRecord
	if err = json.Unmarshal(checkpoint.Events, &records); err != nil {
		t.Fatal(err)
	}
	if len(state.Raw) != 401 || len(records) != 5 || stored.ResetQueries != 5 || publicStatsMaxSyncEvents != 256 {
		t.Fatal(len(state.Raw), len(records), stored.ResetQueries, publicStatsMaxSyncEvents)
	}
	first := append([]byte(nil), stored.Frames["market.stats."+pub.Fill.PoolAddress]...)
	d.close() // Stop connections, projector and SQL peer, then reopen from disk.
	reopened := repairM1043Open(t, path, nil)
	_, p = repairM1043Projector(t, reopened, 2)
	result, err = p.RefreshOnce(ctx, time.Unix(1800000060, 0))
	if err != nil || result.Pools != 1 || result.Appended != 0 || result.Prices != 0 {
		t.Fatal(result, err)
	}
	recovered := reopened.snapshot()
	if recovered.ResetQueries != 5 || !bytes.Equal(first, recovered.Frames["market.stats."+pub.Fill.PoolAddress]) || len(recovered.Checkpoints) != 1 {
		t.Fatal("restart lost verified checkpoint or duplicated frame", recovered.ResetQueries)
	}
}

func TestRepairM_1043_FailedPoolReadinessAndLaterPage(t *testing.T) {
	for _, batch := range []int{1, 2} {
		t.Run(fmt.Sprintf("batch_%d", batch), func(t *testing.T) {
			state, pub := repairM1043StateFor(t, 400)
			bad := "0x" + strings.Repeat("1", 40)
			state.Pools = append(state.Pools, bad)
			topics, err := json.Marshal([]string{poolStateUpdatedTopic, poolTopic(bad)})
			if err != nil {
				t.Fatal(err)
			}
			// An invalid authorized raw reset must remain an integrity failure, while
			// the healthy pool in the next page still commits its frame.
			state.Raw = append(state.Raw, repairM1043Raw{Pool: bad, Emitter: pub.Fill.ActualEmitter, Record: syncRecord{Height: pub.Fill.Position.Height, TransactionIndex: "0", LogIndex: "0", Topics: topics, Data: []byte{1}}})
			d := repairM1043Open(t, filepath.Join(t.TempDir(), "committed.json"), &state)
			repo, p := repairM1043Projector(t, d, batch)
			repairM1043Commit(t, repo, pub)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			result, err := p.RefreshOnce(ctx, time.Unix(1800000000, 0))
			if !errors.Is(err, ErrIntegrity) || len(result.FailedPools) != 1 || result.FailedPools[0] != bad || result.Pools != 1 || result.Appended != 1 {
				t.Fatal(result, err)
			}
			stored := d.snapshot()
			repairM1043Stats(t, stored, pub.Fill.PoolAddress)
			if len(stored.Frames["market.stats."+bad]) != 0 || len(stored.Checkpoints[bad].Events) != 0 {
				t.Fatal("failed pool published or checkpointed invalid evidence")
			}
		})
	}
}

func TestRepairM_1043_SyncSafetyBounds(t *testing.T) {
	state, pub := repairM1043StateFor(t, 1)
	ts, _ := strconv.ParseInt(pub.Fill.BlockTimestampSeconds, 10, 64)
	anchor := publicAnchor{Position: Position(pub.Fill.Position), timestamp: ts}
	good := state.Raw[1].Record
	tooMany := make([]syncRecord, publicStatsMaxSyncEvents+1)
	for i := range tooMany {
		tooMany[i] = good
	}
	if _, err := decodeSyncRecords(tooMany, anchor, pub.Fill.PoolAddress); !errors.Is(err, ErrUnavailable) {
		t.Fatal("safety bound changed", err)
	}
	for _, bad := range []syncRecord{
		{Height: good.Height, TransactionIndex: "0", LogIndex: "0", Topics: good.Topics, Data: good.Data[:191]},
		{Height: pub.Fill.Position.Height, TransactionIndex: pub.Fill.Position.TransactionIndex, LogIndex: "4294967295", Topics: good.Topics, Data: good.Data},
		{Height: good.Height, TransactionIndex: "0", LogIndex: "0", Topics: []byte(`[]`), Data: good.Data},
	} {
		if _, err := decodeSyncRecords([]syncRecord{bad}, anchor, pub.Fill.PoolAddress); !errors.Is(err, ErrIntegrity) {
			t.Fatal("invalid reset admitted", err)
		}
	}
}
