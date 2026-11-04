package config_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
)

func repairM976Environment(t *testing.T) {
	t.Helper()
	typ := reflect.TypeOf(config.Config{})
	for i := 0; i < typ.NumField(); i++ {
		key := strings.Split(typ.Field(i).Tag.Get("env"), ",")[0]
		if key == "" {
			continue
		}
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("clear %s: %v", key, err)
		}
	}
	t.Setenv("DATABASE_URL", "postgres://localhost/launchpad")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
}

func repairM976CheckError(t *testing.T, err error, want string, secrets ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("malformed credentialed configuration was accepted")
	}
	if got := err.Error(); got != want {
		t.Errorf("error = %q, want variable and rule only: %q", got, want)
	}
	for current := err; current != nil; current = errors.Unwrap(current) {
		for _, rendered := range []string{current.Error(), fmt.Sprintf("%+v", current), fmt.Sprintf("%#v", current)} {
			for _, secret := range secrets {
				if strings.Contains(rendered, secret) {
					t.Error("error chain exposes credential or DSN bytes")
				}
			}
		}
		if joined, ok := current.(interface{ Unwrap() []error }); ok {
			for _, child := range joined.Unwrap() {
				repairM976CheckError(t, child, child.Error(), secrets...)
			}
		}
	}
	var output bytes.Buffer
	log.New(&output, "", 0).Printf("apid: %v", err)
	if got := output.String(); got != "apid: "+want+"\n" {
		t.Errorf("startup log = %q, want variable and rule only", got)
	}
	for _, secret := range secrets {
		if strings.Contains(output.String(), secret) {
			t.Error("startup log exposes credential or DSN bytes")
		}
	}
}

func TestRepairM_976_CredentialedURLErrors(t *testing.T) {
	const password = "m976-private-password"
	const databaseRule = "DATABASE_URL must be a valid URL with a scheme and host"
	const redisRule = "REDIS_URL must be a valid redis:// or rediss:// URL with a host"
	cases := []struct {
		name, variable, dsn, rule string
	}{
		{"database_parse_error", "DATABASE_URL", "postgres://dbuser:" + password + "@localhost:invalid/db", databaseRule},
		{"database_missing_host", "DATABASE_URL", "postgres://dbuser:" + password + "@/db", databaseRule},
		{"database_missing_scheme", "DATABASE_URL", "//dbuser:" + password + "@localhost/db", databaseRule},
		{"redis_parse_error", "REDIS_URL", "redis://redisuser:" + password + "@localhost:invalid/0", redisRule},
		{"redis_missing_host", "REDIS_URL", "redis://redisuser:" + password + "@/0", redisRule},
		{"rediss_parse_error", "REDIS_URL", "rediss://redisuser:" + password + "@localhost:invalid/0", redisRule},
		{"rediss_missing_host", "REDIS_URL", "rediss://redisuser:" + password + "@/0", redisRule},
		{"redis_wrong_scheme", "REDIS_URL", "https://redisuser:" + password + "@localhost/0", redisRule},
		{"database_invalid_escape", "DATABASE_URL", "postgres://dbuser:" + password + "%zz@localhost/db", databaseRule},
		{"redis_invalid_escape", "REDIS_URL", "redis://redisuser:" + password + "%zz@localhost/0", redisRule},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repairM976Environment(t)
			t.Setenv(tc.variable, tc.dsn)
			cfg, err := config.Load()
			if cfg != (config.Config{}) {
				t.Error("Load returned configuration after validation failed")
			}
			want := "config: " + tc.rule
			repairM976CheckError(t, err, want, password, tc.dsn, "dbuser", "redisuser")
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			repairM976CheckError(t, app.Run(ctx), want, password, tc.dsn, "dbuser", "redisuser")
		})
	}
}

func TestRepairM_976_JoinedCredentialErrors(t *testing.T) {
	repairM976Environment(t)
	database := "postgres://dbuser:database-private-password@/db"
	redis := "redis://redisuser:redis-private-password@/0"
	t.Setenv("DATABASE_URL", database)
	t.Setenv("REDIS_URL", redis)
	want := "config: DATABASE_URL must be a valid URL with a scheme and host\nREDIS_URL must be a valid redis:// or rediss:// URL with a host"
	_, err := config.Load()
	repairM976CheckError(t, err, want, database, redis, "database-private-password", "redis-private-password", "dbuser", "redisuser")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repairM976CheckError(t, app.Run(ctx), want, database, redis, "database-private-password", "redis-private-password", "dbuser", "redisuser")
}

func TestRepairM_976_ValidCredentialedURLs(t *testing.T) {
	for _, scheme := range []string{"redis", "rediss"} {
		t.Run(scheme, func(t *testing.T) {
			repairM976Environment(t)
			database := "postgres://dbuser:valid%40password@localhost:5432/db"
			redis := scheme + "://redisuser:valid%40password@localhost:6379/0"
			t.Setenv("DATABASE_URL", database)
			t.Setenv("REDIS_URL", redis)
			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("valid credentialed URLs rejected: %v", err)
			}
			if cfg.DatabaseURL != database || cfg.RedisURL != redis {
				t.Error("Load changed valid credentialed URLs")
			}
		})
	}
}
