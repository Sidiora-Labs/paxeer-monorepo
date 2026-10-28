package app_test

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/app"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestApplicationLeaderScheduleFromConfig(t *testing.T) {
	first := ed25519.TestSecretKey([]byte("leader01")).Public().Bytes()
	second := ed25519.TestSecretKey([]byte("leader02")).Public().Bytes()
	want := [][32]byte{[32]byte(first), [32]byte(second)}
	cfg := app.DefaultConfig(t.TempDir())
	_, err := toml.Decode(fmt.Sprintf(`
leader_pin = [%q, %q]
leader_pin_height = 3
`, hex.EncodeToString(first), strings.ToUpper(hex.EncodeToString(second))), cfg)
	require.NoError(t, err)
	application, err := app.NewApplication(cfg)
	require.NoError(t, err)
	var source types.LeaderSource = application
	for _, height := range []int64{1, 3, 5} {
		activation, leaders, err := source.LeaderSchedule(height)
		require.NoError(t, err)
		require.Equal(t, int64(3), activation)
		require.Equal(t, want, leaders)
	}

	cfg.LeaderPin[0] = "invalid"
	cfg.LeaderPinHeight = 100
	_, leaders, err := source.LeaderSchedule(5)
	require.NoError(t, err)
	leaders[0][0] ^= 0xff
	activation, leaders, err := source.LeaderSchedule(5)
	require.NoError(t, err)
	require.Equal(t, int64(3), activation)
	require.Equal(t, want, leaders)

	for _, tc := range []struct {
		name string
		key  string
	}{
		{name: "invalid hex", key: strings.Repeat("zz", 32)},
		{name: "odd hex length", key: strings.Repeat("a", 63)},
		{name: "short key", key: strings.Repeat("ab", 31)},
		{name: "long key", key: strings.Repeat("ab", 33)},
		{name: "empty key", key: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := app.DefaultConfig(t.TempDir())
			cfg.LeaderPin = []string{hex.EncodeToString(first), tc.key}
			cfg.LeaderPinHeight = 3
			application, err := app.NewApplication(cfg)
			require.Error(t, err)
			require.Nil(t, application)
		})
	}
}

func TestApplicationLeaderScheduleUnsetIsZero(t *testing.T) {
	for _, tc := range []struct {
		name   string
		pin    []string
		height int64
	}{
		{name: "unset"},
		{name: "empty", pin: []string{}},
		{name: "height without pin", height: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := app.DefaultConfig(t.TempDir())
			cfg.LeaderPin = tc.pin
			cfg.LeaderPinHeight = tc.height
			application, err := app.NewApplication(cfg)
			require.NoError(t, err)
			var source types.LeaderSource = application
			for _, height := range []int64{1, 3, 5} {
				activation, leaders, err := source.LeaderSchedule(height)
				require.NoError(t, err)
				require.Zero(t, activation)
				require.Nil(t, leaders)
			}
		})
	}
}
