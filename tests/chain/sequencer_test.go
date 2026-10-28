package tests

import (
	"crypto/ed25519"
	"encoding/json"
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	sequencertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/processblock"
	"github.com/stretchr/testify/require"
)

func TestChainUnmergedIsNotSequenced(t *testing.T) {
	app := processblock.NewTestApp(t)
	processblock.CommonPreset(app)

	activation, err := app.SequencerKeeper.ActivationHeight(app.Ctx())
	require.NoError(t, err)
	require.Zero(t, activation)
	for height := int64(1); height <= 6; height++ {
		require.Empty(t, app.RunBlock(nil))
		require.Equal(t, height, app.Ctx().BlockHeight())
		activation, err = app.SequencerKeeper.ActivationHeight(app.Ctx())
		require.NoError(t, err)
		require.Zero(t, activation, "activation after block %d", height)
		for _, queriedHeight := range []int64{0, height, height + 1, math.MaxInt64} {
			sequenced, err := app.SequencerKeeper.Sequenced(app.Ctx(), queriedHeight)
			require.NoError(t, err)
			require.False(t, sequenced, "height %d after block %d", queriedHeight, height)
		}
	}
}

func TestChainSequencerGenesisExportKeepsActivation(t *testing.T) {
	leaderSeed := [ed25519.SeedSize]byte{1}
	receiptSeed := [ed25519.SeedSize]byte{2}
	leader := ed25519.NewKeyFromSeed(leaderSeed[:]).Public().(ed25519.PublicKey)
	receiptKey := ed25519.NewKeyFromSeed(receiptSeed[:]).Public().(ed25519.PublicKey)
	keyID := [sequencertypes.KeyIDLength]byte{3}
	type sequencerGenesis struct {
		Leaders          [][]byte           `json:"leaders"`
		ReceiptKeys      keeper.ReceiptKeys `json:"receipt_keys"`
		ActivationHeight uint64             `json:"activation_height"`
	}
	genesis := sequencerGenesis{
		Leaders: [][]byte{leader},
		ReceiptKeys: keeper.ReceiptKeys{{
			KeyId:       keyID[:],
			PublicKey:   receiptKey,
			FirstHeight: 5,
			LastHeight:  100,
		}},
		ActivationHeight: 5,
	}
	section, err := json.Marshal(genesis)
	require.NoError(t, err)
	app := processblock.NewTestAppWithGenesis(t, map[string]json.RawMessage{
		sequencertypes.ModuleName: section,
	})
	processblock.CommonPreset(app)
	for height := int64(1); height <= 3; height++ {
		require.Empty(t, app.RunBlock(nil))
		require.Equal(t, height, app.Ctx().BlockHeight())
	}
	activation, err := app.SequencerKeeper.ActivationHeight(app.Ctx())
	require.NoError(t, err)
	require.Equal(t, int64(5), activation)

	exported, err := app.ExportAppStateAndValidators(false, nil)
	require.NoError(t, err)
	var state map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(exported.AppState, &state))
	require.Contains(t, state, sequencertypes.ModuleName)
	exportedSection := state[sequencertypes.ModuleName]
	var exportedGenesis sequencerGenesis
	require.NoError(t, json.Unmarshal(exportedSection, &exportedGenesis))
	require.Equal(t, genesis, exportedGenesis)

	restored := processblock.NewTestAppWithGenesis(t, map[string]json.RawMessage{
		sequencertypes.ModuleName: exportedSection,
	})
	for _, chain := range []*processblock.App{app, restored} {
		activation, err := chain.SequencerKeeper.ActivationHeight(chain.Ctx())
		require.NoError(t, err)
		require.Equal(t, int64(5), activation)
		for _, height := range []int64{0, 1, 4, 5, 6} {
			sequenced, err := chain.SequencerKeeper.Sequenced(chain.Ctx(), height)
			require.NoError(t, err)
			require.Equal(t, height >= 5, sequenced, "height %d", height)
		}
	}
}
