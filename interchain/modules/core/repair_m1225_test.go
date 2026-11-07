package ibc_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	clientmodule "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client"
	clientcli "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/client/cli"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/exported"
	tmclient "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/07-tendermint/types"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	govcli "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/cli"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func repairM1225Chains(t *testing.T) (*ibctesting.Coordinator, *ibctesting.TestChain, *ibctesting.TestChain) {
	t.Helper()
	coordinator := ibctesting.NewCoordinator(t, 2)
	a := coordinator.GetChain(ibctesting.GetChainID(1))
	b := coordinator.GetChain(ibctesting.GetChainID(2))
	t.Cleanup(func() { require.NoError(t, a.GetSimApp().Close()) })
	t.Cleanup(func() { require.NoError(t, b.GetSimApp().Close()) })
	return coordinator, a, b
}

// Generate the documented governance command offline, then cross the actual
// transaction JSON and protobuf codecs before executing its proposal content.
func repairM1225Proposal(t *testing.T, chain *ibctesting.TestChain, subject, substitute string) *clienttypes.ClientUpdateProposal {
	t.Helper()
	var output bytes.Buffer
	address := chain.SenderAccount.GetAddress()
	// The chain's generic signing config has no IBC proposal registrations.
	// Use the application's registered codec for both wire formats instead.
	cdc, ok := chain.GetSimApp().AppCodec().(codec.ProtoCodecMarshaler)
	require.True(t, ok)
	txConfig := authtx.NewTxConfigWithHandler(cdc, chain.TxConfig.SignModeHandler())
	ctx := client.Context{}.WithCodec(chain.GetSimApp().AppCodec()).
		WithInterfaceRegistry(chain.GetSimApp().InterfaceRegistry()).WithTxConfig(txConfig).
		WithFrom(address.String()).WithFromAddress(address).WithChainID(chain.ChainID).
		WithKeyring(keyring.NewInMemory()).
		WithGenerateOnly(true).WithOffline(true).WithOutput(&output)
	cmd := govcli.NewTxCmd([]*cobra.Command{clientcli.NewCmdSubmitUpdateClientProposal()})
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"submit-proposal", "update-client", subject, substitute,
		"--title", "Recover IBC client", "--description", "Recover the subject from a compatible active substitute",
		"--deposit", "1stake"})
	require.NoError(t, cmd.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &ctx)), output.String())
	tx, err := txConfig.TxJSONDecoder()(output.Bytes())
	require.NoError(t, err, output.String())
	encoded, err := txConfig.TxEncoder()(tx)
	require.NoError(t, err)
	decoded, err := txConfig.TxDecoder()(encoded)
	require.NoError(t, err)
	require.Len(t, decoded.GetMsgs(), 1)
	msg, ok := decoded.GetMsgs()[0].(*govtypes.MsgSubmitProposal)
	require.True(t, ok)
	require.NoError(t, msg.ValidateBasic())
	require.Equal(t, address.String(), msg.Proposer)
	proposal, ok := msg.GetContent().(*clienttypes.ClientUpdateProposal)
	require.True(t, ok)
	require.Equal(t, subject, proposal.SubjectClientId)
	require.Equal(t, substitute, proposal.SubstituteClientId)
	return proposal
}

func TestRepairM_1225_GovernanceRecovery(t *testing.T) {
	for _, status := range []exported.Status{exported.Frozen, exported.Expired} {
		t.Run(string(status), func(t *testing.T) {
			coordinator, a, b := repairM1225Chains(t)
			subjectPath := ibctesting.NewPath(a, b)
			coordinator.Setup(subjectPath)
			subject := subjectPath.EndpointA.ClientID
			k := a.App.GetIBCKeeper().ClientKeeper
			original := a.GetClientState(subject).(*tmclient.ClientState)
			if status == exported.Expired {
				// Leave the subject untouched while a fresh substitute is created.
				coordinator.IncrementTimeBy(original.TrustingPeriod + time.Hour)
				coordinator.CommitBlock(a, b)
			} else {
				original.FrozenHeight = clienttypes.NewHeight(0, 1)
				k.SetClientState(a.GetContext(), subject, original)
			}
			substitutePath := ibctesting.NewPath(a, b)
			coordinator.SetupClients(substitutePath)
			require.NoError(t, substitutePath.EndpointA.UpdateClient())
			substitute := substitutePath.EndpointA.ClientID
			ctx := a.GetContext()
			require.Equal(t, status, a.GetClientState(subject).Status(ctx, k.ClientStore(ctx, subject), a.Codec))
			require.Equal(t, exported.Active, a.GetClientState(substitute).Status(ctx, k.ClientStore(ctx, substitute), a.Codec))
			// These differences are explicitly allowed by Tendermint recovery.
			compatible := *a.GetClientState(substitute).(*tmclient.ClientState)
			compatible.ChainId = "recovered-counterparty-0"
			compatible.TrustingPeriod -= time.Hour
			compatible.AllowUpdateAfterExpiry = true
			compatible.AllowUpdateAfterMisbehaviour = true
			k.SetClientState(ctx, substitute, &compatible)
			proposal := repairM1225Proposal(t, a, subject, substitute)
			handler := clientmodule.NewClientProposalHandler(k)
			before, found := k.GetClientState(ctx, subject)
			require.True(t, found)
			beforeBytes := clienttypes.MustMarshalClientState(a.Codec, before)
			for _, tc := range []struct {
				name   string
				mutate func(sdk.Context, *clienttypes.ClientUpdateProposal)
				err    error
			}{
				{"missing_subject", func(_ sdk.Context, p *clienttypes.ClientUpdateProposal) { p.SubjectClientId = "07-tendermint-999" }, clienttypes.ErrClientNotFound},
				{"missing_substitute", func(_ sdk.Context, p *clienttypes.ClientUpdateProposal) { p.SubstituteClientId = "07-tendermint-999" }, clienttypes.ErrClientNotFound},
				{"localhost_subject", func(_ sdk.Context, p *clienttypes.ClientUpdateProposal) { p.SubjectClientId = exported.Localhost }, clienttypes.ErrInvalidUpdateClientProposal},
				{"localhost_substitute", func(_ sdk.Context, p *clienttypes.ClientUpdateProposal) { p.SubstituteClientId = exported.Localhost }, clienttypes.ErrInvalidUpdateClientProposal},
				{"active_subject", func(cache sdk.Context, _ *clienttypes.ClientUpdateProposal) {
					active := *a.GetClientState(substitute).(*tmclient.ClientState)
					k.SetClientState(cache, subject, &active)
					cs, exists := k.GetClientConsensusState(ctx, substitute, active.LatestHeight)
					require.True(t, exists)
					k.SetClientConsensusState(cache, subject, active.LatestHeight, cs)
				}, clienttypes.ErrInvalidUpdateClientProposal},
				{"equal_height", func(cache sdk.Context, _ *clienttypes.ClientUpdateProposal) {
					state := *before.(*tmclient.ClientState)
					state.LatestHeight = a.GetClientState(substitute).(*tmclient.ClientState).LatestHeight
					k.SetClientState(cache, subject, &state)
				}, clienttypes.ErrInvalidHeight},
				{"frozen_substitute", func(cache sdk.Context, _ *clienttypes.ClientUpdateProposal) {
					state := *a.GetClientState(substitute).(*tmclient.ClientState)
					state.FrozenHeight = clienttypes.NewHeight(0, 1)
					k.SetClientState(cache, substitute, &state)
				}, clienttypes.ErrClientNotActive},
				{"expired_substitute", func(cache sdk.Context, _ *clienttypes.ClientUpdateProposal) {
					state := *a.GetClientState(substitute).(*tmclient.ClientState)
					state.TrustingPeriod = time.Nanosecond
					k.SetClientState(cache, substitute, &state)
				}, clienttypes.ErrClientNotActive},
				{"incompatible_parameters", func(cache sdk.Context, _ *clienttypes.ClientUpdateProposal) {
					state := *a.GetClientState(substitute).(*tmclient.ClientState)
					state.MaxClockDrift += time.Second
					k.SetClientState(cache, substitute, &state)
				}, clienttypes.ErrInvalidSubstitute},
			} {
				t.Run(tc.name, func(t *testing.T) {
					cache, _ := ctx.CacheContext()
					p := *proposal
					tc.mutate(cache, &p)
					rejectedBefore, exists := k.GetClientState(cache, subject)
					require.True(t, exists)
					rejectedBytes := clienttypes.MustMarshalClientState(a.Codec, rejectedBefore)
					require.ErrorIs(t, handler(cache, &p), tc.err)
					rejectedAfter, exists := k.GetClientState(cache, subject)
					require.True(t, exists)
					require.Equal(t, rejectedBytes, clienttypes.MustMarshalClientState(a.Codec, rejectedAfter))
					unchanged, exists := k.GetClientState(ctx, subject)
					require.True(t, exists)
					require.Equal(t, beforeBytes, clienttypes.MustMarshalClientState(a.Codec, unchanged))
				})
			}
			substituteState := a.GetClientState(substitute).(*tmclient.ClientState)
			height := substituteState.LatestHeight
			consensus, found := k.GetClientConsensusState(ctx, substitute, height)
			require.True(t, found)
			connection := subjectPath.EndpointA.GetConnection()
			require.NoError(t, handler(ctx, proposal))
			recovered, found := k.GetClientState(ctx, subject)
			require.True(t, found)
			require.Equal(t, exported.Active, recovered.Status(ctx, k.ClientStore(ctx, subject), a.Codec))
			expected := *before.(*tmclient.ClientState)
			expected.FrozenHeight = clienttypes.ZeroHeight()
			expected.LatestHeight = substituteState.LatestHeight
			expected.ChainId = substituteState.ChainId
			expected.TrustingPeriod = substituteState.TrustingPeriod
			require.Equal(t, &expected, recovered)
			require.Equal(t, connection, subjectPath.EndpointA.GetConnection())
			require.Equal(t, subject, connection.ClientId)
			copied, found := k.GetClientConsensusState(ctx, subject, height)
			require.True(t, found)
			require.Equal(t, consensus, copied)
			for _, id := range []string{subject, substitute} {
				store := k.ClientStore(ctx, id)
				processedTime, exists := tmclient.GetProcessedTime(store, height)
				require.True(t, exists)
				wantTime, exists := tmclient.GetProcessedTime(k.ClientStore(ctx, substitute), height)
				require.True(t, exists)
				require.Equal(t, wantTime, processedTime)
				processedHeight, exists := tmclient.GetProcessedHeight(store, height)
				require.True(t, exists)
				wantHeight, exists := tmclient.GetProcessedHeight(k.ClientStore(ctx, substitute), height)
				require.True(t, exists)
				require.Equal(t, wantHeight, processedHeight)
			}
		})
	}
}

func TestRepairM_1225_TimeoutDeadlines(t *testing.T) {
	for _, tc := range []struct {
		name                                                 string
		height, timestamp, expireHeight, expireTime, ordered bool
	}{
		{"height_equal_timestamp_future", true, true, true, false, false},
		{"timestamp_equal_height_future", true, true, false, true, false},
		{"height_only", true, false, true, false, false},
		{"timestamp_only", false, true, false, true, false},
		{"neither_reached", true, true, false, false, false},
		{"ordered_height_only", true, false, true, false, true},
		{"both_disabled", false, false, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			coordinator, a, b := repairM1225Chains(t)
			path := ibctesting.NewPath(a, b)
			if tc.ordered {
				path.SetChannelOrdered()
			}
			coordinator.Setup(path)
			height := clienttypes.ZeroHeight()
			if tc.height {
				height = clienttypes.NewHeight(clienttypes.ParseChainID(b.ChainID), uint64(b.App.LastBlockHeight())+1000)
				if tc.expireHeight {
					height.RevisionHeight = uint64(b.App.LastBlockHeight()) + 5
				}
			}
			deadline := coordinator.CurrentTime.Add(time.Hour)
			var timestamp uint64
			if tc.timestamp {
				timestamp = uint64(deadline.UnixNano())
			}
			packet := channeltypes.NewPacket([]byte("repair-m1225"), 1,
				path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
				path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, height, timestamp)
			k := a.App.GetIBCKeeper().ChannelKeeper
			if !tc.height && !tc.timestamp {
				require.ErrorIs(t, packet.ValidateBasic(), channeltypes.ErrInvalidPacket)
				require.ErrorIs(t, path.EndpointA.SendPacket(packet), channeltypes.ErrInvalidPacket)
				require.Empty(t, k.GetPacketCommitment(a.GetContext(), packet.SourcePort, packet.SourceChannel, packet.Sequence))
				return
			}
			require.NoError(t, packet.ValidateBasic())
			require.NoError(t, path.EndpointA.SendPacket(packet))
			if tc.expireTime {
				coordinator.IncrementTimeBy(deadline.Sub(coordinator.CurrentTime))
			}
			require.NoError(t, path.EndpointA.UpdateClient())
			if tc.expireHeight {
				for i := 0; i < 5 && path.EndpointA.GetClientState().GetLatestHeight().LT(height); i++ {
					require.NoError(t, path.EndpointA.UpdateClient())
				}
			}
			key := host.PacketReceiptKey(packet.DestinationPort, packet.DestinationChannel, packet.Sequence)
			if tc.ordered {
				key = host.NextSequenceRecvKey(packet.DestinationPort, packet.DestinationChannel)
			}
			proof, proofHeight := path.EndpointB.QueryProof(key)
			connection := path.EndpointA.GetConnection()
			proofTimestamp, err := a.App.GetIBCKeeper().ConnectionKeeper.GetTimestampAtHeight(a.GetContext(), connection, proofHeight)
			require.NoError(t, err)
			if tc.expireHeight {
				require.Equal(t, height, proofHeight)
			}
			if tc.expireTime {
				require.Equal(t, timestamp, proofTimestamp)
			}
			if tc.height && !tc.expireHeight {
				require.True(t, proofHeight.LT(height))
			}
			if tc.timestamp && !tc.expireTime {
				require.Less(t, proofTimestamp, timestamp)
			}
			// Drive the receiver with the sender's real commitment proof as well.
			// Discard its cache so the sender can still prove non-receipt below.
			sendProof, sendHeight := path.EndpointA.QueryProof(host.PacketCommitmentKey(packet.SourcePort, packet.SourceChannel, packet.Sequence))
			receiver := b.App.GetIBCKeeper().ChannelKeeper
			recvCtx, _ := b.GetContext().CacheContext()
			recvCap := b.GetChannelCapability(packet.DestinationPort, packet.DestinationChannel)
			recvErr := receiver.RecvPacket(recvCtx, recvCap, packet, sendProof, sendHeight)
			if tc.expireHeight || tc.expireTime {
				require.ErrorIs(t, recvErr, channeltypes.ErrPacketTimeout)
				_, received := receiver.GetPacketReceipt(recvCtx, packet.DestinationPort, packet.DestinationChannel, packet.Sequence)
				require.False(t, received)
				if tc.ordered {
					next, found := receiver.GetNextSequenceRecv(recvCtx, packet.DestinationPort, packet.DestinationChannel)
					require.True(t, found)
					require.Equal(t, uint64(1), next)
				}
			} else {
				require.NoError(t, recvErr)
				_, received := receiver.GetPacketReceipt(recvCtx, packet.DestinationPort, packet.DestinationChannel, packet.Sequence)
				require.True(t, received)
			}
			_, received := receiver.GetPacketReceipt(b.GetContext(), packet.DestinationPort, packet.DestinationChannel, packet.Sequence)
			require.False(t, received)
			commitment := channeltypes.CommitPacket(a.Codec, packet)
			require.Equal(t, commitment, k.GetPacketCommitment(a.GetContext(), packet.SourcePort, packet.SourceChannel, packet.Sequence))
			err = k.TimeoutPacket(a.GetContext(), packet, proof, proofHeight, 1)
			if !tc.expireHeight && !tc.expireTime {
				require.ErrorIs(t, err, channeltypes.ErrPacketTimeout)
				require.Equal(t, commitment, k.GetPacketCommitment(a.GetContext(), packet.SourcePort, packet.SourceChannel, packet.Sequence))
				return
			}
			require.NoError(t, err)
			// A deadline alone cannot replace the counterparty non-receipt proof.
			require.Error(t, k.TimeoutPacket(a.GetContext(), packet, []byte{0xff}, proofHeight, 1))
			require.Equal(t, commitment, k.GetPacketCommitment(a.GetContext(), packet.SourcePort, packet.SourceChannel, packet.Sequence))
			cap := a.GetChannelCapability(packet.SourcePort, packet.SourceChannel)
			require.NoError(t, k.TimeoutExecuted(a.GetContext(), cap, packet))
			require.Empty(t, k.GetPacketCommitment(a.GetContext(), packet.SourcePort, packet.SourceChannel, packet.Sequence))
			wantState := channeltypes.OPEN
			if tc.ordered {
				wantState = channeltypes.CLOSED
			}
			require.Equal(t, wantState, path.EndpointA.GetChannel().State)
			require.ErrorIs(t, k.TimeoutPacket(a.GetContext(), packet, proof, proofHeight, 1), channeltypes.ErrNoOpMsg)
		})
	}
}

func TestRepairM_1225_TimeoutOnClose(t *testing.T) {
	coordinator, a, b := repairM1225Chains(t)
	path := ibctesting.NewPath(a, b)
	coordinator.Setup(path)
	packet := channeltypes.NewPacket([]byte("repair-m1225-close"), 1,
		path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
		path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID,
		b.GetTimeoutHeight(), uint64(coordinator.CurrentTime.Add(time.Hour).UnixNano()))
	require.NoError(t, path.EndpointA.SendPacket(packet))
	require.NoError(t, path.EndpointA.UpdateClient())
	k := a.App.GetIBCKeeper().ChannelKeeper
	cap := a.GetChannelCapability(packet.SourcePort, packet.SourceChannel)
	key := host.PacketReceiptKey(packet.DestinationPort, packet.DestinationChannel, packet.Sequence)
	proof, proofHeight := path.EndpointB.QueryProof(key)
	openProof, openHeight := path.EndpointB.QueryProof(host.ChannelKey(packet.DestinationPort, packet.DestinationChannel))
	require.Equal(t, proofHeight, openHeight)
	require.Error(t, k.TimeoutOnClose(a.GetContext(), cap, packet, proof, openProof, proofHeight, 1))
	require.NoError(t, path.EndpointB.SetChannelClosed())
	proof, proofHeight = path.EndpointB.QueryProof(key)
	closedProof, closedHeight := path.EndpointB.QueryProof(host.ChannelKey(packet.DestinationPort, packet.DestinationChannel))
	require.Equal(t, proofHeight, closedHeight)
	proofTimestamp, err := a.App.GetIBCKeeper().ConnectionKeeper.GetTimestampAtHeight(a.GetContext(), path.EndpointA.GetConnection(), proofHeight)
	require.NoError(t, err)
	require.True(t, proofHeight.LT(packet.TimeoutHeight))
	require.Less(t, proofTimestamp, packet.TimeoutTimestamp)
	require.ErrorIs(t, k.TimeoutPacket(a.GetContext(), packet, proof, proofHeight, 1), channeltypes.ErrPacketTimeout)
	require.Error(t, k.TimeoutOnClose(a.GetContext(), cap, packet, []byte{0xff}, closedProof, proofHeight, 1))
	require.Equal(t, channeltypes.CommitPacket(a.Codec, packet), k.GetPacketCommitment(a.GetContext(), packet.SourcePort, packet.SourceChannel, packet.Sequence))
	require.NoError(t, k.TimeoutOnClose(a.GetContext(), cap, packet, proof, closedProof, proofHeight, 1))
	require.NoError(t, k.TimeoutExecuted(a.GetContext(), cap, packet))
	require.Empty(t, k.GetPacketCommitment(a.GetContext(), packet.SourcePort, packet.SourceChannel, packet.Sequence))
	// Closure and settlement are per-packet; unordered local state stays OPEN.
	require.Equal(t, channeltypes.OPEN, path.EndpointA.GetChannel().State)
}

func TestRepairM_1225_ConceptsReferences(t *testing.T) {
	data, err := os.ReadFile("spec/01_concepts.md")
	require.NoError(t, err)
	doc := string(data)
	for _, phrase := range []string{
		"`SubjectClientId`", "`SubstituteClientId`", "frozen or expired", "substitute must be active",
		"paxd tx gov submit-proposal update-client 07-tendermint-0 07-tendermint-1",
		"--title", "--description", "--deposit", "--from", "--chain-id",
		"at least one enabled deadline has been reached", "zero height or timestamp disables",
		"Both cannot be", "deadline is reached at equality", "does not require a packet",
		"TestRepairM_1225_GovernanceRecovery", "TestRepairM_1225_TimeoutDeadlines", "TestRepairM_1225_TimeoutOnClose",
	} {
		require.Contains(t, doc, phrase)
	}
	for _, match := range regexp.MustCompile(`\]\(([^)]+)\)`).FindAllStringSubmatch(doc, -1) {
		target := strings.SplitN(match[1], "#", 2)[0]
		if strings.HasPrefix(target, "https://") {
			continue
		}
		_, err := os.Stat(filepath.Join("spec", target))
		require.NoError(t, err, "broken concepts reference %s", match[1])
	}
}
