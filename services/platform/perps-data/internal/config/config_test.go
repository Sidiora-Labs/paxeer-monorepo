package config_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/portal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/ratelimit"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/loadtest"
)

const (
	envChainID     = "PERPS_CHAIN_ID"
	envRPCURLs     = "PERPS_RPC_URLS"
	envCometURLs   = "PERPS_COMET_URLS"
	envDatabaseURL = "DATABASE_URL"
	envNATSURL     = "NATS_URL"
	envPort        = "PORT"
	envAuthSecret  = "PERPS_AUTH_SECRET"
	envExplorerTx  = "PERPS_EXPLORER_TX_BASE"
	envGatewayURL  = "PERPS_GATEWAY_URL"
	envStartHeight = "PERPS_START_HEIGHT"

	testDatabaseURL = "postgres://perps:p4ss@db/perps"
	testNATSURL     = "nats://indexer:n4ts@nats:4222"
	testSecret32    = "0123456789abcdef0123456789abcdef"
)

var allVars = []string{
	envChainID, envRPCURLs, envCometURLs, envDatabaseURL, envNATSURL, envPort,
	envAuthSecret, envExplorerTx, envGatewayURL, envStartHeight,
	ratelimit.EnvTrustedProxies,
}

func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for _, k := range allVars {
		t.Setenv(k, vars[k])
	}
}

func baseEnv() map[string]string {
	return map[string]string{
		envChainID:     "125",
		envRPCURLs:     "https://rpc-a.example, http://rpc-b.example",
		envCometURLs:   "https://comet.example",
		envDatabaseURL: testDatabaseURL,
		envNATSURL:     testNATSURL,
		envPort:        "8080",
		envAuthSecret:  testSecret32,
		envStartHeight: "1",
		envExplorerTx:  "https://explorer.example/tx/",
	}
}

func with(base map[string]string, k, v string) map[string]string {
	out := make(map[string]string, len(base)+1)
	for bk, bv := range base {
		out[bk] = bv
	}
	out[k] = v
	return out
}

func refuse(t *testing.T, service config.Service, want string) {
	t.Helper()
	cfg, err := config.Load(service)
	if err == nil {
		t.Fatalf("config.Load(%s) = %v, want error %q", service, cfg, want)
	}
	if err.Error() != want {
		t.Fatalf("config.Load(%s) error = %q, want %q", service, err.Error(), want)
	}
	if cfg != nil {
		t.Fatalf("config.Load(%s) returned a Config with error %q", service, err)
	}
}

func TestConfig(t *testing.T) {
	t.Run("loads_all", func(t *testing.T) {
		setEnv(t, with(with(baseEnv(), envStartHeight, "42"), envExplorerTx, "https://explorer.example/tx/"))
		cfg, err := config.Load(config.Indexer)
		if err != nil {
			t.Fatalf("config.Load(indexer): %v", err)
		}
		if cfg.ChainID != 125 || cfg.Port != 8080 || cfg.StartHeight != 42 {
			t.Fatalf("ChainID=%d Port=%d StartHeight=%d, want 125 8080 42", cfg.ChainID, cfg.Port, cfg.StartHeight)
		}
		if strings.Join(cfg.RPCURLs, " ") != "https://rpc-a.example http://rpc-b.example" {
			t.Fatalf("RPCURLs = %q", cfg.RPCURLs)
		}
		if len(cfg.CometURLs) != 1 || cfg.CometURLs[0] != "https://comet.example" {
			t.Fatalf("CometURLs = %q", cfg.CometURLs)
		}
		if cfg.DatabaseURL != testDatabaseURL || cfg.NATSURL != testNATSURL {
			t.Fatalf("DatabaseURL or NATSURL not loaded")
		}
		if cfg.AuthSecret != "" || cfg.ExplorerTxBase != "" || cfg.GatewayURL != "" {
			t.Fatalf("indexer read variables it does not use")
		}

		setEnv(t, map[string]string{
			envChainID:     "125",
			envRPCURLs:     "https://rpc.example",
			envDatabaseURL: testDatabaseURL,
			envNATSURL:     testNATSURL,
			envPort:        "8080",
			envAuthSecret:  testSecret32,
		})
		gw, err := config.Load(config.Gateway)
		if err != nil {
			t.Fatalf("config.Load(gateway): %v", err)
		}
		if gw.ChainID != 125 || gw.Port != 8080 || gw.AuthSecret != testSecret32 || gw.CometURLs != nil {
			t.Fatalf("gateway Config = %v", gw)
		}

		setEnv(t, map[string]string{
			envChainID:    "125",
			envPort:       "3000",
			envGatewayURL: "https://gateway.example",
			envExplorerTx: "https://explorer.example/tx/",
		})
		portalConfig, err := config.Load(config.Portal)
		if err != nil {
			t.Fatalf("config.Load(portal): %v", err)
		}
		if portalConfig.GatewayURL != "https://gateway.example" || portalConfig.ExplorerTxBase != "https://explorer.example/tx/" || portalConfig.Port != 3000 {
			t.Fatalf("portal Config = %v", portalConfig)
		}
		if pages, err := portal.NewPages(portalConfig.GatewayURL, portalConfig.ExplorerTxBase, &http.Client{}); err != nil || pages == nil {
			t.Fatalf("accepted portal config failed construction: %v", err)
		}

		setEnv(t, map[string]string{
			envChainID:     "125",
			envRPCURLs:     "https://rpc.example",
			envCometURLs:   "https://comet.example",
			envDatabaseURL: testDatabaseURL,
		})
		replay, err := config.Load(config.Replay)
		if err != nil {
			t.Fatalf("config.Load(replay): %v", err)
		}
		if replay.Port != 0 || replay.NATSURL != "" {
			t.Fatalf("replay Config = %v", replay)
		}
	})

	t.Run("refuses_missing_chain", func(t *testing.T) {
		setEnv(t, with(baseEnv(), envChainID, ""))
		refuse(t, config.Indexer, "PERPS_CHAIN_ID is required")
		setEnv(t, with(baseEnv(), envChainID, "paxeer"))
		refuse(t, config.Indexer, "PERPS_CHAIN_ID must be numeric")
		setEnv(t, with(baseEnv(), envChainID, "0"))
		refuse(t, config.Gateway, "PERPS_CHAIN_ID is required")
	})

	t.Run("gateway_schemes", func(t *testing.T) {
		for _, scheme := range []string{"http", "https", "ws", "wss"} {
			setEnv(t, with(baseEnv(), envGatewayURL, scheme+"://gateway.example"))
			if scheme == "ws" || scheme == "wss" {
				refuse(t, config.Portal, "PERPS_GATEWAY_URL needs http or https for portal")
			} else {
				cfg, err := config.Load(config.Portal)
				if err != nil {
					t.Fatal(err)
				}
				if pages, err := portal.NewPages(cfg.GatewayURL, cfg.ExplorerTxBase, &http.Client{}); err != nil || pages == nil {
					t.Fatalf("portal construction: %v", err)
				}
			}
			cfg, err := config.Load(config.Loadtest)
			if err != nil {
				t.Fatal(err)
			}
			wantScheme := "ws"
			if scheme == "https" || scheme == "wss" {
				wantScheme = "wss"
			}
			if socket, err := loadtest.SocketURL(cfg.GatewayURL); err != nil || socket != wantScheme+"://gateway.example/ws" {
				t.Fatalf("loadtest socket = %q, %v", socket, err)
			}
		}
	})

	t.Run("refuses_bad_rpc", func(t *testing.T) {
		setEnv(t, with(baseEnv(), envRPCURLs, "ftp://node"))
		refuse(t, config.Indexer, "PERPS_RPC_URLS needs http or https")
		setEnv(t, with(baseEnv(), envRPCURLs, "https://rpc.example,ws://node"))
		refuse(t, config.Gateway, "PERPS_RPC_URLS needs http or https")
		setEnv(t, with(baseEnv(), envRPCURLs, " , "))
		refuse(t, config.Replay, "PERPS_RPC_URLS needs http or https")
		setEnv(t, with(baseEnv(), envRPCURLs, "https://"))
		refuse(t, config.Indexer, "PERPS_RPC_URLS needs http or https")
	})

	t.Run("refuses_bad_comet", func(t *testing.T) {
		setEnv(t, with(baseEnv(), envCometURLs, "ws://node"))
		refuse(t, config.Indexer, "PERPS_COMET_URLS needs http or https")
		setEnv(t, with(baseEnv(), envCometURLs, ""))
		refuse(t, config.Replay, "PERPS_COMET_URLS needs http or https")
		setEnv(t, with(baseEnv(), envCometURLs, "file:///var/comet"))
		refuse(t, config.Indexer, "PERPS_COMET_URLS needs http or https")
	})

	t.Run("refuses_bad_port", func(t *testing.T) {
		setEnv(t, with(baseEnv(), envPort, "eighty"))
		refuse(t, config.Indexer, "PORT must be numeric")
		setEnv(t, with(baseEnv(), envPort, "-1"))
		refuse(t, config.Gateway, "PORT must be numeric")
		setEnv(t, with(baseEnv(), envPort, "65536"))
		refuse(t, config.Gateway, "PORT must be at most 65535")
		setEnv(t, with(baseEnv(), envPort, ""))
		refuse(t, config.Indexer, "PORT is required")
		setEnv(t, with(baseEnv(), envStartHeight, "-1"))
		refuse(t, config.Indexer, "PERPS_START_HEIGHT must be at least 1")
		setEnv(t, with(baseEnv(), envStartHeight, "tip"))
		refuse(t, config.Indexer, "PERPS_START_HEIGHT must be numeric")
	})

	t.Run("start_height_required", func(t *testing.T) {
		setEnv(t, with(baseEnv(), envStartHeight, ""))
		refuse(t, config.Indexer, "PERPS_START_HEIGHT must be set to the first indexed height (the v6.12.0 activation height on mainnet)")
		setEnv(t, with(baseEnv(), envStartHeight, "0"))
		refuse(t, config.Indexer, "PERPS_START_HEIGHT must be at least 1")
		setEnv(t, with(baseEnv(), envStartHeight, "42"))
		cfg, err := config.Load(config.Indexer)
		if err != nil || cfg.StartHeight != 42 {
			t.Fatalf("config.Load(indexer) with PERPS_START_HEIGHT=42 = %v, %v", cfg, err)
		}
		setEnv(t, with(baseEnv(), envStartHeight, ""))
		if _, err := config.Load(config.Replay); err != nil {
			t.Fatalf("replay needs no PERPS_START_HEIGHT: %v", err)
		}
	})

	t.Run("auth_secret_rule", func(t *testing.T) {
		setEnv(t, with(baseEnv(), envAuthSecret, testSecret32[:31]))
		refuse(t, config.Gateway, "PERPS_AUTH_SECRET must be at least 32 bytes")
		setEnv(t, with(baseEnv(), envAuthSecret, ""))
		refuse(t, config.Gateway, "PERPS_AUTH_SECRET must be at least 32 bytes")
		if _, err := config.Load(config.Indexer); err != nil {
			t.Fatalf("indexer needs no PERPS_AUTH_SECRET: %v", err)
		}
		setEnv(t, with(baseEnv(), envAuthSecret, testSecret32))
		cfg, err := config.Load(config.Gateway)
		if err != nil {
			t.Fatalf("config.Load(gateway) with 32 bytes: %v", err)
		}
		out := cfg.String()
		if !strings.Contains(out, "PERPS_AUTH_SECRET=redacted") || strings.Contains(out, testSecret32) {
			t.Fatalf("String() = %q, want the secret redacted", out)
		}
	})

	t.Run("redacts_secrets", func(t *testing.T) {
		vars := baseEnv()
		vars[envRPCURLs] = "https://rpc-user:rpc-password@rpc.example/node?api_key=rpc-token"
		vars[envCometURLs] = "https://comet-user:comet-password@comet.example?token=comet-token"
		vars[envGatewayURL] = "https://gateway-user:gateway-password@gateway.example?access_token=gateway-token"
		vars[envExplorerTx] = "https://explorer-user:explorer-password@explorer.example/tx/?key=explorer-token"
		setEnv(t, vars)
		for _, service := range []config.Service{config.Indexer, config.Gateway, config.Portal, config.Loadtest, config.Replay, config.Explorer} {
			cfg, err := service.Load()
			if err != nil {
				t.Fatal(err)
			}
			assertEndpointRedaction(t, cfg)
		}
		cfg, err := config.Load(config.Gateway)
		if err != nil {
			t.Fatalf("config.Load(gateway): %v", err)
		}
		js, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		for _, out := range []string{fmt.Sprint(cfg), fmt.Sprint(*cfg), fmt.Sprintf("%+v", *cfg), fmt.Sprintf("%#v", *cfg), cfg.LogValue().String(), string(js)} {
			for _, leak := range []string{"p4ss", "n4ts", testSecret32} {
				if strings.Contains(out, leak) {
					t.Fatalf("output %q leaks %q", out, leak)
				}
			}
		}
		out := fmt.Sprint(cfg)
		for _, want := range []string{"DATABASE_URL=redacted", "NATS_URL=redacted", "PERPS_AUTH_SECRET=redacted", "PERPS_CHAIN_ID=125", "PORT=8080"} {
			if !strings.Contains(out, want) {
				t.Fatalf("String() = %q, missing %q", out, want)
			}
		}
	})
}

func TestLoadTrustedProxies(t *testing.T) {
	want := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.168.1.0/24"),
	}
	for _, service := range []config.Service{config.Gateway, config.Explorer} {
		t.Run(string(service), func(t *testing.T) {
			t.Run("valid", func(t *testing.T) {
				setEnv(t, with(baseEnv(), ratelimit.EnvTrustedProxies, "10.0.0.0/8, 192.168.1.0/24"))
				cfg, err := config.Load(service)
				if err != nil {
					t.Fatalf("config.Load(%s): %v", service, err)
				}
				if !slices.Equal(cfg.TrustedProxies, want) {
					t.Fatalf("TrustedProxies = %v, want %v", cfg.TrustedProxies, want)
				}
				if out := cfg.String(); !strings.Contains(out, "PERPS_TRUSTED_PROXY_CIDRS=[10.0.0.0/8 192.168.1.0/24]") {
					t.Fatalf("String() = %q, missing trusted proxy list", out)
				}
			})
			t.Run("empty", func(t *testing.T) {
				setEnv(t, baseEnv())
				cfg, err := config.Load(service)
				if err != nil {
					t.Fatalf("config.Load(%s): %v", service, err)
				}
				if len(cfg.TrustedProxies) != 0 {
					t.Fatalf("TrustedProxies = %v, want empty", cfg.TrustedProxies)
				}
			})
			t.Run("malformed", func(t *testing.T) {
				setEnv(t, with(baseEnv(), ratelimit.EnvTrustedProxies, "not-a-cidr"))
				cfg, err := config.Load(service)
				if err == nil || !strings.Contains(err.Error(), ratelimit.EnvTrustedProxies) {
					t.Fatalf("config.Load(%s) = %v, %v; want error naming %s", service, cfg, err, ratelimit.EnvTrustedProxies)
				}
				if cfg != nil {
					t.Fatalf("config.Load(%s) returned Config with parse error: %v", service, cfg)
				}
			})
		})
	}
	for _, service := range []config.Service{config.Indexer, config.Portal, config.Replay, config.Loadtest} {
		t.Run(string(service)+"_ignores_unused_variable", func(t *testing.T) {
			setEnv(t, with(with(baseEnv(), envGatewayURL, "https://gateway.example"), ratelimit.EnvTrustedProxies, "not-a-cidr"))
			cfg, err := config.Load(service)
			if err != nil {
				t.Fatalf("config.Load(%s): %v", service, err)
			}
			if len(cfg.TrustedProxies) != 0 {
				t.Fatalf("config.Load(%s) read unused proxy list: %v", service, cfg.TrustedProxies)
			}
		})
	}
}

func assertEndpointRedaction(t *testing.T, cfg *config.Config) {
	t.Helper()
	before, err := json.Marshal(struct {
		RPC, Comet        []string
		Gateway, Explorer string
	}{cfg.RPCURLs, cfg.CometURLs, cfg.GatewayURL, cfg.ExplorerTxBase})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{cfg, *cfg} {
		js, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var logs bytes.Buffer
		slog.New(slog.NewJSONHandler(&logs, nil)).Info("startup", "config", value)
		var textLogs bytes.Buffer
		slog.New(slog.NewTextHandler(&textLogs, nil)).Info("startup", "config", value)
		for _, out := range []string{fmt.Sprint(value), fmt.Sprintf("%+v", value), fmt.Sprintf("%#v", value), cfg.LogValue().String(), string(js), logs.String(), textLogs.String()} {
			for _, secret := range []string{"rpc-user", "rpc-password", "rpc-token", "comet-user", "comet-password", "comet-token", "gateway-user", "gateway-password", "gateway-token", "explorer-user", "explorer-password", "explorer-token", "p4ss", "n4ts", testSecret32} {
				if strings.Contains(out, secret) {
					t.Errorf("%s output leaks %q", cfg.Service, secret)
				}
			}
		}
	}
	after, err := json.Marshal(struct {
		RPC, Comet        []string
		Gateway, Explorer string
	}{cfg.RPCURLs, cfg.CometURLs, cfg.GatewayURL, cfg.ExplorerTxBase})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("diagnostics mutated live endpoint credentials")
	}
}
