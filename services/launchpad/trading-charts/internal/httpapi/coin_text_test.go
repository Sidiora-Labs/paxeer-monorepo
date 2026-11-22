package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func coinTextCreation(t *testing.T, pool *pgxpool.Pool, reader *chainread.Reader, address common.Address, receipt *types.Receipt) {
	t.Helper()
	block, err := reader.Block(context.Background(), receipt.BlockNumber.Int64())
	if err != nil {
		t.Fatal(err)
	}
	position := interfaces.ChainPosition{ChainId: "125", Height: interfaces.UInt256(receipt.BlockNumber.String()), BlockHash: block.Hash, TransactionHash: strings.ToLower(receipt.TxHash.Hex()), TransactionIndex: interfaces.UInt256(strconv.FormatUint(uint64(receipt.TransactionIndex), 10)), LogIndex: "0"}
	receiptWire, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	blockWire, err := json.Marshal(block)
	if err != nil {
		t.Fatal(err)
	}
	receiptDigest, headerDigest := sha256.Sum256(receiptWire), sha256.Sum256(blockWire)
	fact, err := json.Marshal(map[string]any{"pool": devPool, "token": strings.ToLower(address.Hex()), "position": position, "provenance": map[string]string{"receiptSHA256": hex.EncodeToString(receiptDigest[:]), "headerSHA256": hex.EncodeToString(headerDigest[:])}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO indexer.indexer_canonical_creations_v1 VALUES($1,$2) ON CONFLICT(pool) DO UPDATE SET payload=EXCLUDED.payload`, devPool, fact); err != nil {
		t.Fatal(err)
	}
}

func TestCoinTextSymbolsFromCanonicalAndDevelopment(t *testing.T) {
	ctx := context.Background()
	f := newDevFixture(t, false)
	chain := newDevChainWithID(t, 125)
	client, err := ethclient.Dial(chain.url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	reader := &chainread.Reader{RPC: client}
	if _, err := f.pool.Exec(ctx, `CREATE TABLE indexer.indexer_canonical_creations_v1(pool text PRIMARY KEY,payload jsonb NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	canonical := CanonicalPoolMetadataSource{Pool: f.pool, Chain: reader}
	for _, tc := range []struct {
		name, coinName, symbol, wantName, wantSymbol string
	}{
		{"empty symbol", "Coin", "", "Coin", ""},
		{"angles and controls", "<Coin>\x00\n", "<A\x00B\n>\x7f\u0085", "Coin", "AB"},
		{"161 code point name", strings.Repeat("界", 161), "COIN", strings.Repeat("界", 100), "COIN"},
		{"33 code point symbol", "Coin", strings.Repeat("🪙", 33), "Coin", strings.Repeat("🪙", 16)},
		{"empty name", "", "COIN", "", "COIN"},
		{"only removed characters", "<>\x00\t\u202e", "\u2066<\x01>\u2069", "", ""},
		{"direction controls", "A\u061c\u200e\u200f\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069B", "A\u202eB", "AB", "AB"},
		{"filter before truncation", strings.Repeat("<\n界", 101), strings.Repeat("\u202e🪙>", 17), strings.Repeat("界", 100), strings.Repeat("🪙", 16)},
		{"text beyond one kilobyte", strings.Repeat("🪙", 300), "COIN", strings.Repeat("🪙", 100), "COIN"},
		{"spaces", " ", " ", " ", " "},
		{"invalid UTF-8 bytes", "A\xffB", "A\xffB", "A\ufffdB", "A\ufffdB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			address := chain.deployToken(t, tc.coinName, tc.symbol, big.NewInt(1000000))
			latest, err := client.BlockByNumber(ctx, nil)
			if err != nil || len(latest.Transactions()) != 1 {
				t.Fatalf("token deployment block: %+v %v", latest, err)
			}
			receipt, err := client.TransactionReceipt(ctx, latest.Transactions()[0].Hash())
			if err != nil {
				t.Fatal(err)
			}
			coinTextCreation(t, f.pool, reader, address, receipt)
			poolID := common.HexToHash("0x" + strings.Repeat("d1", 32))
			market := chain.emitMarketCreated(t, poolID)
			if _, err := f.pool.Exec(ctx, `INSERT INTO indexer.pools(pool_address,token_address,creator,optical,pool_id,nft_id,created_at,created_block,tx_hash) VALUES($1,$2,$3,$3,$4,1,1,$5,$6) ON CONFLICT(pool_address) DO UPDATE SET token_address=EXCLUDED.token_address,created_block=EXCLUDED.created_block,tx_hash=EXCLUDED.tx_hash`, devPool, strings.ToLower(address.Hex()), strings.ToLower(crypto.PubkeyToAddress(chain.key.PublicKey).Hex()), poolID.Hex(), market.BlockNumber.Int64(), market.TxHash.Hex()); err != nil {
				t.Fatal(err)
			}
			development := &DevelopmentPoolMetadataSource{Pool: f.pool, Chain: client.Client()}
			wantName, wantSymbol := tc.wantName, tc.wantSymbol
			short := "0x" + strings.ToLower(address.Hex()[2:6]) + "..." + strings.ToLower(address.Hex()[38:])
			if wantName == "" {
				wantName = short
			}
			if wantSymbol == "" {
				wantSymbol = short
			}
			for _, source := range []struct {
				name     string
				resolver TokenMetadataResolver
				handler  SymbolsHandler
			}{
				{"canonical", canonical, SymbolsHandler{Canonical: &canonical}},
				{"development", development, SymbolsHandler{Development: development}},
			} {
				t.Run(source.name, func(t *testing.T) {
					token, err := source.resolver.Resolve(ctx, devPool)
					if err != nil || token.Name != wantName || token.Symbol != wantSymbol {
						t.Fatalf("resolved text: %+v %v; want %q %q", token, err, wantName, wantSymbol)
					}
					raw, err := json.Marshal(token)
					if err != nil || interfaces.ValidateJSON("Token", raw) != nil {
						t.Fatalf("Token contract: %s %v", raw, err)
					}
					upstream := token
					upstream.Name, upstream.Symbol = tc.coinName, tc.symbol
					wire, err := json.Marshal(map[string]any{"data": map[string]any{"token": upstream, "pool": devPool}, "revision": "9007199254740993"})
					if err != nil {
						t.Fatal(err)
					}
					wire, err = normalizePoolMetadataJSON(wire)
					if err != nil {
						t.Fatal(err)
					}
					var metadata struct {
						Data struct {
							Token interfaces.Token `json:"token"`
							Pool  string           `json:"pool"`
						} `json:"data"`
						Revision string `json:"revision"`
					}
					if err := json.Unmarshal(wire, &metadata); err != nil || metadata.Data.Token.Name != wantName || metadata.Data.Token.Symbol != wantSymbol || metadata.Data.Token.MetadataRevision != token.MetadataRevision || metadata.Data.Token.DecimalsQuality != qualityVerified || metadata.Data.Pool != devPool || metadata.Revision != "9007199254740993" {
						t.Fatalf("upstream display text: %s %v", wire, err)
					}
					router := chi.NewRouter()
					RegisterProductionUDF(router, source.handler, ProductionHistoryHandler{})
					for _, route := range []string{"/symbols", "/v2/udf/symbols"} {
						rec, body := doGET(t, router, route+"?symbol="+devPool)
						if rec.Code != http.StatusOK || body["full_name"] != wantSymbol+"/USDL" || body["description"] != wantName || body["ticker"] != devPool || body["name"] != devPool || body["tokenAddress"] != strings.ToLower(address.Hex()) || body["baseDecimals"] != float64(6) || body["metadataRevision"] != string(token.MetadataRevision) {
							t.Fatalf("symbol text: %d %s", rec.Code, rec.Body.String())
						}
						if !utf8.ValidString(body["description"].(string)) || utf8.RuneCountInString(body["description"].(string)) > 100 || utf8.RuneCountInString(wantSymbol) > 16 || rec.Header().Get("ETag") == "" {
							t.Fatalf("display bounds and cache validator: %s", rec.Body.String())
						}
						request := httptest.NewRequest(http.MethodGet, route+"?symbol="+devPool, nil)
						request.Header.Set("If-None-Match", rec.Header().Get("ETag"))
						cached := httptest.NewRecorder()
						router.ServeHTTP(cached, request)
						if cached.Code != http.StatusNotModified || cached.Body.Len() != 0 {
							t.Fatalf("conditional symbol text: %d %s", cached.Code, cached.Body.String())
						}
					}
				})
			}
		})
	}
}

func TestCoinTextControlCharacters(t *testing.T) {
	var controls strings.Builder
	for r := rune(0); r < 32; r++ {
		controls.WriteRune(r)
	}
	for r := rune(127); r <= 159; r++ {
		controls.WriteRune(r)
	}
	controls.WriteString("\u061c\u200e\u200f\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069<>")
	address := "0x1234567890abcdef1234567890abcdef1234abcd"
	for _, max := range []int{100, 16} {
		if got := boundedMetadataText(controls.String(), max, address); got != "0x1234...abcd" {
			t.Fatalf("empty display text: %q", got)
		}
		if got := boundedMetadataText("A"+controls.String()+"B", max, address); got != "AB" {
			t.Fatalf("filtered display text: %q", got)
		}
	}
	for _, raw := range []string{
		`{"data":{"token":{"address":"0x1234567890abcdef1234567890abcdef1234abcd","name":null,"symbol":42}}}`,
		`{"data":{"token":{"address":"0x1234567890abcdef1234567890abcdef1234abcd"}}}`,
	} {
		wire, err := normalizePoolMetadataJSON([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		var before, after any
		if err := json.Unmarshal([]byte(raw), &before); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(wire, &after); err != nil {
			t.Fatal(err)
		}
		original, err := json.Marshal(before)
		if err != nil {
			t.Fatal(err)
		}
		preserved, err := json.Marshal(after)
		if err != nil || string(original) != string(preserved) {
			t.Fatalf("metadata field types: %s %v", wire, err)
		}
	}
}

func TestCoinTextUnavailableDecimals(t *testing.T) {
	ctx := context.Background()
	pool := historyPostgres(t)
	chain := newDevChainWithID(t, 125)
	client, err := ethclient.Dial(chain.url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	reader := &chainread.Reader{RPC: client}
	if _, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS indexer; CREATE TABLE indexer.indexer_canonical_creations_v1(pool text PRIMARY KEY,payload jsonb NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	receipt := chain.emitMarketCreated(t, common.HexToHash("0x"+strings.Repeat("d1", 32)))
	coinTextCreation(t, pool, reader, receipt.ContractAddress, receipt)
	source := CanonicalPoolMetadataSource{Pool: pool, Chain: reader}
	if token, err := source.Resolve(ctx, devPool); err == nil || token.Decimals != nil || token.DecimalsQuality == qualityVerified {
		t.Fatalf("unavailable decimals: %+v %v", token, err)
	}
	handler := SymbolsHandler{Canonical: &source}
	rec, body := doGET(t, handler, "/v2/udf/symbols?symbol="+devPool)
	if rec.Code != http.StatusServiceUnavailable || body["code"] != "DEPENDENCY_UNAVAILABLE" || body["retryable"] != true || rec.Header().Get("ETag") != "" || body["baseDecimals"] != nil || body["pricescale"] != nil {
		t.Fatalf("unavailable decimal response: %d %s", rec.Code, rec.Body.String())
	}
	unknown, body := doGET(t, handler, "/v2/udf/symbols?symbol=0x"+strings.Repeat("f", 40))
	if unknown.Code != http.StatusNotFound || body["code"] != "NOT_FOUND" || body["retryable"] != false {
		t.Fatalf("unknown pool response: %d %s", unknown.Code, unknown.Body.String())
	}
}
