package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	e2e "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/pkg"
)

func writeLeaderPinManifest(t *testing.T, contents string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "testnet.toml")
	require.NoError(t, os.WriteFile(file, []byte(contents), 0600))
	return file
}

func TestLoadTestnetResolvesLeaderPin(t *testing.T) {
	file := writeLeaderPinManifest(t, `
initial_height = 10
leader_pin = ["validator02", "validator01"]
leader_pin_height = 20
validators = {validator01 = 10, validator02 = 20}
[node.validator01]
[node.validator02]
`)
	manifest, err := e2e.LoadManifest(file)
	require.NoError(t, err)
	require.Equal(t, []string{"validator02", "validator01"}, manifest.LeaderPin)
	require.Equal(t, int64(20), manifest.LeaderPinHeight)

	testnet, err := e2e.LoadTestnet(file)
	require.NoError(t, err)
	require.Equal(t, int64(20), testnet.LeaderPinHeight)
	require.Len(t, testnet.LeaderPin, 2)
	reloaded, err := e2e.LoadTestnet(file)
	require.NoError(t, err)
	for i, name := range manifest.LeaderPin {
		node := testnet.LeaderPin[i]
		require.Equal(t, name, node.Name)
		require.Same(t, testnet.LookupNode(name), node)
		require.Equal(t, e2e.ModeValidator, node.Mode)
		require.Contains(t, testnet.Validators, node)
		require.Equal(t, node.PrivvalKey.Public(), reloaded.LeaderPin[i].PrivvalKey.Public())
	}
	require.NotEqual(t, testnet.LeaderPin[0].PrivvalKey.Public(), testnet.LeaderPin[1].PrivvalKey.Public())
	require.NoError(t, testnet.Validate())

	t.Run("without pin", func(t *testing.T) {
		file := writeLeaderPinManifest(t, `
initial_height = 10
[node.validator01]
`)
		testnet, err := e2e.LoadTestnet(file)
		require.NoError(t, err)
		require.Empty(t, testnet.LeaderPin)
		require.Zero(t, testnet.LeaderPinHeight)
		require.NoError(t, testnet.Validate())
	})
}

func TestTestnetValidateRefusesUnknownLeader(t *testing.T) {
	for _, tc := range []struct {
		name string
		pin  string
		mode string
	}{
		{name: "unknown", pin: `["missing"]`, mode: "validator"},
		{name: "full", pin: `["other"]`, mode: "full"},
		{name: "light", pin: `["other"]`, mode: "light"},
		{name: "seed", pin: `["other"]`, mode: "seed"},
		{name: "duplicate", pin: `["validator01", "validator01"]`, mode: "validator"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := writeLeaderPinManifest(t, fmt.Sprintf(`
leader_pin = %s
leader_pin_height = 10
[node.validator01]
[node.other]
mode = %q
`, tc.pin, tc.mode))
			testnet, err := e2e.LoadTestnet(file)
			require.Error(t, err)
			if tc.name != "unknown" {
				require.NotNil(t, testnet)
				require.Error(t, testnet.Validate())
			}
		})
	}

	for _, tc := range []struct {
		name string
		node *e2e.Node
	}{
		{name: "nil node"},
		{name: "unknown node", node: &e2e.Node{Name: "missing", Mode: e2e.ModeValidator}},
		{name: "foreign node", node: &e2e.Node{Name: "validator01", Mode: e2e.ModeValidator}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := writeLeaderPinManifest(t, `
leader_pin = ["validator01"]
leader_pin_height = 10
[node.validator01]
`)
			testnet, err := e2e.LoadTestnet(file)
			require.NoError(t, err)
			testnet.LeaderPin = []*e2e.Node{tc.node}
			require.Error(t, testnet.Validate())
		})
	}
}

func TestTestnetValidateRefusesLeaderPinBelowInitialHeight(t *testing.T) {
	for _, tc := range []struct {
		name    string
		initial int64
		height  int64
		valid   bool
	}{
		{name: "below initial", initial: 10, height: 9},
		{name: "negative", initial: 10, height: -1},
		{name: "zero", initial: 10, height: 0},
		{name: "omitted initial", initial: 0, height: 0},
		{name: "at initial", initial: 10, height: 10, valid: true},
		{name: "above initial", initial: 10, height: 11, valid: true},
		{name: "default initial", initial: 0, height: 1, valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := writeLeaderPinManifest(t, fmt.Sprintf(`
initial_height = %d
leader_pin = ["validator01"]
leader_pin_height = %d
[node.validator01]
`, tc.initial, tc.height))
			testnet, err := e2e.LoadTestnet(file)
			if tc.valid {
				require.NoError(t, err)
				require.NoError(t, testnet.Validate())
			} else {
				require.Error(t, err)
				require.NotNil(t, testnet)
				require.Error(t, testnet.Validate())
			}
		})
	}
}
