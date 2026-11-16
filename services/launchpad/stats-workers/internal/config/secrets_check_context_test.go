package config

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSecretsCheckContext(t *testing.T) {
	base := productionConfigForTests()
	values := map[string]string{"DATABASE_URL": "DATABASE_URL-initial", "REDIS_URL": "REDIS_URL-initial", "RPC_URL": "RPC_URL-initial", "WEBHOOK_HMAC_SECRET": "WEBHOOK_HMAC_SECRET-initial", "NATS_URL": "NATS_URL-initial", "STATS_CURSOR_KEY": base.StatsCursorKey, "STATS_AUDIT_KEY": base.StatsAuditKey, "IDENTITY_ASSERTION_SIGNING_KEY": base.IdentitySigningKey, "IDENTITY_MTLS_CLIENT_KEY_PEM": base.IdentityMTLSClientKeyPEM, "NATS_CREDENTIALS_CREDS": base.NATSCredentialsCreds}
	for key, value := range values {
		t.Setenv(key, value)
	}
	newCheck := func() (func(context.Context) error, context.CancelFunc) {
		cfg := base
		check, stop, err := cfg.ResolveProductionSecrets(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return check, stop
	}
	check, stop := newCheck()
	defer stop()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := check(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled check = %v", err)
	}
	deadline, expire := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expire()
	if err := check(deadline); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired check = %v", err)
	}
	if err := check(context.Background()); err != nil {
		t.Fatalf("live check after ended contexts = %v", err)
	}
	t.Setenv("DATABASE_URL", "DATABASE_URL-rotated")
	if err := check(context.Background()); err == nil || err.Error() != "required_secret_rotated_restart_required: DATABASE_URL" {
		t.Fatalf("rotation check = %v", err)
	}
	canceled, cancel = context.WithCancel(context.Background())
	cancel()
	if err := check(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("ended rotation check = %v", err)
	}
	t.Setenv("DATABASE_URL", "DATABASE_URL-initial")
	if err := check(context.Background()); err == nil || err.Error() != "required_secret_rotated_restart_required: DATABASE_URL" {
		t.Fatalf("retained rotation check = %v", err)
	}
	missing, missingStop := newCheck()
	defer missingStop()
	t.Setenv("REDIS_URL", "")
	if err := missing(context.Background()); err == nil || err.Error() != "required_secret_unavailable: REDIS_URL" {
		t.Fatalf("missing check = %v", err)
	}
	deadline, expire = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expire()
	if err := missing(deadline); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ended missing check = %v", err)
	}
	t.Setenv("REDIS_URL", "REDIS_URL-initial")
	if err := missing(context.Background()); err == nil || err.Error() != "required_secret_unavailable: REDIS_URL" {
		t.Fatalf("retained missing check = %v", err)
	}
}
