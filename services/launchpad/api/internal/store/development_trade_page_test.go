package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestDevelopmentTradePageRequiresAndReturnsCommittedProjection(t *testing.T) {
	model, db := productionAccountingFixture(t)
	ctx := context.Background()
	insertProductionFill(t, db, 1, "")
	page, err := model.Trades(ctx, store.AccountingPageRequest{Pool: productionAddress, Limit: 10})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("committed accounting trade: %+v, %v", page, err)
	}
	var trade struct {
		EventID        string `json:"eventId"`
		PoolAddress    string `json:"poolAddress"`
		Sender         string `json:"sender"`
		Side           string `json:"side"`
		TokenAmountRaw string `json:"tokenAmountRaw"`
		USDLAmountRaw  string `json:"usdlAmountRaw"`
		FeeRaw         string `json:"feeRaw"`
		Position       struct {
			Height           string `json:"height"`
			TransactionHash  string `json:"transactionHash"`
			LogIndex         string `json:"logIndex"`
			TransactionIndex string `json:"transactionIndex"`
		} `json:"position"`
	}
	if err := json.Unmarshal(page.Items[0], &trade); err != nil || trade.EventID == "" || trade.Side != "buy" {
		t.Fatalf("trade document: %+v, %v", trade, err)
	}
	for _, statement := range []string{
		`CREATE TABLE indexer.swaps(id text PRIMARY KEY,pool_id text,pool_address text,sender text,is_buy boolean,amount_in text,amount_out text,fee text,block_number bigint,tx_hash text,log_index integer)`,
		`CREATE SCHEMA api_stream_dev`,
		`CREATE TABLE api_stream_dev.trade_projection(swap_id text PRIMARY KEY,pool_address text,block_number bigint,transaction_index bigint,log_index integer,event_id text,payload bytea)`,
	} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.swaps VALUES($1,'pool-id',$2,$3,true,$4,$5,$6,$7,$8,$9)`, trade.EventID, trade.PoolAddress, trade.Sender, trade.USDLAmountRaw, trade.TokenAmountRaw, trade.FeeRaw, trade.Position.Height, trade.Position.TransactionHash, trade.Position.LogIndex); err != nil {
		t.Fatal(err)
	}
	st := store.New(db, nil)
	request := store.AccountingPageRequest{Pool: productionAddress, Side: "buy", Limit: 10}
	if _, err := st.DevelopmentTrades(ctx, request); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("unprojected trade accepted: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO api_stream_dev.trade_projection VALUES($1,$2,$3,$4,$5,$6,$7)`, trade.EventID, trade.PoolAddress, trade.Position.Height, trade.Position.TransactionIndex, trade.Position.LogIndex, trade.EventID, page.Items[0]); err != nil {
		t.Fatal(err)
	}
	items, err := st.DevelopmentTrades(ctx, request)
	if err != nil || len(items) != 1 || string(items[0]) != string(page.Items[0]) {
		t.Fatalf("projected trade page: %s, %v", items, err)
	}
	request.Side = "sell"
	items, err = st.DevelopmentTrades(ctx, request)
	if err != nil || len(items) != 0 {
		t.Fatalf("sell filter returned buy trade: %s, %v", items, err)
	}
}
