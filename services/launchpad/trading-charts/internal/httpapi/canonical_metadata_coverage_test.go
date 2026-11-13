package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestCanonicalMetadataReadsDeployedTokenAtCreationBlock(t *testing.T) {
	ctx := context.Background()
	chain := newDevChainWithID(t, 125)
	supply := new(big.Int).SetUint64(1000000)
	tokenAddress := chain.deployToken(t, "Canonical Chart Token", "CCT", supply)
	rpc, err := ethclient.Dial(chain.url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rpc.Close)
	latest, err := rpc.BlockByNumber(ctx, nil)
	if err != nil || len(latest.Transactions()) != 1 {
		t.Fatalf("token deployment block: %+v, %v", latest, err)
	}
	receipt, err := rpc.TransactionReceipt(ctx, latest.Transactions()[0].Hash())
	if err != nil {
		t.Fatal(err)
	}
	reader := &chainread.Reader{RPC: rpc}
	block, err := reader.Block(ctx, receipt.BlockNumber.Int64())
	if err != nil {
		t.Fatal(err)
	}
	position := interfaces.ChainPosition{ChainId: "125", Height: interfaces.UInt256(receipt.BlockNumber.String()), BlockHash: block.Hash, TransactionHash: strings.ToLower(receipt.TxHash.Hex()), TransactionIndex: interfaces.UInt256(strconv.FormatUint(uint64(receipt.TransactionIndex), 10)), LogIndex: "0"}
	receiptWire, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	receiptDigest := sha256.Sum256(receiptWire)
	blockWire, err := json.Marshal(block)
	if err != nil {
		t.Fatal(err)
	}
	blockDigest := sha256.Sum256(blockWire)
	payload, err := json.Marshal(map[string]any{"token": strings.ToLower(tokenAddress.Hex()), "pool": devPool, "position": position, "provenance": map[string]string{"receiptSHA256": hex.EncodeToString(receiptDigest[:]), "headerSHA256": hex.EncodeToString(blockDigest[:])}})
	if err != nil {
		t.Fatal(err)
	}
	pool := historyPostgres(t)
	if _, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS indexer;
CREATE TABLE indexer.indexer_canonical_creations_v1(pool text PRIMARY KEY,payload jsonb NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations_v1 VALUES($1,$2)`, devPool, payload); err != nil {
		t.Fatal(err)
	}
	source := CanonicalPoolMetadataSource{Pool: pool, Chain: reader}
	resolved, err := source.Resolve(ctx, devPool)
	if err != nil || resolved.Name != "Canonical Chart Token" || resolved.Symbol != "CCT" || resolved.Decimals == nil || *resolved.Decimals != 6 || resolved.TotalSupplyRaw == nil || string(*resolved.TotalSupplyRaw) != supply.String() || resolved.SourcePosition == nil || *resolved.SourcePosition != position {
		t.Fatalf("canonical token metadata: %+v, %v", resolved, err)
	}
	snapshot, err := (RPCSupplySource{Chain: reader}).At(ctx, resolved.Address, position)
	if err != nil || snapshot.TotalSupplyRaw != supply.String() || snapshot.Decimals != 6 || snapshot.SourceBlock != receipt.BlockNumber.String() || snapshot.SourceHash != block.Hash || snapshot.Alignment != "canonical_block_end" {
		t.Fatalf("historical token supply: %+v, %v", snapshot, err)
	}
	price := interfaces.Price{Numerator: "2", Denominator: "1", BaseAsset: interfaces.AssetRef{Address: &resolved.Address, Decimals: resolved.Decimals}, Source: interfaces.PriceSource{Position: &position}}
	bar := interfaces.Candle{Start: time.Unix(block.Time, 0).UTC().Format("2006-01-02T15:04:05.000Z"), Open: price, High: price, Low: price, Close: price, VolumeUsdlRaw: "1000000"}
	valuation, err := valuationHistory(ctx, CanonicalCandlePage{Items: []interfaces.Candle{bar}, SnapshotID: "owned", Algorithm: "qualified-exact-v2", Complete: true}, &RPCSupplySource{Chain: reader}, modeTotalSupplyValuation)
	if err != nil {
		t.Fatal(err)
	}
	var valued struct {
		Status    string    `json:"s"`
		Available bool      `json:"valuationAvailable"`
		Close     []float64 `json:"c"`
	}
	if err := json.Unmarshal(valuation, &valued); err != nil || valued.Status != "ok" || !valued.Available || len(valued.Close) != 1 || valued.Close[0] != 2 {
		t.Fatalf("historical total supply valuation: %+v, %v", valued, err)
	}
	rec := httptest.NewRecorder()
	(SymbolsHandler{Canonical: &source, DisplayPrecision: 6}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v2/udf/symbols?symbol="+devPool, nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"full_name":"CCT/USDL"`) || rec.Header().Get("ETag") == "" {
		t.Fatalf("canonical symbol response: %d %s", rec.Code, rec.Body.String())
	}
	conditional := httptest.NewRequest(http.MethodGet, "/v2/udf/symbols?symbol="+devPool, nil)
	conditional.Header.Set("If-None-Match", rec.Header().Get("ETag"))
	unchanged := httptest.NewRecorder()
	(SymbolsHandler{Canonical: &source, DisplayPrecision: 6}).ServeHTTP(unchanged, conditional)
	if unchanged.Code != http.StatusNotModified || unchanged.Body.Len() != 0 {
		t.Fatalf("unchanged canonical metadata: %d %s", unchanged.Code, unchanged.Body.String())
	}
	invalidPrecision := httptest.NewRecorder()
	(SymbolsHandler{Canonical: &source, DisplayPrecision: 13}).ServeHTTP(invalidPrecision, httptest.NewRequest(http.MethodGet, "/v2/udf/symbols?symbol="+devPool, nil))
	if invalidPrecision.Code != http.StatusServiceUnavailable || !strings.Contains(invalidPrecision.Body.String(), "invalid vendor precision") {
		t.Fatalf("out of range display precision: %d %s", invalidPrecision.Code, invalidPrecision.Body.String())
	}
	if _, err := source.Resolve(ctx, "0x"+strings.Repeat("f", 40)); !errors.Is(err, ErrUnknownPool) {
		t.Fatalf("unknown canonical pool: %v", err)
	}
	wrong := position
	wrong.BlockHash = "0x" + strings.Repeat("f", 64)
	if _, err := (RPCSupplySource{Chain: reader}).At(ctx, resolved.Address, wrong); err == nil {
		t.Fatal("historical supply accepted another block identity")
	}
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
		want   string
	}{
		{"different pool", func(f map[string]any) { f["pool"] = "0x" + strings.Repeat("f", 40) }, "invalid canonical creation"},
		{"different chain", func(f map[string]any) { f["position"].(map[string]any)["chainId"] = "1" }, "invalid canonical creation"},
		{"unbound receipt digest", func(f map[string]any) { f["provenance"].(map[string]any)["receiptSHA256"] = strings.Repeat("z", 64) }, "invalid byte"},
		{"unbound header digest", func(f map[string]any) { f["provenance"].(map[string]any)["headerSHA256"] = strings.Repeat("z", 64) }, "invalid byte"},
		{"unparseable height", func(f map[string]any) { f["position"].(map[string]any)["height"] = "invalid" }, "invalid syntax"},
		{"different block", func(f map[string]any) { f["position"].(map[string]any)["blockHash"] = "0x" + strings.Repeat("f", 64) }, "pinned token metadata unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var fact map[string]any
			if err := json.Unmarshal(payload, &fact); err != nil {
				t.Fatal(err)
			}
			tc.change(fact)
			changed, err := json.Marshal(fact)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, `UPDATE indexer.indexer_canonical_creations_v1 SET payload=$2 WHERE pool=$1`, devPool, changed); err != nil {
				t.Fatal(err)
			}
			if token, err := source.Resolve(ctx, devPool); err == nil || !strings.Contains(err.Error(), tc.want) || token.Address != "" {
				t.Fatalf("canonical metadata binding: %+v, %v", token, err)
			}
		})
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_canonical_creations_v1 SET payload=$2 WHERE pool=$1`, devPool, payload); err != nil {
		t.Fatal(err)
	}
	if token, err := (CanonicalPoolMetadataSource{Pool: nil, Chain: reader}).Resolve(ctx, devPool); token.Address != "" || err == nil || !strings.Contains(err.Error(), "authority unavailable") {
		t.Fatalf("metadata without owned source: %+v, %v", token, err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE indexer.indexer_canonical_creations_v1 RENAME TO indexer_creations_unavailable`); err != nil {
		t.Fatal(err)
	}
	if token, err := source.Resolve(ctx, devPool); token.Address != "" || err == nil {
		t.Fatalf("metadata without committed creation catalog: %+v, %v", token, err)
	}
}
