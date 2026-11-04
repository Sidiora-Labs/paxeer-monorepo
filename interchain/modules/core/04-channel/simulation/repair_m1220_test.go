package simulation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	core "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core"
	channel "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/simulation"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
)

func TestRepairM_1220_UnorderedReceiveReceipts(t *testing.T) {
	coordinator := ibctesting.NewCoordinator(t, 2)
	chainA := coordinator.GetChain(ibctesting.GetChainID(1))
	chainB := coordinator.GetChain(ibctesting.GetChainID(2))
	t.Cleanup(func() { require.NoError(t, chainA.App.GetBaseApp().Close()) })
	t.Cleanup(func() { require.NoError(t, chainB.App.GetBaseApp().Close()) })
	coordinator.CommitNBlocks(chainA, 2)
	coordinator.CommitNBlocks(chainB, 2)
	path := ibctesting.NewPath(chainA, chainB)
	coordinator.Setup(path)
	require.Equal(t, types.UNORDERED, path.EndpointB.GetChannel().Ordering)

	packets := make([]types.Packet, 2)
	for i := range packets {
		packets[i] = types.NewPacket(ibctesting.MockPacketData, uint64(i+1),
			path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
			path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID,
			chainB.GetTimeoutHeight(), 0)
		require.NoError(t, path.EndpointA.SendPacket(packets[i]))
	}

	keeper := chainB.App.GetIBCKeeper()
	decoders := make(sdk.StoreDecoderRegistry)
	core.NewAppModule(keeper).RegisterStoreDecoder(decoders)
	decode := decoders[host.StoreKey]
	require.NotNil(t, decode)
	ctx := chainB.GetContext()
	store := ctx.KVStore(chainB.GetSimApp().GetKey(host.StoreKey))
	channelCap := chainB.GetChannelCapability(path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID)
	receipts := make([]kv.Pair, len(packets))
	for _, i := range []int{1, 0} {
		packet := packets[i]
		key := host.PacketReceiptKey(packet.DestinationPort, packet.DestinationChannel, packet.Sequence)
		before := kv.Pair{Key: key, Value: store.Get(key)}
		require.Nil(t, before.Value)
		proof, height := path.EndpointA.QueryProof(host.PacketCommitmentKey(packet.SourcePort, packet.SourceChannel, packet.Sequence))
		require.NoError(t, keeper.ChannelKeeper.RecvPacket(ctx, channelCap, packet, proof, height))
		after := kv.Pair{Key: key, Value: store.Get(key)}
		require.Equal(t, []byte{1}, after.Value)
		receipt, found := keeper.ChannelKeeper.GetPacketReceipt(ctx, packet.DestinationPort, packet.DestinationChannel, packet.Sequence)
		require.True(t, found)
		require.Equal(t, string(after.Value), receipt)
		require.Equal(t, "Receipt A: \nReceipt B: 01", decode(before, after))
		require.Equal(t, "Receipt A: 01\nReceipt B: ", decode(after, before))
		require.Equal(t, "Receipt A: 01\nReceipt B: 01", decode(after, after))
		receipts[i] = after
		require.ErrorIs(t, keeper.ChannelKeeper.RecvPacket(ctx, channelCap, packet, proof, height), types.ErrNoOpMsg)
	}
	next, found := keeper.ChannelKeeper.GetNextSequenceRecv(ctx, path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID)
	require.True(t, found)
	require.Equal(t, uint64(1), next)

	genesis := channel.ExportGenesis(ctx, keeper.ChannelKeeper)
	require.NoError(t, genesis.Validate())
	require.Len(t, genesis.Receipts, len(receipts))
	wire := chainB.App.AppCodec().MustMarshalJSON(&genesis)

	restored := simapp.Setup(false)
	t.Cleanup(func() { require.NoError(t, restored.Close()) })
	restoredCtx := restored.NewContext(false, chainB.CurrentHeader)
	var imported types.GenesisState
	restored.AppCodec().MustUnmarshalJSON(wire, &imported)
	require.NoError(t, imported.Validate())
	channel.InitGenesis(restoredCtx, restored.IBCKeeper.ChannelKeeper, imported)
	require.Equal(t, genesis, channel.ExportGenesis(restoredCtx, restored.IBCKeeper.ChannelKeeper))
	restoredDecoders := make(sdk.StoreDecoderRegistry)
	core.NewAppModule(restored.IBCKeeper).RegisterStoreDecoder(restoredDecoders)
	for i, receipt := range receipts {
		packet := packets[i]
		value, found := restored.IBCKeeper.ChannelKeeper.GetPacketReceipt(restoredCtx, packet.DestinationPort, packet.DestinationChannel, packet.Sequence)
		require.True(t, found)
		require.Equal(t, string(receipt.Value), value)
		retained := kv.Pair{Key: receipt.Key, Value: restoredCtx.KVStore(restored.GetKey(host.StoreKey)).Get(receipt.Key)}
		require.Equal(t, receipt, retained)
		require.Equal(t, "Receipt A: 01\nReceipt B: 01", restoredDecoders[host.StoreKey](receipt, retained))
		require.Equal(t, "Receipt A: \nReceipt B: 01", restoredDecoders[host.StoreKey](kv.Pair{Key: receipt.Key}, retained))
	}
}

func TestRepairM_1220_UnknownKey(t *testing.T) {
	app := simapp.Setup(false)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	unknown := kv.Pair{Key: []byte("unknown/packet/key"), Value: []byte{1}}
	decoded, found := simulation.NewDecodeStore(app.AppCodec(), unknown, unknown)
	require.False(t, found)
	require.Empty(t, decoded)
	decoders := make(sdk.StoreDecoderRegistry)
	core.NewAppModule(app.IBCKeeper).RegisterStoreDecoder(decoders)
	require.PanicsWithValue(t, "invalid ibc key prefix: unknown/packet/key", func() {
		decoders[host.StoreKey](unknown, unknown)
	})
}
