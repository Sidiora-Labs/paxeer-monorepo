package config_test

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/config"
	sharedconfig "github.com/Sidiora-Technologies/KindleLaunch/shared/config"
)

func TestIdentityKeyVariableLoad(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("IDENTITY_MTLS_CLIENT_KEY_PEM", "")
	cfg, err := config.Load()
	if err != nil || cfg.IdentityMTLSClientKeyPEM != "" {
		t.Fatal("identity client key must default to empty")
	}
	t.Setenv("IDENTITY_MTLS_CLIENT_KEY_PEM", "client-key-text")
	cfg, err = config.Load()
	if err != nil || cfg.IdentityMTLSClientKeyPEM != "client-key-text" {
		t.Fatal("identity client key variable was not loaded")
	}
}

func TestIdentityKeyVariableSecretBinding(t *testing.T) {
	for _, tc := range []struct {
		name, file, document, key string
		bound                     bool
	}{
		{"journey file", "bound", "", "input", true},
		{"journey document", "", "bound", "input", true},
		{"empty with file", "bound", "", "", false},
		{"empty with document", "", "bound", "", false},
		{"no journey", "", "", "input", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := config.Config{
				BaseEnv:              sharedconfig.BaseEnv{NodeEnv: "production", SecretsProvider: "env", SecretsRefreshSeconds: 60},
				NATSCredentialsCreds: "bound", JourneyConfigFile: tc.file,
				JourneyConfigDocument: tc.document, IdentityMTLSClientKeyPEM: tc.key,
			}
			for _, name := range []string{"DATABASE_URL", "REDIS_URL", "RPC_URL", "WEBHOOK_HMAC_SECRET", "NATS_URL", "NATS_CREDENTIALS_CREDS", "IDENTITY_SIGNING_KEY", "PNL_CURSOR_KEY"} {
				t.Setenv(name, "value-"+name)
			}
			value := ""
			if tc.bound {
				value = "resolved-client-key"
			}
			t.Setenv("IDENTITY_MTLS_CLIENT_KEY_PEM", value)
			check, stop, err := cfg.ResolveProductionSecrets(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer stop()
			want := tc.key
			if tc.bound {
				want = value
			}
			if cfg.IdentityMTLSClientKeyPEM != want {
				t.Fatal("identity client key secret binding differs")
			}
			if err = check(ctx); err != nil {
				t.Fatal(err)
			}
			t.Setenv("IDENTITY_MTLS_CLIENT_KEY_PEM", "rotated-client-key")
			err = check(ctx)
			if tc.bound {
				if err == nil || err.Error() != "required_secret_rotated_restart_required: IDENTITY_MTLS_CLIENT_KEY_PEM" {
					t.Fatal("bound identity key rotation must require restart")
				}
			} else if err != nil {
				t.Fatal("unused identity key must not be requested")
			}
		})
	}
}
