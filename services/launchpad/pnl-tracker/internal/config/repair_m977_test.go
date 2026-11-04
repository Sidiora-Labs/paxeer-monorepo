package config_test

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/config"
)

// repairM977Environment supplies the production loader with the committed
// deployment fixture, independently of the environment running the gate.
func repairM977Environment(t *testing.T) {
	t.Helper()
	var clear func(reflect.Type)
	clear = func(typ reflect.Type) {
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Anonymous {
				clear(field.Type)
			}
			name := strings.Split(field.Tag.Get("env"), ",")[0]
			if name != "" {
				t.Setenv(name, "")
				if err := os.Unsetenv(name); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	clear(reflect.TypeOf(config.Config{}))
	setBaseEnv(t)
}

func TestRepairM_977_ValidIntervals(t *testing.T) {
	const largestMS int64 = (1<<63 - 1) / 1_000_000
	for _, tc := range []struct {
		name  string
		input string
		ms    int64
	}{
		{"default", "", 30000},
		{"minimum", "1", 1},
		{"override", "5000", 5000},
		{"below_maximum", strconv.FormatInt(largestMS-1, 10), largestMS - 1},
		{"maximum", strconv.FormatInt(largestMS, 10), largestMS},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repairM977Environment(t)
			if tc.input != "" {
				t.Setenv("PNL_RECONCILE_INTERVAL_MS", tc.input)
			}
			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("Load interval %q: %v", tc.input, err)
			}
			if int64(cfg.ReconcileIntervalMS) != tc.ms {
				t.Fatalf("interval milliseconds = %d, want %d", cfg.ReconcileIntervalMS, tc.ms)
			}
			interval := cfg.ReconcileInterval()
			if want := time.Duration(tc.ms * 1_000_000); interval != want || interval <= 0 {
				t.Fatalf("interval = %v, want positive %v", interval, want)
			}
			// This is the actual runtime consumer that previously panicked for
			// accepted non-positive or overflowing operator input.
			ticker := time.NewTicker(interval)
			ticker.Stop()
		})
	}
}

func TestRepairM_977_RejectInvalidIntervals(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"zero", "0"},
		{"negative", "-1"},
		{"minimum_integer", "-9223372036854775808"},
		{"above_maximum", "9223372036855"},
		{"overflow_wraps_positive", "18446744073710"},
		{"maximum_integer", "9223372036854775807"},
		{"integer_overflow", "9223372036854775808"},
		{"negative_integer_overflow", "-9223372036854775809"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repairM977Environment(t)
			t.Setenv("PNL_RECONCILE_INTERVAL_MS", tc.input)
			cfg, err := config.Load()
			if err == nil {
				t.Fatalf("Load accepted invalid interval %q", tc.input)
			}
			if !strings.Contains(err.Error(), "PNL_RECONCILE_INTERVAL_MS") {
				t.Fatalf("Load failed for an unrelated reason: %v", err)
			}
			if !reflect.DeepEqual(cfg, config.Config{}) {
				t.Fatal("Load returned a usable configuration after rejecting the interval")
			}
		})
	}
}
