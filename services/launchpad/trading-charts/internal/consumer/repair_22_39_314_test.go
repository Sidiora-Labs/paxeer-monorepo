package consumer

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/migrate"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/migrations"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/redis/go-redis/v9"
)

// Emit a real Swap on a local chain. TIMESTAMP and NUMBER in the emitter write
// the source metadata into the event, so the bootstrap verifies real receipts.
func repair314Source(t *testing.T) (*rpc.Client, developmentSourceRow) {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	emitter := common.HexToAddress("0x1234")
	runtime := []byte{0x60, 0xa0, 0x60, 0x60, 0x60, 0x00, 0x37, 0x42, 0x60, 0xa0, 0x52, 0x43, 0x60, 0xc0, 0x52, 0x60, 0x40, 0x35, 0x60, 0x20, 0x35, 0x60, 0x00, 0x35, 0x60, 0xe0, 0x60, 0x00, 0xa3, 0x00}
	backend := simulated.NewBackend(types.GenesisAlloc{
		crypto.PubkeyToAddress(key.PublicKey): {Balance: new(big.Int).Exp(big.NewInt(10), big.NewInt(21), nil)},
		emitter:                               {Code: runtime},
	}, func(n *node.Config, eth *ethconfig.Config) {
		chain := *eth.Genesis.Config
		chain.ChainID = big.NewInt(125)
		eth.Genesis.Config = &chain
		n.HTTPHost = "127.0.0.1"
		n.HTTPPort = port
		n.HTTPModules = []string{"eth"}
	})
	t.Cleanup(func() { _ = backend.Close() })
	backend.Commit()
	row := developmentSourceRow{poolID: common.HexToHash("0x5678").Hex(), pool: "0x1111111111111111111111111111111111111111", sender: "0x2222222222222222222222222222222222222222", buy: true, amountIn: "1000000", amountOut: "2000000000000000000", fee: "1000", price: "500000"}
	data := append([]byte{}, swapEventTopic.Bytes()...)
	data = append(data, common.HexToHash(row.poolID).Bytes()...)
	data = append(data, common.LeftPadBytes(common.HexToAddress(row.sender).Bytes(), 32)...)
	for _, value := range []string{"1", row.amountIn, row.amountOut, row.fee, row.price} {
		v, _ := new(big.Int).SetString(value, 10)
		data = append(data, common.LeftPadBytes(v.Bytes(), 32)...)
	}
	client := backend.Client()
	ctx := context.Background()
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	tip, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	recipient := crypto.PubkeyToAddress(key.PublicKey)
	preceding, err := types.SignNewTx(key, types.LatestSignerForChainID(big.NewInt(125)), &types.DynamicFeeTx{ChainID: big.NewInt(125), Gas: 21000, GasTipCap: tip, GasFeeCap: new(big.Int).Add(tip, new(big.Int).Mul(head.BaseFee, big.NewInt(2))), To: &recipient})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.SendTransaction(ctx, preceding); err != nil {
		t.Fatal(err)
	}
	tx, err := types.SignNewTx(key, types.LatestSignerForChainID(big.NewInt(125)), &types.DynamicFeeTx{ChainID: big.NewInt(125), Nonce: 1, Gas: 200000, GasTipCap: tip, GasFeeCap: new(big.Int).Add(tip, new(big.Int).Mul(head.BaseFee, big.NewInt(2))), To: &emitter, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.SendTransaction(ctx, tx); err != nil {
		t.Fatal(err)
	}
	backend.Commit()
	receipt, err := client.TransactionReceipt(ctx, tx.Hash())
	if err != nil || receipt.Status != types.ReceiptStatusSuccessful || len(receipt.Logs) != 1 {
		t.Fatalf("Swap receipt: %+v %v", receipt, err)
	}
	head, err = client.HeaderByNumber(ctx, receipt.BlockNumber)
	if err != nil {
		t.Fatal(err)
	}
	row.height, row.timestamp, row.logIndex, row.hash = receipt.BlockNumber.Int64(), int64(head.Time), int64(receipt.Logs[0].Index), strings.ToLower(receipt.TxHash.Hex())
	raw, err := rpc.DialContext(ctx, "http://127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(raw.Close)
	return raw, row
}

func TestRepair_22_39_314_DevelopmentSwapCarriesPosition(t *testing.T) {
	ctx := context.Background()
	_, pool := internaltest.NewPostgres(t)
	opt, err := redis.ParseURL(internaltest.NewRedisURL(t))
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	st := store.New(pool)
	builder := engine.New(pool, rdb, st, logger)
	rpc, row := repair314Source(t)
	source := NewDevelopmentSource(pool, rpc, builder, logger)
	frame, err := source.frame(ctx, row)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	swap, err := DecodeDevelopmentSwap(wire)
	if err != nil {
		t.Fatal(err)
	}
	want := engine.ChainOrder{Height: uint64(row.height), TransactionIndex: 1, LogIndex: uint64(row.logIndex)}
	if swap.Position == nil || *swap.Position != want || swap.LogIndex != int(row.logIndex) {
		t.Fatalf("source position lost: %+v want %+v", swap, want)
	}
	if err = builder.ProcessSwap(ctx, swap); err != nil {
		t.Fatalf("positioned source rejected: %v", err)
	}
	start := row.timestamp / 60 * 60
	candle, err := st.GetCandle(ctx, row.pool, "1m", start)
	if err != nil || candle == nil || candle.TradeCount != 1 || candle.VolumeUsdl != row.amountIn {
		t.Fatalf("committed candle %+v: %v", candle, err)
	}
	// A restart uses the same claim and ordered candle state without refolding.
	builder = engine.New(pool, rdb, st, logger)
	if err = builder.ProcessSwap(ctx, swap); err != nil {
		t.Fatal(err)
	}
	candle, err = st.GetCandle(ctx, row.pool, "1m", start)
	if err != nil || candle.TradeCount != 1 {
		t.Fatalf("restart doubled source: %+v %v", candle, err)
	}
	for _, missing := range []string{"blockNumber", "transactionIndex", "logIndex"} {
		var incomplete map[string]json.RawMessage
		if err = json.Unmarshal(wire, &incomplete); err != nil {
			t.Fatal(err)
		}
		delete(incomplete, missing)
		bad, err := json.Marshal(incomplete)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = DecodeDevelopmentSwap(bad); err == nil {
			t.Fatalf("frame missing %s accepted", missing)
		}
		incomplete[missing] = json.RawMessage("null")
		bad, err = json.Marshal(incomplete)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = DecodeDevelopmentSwap(bad); err == nil {
			t.Fatalf("frame with null %s accepted", missing)
		}
	}
	// Preserve the canonical position envelope added on main, and reject any
	// disagreement with the development source's explicit indexes.
	frame.Position = &want
	positioned, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeDevelopmentSwap(positioned)
	if err != nil || decoded.Position == nil || *decoded.Position != want {
		t.Fatalf("canonical position envelope rejected: %+v %v", decoded, err)
	}
	for _, field := range []string{"height", "transaction", "log"} {
		conflict := want
		switch field {
		case "height":
			conflict.Height++
		case "transaction":
			conflict.TransactionIndex++
		case "log":
			conflict.LogIndex++
		}
		frame.Position = &conflict
		bad, err := json.Marshal(frame)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = DecodeDevelopmentSwap(bad); err == nil {
			t.Fatalf("conflicting %s position accepted", field)
		}
	}
	frame.Position, frame.TransactionIndex = &want, nil
	positioned, err = json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err = DecodeDevelopmentSwap(positioned)
	if err != nil || decoded.Position == nil || *decoded.Position != want {
		t.Fatalf("canonical position without duplicate transaction index rejected: %+v %v", decoded, err)
	}
	swap.Position = nil
	if err = builder.ProcessSwap(ctx, swap); err == nil {
		t.Fatal("builder accepted missing position")
	}
}

func TestRepair_22_39_314_DevelopmentSchemaMigrates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dsn := internaltest.NewDatabase(t)
	if err := migrate.Up(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool := internaltest.OpenPool(t, dsn)
	var table string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('candles.development_state')::text`).Scan(&table); err != nil || table != "candles.development_state" {
		t.Fatalf("embedded migration omitted development state: %s %v", table, err)
	}
	raw, err := migrations.FS.ReadFile("00003_development_state.sql")
	if err != nil || !strings.Contains(string(raw), engine.DevelopmentSchemaDDL) {
		t.Fatalf("migration differs from engine schema: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.development_state(key,payload) VALUES('migration-proof',$1)`, []byte("committed")); err != nil {
		t.Fatal(err)
	}
	if err = migrate.Up(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	var payload []byte
	if err = pool.QueryRow(ctx, `SELECT payload FROM candles.development_state WHERE key='migration-proof'`).Scan(&payload); err != nil || string(payload) != "committed" {
		t.Fatalf("repeat migration lost state: %q %v", payload, err)
	}
}
