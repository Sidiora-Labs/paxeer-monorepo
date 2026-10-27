package localnode

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	_ "github.com/lib/pq"

	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
)

func TestLocalPerpsGatewayServesMarkets(t *testing.T) {
	endpoint := os.Getenv("NEXT_PUBLIC_PERPS_GATEWAY_URL")
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Port() == "0" {
		t.Fatalf("NEXT_PUBLIC_PERPS_GATEWAY_URL must name the running loopback gateway: %q", endpoint)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := ethclient.DialContext(ctx, os.Getenv("PERPS_DATA_TEST_RPC"))
	if err != nil {
		t.Fatalf("dial disposable node: %v", err)
	}
	defer client.Close()
	chain, err := client.ChainID(ctx)
	if err != nil {
		t.Fatalf("read chain ID: %v", err)
	}
	if chain.Cmp(big.NewInt(125)) != 0 {
		t.Fatalf("chain ID = %s, want 125", chain)
	}
	contract, err := perpsabi.ABI()
	if err != nil {
		t.Fatalf("load perps ABI: %v", err)
	}
	address := common.HexToAddress("0x0000000000000000000000000000000000001015")
	call := func(method string, at *big.Int, args ...any) []any {
		t.Helper()
		input, err := contract.Pack(method, args...)
		if err != nil {
			t.Fatalf("pack %s: %v", method, err)
		}
		output, err := client.CallContract(ctx, ethereum.CallMsg{To: &address, Data: input}, at)
		if err != nil {
			t.Fatalf("call %s: %v", method, err)
		}
		values, err := contract.Unpack(method, output)
		if err != nil {
			t.Fatalf("decode %s: %v", method, err)
		}
		return values
	}
	count := call("totalMarkets", nil)
	if len(count) != 1 || count[0].(*big.Int).Cmp(big.NewInt(18)) != 0 {
		t.Fatalf("chain market count = %v, want 18", count)
	}
	body := bytes.NewBufferString(`{"query":"query LocalPerpsMarkets { markets { marketId name symbol maxLeverage enabled } }"}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/graphql", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("query gateway markets: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		t.Fatalf("read markets response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("markets status %d: %s", resp.StatusCode, raw)
	}
	var result struct {
		Errors []json.RawMessage `json:"errors"`
		Data   struct {
			Markets []struct {
				ID          int64  `json:"marketId"`
				Name        string `json:"name"`
				Symbol      string `json:"symbol"`
				MaxLeverage string `json:"maxLeverage"`
				Enabled     bool   `json:"enabled"`
			} `json:"markets"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode markets: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("GraphQL errors: %s", raw)
	}
	if len(result.Data.Markets) != 18 {
		t.Fatalf("gateway returned %d markets, want 18", len(result.Data.Markets))
	}
	databaseURL := os.Getenv("PERPS_DATA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("PERPS_DATA_TEST_DATABASE_URL is required")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open migrated database: %v", err)
	}
	defer db.Close()
	seen := make(map[int64]bool)
	for _, market := range result.Data.Markets {
		if market.ID < 0 || market.ID >= 18 || seen[market.ID] {
			t.Fatalf("invalid or duplicate market ID: %d", market.ID)
		}
		seen[market.ID] = true
		var source string
		var height int64
		var margin, openInterest string
		err := db.QueryRowContext(ctx, `SELECT source, refreshed_block, maintenance_margin_bps::text, max_open_interest::text FROM perps.markets WHERE market_id = $1`, market.ID).Scan(&source, &height, &margin, &openInterest)
		if err != nil {
			t.Fatalf("read refreshed market %d: %v", market.ID, err)
		}
		if source != "chain" || height <= 0 {
			t.Fatalf("market %d was not refreshed: source=%q height=%d", market.ID, source, height)
		}
		values := call("getMarket", big.NewInt(height), big.NewInt(market.ID))
		if len(values) != 6 {
			t.Fatalf("getMarket(%d) returned %d fields, want 6", market.ID, len(values))
		}
		if market.Name != values[0].(string) || market.Symbol != values[1].(string) || market.MaxLeverage != values[2].(*big.Int).String() || market.Enabled != values[5].(bool) || margin != values[3].(*big.Int).String() || openInterest != values[4].(*big.Int).String() {
			t.Fatalf("market %d differs from chain-125 config: gateway=%+v margin=%s openInterest=%s chain=%v", market.ID, market, margin, openInterest, values)
		}
	}
}
