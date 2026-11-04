package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	channel "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	capabilitykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
)

func TestRepairM_1222_ReceiveSequenceExportImport(t *testing.T) {
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	newKeeper := func() (sdk.Context, keeper.Keeper) {
		key := sdk.NewKVStoreKey("repair_m1222")
		tkey := sdk.NewTransientStoreKey("repair_m1222_transient")
		ctx := testutil.DefaultContext(key, tkey)
		subspace := paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), key, tkey, "channel")
		k := keeper.NewKeeper(cdc, key, subspace, nil, nil, nil, capabilitykeeper.ScopedKeeper{})
		channel.InitGenesis(ctx, k, types.DefaultGenesisState())
		return ctx, k
	}
	ctx, source := newKeeper()
	source.SetChannel(ctx, "transfer", "channel-0", types.NewChannel(
		types.OPEN, types.UNORDERED, types.NewCounterparty("transfer", "channel-0"), []string{"connection-0"}, "1",
	))
	source.SetNextChannelSequence(ctx, 1)
	source.SetPacketAcknowledgement(ctx, "transfer", "channel-0", 1, []byte("acknowledgement-hash"))
	source.SetPacketCommitment(ctx, "transfer", "channel-0", 1, []byte("packet-commitment"))
	source.SetPacketReceipt(ctx, "transfer", "channel-0", 1)
	counters := []struct {
		portID    string
		channelID string
		send      uint64
		recv      uint64
		ack       uint64
	}{
		{"transfer", "channel-0", 10, 20, 30},
		{"transfer", "channel-1", 1<<53 + 1, 1<<53 + 2, 1<<53 + 3},
		{"other-port", "channel-0", ^uint64(0) - 2, ^uint64(0) - 1, ^uint64(0)},
	}
	assertCounters := func(ctx sdk.Context, k keeper.Keeper) {
		t.Helper()
		for _, counter := range counters {
			send, found := k.GetNextSequenceSend(ctx, counter.portID, counter.channelID)
			require.True(t, found, "send %s/%s", counter.portID, counter.channelID)
			require.Equal(t, counter.send, send)
			recv, found := k.GetNextSequenceRecv(ctx, counter.portID, counter.channelID)
			require.True(t, found, "receive %s/%s", counter.portID, counter.channelID)
			require.Equal(t, counter.recv, recv)
			ack, found := k.GetNextSequenceAck(ctx, counter.portID, counter.channelID)
			require.True(t, found, "acknowledgement %s/%s", counter.portID, counter.channelID)
			require.Equal(t, counter.ack, ack)
		}
	}
	var sendSequences, recvSequences, ackSequences []types.PacketSequence
	for _, counter := range counters {
		source.SetNextSequenceSend(ctx, counter.portID, counter.channelID, counter.send)
		source.SetNextSequenceRecv(ctx, counter.portID, counter.channelID, counter.recv)
		source.SetNextSequenceAck(ctx, counter.portID, counter.channelID, counter.ack)
		sendSequences = append(sendSequences, types.NewPacketSequence(counter.portID, counter.channelID, counter.send))
		recvSequences = append(recvSequences, types.NewPacketSequence(counter.portID, counter.channelID, counter.recv))
		ackSequences = append(ackSequences, types.NewPacketSequence(counter.portID, counter.channelID, counter.ack))
	}
	assertCounters(ctx, source)

	exported := channel.ExportGenesis(ctx, source)
	require.NoError(t, exported.Validate())
	require.ElementsMatch(t, sendSequences, exported.SendSequences)
	require.ElementsMatch(t, recvSequences, exported.RecvSequences)
	require.ElementsMatch(t, ackSequences, exported.AckSequences)
	encoded, err := cdc.MarshalAsJSON(&exported)
	require.NoError(t, err)
	var decoded types.GenesisState
	require.NoError(t, cdc.UnmarshalAsJSON(encoded, &decoded))
	require.Equal(t, exported, decoded)

	freshCtx, restored := newKeeper()
	for _, counter := range counters {
		send, found := restored.GetNextSequenceSend(freshCtx, counter.portID, counter.channelID)
		require.False(t, found)
		require.Zero(t, send)
		recv, found := restored.GetNextSequenceRecv(freshCtx, counter.portID, counter.channelID)
		require.False(t, found)
		require.Zero(t, recv)
		ack, found := restored.GetNextSequenceAck(freshCtx, counter.portID, counter.channelID)
		require.False(t, found)
		require.Zero(t, ack)
	}
	channel.InitGenesis(freshCtx, restored, decoded)
	assertCounters(freshCtx, restored)
	require.Equal(t, exported, channel.ExportGenesis(freshCtx, restored))

	for _, counter := range counters {
		source.SetNextSequenceSend(ctx, counter.portID, counter.channelID, 41)
		source.SetNextSequenceRecv(ctx, counter.portID, counter.channelID, 42)
		source.SetNextSequenceAck(ctx, counter.portID, counter.channelID, 43)
	}
	assertCounters(freshCtx, restored)
}
