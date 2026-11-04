package config_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/portal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/records"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/loadtest"
)

func TestRepairM_830_ChainID(t *testing.T) {
	for _, service := range []config.Service{config.Indexer, config.Gateway, config.Explorer, config.Portal, config.Replay, config.Loadtest} {
		t.Run(string(service), func(t *testing.T) {
			proofConfigurationEnv(t)
			accepted, err := service.Load()
			if err != nil || accepted == nil || accepted.ChainID != 125 {
				t.Fatalf("mainnet startup: %v", err)
			}
			for _, chain := range []uint64{1, 124, 126, 18446744073709551615} {
				t.Setenv(envChainID, fmt.Sprint(chain))
				cfg, err := service.Load()
				want := string(service) + " config: PERPS_CHAIN_ID must be 125 (the only supported network)"
				if cfg != nil || err == nil || err.Error() != want {
					t.Fatalf("chain %d startup = %v, %v; want refusal %q", chain, cfg, err, want)
				}
				direct := *accepted
				direct.ChainID = chain
				if err := direct.Validate(); err == nil || err.Error() != "PERPS_CHAIN_ID must be 125 (the only supported network)" {
					t.Fatalf("direct validation admitted chain %d: %v", chain, err)
				}
			}
			t.Setenv(envChainID, "125")
			recovered, err := service.Load()
			if err != nil || recovered == nil || recovered == accepted || recovered.ChainID != 125 || accepted.ChainID != 125 {
				t.Fatalf("fresh startup after refusal: %v", err)
			}
		})
	}
}

func TestRepairM_830_PortalHTTP(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(fmt.Sprintf("tls=%t", secure), func(t *testing.T) {
			requests := make(chan string, 4)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r.Method + " " + r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				var response any
				switch r.URL.Path {
				case "/api/markets":
					response = struct {
						Markets []records.MarketRecord `json:"markets"`
					}{[]records.MarketRecord{{MarketID: "1", Symbol: "BTC", SymbolFull: "Bitcoin", TradesCount: 7}}}
				case "/api/totals":
					response = struct {
						Totals records.Totals `json:"totals"`
					}{records.Totals{TotalTrades: 19}}
				default:
					t.Errorf("unexpected portal route %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("encode gateway record: %v", err)
				}
			})
			var gateway *httptest.Server
			if secure {
				gateway = httptest.NewTLSServer(handler)
			} else {
				gateway = httptest.NewServer(handler)
			}
			defer gateway.Close()
			for _, explorer := range []string{"http://explorer.example/tx/", "https://explorer.example/tx/"} {
				t.Run(explorer, func(t *testing.T) {
					setEnv(t, with(with(baseEnv(), envGatewayURL, gateway.URL), envExplorerTx, explorer))
					cfg, err := config.Portal.Load()
					if err != nil {
						t.Fatal(err)
					}
					if cfg.GatewayURL != gateway.URL || cfg.ExplorerTxBase != explorer {
						t.Fatal("configuration changed the consumer's endpoints")
					}
					client := gateway.Client()
					client.Timeout = 2 * time.Second
					pages, err := portal.NewPages(cfg.GatewayURL, cfg.ExplorerTxBase, client)
					if err != nil {
						t.Fatalf("Config-to-NewPages: %v", err)
					}
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					var rendered bytes.Buffer
					if err := pages.Markets(ctx, &rendered); err != nil {
						t.Fatalf("read gateway and render: %v", err)
					}
					for _, want := range []string{`id="markets"`, `href="/markets/btc"`, "Bitcoin", "19"} {
						if !strings.Contains(rendered.String(), want) {
							t.Errorf("rendered page missing %q", want)
						}
					}
					for _, want := range []string{"GET /api/markets", "GET /api/totals"} {
						select {
						case got := <-requests:
							if got != want {
								t.Errorf("request = %q, want %q", got, want)
							}
						default:
							t.Errorf("missing consumer request %q", want)
						}
					}
				})
			}
		})
	}
}

func TestRepairM_830_GatewaySchemes(t *testing.T) {
	for _, scheme := range []string{"http", "https", "ws", "wss", "ftp"} {
		t.Run(scheme, func(t *testing.T) {
			endpoint := scheme + "://gateway.example"
			setEnv(t, with(baseEnv(), envGatewayURL, endpoint))
			cfg, err := config.Portal.Load()
			if scheme == "http" || scheme == "https" {
				if err != nil || cfg == nil {
					t.Fatalf("HTTP portal startup: %v", err)
				}
				if pages, err := portal.NewPages(cfg.GatewayURL, cfg.ExplorerTxBase, &http.Client{}); err != nil || pages == nil {
					t.Fatalf("portal consumer refused accepted configuration: %v", err)
				}
			} else if cfg != nil || err == nil || err.Error() != "portal config: PERPS_GATEWAY_URL needs http or https for portal" {
				t.Fatalf("portal scheme %s = %v, %v", scheme, cfg, err)
			}
			cfg, err = config.Loadtest.Load()
			if scheme == "ftp" {
				if cfg != nil || err == nil || err.Error() != "loadtest config: PERPS_GATEWAY_URL needs http, https, ws or wss" {
					t.Fatalf("invalid loadtest scheme: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			wantScheme := "ws"
			if scheme == "https" || scheme == "wss" {
				wantScheme = "wss"
			}
			socket, err := loadtest.SocketURL(cfg.GatewayURL)
			if err != nil || socket != wantScheme+"://gateway.example/ws" {
				t.Fatalf("Config-to-SocketURL = %q, %v", socket, err)
			}
		})
	}
	t.Run("optional_explorer_requires_consumer_wiring", func(t *testing.T) {
		setEnv(t, with(with(baseEnv(), envGatewayURL, "https://gateway.example"), envExplorerTx, ""))
		cfg, err := config.Portal.Load()
		if cfg != nil || err == nil || err.Error() != "portal config: PERPS_EXPLORER_TX_BASE is required for portal" {
			t.Fatalf("unsupported optional explorer startup = %v, %v", cfg, err)
		}
		if pages, err := portal.NewPages("https://gateway.example", "", &http.Client{}); pages != nil || err == nil || err.Error() != "portal_page_url_needs_http_or_https" {
			t.Fatalf("consumer's current optional explorer contract = %v, %v", pages, err)
		}
		// The explorer service itself still supports an omitted link base.
		if cfg, err := config.Explorer.Load(); err != nil || cfg == nil || cfg.ExplorerTxBase != "" {
			t.Fatalf("explorer's optional base = %v, %v", cfg, err)
		}
	})
	t.Run("loadtest_socket_origin_discovery", func(t *testing.T) {
		gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/markets" {
				t.Errorf("loadtest requested %s", r.URL.Path)
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(struct {
				Markets []records.MarketRecord `json:"markets"`
			}{[]records.MarketRecord{{MarketID: "1", Symbol: "BTC"}}}); err != nil {
				t.Error(err)
			}
		}))
		defer gateway.Close()
		for _, scheme := range []string{"http", "ws"} {
			setEnv(t, with(baseEnv(), envGatewayURL, scheme+strings.TrimPrefix(gateway.URL, "http")))
			cfg, err := config.Loadtest.Load()
			if err != nil {
				t.Fatal(err)
			}
			symbols, err := loadtest.FeedSymbols(context.Background(), cfg.GatewayURL)
			if err != nil || !slices.Equal(symbols, []string{"BTC"}) {
				t.Fatalf("Config-to-FeedSymbols = %v, %v", symbols, err)
			}
		}
	})
}

func TestRepairM_830_EndpointRedaction(t *testing.T) {
	vars := baseEnv()
	vars[envRPCURLs] = "https://rpc-user:rpc-password@rpc.example/node?api_key=rpc-token&custom=rpc-token#rpc-token,https://rpc-user@rpc-backup.example?unknown=rpc-token"
	vars[envCometURLs] = "http://comet-user:comet-password@comet.example/rpc?token=comet-token"
	vars[envGatewayURL] = "https://gateway-user:gateway-password@gateway.example?access_token=gateway-token"
	vars[envExplorerTx] = "https://explorer-user:explorer-password@explorer.example/tx/?key=explorer-token"
	for _, service := range []config.Service{config.Indexer, config.Gateway, config.Explorer, config.Portal, config.Replay, config.Loadtest} {
		t.Run(string(service), func(t *testing.T) {
			setEnv(t, vars)
			cfg, err := service.Load()
			if err != nil {
				t.Fatal(err)
			}
			original := *cfg
			original.RPCURLs = slices.Clone(cfg.RPCURLs)
			original.CometURLs = slices.Clone(cfg.CometURLs)
			assertEndpointRedaction(t, cfg)
			raw, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			var diagnostic config.Config
			if err := json.Unmarshal(raw, &diagnostic); err != nil {
				t.Fatal(err)
			}
			if diagnostic.Service != service || diagnostic.ChainID != 125 || diagnostic.Port != cfg.Port {
				t.Fatal("JSON lost nonsecret configuration")
			}
			if diagnostic.DatabaseURL != "" || diagnostic.NATSURL != "" || diagnostic.AuthSecret != "" {
				t.Fatal("JSON exposed a private service field")
			}
			endpoints := append(slices.Clone(diagnostic.RPCURLs), diagnostic.CometURLs...)
			endpoints = append(endpoints, diagnostic.GatewayURL, diagnostic.ExplorerTxBase)
			for _, endpoint := range endpoints {
				if endpoint == "" {
					continue
				}
				parsed, err := url.Parse(endpoint)
				if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
					t.Errorf("unsafe diagnostic endpoint %q", endpoint)
				}
				if !strings.Contains(cfg.String(), endpoint) {
					t.Errorf("text and JSON disagree on endpoint %q", endpoint)
				}
			}
			if !reflect.DeepEqual(*cfg, original) {
				t.Fatal("diagnostics altered the transport's live configuration")
			}
		})
	}
	t.Run("malformed_endpoint_fails_closed", func(t *testing.T) {
		cfg := &config.Config{Service: config.Portal, ChainID: 125, GatewayURL: "https://gateway-user:gateway-password@%zz?token=gateway-token"}
		assertEndpointRedaction(t, cfg)
		if !strings.Contains(cfg.String(), "PERPS_GATEWAY_URL=redacted") {
			t.Fatal("malformed endpoint did not fail closed")
		}
	})
}
