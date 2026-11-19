package config

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSecretsCheckContext(t *testing.T) {
	for key, value := range map[string]string{
		"DATABASE_URL": "database-initial", "REDIS_URL": "redis-initial", "ADMIN_API_KEY": "admin-initial",
		"IDENTITY_SIGNING_KEY": "identity-initial", "STREAM_SIGNING_KEY": "stream-initial",
	} {
		t.Setenv(key, value)
	}
	base := Config{NodeEnv: "production", SecretsProvider: "env", SecretsRefreshSeconds: 1, AdminAPIKey: "configured"}
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
	t.Setenv("DATABASE_URL", "database-rotated")
	if err := check(context.Background()); err == nil || err.Error() != "required_secret_rotated_restart_required: DATABASE_URL" {
		t.Fatalf("rotation check = %v", err)
	}
	canceled, cancel = context.WithCancel(context.Background())
	cancel()
	if err := check(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("ended rotation check = %v", err)
	}
	t.Setenv("DATABASE_URL", "database-initial")
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
	t.Setenv("REDIS_URL", "redis-initial")
	if err := missing(context.Background()); err == nil || err.Error() != "required_secret_unavailable: REDIS_URL" {
		t.Fatalf("retained missing check = %v", err)
	}
}
