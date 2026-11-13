package rest

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-chi/chi/v5"
)

func TestDevelopmentTradeProjectsVerifiedReceipt(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	raw, err := os.ReadFile("../../../indexer/migrations/00001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"cursors", "swaps"} {
		found := false
		for _, block := range strings.Split(strings.SplitN(string(raw), "-- +goose Down", 2)[0], "-- +goose StatementBegin") {
			statement := strings.SplitN(block, "-- +goose StatementEnd", 2)[0]
			if strings.Contains(statement, `CREATE TABLE "indexer"."`+table+`"`) {
				if _, err := db.Exec(ctx, statement); err != nil {
					t.Fatal(err)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("indexer %s table is absent from its migration", table)
		}
	}
	chain := newRepair58Chain(t)
	pool := "0x" + strings.Repeat("7a", 20)
	poolID := common.HexToHash("0x" + strings.Repeat("7a", 32))
	sender := strings.ToLower(crypto.PubkeyToAddress(chain.key.PublicKey).Hex())
	data := append([]byte{}, common.HexToHash(swapEventTopic).Bytes()...)
	data = append(data, poolID.Bytes()...)
	data = append(data, common.BytesToHash(common.HexToAddress(sender).Bytes()).Bytes()...)
	for _, value := range []int64{1, 1000000, 2000000000000000000, 3000, 1, 0, 0} {
		data = append(data, common.LeftPadBytes(big.NewInt(value).Bytes(), 32)...)
	}
	receipt := chain.send(t, &chain.emitter, data)
	if len(receipt.Logs) != 1 {
		t.Fatalf("swap receipt has %d logs", len(receipt.Logs))
	}
	header, err := chain.backend.Client().HeaderByNumber(ctx, receipt.BlockNumber)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.pools(pool_address,token_address,creator,optical,pool_id,created_at,created_block,tx_hash) VALUES($1,$2,$3,$4,$5,1760000000,1,$6)`, pool, strings.ToLower(chain.token.Hex()), sender, zeroAddressHex, poolID.Hex(), common.Hash{}.Hex()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.swaps(id,pool_id,pool_address,sender,is_buy,amount_in,amount_out,fee,price,block_number,block_timestamp,tx_hash,log_index) VALUES($1,$2,$3,$4,true,'1000000','2000000000000000000','3000','',$5,$6,$7,$8)`, receipt.TxHash.Hex()+"-"+strconv.FormatUint(uint64(receipt.Logs[0].Index), 10), poolID.Hex(), pool, sender, receipt.BlockNumber.Int64(), int64(header.Time), strings.ToLower(receipt.TxHash.Hex()), int32(receipt.Logs[0].Index)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.cursors(chain_id,last_processed_block) VALUES(125,$1)`, receipt.BlockNumber.Int64()); err != nil {
		t.Fatal(err)
	}
	d, err := NewDevelopmentStream(ctx, DevelopmentStreamOptions{Pool: db, Store: store.New(db, nil), ChainRPCURL: chain.origin, SigningKey: []byte(strings.Repeat("k", 32)), Authority: sharedauth.HTTPStreamAuthority{}})
	if err != nil {
		t.Fatal(err)
	}
	touched := map[string]bool{}
	n, err := d.projectTrades(ctx, touched)
	if err != nil || n != 1 || !touched[pool] {
		t.Fatalf("projected %d trades, touched %v: %v", n, touched, err)
	}
	var frame []byte
	if err := db.QueryRow(ctx, `SELECT frame FROM api_stream_dev.stream_projection WHERE topic=$1`, developmentStreamTradesPrefix+pool).Scan(&frame); err != nil || !strings.Contains(string(frame), `"tokenAmountRaw":"2000000000000000000"`) || !strings.Contains(string(frame), receipt.BlockHash.Hex()) {
		t.Fatalf("trade frame: %s, %v", frame, err)
	}
	var count int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM api_stream_dev.stream_delivery_outbox`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("trade and price deliveries = %d: %v", count, err)
	}
	if n, err := d.projectTrades(ctx, map[string]bool{}); err != nil || n != 0 {
		t.Fatalf("repeated projection = %d: %v", n, err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM api_stream_dev.stream_delivery_outbox`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("repeated projection appended %d deliveries: %v", count, err)
	}
	router := chi.NewRouter()
	router.Get("/v2/pools/{poolAddress}/trades", devHistory(d.market, false))
	read := func() (int, struct {
		Data struct {
			Items []json.RawMessage `json:"items"`
		} `json:"data"`
		Watermark *store.ChainWatermark `json:"watermark"`
	}) {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/pools/"+pool+"/trades", nil))
		var document struct {
			Data struct {
				Items []json.RawMessage `json:"items"`
			} `json:"data"`
			Watermark *store.ChainWatermark `json:"watermark"`
		}
		if response.Code == http.StatusOK {
			if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
				t.Fatal(err)
			}
		}
		return response.Code, document
	}
	if status, page := read(); status != http.StatusOK || len(page.Data.Items) != 1 || page.Watermark == nil || page.Watermark.Height != receipt.BlockNumber.String() {
		t.Fatalf("committed trade page: status=%d page=%+v", status, page)
	}
	lagging := receipt.BlockNumber.Int64() - 1
	if _, err := db.Exec(ctx, `UPDATE indexer.cursors SET last_processed_block=$1 WHERE chain_id=125`, lagging); err != nil {
		t.Fatal(err)
	}
	if status, page := read(); status != http.StatusOK || len(page.Data.Items) != 0 || page.Watermark == nil || page.Watermark.Height != strconv.FormatInt(lagging, 10) {
		t.Fatalf("lagging trade page: status=%d page=%+v", status, page)
	}
}
