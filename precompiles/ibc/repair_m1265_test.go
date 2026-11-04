package ibc_test

import (
	"encoding/hex"
	"testing"
	"time"

	transfertypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/types"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	commitmenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/23-commitment/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	solotypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/06-solomachine/types"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/ibc"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1265_GasRealTransfer(t *testing.T) {
	TestIbcGasGolden(t)
}

// The supported solo-machine protocol authenticates committed peer state with
// real signatures; both packet consumers are the application's IBC Msg server.
func TestRepairM_1265_TransferRealPath(t *testing.T) {
	for _, method := range []string{ibc.TransferMethod, ibc.TransferWithDefaultTimeoutMethod} {
		for _, settlement := range []string{"acknowledgement", "error_acknowledgement", "timeout"} {
			t.Run(method+"/"+settlement, func(t *testing.T) {
				source := newRealIBCFixture(t)
				peer := newRealIBCFixture(t)
				sourceSigner := ibctesting.NewSolomachine(t, source.app.AppCodec(), "", "repair-1265-source", 1)
				peerSigner := ibctesting.NewSolomachine(t, peer.app.AppCodec(), "", "repair-1265-peer", 1)
				sourceSigner.Time = uint64(realIBCTime.UnixNano())
				peerSigner.Time = uint64(realIBCTime.UnixNano())
				sourceClient := source.channel(t, peerSigner.ClientState(), peerSigner.ConsensusState())
				peer.channel(t, sourceSigner.ClientState(), sourceSigner.ConsensusState())
				receiver := peer.sender.String()
				if settlement == "error_acknowledgement" {
					receiver = "invalid-bech32-receiver"
				}
				packet := source.send(t, method, receiver, 0, 1001)
				require.Equal(t, clienttypes.NewHeight(0, 1001), packet.TimeoutHeight)
				require.Equal(t, uint64(realIBCTime.Add(10*time.Minute).UnixNano()), packet.TimeoutTimestamp)
				source.reopen(t)
				source.assertSent(t, source.ctx, packet, receiver)
				require.True(t, source.app.CapabilityKeeper.IsInitialized(source.ctx))
				_, found := source.app.ScopedTransferKeeper.GetCapability(source.ctx, host.ChannelCapabilityPath("transfer", "channel-0"))
				require.True(t, found, "channel ownership must recover from persistent state")
				var ack []byte
				var proof []byte
				var proofHeight clienttypes.Height
				if settlement == "timeout" {
					peer.ctx = peer.ctx.WithBlockTime(time.Unix(0, int64(packet.TimeoutTimestamp)+1).UTC())
					peer.reopen(t)
					_, received := peer.app.IBCKeeper.ChannelKeeper.GetPacketReceipt(peer.ctx, "transfer", "channel-0", packet.Sequence)
					require.False(t, received)
					peerSigner.Time = packet.TimeoutTimestamp + 1
					// A real signed header advances the source's light client before timeout verification.
					require.NoError(t, source.app.IBCKeeper.ClientKeeper.UpdateClient(source.ctx, sourceClient, peerSigner.CreateHeader()))
					proofHeight = clienttypes.NewHeight(0, peerSigner.Sequence)
					updated, found := source.app.IBCKeeper.ClientKeeper.GetClientState(source.ctx, sourceClient)
					require.True(t, found)
					authenticated, ok := updated.(*solotypes.ClientState)
					require.True(t, ok)
					require.Equal(t, proofHeight, authenticated.GetLatestHeight())
					require.Equal(t, peerSigner.ConsensusState(), authenticated.ConsensusState)
					// The fixture must persist the authenticated consensus state at the proof sequence,
					// just as channel creation persists the initial consensus state for acknowledgements.
					source.app.IBCKeeper.ClientKeeper.SetClientConsensusState(source.ctx, sourceClient, proofHeight, authenticated.ConsensusState)
					source.reopen(t)
					consensus, found := source.app.IBCKeeper.ClientKeeper.GetClientConsensusState(source.ctx, sourceClient, proofHeight)
					require.True(t, found)
					require.Equal(t, authenticated.ConsensusState, consensus)
					require.Greater(t, consensus.GetTimestamp(), packet.TimeoutTimestamp)
					source.assertSent(t, source.ctx, packet, receiver)
					path := repair1265Path(t, host.PacketReceiptPath("transfer", "channel-0", packet.Sequence))
					signBytes, err := solotypes.PacketReceiptAbsenceSignBytes(source.app.AppCodec(), peerSigner.Sequence, peerSigner.Time, peerSigner.Diversifier, path)
					require.NoError(t, err)
					proof = repair1265Proof(t, source, peerSigner, signBytes)
					response, err := source.app.IBCKeeper.Timeout(sdk.WrapSDKContext(source.ctx), &channeltypes.MsgTimeout{
						Packet: packet, ProofUnreceived: proof, ProofHeight: proofHeight, NextSequenceRecv: 1, Signer: source.sender.String(),
					})
					require.NoError(t, err)
					require.Equal(t, channeltypes.SUCCESS, response.Result)
				} else {
					commitment := source.app.IBCKeeper.ChannelKeeper.GetPacketCommitment(source.ctx, "transfer", "channel-0", packet.Sequence)
					require.Equal(t, realIBCCommitment(packet), commitment)
					path := repair1265Path(t, host.PacketCommitmentPath("transfer", "channel-0", packet.Sequence))
					signBytes, err := solotypes.PacketCommitmentSignBytes(source.app.AppCodec(), sourceSigner.Sequence, sourceSigner.Time, sourceSigner.Diversifier, path, commitment)
					require.NoError(t, err)
					proofCommitment := repair1265Proof(t, source, sourceSigner, signBytes)
					altered := packet
					altered.Data = append(append([]byte(nil), packet.Data...), byte(' '))
					refused, _ := peer.ctx.CacheContext()
					_, err = peer.app.IBCKeeper.RecvPacket(sdk.WrapSDKContext(refused), &channeltypes.MsgRecvPacket{
						Packet: altered, ProofCommitment: proofCommitment,
						ProofHeight: clienttypes.NewHeight(0, sourceSigner.Sequence), Signer: peer.sender.String(),
					})
					require.Error(t, err, "a signed commitment must refuse altered packet data")
					_, receivedBefore := peer.app.IBCKeeper.ChannelKeeper.GetPacketReceipt(peer.ctx, "transfer", "channel-0", packet.Sequence)
					require.False(t, receivedBefore)
					peer.ctx = peer.ctx.WithEventManager(sdk.NewEventManager())
					receive, err := peer.app.IBCKeeper.RecvPacket(sdk.WrapSDKContext(peer.ctx), &channeltypes.MsgRecvPacket{
						Packet: packet, ProofCommitment: proofCommitment,
						ProofHeight: clienttypes.NewHeight(0, sourceSigner.Sequence), Signer: peer.sender.String(),
					})
					require.NoError(t, err)
					require.Equal(t, channeltypes.SUCCESS, receive.Result)
					ack = repair1265Ack(t, peer.ctx.EventManager().Events())
					var decoded channeltypes.Acknowledgement
					require.NoError(t, peer.app.AppCodec().UnmarshalAsJSON(ack, &decoded))
					require.Equal(t, settlement == "acknowledgement", decoded.Success())
					if decoded.Success() {
						require.Equal(t, []byte{1}, decoded.GetResult())
					} else {
						require.NotEmpty(t, decoded.GetError())
					}
					voucher := transfertypes.ParseDenomTrace("transfer/channel-0/uhpx").IBCDenom()
					wantVoucher := sdk.ZeroInt()
					if settlement == "acknowledgement" {
						wantVoucher = sdk.NewInt(realIBCAmount)
					}
					require.Equal(t, wantVoucher, peer.app.BankKeeper.GetBalance(peer.ctx, peer.sender, voucher).Amount)
					peer.reopen(t)
					require.Equal(t, wantVoucher, peer.app.BankKeeper.GetBalance(peer.ctx, peer.sender, voucher).Amount)
					_, received := peer.app.IBCKeeper.ChannelKeeper.GetPacketReceipt(peer.ctx, "transfer", "channel-0", packet.Sequence)
					require.True(t, received)
					stored, found := peer.app.IBCKeeper.ChannelKeeper.GetPacketAcknowledgement(peer.ctx, "transfer", "channel-0", packet.Sequence)
					require.True(t, found)
					require.Equal(t, channeltypes.CommitAcknowledgement(ack), stored)
					path = repair1265Path(t, host.PacketAcknowledgementPath("transfer", "channel-0", packet.Sequence))
					signBytes, err = solotypes.PacketAcknowledgementSignBytes(source.app.AppCodec(), peerSigner.Sequence, peerSigner.Time, peerSigner.Diversifier, path, ack)
					require.NoError(t, err)
					proof = repair1265Proof(t, source, peerSigner, signBytes)
					proofHeight = clienttypes.NewHeight(0, peerSigner.Sequence)
					refusedAck, _ := source.ctx.CacheContext()
					_, err = source.app.IBCKeeper.Acknowledgement(sdk.WrapSDKContext(refusedAck), &channeltypes.MsgAcknowledgement{
						Packet: packet, Acknowledgement: channeltypes.NewResultAcknowledgement([]byte{2}).Acknowledgement(),
						ProofAcked: proof, ProofHeight: proofHeight, Signer: source.sender.String(),
					})
					require.Error(t, err, "an acknowledgement different from the peer's signed output must be refused")
					source.assertSent(t, source.ctx, packet, receiver)
					response, err := source.app.IBCKeeper.Acknowledgement(sdk.WrapSDKContext(source.ctx), &channeltypes.MsgAcknowledgement{
						Packet: packet, Acknowledgement: ack, ProofAcked: proof, ProofHeight: proofHeight, Signer: source.sender.String(),
					})
					require.NoError(t, err)
					require.Equal(t, channeltypes.SUCCESS, response.Result)
				}
				assertSettled := func() {
					t.Helper()
					wantSender, wantEscrow := realIBCFunding, int64(0)
					if settlement == "acknowledgement" {
						wantSender -= realIBCAmount
						wantEscrow = realIBCAmount
					}
					require.Equal(t, sdk.NewInt(wantSender), source.app.BankKeeper.GetBalance(source.ctx, source.sender, "uhpx").Amount)
					require.Equal(t, sdk.NewInt(wantEscrow), source.app.BankKeeper.GetBalance(source.ctx, transfertypes.GetEscrowAddress("transfer", "channel-0"), "uhpx").Amount)
					require.Empty(t, source.app.IBCKeeper.ChannelKeeper.GetPacketCommitment(source.ctx, "transfer", "channel-0", packet.Sequence))
					next, found := source.app.IBCKeeper.ChannelKeeper.GetNextSequenceSend(source.ctx, "transfer", "channel-0")
					require.True(t, found)
					require.Equal(t, uint64(2), next)
				}
				assertSettled()
				source.reopen(t)
				assertSettled()
				// Replaying the same authenticated settlement after another restart is a no-op.
				if settlement == "timeout" {
					response, err := source.app.IBCKeeper.Timeout(sdk.WrapSDKContext(source.ctx), &channeltypes.MsgTimeout{
						Packet: packet, ProofUnreceived: proof, ProofHeight: proofHeight, NextSequenceRecv: 1, Signer: source.sender.String(),
					})
					require.NoError(t, err)
					require.Equal(t, channeltypes.NOOP, response.Result)
				} else {
					response, err := source.app.IBCKeeper.Acknowledgement(sdk.WrapSDKContext(source.ctx), &channeltypes.MsgAcknowledgement{
						Packet: packet, Acknowledgement: ack, ProofAcked: proof, ProofHeight: proofHeight, Signer: source.sender.String(),
					})
					require.NoError(t, err)
					require.Equal(t, channeltypes.NOOP, response.Result)
				}
				assertSettled()
			})
		}
	}
}

func repair1265Path(t *testing.T, key string) commitmenttypes.MerklePath {
	t.Helper()
	path, err := commitmenttypes.ApplyPrefix(commitmenttypes.NewMerklePrefix([]byte(host.StoreKey)), commitmenttypes.NewMerklePath(key))
	require.NoError(t, err)
	return path
}

func repair1265Proof(t *testing.T, f *realIBCFixture, signer *ibctesting.Solomachine, signBytes []byte) []byte {
	t.Helper()
	proof, err := f.app.AppCodec().Marshal(&solotypes.TimestampedSignatureData{
		SignatureData: signer.GenerateSignature(signBytes), Timestamp: signer.Time,
	})
	require.NoError(t, err)
	return proof
}

func repair1265Ack(t *testing.T, events sdk.Events) []byte {
	t.Helper()
	var ack []byte
	count := 0
	for _, event := range events {
		if event.Type != channeltypes.EventTypeWriteAck {
			continue
		}
		for _, attr := range event.Attributes {
			if string(attr.Key) != channeltypes.AttributeKeyAckHex {
				continue
			}
			count++
			var err error
			ack, err = hex.DecodeString(string(attr.Value))
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, count, "the real receiver must emit one acknowledgement")
	require.NotEmpty(t, ack)
	return ack
}
