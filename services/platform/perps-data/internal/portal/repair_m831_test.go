package portal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
)

func TestRepairM_831_ConfigToPages(t *testing.T) {
	const address = "0x00000000000000000000000000000000000000a1"
	hashes := make([]string, 7)
	for i := range hashes {
		hashes[i] = fmt.Sprintf("0x%064x", i+1)
	}
	for _, tc := range []struct {
		name, base string
		unset      bool
	}{
		{name: "unset", unset: true},
		{name: "empty"},
		{name: "http", base: "http://explorer.invalid/tx"},
		{name: "https", base: "https://explorer.invalid/tx/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			type operation struct {
				document  string
				variables map[string]any
				data      string
			}
			operations := map[string]operation{
				"GetPositions": {getPositions, map[string]any{"userAddress": address},
					fmt.Sprintf(`{"positions":[{"positionId":"1","isLong":true,"sizeUsd":"1000000000000000000","leverage":"1000000000000000000","entryPrice":"1000000000000000000","collateralAmount":"1000000","collateralUsd":"1000000000000000000","openedAt":"2026-01-01T00:00:00Z","openTxHash":%q,"market":{"symbol":"BTC"}},{"positionId":"2","isLong":false,"sizeUsd":"1000000000000000000","leverage":"1000000000000000000","entryPrice":"1000000000000000000","collateralAmount":"1000000","collateralUsd":"1000000000000000000","openedAt":"2026-01-01T00:00:00Z","openTxHash":null,"market":{"symbol":"BTC"}}]}`, hashes[0])},
				"GetAllOrdersV2": {getAllOrdersV2, map[string]any{"userAddress": address, "status": "active", "limit": float64(100)},
					fmt.Sprintf(`{"orders":[{"orderId":"3","marketId":0,"orderTypeName":"Limit","isLong":true,"triggerPrice":"1000000000000000000","sizeUsd":"1000000000000000000","status":"active","placedAt":"2026-01-01T00:00:00Z","placedTxHash":%q},{"orderId":"4","marketId":0,"orderTypeName":"Limit","isLong":false,"triggerPrice":"1000000000000000000","sizeUsd":"1000000000000000000","status":"active","placedAt":"2026-01-01T00:00:00Z","placedTxHash":null}]}`, hashes[1])},
				"GetTradeSettlements": {getTradeSettlements, map[string]any{"userAddress": address, "limit": float64(100)},
					fmt.Sprintf(`{"tradeSettlements":[{"id":1,"positionId":"1","marketId":0,"tradeTypeName":"Open","sizeUsd":"1000000000000000000","executionPrice":"1000000000000000000","grossPnl":"0","totalFeesUsd":"0","fundingPaidUsd":"0","blockNumber":1,"txHash":%q,"blockTimestamp":"2026-01-01T00:00:00Z"}]}`, hashes[2])},
				"GetFundingPayments": {getFundingPayments, map[string]any{"userAddress": address, "limit": float64(100)},
					fmt.Sprintf(`{"fundingPayments":[{"id":1,"positionId":"1","marketId":0,"fundingPaymentUsd":"0","newCollateralUsd":"1000000000000000000","blockNumber":1,"txHash":%q,"blockTimestamp":"2026-01-01T00:00:00Z"}]}`, hashes[3])},
				"GetLiquidations": {getLiquidations, map[string]any{"userAddress": address, "limit": float64(100)},
					fmt.Sprintf(`{"liquidations":[{"id":1,"positionId":"5","marketId":0,"price":"1000000000000000000","penalty":"0","keeper":"keeper","txHash":%q,"blockTimestamp":"2026-01-01T00:00:00Z"}]}`, hashes[4])},
				"GetVaultEvents": {getVaultEvents, map[string]any{"userAddress": address, "limit": float64(100)},
					fmt.Sprintf(`{"vaultEvents":[{"id":1,"eventType":"CollateralDeposited","tokenAddress":"USDL","amount":"1000000","blockNumber":1,"txHash":%q,"blockTimestamp":"2026-01-01T00:00:00Z"}]}`, hashes[5])},
				"PortalFundingRates": {getFundingRates, map[string]any{"marketId": float64(0), "limit": float64(100)},
					fmt.Sprintf(`{"fundingRates":[{"id":1,"ratePerSecond":"0","rate24h":"0","blockNumber":1,"txHash":%q,"blockTimestamp":"2026-01-01T00:00:00Z"}]}`, hashes[6])},
				"GetMarketSnapshots": {getMarketSnapshots, map[string]any{"marketId": float64(0), "limit": float64(168)}, `{"marketSnapshots":[]}`},
			}
			seen := make(chan string, len(operations)+4)
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet {
					seen <- r.URL.RequestURI()
					switch r.URL.RequestURI() {
					case "/api/markets":
						io.WriteString(w, `{"markets":[{"market_id":"0","symbol":"BTC","symbol_full":"Bitcoin"}]}`)
					case "/api/totals":
						io.WriteString(w, `{"totals":{"total_trades":1}}`)
					case "/api/markets/BTC":
						io.WriteString(w, `{"market":{"market_id":"0","symbol":"BTC","symbol_full":"Bitcoin"}}`)
					case "/api/btc/perp_trades?limit=100":
						io.WriteString(w, `{"trades":[]}`)
					default:
						t.Errorf("unexpected gateway request: %s", r.URL)
						http.Error(w, "unexpected route", http.StatusNotFound)
					}
					return
				}
				if r.Method != http.MethodPost || r.URL.Path != "/graphql" || r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("unexpected GraphQL transport: %s %s", r.Method, r.URL)
					http.Error(w, "unexpected transport", http.StatusBadRequest)
					return
				}
				var req struct {
					Query         string         `json:"query"`
					OperationName string         `json:"operationName"`
					Variables     map[string]any `json:"variables"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("decode gateway request: %v", err)
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
				want, ok := operations[req.OperationName]
				if !ok || req.Query != want.document || !reflect.DeepEqual(req.Variables, want.variables) {
					t.Errorf("unexpected operation: %+v", req)
					http.Error(w, "unexpected operation", http.StatusBadRequest)
					return
				}
				seen <- req.OperationName
				fmt.Fprintf(w, `{"data":%s}`, want.data)
			}))
			defer gateway.Close()
			t.Setenv("PERPS_CHAIN_ID", "125")
			t.Setenv("PORT", "8080")
			t.Setenv("PERPS_GATEWAY_URL", gateway.URL)
			t.Setenv("PERPS_EXPLORER_TX_BASE", tc.base)
			if tc.unset {
				if err := os.Unsetenv("PERPS_EXPLORER_TX_BASE"); err != nil {
					t.Fatal(err)
				}
			}
			cfg, err := config.Load(config.Portal)
			if err != nil {
				t.Fatalf("load portal configuration: %v", err)
			}
			if cfg.ExplorerTxBase != tc.base {
				t.Fatalf("explorer base = %q, want %q", cfg.ExplorerTxBase, tc.base)
			}
			pages, err := NewPages(cfg.GatewayURL, cfg.ExplorerTxBase, gateway.Client())
			if err != nil {
				t.Fatalf("construct pages from loaded configuration: %v", err)
			}
			var account, markets, market bytes.Buffer
			if err := pages.Account(t.Context(), &account, address); err != nil {
				t.Fatalf("render account: %v", err)
			}
			if err := pages.Markets(t.Context(), &markets); err != nil {
				t.Fatalf("render markets: %v", err)
			}
			if !strings.Contains(markets.String(), `<a href="/markets/btc">BTC</a>`) {
				t.Fatalf("markets missing BTC: %s", markets.String())
			}
			if err := pages.Market(t.Context(), &market, "btc"); err != nil {
				t.Fatalf("render market: %v", err)
			}
			for i, hash := range hashes {
				body := account.String()
				if i == 6 {
					body = market.String()
				}
				want := "<td>" + hash + "</td>"
				if tc.base != "" {
					want = `<td><a href="` + strings.TrimRight(tc.base, "/") + "/" + hash + `">` + hash + `</a></td>`
				}
				if strings.Count(body, want) != 1 {
					t.Errorf("transaction cell %d missing or duplicated: want %s in %s", i, want, body)
				}
				if tc.base == "" && (strings.Contains(body, `href="`+hash) || strings.Contains(body, `href="/`+hash)) {
					t.Errorf("unconfigured explorer produced a transaction link: %s", body)
				}
			}
			if strings.Count(account.String(), "<td>-</td>") != 2 || strings.Contains(account.String(), `href=""`) {
				t.Errorf("missing optional transaction hashes must remain plain dashes: %s", account.String())
			}
			wantRequests := []string{"GetPositions", "GetAllOrdersV2", "GetTradeSettlements", "GetFundingPayments", "GetLiquidations", "GetVaultEvents", "/api/markets", "/api/totals", "/api/markets/BTC", "/api/btc/perp_trades?limit=100", "PortalFundingRates", "GetMarketSnapshots"}
			for _, want := range wantRequests {
				select {
				case got := <-seen:
					if got != want {
						t.Errorf("gateway request = %q, want %q", got, want)
					}
				default:
					t.Fatalf("missing gateway request %q", want)
				}
			}
			if len(seen) != 0 {
				t.Errorf("unexpected additional gateway requests: %d", len(seen))
			}
		})
	}
}

func TestRepairM_831_RejectInvalidURLs(t *testing.T) {
	client := &http.Client{}
	for _, raw := range []string{" ", "/tx/", "//explorer.invalid/tx/", "https:///tx/", "ws://explorer.invalid/tx/", "wss://explorer.invalid/tx/", "ftp://explorer.invalid/tx/", "javascript:alert(1)", "https://%zz/tx/"} {
		t.Run(url.PathEscape(raw), func(t *testing.T) {
			if pages, err := NewPages("http://gateway.invalid", raw, client); err == nil || pages != nil {
				t.Fatalf("invalid nonempty explorer base %q accepted: pages=%v err=%v", raw, pages, err)
			}
			if pages, err := NewPages(raw, "", client); err == nil || pages != nil {
				t.Fatalf("invalid gateway %q accepted with no explorer: pages=%v err=%v", raw, pages, err)
			}
		})
	}
	if pages, err := NewPages("", "", client); err == nil || pages != nil {
		t.Fatalf("empty gateway accepted: pages=%v err=%v", pages, err)
	}
	if pages, err := NewPages("http://gateway.invalid", "", nil); err == nil || pages != nil {
		t.Fatalf("nil client accepted: pages=%v err=%v", pages, err)
	}
}
