package rest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/records"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/ethereum/go-ethereum/common"
)

var repair828At = time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)

func repair828Write(t *testing.T, db *sql.DB, block store.Block, rows ...decode.Row) {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO perps.blocks(number,hash,parent_hash,time,proposer,tx_count)
VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(number) DO UPDATE SET time=EXCLUDED.time`, block.Number, block.Hash.Bytes(), block.ParentHash.Bytes(), block.Time, block.Proposer, block.TxCount); err != nil {
		t.Fatal(err)
	}
	result := &decode.Result{}
	for _, row := range rows {
		name := ""
		switch row.(type) {
		case decode.MarkPriceUpdatedRow:
			name = "MarkPriceUpdated"
		case decode.FundingSettledRow:
			name = "FundingSettled"
		case decode.IndexPriceRow:
			name = decode.EventAggregate
		case decode.IndexPriceSkippedRow:
			name = decode.EventAggregateSkipped
		case decode.SpotTokenRegisteredRow:
			name = "TokenRegistered"
		case decode.SpotMarketOrderRow:
			name = "MarketOrderExecuted"
		case decode.SpotSimpleSwapRow:
			name = "SimpleSwap"
		case decode.SpotLimitCreatedRow:
			name = "LimitOrderCreated"
		case decode.SpotOrderExecutedRow:
			name = "LimitOrderExecuted"
		default:
			t.Fatalf("unsupported fixture row %T", row)
		}
		result.Decoded = append(result.Decoded, decode.Decoded{Event: decode.Event{EventName: name}, Rows: []decode.Row{row}})
	}
	if err = store.WriteBlock(context.Background(), tx, block, result); err != nil {
		t.Fatalf("WriteBlock: %v", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func repair828Fact(n uint64, index int64, kind string, account common.Address) (store.Block, decode.Fact) {
	b := store.Block{Number: n, Hash: common.BigToHash(new(big.Int).SetUint64(n)), Time: repair828At.Add(time.Duration(n) * time.Second), Proposer: strings.Repeat("AB", 20), TxCount: 1}
	return b, decode.Fact{Block: n, BlockHash: b.Hash, BlockTime: b.Time, TxHash: b.Hash, LogIndex: index, ReceiptKind: decode.ReceiptTx, Emitter: decode.SpotPoolAddress, Kind: kind, Account: account, TxFrom: &account}
}

func TestRepairM_828_LatestIndex(t *testing.T) {
	db, _ := repair828Database(t, "latest_index")
	b, f := repair828Fact(1, 0, "mark", decode.PerpsAddress)
	f.Emitter = decode.PerpsAddress
	repair828Write(t, db, b, decode.MarkPriceUpdatedRow{Fact: f, MarketID: "0", MarkPrice: e18(t, "100"), IndexPrice: e18(t, "99"), MidPrice: e18(t, "100")})
	b, f = repair828Fact(2, 0, "settled", decode.PerpsAddress)
	f.Emitter = decode.PerpsAddress
	repair828Write(t, db, b, decode.FundingSettledRow{Fact: f, MarketID: "0", FundingRate: "-123456789012345678", LongPayment: "0", ShortPayment: "0"})
	markets, err := NewMarkets(db, func() time.Time { return repair828At.Add(time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	symbol, err := NewSymbol(db)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	markets.Register(mux)
	symbol.Register(mux)
	for _, step := range []struct {
		n       uint64
		skipped bool
		price   string
	}{{3, false, "102"}, {4, true, "102"}} {
		b, f = repair828Fact(step.n, 0, "aggregate", decode.PricefeedAddress)
		f.Emitter = decode.PricefeedAddress
		f.ReceiptKind = decode.ReceiptBlockEvent
		f.TxFrom = nil
		f.TxIndex = -1
		if step.skipped {
			f.Kind = "skipped"
			repair828Write(t, db, b, decode.IndexPriceSkippedRow{Fact: f, MarketID: "0", Sources: "1", Required: "2"})
		} else {
			repair828Write(t, db, b, decode.IndexPriceRow{Fact: f, MarketID: "0", IndexPrice: e18(t, step.price), MarkPrice: e18(t, "100"), Basis: e18(t, "-2"), TimestampMs: "1769940003000", Sources: "3", Required: "2"})
		}
		var one marketBody
		spotGet(t, mux, "/api/markets/BTC", http.StatusOK, &one)
		var all marketsBody
		spotGet(t, mux, "/api/markets", http.StatusOK, &all)
		var perp records.PerpStats
		spotGet(t, mux, "/api/BTC/perp_stats", http.StatusOK, &perp)
		var price records.Price
		spotGet(t, mux, "/api/BTC/price", http.StatusOK, &price)
		if len(all.Markets) == 0 {
			t.Fatal("empty market list")
		}
		spotEqual(t, "all vs one", all.Markets[0], one.Market)
		rec := one.Market
		spotEqual(t, "mark", rec.PerpMarkPrice, spotPtr(100.0))
		spotEqual(t, "index", rec.PerpIndexPrice, spotPtr(102.0))
		spotEqual(t, "symbol index", rec.PerpIndexPrice, perp.IndexPrice)
		spotEqual(t, "symbol basis", rec.PerpBasisBps, perp.BasisBps)
		spotEqual(t, "basis", rec.PerpBasisBps, spotPtr(-196.078))
		spotEqual(t, "price", rec.Price, price.Price)
		observed := repair828At.Add(3 * time.Second)
		spotEqual(t, "observation", rec.LastUpdated, spotPtr(observed.UnixMilli()))
		spotEqual(t, "symbol observation", price.Timestamp, spotPtr(spotISO(observed)))
		spotEqual(t, "exact hourly", rec.LastFundingRateHourly, spotPtr("-0.123456789012345678"))
		spotEqual(t, "exact eight hours", rec.Funding8h, spotPtr("-0.987654312098765424"))
		spotEqual(t, "perp hourly", rec.PerpFundingRate, rec.LastFundingRateHourly)
	}
}

func TestRepairM_828_UnknownKind(t *testing.T) {
	// Validation must happen before any database or node access.
	mux := spotMux(&SpotUsers{})
	for _, kind := range []string{"streak", "unknown"} {
		spotRefused(t, mux, "/v1/leaderboards/"+kind, http.StatusNotFound, "unknown leaderboard "+kind)
		spotRefused(t, mux, "/v1/leaderboards/"+kind+"?limit=bad", http.StatusNotFound, "unknown leaderboard "+kind)
	}
	spotRefused(t, mux, "/v1/leaderboards/pnl?limit=bad", http.StatusBadRequest, "invalid limit")
}

func TestRepairM_828_DurableBoard(t *testing.T) {
	db, reopen := repair828Database(t, "durable_board")
	repair828BoardScenario(t, db, reopen)
}

func repair828BoardScenario(t *testing.T, db *sql.DB, reopen func() *sql.DB) {
	t.Helper()
	accounts := []common.Address{common.HexToAddress("0x8281"), common.HexToAddress("0x8282"), common.HexToAddress("0x8283")}
	token := common.HexToAddress("0x828f")
	b, f := repair828Fact(10, 0, "token_registered", decode.SpotVaultAddress)
	f.Emitter = decode.SpotVaultAddress
	stable := decode.SpotTokenRegisteredRow{Fact: f, Token: store.USDL, Decimals: 6, IsStablecoin: true}
	f.LogIndex = 1
	repair828Write(t, db, b, stable, decode.SpotTokenRegisteredRow{Fact: f, Token: token, Decimals: 6})
	swap := func(n uint64, account common.Address, buy bool, amount, tokens string) {
		b, f := repair828Fact(n, 0, "market_order", account)
		repair828Write(t, db, b, decode.SpotMarketOrderRow{Fact: f, User: account, Stablecoin: store.USDL, Token: token, StablecoinAmount: amount, TokenAmount: tokens, IsBuy: buy})
	}
	// Exact values exceed float64's integer range; order is computed by the
	// production fold, not by fixture ranking inserts.
	swap(11, accounts[0], true, "123456789012345678901234567890", "100")
	swap(12, accounts[0], false, "123456789012345678901234567897", "100")
	swap(13, accounts[1], true, "123456789012345678901234567990", "100")
	swap(14, accounts[1], false, "100", "100")
	// Attribution refusal must persist through an unrelated valid update.
	swap(15, accounts[2], false, "50", "5")
	swap(16, accounts[1], true, "10", "10")
	users := &SpotUsers{db: db}
	mux := spotMux(users)
	var original spotLeaderboardBody
	fields := spotGet(t, mux, "/v1/leaderboards/pnl", http.StatusOK, &original)
	wantKeys(t, fields, "kind", "as_of", "omitted_accounts", "items")
	var generated time.Time
	var omitted int
	if err := db.QueryRow(`SELECT generated_at, omitted_accounts FROM perps.spot_board_generation WHERE singleton = true`).Scan(&generated, &omitted); err != nil {
		t.Fatal(err)
	}
	want := spotLeaderboardBody{Kind: "pnl", AsOf: spotISO(generated), OmittedAccounts: 1, Items: []spotRankJSON{
		{Rank: 1, Address: spotHex(accounts[0]), Value: "7"}, {Rank: 2, Address: spotHex(accounts[1]), Value: "-123456789012345678901234567890"}}}
	spotEqual(t, "published generation", original, want)
	spotEqual(t, "persisted omissions", omitted, 1)
	if generated.IsZero() {
		t.Fatal("missing generation time")
	}
	// Stop the consumer and reopen its database connection. The producer is
	// paused: requests cannot cause another fold or a fresher timestamp.
	db = reopen()
	users = &SpotUsers{db: db}
	mux = spotMux(users)
	lock, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err = lock.Exec(`LOCK TABLE perps.spot_swaps, perps.spot_order_events, perps.spot_orders,
perps.spot_token_registry, perps.spot_fee_events, perps.spot_account_legs IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	bounded := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
	responses := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		go func() {
			response := httptest.NewRecorder()
			bounded.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/leaderboards/pnl", nil))
			responses <- response
		}()
	}
	for range 2 {
		select {
		case response := <-responses:
			if response.Code != http.StatusOK {
				t.Fatalf("paused read: %d %s", response.Code, response.Body.String())
			}
			var body spotLeaderboardBody
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			spotEqual(t, "paused generation after reopen", body, want)
		case <-time.After(3 * time.Second):
			t.Fatal("projection read waited for a fold")
		}
	}
	for _, tc := range []struct {
		kind  string
		items []spotRankJSON
	}{
		{"volume", []spotRankJSON{{Rank: 1, Address: spotHex(accounts[0]), Value: "246913578024691357802469135787"}, {Rank: 2, Address: spotHex(accounts[1]), Value: "123456789012345678901234568100"}}},
		{"win-rate", []spotRankJSON{{Rank: 1, Address: spotHex(accounts[0]), Value: "100.0000"}, {Rank: 2, Address: spotHex(accounts[1]), Value: "0.0000"}}},
	} {
		for _, suffix := range []string{"", "?limit=999999999999999999999999999999"} {
			var body spotLeaderboardBody
			fields := spotGet(t, bounded, "/v1/leaderboards/"+tc.kind+suffix, http.StatusOK, &body)
			spotEqual(t, "exact ranked "+tc.kind, body, spotLeaderboardBody{Kind: tc.kind, AsOf: want.AsOf, OmittedAccounts: 1, Items: tc.items})
			var items []map[string]json.RawMessage
			if err := json.Unmarshal(fields["items"], &items); err != nil {
				t.Fatal(err)
			}
			for _, item := range items {
				wantKeys(t, item, "rank", "address", "value")
			}
		}
		var limited spotLeaderboardBody
		spotGet(t, bounded, "/v1/leaderboards/"+tc.kind+"?limit=1", http.StatusOK, &limited)
		spotEqual(t, "bounded ranks", limited.Items, tc.items[:1])
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := users.projectedRank(ctx, "volume", 2); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled projection: %v", err)
	}
	for _, kind := range []string{"streak", "unknown"} {
		spotRefused(t, bounded, "/v1/leaderboards/"+kind, http.StatusNotFound, "unknown leaderboard "+kind)
	}
	spotRefused(t, bounded, "/v1/leaderboards/volume?limit=bad", http.StatusBadRequest, "invalid limit")
	if n := users.board.folds.Load(); n != 0 {
		t.Fatalf("%d request-time folds", n)
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	// A new producer generation is immediately visible without TTL expiry.
	swap(17, accounts[1], false, "20", "10")
	var next spotLeaderboardBody
	spotGet(t, mux, "/v1/leaderboards/pnl", http.StatusOK, &next)
	if next.AsOf <= original.AsOf {
		t.Fatalf("generation did not advance: %s <= %s", next.AsOf, original.AsOf)
	}
	spotEqual(t, "omission retained in later generation", next.OmittedAccounts, 1)
	spotEqual(t, "updated pnl", next.Items, []spotRankJSON{{Rank: 1, Address: spotHex(accounts[0]), Value: "7"}, {Rank: 2, Address: spotHex(accounts[1]), Value: "-123456789012345678901234567880"}})
	// A transaction that fails to commit must leave metadata and ranks together.
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	b, f = repair828Fact(18, 0, "market_order", accounts[0])
	if err = store.FoldSpotBoard(context.Background(), tx, []decode.Row{decode.SpotMarketOrderRow{Fact: f, User: accounts[0]}}); err != nil {
		t.Fatal(err)
	}
	var uncommitted spotLeaderboardBody
	spotGet(t, mux, "/v1/leaderboards/pnl", http.StatusOK, &uncommitted)
	spotEqual(t, "uncommitted generation is invisible", uncommitted, next)
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var rolled spotLeaderboardBody
	spotGet(t, mux, "/v1/leaderboards/pnl", http.StatusOK, &rolled)
	spotEqual(t, "atomic rollback", rolled, next)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	spotRefused(t, mux, "/v1/leaderboards/pnl", http.StatusInternalServerError, "internal error")
}

func TestRepairM_828_EmptyBoard(t *testing.T) {
	db, reopen := repair828Database(t, "empty_board")
	mux := spotMux(&SpotUsers{db: db})
	var body spotLeaderboardBody
	fields := spotGet(t, mux, "/v1/leaderboards/pnl", http.StatusOK, &body)
	spotEqual(t, "unpublished board", body, spotLeaderboardBody{Kind: "pnl", Items: []spotRankJSON{}})
	if string(fields["items"]) != "[]" {
		t.Fatalf("empty items: %s", fields["items"])
	}
	if !reflect.DeepEqual(body.Items, []spotRankJSON{}) {
		t.Fatal("nil items")
	}
	b, f := repair828Fact(20, 0, "token_registered", decode.SpotVaultAddress)
	f.Emitter = decode.SpotVaultAddress
	repair828Write(t, db, b, decode.SpotTokenRegisteredRow{Fact: f, Token: store.USDL, Decimals: 6, IsStablecoin: true})
	account := common.HexToAddress("0x828a")
	b, f = repair828Fact(21, 0, "market_order", account)
	repair828Write(t, db, b, decode.SpotMarketOrderRow{Fact: f, User: account, Stablecoin: store.USDL, Token: common.HexToAddress("0x828f"), StablecoinAmount: "50", TokenAmount: "5", IsBuy: false})
	var at time.Time
	if err := db.QueryRow(`SELECT generated_at FROM perps.spot_board_generation WHERE singleton = true`).Scan(&at); err != nil {
		t.Fatal(err)
	}
	db = reopen()
	mux = spotMux(&SpotUsers{db: db})
	fields = spotGet(t, mux, "/v1/leaderboards/pnl", http.StatusOK, &body)
	spotEqual(t, "published empty generation", body, spotLeaderboardBody{Kind: "pnl", AsOf: spotISO(at), OmittedAccounts: 1, Items: []spotRankJSON{}})
	if string(fields["items"]) != "[]" {
		t.Fatalf("empty published items: %s", fields["items"])
	}
}
