package config

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSecretsCheckContext(t *testing.T) {
	for _, key := range []string{"REDIS_URL", "GATEWAY_JWT_SECRET", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", "DATABASE_URL", "IDENTITY_SERVICE_TOKEN", "IDENTITY_LIMITER_KEY", "IDENTITY_SIGNING_SECRET", "RPC_URL"} {
		t.Setenv(key, key+"-initial")
	}
	base := Config{NodeEnv: nodeEnvironmentProduction, SecretsProvider: "env", SecretsRefreshSeconds: 1}
	newCheck := func() (func(context.Context) error, context.CancelFunc) {
		check, stop, err := (&base).ResolveProductionSecrets(context.Background())
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
	t.Setenv("RPC_URL", "RPC_URL-rotated")
	if err := check(context.Background()); err == nil || err.Error() != "required_secret_rotated_restart_required: RPC_URL" {
		t.Fatalf("rotation check = %v", err)
	}
	canceled, cancel = context.WithCancel(context.Background())
	cancel()
	if err := check(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("ended rotation check = %v", err)
	}
	t.Setenv("RPC_URL", "RPC_URL-initial")
	if err := check(context.Background()); err == nil || err.Error() != "required_secret_rotated_restart_required: RPC_URL" {
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
