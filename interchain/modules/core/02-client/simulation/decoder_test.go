package simulation_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/stretchr/testify/require"

	core "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/simulation"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	commitmenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/23-commitment/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	ibckeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/keeper"
	ibctmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/07-tendermint/types"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp"
)

func TestDecodeStore(t *testing.T) {
	app := simapp.Setup(false)
	clientID := "clientidone"

	height := types.NewHeight(0, 10)

	clientState := &ibctmtypes.ClientState{
		FrozenHeight: height,
	}

	consState := &ibctmtypes.ConsensusState{
		Timestamp: time.Now().UTC(),
	}

	kvPairs := kv.Pairs{
		Pairs: []kv.Pair{
			{
				Key:   host.FullClientStateKey(clientID),
				Value: app.IBCKeeper.ClientKeeper.MustMarshalClientState(clientState),
			},
			{
				Key:   host.FullConsensusStateKey(clientID, height),
				Value: app.IBCKeeper.ClientKeeper.MustMarshalConsensusState(consState),
			},
			{
				Key:   []byte{0x99},
				Value: []byte{0x99},
			},
		},
	}
	tests := []struct {
		name        string
		expectedLog string
	}{
		{"ClientState", fmt.Sprintf("ClientState A: %v\nClientState B: %v", clientState, clientState)},
		{"ConsensusState", fmt.Sprintf("ConsensusState A: %v\nConsensusState B: %v", consState, consState)},
		{"other", ""},
	}

	for i, tt := range tests {
		i, tt := i, tt
		t.Run(tt.name, func(t *testing.T) {
			res, found := simulation.NewDecodeStore(app.IBCKeeper.ClientKeeper, kvPairs.Pairs[i], kvPairs.Pairs[i])
			if i == len(tests)-1 {
				require.False(t, found, string(kvPairs.Pairs[i].Key))
				require.Empty(t, res, string(kvPairs.Pairs[i].Key))
			} else {
				require.True(t, found, string(kvPairs.Pairs[i].Key))
				require.Equal(t, tt.expectedLog, res, string(kvPairs.Pairs[i].Key))
			}
		})
	}
}

func TestDecodeStoreKeeperMetadata(t *testing.T) {
	assertKeeperDecoderLifecycle(t, true)
}

type keeperDecoderFixture struct {
	keeper   *ibckeeper.Keeper
	ctxA     sdk.Context
	ctxB     sdk.Context
	peer     *ibctesting.TestChain
	clientID string
	height   types.Height
	decode   func(kv.Pair, kv.Pair) string
}

func newKeeperDecoderFixture(t *testing.T) *keeperDecoderFixture {
	t.Helper()
	coordinator := ibctesting.NewCoordinator(t, 1)
	peer := coordinator.GetChain(ibctesting.GetChainID(1))
	app := simapp.Setup(false)
	base := app.BaseApp.NewContext(false, tmproto.Header{
		ChainID: "decoder-host-3", Height: 10, Time: peer.CurrentHeader.Time.Add(time.Minute),
	})
	ctxA, _ := base.CacheContext()
	ctxB, _ := base.CacheContext()
	ctxB = ctxB.WithBlockHeight(11).WithBlockTime(ctxA.BlockTime().Add(5 * time.Second))
	f := &keeperDecoderFixture{
		keeper: app.IBCKeeper, ctxA: ctxA, ctxB: ctxB, peer: peer,
		height: peer.LastHeader.GetHeight().(types.Height),
	}
	registry := sdk.StoreDecoderRegistry{}
	core.NewAppModule(f.keeper).RegisterStoreDecoder(registry)
	f.decode = registry[host.StoreKey]
	require.NotNil(t, f.decode)
	for _, ctx := range []sdk.Context{ctxA, ctxB} {
		config := ibctesting.NewTendermintConfig()
		state := ibctmtypes.NewClientState(peer.ChainID, config.TrustLevel, config.TrustingPeriod,
			config.UnbondingPeriod, config.MaxClockDrift, f.height, commitmenttypes.GetSDKSpecs(),
			ibctesting.UpgradePath, false, false)
		clientID, err := f.keeper.ClientKeeper.CreateClient(ctx, state, peer.LastHeader.ConsensusState())
		require.NoError(t, err)
		if f.clientID == "" {
			f.clientID = clientID
		}
		require.Equal(t, f.clientID, clientID)
	}
	return f
}

func clientSnapshot(t *testing.T, f *keeperDecoderFixture, ctx sdk.Context) map[string]kv.Pair {
	t.Helper()
	store := f.keeper.ClientKeeper.ClientStore(ctx, f.clientID)
	iterator := store.Iterator(nil, nil)
	defer iterator.Close()
	pairs := make(map[string]kv.Pair)
	for ; iterator.Valid(); iterator.Next() {
		key := host.FullClientKey(f.clientID, iterator.Key())
		pairs[string(key)] = kv.Pair{Key: key, Value: bytes.Clone(iterator.Value())}
	}
	return pairs
}

func assertKeeperDecoderLifecycle(t *testing.T, update bool) {
	t.Helper()
	f := newKeeperDecoderFixture(t)
	beforeA := clientSnapshot(t, f, f.ctxA)
	beforeB := clientSnapshot(t, f, f.ctxB)
	require.Len(t, beforeA, 5)
	assertKeeperDecoderSnapshots(t, f, beforeA, beforeB, []types.Height{f.height})
	if !update {
		return
	}
	updateHeight := f.height.Increment().(types.Height)
	header := f.peer.CreateTMClientHeader(f.peer.ChainID, int64(updateHeight.RevisionHeight), f.height,
		f.peer.LastHeader.SignedHeader.Header.Time.Add(5*time.Second), f.peer.Vals, f.peer.Vals, f.peer.Signers)
	f.ctxA = f.ctxA.WithBlockHeight(12).WithBlockTime(f.ctxA.BlockTime().Add(10 * time.Second))
	f.ctxB = f.ctxB.WithBlockHeight(13).WithBlockTime(f.ctxB.BlockTime().Add(10 * time.Second))
	for _, ctx := range []sdk.Context{f.ctxA, f.ctxB} {
		require.NoError(t, f.keeper.ClientKeeper.UpdateClient(ctx, f.clientID, header))
		state, found := f.keeper.ClientKeeper.GetClientState(ctx, f.clientID)
		require.True(t, found)
		require.Equal(t, updateHeight, state.GetLatestHeight())
	}
	afterA := clientSnapshot(t, f, f.ctxA)
	afterB := clientSnapshot(t, f, f.ctxB)
	require.Len(t, afterA, 9)
	for key, pair := range beforeA {
		if key != string(host.FullClientStateKey(f.clientID)) {
			require.Equal(t, pair, afterA[key], "previous consensus state and metadata must survive update")
			require.Equal(t, beforeB[key], afterB[key])
		}
	}
	require.NotEqual(t, beforeA[string(host.FullClientStateKey(f.clientID))].Value,
		afterA[string(host.FullClientStateKey(f.clientID))].Value)
	clientKey := string(host.FullClientStateKey(f.clientID))
	oldState := f.keeper.ClientKeeper.MustUnmarshalClientState(beforeA[clientKey].Value)
	newState, found := f.keeper.ClientKeeper.GetClientState(f.ctxA, f.clientID)
	require.True(t, found)
	require.Equal(t, fmt.Sprintf("ClientState A: %v\nClientState B: %v", oldState, newState),
		f.decode(beforeA[clientKey], afterA[clientKey]))
	assertKeeperDecoderSnapshots(t, f, afterA, afterB, []types.Height{f.height, updateHeight})
}

func assertKeeperDecoderSnapshots(t *testing.T, f *keeperDecoderFixture, a, b map[string]kv.Pair, heights []types.Height) {
	t.Helper()
	require.Len(t, b, len(a))
	expected := make(map[string]string)
	clientA, found := f.keeper.ClientKeeper.GetClientState(f.ctxA, f.clientID)
	require.True(t, found)
	clientB, found := f.keeper.ClientKeeper.GetClientState(f.ctxB, f.clientID)
	require.True(t, found)
	expected[string(host.FullClientStateKey(f.clientID))] = fmt.Sprintf("ClientState A: %v\nClientState B: %v", clientA, clientB)
	for _, height := range heights {
		consA, found := f.keeper.ClientKeeper.GetClientConsensusState(f.ctxA, f.clientID, height)
		require.True(t, found)
		consB, found := f.keeper.ClientKeeper.GetClientConsensusState(f.ctxB, f.clientID, height)
		require.True(t, found)
		expected[string(host.FullConsensusStateKey(f.clientID, height))] = fmt.Sprintf("ConsensusState A: %v\nConsensusState B: %v", consA, consB)
		// Initialization metadata stays at heights 10/11 after an update; new
		// metadata records the update contexts at heights 12/13.
		processedA, processedB := f.ctxA, f.ctxB
		if len(heights) > 1 && height == f.height {
			processedA = processedA.WithBlockHeight(10).WithBlockTime(processedA.BlockTime().Add(-10 * time.Second))
			processedB = processedB.WithBlockHeight(11).WithBlockTime(processedB.BlockTime().Add(-10 * time.Second))
		}
		expected[string(host.FullClientKey(f.clientID, ibctmtypes.ProcessedTimeKey(height)))] = fmt.Sprintf(
			"ProcessedTime A: %d\nProcessedTime B: %d", processedA.BlockTime().UnixNano(), processedB.BlockTime().UnixNano())
		expected[string(host.FullClientKey(f.clientID, ibctmtypes.ProcessedHeightKey(height)))] = fmt.Sprintf(
			"ProcessedHeight A: %s\nProcessedHeight B: %s", types.GetSelfHeight(processedA), types.GetSelfHeight(processedB))
		expected[string(host.FullClientKey(f.clientID, ibctmtypes.IterationKey(height)))] = fmt.Sprintf(
			"IterationKey (%s) A: %s\nIterationKey (%s) B: %s", height, host.ConsensusStateKey(height), height, host.ConsensusStateKey(height))
	}
	require.Len(t, a, len(expected), "every keeper-produced key must be checked")
	for key, pairA := range a {
		pairB, exists := b[key]
		require.True(t, exists, "%q", key)
		want, exists := expected[key]
		require.True(t, exists, "unexpected producer key %q", key)
		t.Run(fmt.Sprintf("%x", pairA.Key), func(t *testing.T) {
			require.Equal(t, want, f.decode(pairA, pairB))
		})
	}
}
