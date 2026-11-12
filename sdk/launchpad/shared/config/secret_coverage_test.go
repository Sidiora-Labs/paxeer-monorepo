package config

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRequiredSecretsTrackEnvironmentRotation(t *testing.T) {
	ctx := context.Background()
	config := BaseEnv{SecretsProvider: "env", SecretsRefreshSeconds: 30}
	if provider, values, err := (BaseEnv{SecretsProvider: "env"}).RequiredSecrets(ctx, "SDK_COVERAGE_VALUE"); err == nil || provider != nil || values != nil {
		t.Fatalf("unbounded refresh = %v, %+v, %v", provider, values, err)
	}
	if provider, values, err := config.RequiredSecrets(ctx); err == nil || provider != nil || values != nil {
		t.Fatalf("empty required inventory = %v, %+v, %v", provider, values, err)
	}
	if provider, values, err := (BaseEnv{SecretsProvider: "cloud", SecretsRefreshSeconds: 30}).RequiredSecrets(ctx, "SDK_COVERAGE_VALUE"); err == nil || provider != nil || values != nil {
		t.Fatalf("unavailable provider = %v, %+v, %v", provider, values, err)
	}
	t.Setenv("SDK_COVERAGE_VALUE", "")
	if provider, values, err := config.RequiredSecrets(ctx, "SDK_COVERAGE_VALUE"); err == nil || provider != nil || values != nil {
		t.Fatalf("empty required value = %v, %+v, %v", provider, values, err)
	}
	t.Setenv("SDK_COVERAGE_VALUE", "first-value")
	provider, values, err := config.RequiredSecrets(ctx, "SDK_COVERAGE_VALUE")
	if err != nil || provider == nil || values["SDK_COVERAGE_VALUE"] != "first-value" || config.SecretRefreshInterval() != 30*time.Second {
		t.Fatalf("loaded required value = %+v, %v", values, err)
	}
	check := RequiredSecretsCheck(provider, values)
	if err := check(ctx); err != nil {
		t.Fatalf("unchanged required value rejected: %v", err)
	}
	values["SDK_COVERAGE_VALUE"] = "caller-change"
	if err := check(ctx); err != nil {
		t.Fatalf("caller map changed captured value: %v", err)
	}
	t.Setenv("SDK_COVERAGE_VALUE", "next-value")
	if err := check(ctx); err == nil || !strings.Contains(err.Error(), "rotated_restart_required") {
		t.Fatalf("rotated value was retained: %v", err)
	}
	t.Setenv("SDK_COVERAGE_VALUE", "")
	if err := check(ctx); err == nil || !strings.Contains(err.Error(), "required_secret_unavailable") {
		t.Fatalf("missing current value was retained: %v", err)
	}
	if err := RequiredSecretsCheck(nil, values)(ctx); err == nil {
		t.Fatal("required value check admitted an absent provider")
	}
	for _, role := range []string{"rest", "sockets"} {
		if err := config.ValidateAPIRole(role); err != nil {
			t.Fatalf("registered role %s rejected: %v", role, err)
		}
	}
	if err := (BaseEnv{NodeEnv: "test"}).ValidateAPIRole("development"); err != nil {
		t.Fatalf("development role in test environment rejected: %v", err)
	}
	if err := config.ValidateAPIRole("development"); err == nil {
		t.Fatal("development role admitted to production environment")
	}
}
