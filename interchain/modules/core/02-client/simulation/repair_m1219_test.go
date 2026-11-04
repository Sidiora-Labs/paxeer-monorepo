package simulation_test

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/simulation"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	ibctmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/07-tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
)

func TestRepairM_1219_ClientMetadataDecoder(t *testing.T) {
	assertKeeperDecoderLifecycle(t, false)
}

func TestRepairM_1219_MetadataDecoderFixtureGap(t *testing.T) {
	assertKeeperDecoderLifecycle(t, true)
}

func TestRepairM_1219_ExactKeyShapes(t *testing.T) {
	f := newKeeperDecoderFixture(t)
	pairs := clientSnapshot(t, f, f.ctxA)
	consensus := pairs[string(host.FullConsensusStateKey(f.clientID, f.height))]
	iteration := host.FullClientKey(f.clientID, ibctmtypes.IterationKey(f.height))
	unknown := [][]byte{
		{0x99},
		[]byte("clientsExtra/07-tendermint-0/clientState"),
		[]byte("clients//clientState"),
		[]byte("clients/07-tendermint-0/notclientState"),
		[]byte("clients/07-tendermint-0/nested/clientState"),
		[]byte("clients/07-tendermint-0/consensusStatesExtra/0-1"),
		[]byte("clients/07-tendermint-0/nested/consensusStates/0-1"),
		[]byte("clients/07-tendermint-0/consensusStates/not-a-height"),
		[]byte("clients/07-tendermint-0/consensusStates/00-1"),
		[]byte("clients/07-tendermint-0/consensusStates/0-1/unknown"),
		[]byte("clients/07-tendermint-0/consensusStates/0-1/processedTime/extra"),
		[]byte("clients/07-tendermint-0/consensusStates/0-1/processedHeight/extra"),
		bytes.Clone(iteration[:len(iteration)-1]),
		append(bytes.Clone(iteration), 0),
	}
	for _, key := range unknown {
		t.Run(fmt.Sprintf("%x", key), func(t *testing.T) {
			pair := kv.Pair{Key: key, Value: consensus.Value}
			decoded, found := simulation.NewDecodeStore(f.keeper.ClientKeeper, pair, pair)
			require.False(t, found)
			require.Empty(t, decoded)
			require.Panics(t, func() { f.decode(pair, pair) }, "aggregate decoder must still reject unknown keys")
		})
	}
}

func TestRepairM_1219_MetadataEncodings(t *testing.T) {
	f := newKeeperDecoderFixture(t)
	// Both height components contain '/' in binary, so the iteration key must
	// remain a binary suffix after the textual client prefix is removed.
	height := types.NewHeight(47, 47)
	storeA := f.keeper.ClientKeeper.ClientStore(f.ctxA, f.clientID)
	storeB := f.keeper.ClientKeeper.ClientStore(f.ctxB, f.clientID)
	ibctmtypes.SetProcessedTime(storeA, height, 0)
	ibctmtypes.SetProcessedTime(storeB, height, math.MaxUint64)
	processedA := types.NewHeight(0, 1)
	processedB := types.NewHeight(math.MaxUint64, math.MaxUint64)
	ibctmtypes.SetProcessedHeight(storeA, height, processedA)
	ibctmtypes.SetProcessedHeight(storeB, height, processedB)
	ibctmtypes.SetIterationKey(storeA, height)
	ibctmtypes.SetIterationKey(storeB, height)
	cases := []struct {
		key  []byte
		want string
	}{
		{ibctmtypes.ProcessedTimeKey(height), fmt.Sprintf("ProcessedTime A: 0\nProcessedTime B: %d", uint64(math.MaxUint64))},
		{ibctmtypes.ProcessedHeightKey(height), fmt.Sprintf("ProcessedHeight A: %s\nProcessedHeight B: %s", processedA, processedB)},
		{ibctmtypes.IterationKey(height), fmt.Sprintf("IterationKey (%s) A: %s\nIterationKey (%s) B: %s", height, host.ConsensusStateKey(height), height, host.ConsensusStateKey(height))},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%x", tc.key), func(t *testing.T) {
			key := host.FullClientKey(f.clientID, tc.key)
			a := kv.Pair{Key: key, Value: storeA.Get(tc.key)}
			b := kv.Pair{Key: key, Value: storeB.Get(tc.key)}
			require.Equal(t, tc.want, f.decode(a, b))
		})
	}
	for _, key := range [][]byte{ibctmtypes.ProcessedTimeKey(height), ibctmtypes.ProcessedHeightKey(height)} {
		valid := kv.Pair{Key: host.FullClientKey(f.clientID, key), Value: storeA.Get(key)}
		corrupt := kv.Pair{Key: valid.Key, Value: []byte{0xff}}
		require.Panics(t, func() { f.decode(corrupt, valid) })
		require.Panics(t, func() { f.decode(valid, corrupt) })
	}
}
