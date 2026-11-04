package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

func repairM952Environment(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"CHAIN_ID", "LOG_LEVEL", "NODE_ENV", "PORT", "RPC_URL_FALLBACK",
		"DEPLOYMENT_REALM", "PRIVATE_FIXTURE_MANIFEST_FILE", "PRIVATE_FIXTURE_MANIFEST_SHA256",
		"SECRETS_PROVIDER", "SECRETS_REFRESH_SECONDS",
	} {
		t.Setenv(key, "")
	}
	validEnv(t)
}

func TestRepairM_952_ReviewedSuccessFixtures(t *testing.T) {
	repairM952Environment(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RPCURL != "https://api1.mainnet-beta.paxeer.network" || cfg.RPCURLFallback != "" || cfg.ChainID != 125 {
		t.Fatalf("success fixture must load chain 125 with the reviewed primary origin and no fallback")
	}
	t.Run("defaults", TestLoadValidWithDefaults)
	t.Run("overrides", TestLoadCoercesAndOverrides)
}

func TestRepairM_952_CurrentRPCKeys(t *testing.T) {
	origins := []string{"https://api-mainnet-beta.paxeer.network"}
	for i := 1; i <= 14; i++ {
		origins = append(origins, fmt.Sprintf("https://api%d.mainnet-beta.paxeer.network", i))
	}
	for i, primary := range origins {
		fallback := origins[(i+1)%len(origins)]
		t.Run(primary, func(t *testing.T) {
			repairM952Environment(t)
			t.Setenv("RPC_URL", primary)
			t.Setenv("RPC_URL_FALLBACK", fallback)
			for _, loader := range []struct {
				name string
				load func() (BaseEnv, error)
			}{
				{"Load", Load},
				{"LoadWithManifest", func() (BaseEnv, error) { return LoadWithManifest(deployment.Current()) }},
			} {
				t.Run(loader.name, func(t *testing.T) {
					cfg, err := loader.load()
					if err != nil {
						t.Fatal(err)
					}
					if cfg.ChainID != 125 || cfg.RPCURL != primary || cfg.RPCURLFallback != fallback {
						t.Fatal("environment parsing must preserve both reviewed RPC origins and chain 125")
					}
				})
			}
		})
	}
}

func TestRepairM_952_NegativeRPCFixtures(t *testing.T) {
	for _, key := range []string{"RPC_URL", "RPC_URL_FALLBACK"} {
		for _, raw := range []string{"not-a-url", "::::bad", "file://localhost/etc/passwd", "https://operator:super-secret@%%/private-key"} {
			t.Run(key+"/"+fmt.Sprint(len(raw)), func(t *testing.T) {
				repairM952Environment(t)
				t.Setenv(key, raw)
				cfg, err := Load()
				if err == nil || !strings.Contains(err.Error(), key+": url_") {
					t.Fatalf("expected actionable URL refusal for %s, got %v", key, err)
				}
				if cfg != (BaseEnv{}) {
					t.Fatal("invalid RPC configuration returned a partial environment")
				}
				for _, secret := range []string{"operator", "super-secret", "private-key"} {
					if strings.Contains(err.Error(), secret) {
						t.Fatal("URL refusal disclosed private endpoint data")
					}
				}
			})
		}
	}
	t.Run("redaction", func(t *testing.T) {
		repairM952Environment(t)
		TestPrivateEnvironmentRedaction(t)
	})
	t.Run("conflicting_private_export", func(t *testing.T) {
		repairM952Environment(t)
		TestConflictingPrivateConfigNeverExports(t)
	})
}
