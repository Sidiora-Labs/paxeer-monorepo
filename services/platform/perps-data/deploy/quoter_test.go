package deploy

import (
	"errors"
	"maps"
	"os"
	"slices"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/stretchr/testify/require"
)

var quoterVariables = []string{
	"PERPS_CHAIN_ID", "PERPS_RPC_URLS", "PORT",
	"PERPS_QUOTER_KEY_FILE", "PERPS_QUOTER_MARKETS", "PERPS_QUOTER_LEVELS", "PERPS_QUOTER_SPREAD_BPS", "PERPS_QUOTER_STEP_BPS", "PERPS_QUOTER_LEVEL_USD", "PERPS_QUOTER_REFRESH_BPS", "PERPS_QUOTER_MAX_QUOTE_AGE_SECONDS", "PERPS_QUOTER_MAX_POSITION_USD", "PERPS_QUOTER_MAX_PRICE_AGE_SECONDS", "PERPS_QUOTER_INTERVAL_SECONDS", "PERPS_QUOTER_REFUSAL_LIMIT", "PERPS_QUOTER_COOLDOWN_SECONDS", "PERPS_QUOTER_MAX_TRANSACTIONS_PER_ROUND", "PERPS_QUOTER_MODE", "PERPS_QUOTER_STOP", "PERPS_QUOTER_HANDOVER",
}

func checkQuoterService(s Service) error {
	if s.Name != "perps-quoter" || s.Build == nil || s.Build.Dockerfile != dockerfilePath || s.Build.Service != "perps-quoter" || !slices.Equal(s.Build.Inputs, imageInputs) {
		return errors.New("perps-quoter must use the shared image build")
	}
	if s.Run != "manual" || s.Replicas != 0 || s.Restart != "never" || s.Public || s.Healthcheck.Kind != "none" || s.Provider != "" || s.Volume != nil || s.Members != nil {
		return errors.New("perps-quoter must be one owner-started private process without automatic restart")
	}
	want := slices.Clone(quoterVariables)
	slices.Sort(want)
	if !slices.Equal(slices.Sorted(maps.Keys(s.Variables)), want) {
		return errors.New("perps-quoter environment inputs must match its configuration")
	}
	for _, v := range s.Variables {
		if v != (Value{Source: "input"}) {
			return errors.New("perps-quoter configuration must come from owner inputs")
		}
	}
	return nil
}
func TestQuoterDeploymentAndEnvironment(t *testing.T) {
	raw, err := os.ReadFile("services.json")
	require.NoError(t, err)
	layout, err := decodeServices(raw)
	require.NoError(t, err)
	byName, err := layout.byName()
	require.NoError(t, err)
	q := byName["perps-quoter"]
	require.NoError(t, checkQuoterService(q))
	values := map[string]string{"PERPS_CHAIN_ID": "125", "PERPS_RPC_URLS": "http://localhost", "PORT": "8081", "PERPS_QUOTER_KEY_FILE": "account-key", "PERPS_QUOTER_MARKETS": "0,11", "PERPS_QUOTER_LEVELS": "2", "PERPS_QUOTER_SPREAD_BPS": "10", "PERPS_QUOTER_STEP_BPS": "10", "PERPS_QUOTER_LEVEL_USD": "25", "PERPS_QUOTER_REFRESH_BPS": "5", "PERPS_QUOTER_MAX_QUOTE_AGE_SECONDS": "30", "PERPS_QUOTER_MAX_POSITION_USD": "500", "PERPS_QUOTER_MAX_PRICE_AGE_SECONDS": "10", "PERPS_QUOTER_INTERVAL_SECONDS": "2", "PERPS_QUOTER_REFUSAL_LIMIT": "3", "PERPS_QUOTER_COOLDOWN_SECONDS": "30", "PERPS_QUOTER_MAX_TRANSACTIONS_PER_ROUND": "8", "PERPS_QUOTER_MODE": "dry", "PERPS_QUOTER_STOP": "false", "PERPS_QUOTER_HANDOVER": "false"}
	for name, value := range values {
		t.Setenv(name, value)
	}
	cfg, err := config.Quoter.Load()
	require.NoError(t, err)
	require.Equal(t, "dry", cfg.Quoting.Mode)
	for _, name := range quoterVariables {
		t.Run(name, func(t *testing.T) {
			value := "invalid"
			if name == "PERPS_QUOTER_KEY_FILE" {
				value = ""
			}
			t.Setenv(name, value)
			_, err := config.Quoter.Load()
			require.Error(t, err)
		})
	}
	for _, change := range []func(*Service){func(s *Service) { s.Public = true }, func(s *Service) { s.Replicas = 2 }, func(s *Service) { s.Restart = "always" }, func(s *Service) { s.Run = "service" }} {
		mutated := q
		change(&mutated)
		require.Error(t, checkQuoterService(mutated))
	}
}
