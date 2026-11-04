// Proves: os.Setenv + config.Load -> config.Config.TrustedProxies ([]netip.Prefix) -> config.Service.Load, ratelimit.Report and ratelimit.ClientKey.
//
// The six perps commands use Service.Load as their startup consumer; gateway
// and explorer pass its TrustedProxies to the real ingress consumers below.
// Configuration has no persistent store: repeated loads and changes to the
// environment prove that startup does not reuse an earlier result.
// Service.Load also repairs the unlabelled configuration refusal in
// cmd/perps-replay/main.go:replayRange and gives every command service context.
package config_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/ratelimit"
)

func proofConfigurationEnv(t *testing.T) {
	t.Helper()
	for name, value := range map[string]string{
		"PERPS_CHAIN_ID":            "125",
		"PERPS_RPC_URLS":            "https://rpc-a.example, http://rpc-b.example",
		"PERPS_COMET_URLS":          "https://comet.example",
		"DATABASE_URL":              "postgres://perps:proof@db.example/perps",
		"NATS_URL":                  "nats://indexer:proof@nats.example:4222",
		"PORT":                      "8080",
		"PERPS_AUTH_SECRET":         "0123456789abcdef0123456789abcdef",
		"PERPS_EXPLORER_TX_BASE":    "https://explorer.example",
		"PERPS_GATEWAY_URL":         "https://gateway.example",
		"PERPS_START_HEIGHT":        "42",
		"PERPS_TRUSTED_PROXY_CIDRS": "10.64.1.27/16, 2001:db8:1::123/48",
	} {
		t.Setenv(name, value)
	}
}

func TestProofConfigurationCommands(t *testing.T) {
	for _, service := range []config.Service{
		config.Explorer, config.Gateway, config.Indexer,
		config.Loadtest, config.Portal, config.Replay,
	} {
		t.Run(string(service), func(t *testing.T) {
			proofConfigurationEnv(t)
			want := config.Config{Service: service, ChainID: 125}
			switch service {
			case config.Indexer, config.Gateway, config.Replay:
				want.RPCURLs = []string{"https://rpc-a.example", "http://rpc-b.example"}
			}
			switch service {
			case config.Indexer, config.Replay:
				want.CometURLs = []string{"https://comet.example"}
			}
			switch service {
			case config.Indexer, config.Gateway, config.Explorer, config.Replay:
				want.DatabaseURL = "postgres://perps:proof@db.example/perps"
			}
			switch service {
			case config.Indexer, config.Gateway:
				want.NATSURL = "nats://indexer:proof@nats.example:4222"
			}
			switch service {
			case config.Indexer, config.Gateway, config.Explorer, config.Portal:
				want.Port = 8080
			}
			switch service {
			case config.Explorer, config.Portal:
				want.ExplorerTxBase = "https://explorer.example"
			}
			switch service {
			case config.Portal, config.Loadtest:
				want.GatewayURL = "https://gateway.example"
			}
			if service == config.Indexer {
				want.StartHeight = 42
			}
			if service == config.Gateway {
				want.AuthSecret = "0123456789abcdef0123456789abcdef"
			}
			if service == config.Gateway || service == config.Explorer {
				want.TrustedProxies = []netip.Prefix{
					netip.MustParsePrefix("10.64.0.0/16"),
					netip.MustParsePrefix("2001:db8:1::/48"),
				}
			}

			producer, err := config.Load(service)
			if err != nil {
				t.Fatalf("producer Load: %v", err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				observed, err := service.Load() // Exactly the six commands' startup call.
				if err != nil {
					t.Fatalf("startup load %d: %v", attempt, err)
				}
				if !reflect.DeepEqual(*observed, want) || !reflect.DeepEqual(observed, producer) {
					t.Fatalf("startup load %d changed the producer's service configuration", attempt)
				}
				if observed == producer {
					t.Fatal("startup reused a cached Config")
				}
				if len(observed.TrustedProxies) > 0 && &observed.TrustedProxies[0] == &producer.TrustedProxies[0] {
					t.Fatal("startup reused a cached proxy list")
				}
				producer = observed
			}

			t.Setenv("PERPS_CHAIN_ID", "126")
			fresh, err := service.Load()
			if fresh != nil || err == nil || err.Error() != string(service)+" config: PERPS_CHAIN_ID must be 125 (the only supported network)" || producer.ChainID != 125 {
				t.Fatalf("startup did not refuse the newly configured unsupported chain: %v", err)
			}
			t.Setenv("PERPS_CHAIN_ID", "125")
			fresh, err = service.Load()
			if err != nil || fresh == nil || fresh == producer || fresh.ChainID != 125 {
				t.Fatalf("startup did not independently recover after refusal: %v", err)
			}
			t.Setenv("PERPS_CHAIN_ID", "not-a-number")
			refused, err := service.Load()
			wantError := string(service) + " config: PERPS_CHAIN_ID must be numeric"
			if refused != nil || err == nil || err.Error() != wantError {
				t.Fatalf("startup refusal = %v, %v; want nil, %q", refused, err, wantError)
			}
			if cause := errors.Unwrap(err); cause == nil || cause.Error() != "PERPS_CHAIN_ID must be numeric" {
				t.Fatalf("startup lost the producer refusal: %v", err)
			}
		})
	}
}

func TestProofConfigurationTrustedProxies(t *testing.T) {
	for _, service := range []config.Service{config.Gateway, config.Explorer} {
		t.Run(string(service), func(t *testing.T) {
			proofConfigurationEnv(t)
			for _, input := range []struct {
				raw      string
				prefixes []string
				key      string
			}{
				{"10.64.1.27/16, 2001:db8:1::123/48", []string{"10.64.0.0/16", "2001:db8:1::/48"}, "ip:198.51.100.7"},
				{"192.0.2.99/24", []string{"192.0.2.0/24"}, "ip:10.64.2.3"},
				{"", []string{}, "ip:10.64.2.3"},
			} {
				t.Setenv(ratelimit.EnvTrustedProxies, input.raw)
				for attempt := 0; attempt < 2; attempt++ {
					cfg, err := service.Load()
					if err != nil {
						t.Fatalf("startup load %d: %v", attempt, err)
					}
					var output bytes.Buffer
					logger := slog.New(slog.NewJSONHandler(&output, nil))
					registry := prometheus.NewRegistry()
					if err := ratelimit.Report(logger, registry.Register, cfg.TrustedProxies); err != nil {
						t.Fatalf("ingress report: %v", err)
					}
					var report struct {
						Proxies []string `json:"trusted_proxies"`
					}
					if err := json.Unmarshal(output.Bytes(), &report); err != nil {
						t.Fatalf("decode ingress report: %v", err)
					}
					if !slices.Equal(report.Proxies, input.prefixes) {
						t.Fatalf("ingress observed %v, want %v", report.Proxies, input.prefixes)
					}
					families, err := registry.Gather()
					if err != nil || len(families) != 1 || families[0].GetName() != "perps_untrusted_proxy_headers_total" {
						t.Fatalf("ingress did not register its real counter: %v", err)
					}
					for _, header := range []string{"X-Real-IP", "X-Forwarded-For"} {
						r, err := http.NewRequest(http.MethodGet, "http://perps.example/api/chain/blocks", nil)
						if err != nil {
							t.Fatal(err)
						}
						// Both public servers call this consumer with their loaded list.
						r.RemoteAddr = "10.64.2.3:12345"
						r.Header.Set(header, "198.51.100.7")
						if key := ratelimit.ClientKey(r, cfg.TrustedProxies); key != input.key {
							t.Fatalf("%s client key = %q, want %q", header, key, input.key)
						}
						r.RemoteAddr = "[2001:db8:1::7]:12345"
						want := "ip:2001:db8:1::7"
						if len(input.prefixes) == 2 {
							want = "ip:198.51.100.7"
						}
						if key := ratelimit.ClientKey(r, cfg.TrustedProxies); key != want {
							t.Fatalf("IPv6 %s client key = %q, want %q", header, key, want)
						}
					}
				}
			}

			for _, malformed := range []string{
				"10.64.0.0/16,not-a-cidr", "10.64.0.0/33", "2001:db8::/129",
				strings.Repeat("10.64.0.0/16,", 32) + "192.0.2.0/24",
			} {
				t.Setenv(ratelimit.EnvTrustedProxies, malformed)
				for attempt := 0; attempt < 2; attempt++ {
					producer, producerErr := config.Load(service)
					if producer != nil || producerErr == nil || !strings.Contains(producerErr.Error(), ratelimit.EnvTrustedProxies) {
						t.Fatalf("malformed producer result = %v, %v", producer, producerErr)
					}
					cfg, err := service.Load()
					if cfg != nil || err == nil || err.Error() != string(service)+" config: "+producerErr.Error() {
						t.Fatalf("consumer did not refuse malformed producer result: %v, %v", cfg, err)
					}
				}
			}
			// A refusal must not poison a later valid startup.
			t.Setenv(ratelimit.EnvTrustedProxies, "10.64.0.0/16")
			cfg, err := service.Load()
			if err != nil || cfg == nil || len(cfg.TrustedProxies) != 1 {
				t.Fatalf("startup after refusal: %v, %v", cfg, err)
			}
		})
	}
}

func TestProofConfigurationUnusedProxies(t *testing.T) {
	for _, service := range []config.Service{config.Indexer, config.Loadtest, config.Portal, config.Replay} {
		t.Run(string(service), func(t *testing.T) {
			proofConfigurationEnv(t)
			t.Setenv(ratelimit.EnvTrustedProxies, "malformed-unused-cidr")
			var previous *config.Config
			for attempt := 0; attempt < 2; attempt++ {
				t.Setenv("PERPS_CHAIN_ID", "125")
				cfg, err := service.Load()
				if err != nil || cfg == nil || cfg == previous || cfg.ChainID != 125 || len(cfg.TrustedProxies) != 0 {
					t.Fatalf("unused proxy variable affected %s startup: %v, %v", service, cfg, err)
				}
				previous = cfg
			}
		})
	}
	unknown := config.Service("unknown")
	if cfg, err := unknown.Load(); cfg != nil || err == nil || err.Error() != `unknown config: unknown service "unknown"` {
		t.Fatalf("unknown startup = %v, %v", cfg, err)
	}
}
