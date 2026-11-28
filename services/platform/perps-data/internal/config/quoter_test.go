package config_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/stretchr/testify/require"
)

func quoterEnv(t *testing.T) {
	t.Helper()
	t.Setenv("PERPS_CHAIN_ID", "125")
	t.Setenv("PERPS_RPC_URLS", "https://user:password@rpc.example/private-provider-token?key=query-secret")
	t.Setenv("PORT", "")
	t.Setenv("PERPS_QUOTER_KEY_FILE", "/private/account-key")
	for _, name := range []string{"MARKETS", "MODE", "LEVELS", "SPREAD_BPS", "STEP_BPS", "LEVEL_USD", "REFRESH_BPS", "MAX_QUOTE_AGE_SECONDS", "MAX_POSITION_USD", "MAX_PRICE_AGE_SECONDS", "INTERVAL_SECONDS", "REFUSAL_LIMIT", "COOLDOWN_SECONDS", "MAX_TRANSACTIONS_PER_ROUND", "STOP", "HANDOVER"} {
		// Optional inputs are unset rather than assigned empty strings.
		full := "PERPS_QUOTER_" + name
		t.Setenv(full, "")
		require.NoError(t, os.Unsetenv(full))
	}
}
func TestQuoterDefaultsAndExplicitLive(t *testing.T) {
	quoterEnv(t)
	cfg, err := config.Quoter.Load()
	require.NoError(t, err)
	q := cfg.Quoting
	require.Equal(t, "dry", q.Mode)
	require.Empty(t, q.Markets)
	require.Equal(t, 2, q.Levels)
	require.Equal(t, uint64(10), q.SpreadBPS)
	require.Equal(t, uint64(10), q.StepBPS)
	require.Equal(t, uint64(5), q.RefreshBPS)
	require.Equal(t, "25000000000000000000", q.LevelUSD.String())
	require.Equal(t, "500000000000000000000", q.MaxPositionUSD.String())
	require.Equal(t, 30*time.Second, q.MaxQuoteAge)
	require.Equal(t, 10*time.Second, q.MaxPriceAge)
	require.Equal(t, 2*time.Second, q.Interval)
	require.Equal(t, 30*time.Second, q.Cooldown)
	require.Equal(t, 3, q.RefusalLimit)
	require.Equal(t, 8, q.MaxTransactionsPerRound)
	require.Equal(t, uint16(8081), cfg.Port)
	require.False(t, q.Stop)
	require.False(t, q.Handover)
	t.Setenv("PERPS_QUOTER_MODE", "live")
	t.Setenv("PERPS_QUOTER_MAX_TRANSACTIONS_PER_ROUND", "4")
	t.Setenv("PERPS_QUOTER_MARKETS", "0, 11")
	t.Setenv("PERPS_QUOTER_LEVEL_USD", "25.125")
	t.Setenv("PERPS_QUOTER_STOP", "true")
	t.Setenv("PORT", "8090")
	cfg, err = config.Quoter.Load()
	require.NoError(t, err)
	require.Equal(t, "live", cfg.Quoting.Mode)
	require.Equal(t, 4, cfg.Quoting.MaxTransactionsPerRound)
	require.Equal(t, []uint64{0, 11}, cfg.Quoting.Markets)
	require.Equal(t, "25125000000000000000", cfg.Quoting.LevelUSD.String())
	require.True(t, cfg.Quoting.Stop)
	require.Equal(t, uint16(8090), cfg.Port)
}
func TestQuoterRejectsInvalidBoundsWithoutEchoingInputs(t *testing.T) {
	for name, value := range map[string]string{
		"KEY_FILE": "", "MODE": "secret-live", "MARKETS": "0,0", "LEVELS": "17", "SPREAD_BPS": "10000", "STEP_BPS": "9999", "LEVEL_USD": "1.0000000000000000001", "REFRESH_BPS": "0", "MAX_QUOTE_AGE_SECONDS": "0", "MAX_POSITION_USD": "-1", "MAX_PRICE_AGE_SECONDS": "86401", "INTERVAL_SECONDS": "1.5", "REFUSAL_LIMIT": "0", "COOLDOWN_SECONDS": "0", "MAX_TRANSACTIONS_PER_ROUND": "257", "STOP": "yes", "HANDOVER": "yes",
	} {
		t.Run(name, func(t *testing.T) {
			quoterEnv(t)
			t.Setenv("PERPS_QUOTER_"+name, value)
			cfg, err := config.Quoter.Load()
			require.Error(t, err)
			require.Nil(t, cfg)
			require.NotContains(t, err.Error(), "secret-live")
		})
	}
}
func TestQuoterDiagnosticsOmitKeyAndEntireEndpoint(t *testing.T) {
	quoterEnv(t)
	cfg, err := config.Quoter.Load()
	require.NoError(t, err)
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	var logs bytes.Buffer
	slog.New(slog.NewJSONHandler(&logs, nil)).Info("config", "config", cfg, "quoting", cfg.Quoting)
	for _, out := range []string{fmt.Sprint(cfg), fmt.Sprintf("%+v", cfg), fmt.Sprintf("%#v", cfg), cfg.LogValue().String(), cfg.Quoting.String(), fmt.Sprintf("%#v", cfg.Quoting), logs.String(), string(raw)} {
		for _, secret := range []string{"user:", "password", "rpc.example", "private-provider-token", "query-secret", "account-key"} {
			require.False(t, strings.Contains(out, secret), "diagnostic contains %s", secret)
		}
	}
	require.Contains(t, cfg.RPCURLs[0], "private-provider-token", "diagnostics must preserve the actual connection config")
}
func TestParseUSDIsExact(t *testing.T) {
	for raw, want := range map[string]string{"0.000000000000000001": "1", "10": "10000000000000000000", "25.125": "25125000000000000000"} {
		n, err := config.ParseUSD(raw)
		require.NoError(t, err)
		require.Equal(t, want, n.String())
	}
	for _, raw := range []string{"0", "-1", "1e18", "NaN", "1.", ".5", "1.0000000000000000001", "+5", " 1"} {
		_, err := config.ParseUSD(raw)
		require.Error(t, err)
	}
}

func TestQuoterTransactionBound(t *testing.T) {
	for _, raw := range []string{"0", "-1", "257", "1.5", ""} {
		t.Run(raw, func(t *testing.T) {
			quoterEnv(t)
			t.Setenv("PERPS_QUOTER_MAX_TRANSACTIONS_PER_ROUND", raw)
			_, err := config.Quoter.Load()
			require.Error(t, err)
		})
	}
}

func TestQuoterHandoverAndUpperTransactionBound(t *testing.T) {
	for _, raw := range []string{"true", "false"} {
		t.Run(raw, func(t *testing.T) {
			quoterEnv(t)
			t.Setenv("PERPS_QUOTER_HANDOVER", raw)
			t.Setenv("PERPS_QUOTER_MAX_TRANSACTIONS_PER_ROUND", "256")
			cfg, err := config.Quoter.Load()
			require.NoError(t, err)
			require.Equal(t, raw == "true", cfg.Quoting.Handover)
			require.Equal(t, 256, cfg.Quoting.MaxTransactionsPerRound)
		})
	}
}
