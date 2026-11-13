package config

import (
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

func TestEnvironmentValidationBindsRealmManifestAndSecretsPolicy(t *testing.T) {
	validEnv(t)
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	manifest := deployment.Current()
	var absent *BaseEnv
	if err := absent.Validate(); err == nil {
		t.Fatal("missing environment admitted")
	}
	if err := absent.ValidateAgainst(manifest); err == nil {
		t.Fatal("missing deployment environment admitted")
	}
	for _, mode := range []string{"realm", "private file", "manifest", "provider", "refresh", "fallback", "missing role"} {
		t.Run(mode, func(t *testing.T) {
			candidate, m := config, manifest
			switch mode {
			case "realm":
				candidate.DeploymentRealm = "unknown"
			case "private file":
				candidate.PrivateFixtureManifestFile = "manifest.json"
			case "manifest":
				m.ChainID = 1
			case "provider":
				candidate.SecretsProvider = "unavailable"
			case "refresh":
				candidate.SecretsRefreshSeconds = 0
			case "fallback":
				candidate.RPCURLFallback = "file:///rpc"
			case "missing role":
				m.Roles = make(map[string]deployment.Role, len(manifest.Roles))
				for name, role := range manifest.Roles {
					if name != "Router" {
						m.Roles[name] = role
					}
				}
			}
			if mode == "missing role" {
				if err := candidate.validateEnvironmentAgainst(m); err == nil {
					t.Fatal("incomplete deployment roles admitted")
				}
			} else if err := candidate.ValidateAgainst(m); err == nil {
				t.Fatal("invalid environment admitted")
			}
		})
	}
	config.DeploymentRealm = valuePrivateFixture
	if err := config.Validate(); err == nil {
		t.Fatal("private realm without binding admitted")
	}
	if err := requireURL("RPC_URL", "%", "http"); err == nil {
		t.Fatal("malformed transport URL admitted")
	}
	if fixtureLoopbackRPC("http://127.0.0.1:8545?token=private") {
		t.Fatal("query-bearing fixture endpoint admitted")
	}
	t.Setenv("CHAIN_ID", "invalid")
	if _, err := LoadWithManifest(manifest); err == nil || !strings.Contains(err.Error(), "environment_parse_failed") {
		t.Fatalf("malformed environment: %v", err)
	}
	t.Setenv("DATABASE_URL", "")
	if _, err := LoadWithManifest(manifest); err == nil {
		t.Fatal("missing required environment admitted")
	}
}
