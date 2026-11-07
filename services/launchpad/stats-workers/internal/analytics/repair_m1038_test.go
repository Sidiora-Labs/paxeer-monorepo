package analytics_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/holdings"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/projectionstats"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/risk"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

const repair1038Version = "market-v1"
const repair1038Stream = "archive-125"

type repair1038Creation struct {
	Pool, Token, Creator, Height, Transaction, Hash, Supply string
	CreatedAt                                               int64
}
type repair1038Progress struct{ Height, TI, LI, Hash, Revision, Observed string }
type repair1038Trade struct {
	Fill   accounting.FillV1
	Digest string
}
type repair1038InputRow struct {
	Height, Hash string
	Inputs       analytics.RiskInputs
}

// This is the database peer's transactional fixture state, not an alternate
// implementation of the projector or of the canonical input producer.
type repair1038State struct {
	Creation       repair1038Creation
	Source         []accounting.FillV1
	Transfers      []holdings.Transfer
	HolderEvents   []holdings.Transfer
	Committed      map[string]accounting.FillV1
	Progress       map[string]repair1038Progress
	Trades         map[string]repair1038Trade
	Inputs         map[string]repair1038InputRow
	Snapshots      map[string]analytics.Snapshot
	Outbox         []analytics.Snapshot
	Complete       bool
	HolderComplete bool
	Now            time.Time
}

func repair1038JSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
func repair1038Clone(s repair1038State) repair1038State {
	var copy repair1038State
	if err := json.Unmarshal(repair1038JSON(s), &copy); err != nil {
		panic(err)
	}
	return copy
}
func repair1038Fixture(t *testing.T) repair1038State {
	t.Helper()
	directory := filepath.Join("..", "..", "..", "..", "..", "sdk", "launchpad", "protocol", "contracts", "v1", "chain")
	authority, err := projectionstats.OpenHistoricalCoreAuthority(directory)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(directory, "golden-fixtures.json"))
	if err != nil {
		t.Fatal(err)
	}
	var archive struct {
		Addresses map[string]string
		Receipts  []struct {
			Label, Hash, BlockHash, From string
			BlockNumber, Index           uint64
			Logs                         []struct {
				Address, BlockHash, TransactionHash, Data string
				BlockNumber, TransactionIndex, Index      uint64
				Topics                                    []string
				Removed                                   bool
			}
		}
		DecodedLogs []struct {
			Name, Address, TransactionHash, BlockHash string
			BlockNumber                               uint64
			Args                                      []json.RawMessage
		}
	}
	if err = json.Unmarshal(raw, &archive); err != nil {
		t.Fatal(err)
	}
	text := func(b []byte) string {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			t.Fatal(err)
		}
		return strings.ToLower(s)
	}
	s := repair1038State{Complete: true, Committed: map[string]accounting.FillV1{}, Progress: map[string]repair1038Progress{}, Trades: map[string]repair1038Trade{}, Inputs: map[string]repair1038InputRow{}, Snapshots: map[string]analytics.Snapshot{}}
	for _, d := range archive.DecodedLogs {
		if d.Name == "MarketCreated" && strings.EqualFold(d.Address, archive.Addresses["factory"]) && d.BlockNumber == 57 {
			s.Creation = repair1038Creation{Token: text(d.Args[0]), Pool: text(d.Args[1]), Creator: text(d.Args[2]), Height: "57", Transaction: strings.ToLower(d.TransactionHash), Hash: strings.ToLower(d.BlockHash), Supply: "1000000000000000", CreatedAt: 1791504057}
		}
	}
	if s.Creation.Token == "" {
		t.Fatal("recorded canonical factory creation missing")
	}
	evidence := fmt.Sprintf("%x", sha256.Sum256(raw))
	for _, receipt := range archive.Receipts {
		if receipt.BlockNumber > 68 {
			continue
		}
		for _, l := range receipt.Logs {
			if !strings.EqualFold(l.Address, s.Creation.Token) || len(l.Topics) != 3 || !strings.EqualFold(l.Topics[0], "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef") {
				continue
			}
			data, err := hex.DecodeString(strings.TrimPrefix(l.Data, "0x"))
			if err != nil {
				t.Fatal(err)
			}
			topics := make([]common.Hash, len(l.Topics))
			for i, topic := range l.Topics {
				topics[i] = common.HexToHash(topic)
			}
			event, err := holdings.TransferFromLog(types.Log{Address: common.HexToAddress(l.Address), Topics: topics, Data: data, BlockNumber: l.BlockNumber, TxHash: common.HexToHash(l.TransactionHash), TxIndex: uint(l.TransactionIndex), BlockHash: common.HexToHash(l.BlockHash), Index: uint(l.Index), Removed: l.Removed})
			if err != nil {
				t.Fatal(err)
			}
			s.Transfers = append(s.Transfers, event)
		}
		for _, trade := range authority.Trades(receipt.Hash) {
			if trade.PoolAddress != s.Creation.Pool {
				continue
			}
			if _, err := authority.Account(trade); err != nil {
				t.Fatal(err)
			}
			timestamp := ""
			for _, l := range receipt.Logs {
				if strconv.FormatUint(l.Index, 10) == trade.Position.LogIndex {
					data, err := hex.DecodeString(strings.TrimPrefix(l.Data, "0x"))
					if err != nil || len(data) != 224 {
						t.Fatal("recorded swap ABI", err)
					}
					timestamp = new(big.Int).SetBytes(data[160:192]).String()
				}
			}
			fill := accounting.FillV1{Version: accounting.FillVersion, Position: accounting.SourcePosition{ChainID: trade.Position.ChainID, Height: trade.Position.Height, TransactionIndex: trade.Position.TransactionIndex, LogIndex: trade.Position.LogIndex, TransactionHash: trade.Position.TransactionHash, BlockHash: trade.Position.BlockHash}, ActualEmitter: trade.EmittingContract, PoolAddress: trade.PoolAddress, Token: accounting.Asset{Address: s.Creation.Token, Decimals: 6}, Settlement: accounting.Asset{Address: strings.ToLower(archive.Addresses["usdl"]), Decimals: 6}, Side: trade.Side, TokenAmountRaw: trade.TokenAmountRaw, USDLAmountRaw: trade.USDLAmountRaw, PaidInputRaw: *trade.PaidInputRaw, FeeRaw: trade.FeeRaw, InputDerivationQuality: trade.InputDerivationQuality, FeeDisposition: trade.FeeDisposition, Attribution: accounting.Attribution{TransactionSender: strings.ToLower(receipt.From), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Execution: accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Provenance: accounting.Provenance{DeploymentID: "isolated-hardhat-chain125", ABIRevision: "sidiora-v1", EmitterCodeEpoch: trade.CodeEpoch, PoolCodeEpoch: "archived-pool-v1", ReceiptSHA256: evidence, HeaderSHA256: evidence}, BlockTimestampSeconds: timestamp}
			fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
			if err = accounting.ValidateFillV1(fill); err != nil {
				t.Fatal(err)
			}
			s.Source = append(s.Source, fill)
		}
	}
	if len(s.Source) < 3 || len(s.Transfers) < 4 {
		t.Fatal("real producer transcript incomplete", len(s.Source), len(s.Transfers))
	}
	last := s.Source[len(s.Source)-1]
	timestamp, err := strconv.ParseInt(last.BlockTimestampSeconds, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	s.Now = time.Unix(timestamp+10, 0).UTC()
	return s
}
func repair1038Envelope(t *testing.T, fill accounting.FillV1, sequence int, now time.Time) (delivery.Envelope, []byte) {
	t.Helper()
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	position := accounting.PartitionPosition{ProjectionVersion: repair1038Version, PartitionKind: "market", PartitionKey: "125:" + fill.PoolAddress, Sequence: strconv.Itoa(sequence), SourceEventID: fill.SourceFillID, PayloadDigest: digest, SourceOrder: fill.Position}
	if sequence > 1 {
		position.PreviousEffectIdentity = accounting.HashTuple("archive-previous", strconv.Itoa(sequence-1))
	}
	position.EffectIdentity = accounting.EffectIdentity(fill.SourceFillID, "market", repair1038Version, "", "market", position.PartitionKey)
	publication, err := accounting.NewMarketPublication(fill, []accounting.PartitionPosition{position})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		t.Fatal(err)
	}
	e := delivery.Envelope{EventID: publication.PublicationID, EventType: "accounting.market.v1", SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: fill.Position.Height, TransactionIndex: fill.Position.TransactionIndex, LogIndex: fill.Position.LogIndex}, EmittedAt: now, TraceID: "repair-m1038", Payload: payload, PayloadHash: delivery.Hash(payload)}
	wire, err := e.Encode(repair1038Policy(), accounting.MarketSubject)
	if err != nil {
		t.Fatal(err)
	}
	return e, wire
}
func repair1038Policy() delivery.Policy {
	return delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true}, EventTypes: map[string]bool{"accounting.market.v1": true}}
}

// The loopback peer speaks PostgreSQL's wire protocol to real pgx connections.
// Its source rows are the committed receipt transcript above; its small store
// commits atomically to a journal so reopen tests exercise durable recovery.
type repair1038Database struct {
	mu          sync.Mutex
	state       repair1038State
	listener    net.Listener
	connections map[net.Conn]bool
	file        string
	wg          sync.WaitGroup
}

func repair1038OpenDatabase(t *testing.T, file string, initial *repair1038State) *repair1038Database {
	t.Helper()
	d := &repair1038Database{file: file, connections: map[net.Conn]bool{}}
	if initial != nil {
		d.state = repair1038Clone(*initial)
		if err := os.WriteFile(file, repair1038JSON(d.state), 0600); err != nil {
			t.Fatal(err)
		}
	} else {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, &d.state); err != nil {
			t.Fatal(err)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	d.listener = listener
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		for {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			d.mu.Lock()
			d.connections[c] = true
			d.mu.Unlock()
			d.wg.Add(1)
			go func() {
				defer d.wg.Done()
				defer c.Close()
				d.serve(c)
				d.mu.Lock()
				delete(d.connections, c)
				d.mu.Unlock()
			}()
		}
	}()
	t.Cleanup(d.Close)
	return d
}
func (d *repair1038Database) Close() {
	_ = d.listener.Close()
	d.mu.Lock()
	for c := range d.connections {
		_ = c.Close()
	}
	d.mu.Unlock()
	d.wg.Wait()
}
func (d *repair1038Database) snapshot() repair1038State {
	d.mu.Lock()
	defer d.mu.Unlock()
	return repair1038Clone(d.state)
}
func (d *repair1038Database) change(t *testing.T, fn func(*repair1038State)) {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	fn(&d.state)
	if err := os.WriteFile(d.file, repair1038JSON(d.state), 0600); err != nil {
		t.Fatal(err)
	}
}
func (d *repair1038Database) pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig("postgres://fixture@" + d.listener.Addr().String() + "/fixture?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	cfg.MaxConns = 4
	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

type repair1038Result struct {
	Rows [][]any
	Tag  string
	Err  error
}

func repair1038Rows(values ...any) repair1038Result {
	return repair1038Result{Rows: [][]any{values}, Tag: "SELECT 1"}
}
func repair1038Empty() repair1038Result { return repair1038Result{Tag: "SELECT 0"} }
func repair1038Args(bind *pgproto3.Bind) []string {
	result := make([]string, len(bind.Parameters))
	for i, b := range bind.Parameters {
		format := int16(0)
		if len(bind.ParameterFormatCodes) == 1 {
			format = bind.ParameterFormatCodes[0]
		} else if len(bind.ParameterFormatCodes) > i {
			format = bind.ParameterFormatCodes[i]
		}
		if b == nil {
			result[i] = ""
		} else if format == 0 {
			result[i] = string(b)
		} else {
			switch len(b) {
			case 2:
				result[i] = strconv.FormatInt(int64(int16(binary.BigEndian.Uint16(b))), 10)
			case 4:
				result[i] = strconv.FormatInt(int64(int32(binary.BigEndian.Uint32(b))), 10)
			case 8:
				result[i] = strconv.FormatInt(int64(binary.BigEndian.Uint64(b)), 10)
			default:
				panic("unsupported fixture binary parameter")
			}
		}
	}
	return result
}
func (d *repair1038Database) serve(c net.Conn) {
	backend := pgproto3.NewBackend(c, c)
	if _, err := backend.ReceiveStartupMessage(); err != nil {
		return
	}
	backend.Send(&pgproto3.AuthenticationOk{})
	for _, p := range [][2]string{{"server_version", "16.0"}, {"client_encoding", "UTF8"}, {"DateStyle", "ISO, MDY"}, {"TimeZone", "UTC"}} {
		backend.Send(&pgproto3.ParameterStatus{Name: p[0], Value: p[1]})
	}
	backend.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
	backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if backend.Flush() != nil {
		return
	}
	var state *repair1038State
	status := byte('I')
	parsed := map[string]string{}
	query := ""
	args := []string{}
	result := repair1038Result{}
	run := func(sql string, args []string) repair1038Result {
		lower := strings.ToLower(strings.TrimSpace(sql))
		switch {
		case strings.HasPrefix(lower, "begin"):
			copy := d.snapshot()
			state = &copy
			status = 'T'
			return repair1038Result{Tag: "BEGIN"}
		case lower == "commit":
			if state == nil {
				return repair1038Result{Err: errors.New("fixture commit without transaction")}
			}
			d.mu.Lock()
			data := repair1038JSON(*state)
			err := os.WriteFile(d.file+".next", data, 0600)
			if err == nil {
				err = os.Rename(d.file+".next", d.file)
			}
			if err == nil {
				d.state = repair1038Clone(*state)
			}
			d.mu.Unlock()
			state = nil
			status = 'I'
			return repair1038Result{Tag: "COMMIT", Err: err}
		case lower == "rollback":
			state = nil
			status = 'I'
			return repair1038Result{Tag: "ROLLBACK"}
		}
		if state != nil {
			return repair1038Query(state, sql, args)
		}
		copy := d.snapshot()
		return repair1038Query(&copy, sql, args)
	}
	sendDescription := func(r repair1038Result) {
		if len(r.Rows) == 0 {
			backend.Send(&pgproto3.NoData{})
			return
		}
		fields := make([]pgproto3.FieldDescription, len(r.Rows[0]))
		for i, v := range r.Rows[0] {
			oid := uint32(25)
			switch v.(type) {
			case bool:
				oid = 16
			case int, int64:
				oid = 20
			case time.Time:
				oid = 1184
			case []byte:
				oid = 3802
			}
			fields[i] = pgproto3.FieldDescription{Name: []byte(fmt.Sprintf("c%d", i)), DataTypeOID: oid, DataTypeSize: -1}
		}
		backend.Send(&pgproto3.RowDescription{Fields: fields})
	}
	sendRows := func(r repair1038Result) {
		if r.Err != nil {
			backend.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: r.Err.Error()})
			if status == 'T' {
				status = 'E'
			}
			return
		}
		for _, row := range r.Rows {
			values := make([][]byte, len(row))
			for i, v := range row {
				switch v := v.(type) {
				case nil:
					values[i] = nil
				case []byte:
					values[i] = v
				case time.Time:
					values[i] = []byte(v.Format("2006-01-02 15:04:05.999999999Z07:00"))
				case bool:
					if v {
						values[i] = []byte("t")
					} else {
						values[i] = []byte("f")
					}
				default:
					values[i] = []byte(fmt.Sprint(v))
				}
			}
			backend.Send(&pgproto3.DataRow{Values: values})
		}
		tag := r.Tag
		if tag == "" {
			tag = fmt.Sprintf("SELECT %d", len(r.Rows))
		}
		backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
	}
	for {
		message, err := backend.Receive()
		if err != nil {
			return
		}
		switch m := message.(type) {
		case *pgproto3.Parse:
			parsed[m.Name] = m.Query
			backend.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Bind:
			query = parsed[m.PreparedStatement]
			args = repair1038Args(m)
			backend.Send(&pgproto3.BindComplete{})
			result = run(query, args)
		case *pgproto3.Describe:
			sendDescription(result)
		case *pgproto3.Execute:
			sendRows(result)
		case *pgproto3.Sync:
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.Query:
			r := run(m.String, nil)
			if len(r.Rows) > 0 {
				sendDescription(r)
			}
			sendRows(r)
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: status})
		case *pgproto3.Close:
			backend.Send(&pgproto3.CloseComplete{})
		case *pgproto3.Terminate:
			return
		default:
			return
		}
		if backend.Flush() != nil {
			return
		}
	}
}

func repair1038LE(a, b string) bool {
	x, ok := new(big.Int).SetString(a, 10)
	if !ok {
		return false
	}
	y, ok := new(big.Int).SetString(b, 10)
	return ok && x.Cmp(y) <= 0
}
func repair1038BalanceRows(events []holdings.Transfer, height string) ([][]any, error) {
	supply := "0"
	balances := map[string]string{}
	for _, e := range events {
		if !repair1038LE(e.Height, height) {
			continue
		}
		from, to := balances[e.From], balances[e.To]
		if from == "" {
			from = "0"
		}
		if to == "" {
			to = "0"
		}
		next, f, t, err := holdings.Mutation(e, supply, from, to)
		if err != nil {
			return nil, err
		}
		supply = next
		if e.From != holdings.Zero {
			balances[e.From] = f
		}
		if e.To != holdings.Zero {
			balances[e.To] = t
		}
	}
	addresses := []string{}
	for a, b := range balances {
		if b != "0" {
			addresses = append(addresses, a)
		}
	}
	sort.Strings(addresses)
	rows := [][]any{}
	for _, a := range addresses {
		rows = append(rows, []any{a, balances[a], supply})
	}
	return rows, nil
}
func repair1038Query(s *repair1038State, sql string, args []string) repair1038Result {
	q := strings.Join(strings.Fields(strings.ReplaceAll(sql, `"`, "")), " ")
	a := func(i int) string {
		if i < 1 || i > len(args) {
			panic("fixture parameter index")
		}
		return args[i-1]
	}
	fail := func(err error) repair1038Result { return repair1038Result{Err: err} }
	key := func(pool, model string) string { return pool + ":" + model }
	switch {
	case q == "SELECT clock_timestamp()":
		return repair1038Rows(s.Now)
	case strings.HasPrefix(q, "SELECT pg_advisory_xact_lock("):
		return repair1038Result{Tag: "SELECT 1"}
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM indexer.indexer_block_completeness_v1"):
		good := s.Complete && a(1) == repair1038Stream
		found := false
		for _, f := range s.Source {
			if f.Position.Height == a(2) && f.Position.BlockHash == a(3) {
				found = true
			}
		}
		return repair1038Rows(good && found)
	case strings.HasPrefix(q, "SELECT token,lower(creator),block_timestamp,height::text,transaction_hash,payload FROM indexer.indexer_canonical_creations_v1"):
		c := s.Creation
		if a(1) != c.Pool || !repair1038LE(c.Height, a(2)) {
			return repair1038Empty()
		}
		return repair1038Rows(c.Token, c.Creator, c.CreatedAt, c.Height, c.Transaction, repair1038JSON(map[string]any{"metadata": map[string]any{"totalSupply": c.Supply}}))
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM holders.tokens"):
		healthy := s.HolderComplete && s.Complete && a(1) == s.Creation.Token && a(3) == repair1038Stream && a(4) == s.Creation.Supply && a(5) == s.Creation.Transaction && a(6) == s.Creation.Height
		source := map[string]holdings.Transfer{}
		local := map[string]holdings.Transfer{}
		for _, e := range s.Transfers {
			if repair1038LE(e.Height, a(2)) {
				source[e.Identity()] = e
			}
		}
		for _, e := range s.HolderEvents {
			if repair1038LE(e.Height, a(2)) {
				local[e.Identity()] = e
			}
		}
		return repair1038Rows(healthy && reflect.DeepEqual(source, local))
	case strings.HasPrefix(q, "WITH deltas AS ("):
		if a(1) != s.Creation.Token || a(3) != holdings.Zero {
			return fail(errors.New("wrong ownership source"))
		}
		rows, err := repair1038BalanceRows(s.HolderEvents, a(2))
		return repair1038Result{Rows: rows, Err: err}
	case strings.HasPrefix(q, "SELECT f.payload,f.source_fact_digest,COALESCE(c.fill"):
		r := repair1038Result{}
		for _, f := range s.Source {
			if f.PoolAddress != a(1) || !repair1038LE(f.Position.Height, a(2)) {
				continue
			}
			digest, err := accounting.FillDigest(f)
			if err != nil {
				return fail(err)
			}
			raw := []byte("null")
			cd := ""
			if c, ok := s.Committed[f.SourceFillID]; ok && a(3) == repair1038Version {
				raw = repair1038JSON(c)
				d, err := accounting.FillDigest(c)
				if err != nil {
					return fail(err)
				}
				cd = string(d)
			}
			r.Rows = append(r.Rows, []any{repair1038JSON(f), string(digest), raw, cd})
		}
		return r
	case strings.HasPrefix(q, "SELECT NOT EXISTS(SELECT 1 FROM stats.accounting_market_commits"):
		good := a(3) == repair1038Version
		for id, c := range s.Committed {
			if c.PoolAddress != a(1) || !repair1038LE(c.Position.Height, a(2)) {
				continue
			}
			found := false
			for _, f := range s.Source {
				if f.SourceFillID == id && reflect.DeepEqual(c, f) {
					found = true
				}
			}
			good = good && found
		}
		return repair1038Rows(good)
	case strings.HasPrefix(q, "SELECT block_hash,payload FROM stats.analytics_risk_inputs"):
		r, ok := s.Inputs[a(1)]
		if !ok || r.Height != a(2) {
			return repair1038Empty()
		}
		return repair1038Rows(r.Hash, repair1038JSON(r.Inputs))
	case strings.HasPrefix(q, "INSERT INTO stats.analytics_risk_inputs"):
		var in analytics.RiskInputs
		if err := json.Unmarshal([]byte(a(4)), &in); err != nil {
			return fail(err)
		}
		old, ok := s.Inputs[a(1)]
		advance := !ok || !repair1038LE(a(2), old.Height)
		if ok && a(2) == old.Height && a(3) == old.Hash && in.ProjectionRevision != "" {
			advance = !repair1038LE(in.ProjectionRevision, old.Inputs.ProjectionRevision) || old.Inputs.ProjectionRevision == ""
		}
		if !advance {
			return repair1038Result{Tag: "INSERT 0 0"}
		}
		s.Inputs[a(1)] = repair1038InputRow{Height: a(2), Hash: a(3), Inputs: in}
		return repair1038Result{Tag: "INSERT 0 1"}
	case strings.HasPrefix(q, "SELECT payload FROM stats.analytics_risk_inputs"):
		r, ok := s.Inputs[a(1)]
		if !ok || r.Height != a(2) || r.Hash != a(3) {
			return repair1038Empty()
		}
		return repair1038Rows(repair1038JSON(r.Inputs))
	case strings.HasPrefix(q, "SELECT digest FROM stats.analytics_trades"):
		r, ok := s.Trades[a(1)]
		if !ok {
			return repair1038Empty()
		}
		return repair1038Rows(r.Digest)
	case strings.HasPrefix(q, "SELECT block_hash=$3 FROM stats.analytics_progress"):
		r, ok := s.Progress[a(1)]
		if !ok || r.Height != a(2) {
			return repair1038Empty()
		}
		return repair1038Rows(r.Hash == a(3))
	case strings.HasPrefix(q, "INSERT INTO stats.analytics_trades"):
		f := accounting.FillV1{SourceFillID: a(1), PoolAddress: a(2), Position: accounting.SourcePosition{Height: a(3), TransactionIndex: a(4), LogIndex: a(5), BlockHash: a(6)}, BlockTimestampSeconds: a(7), Side: a(8), USDLAmountRaw: a(9)}
		s.Trades[a(1)] = repair1038Trade{Fill: f, Digest: a(10)}
		return repair1038Result{Tag: "INSERT 0 1"}
	case strings.HasPrefix(q, "INSERT INTO stats.analytics_progress"):
		old, ok := s.Progress[a(1)]
		if ok {
			cmp := func() int {
				for i, p := range []string{old.Height, old.TI, old.LI} {
					n, _ := new(big.Int).SetString(a(i+2), 10)
					o, _ := new(big.Int).SetString(p, 10)
					if c := n.Cmp(o); c != 0 {
						return c
					}
				}
				return 0
			}()
			if cmp <= 0 {
				return repair1038Empty()
			}
		}
		rev := big.NewInt(1)
		if ok {
			rev.SetString(old.Revision, 10)
			rev.Add(rev, big.NewInt(1))
		}
		s.Progress[a(1)] = repair1038Progress{Height: a(2), TI: a(3), LI: a(4), Hash: a(5), Revision: rev.String(), Observed: a(6)}
		return repair1038Rows(rev.String())
	case strings.HasPrefix(q, "UPDATE stats.analytics_progress SET revision=revision+1"):
		p, ok := s.Progress[a(1)]
		if !ok {
			return repair1038Empty()
		}
		n, _ := new(big.Int).SetString(p.Revision, 10)
		p.Revision = n.Add(n, big.NewInt(1)).String()
		s.Progress[a(1)] = p
		return repair1038Rows(p.Height, p.Hash, p.Revision, p.Observed)
	case strings.HasPrefix(q, "SELECT p.pool FROM stats.analytics_progress"):
		r := repair1038Result{}
		if a(1) == s.Creation.Token {
			if _, ok := s.Progress[s.Creation.Pool]; ok {
				r.Rows = append(r.Rows, []any{s.Creation.Pool})
			}
		}
		return r
	case strings.HasPrefix(q, "SELECT count(*) FILTER(WHERE side='buy')"):
		var buy, sell int64
		bn, sn := new(big.Int), new(big.Int)
		start, _ := strconv.ParseInt(a(2), 10, 64)
		end, _ := strconv.ParseInt(a(3), 10, 64)
		for _, trade := range s.Trades {
			f := trade.Fill
			ts, _ := strconv.ParseInt(f.BlockTimestampSeconds, 10, 64)
			if f.PoolAddress != a(1) || ts <= start || ts > end {
				continue
			}
			n, _ := new(big.Int).SetString(f.USDLAmountRaw, 10)
			if f.Side == "buy" {
				buy++
				bn.Add(bn, n)
			} else {
				sell++
				sn.Add(sn, n)
			}
		}
		return repair1038Rows(buy, sell, bn.String(), sn.String())
	case strings.HasPrefix(q, "SELECT payload->'inputs_watermark'->>'blockHash' FROM stats.analytics_snapshots"):
		r, ok := s.Snapshots[key(a(1), a(2))]
		if !ok || r.Watermark.Height != a(3) {
			return repair1038Empty()
		}
		return repair1038Rows(r.Watermark.BlockHash)
	case strings.HasPrefix(q, "SELECT payload=$3::jsonb FROM stats.analytics_snapshots"):
		r, ok := s.Snapshots[key(a(1), a(2))]
		if !ok || r.Watermark.Height != a(4) || r.Watermark.ProjectionRevision != a(5) {
			return repair1038Empty()
		}
		var expected analytics.Snapshot
		if err := json.Unmarshal([]byte(a(3)), &expected); err != nil {
			return fail(err)
		}
		return repair1038Rows(reflect.DeepEqual(r, expected))
	case strings.HasPrefix(q, "INSERT INTO stats.analytics_snapshots"):
		var in analytics.Snapshot
		if err := json.Unmarshal([]byte(a(5)), &in); err != nil {
			return fail(err)
		}
		old, ok := s.Snapshots[key(a(1), a(2))]
		if ok && repair1038LE(a(3), old.Watermark.Height) && (a(3) != old.Watermark.Height || repair1038LE(a(4), old.Watermark.ProjectionRevision)) {
			return repair1038Result{Tag: "INSERT 0 0"}
		}
		s.Snapshots[key(a(1), a(2))] = in
		return repair1038Result{Tag: "INSERT 0 1"}
	case strings.HasPrefix(q, "INSERT INTO stats.analytics_outbox"):
		var in analytics.Snapshot
		if err := json.Unmarshal([]byte(a(4)), &in); err != nil {
			return fail(err)
		}
		for _, old := range s.Outbox {
			if old.Pool == a(1) && old.Model == a(2) && old.Watermark.ProjectionRevision == a(3) {
				return fail(errors.New("duplicate durable outbox revision"))
			}
		}
		s.Outbox = append(s.Outbox, in)
		return repair1038Result{Tag: "INSERT 0 1"}
	case strings.HasPrefix(q, "SELECT height::text,revision::text,payload FROM stats.analytics_snapshots"):
		r, ok := s.Snapshots[key(a(1), a(2))]
		if !ok {
			return repair1038Empty()
		}
		return repair1038Rows(r.Watermark.Height, r.Watermark.ProjectionRevision, repair1038JSON(r))
	case strings.HasPrefix(q, "SELECT id::text,model,revision::text,payload FROM stats.analytics_outbox"):
		r := repair1038Result{}
		after, _ := strconv.Atoi(a(2))
		limit, _ := strconv.Atoi(a(3))
		for i, item := range s.Outbox {
			if item.Pool == a(1) && i+1 > after && len(r.Rows) < limit {
				r.Rows = append(r.Rows, []any{strconv.Itoa(i + 1), item.Model, item.Watermark.ProjectionRevision, repair1038JSON(item)})
			}
		}
		return r
	default:
		return fail(fmt.Errorf("query absent from the committed protocol peer: %s", q))
	}
}

func repair1038Projector(t *testing.T, d *repair1038Database) (*analytics.Projector, *pgxpool.Pool) {
	t.Helper()
	pool := d.pool(t)
	p, err := app.NewAnalyticsProjector(analytics.ReactionAuthority{Pool: pool, Schema: "stats"}, "holders", "indexer", repair1038Stream, repair1038Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Inputs.(analytics.CanonicalRiskInputs); !ok {
		t.Fatal("app wired a passive risk table reader")
	}
	return p, pool
}
func repair1038Apply(t *testing.T, p *analytics.Projector, d *repair1038Database, index int, commit bool) error {
	t.Helper()
	s := d.snapshot()
	fill := s.Source[index]
	e, wire := repair1038Envelope(t, fill, index+1, s.Now)
	decoded, err := delivery.Decode(wire, repair1038Policy(), accounting.MarketSubject)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(e, decoded) {
		return errors.New("real delivery codec changed the producer publication")
	}
	d.change(t, func(s *repair1038State) { s.Committed[fill.SourceFillID] = fill })
	tx, err := p.Authority.Pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = p.Effect(context.Background(), tx, decoded); err != nil {
		return err
	}
	if commit {
		return tx.Commit(context.Background())
	}
	return nil
}
func repair1038Serve(t *testing.T, a analytics.ReactionAuthority) *httptest.Server {
	t.Helper()
	router := chi.NewRouter()
	httpapi.RegisterDurableAnalytics(router, a)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}
func repair1038Read(t *testing.T, server *httptest.Server, pool string) analytics.Snapshot {
	t.Helper()
	response, err := server.Client().Get(server.URL + "/v2/stats/" + pool + "/risk")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != 200 {
		t.Fatal(response.StatusCode, string(body), err)
	}
	var read analytics.SnapshotRead
	if err = json.Unmarshal(body, &read); err != nil {
		t.Fatal(err)
	}
	if err = read.Snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
	return read.Snapshot
}
func repair1038Risk(t *testing.T, s analytics.Snapshot) risk.V1Result {
	t.Helper()
	var result risk.V1Result
	if err := json.Unmarshal(s.Payload, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestRepairM_1038_RiskInputsAppWiringLateHoldersAndRestart(t *testing.T) {
	initial := repair1038Fixture(t)
	file := filepath.Join(t.TempDir(), "committed.json")
	d := repair1038OpenDatabase(t, file, &initial)
	projector, pool := repair1038Projector(t, d)
	for i := range initial.Source {
		if err := repair1038Apply(t, projector, d, i, true); err != nil {
			t.Fatal(err)
		}
	}
	state := d.snapshot()
	input, ok := state.Inputs[initial.Creation.Pool]
	if !ok || input.Inputs.CreatedAt != initial.Creation.CreatedAt || input.Inputs.HolderCount != -1 || input.Inputs.UniqueTraders24h < 1 || input.Inputs.SupplyRaw != "" {
		t.Fatal("missing genuine source coverage was not disclosed", input)
	}
	server := repair1038Serve(t, projector.Authority)
	before := repair1038Read(t, server, initial.Creation.Pool)
	partial := repair1038Risk(t, before)
	if partial.Score != nil || partial.Coverage != "incomplete" || !reflect.DeepEqual(partial.Unavailable, []string{"top10_concentration", "creator_holdings", "holders", "supply"}) {
		t.Fatal(partial)
	}
	// Transfer ingestion advances after the final market fill. No new trade is
	// supplied to make the risk route complete.
	d.change(t, func(s *repair1038State) {
		s.HolderEvents = append([]holdings.Transfer(nil), s.Transfers...)
		s.HolderComplete = true
	})
	if err := projector.RefreshTokenInputs(context.Background(), initial.Creation.Token); err != nil {
		t.Fatal(err)
	}
	after := repair1038Read(t, server, initial.Creation.Pool)
	complete := repair1038Risk(t, after)
	if complete.Score == nil || complete.Coverage != "complete" || len(complete.Unavailable) != 0 || after.Watermark.Height != before.Watermark.Height || after.Watermark.BlockHash != before.Watermark.BlockHash || repair1038LE(after.Watermark.ProjectionRevision, before.Watermark.ProjectionRevision) {
		t.Fatal("late holder catchup was not projected", before, after, complete)
	}
	persisted := d.snapshot()
	if err := repair1038Apply(t, projector, d, len(initial.Source)-1, true); err != nil {
		t.Fatal(err)
	}
	duplicate := d.snapshot()
	if !reflect.DeepEqual(duplicate.Inputs, persisted.Inputs) || !reflect.DeepEqual(duplicate.Snapshots, persisted.Snapshots) || !reflect.DeepEqual(duplicate.Outbox, persisted.Outbox) {
		t.Fatal("redelivery changed a qualified source identity")
	}
	in := persisted.Inputs[initial.Creation.Pool].Inputs
	if in.SupplyRaw != "1000000000000000" || in.Top10Concentration != "10000" || in.CreatorHoldingsPct != "0" || in.HolderCount < 2 || in.ProjectionRevision != after.Watermark.ProjectionRevision {
		t.Fatal(in)
	}
	if len(persisted.Trades) != len(initial.Source) || len(persisted.Outbox) != 2*(len(initial.Source)+1) {
		t.Fatal("refresh re-ingested trades or lost its durable outbox")
	}
	replay, err := projector.Authority.ReplaySnapshots(context.Background(), initial.Creation.Pool, "0", 100)
	if err != nil || len(replay.Items) != len(persisted.Outbox) {
		t.Fatal(replay, err)
	}
	server.Close()
	pool.Close()
	d.Close()
	reopened := repair1038OpenDatabase(t, file, nil)
	recovered, recoveredPool := repair1038Projector(t, reopened)
	_ = recoveredPool
	resumed := repair1038Serve(t, recovered.Authority)
	got := repair1038Read(t, resumed, initial.Creation.Pool)
	if !reflect.DeepEqual(got, after) || !reflect.DeepEqual(reopened.snapshot().Inputs, persisted.Inputs) {
		t.Fatal("restart lost the durable canonical snapshot", got, after)
	}
	if err := recovered.RefreshTokenInputs(context.Background(), initial.Creation.Token); err != nil {
		t.Fatal(err)
	}
	refreshed := repair1038Read(t, resumed, initial.Creation.Pool)
	if !reflect.DeepEqual(repair1038Risk(t, refreshed).Factors, complete.Factors) || refreshed.Watermark.Height != after.Watermark.Height {
		t.Fatal("restart refresh changed canonical inputs")
	}
}

func TestRepairM_1038_RiskInputsAtomicRollbackAndConflicts(t *testing.T) {
	initial := repair1038Fixture(t)
	initial.HolderComplete = true
	initial.HolderEvents = append([]holdings.Transfer(nil), initial.Transfers...)
	d := repair1038OpenDatabase(t, filepath.Join(t.TempDir(), "committed.json"), &initial)
	p, _ := repair1038Projector(t, d)
	if err := repair1038Apply(t, p, d, 0, false); err != nil {
		t.Fatal(err)
	}
	state := d.snapshot()
	if len(state.Inputs) != 0 || len(state.Progress) != 0 || len(state.Trades) != 0 || len(state.Snapshots) != 0 || len(state.Outbox) != 0 {
		t.Fatal("risk input escaped projection rollback")
	}
	if err := repair1038Apply(t, p, d, 0, true); err != nil {
		t.Fatal(err)
	}
	committed := d.snapshot()
	wm := committed.Snapshots[initial.Creation.Pool+":risk-v1"].Watermark
	// Equal-revision payload conflicts remain errors, including writes made
	// through the original persistence entry point.
	tx, err := p.Authority.Pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	in := committed.Inputs[initial.Creation.Pool].Inputs
	in.HolderCount++
	if err = p.Authority.PersistRiskInputsTx(context.Background(), tx, initial.Creation.Pool, wm, in); !errors.Is(err, analytics.ErrConflict) {
		t.Fatal("same revision conflict accepted", err)
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	d.change(t, func(s *repair1038State) { s.Complete = false })
	if err = p.RefreshTokenInputs(context.Background(), initial.Creation.Token); !errors.Is(err, analytics.ErrDependencyUnavailable) {
		t.Fatal("uncommitted source accepted", err)
	}
	after := d.snapshot()
	if !reflect.DeepEqual(after.Inputs, committed.Inputs) || !reflect.DeepEqual(after.Snapshots, committed.Snapshots) || !reflect.DeepEqual(after.Progress, committed.Progress) || len(after.Outbox) != len(committed.Outbox) {
		t.Fatal("failed source refresh committed partial state")
	}
	d.change(t, func(s *repair1038State) {
		s.Complete = true
		c := s.Committed[initial.Source[0].SourceFillID]
		if c.Attribution.TransactionSender == s.Creation.Creator {
			t.Fatal("conflict fixture requires distinct recorded trader and creator")
		}
		c.Attribution.TransactionSender = s.Creation.Creator
		s.Committed[c.SourceFillID] = c
	})
	if err = p.RefreshTokenInputs(context.Background(), initial.Creation.Token); !errors.Is(err, analytics.ErrConflict) {
		t.Fatal("mutated committed trader fact accepted", err)
	}
	if !reflect.DeepEqual(d.snapshot().Inputs, committed.Inputs) {
		t.Fatal("conflicting trader inputs replaced durable inputs")
	}
}

func TestRepairM_1038_RiskInputsPositionAndExpiry(t *testing.T) {
	initial := repair1038Fixture(t)
	initial.HolderComplete = true
	initial.HolderEvents = append([]holdings.Transfer(nil), initial.Transfers...)
	d := repair1038OpenDatabase(t, filepath.Join(t.TempDir(), "committed.json"), &initial)
	p, _ := repair1038Projector(t, d)
	if err := repair1038Apply(t, p, d, 0, true); err != nil {
		t.Fatal(err)
	}
	first := d.snapshot()
	in := first.Inputs[initial.Creation.Pool].Inputs
	// Later genuine transfers are already in the holder store. They must not
	// alter an earlier position's risk snapshot.
	expectedRows, err := repair1038BalanceRows(initial.Transfers, initial.Source[0].Position.Height)
	if err != nil {
		t.Fatal(err)
	}
	if in.HolderCount != len(expectedRows) || in.UniqueTraders24h != 1 {
		t.Fatal("future ownership or traders leaked into the anchor", in)
	}
	source := p.Inputs.(analytics.CanonicalRiskInputs)
	wm := first.Snapshots[initial.Creation.Pool+":risk-v1"].Watermark
	wm.ProjectionRevision = "2"
	wm.BlockHash = "0x" + strings.Repeat("f", 64)
	tx, err := p.Authority.Pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = source.LoadRiskInputs(context.Background(), tx, initial.Creation.Pool, wm); !errors.Is(err, analytics.ErrDependencyUnavailable) {
		t.Fatal("wrong block hash admitted", err)
	}
	_ = tx.Rollback(context.Background())
	d.change(t, func(s *repair1038State) { s.Now = s.Now.Add(24 * time.Hour) })
	if err = p.RefreshTokenInputs(context.Background(), initial.Creation.Token); err != nil {
		t.Fatal(err)
	}
	expired := d.snapshot().Inputs[initial.Creation.Pool].Inputs
	if expired.UniqueTraders24h != 0 || expired.HolderCount != in.HolderCount || expired.SupplyRaw != in.SupplyRaw || expired.Top10Concentration != in.Top10Concentration {
		t.Fatal("fixed activity window did not expire independently of ownership", expired)
	}
	// Missing committed facts at the same height are reported as unknown, never
	// counted as either a measured zero or an inferred wallet.
	d.change(t, func(s *repair1038State) { delete(s.Committed, initial.Source[0].SourceFillID) })
	if err = p.RefreshTokenInputs(context.Background(), initial.Creation.Token); err != nil {
		t.Fatal(err)
	}
	if got := d.snapshot().Inputs[initial.Creation.Pool].Inputs.UniqueTraders24h; got != -1 {
		t.Fatal("missing committed trader coverage became measured", got)
	}
}

// A small loopback NATS protocol peer supplies the broker side of the pause
// interface. Publications queue while the named durable is paused; resumption
// drains the identical encoded publications through a real NATS connection.
type repair1038Broker struct {
	listener  net.Listener
	mu        sync.Mutex
	pending   [][]byte
	paused    bool
	delivered int
	conn      net.Conn
	wg        sync.WaitGroup
}

func repair1038OpenBroker(t *testing.T) *repair1038Broker {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	b := &repair1038Broker{listener: listener}
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		c, err := listener.Accept()
		if err != nil {
			return
		}
		b.mu.Lock()
		b.conn = c
		b.mu.Unlock()
		defer c.Close()
		b.serve(c)
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		b.mu.Lock()
		if b.conn != nil {
			_ = b.conn.Close()
		}
		b.mu.Unlock()
		b.wg.Wait()
	})
	return b
}
func (b *repair1038Broker) backlog() (int, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.pending), b.delivered
}
func (b *repair1038Broker) serve(c net.Conn) {
	_, _ = fmt.Fprintf(c, "INFO {\"server_id\":\"receipt-peer\",\"version\":\"2.12.2\",\"proto\":1,\"max_payload\":1048576}\r\n")
	reader := bufio.NewReader(c)
	subscriptions := map[string]string{}
	send := func(subject string, body []byte) {
		for sid, pattern := range subscriptions {
			if pattern == subject || strings.HasSuffix(pattern, "*") && strings.HasPrefix(subject, strings.TrimSuffix(pattern, "*")) {
				_, _ = fmt.Fprintf(c, "MSG %s %s %d\r\n", subject, sid, len(body))
				_, _ = c.Write(body)
				_, _ = c.Write([]byte("\r\n"))
			}
		}
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "CONNECT":
		case "PING":
			_, _ = c.Write([]byte("PONG\r\n"))
		case "PONG":
		case "SUB":
			if len(fields) >= 3 {
				subscriptions[fields[len(fields)-1]] = fields[1]
			}
		case "UNSUB":
			if len(fields) == 2 {
				delete(subscriptions, fields[1])
			}
		case "PUB":
			n, err := strconv.Atoi(fields[len(fields)-1])
			if err != nil || n < 0 || n > 1<<20 {
				return
			}
			wire := make([]byte, n+2)
			if _, err = io.ReadFull(reader, wire); err != nil {
				return
			}
			body := wire[:n]
			subject := fields[1]
			reply := ""
			if len(fields) == 4 {
				reply = fields[2]
			}
			switch subject {
			case "$JS.API.CONSUMER.PAUSE.MARKET.analytics":
				var request struct {
					Until time.Time `json:"pause_until"`
				}
				if json.Unmarshal(body, &request) != nil {
					return
				}
				b.mu.Lock()
				b.paused = !request.Until.IsZero()
				response := repair1038JSON(map[string]any{"type": "io.nats.jetstream.api.v1.consumer_pause_response", "paused": b.paused, "pause_until": request.Until})
				send(reply, response)
				if !b.paused {
					for _, queued := range b.pending {
						send("delivered.market", queued)
						b.delivered++
					}
					b.pending = nil
				}
				b.mu.Unlock()
			case accounting.MarketSubject:
				b.mu.Lock()
				if b.paused {
					b.pending = append(b.pending, bytes.Clone(body))
				} else {
					send("delivered.market", body)
					b.delivered++
				}
				b.mu.Unlock()
			default:
				send(subject, body)
			}
		default:
			return
		}
	}
}

func TestRepairM_1038_CapacityPauseSLOConflict(t *testing.T) {
	initial := repair1038Fixture(t)
	initial.HolderComplete = true
	initial.HolderEvents = append([]holdings.Transfer(nil), initial.Transfers...)
	d := repair1038OpenDatabase(t, filepath.Join(t.TempDir(), "paused.json"), &initial)
	p, _ := repair1038Projector(t, d)
	if err := repair1038Apply(t, p, d, 0, true); err != nil {
		t.Fatal(err)
	}
	server := repair1038Serve(t, p.Authority)
	plan := phase4Plan{Origin: server.URL, Processes: []phase4Process{{Role: "stats", PID: os.Getpid()}}}
	if err := phase4BindOrigin(plan); err != nil {
		t.Fatal("origin is not bound to the actual serving process", err)
	}
	broker := repair1038OpenBroker(t)
	nc, err := nats.Connect("nats://"+broker.listener.Addr().String(), nats.NoReconnect(), nats.Timeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	subscription, err := nc.SubscribeSync("delivered.market")
	if err != nil {
		t.Fatal(err)
	}
	defer subscription.Unsubscribe()
	if err = nc.FlushTimeout(time.Second); err != nil {
		t.Fatal(err)
	}
	config := natsdriver.ConsumerConfig{Stream: "MARKET", Durable: "analytics"}
	until := time.Now().UTC().Add(30 * time.Second)
	if err = phase4SetConsumerPause(context.Background(), nc, config, until); err != nil {
		t.Fatal(err)
	}
	for i, fill := range initial.Source {
		if i == 0 {
			continue
		}
		_, wire := repair1038Envelope(t, fill, i+1, initial.Now)
		if err = nc.Publish(accounting.MarketSubject, wire); err != nil {
			t.Fatal(err)
		}
	}
	if err = nc.FlushTimeout(time.Second); err != nil {
		t.Fatal(err)
	}
	pending, delivered := broker.backlog()
	if pending != len(initial.Source)-1 || delivered != 0 {
		t.Fatal("consumer-only fault did not accumulate a measured backlog", pending, delivered)
	}
	latencies := []time.Duration{}
	before := d.snapshot()
	for n := 0; n < 40; n++ {
		start := time.Now()
		snapshot := repair1038Read(t, server, initial.Creation.Pool)
		latencies = append(latencies, time.Since(start))
		if snapshot.Watermark != before.Snapshots[initial.Creation.Pool+":risk-v1"].Watermark {
			t.Fatal("paused consumer changed served durable state")
		}
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	if latencies[(len(latencies)-1)*95/100] > 150*time.Millisecond || latencies[(len(latencies)-1)*99/100] > 400*time.Millisecond {
		t.Fatal("read SLO changed during the delivery fault", latencies)
	}
	recovery := time.Now()
	if err = phase4SetConsumerPause(context.Background(), nc, config, time.Time{}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(initial.Source); i++ {
		message, err := subscription.NextMsg(time.Second)
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := delivery.Decode(message.Data, repair1038Policy(), accounting.MarketSubject)
		if err != nil {
			t.Fatal(err)
		}
		publication, err := accounting.DecodePublicationV1(envelope.Payload)
		if err != nil {
			t.Fatal(err)
		}
		d.change(t, func(s *repair1038State) { s.Committed[publication.Fill.SourceFillID] = *publication.Fill })
		tx, err := p.Authority.Pool.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if err = p.Effect(context.Background(), tx, envelope); err != nil {
			_ = tx.Rollback(context.Background())
			t.Fatal(err)
		}
		if err = tx.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	pending, delivered = broker.backlog()
	if pending != 0 || delivered != len(initial.Source)-1 || time.Since(recovery) > 2*time.Second {
		t.Fatal("measured backlog catchup failed", pending, delivered, time.Since(recovery))
	}
	control := repair1038OpenDatabase(t, filepath.Join(t.TempDir(), "uninterrupted.json"), &initial)
	reference, _ := repair1038Projector(t, control)
	for i := range initial.Source {
		if err := repair1038Apply(t, reference, control, i, true); err != nil {
			t.Fatal(err)
		}
	}
	recovered, expected := d.snapshot(), control.snapshot()
	if !reflect.DeepEqual(recovered.Inputs, expected.Inputs) || !reflect.DeepEqual(recovered.Snapshots, expected.Snapshots) || !reflect.DeepEqual(recovered.Trades, expected.Trades) || !reflect.DeepEqual(recovered.Progress, expected.Progress) || !reflect.DeepEqual(recovered.Outbox, expected.Outbox) {
		t.Fatal("paused delivery recovery differs from uninterrupted durable state")
	}
	t.Logf("maximum backlog=%d recovered=%d recovery=%s reads=%d failures=0 p95=%s p99=%s", len(initial.Source)-1, delivered, time.Since(recovery), len(latencies), latencies[(len(latencies)-1)*95/100], latencies[(len(latencies)-1)*99/100])
}

func TestRepairM_1038_OriginAndConsumerBindingsFailClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer server.Close()
	plan := phase4Plan{Origin: server.URL, Processes: []phase4Process{{Role: "stats", PID: os.Getpid()}}}
	if err := phase4BindOrigin(plan); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []phase4Plan{{Origin: server.URL}, {Origin: server.URL, Processes: []phase4Process{{Role: "indexer", PID: os.Getpid()}}}, {Origin: server.URL, Processes: []phase4Process{{Role: "stats", PID: 1}}}, {Origin: server.URL + "/proxy", Processes: plan.Processes}, {Origin: "http://127.0.0.1", Processes: plan.Processes}} {
		if err := phase4BindOrigin(bad); err == nil {
			t.Fatal("unverified serving origin admitted", bad)
		}
	}
	if err := phase4SetConsumerPause(context.Background(), nil, natsdriver.ConsumerConfig{Stream: "MARKET", Durable: "analytics"}, time.Now()); err == nil {
		t.Fatal("absent consumer transport admitted")
	}
	authority := analytics.ReactionAuthority{Schema: "stats"}
	if _, err := app.NewAnalyticsProjector(authority, "holders", "indexer", repair1038Stream, repair1038Version); err == nil {
		t.Fatal("missing production authority admitted")
	}
	initial := repair1038Fixture(t)
	d := repair1038OpenDatabase(t, filepath.Join(t.TempDir(), "binding.json"), &initial)
	pool := d.pool(t)
	authority.Pool = pool
	for _, bad := range [][4]string{{"holders;DROP SCHEMA stats", "indexer", repair1038Stream, repair1038Version}, {"holders", "indexer", "", repair1038Version}, {"holders", "indexer", repair1038Stream, ""}} {
		if _, err := app.NewAnalyticsProjector(authority, bad[0], bad[1], bad[2], bad[3]); err == nil {
			t.Fatal("incomplete source binding admitted", bad)
		}
	}
}
