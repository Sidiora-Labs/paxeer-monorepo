package types_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	transfertypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/types"
	ibc "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	ibctypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/types"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp"
)

func TestRepairM_1218_ChannelSequenceValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		sequences []uint64
		next      uint64
		valid     bool
	}{
		{"empty starts at zero", nil, 0, true},
		{"empty may retain a counter", nil, 7, true},
		{"zero cannot be reused", []uint64{0}, 0, false},
		{"zero advances to one", []uint64{0}, 1, true},
		{"zero on multiple ports cannot be reused", []uint64{0, 0}, 0, false},
		{"zero on multiple ports advances to one", []uint64{0, 0}, 1, true},
		{"unsorted maximum cannot be reused", []uint64{8, 0, 3}, 8, false},
		{"unsorted maximum advances", []uint64{8, 0, 3}, 9, true},
		{"exhausted sequence cannot wrap", []uint64{math.MaxUint64}, 0, false},
		{"exhausted sequence cannot be reused", []uint64{math.MaxUint64}, math.MaxUint64, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gs := types.DefaultGenesisState()
			gs.NextChannelSequence = tc.next
			for i, sequence := range tc.sequences {
				port := testPort1
				if i%2 != 0 {
					port = testPort2
				}
				gs.Channels = append(gs.Channels, types.NewIdentifiedChannel(port, types.FormatChannelIdentifier(sequence), types.NewChannel(
					types.INIT, types.UNORDERED, types.NewCounterparty(testPort2, ""), []string{"connection-0"}, testChannelVersion,
				)))
			}
			if tc.valid {
				require.NoError(t, gs.Validate())
			} else {
				require.ErrorContains(t, gs.Validate(), "must be greater than maximum sequence used in channel identifier")
			}
		})
	}
}

func TestRepairM_1218_ChannelSequenceRestartRecovery(t *testing.T) {
	coordinator := ibctesting.NewCoordinator(t, 2)
	original := coordinator.GetChain(ibctesting.GetChainID(1))
	peer := coordinator.GetChain(ibctesting.GetChainID(2))
	t.Cleanup(func() { require.NoError(t, peer.App.GetBaseApp().Close()) })
	originalClosed := false
	t.Cleanup(func() {
		if !originalClosed {
			require.NoError(t, original.App.GetBaseApp().Close())
		}
	})
	path := ibctesting.NewPath(original, peer)
	path.EndpointA.ChannelConfig.PortID = ibctesting.TransferPort
	path.EndpointB.ChannelConfig.PortID = ibctesting.TransferPort
	path.EndpointA.ChannelConfig.Version = transfertypes.Version
	path.EndpointB.ChannelConfig.Version = transfertypes.Version
	coordinator.SetupConnections(path)
	require.NoError(t, path.EndpointA.ChanOpenInit())
	require.Equal(t, "channel-0", path.EndpointA.ChannelID)
	genesis := ibc.ExportGenesis(original.GetContext(), *original.App.GetIBCKeeper())
	exported := genesis.ChannelGenesis
	require.Len(t, exported.Channels, 1)
	require.Equal(t, uint64(1), exported.NextChannelSequence)
	colliding := exported
	colliding.NextChannelSequence = 0
	require.ErrorContains(t, colliding.Validate(), "next channel sequence 0 must be greater than maximum sequence used in channel identifier 0")
	require.NoError(t, exported.Validate())
	encoded, err := original.App.AppCodec().MarshalAsJSON(genesis)
	require.NoError(t, err)
	header := original.CurrentHeader
	require.NoError(t, original.App.GetBaseApp().Close())
	originalClosed = true

	recovered := simapp.Setup(false)
	t.Cleanup(func() { require.NoError(t, recovered.Close()) })
	keeper := simapp.NewTestSupport(t, recovered).IBCKeeper()
	ctx := recovered.NewContext(false, header)
	portID := path.EndpointA.ChannelConfig.PortID
	_, found := keeper.ChannelKeeper.GetChannel(ctx, portID, "channel-0")
	require.False(t, found, "the reopened component must start with a fresh store")
	var decoded ibctypes.GenesisState
	require.NoError(t, recovered.AppCodec().UnmarshalAsJSON(encoded, &decoded))
	require.NoError(t, decoded.ChannelGenesis.Validate())
	ibc.InitGenesis(ctx, *keeper, false, &decoded)
	before, found := keeper.ChannelKeeper.GetChannel(ctx, portID, "channel-0")
	require.True(t, found)
	require.Equal(t, exported.Channels, keeper.ChannelKeeper.GetAllChannels(ctx))
	require.Equal(t, uint64(1), keeper.ChannelKeeper.GetNextChannelSequence(ctx))
	_, err = recovered.GetScopedIBCKeeper().NewCapability(ctx, host.ChannelCapabilityPath(portID, "channel-0"))
	require.NoError(t, err)
	portCap, found := recovered.GetScopedIBCKeeper().GetCapability(ctx, host.PortPath(portID))
	require.True(t, found)
	counterparty := types.NewCounterparty(path.EndpointB.ChannelConfig.PortID, "")
	nextID, nextCap, err := keeper.ChannelKeeper.ChanOpenInit(
		ctx, before.Ordering, before.ConnectionHops, portID, portCap, counterparty, before.Version,
	)
	require.NoError(t, err, "allocation must not collide with the imported channel capability")
	require.NotNil(t, nextCap)
	require.Equal(t, "channel-1", nextID)
	keeper.ChannelKeeper.WriteOpenInitChannel(ctx, portID, nextID, before.Ordering, before.ConnectionHops, counterparty, before.Version)
	after, found := keeper.ChannelKeeper.GetChannel(ctx, portID, "channel-0")
	require.True(t, found)
	require.Equal(t, before, after, "allocating the next channel must not overwrite the imported channel")
	created, found := keeper.ChannelKeeper.GetChannel(ctx, portID, nextID)
	require.True(t, found)
	require.Equal(t, types.INIT, created.State)
	require.Len(t, keeper.ChannelKeeper.GetAllChannels(ctx), 2)
	require.Equal(t, uint64(2), keeper.ChannelKeeper.GetNextChannelSequence(ctx))
	for _, sequence := range exported.SendSequences {
		recoveredSequence, found := keeper.ChannelKeeper.GetNextSequenceSend(ctx, sequence.PortId, sequence.ChannelId)
		require.True(t, found)
		require.Equal(t, sequence.Sequence, recoveredSequence)
	}
	for _, sequence := range exported.RecvSequences {
		recoveredSequence, found := keeper.ChannelKeeper.GetNextSequenceRecv(ctx, sequence.PortId, sequence.ChannelId)
		require.True(t, found)
		require.Equal(t, sequence.Sequence, recoveredSequence)
	}
	for _, sequence := range exported.AckSequences {
		recoveredSequence, found := keeper.ChannelKeeper.GetNextSequenceAck(ctx, sequence.PortId, sequence.ChannelId)
		require.True(t, found)
		require.Equal(t, sequence.Sequence, recoveredSequence)
	}
}

func TestRepairM_1218_ReceiptConstructorRecovery(t *testing.T) {
	testGenesisReceiptImportReplay(t)
}
