package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	spotabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot/abi"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/source"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
)

type repairM827Producer struct {
	c        *chain
	db       *sql.DB
	registry *decode.Registry
}

func newRepairM827Producer(t *testing.T, db *sql.DB, state ...decode.StateReader) *repairM827Producer {
	t.Helper()
	metrics, err := telemetry.New(config.Indexer)
	if err != nil {
		t.Fatal(err)
	}
	opts := decode.Options{PerpsModule: decode.ModuleAddress("perps"),
		SpotModule: decode.ModuleAddress("spot"), PricefeedModule: decode.ModuleAddress("pricefeed")}
	if len(state) != 0 {
		opts.State = state[0]
	}
	registry, err := decode.NewRegistry(metrics, opts)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &chain{t: t, ctx: context.Background(), tx: tx}
	p := &repairM827Producer{c: c, db: db, registry: registry}
	t.Cleanup(func() {
		if err := c.tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			t.Error(err)
		}
		if err := p.db.Close(); err != nil {
			t.Error(err)
		}
	})
	return p
}

func (p *repairM827Producer) reopen() {
	c := p.c
	c.t.Helper()
	raw := os.Getenv(testDatabaseEnv)
	if raw == "" {
		c.t.Fatal(testDatabaseEnv + " is required")
	}
	dsn, err := url.Parse(raw)
	if err != nil {
		c.t.Fatalf("parse %s: %v", testDatabaseEnv, err)
	}
	var database string
	if err := c.tx.QueryRowContext(c.ctx, `SELECT current_database()`).Scan(&database); err != nil {
		c.t.Fatal(err)
	}
	dsn.Path = "/" + database
	if err := c.tx.Commit(); err != nil {
		c.t.Fatal(err)
	}
	if err := p.db.Close(); err != nil {
		c.t.Fatal(err)
	}
	db, err := Open(dsn.String())
	if err != nil {
		c.t.Fatal(err)
	}
	p.db = db
	c.tx, err = db.BeginTx(c.ctx, nil)
	if err != nil {
		c.t.Fatal(err)
	}
}

func repairM827Perps(t *testing.T, name string, values ...any) decode.Log {
	t.Helper()
	topics, data, err := perpsabi.PackEvent(name, values...)
	if err != nil {
		t.Fatal(err)
	}
	return decode.Log{Log: ethtypes.Log{Address: decode.PerpsAddress, Topics: topics, Data: data}, ReceiptKind: decode.ReceiptTx, TxFrom: &traderA}
}

func repairM827Spot(t *testing.T, load func() (abi.ABI, error), emitter common.Address, name string, values ...any) decode.Log {
	t.Helper()
	schema, err := load()
	if err != nil {
		t.Fatal(err)
	}
	event, ok := schema.Events[name]
	if !ok || len(event.Inputs) != len(values) {
		t.Fatalf("invalid event fixture %s", name)
	}
	topics := []common.Hash{event.ID}
	var args []any
	for i, input := range event.Inputs {
		if input.Indexed {
			words, err := abi.MakeTopics([]any{values[i]})
			if err != nil {
				t.Fatal(err)
			}
			topics = append(topics, words[0][0])
		} else {
			args = append(args, values[i])
		}
	}
	data, err := event.Inputs.NonIndexed().Pack(args...)
	if err != nil {
		t.Fatal(err)
	}
	return decode.Log{Log: ethtypes.Log{Address: emitter, Topics: topics, Data: data}, ReceiptKind: decode.ReceiptTx, TxFrom: &traderA}
}

func repairM827Trade(t *testing.T, account common.Address, buy bool, stable, tokens int64) decode.Log {
	t.Helper()
	return repairM827Spot(t, spotabi.PECOR, decode.SpotPoolAddress, "MarketOrderExecuted", account, USDL, tokenPAX, big.NewInt(stable), big.NewInt(tokens), buy)
}

func (p *repairM827Producer) write(n uint64, at time.Time, logs ...decode.Log) {
	c := p.c
	c.t.Helper()
	c.next(n, at)
	c.blockRow(n, at)
	b := header(n, at)
	c.writeBlock(b, p.decode(b, 0, logs))
}

func (p *repairM827Producer) decode(b Block, firstLog uint, logs []decode.Log) *decode.Result {
	c := p.c
	c.t.Helper()
	for i := range logs {
		logs[i].BlockNumber, logs[i].BlockHash, logs[i].BlockTime = b.Number, b.Hash, b.Time
		logs[i].Index, logs[i].TxHash = firstLog+uint(i), crypto.Keccak256Hash(b.Hash.Bytes())
	}
	result, err := p.registry.Decode(context.Background(), c.tx, logs)
	if err != nil {
		c.t.Fatal(err)
	}
	if len(result.Raw) != 0 || len(result.Decoded) != len(logs) {
		c.t.Fatalf("decoder rejected fixture: %d decoded, %d raw, %d logs", len(result.Decoded), len(result.Raw), len(logs))
	}
	return result
}

func repairM827Exec(c *chain, query string, args ...any) {
	c.t.Helper()
	if _, err := c.tx.ExecContext(c.ctx, query, args...); err != nil {
		c.t.Fatalf("%s: %v", query, err)
	}
}

func repairM827Open(t *testing.T, id int64, account common.Address) decode.Log {
	return repairM827Perps(t, "PositionOpened", big.NewInt(id), account, big.NewInt(0), true, big.NewInt(1000), big.NewInt(10), big.NewInt(60000), USDL, big.NewInt(100))
}

func repairM827Close(t *testing.T, id int64, account common.Address, size, realized int64, full bool) decode.Log {
	return repairM827Perps(t, "PositionClosed", big.NewInt(id), account, big.NewInt(0), big.NewInt(size), big.NewInt(61000), big.NewInt(realized), full)
}

func repairM827Settlement(t *testing.T, id int64, account common.Address, size, gross int64) decode.Log {
	return repairM827Perps(t, "TradeSettled", big.NewInt(id), account, big.NewInt(0), uint8(1), big.NewInt(size), big.NewInt(61000), big.NewInt(gross), big.NewInt(0), big.NewInt(0), big.NewInt(0))
}

func TestRepairM_827_RebuildProjectionInventory(t *testing.T) {
	testRepairM827RebuildInventory(t)
}

func TestRepairM_827_StaggeredMarketExpiry(t *testing.T) {
	testRepairM827MarketExpiry(t)
}

func TestRepairM_827_StaggeredLeaderboardExpiry(t *testing.T) {
	testRepairM827LeaderboardExpiry(t)
}

// This peer serves the fixed getLimitOrder transcript for order 27. The
// production RPC client and decoder consume it; no store behavior is replaced.
func repairM827OrderState(t *testing.T) *source.RPC {
	t.Helper()
	schema, err := spotabi.Orders()
	if err != nil {
		t.Fatal(err)
	}
	record := struct {
		Id                      *big.Int
		User, Stablecoin, Token common.Address
		Amount, TargetPrice     *big.Int
		OrderType, Status       uint8
		CreatedAt, ExpiresAt    *big.Int
	}{big.NewInt(27), traderB, USDL, tokenPAX, big.NewInt(10), big.NewInt(1), 0, 0, big.NewInt(1), big.NewInt(9999999999)}
	result, err := schema.Methods["getLimitOrder"].Outputs.Pack(record)
	if err != nil {
		t.Fatal(err)
	}
	call, err := schema.Pack("getLimitOrder", big.NewInt(27))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "invalid request", 400)
			return
		}
		answer := "0x1000"
		switch request.Method {
		case "eth_blockNumber":
		case "eth_call":
			var args struct {
				To   string `json:"to"`
				Data string `json:"data"`
			}
			if len(request.Params) != 2 {
				t.Error("invalid call parameters")
				http.Error(w, "invalid call", 400)
				return
			}
			if err := json.Unmarshal(request.Params[0], &args); err != nil {
				t.Error(err)
				http.Error(w, "invalid call", 400)
				return
			}
			if common.HexToAddress(args.To) != decode.SpotOrdersAddress || args.Data != hexutil.Encode(call) || string(request.Params[1]) != `"0x64"` {
				t.Errorf("unexpected transcript request: %+v %s", args, request.Params[1])
				http.Error(w, "unrecorded call", 400)
				return
			}
			answer = hexutil.Encode(result)
		default:
			t.Errorf("unrecorded method %s", request.Method)
			http.Error(w, "unrecorded method", 400)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": answer}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	client, err := source.NewRPC(context.Background(), source.RPCOptions{URLs: []string{server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}
