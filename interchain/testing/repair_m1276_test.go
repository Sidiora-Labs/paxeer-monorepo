package ibctesting_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	icatypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/types"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	connectiontypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/03-connection/types"
	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/mock"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp/helpers"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	capabilitytypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability/types"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1276_ReadmeIdentifiers(t *testing.T) {
	coord := ibctesting.NewCoordinator(t, 2)
	chainA := coord.GetChain(ibctesting.GetChainID(1))
	chainB := coord.GetChain(ibctesting.GetChainID(2))
	path := ibctesting.NewPath(chainA, chainB)
	coord.Setup(path)

	require.Equal(t, "07-tendermint-0", path.EndpointA.ClientID)
	require.Equal(t, "connection-0", path.EndpointA.ConnectionID)
	require.Equal(t, "channel-0", path.EndpointA.ChannelID)
	for _, endpoint := range []*ibctesting.Endpoint{path.EndpointA, path.EndpointB} {
		require.Equal(t, connectiontypes.OPEN, endpoint.GetConnection().State)
		require.Equal(t, channeltypes.OPEN, endpoint.GetChannel().State)
		require.Equal(t, endpoint.ClientID, endpoint.GetConnection().ClientId)
		require.Equal(t, []string{endpoint.ConnectionID}, endpoint.GetChannel().ConnectionHops)
	}

	packet := channeltypes.NewPacket(
		mock.MockPacketData, 1,
		path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
		path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID,
		chainB.GetTimeoutHeight(), 0,
	)
	require.NoError(t, path.EndpointA.SendPacket(packet))
	require.NoError(t, path.RelayPacket(packet))
	require.Empty(t, chainA.App.GetIBCKeeper().ChannelKeeper.GetPacketCommitment(chainA.GetContext(), packet.SourcePort, packet.SourceChannel, packet.Sequence))
	require.NotEmpty(t, chainB.GetAcknowledgement(packet))

	readme, err := os.ReadFile("README.md")
	require.NoError(t, err)
	for _, assertion := range []string{
		`require.Equal(t, "07-tendermint-0", path.EndpointA.ClientID)`,
		`require.Equal(t, "connection-0", path.EndpointA.ConnectionID)`,
		`require.Equal(t, "channel-0", path.EndpointA.ChannelID)`,
	} {
		require.Contains(t, string(readme), assertion)
	}
}

func TestRepairM_1276_ReadmeCallback(t *testing.T) {
	coord := ibctesting.NewCoordinator(t, 2)
	chainA := coord.GetChain(ibctesting.GetChainID(1))
	chainB := coord.GetChain(ibctesting.GetChainID(2))
	path := ibctesting.NewPath(chainA, chainB)
	coord.SetupConnections(path)

	// Register through the real controller keeper, which produces the channel,
	// metadata, port binding and channel capability used by the callbacks below.
	owner := chainA.SenderAccount.GetAddress().String()
	app := chainA.GetSimApp()
	require.NoError(t, app.ICAControllerKeeper.RegisterInterchainAccount(chainA.GetContext(), path.EndpointA.ConnectionID, owner))
	portID, err := icatypes.NewControllerPortID(owner)
	require.NoError(t, err)
	channelID := channeltypes.FormatChannelIdentifier(app.IBCKeeper.ChannelKeeper.GetNextChannelSequence(chainA.GetContext()) - 1)
	channel, found := app.IBCKeeper.ChannelKeeper.GetChannel(chainA.GetContext(), portID, channelID)
	require.True(t, found)
	channelCap := chainA.GetChannelCapability(portID, channelID)

	// This assignment is the README example, compiled against MockIBCApp.
	called := false
	chainA.GetSimApp().ICAAuthModule.IBCApp.OnChanOpenTry = func(
		ctx sdk.Context,
		order channeltypes.Order,
		connectionHops []string,
		portID, channelID string,
		chanCap *capabilitytypes.Capability,
		counterparty channeltypes.Counterparty,
		counterpartyVersion string,
	) (string, error) {
		called = true
		return "", fmt.Errorf("mock authentication application rejected OnChanOpenTry")
	}

	module, _, err := app.IBCKeeper.PortKeeper.LookupModuleByPort(chainA.GetContext(), portID)
	require.NoError(t, err)
	middleware, found := app.IBCKeeper.Router.GetRoute(module)
	require.True(t, found)
	version, err := middleware.OnChanOpenTry(chainA.GetContext(), channel.Ordering, channel.ConnectionHops, portID, channelID, channelCap, channel.Counterparty, channel.Version)
	require.ErrorIs(t, err, icatypes.ErrInvalidChannelFlow)
	require.Empty(t, version)
	require.False(t, called, "controller must reject open-try before invoking its authentication app")

	// The base module consumes the same callback arguments and propagates both
	// return values, proving the documented assignment is callable as well.
	version, err = app.ICAAuthModule.OnChanOpenTry(chainA.GetContext(), channel.Ordering, channel.ConnectionHops, portID, channelID, channelCap, channel.Counterparty, channel.Version)
	require.EqualError(t, err, "mock authentication application rejected OnChanOpenTry")
	require.Empty(t, version)
	require.True(t, called)

	readme, err := os.ReadFile("README.md")
	require.NoError(t, err)
	require.Contains(t, string(readme), "counterpartyVersion string,\n) (string, error)")
	require.Contains(t, string(readme), "controller rejects open-try before calling the authentication application")
}

func TestRepairM_1276_ConnectionVersion(t *testing.T) {
	for _, feature := range []string{"ORDER_ORDERED", "ORDER_UNORDERED"} {
		t.Run(feature, func(t *testing.T) {
			coord := ibctesting.NewCoordinator(t, 2)
			path := ibctesting.NewPath(coord.GetChain(ibctesting.GetChainID(1)), coord.GetChain(ibctesting.GetChainID(2)))
			path.EndpointA.ConnectionConfig.Version = connectiontypes.NewVersion("1", []string{feature})
			// The two endpoints have distinct configurations; open-try relays A's
			// proposal, then open-ack relays B's actual negotiated feature set.
			path.EndpointB.ConnectionConfig.Version = connectiontypes.NewVersion("1", []string{"ORDER_ORDERED", "ORDER_UNORDERED"})
			coord.SetupClients(path)
			require.NoError(t, path.EndpointA.ConnOpenInit())
			want := []*connectiontypes.Version{connectiontypes.NewVersion("1", []string{feature})}
			require.Equal(t, want, path.EndpointA.GetConnection().Versions)
			require.NoError(t, path.EndpointB.ConnOpenTry())
			require.Equal(t, connectiontypes.TRYOPEN, path.EndpointB.GetConnection().State)
			require.Equal(t, want, path.EndpointB.GetConnection().Versions)
			require.NoError(t, path.EndpointA.ConnOpenAck())
			require.NoError(t, path.EndpointB.ConnOpenConfirm())
			for _, endpoint := range []*ibctesting.Endpoint{path.EndpointA, path.EndpointB} {
				require.Equal(t, connectiontypes.OPEN, endpoint.GetConnection().State)
				require.Equal(t, want, endpoint.GetConnection().Versions)
			}
		})
	}

	t.Run("nil proposal uses committed defaults", func(t *testing.T) {
		coord := ibctesting.NewCoordinator(t, 2)
		path := ibctesting.NewPath(coord.GetChain(ibctesting.GetChainID(1)), coord.GetChain(ibctesting.GetChainID(2)))
		path.EndpointA.ConnectionConfig.Version = nil
		coord.SetupConnections(path)
		require.Equal(t, connectiontypes.ExportedVersionsToProto(connectiontypes.GetCompatibleVersions()), path.EndpointA.GetConnection().Versions)
		require.Equal(t, path.EndpointA.GetConnection().Versions, path.EndpointB.GetConnection().Versions)
	})

	for _, side := range []string{"A", "B"} {
		t.Run("incompatible proposal on "+side, func(t *testing.T) {
			coord := ibctesting.NewCoordinator(t, 2)
			path := ibctesting.NewPath(coord.GetChain(ibctesting.GetChainID(1)), coord.GetChain(ibctesting.GetChainID(2)))
			coord.SetupClients(path)
			endpoint := path.EndpointA
			if side == "B" {
				endpoint = path.EndpointB
			}
			endpoint.ConnectionConfig.Version = connectiontypes.NewVersion("unsupported", []string{"ORDER_ORDERED"})
			seq := endpoint.Chain.SenderAccount.GetSequence()
			err := endpoint.ConnOpenInit()
			require.ErrorIs(t, err, connectiontypes.ErrInvalidVersion)
			require.Empty(t, endpoint.ConnectionID)
			_, found := endpoint.Chain.App.GetIBCKeeper().ConnectionKeeper.GetConnection(endpoint.Chain.GetContext(), "connection-0")
			require.False(t, found, "a rejected signed transaction must not create a connection")
			require.Equal(t, seq+1, endpoint.Chain.SenderAccount.GetSequence(), "failed handler still commits the signed transaction's ante sequence")
			endpoint.ConnectionConfig.Version = connectiontypes.NewVersion("1", []string{"ORDER_ORDERED"})
			require.NoError(t, endpoint.ConnOpenInit(), "the next signed transaction uses the committed account sequence")
			require.Equal(t, "connection-0", endpoint.ConnectionID)
		})
	}
}

func TestRepairM_1276_OrderedTimeoutDestination(t *testing.T) {
	repairM1276Timeout(t, false)
}

func TestRepairM_1276_TimeoutOnCloseDestination(t *testing.T) {
	repairM1276Timeout(t, true)
}

func repairM1276Timeout(t *testing.T, onClose bool) {
	t.Helper()
	coord := ibctesting.NewCoordinator(t, 2)
	chainA := coord.GetChain(ibctesting.GetChainID(1))
	chainB := coord.GetChain(ibctesting.GetChainID(2))
	path := ibctesting.NewPath(chainA, chainB)
	path.SetChannelOrdered()
	coord.SetupConnections(path)

	// Allocate a real unrelated B channel with the same identifiers A will use.
	// Its receive sequence remains 1, while the packet destination advances to 2.
	decoy := ibctesting.NewPath(chainA, chainB)
	decoy.EndpointB.ConnectionID = path.EndpointB.ConnectionID
	decoy.SetChannelOrdered()
	require.NoError(t, decoy.EndpointB.ChanOpenInit())
	path.EndpointB.ChannelConfig.PortID = "mock-destination"
	appB := chainB.GetSimApp()
	portCap := appB.IBCKeeper.PortKeeper.BindPort(chainB.GetContext(), path.EndpointB.ChannelConfig.PortID)
	require.NoError(t, appB.ScopedIBCMockKeeper.ClaimCapability(chainB.GetContext(), portCap, host.PortPath(path.EndpointB.ChannelConfig.PortID)))
	coord.CommitBlock(chainB)
	coord.CreateChannels(path)
	require.NotEqual(t, path.EndpointA.ChannelID, path.EndpointB.ChannelID)
	require.NotEqual(t, path.EndpointA.ChannelConfig.PortID, path.EndpointB.ChannelConfig.PortID)

	first := channeltypes.NewPacket(mock.MockPacketData, 1, path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, chainB.GetTimeoutHeight(), 0)
	require.NoError(t, path.EndpointA.SendPacket(first))
	require.NoError(t, path.RelayPacket(first))
	next, found := appB.IBCKeeper.ChannelKeeper.GetNextSequenceRecv(chainB.GetContext(), first.DestinationPort, first.DestinationChannel)
	require.True(t, found)
	require.Equal(t, uint64(2), next)
	decoyNext, found := appB.IBCKeeper.ChannelKeeper.GetNextSequenceRecv(chainB.GetContext(), first.SourcePort, first.SourceChannel)
	require.True(t, found)
	require.Equal(t, uint64(1), decoyNext)

	packet := channeltypes.NewPacket(mock.MockPacketData, 2, first.SourcePort, first.SourceChannel, first.DestinationPort, first.DestinationChannel, clienttypes.ZeroHeight(), uint64(coord.CurrentTime.Add(time.Minute).UnixNano()))
	require.NoError(t, path.EndpointA.SendPacket(packet))
	require.NotEmpty(t, chainA.App.GetIBCKeeper().ChannelKeeper.GetPacketCommitment(chainA.GetContext(), packet.SourcePort, packet.SourceChannel, packet.Sequence))
	if onClose {
		require.NoError(t, path.EndpointB.ChanCloseInit())
		require.NoError(t, path.EndpointA.UpdateClient())
		require.NoError(t, path.EndpointA.TimeoutOnClose(packet))
	} else {
		coord.IncrementTimeBy(2 * time.Minute)
		require.NoError(t, path.EndpointA.UpdateClient())
		require.NoError(t, path.EndpointA.TimeoutPacket(packet))
	}
	require.Empty(t, chainA.App.GetIBCKeeper().ChannelKeeper.GetPacketCommitment(chainA.GetContext(), packet.SourcePort, packet.SourceChannel, packet.Sequence))
	require.Equal(t, channeltypes.CLOSED, path.EndpointA.GetChannel().State)
	_, found = chainA.GetSimApp().ScopedIBCMockKeeper.GetCapability(chainA.GetContext(), mock.GetMockTimeoutCanaryCapabilityName(packet))
	require.True(t, found, "real timeout handler invoked the application callback")
	decoyNext, found = appB.IBCKeeper.ChannelKeeper.GetNextSequenceRecv(chainB.GetContext(), first.SourcePort, first.SourceChannel)
	require.True(t, found)
	require.Equal(t, uint64(1), decoyNext)
}

func TestRepairM_1276_IntegrationSignedCommittedQueries(t *testing.T) {
	// Run the restored suite, including all signed CLI, legacy REST and gRPC
	// assertions, on a loopback consensus node started by network.New.
	TestIntegrationTestSuite(t)
}

func TestRepairM_1276_IntegrationProposalValidation(t *testing.T) {
	coord := ibctesting.NewCoordinator(t, 1)
	chain := coord.GetChain(ibctesting.GetChainID(1))
	solo := ibctesting.NewSolomachine(t, chain.Codec, "06-solomachine-0", "proposal", 1)
	msg, err := clienttypes.NewMsgCreateClient(solo.ClientState(), solo.ConsensusState(), chain.SenderAccount.GetAddress().String())
	require.NoError(t, err)
	seq := chain.SenderAccount.GetSequence()
	tx, err := helpers.GenTx(chain.TxConfig, []sdk.Msg{msg}, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10)), helpers.DefaultGenTxGas, chain.ChainID, []uint64{chain.SenderAccount.GetAccountNumber()}, []uint64{seq}, chain.SenderPrivKey)
	require.NoError(t, err)
	encoded, err := chain.TxConfig.TxEncoder()(tx)
	require.NoError(t, err)
	check := func(txs [][]byte, want abci.ResponseProcessProposal_ProposalStatus) {
		t.Helper()
		response, err := chain.App.ProcessProposal(t.Context(), &abci.RequestProcessProposal{Header: &chain.CurrentHeader, Txs: txs})
		require.NoError(t, err)
		require.Equal(t, want, response.Status)
		account := chain.GetSimApp().AccountKeeper.GetAccount(chain.GetContext(), chain.SenderAccount.GetAddress())
		require.Equal(t, seq, account.GetSequence(), "proposal validation must not mutate deliver state")
	}
	check([][]byte{encoded}, abci.ResponseProcessProposal_ACCEPT)
	check([][]byte{encoded, encoded}, abci.ResponseProcessProposal_REJECT)
	check([][]byte{[]byte("invalid protobuf transaction")}, abci.ResponseProcessProposal_REJECT)
	builder, err := chain.TxConfig.WrapTxBuilder(tx)
	require.NoError(t, err)
	builder.SetGasLimit(uint64(chain.GetSimApp().GetConsensusParams(chain.GetContext()).Block.MaxGas) + 1)
	oversized, err := chain.TxConfig.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	check([][]byte{oversized}, abci.ResponseProcessProposal_REJECT)
	builder.SetGasLimit(helpers.DefaultGenTxGas)
	builder.SetMemo("changed after signing")
	tampered, err := chain.TxConfig.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	check([][]byte{tampered}, abci.ResponseProcessProposal_REJECT)
	// The same genuinely signed transaction must still deliver after speculative
	// validation and rejections, with a client visible in committed query state.
	result, err := chain.App.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{Header: &chain.CurrentHeader, Txs: [][]byte{encoded}})
	require.NoError(t, err)
	require.Len(t, result.TxResults, 1)
	require.Zero(t, result.TxResults[0].Code, result.TxResults[0].Log)
	_, err = chain.App.Commit(t.Context())
	require.NoError(t, err)
	query, err := chain.App.Query(t.Context(), &abci.RequestQuery{Path: "store/ibc/key", Data: host.FullClientStateKey("06-solomachine-0"), Height: chain.App.LastBlockHeight()})
	require.NoError(t, err)
	require.Zero(t, query.Code, query.Log)
	require.NotEmpty(t, query.Value)
}
