package graphql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/graph-gophers/graphql-go/relay"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	spotabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot/abi"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/delivery"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/feeds"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/records"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
)

// PostgreSQL is a recorded peer, not the component under repair. Requests
// must match the captured SQL and parameters; results are actual PostgreSQL
// rows, never calculated by a second implementation of the repaired selector.
// capture.py documents and reproduces their provenance. No external process
// or database is used by this gate.
//
//go:embed testdata/index_price/postgres.json
var indexPriceData embed.FS

type indexPriceQuery struct {
	Name string
	SQL  string
	Args []struct {
		Value        string
		Clock        bool
		ClockSeconds int64 `json:"clock_seconds"`
	}
	Columns []string
	Types   []uint32
	Rows    [][]*string
}

type indexPriceBlock struct {
	Number uint64
	Time   string
	Events []struct {
		Type   string
		Values map[string]string
	}
	Marks []struct {
		Market, Index, Mark string
		Tx, Log             int64
	}
}

type indexPriceScenario struct {
	Name    string
	Blocks  []indexPriceBlock
	Facts   indexPriceQuery
	Queries []indexPriceQuery
}

func indexPriceScenarios(t *testing.T) map[string]*indexPriceScenario {
	t.Helper()
	raw, err := indexPriceData.ReadFile("testdata/index_price/postgres.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct{ Scenarios []*indexPriceScenario }
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]*indexPriceScenario)
	for _, scenario := range data.Scenarios {
		indexPriceDecode(t, scenario)
		out[scenario.Name] = scenario
	}
	if len(out) != 9 {
		t.Fatalf("captured %d scenarios, want 9", len(out))
	}
	return out
}

func indexPriceTime(t *testing.T, value string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

func indexPriceInt(t *testing.T, value string) *big.Int {
	t.Helper()
	n, ok := new(big.Int).SetString(value, 10)
	if !ok {
		t.Fatalf("invalid recorded integer %q", value)
	}
	return n
}

// Decode the recorded finalize_block_events and ABI logs with the production
// codec, then compare every price/source-order column with the stored rows
// that produced the PostgreSQL transcript. This binds the reader replay to
// the producer inputs rather than hand-built reader responses.
func indexPriceDecode(t *testing.T, scenario *indexPriceScenario) {
	t.Helper()
	metrics, err := telemetry.New(config.Indexer)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := decode.NewRegistry(metrics, decode.Options{
		PerpsModule: decode.ModuleAddress("perps"), SpotModule: decode.ModuleAddress("spot"), PricefeedModule: decode.ModuleAddress("pricefeed"),
	})
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := perpsabi.ABI()
	if err != nil {
		t.Fatal(err)
	}
	event := leaf.Events["MarkPriceUpdated"]
	var got [][]*string
	text := func(value any) *string { s := fmt.Sprint(value); return &s }
	for _, block := range scenario.Blocks {
		at := indexPriceTime(t, block.Time)
		hash := common.BigToHash(new(big.Int).SetUint64(block.Number))
		input := decode.BlockInput{Number: block.Number, Hash: hash, Time: at}
		for _, ev := range block.Events {
			record := abci.Event{Type: ev.Type}
			keys := make([]string, 0, len(ev.Values))
			for key := range ev.Values {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				record.Attributes = append(record.Attributes, abci.EventAttribute{Key: []byte(key), Value: []byte(ev.Values[key])})
			}
			input.Events = append(input.Events, record)
		}
		for _, mark := range block.Marks {
			values := []*big.Int{indexPriceInt(t, mark.Market), indexPriceInt(t, mark.Mark), indexPriceInt(t, mark.Index), indexPriceInt(t, mark.Index)}
			topics := []common.Hash{event.ID}
			var data []any
			for i, field := range event.Inputs {
				if field.Indexed {
					encoded, err := abi.MakeTopics([]any{values[i]})
					if err != nil {
						t.Fatal(err)
					}
					topics = append(topics, encoded[0][0])
				} else {
					data = append(data, values[i])
				}
			}
			packed, err := event.Inputs.NonIndexed().Pack(data...)
			if err != nil {
				t.Fatal(err)
			}
			input.Logs = append(input.Logs, decode.Log{
				Log: ethtypes.Log{Address: decode.PerpsAddress, Topics: topics, Data: packed, BlockNumber: block.Number, BlockHash: hash,
					TxIndex: uint(mark.Tx), Index: uint(mark.Log), TxHash: common.BigToHash(new(big.Int).SetUint64(block.Number*100 + uint64(mark.Tx)))},
				ReceiptKind: decode.ReceiptPerpsEndBlock, BlockTime: at,
			})
		}
		result, err := registry.DecodeBlock(context.Background(), nil, input)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Raw) != 0 {
			t.Fatalf("undecoded producer logs: %+v", result.Raw)
		}
		for _, decoded := range result.Decoded {
			for _, row := range decoded.Rows {
				f := row.Common()
				cols := []*string{text(row.Table()), nil, nil, nil, text(f.Block), text(f.TxIndex), text(f.LogIndex), text(f.Sub), text(f.Kind), text(f.BlockTime.UTC().Format(time.RFC3339Nano)), nil, nil, nil, nil}
				switch row := row.(type) {
				case decode.MarkPriceUpdatedRow:
					cols[1], cols[2], cols[3] = text(row.MarketID), text(row.IndexPrice), text(row.MarkPrice)
				case decode.IndexPriceRow:
					cols[1], cols[2], cols[3] = text(row.MarketID), text(row.IndexPrice), text(row.MarkPrice)
					cols[10], cols[11], cols[12], cols[13] = text(row.TimestampMs), text(row.Basis), text(row.Sources), text(row.Required)
				case decode.IndexPriceSkippedRow:
					cols[1], cols[12], cols[13] = text(row.MarketID), text(row.Sources), text(row.Required)
				default:
					t.Fatalf("unexpected decoded row %T", row)
				}
				got = append(got, cols)
			}
		}
	}
	// PostgreSQL represents an empty result as [], while append starts nil.
	if len(got) == 0 {
		got = [][]*string{}
	}
	if !reflect.DeepEqual(got, scenario.Facts.Rows) {
		a, _ := json.Marshal(got)
		b, _ := json.Marshal(scenario.Facts.Rows)
		t.Fatalf("%s decoded producer != stored transcript facts\ngot %s\nwant %s", scenario.Name, a, b)
	}
}

type indexPriceReplay struct {
	mu       sync.Mutex
	scenario *indexPriceScenario
	seen     map[string]int
}

func (r *indexPriceReplay) Connect(context.Context) (driver.Conn, error) {
	return &indexPriceConn{r}, nil
}
func (r *indexPriceReplay) Driver() driver.Driver            { return r }
func (r *indexPriceReplay) Open(string) (driver.Conn, error) { return r.Connect(context.Background()) }
func (r *indexPriceReplay) use(scenario *indexPriceScenario) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scenario = scenario
}

type indexPriceConn struct{ replay *indexPriceReplay }

func (*indexPriceConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("transcript has no prepared-statement exchange")
}
func (*indexPriceConn) Begin() (driver.Tx, error) {
	return nil, errors.New("read-only transcript has no transaction exchange")
}
func (*indexPriceConn) Close() error { return nil }

func indexPriceSQL(sql string) string { return strings.Join(strings.Fields(sql), " ") }

func (c *indexPriceConn) QueryContext(ctx context.Context, sql string, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r := c.replay
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, q := range r.scenario.Queries {
		if indexPriceSQL(q.SQL) != indexPriceSQL(sql) || len(q.Args) != len(args) {
			continue
		}
		matches := true
		for i, arg := range q.Args {
			if arg.Clock {
				at, ok := args[i].Value.(time.Time)
				delta := time.Since(at) + time.Duration(arg.ClockSeconds)*time.Second
				matches = matches && ok && delta >= -time.Second && delta <= 5*time.Second
			} else {
				matches = matches && fmt.Sprint(args[i].Value) == arg.Value
			}
		}
		if !matches {
			continue
		}
		r.seen[q.Name]++
		return &indexPriceRows{query: q}, nil
	}
	return nil, fmt.Errorf("unrecorded database request in %s: %s args=%v", r.scenario.Name, sql, args)
}

type indexPriceRows struct {
	query indexPriceQuery
	next  int
}

func (r *indexPriceRows) Columns() []string { return r.query.Columns }
func (*indexPriceRows) Close() error        { return nil }
func (r *indexPriceRows) Next(dest []driver.Value) error {
	if r.next == len(r.query.Rows) {
		return io.EOF
	}
	for i, value := range r.query.Rows[r.next] {
		if value == nil {
			dest[i] = nil
			continue
		}
		var err error
		switch r.query.Types[i] {
		case 16:
			dest[i] = *value == "t"
		case 20, 21, 23:
			dest[i], err = strconv.ParseInt(*value, 10, 64)
		case 1184:
			dest[i], err = time.Parse(time.RFC3339Nano, *value)
		default:
			dest[i] = *value
		}
		if err != nil {
			return err
		}
	}
	r.next++
	return nil
}

func indexPriceOpen(t *testing.T, scenario *indexPriceScenario) (*sql.DB, *indexPriceReplay) {
	t.Helper()
	replay := &indexPriceReplay{scenario: scenario, seen: map[string]int{}}
	db := sql.OpenDB(replay)
	t.Cleanup(func() { _ = db.Close() })
	return db, replay
}

func indexPriceFloat(t *testing.T, got *float64, want *float64) {
	t.Helper()
	if (got == nil) != (want == nil) || got != nil && math.Abs(*got-*want) > 1e-9 {
		t.Fatalf("number got %v, want %v", got, want)
	}
}

func indexPricePtr(value float64) *float64 { return &value }

type indexPriceWant struct {
	scenario    string
	index, mark *float64
	block       uint64
	seconds     int64
}

var indexPriceCases = []indexPriceWant{
	{"empty", nil, nil, 0, 0},
	{"mark_and_index_only", indexPricePtr(100), indexPricePtr(105), 100, -26 * 3600},
	{"index_without_mark", indexPricePtr(110), indexPricePtr(105), 101, -2 * 3600},
	{"skipped", indexPricePtr(110), indexPricePtr(105), 101, -2 * 3600},
	{"same_block_mark", indexPricePtr(115), indexPricePtr(106), 103, -60},
	{"aggregate_log_order", indexPricePtr(127), indexPricePtr(106), 104, -30},
	{"zero_index", indexPricePtr(0), indexPricePtr(106), 105, -15},
	{"new_mark", indexPricePtr(130), indexPricePtr(107), 106, -10},
	{"zero_baseline", indexPricePtr(127), indexPricePtr(105), 104, -30},
}

func TestIndexPriceSharePointerTable(t *testing.T) {
	share, err := spotabi.Share()
	if err != nil {
		t.Fatal(err)
	}
	if len(share.Events) != 2 || share.Events["Transfer"].Name != "Transfer" || share.Events["Approval"].Name != "Approval" {
		t.Fatalf("unexpected deployed share events: %+v", share.Events)
	}
	vault, err := spotabi.Vault()
	if err != nil {
		t.Fatal(err)
	}
	historical, err := spotabi.HistoricalSolidityShare()
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := telemetry.New(config.Indexer)
	if err != nil {
		t.Fatal(err)
	}
	spotModule := decode.ModuleAddress("spot")
	registry, err := decode.NewRegistry(metrics, decode.Options{
		PerpsModule: decode.ModuleAddress("perps"), SpotModule: spotModule, PricefeedModule: decode.ModuleAddress("pricefeed"),
	})
	if err != nil {
		t.Fatalf("deployed ABI and decoder tables disagree: %v", err)
	}
	token := common.HexToAddress("0x1000000000000000000000000000000000000001")
	pointer := common.HexToAddress("0x1000000000000000000000000000000000000002")
	owner := common.HexToAddress("0x1000000000000000000000000000000000000003")
	peer := common.HexToAddress("0x1000000000000000000000000000000000000004")
	pack := func(event abi.Event, emitter common.Address, index uint, values ...any) decode.Log {
		t.Helper()
		if len(values) != len(event.Inputs) {
			t.Fatalf("%s: %d arguments, want %d", event.Name, len(values), len(event.Inputs))
		}
		topics := []common.Hash{event.ID}
		var data []any
		for i, input := range event.Inputs {
			if input.Indexed {
				encoded, err := abi.MakeTopics([]any{values[i]})
				if err != nil {
					t.Fatal(err)
				}
				topics = append(topics, encoded[0][0])
			} else {
				data = append(data, values[i])
			}
		}
		encoded, err := event.Inputs.NonIndexed().Pack(data...)
		if err != nil {
			t.Fatal(err)
		}
		return decode.Log{
			Log: ethtypes.Log{Address: emitter, Topics: topics, Data: encoded, BlockNumber: 100,
				BlockHash: common.BigToHash(big.NewInt(100)), TxHash: common.BigToHash(big.NewInt(101)), TxIndex: 1, Index: index},
			ReceiptKind: decode.ReceiptTx, TxFrom: &owner, BlockTime: time.Unix(1800000000, 0).UTC(),
		}
	}
	logs := []decode.Log{
		pack(vault.Events["ShareTokenCreated"], decode.SpotVaultAddress, 0, token, pointer),
		pack(share.Events["Transfer"], pointer, 1, owner, peer, big.NewInt(5)),
		pack(share.Events["Approval"], pointer, 2, owner, peer, big.NewInt(7)),
		pack(historical.Events["OwnershipTransferred"], pointer, 3, owner, peer),
	}
	result, err := registry.Decode(context.Background(), nil, logs)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Decoded) != 3 || len(result.Raw) != 1 {
		t.Fatalf("share decoder coverage: %+v", result)
	}
	for i, want := range []struct {
		name string
		row  decode.Row
	}{
		{"ShareTokenCreated", decode.SpotShareTokenCreatedRow{Token: token, ShareToken: pointer}},
		{"Transfer", decode.SpotShareTransferRow{ShareToken: pointer, Token: token, FromAccount: owner, ToAccount: peer, Value: "5"}},
		{"Approval", decode.SpotShareApprovalRow{ShareToken: pointer, Token: token, Holder: owner, Spender: peer, Value: "7"}},
	} {
		got := result.Decoded[i]
		if got.Event.EventName != want.name || len(got.Rows) != 1 {
			t.Fatalf("%s: %+v", want.name, got)
		}
		row := reflect.ValueOf(got.Rows[0])
		expected := reflect.ValueOf(want.row)
		if row.Type() != expected.Type() {
			t.Fatalf("%s: row type %T, want %T", want.name, got.Rows[0], want.row)
		}
		for field := 1; field < row.NumField(); field++ {
			if !reflect.DeepEqual(row.Field(field).Interface(), expected.Field(field).Interface()) {
				t.Fatalf("%s: row %+v, want %+v", want.name, got.Rows[0], want.row)
			}
		}
		account := owner
		if i == 0 {
			account = spotModule
		}
		if got.Rows[0].Common().Account != account {
			t.Fatalf("%s: account %s, want %s", want.name, got.Rows[0].Common().Account, account)
		}
	}
	raw := result.Raw[0]
	if raw.FailureClass != decode.FailureUnknownTopic || raw.Kind != "raw" || raw.Emitter != pointer || raw.Account != spotModule ||
		!reflect.DeepEqual(raw.Topics, logs[3].Topics) || !reflect.DeepEqual(raw.Data, logs[3].Data) {
		t.Fatalf("historical ownership event must remain raw: %+v", raw)
	}
}

func TestIndexPriceProducerToGraphQLAndSnapshots(t *testing.T) {
	scenarios := indexPriceScenarios(t)
	// The same stored producer inputs, read by the exact base-revision SQL,
	// reproduce finding 7: BTC remains 100 instead of 110 and ETH is absent.
	baseline := scenarios["index_without_mark"]
	found := false
	for _, query := range baseline.Queries {
		if query.Name == "baseline_latest" {
			found = true
			if len(query.Rows) != 1 || *query.Rows[0][0] != "1" || *query.Rows[0][1] != "100000000000000000000" || *query.Rows[0][3] != "100" {
				t.Fatalf("baseline did not reproduce stale mark-only selection: %+v", query.Rows)
			}
		}
	}
	if !found {
		t.Fatal("missing recorded base-revision evidence")
	}
	for _, want := range indexPriceCases {
		t.Run(want.scenario, func(t *testing.T) {
			db, replay := indexPriceOpen(t, scenarios[want.scenario])
			schema, err := NewSchema(db)
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			mux.Handle("POST /graphql", &relay.Handler{Schema: schema})
			routes, err := rest.NewSymbol(db)
			if err != nil {
				t.Fatal(err)
			}
			routes.Register(mux)
			query := `{"query":"{ latestPrices { marketId price onchainTimestamp blockNumber updatedAt } market(marketId: 1) { latestPrice { marketId price onchainTimestamp blockNumber updatedAt } poolState { oraclePrice blockNumber updatedAt } } markets { marketId latestPrice { marketId price onchainTimestamp blockNumber updatedAt } } poolState(marketId: 1) { oraclePrice blockNumber updatedAt } poolStates { marketId oraclePrice } orderlyTickers { paxeerSymbol mark_price index_price } }"}`
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(query)))
			var body struct {
				Data struct {
					LatestPrices []LatestPrice
					Market       struct {
						LatestPrice *LatestPrice
						PoolState   *struct {
							OraclePrice *BigDecimal
							BlockNumber *int32
							UpdatedAt   *string
						}
					}
					Markets []struct {
						MarketID    int32
						LatestPrice *LatestPrice
					}
					PoolState *struct {
						OraclePrice *BigDecimal
						BlockNumber *int32
						UpdatedAt   *string
					}
					PoolStates []struct {
						MarketID    int32
						OraclePrice *BigDecimal
					}
					OrderlyTickers []struct {
						PaxeerSymbol string
						Mark         *float64 `json:"mark_price"`
						Index        *float64 `json:"index_price"`
					}
				}
				Errors []any
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || len(body.Errors) != 0 {
				t.Fatalf("GraphQL %d: %s", response.Code, response.Body)
			}
			checkPrice := func(price *LatestPrice) {
				t.Helper()
				if want.index == nil {
					if price != nil {
						t.Fatalf("unexpected price %+v", price)
					}
					return
				}
				at := time.Date(2026, 11, 4, 7, 0, 0, 0, time.UTC).Add(time.Duration(want.seconds) * time.Second)
				raw := new(big.Int).Mul(big.NewInt(int64(*want.index)), big.NewInt(1_000_000_000_000_000_000)).String()
				if price == nil || string(price.Price) != raw || price.BlockNumber != int32(want.block) || price.OnchainTimestamp != int32(at.Unix()) || price.UpdatedAt == nil || !price.UpdatedAt.Equal(at) {
					t.Fatalf("price %+v, want %s at block %d/time %s", price, raw, want.block, at)
				}
			}
			checkPrice(body.Data.Market.LatestPrice)
			if len(body.Data.Markets) != 3 || len(body.Data.OrderlyTickers) != 3 {
				t.Fatalf("market/ticker inventory: %s", response.Body)
			}
			checkPrice(body.Data.Markets[0].LatestPrice)
			for _, pool := range []*struct {
				OraclePrice *BigDecimal
				BlockNumber *int32
				UpdatedAt   *string
			}{body.Data.PoolState, body.Data.Market.PoolState} {
				if want.index == nil {
					if pool != nil {
						t.Fatalf("unexpected pool %+v", pool)
					}
					continue
				}
				if pool == nil || pool.OraclePrice == nil || *pool.OraclePrice != body.Data.Market.LatestPrice.Price || pool.BlockNumber == nil || *pool.BlockNumber != int32(want.block) || pool.UpdatedAt == nil || *pool.UpdatedAt != body.Data.Market.LatestPrice.UpdatedAt.Format(time.RFC3339) {
					t.Fatalf("pool differs from latest price: %+v", pool)
				}
			}
			if want.index == nil {
				if len(body.Data.LatestPrices) != 0 || len(body.Data.PoolStates) != 0 {
					t.Fatalf("empty snapshots: %s", response.Body)
				}
			} else {
				if len(body.Data.LatestPrices) != 2 || body.Data.LatestPrices[0].MarketID != 1 || body.Data.LatestPrices[1].MarketID != 2 || len(body.Data.PoolStates) != 2 {
					t.Fatalf("latest market selection: %s", response.Body)
				}
				checkPrice(&body.Data.LatestPrices[0])
				if body.Data.Markets[1].LatestPrice == nil || body.Data.Markets[1].LatestPrice.Price != "200000000000000000000" || body.Data.Markets[2].LatestPrice != nil {
					t.Fatalf("index-only/skipped-only markets: %s", response.Body)
				}
				if body.Data.PoolStates[0].OraclePrice == nil || *body.Data.PoolStates[0].OraclePrice != body.Data.LatestPrices[0].Price || body.Data.PoolStates[1].OraclePrice == nil || *body.Data.PoolStates[1].OraclePrice != body.Data.LatestPrices[1].Price {
					t.Fatalf("pool list differs: %s", response.Body)
				}
			}
			indexPriceFloat(t, body.Data.OrderlyTickers[0].Index, want.index)
			indexPriceFloat(t, body.Data.OrderlyTickers[0].Mark, want.mark)
			if want.index != nil {
				indexPriceFloat(t, body.Data.OrderlyTickers[1].Index, indexPricePtr(200))
				indexPriceFloat(t, body.Data.OrderlyTickers[1].Mark, nil)
				indexPriceFloat(t, body.Data.OrderlyTickers[2].Index, nil)
			}
			reader, err := records.NewSymbol(db)
			if err != nil {
				t.Fatal(err)
			}
			price, err := reader.Price(context.Background(), "BTC")
			if err != nil {
				t.Fatal(err)
			}
			stats, err := reader.PerpStats(context.Background(), "BTC")
			if err != nil {
				t.Fatal(err)
			}
			indexPriceFloat(t, price.Price, want.index)
			indexPriceFloat(t, stats.IndexPrice, want.index)
			indexPriceFloat(t, stats.MarkPrice, want.mark)
			var basis *float64
			if want.index != nil && *want.index != 0 {
				basis = indexPricePtr(math.Round((*want.mark-*want.index)*10000 / *want.index * 1000) / 1000)
			}
			indexPriceFloat(t, stats.BasisBps, basis)
			if want.index == nil {
				if price.Timestamp != nil {
					t.Fatalf("empty price timestamp %v", price.Timestamp)
				}
			} else if price.Timestamp == nil || *price.Timestamp != body.Data.Market.LatestPrice.UpdatedAt.Format(records.ISOMillis) {
				t.Fatalf("records observation time differs: %+v", price)
			}
			for route, expected := range map[string]any{"price": price, "perp_stats": stats} {
				response := httptest.NewRecorder()
				mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/BTC/"+route, nil))
				expectedJSON, err := json.Marshal(expected)
				if err != nil {
					t.Fatal(err)
				}
				if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != string(expectedJSON) {
					t.Fatalf("REST %s: %d %s, want %s", route, response.Code, response.Body, expectedJSON)
				}
			}
			providers, err := feeds.NewProviders(context.Background(), db, nil)
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := providers.Snapshot(context.Background(), "BTC@perp_stats")
			if err != nil {
				t.Fatal(err)
			}
			expectedJSON, err := json.Marshal(stats)
			if err != nil {
				t.Fatal(err)
			}
			if string(snapshot) != string(expectedJSON) {
				t.Fatalf("socket snapshot %s, want REST/records %s", snapshot, expectedJSON)
			}
			market, err := loadMarket(context.Background(), db, 1)
			if err != nil {
				t.Fatal(err)
			}
			if want.index == nil {
				if market.LatestPrice != nil {
					t.Fatalf("subscription market has unexpected price %+v", market)
				}
			} else if market.LatestPrice == nil || string(market.LatestPrice.Price) != string(body.Data.Market.LatestPrice.Price) || !market.LatestPrice.UpdatedAt.Equal(body.Data.Market.LatestPrice.UpdatedAt.Time) {
				t.Fatalf("subscription market differs %+v", market)
			}
			for _, name := range []string{"latest", "market", "markets", "states", "pool", "tickers", "recordsPrice", "recordsWindow", "recordsState", "subMarketSQL", "feedIDs"} {
				if replay.seen[name] == 0 {
					t.Fatalf("production reader did not execute %s", name)
				}
			}
		})
	}
}

func TestIndexPricePriceChangeWindows(t *testing.T) {
	scenarios := indexPriceScenarios(t)
	for _, tc := range []struct {
		name   string
		latest float64
		short  *float64
		long   float64
	}{
		{"aggregate_log_order", 127, indexPricePtr(17.0 / 110), .27},
		{"zero_index", 0, indexPricePtr(-1), -1},
		{"zero_baseline", 127, nil, .27},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, replay := indexPriceOpen(t, scenarios[tc.name])
			rows, err := (&Resolver{DB: db}).OrderlyPriceChanges(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 2 {
				t.Fatalf("price changes rows %d, want priced markets only", len(rows))
			}
			p := rows[0]
			indexPriceFloat(t, p.LastPrice, indexPricePtr(tc.latest))
			for _, got := range []*float64{p.Change5m, p.Change30m, p.Change1h} {
				indexPriceFloat(t, got, tc.short)
			}
			for _, got := range []*float64{p.Change4h, p.Change24h} {
				indexPriceFloat(t, got, indexPricePtr(tc.long))
			}
			for _, got := range []*float64{p.Change3d, p.Change7d, p.Change30d} {
				indexPriceFloat(t, got, nil)
			}
			if replay.seen["changes"] != 1 {
				t.Fatal("price-change production query did not execute")
			}
		})
	}
}

func TestIndexPriceIndexSubscription(t *testing.T) {
	scenarios := indexPriceScenarios(t)
	db, replay := indexPriceOpen(t, scenarios["mark_and_index_only"])
	feed, err := NewFeed(125, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(withFeed(context.Background(), feed))
	defer cancel()
	updates, err := (&Resolver{DB: db}).LatestPricesUpdated(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct{ scenario, subject, price string }{
		{"index_without_mark", "perps.125.1.index", "110000000000000000000"},
		{"aggregate_log_order", "perps.125.1.index", "127000000000000000000"},
		{"new_mark", "perps.125.1.mark", "130000000000000000000"},
	} {
		replay.use(scenarios[step.scenario])
		feed.Deliver(step.subject, delivery.Envelope{})
		select {
		case snapshot := <-updates:
			if len(snapshot) != 2 || snapshot[0].MarketID != 1 || string(snapshot[0].Price) != step.price || snapshot[1].MarketID != 2 {
				t.Fatalf("%s: subscription snapshot %+v", step.scenario, snapshot)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s did not wake latest index subscription", step.subject)
		}
	}
	// A skipped aggregate must not advance the successful-index cursor.
	replay.use(scenarios["skipped"])
	before, err := latestPriceCursor(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	replay.use(scenarios["index_without_mark"])
	after, err := latestPriceCursor(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("skipped aggregate advanced cursor: %v != %v", before, after)
	}

	// Exercise the public GraphQL websocket transport and its consumer too.
	socketFeed, err := NewFeed(125, nil)
	if err != nil {
		t.Fatal(err)
	}
	replay.use(scenarios["mark_and_index_only"])
	schema, err := NewSchema(db)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(&TransportWS{Schema: schema, Feed: socketFeed})
	defer server.Close()
	client := dialSub(t, server.URL, "/graphql-ws")
	defer client.conn.Close()
	client.init(t)
	client.subscribe(t, "index", `subscription { latestPricesUpdated { marketId price blockNumber onchainTimestamp updatedAt } }`, nil)
	deadline := time.Now().Add(5 * time.Second)
	for {
		socketFeed.mu.Lock()
		count := len(socketFeed.listeners)
		socketFeed.mu.Unlock()
		if count == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("websocket subscription was not registered")
		}
		time.Sleep(time.Millisecond)
	}
	replay.use(scenarios["index_without_mark"])
	socketFeed.Deliver("perps.125.1.index", delivery.Envelope{})
	client.wantNext(t, "index", `{"latestPricesUpdated":[{"marketId":1,"price":"110000000000000000000","blockNumber":101,"onchainTimestamp":1793768400,"updatedAt":"2026-11-04T05:00:00Z"},{"marketId":2,"price":"200000000000000000000","blockNumber":100,"onchainTimestamp":1793682000,"updatedAt":"2026-11-03T05:00:00Z"}]}`)
	replay.use(scenarios["skipped"])
	socketFeed.Deliver("perps.125.1.index", delivery.Envelope{})
	client.quiet(t)
}
