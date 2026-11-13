package rest

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/development"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestDevelopmentCandleProjectsCommittedChartState(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	chain := newRepair58Chain(t)
	chain.emitter = chain.deploy(t, []byte{0x60, 0x60, 0x36, 0x03, 0x80, 0x60, 0x60, 0x60, 0x00, 0x37, 0x42, 0x60, 0xa0, 0x52, 0x43, 0x60, 0xc0, 0x52, 0x60, 0x40, 0x35, 0x60, 0x20, 0x35, 0x60, 0x00, 0x35, 0x83, 0x60, 0x00, 0xa3, 0x00})
	poolAddress := "0x" + strings.Repeat("7a", 20)
	poolID := common.HexToHash("0x" + strings.Repeat("7a", 32))
	sender := strings.ToLower(crypto.PubkeyToAddress(chain.key.PublicKey).Hex())
	if _, err := pool.Exec(ctx, `CREATE TABLE candles.development_state(key text PRIMARY KEY,payload bytea NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.pools(pool_address,token_address,creator,optical,pool_id,created_at,created_block,tx_hash) VALUES($1,$2,$3,$4,$5,1760000000,1,$6)`, poolAddress, strings.ToLower(chain.token.Hex()), sender, zeroAddressHex, poolID.Hex(), common.Hash{}.Hex()); err != nil {
		t.Fatal(err)
	}
	data := append([]byte{}, common.HexToHash(swapEventTopic).Bytes()...)
	data = append(data, poolID.Bytes()...)
	data = append(data, common.BytesToHash(common.HexToAddress(sender).Bytes()).Bytes()...)
	for _, value := range []int64{1, 1000000, 2000000000000000000, 3000, 1, 0, 0} {
		data = append(data, common.LeftPadBytes(big.NewInt(value).Bytes(), 32)...)
	}
	receipt := chain.send(t, &chain.emitter, data)
	if len(receipt.Logs) != 1 {
		t.Fatalf("swap log count: %d", len(receipt.Logs))
	}
	header, err := chain.backend.Client().HeaderByNumber(ctx, receipt.BlockNumber)
	if err != nil {
		t.Fatal(err)
	}
	stamp := int64(header.Time)
	order := development.Order{Height: receipt.BlockNumber.Uint64(), TransactionIndex: uint64(receipt.TransactionIndex), LogIndex: uint64(receipt.Logs[0].Index)}
	source := &development.Swap{PoolID: poolID.Hex(), PoolAddress: poolAddress, Sender: sender, IsBuy: true, AmountIn: "1000000", AmountOut: "2000000000000000000", Fee: "3000", Price: "1", BlockTimestamp: stamp, TxHash: strings.ToLower(receipt.TxHash.Hex()), LogIndex: int(receipt.Logs[0].Index), Position: &order}
	candle := development.Candle{OpenSource: source, HighSource: source, LowSource: source, CloseSource: source, First: order, Last: order, Traders: map[string]bool{sender: true}}
	start := stamp - stamp%60
	row, err := json.Marshal(map[string]any{"Row": map[string]any{"PoolAddress": poolAddress, "Timeframe": "1m", "CandleStart": start, "Open": "1", "High": "1", "Low": "1", "Close": "1", "VolumeUsdl": "1000000", "VolumeToken": "2000000000000000000", "BuyVolumeUsdl": "1000000", "SellVolumeUsdl": "0", "TradeCount": 1, "UniqueTraders": 1, "LastTradeTs": stamp, "SequenceNum": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(row, &candle); err != nil || candle.Row.TradeCount != 1 {
		t.Fatalf("chart state: %+v, %v", candle.Row, err)
	}
	raw, err := json.Marshal(candle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO candles.development_state(key,payload) VALUES($1,$2)`, "candle/"+poolAddress+"/1m", raw); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO candles.candles(pool_address,timeframe,candle_start,open,high,low,close,trade_count,last_trade_ts,sequence_num) VALUES($1,'1m',$2,'1','1','1','1',1,$3,1)`, poolAddress, start, stamp); err != nil {
		t.Fatal(err)
	}
	projector, err := NewDevelopmentStream(ctx, DevelopmentStreamOptions{Pool: pool, Store: store.New(pool, nil), ChainRPCURL: chain.origin, SigningKey: []byte(strings.Repeat("k", 32)), Authority: sharedauth.HTTPStreamAuthority{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := projector.projectCandles(ctx); err != nil {
		t.Fatal(err)
	}
	var payload []byte
	var revision, sequence int64
	if err := pool.QueryRow(ctx, `SELECT payload,source_revision,topic_sequence FROM api_stream_dev.candle_projection WHERE pool_address=$1 AND timeframe='1m' AND candle_start=$2`, poolAddress, start).Scan(&payload, &revision, &sequence); err != nil {
		t.Fatal(err)
	}
	if revision != 1 || sequence != 1 || interfaces.ValidateJSON("Candle", payload) != nil {
		t.Fatalf("projected candle: revision=%d sequence=%d body=%s", revision, sequence, payload)
	}
	var document struct {
		PoolAddress string `json:"poolAddress"`
		Trades      string `json:"trades"`
	}
	if err := json.Unmarshal(payload, &document); err != nil || document.PoolAddress != poolAddress || document.Trades != "1" {
		t.Fatalf("candle body: %+v, %v", document, err)
	}
	request := store.AccountingPageRequest{Pool: poolAddress, Interval: "1m", From: start * 1000, To: (start + 60) * 1000, Limit: 10}
	page, err := store.New(pool, nil).DevelopmentCandlePage(ctx, request)
	if err != nil || page == nil || len(page.Items) != 1 || page.Revision != "1" || page.SnapshotID == "" {
		t.Fatalf("committed candle page: %+v, %v", page, err)
	}
	read := func(r store.AccountingPageRequest, token string) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v2/pools/"+poolAddress+"/candles", nil)
		serveDevelopmentCandles(response, request, store.New(pool, nil), r, token)
		return response
	}
	served := read(request, "")
	if served.Code != http.StatusOK || interfaces.ValidateJSON("CandlesResponse", served.Body.Bytes()) != nil || !strings.Contains(served.Body.String(), page.SnapshotID) || !strings.Contains(served.Body.String(), `"trades":"1"`) {
		t.Fatalf("committed candle HTTP page: %d %s", served.Code, served.Body.String())
	}
	badCursor := read(request, "invalid")
	if badCursor.Code != http.StatusBadRequest || !strings.Contains(badCursor.Body.String(), `"code":"CURSOR_INVALID"`) {
		t.Fatalf("invalid candle cursor: %d %s", badCursor.Code, badCursor.Body.String())
	}
	filter := hex.EncodeToString(accountingFilter(request, true))
	cursorRaw, err := json.Marshal(developmentCandleCursor{Snapshot: page.SnapshotID, Filter: filter, After: start})
	if err != nil {
		t.Fatal(err)
	}
	continued := read(request, base64.RawURLEncoding.EncodeToString(cursorRaw))
	if continued.Code != http.StatusOK || interfaces.ValidateJSON("CandlesResponse", continued.Body.Bytes()) != nil || strings.Contains(continued.Body.String(), `"trades":"1"`) {
		t.Fatalf("continued candle HTTP page: %d %s", continued.Code, continued.Body.String())
	}
	changedCursorRaw, err := json.Marshal(developmentCandleCursor{Snapshot: "another-snapshot", Filter: filter, After: start})
	if err != nil {
		t.Fatal(err)
	}
	changed := read(request, base64.RawURLEncoding.EncodeToString(changedCursorRaw))
	if changed.Code != http.StatusConflict || !strings.Contains(changed.Body.String(), `"code":"RESNAPSHOT_REQUIRED"`) {
		t.Fatalf("changed candle snapshot: %d %s", changed.Code, changed.Body.String())
	}
	request.ExpectedSnapshot = "different-snapshot"
	if _, err := store.New(pool, nil).DevelopmentCandlePage(ctx, request); err != store.ErrDevelopmentCandleSnapshotChanged {
		t.Fatalf("changed candle snapshot: %v", err)
	}
	if err := projector.projectCandles(ctx); err != nil {
		t.Fatalf("repeat projection: %v", err)
	}
	var frames int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM api_stream_dev.stream_delivery_outbox`).Scan(&frames); err != nil || frames != 1 {
		t.Fatalf("unchanged chart state emitted %d frames: %v", frames, err)
	}
}
