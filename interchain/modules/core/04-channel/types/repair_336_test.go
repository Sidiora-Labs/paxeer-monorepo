package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	transfertypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/types"
	ibc "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	ibctypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/types"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp"
)

func TestRepair_336_GenesisReceiptRecovery(t *testing.T) {
	coordinator := ibctesting.NewCoordinator(t, 2)
	sender := coordinator.GetChain(ibctesting.GetChainID(1))
	receiver := coordinator.GetChain(ibctesting.GetChainID(2))
	t.Cleanup(func() { require.NoError(t, sender.App.GetBaseApp().Close()) })

	path := ibctesting.NewPath(sender, receiver)
	path.EndpointA.ChannelConfig.Version = transfertypes.Version
	path.EndpointB.ChannelConfig.Version = transfertypes.Version
	coordinator.SetupConnections(path)
	coordinator.CreateTransferChannels(path)
	require.Equal(t, types.UNORDERED, path.EndpointB.GetChannel().Ordering)

	packets := make([]types.Packet, 3)
	proofs := make([][]byte, len(packets))
	heights := make([]clienttypes.Height, len(packets))
	for i := range packets {
		packets[i] = types.NewPacket(
			[]byte("receipt recovery"), uint64(i+1),
			path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
			path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID,
			receiver.GetTimeoutHeight(), 0,
		)
		require.NoError(t, path.EndpointA.SendPacket(packets[i]))
	}
	for i, packet := range packets {
		proofs[i], heights[i] = path.EndpointA.QueryProof(host.PacketCommitmentKey(
			packet.SourcePort, packet.SourceChannel, packet.Sequence,
		))
	}

	keeper := receiver.App.GetIBCKeeper()
	ctx := receiver.GetContext()
	channelCap := receiver.GetChannelCapability(packets[0].DestinationPort, packets[0].DestinationChannel)
	for _, i := range []int{2, 0} {
		require.NoError(t, keeper.ChannelKeeper.RecvPacket(ctx, channelCap, packets[i], proofs[i], heights[i]))
		_, found := keeper.ChannelKeeper.GetPacketReceipt(ctx, packets[i].DestinationPort, packets[i].DestinationChannel, packets[i].Sequence)
		require.True(t, found, "a successful unordered receive must produce a receipt")
		require.NoError(t, keeper.ChannelKeeper.WriteAcknowledgement(
			ctx, channelCap, packets[i], types.NewResultAcknowledgement([]byte{1}),
		))
	}

	outbound := types.NewPacket(
		[]byte("outbound recovery"), 1,
		path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID,
		path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
		sender.GetTimeoutHeight(), 0,
	)
	require.NoError(t, keeper.ChannelKeeper.SendPacket(ctx, channelCap, outbound))

	genesis := ibc.ExportGenesis(ctx, *keeper)
	exported := genesis.ChannelGenesis
	require.Len(t, exported.Receipts, 2)
	require.Len(t, exported.Acknowledgements, 2)
	require.Len(t, exported.Commitments, 1)
	genesis.ChannelGenesis = types.NewGenesisState(
		exported.Channels, exported.Acknowledgements, exported.Receipts, exported.Commitments,
		exported.SendSequences, exported.RecvSequences, exported.AckSequences, exported.NextChannelSequence,
	)
	require.Equal(t, exported, genesis.ChannelGenesis, "the public constructor must retain every exported field")
	require.NoError(t, genesis.ChannelGenesis.Validate())
	encoded, err := receiver.App.AppCodec().MarshalAsJSON(genesis)
	require.NoError(t, err)
	header := receiver.CurrentHeader
	require.NoError(t, receiver.App.GetBaseApp().Close())

	recovered := simapp.Setup(false)
	t.Cleanup(func() { require.NoError(t, recovered.Close()) })
	recoveredKeeper := simapp.NewTestSupport(t, recovered).IBCKeeper()
	recoveredCtx := recovered.NewContext(false, header)
	_, found := recoveredKeeper.ChannelKeeper.GetPacketReceipt(recoveredCtx, packets[0].DestinationPort, packets[0].DestinationChannel, packets[0].Sequence)
	require.False(t, found, "recovery must use a fresh store")

	var decoded ibctypes.GenesisState
	require.NoError(t, recovered.AppCodec().UnmarshalAsJSON(encoded, &decoded))
	require.Equal(t, exported, decoded.ChannelGenesis, "JSON must preserve the constructor's complete channel state")
	ibc.InitGenesis(recoveredCtx, *recoveredKeeper, false, &decoded)
	require.Equal(t, exported.Receipts, recoveredKeeper.ChannelKeeper.GetAllPacketReceipts(recoveredCtx))
	require.Equal(t, exported, ibc.ExportGenesis(recoveredCtx, *recoveredKeeper).ChannelGenesis, "import must restore the complete channel state")

	recoveredCap, err := recovered.GetScopedIBCKeeper().NewCapability(recoveredCtx, host.ChannelCapabilityPath(
		packets[0].DestinationPort, packets[0].DestinationChannel,
	))
	require.NoError(t, err)
	for _, i := range []int{0, 2} {
		receipt, found := recoveredKeeper.ChannelKeeper.GetPacketReceipt(recoveredCtx, packets[i].DestinationPort, packets[i].DestinationChannel, packets[i].Sequence)
		require.True(t, found, "import must restore each received sequence")
		require.Equal(t, string([]byte{1}), receipt)
		err := recoveredKeeper.ChannelKeeper.RecvPacket(recoveredCtx, recoveredCap, packets[i], proofs[i], heights[i])
		require.ErrorIs(t, err, types.ErrNoOpMsg, "a recovered receipt must prevent replay processing")
	}

	_, found = recoveredKeeper.ChannelKeeper.GetPacketReceipt(recoveredCtx, packets[1].DestinationPort, packets[1].DestinationChannel, packets[1].Sequence)
	require.False(t, found, "an unreceived packet must not gain a receipt during recovery")
	require.NoError(t, recoveredKeeper.ChannelKeeper.RecvPacket(recoveredCtx, recoveredCap, packets[1], proofs[1], heights[1]))
	_, found = recoveredKeeper.ChannelKeeper.GetPacketReceipt(recoveredCtx, packets[1].DestinationPort, packets[1].DestinationChannel, packets[1].Sequence)
	require.True(t, found, "the restored channel must still accept a new unordered packet")
}
