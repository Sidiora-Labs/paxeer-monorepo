package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
)

func TestPriceRetentionConfiguration(t *testing.T) {
	setEnv(t, baseEnv())
	const name = "PERPS_PRICE_RETENTION_DAYS"
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Indexer.Load()
	if err != nil || cfg.PriceRetentionDays != 7 {
		t.Fatalf("default configuration = %v: %v", cfg, err)
	}
	if !strings.Contains(cfg.String(), name+"=7") {
		t.Fatal("configuration omits price retention days")
	}
	for _, raw := range []string{"1", "7", "30"} {
		t.Setenv(name, raw)
		cfg, err = config.Indexer.Load()
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		want := map[string]int{"1": 1, "7": 7, "30": 30}[raw]
		if cfg.PriceRetentionDays != want {
			t.Fatalf("%s gives %d days", raw, cfg.PriceRetentionDays)
		}
	}
	for _, raw := range []string{"0", "-1", "1.5", "1.0", "seven", "", " 7", "7d", "999999999999999999999999999"} {
		t.Setenv(name, raw)
		cfg, err := config.Indexer.Load()
		if cfg != nil || err == nil || !strings.Contains(err.Error(), name+" must be a whole number of days of at least 1") {
			t.Fatalf("invalid days %q: %v %v", raw, cfg, err)
		}
	}
	t.Setenv(name, "7")
	cfg, err = config.Indexer.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, days := range []int{0, -1} {
		cfg.PriceRetentionDays = days
		if err := cfg.Validate(); err == nil {
			t.Fatalf("direct configuration accepts %d days", days)
		}
	}
	t.Setenv(name, "1.5")
	for _, service := range []config.Service{config.Gateway, config.Explorer, config.Replay} {
		if _, err := service.Load(); err != nil {
			t.Fatalf("%s reads indexer retention setting: %v", service, err)
		}
	}
}
