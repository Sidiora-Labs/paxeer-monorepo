package cmd

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	sequencertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil"
	genutiltypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/types"
	"github.com/stretchr/testify/require"
)

func TestAddGenesisSequencerLeaderAppendsLeader(t *testing.T) {
	first := genesisLeaderTestKey(1)
	second := genesisLeaderTestKey(2)
	added := genesisLeaderTestKey(3)
	for _, existing := range []struct {
		name    string
		leaders [][]byte
	}{
		{name: "first leader"},
		{name: "after existing leaders", leaders: [][]byte{first, second}},
	} {
		t.Run(existing.name, func(t *testing.T) {
			home, genFile, original := writeGenesisLeaderTestFile(t, existing.leaders)
			cmd := AddGenesisSequencerLeaderCmd(home)
			cmd.SetArgs([]string{"--pubkey", strings.ToUpper(hex.EncodeToString(added))})
			require.NoError(t, cmd.Execute())

			appState, genDoc, err := genutiltypes.GenesisStateFromGenFile(genFile)
			require.NoError(t, err)
			require.Equal(t, "sequencer-genesis-test", genDoc.ChainID)
			require.Equal(t, int64(8), genDoc.InitialHeight)
			require.JSONEq(t, string(original["bank"]), string(appState["bank"]))
			var before, after map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(original[sequencertypes.ModuleName], &before))
			require.NoError(t, json.Unmarshal(appState[sequencertypes.ModuleName], &after))
			var leaders [][]byte
			require.NoError(t, json.Unmarshal(after["leaders"], &leaders))
			expected := append(append([][]byte{}, existing.leaders...), added)
			require.Equal(t, expected, leaders)
			for field, value := range before {
				if field != "leaders" {
					require.JSONEq(t, string(value), string(after[field]), field)
				}
			}
			require.NoError(t, (sequencer.AppModuleBasic{}).ValidateGenesis(nil, nil, appState[sequencertypes.ModuleName]))
		})
	}

	t.Run("explicit home", func(t *testing.T) {
		home, genFile, _ := writeGenesisLeaderTestFile(t, [][]byte{first})
		cmd := AddGenesisSequencerLeaderCmd(t.TempDir())
		cmd.SetArgs([]string{"--home", home, "--pubkey", hex.EncodeToString(added)})
		require.NoError(t, cmd.Execute())
		appState, _, err := genutiltypes.GenesisStateFromGenFile(genFile)
		require.NoError(t, err)
		var state struct {
			Leaders [][]byte `json:"leaders"`
		}
		require.NoError(t, json.Unmarshal(appState[sequencertypes.ModuleName], &state))
		require.Equal(t, [][]byte{first, added}, state.Leaders)
	})
}

func TestAddGenesisSequencerLeaderRefusesDuplicate(t *testing.T) {
	first := genesisLeaderTestKey(1)
	second := genesisLeaderTestKey(2)
	for _, tc := range []struct {
		name string
		args []string
		err  string
	}{
		{name: "duplicate primary", args: []string{"--pubkey", hex.EncodeToString(first)}, err: "repeats an earlier leader"},
		{name: "duplicate later leader", args: []string{"--pubkey", strings.ToUpper(hex.EncodeToString(second))}, err: "repeats an earlier leader"},
		{name: "zero key", args: []string{"--pubkey", strings.Repeat("00", 32)}, err: "zero key"},
		{name: "short key", args: []string{"--pubkey", strings.Repeat("01", 31)}, err: "want 32"},
		{name: "long key", args: []string{"--pubkey", strings.Repeat("01", 33)}, err: "want 32"},
		{name: "invalid hex", args: []string{"--pubkey", strings.Repeat("zz", 32)}, err: "invalid consensus public key hex"},
		{name: "missing key", err: "required flag(s)"},
		{name: "empty key", args: []string{"--pubkey", ""}, err: "want 32"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, genFile, _ := writeGenesisLeaderTestFile(t, [][]byte{first, second})
			before, err := os.ReadFile(genFile)
			require.NoError(t, err)
			cmd := AddGenesisSequencerLeaderCmd(home)
			cmd.SetArgs(tc.args)
			require.ErrorContains(t, cmd.Execute(), tc.err)
			after, err := os.ReadFile(genFile)
			require.NoError(t, err)
			require.Equal(t, before, after, "refused keys must leave genesis.json unchanged")
		})
	}
}

func genesisLeaderTestKey(seed byte) []byte {
	return []byte(ed25519.NewKeyFromSeed(bytesForGenesisLeaderTest(seed)).Public().(ed25519.PublicKey))
}

func bytesForGenesisLeaderTest(value byte) []byte {
	key := make([]byte, ed25519.SeedSize)
	for i := range key {
		key[i] = value
	}
	return key
}

func writeGenesisLeaderTestFile(t *testing.T, leaders [][]byte) (string, string, map[string]json.RawMessage) {
	t.Helper()
	home := t.TempDir()
	genFile := filepath.Join(home, "config", "genesis.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(genFile), 0755))
	state := map[string]interface{}{
		"receipt_keys": keeper.ReceiptKeys{{
			KeyId: bytesForGenesisLeaderTest(4), PublicKey: genesisLeaderTestKey(5),
			FirstHeight: 1, LastHeight: 100,
		}},
		"last_seq": uint64(7), "last_seq_hash": bytesForGenesisLeaderTest(6),
		"activation_height": uint64(8),
	}
	if len(leaders) > 0 {
		state["leaders"] = leaders
	}
	encoded, err := json.Marshal(state)
	require.NoError(t, err)
	appState := map[string]json.RawMessage{
		sequencertypes.ModuleName: encoded,
		"bank":                    json.RawMessage(`{"balances":[],"supply":[]}`),
	}
	appJSON, err := json.Marshal(appState)
	require.NoError(t, err)
	doc := &tmtypes.GenesisDoc{
		ChainID: "sequencer-genesis-test", InitialHeight: 8,
		GenesisTime: time.Unix(1700000000, 0).UTC(), AppState: appJSON,
	}
	require.NoError(t, genutil.ExportGenesisFile(doc, genFile))
	return home, genFile, appState
}
