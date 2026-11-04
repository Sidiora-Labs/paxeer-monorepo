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

const (
	testPort1         = "firstport"
	testPort2         = "secondport"
	testConnectionIDA = "connectionidatob"

	testChannel1 = "channel-0"
	testChannel2 = "channel-1"

	testChannelOrder   = types.ORDERED
	testChannelVersion = "1.0"
)

func TestValidateGenesis(t *testing.T) {
	counterparty1 := types.NewCounterparty(testPort1, testChannel1)
	counterparty2 := types.NewCounterparty(testPort2, testChannel2)
	testCases := []struct {
		name     string
		genState types.GenesisState
		expPass  bool
	}{
		{
			name:     "default",
			genState: types.DefaultGenesisState(),
			expPass:  true,
		},
		{
			name: "valid genesis",
			genState: types.NewGenesisState(
				[]types.IdentifiedChannel{
					types.NewIdentifiedChannel(
						testPort1, testChannel1, types.NewChannel(
							types.INIT, testChannelOrder, counterparty2, []string{testConnectionIDA}, testChannelVersion,
						),
					),
					types.NewIdentifiedChannel(
						testPort2, testChannel2, types.NewChannel(
							types.INIT, testChannelOrder, counterparty1, []string{testConnectionIDA}, testChannelVersion,
						),
					),
				},
				[]types.PacketState{
					types.NewPacketState(testPort2, testChannel2, 1, []byte("ack")),
				},
				[]types.PacketState{
					types.NewPacketState(testPort2, testChannel2, 1, []byte("")),
				},
				[]types.PacketState{
					types.NewPacketState(testPort1, testChannel1, 1, []byte("commit_hash")),
				},
				[]types.PacketSequence{
					types.NewPacketSequence(testPort1, testChannel1, 1),
				},
				[]types.PacketSequence{
					types.NewPacketSequence(testPort2, testChannel2, 1),
				},
				[]types.PacketSequence{
					types.NewPacketSequence(testPort2, testChannel2, 1),
				},
				2,
			),
			expPass: true,
		},
		{
			name: "channel zero cannot reuse sequence zero",
			genState: types.NewGenesisState(
				[]types.IdentifiedChannel{
					types.NewIdentifiedChannel(testPort1, testChannel1, types.NewChannel(
						types.INIT, testChannelOrder, counterparty2, []string{testConnectionIDA}, testChannelVersion,
					)),
				}, nil, nil, nil, nil, nil, nil, 0,
			),
			expPass: false,
		},
		{
			name: "invalid channel",
			genState: types.GenesisState{
				Channels: []types.IdentifiedChannel{
					types.NewIdentifiedChannel(
						testPort1, "(testChannel1)", types.NewChannel(
							types.INIT, testChannelOrder, counterparty2, []string{testConnectionIDA}, testChannelVersion,
						),
					),
				},
			},
			expPass: false,
		},
		{
			name: "invalid ack",
			genState: types.GenesisState{
				Acknowledgements: []types.PacketState{
					types.NewPacketState(testPort2, testChannel2, 1, nil),
				},
			},
			expPass: false,
		},
		{
			name: "invalid commitment",
			genState: types.GenesisState{
				Commitments: []types.PacketState{
					types.NewPacketState(testPort1, testChannel1, 1, nil),
				},
			},
			expPass: false,
		},
		{
			name: "invalid send seq",
			genState: types.GenesisState{
				SendSequences: []types.PacketSequence{
					types.NewPacketSequence(testPort1, testChannel1, 0),
				},
			},
			expPass: false,
		},
		{
			name: "invalid recv seq",
			genState: types.GenesisState{
				RecvSequences: []types.PacketSequence{
					types.NewPacketSequence(testPort1, "(testChannel1)", 1),
				},
			},
			expPass: false,
		},
		{
			name: "invalid recv seq 2",
			genState: types.GenesisState{
				RecvSequences: []types.PacketSequence{
					types.NewPacketSequence("(testPort1)", testChannel1, 1),
				},
			},
			expPass: false,
		},
		{
			name: "invalid ack seq",
			genState: types.GenesisState{
				AckSequences: []types.PacketSequence{
					types.NewPacketSequence(testPort1, "(testChannel1)", 1),
				},
			},
			expPass: false,
		},
		{
			name: "invalid channel identifier",
			genState: types.NewGenesisState(
				[]types.IdentifiedChannel{
					types.NewIdentifiedChannel(
						testPort1, "chan-0", types.NewChannel(
							types.INIT, testChannelOrder, counterparty2, []string{testConnectionIDA}, testChannelVersion,
						),
					),
					types.NewIdentifiedChannel(
						testPort2, testChannel2, types.NewChannel(
							types.INIT, testChannelOrder, counterparty1, []string{testConnectionIDA}, testChannelVersion,
						),
					),
				},
				[]types.PacketState{
					types.NewPacketState(testPort2, testChannel2, 1, []byte("ack")),
				},
				[]types.PacketState{
					types.NewPacketState(testPort2, testChannel2, 1, []byte("")),
				},
				[]types.PacketState{
					types.NewPacketState(testPort1, testChannel1, 1, []byte("commit_hash")),
				},
				[]types.PacketSequence{
					types.NewPacketSequence(testPort1, testChannel1, 1),
				},
				[]types.PacketSequence{
					types.NewPacketSequence(testPort2, testChannel2, 1),
				},
				[]types.PacketSequence{
					types.NewPacketSequence(testPort2, testChannel2, 1),
				},
				0,
			),
			expPass: false,
		},
		{
			name: "next channel sequence is less than maximum channel identifier sequence used",
			genState: types.NewGenesisState(
				[]types.IdentifiedChannel{
					types.NewIdentifiedChannel(
						testPort1, "channel-10", types.NewChannel(
							types.INIT, testChannelOrder, counterparty2, []string{testConnectionIDA}, testChannelVersion,
						),
					),
					types.NewIdentifiedChannel(
						testPort2, testChannel2, types.NewChannel(
							types.INIT, testChannelOrder, counterparty1, []string{testConnectionIDA}, testChannelVersion,
						),
					),
				},
				[]types.PacketState{
					types.NewPacketState(testPort2, testChannel2, 1, []byte("ack")),
				},
				[]types.PacketState{
					types.NewPacketState(testPort2, testChannel2, 1, []byte("")),
				},
				[]types.PacketState{
					types.NewPacketState(testPort1, testChannel1, 1, []byte("commit_hash")),
				},
				[]types.PacketSequence{
					types.NewPacketSequence(testPort1, testChannel1, 1),
				},
				[]types.PacketSequence{
					types.NewPacketSequence(testPort2, testChannel2, 1),
				},
				[]types.PacketSequence{
					types.NewPacketSequence(testPort2, testChannel2, 1),
				},
				0,
			),
			expPass: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		if tc.name == "valid genesis" {
			require.Equal(t, []types.PacketState{
				types.NewPacketState(testPort2, testChannel2, 1, []byte{}),
			}, tc.genState.Receipts, "construction must retain the supplied receipt")
		}
		err := tc.genState.Validate()
		if tc.expPass {
			require.NoError(t, err, tc.name)
		} else {
			require.Error(t, err, tc.name)
		}
	}
}

func testGenesisReceiptImportReplay(t *testing.T) {
	t.Helper()
	coordinator := ibctesting.NewCoordinator(t, 2)
	sender := coordinator.GetChain(ibctesting.GetChainID(1))
	receiver := coordinator.GetChain(ibctesting.GetChainID(2))
	t.Cleanup(func() { require.NoError(t, sender.App.GetBaseApp().Close()) })
	receiverClosed := false
	t.Cleanup(func() {
		if !receiverClosed {
			require.NoError(t, receiver.App.GetBaseApp().Close())
		}
	})
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
			[]byte("receipt constructor recovery"), uint64(i+1),
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
	cap := receiver.GetChannelCapability(packets[0].DestinationPort, packets[0].DestinationChannel)
	for _, i := range []int{2, 0} {
		require.NoError(t, keeper.ChannelKeeper.RecvPacket(ctx, cap, packets[i], proofs[i], heights[i]))
		receipt, found := keeper.ChannelKeeper.GetPacketReceipt(ctx, packets[i].DestinationPort, packets[i].DestinationChannel, packets[i].Sequence)
		require.True(t, found, "the receipt must be produced by a successful unordered receive")
		require.Equal(t, string([]byte{1}), receipt)
		require.NoError(t, keeper.ChannelKeeper.WriteAcknowledgement(ctx, cap, packets[i], types.NewResultAcknowledgement([]byte{1})))
	}
	outbound := types.NewPacket(
		[]byte("outbound receipt recovery"), 1,
		path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID,
		path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
		sender.GetTimeoutHeight(), 0,
	)
	require.NoError(t, keeper.ChannelKeeper.SendPacket(ctx, cap, outbound))
	genesis := ibc.ExportGenesis(ctx, *keeper)
	exported := genesis.ChannelGenesis
	require.Len(t, exported.Receipts, 2)
	genesis.ChannelGenesis = types.NewGenesisState(
		exported.Channels, exported.Acknowledgements, exported.Receipts, exported.Commitments,
		exported.SendSequences, exported.RecvSequences, exported.AckSequences, exported.NextChannelSequence,
	)
	require.Equal(t, exported.Receipts, genesis.ChannelGenesis.Receipts)
	require.Equal(t, exported, genesis.ChannelGenesis)
	require.NoError(t, genesis.ChannelGenesis.Validate())
	encoded, err := receiver.App.AppCodec().MarshalAsJSON(genesis)
	require.NoError(t, err)
	header := receiver.CurrentHeader
	require.NoError(t, receiver.App.GetBaseApp().Close())
	receiverClosed = true

	recovered := simapp.Setup(false)
	t.Cleanup(func() { require.NoError(t, recovered.Close()) })
	recoveredKeeper := simapp.NewTestSupport(t, recovered).IBCKeeper()
	recoveredCtx := recovered.NewContext(false, header)
	require.Empty(t, recoveredKeeper.ChannelKeeper.GetAllPacketReceipts(recoveredCtx), "import must use a fresh receiver store")
	var decoded ibctypes.GenesisState
	require.NoError(t, recovered.AppCodec().UnmarshalAsJSON(encoded, &decoded))
	require.Equal(t, exported.Receipts, decoded.ChannelGenesis.Receipts)
	require.Equal(t, exported, decoded.ChannelGenesis)
	require.NoError(t, decoded.ChannelGenesis.Validate())
	ibc.InitGenesis(recoveredCtx, *recoveredKeeper, false, &decoded)
	require.Equal(t, exported, ibc.ExportGenesis(recoveredCtx, *recoveredKeeper).ChannelGenesis)
	recoveredCap, err := recovered.GetScopedIBCKeeper().NewCapability(recoveredCtx, host.ChannelCapabilityPath(
		packets[0].DestinationPort, packets[0].DestinationChannel,
	))
	require.NoError(t, err)
	for _, i := range []int{0, 2} {
		err := recoveredKeeper.ChannelKeeper.RecvPacket(recoveredCtx, recoveredCap, packets[i], proofs[i], heights[i])
		require.ErrorIs(t, err, types.ErrNoOpMsg, "recovered receipts must prevent replay")
	}
	require.Equal(t, exported, ibc.ExportGenesis(recoveredCtx, *recoveredKeeper).ChannelGenesis, "replay must leave channel state unchanged")
	_, found := recoveredKeeper.ChannelKeeper.GetPacketReceipt(recoveredCtx, packets[1].DestinationPort, packets[1].DestinationChannel, packets[1].Sequence)
	require.False(t, found)
	require.NoError(t, recoveredKeeper.ChannelKeeper.RecvPacket(recoveredCtx, recoveredCap, packets[1], proofs[1], heights[1]))
	_, found = recoveredKeeper.ChannelKeeper.GetPacketReceipt(recoveredCtx, packets[1].DestinationPort, packets[1].DestinationChannel, packets[1].Sequence)
	require.True(t, found, "recovery must still accept an unreceived packet")
}
