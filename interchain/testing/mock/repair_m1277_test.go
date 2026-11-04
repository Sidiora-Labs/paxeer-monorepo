package mock_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	icacontrollerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/types"
	transfertypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/types"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	connectiontypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/03-connection/types"
	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	commitmenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/23-commitment/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	ibctypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/types"
	tmclient "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/07-tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/mock"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	capabilitytypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// Task 22.33.1277: ibc-mock-genesis-stream is proved by
// TestRepairM_1277_ClosedEmptyStream and TestRepairM_1277_SimappRestartRecovery.
// No out-of-path wiring is required. The implementation-first gate order
// forbids the embedded baseline run; no unchanged-tree test result is claimed.
// Attempt 1 did not execute either test: SDK auth/staking CLI dependencies
// failed to compile at revision 60bcfd3c4ffb47faef1f5dcd13c8da3dd1245f8c.
// Attempt 2 recorded SimappRestartRecovery failing after export and import;
// controller port inventory must be restored even when capability genesis
// has already restored the binding. ControllerGenesisPortRecovery also proves
// binding absent capabilities and repeated initialization without rebinding.

func TestRepairM_1277_ClosedEmptyStream(t *testing.T) {
	for i := 0; i < 2; i++ {
		stream := (mock.AppModule{}).ExportGenesisStream(sdk.Context{}, nil)
		require.NotNil(t, stream)
		select {
		case _, open := <-stream:
			require.False(t, open, "a stateless module must emit no genesis records")
		default:
			t.Fatal("genesis stream must already be closed when returned")
		}
	}
}

func repairM1277Bounded(t *testing.T, operation string, work func() error) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				done <- fmt.Errorf("%s panicked: %v", operation, p)
			}
		}()
		done <- work()
	}()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case err := <-done:
		require.NoError(t, err, operation)
	case <-timer.C:
		t.Fatalf("%s did not complete within five seconds", operation)
	}
}

func repairM1277Open(t *testing.T, state []byte) (*simapp.SimApp, func()) {
	t.Helper()
	app := simapp.NewSimApp(dbm.NewMemDB(), nil, true, map[int64]bool{}, t.TempDir(), 0,
		nil, simapp.MakeTestEncodingConfig(), simapp.EmptyAppOptions{})
	closed := false
	stop := func() {
		if !closed {
			require.NoError(t, app.Close())
			closed = true
		}
	}
	t.Cleanup(stop)
	if state == nil {
		var err error
		genesis := simapp.NewDefaultGenesisState(app.AppCodec())
		// DefaultGenesis returns nil for the stateless mock, which the manager
		// skips. Include its empty state to initialize the real mock port.
		genesis[mock.ModuleName] = json.RawMessage(`{}`)
		state, err = json.Marshal(genesis)
		require.NoError(t, err)
	}
	repairM1277Bounded(t, "full simapp genesis import", func() error {
		_, err := app.InitChain(t.Context(), &abci.RequestInitChain{
			ChainId: "repair-m1277-1", Time: time.Unix(100, 0).UTC(),
			ConsensusParams: simapp.DefaultConsensusParams, AppStateBytes: state,
		})
		if err != nil {
			return err
		}
		app.SetProcessProposalStateToCommit()
		_, err = app.Commit(t.Context())
		return err
	})
	return app, stop
}

func TestRepairM_1277_SimappRestartRecovery(t *testing.T) {
	source, stopSource := repairM1277Open(t, nil)
	header := tmproto.Header{ChainID: "repair-m1277-1", Height: 1, Time: time.Unix(100, 0).UTC()}
	ctx := source.NewUncachedContext(false, header)
	_, bound := source.ScopedIBCMockKeeper.GetCapability(ctx, host.PortPath(mock.PortID))
	require.True(t, bound, "source mock port must be initialized before export")
	const clientID = "07-tendermint-0"
	const connectionID = "connection-0"
	const channelID = "channel-0"
	const controllerPort = "icacontroller-repair"
	account := sdk.AccAddress(bytes.Repeat([]byte{7}, 20)).String()
	height := clienttypes.NewHeight(1, 9)
	clientState := tmclient.NewClientState("counterparty-1", tmclient.DefaultTrustLevel,
		24*time.Hour, 48*time.Hour, time.Minute, height, commitmenttypes.GetSDKSpecs(),
		[]string{"upgrade", "upgradedIBCState"}, false, false)
	source.IBCKeeper.ClientKeeper.SetClientState(ctx, clientID, clientState)
	source.IBCKeeper.ClientKeeper.SetClientConsensusState(ctx, clientID, height,
		tmclient.NewConsensusState(header.Time, commitmenttypes.NewMerkleRoot(bytes.Repeat([]byte{1}, 32)), bytes.Repeat([]byte{2}, 32)))
	source.IBCKeeper.ClientKeeper.SetNextClientSequence(ctx, 1)
	source.IBCKeeper.ConnectionKeeper.SetConnection(ctx, connectionID, connectiontypes.NewConnectionEnd(
		connectiontypes.OPEN, clientID, connectiontypes.NewCounterparty(clientID, connectionID,
			commitmenttypes.NewMerklePrefix([]byte(host.StoreKey))), []*connectiontypes.Version{connectiontypes.DefaultIBCVersion}, 17))
	source.IBCKeeper.ConnectionKeeper.SetClientConnectionPaths(ctx, clientID, []string{connectionID})
	source.IBCKeeper.ConnectionKeeper.SetNextConnectionSequence(ctx, 1)
	source.IBCKeeper.ChannelKeeper.SetChannel(ctx, mock.PortID, channelID, channeltypes.NewChannel(
		channeltypes.OPEN, channeltypes.UNORDERED, channeltypes.NewCounterparty(mock.PortID, channelID),
		[]string{connectionID}, mock.Version))
	source.IBCKeeper.ChannelKeeper.SetNextChannelSequence(ctx, 1)
	source.IBCKeeper.ChannelKeeper.SetNextSequenceSend(ctx, mock.PortID, channelID, 8)
	source.IBCKeeper.ChannelKeeper.SetNextSequenceRecv(ctx, mock.PortID, channelID, 5)
	source.IBCKeeper.ChannelKeeper.SetNextSequenceAck(ctx, mock.PortID, channelID, 4)
	packet := channeltypes.NewPacket(mock.MockPacketData, 7, mock.PortID, channelID,
		mock.PortID, channelID, clienttypes.NewHeight(1, 100), 0)
	commitment := channeltypes.CommitPacket(source.AppCodec(), packet)
	source.IBCKeeper.ChannelKeeper.SetPacketCommitment(ctx, mock.PortID, channelID, 7, commitment)
	ack := channeltypes.CommitAcknowledgement(mock.MockAcknowledgement.Acknowledgement())
	source.IBCKeeper.ChannelKeeper.SetPacketAcknowledgement(ctx, mock.PortID, channelID, 3, ack)
	source.IBCKeeper.ChannelKeeper.SetPacketReceipt(ctx, mock.PortID, channelID, 3)
	source.IBCKeeper.SetParams(ctx, ibctypes.Params{InboundEnabled: false, OutboundEnabled: true})
	trace := transfertypes.ParseDenomTrace("transfer/channel-7/uhpx")
	source.TransferKeeper.SetDenomTrace(ctx, trace)
	source.TransferKeeper.SetParams(ctx, transfertypes.NewParams(false, true))
	cap := source.ICAControllerKeeper.BindPort(ctx, controllerPort)
	require.NoError(t, source.ICAControllerKeeper.ClaimCapability(ctx, cap, host.PortPath(controllerPort)))
	source.ICAControllerKeeper.SetActiveChannelID(ctx, connectionID, controllerPort, channelID)
	source.ICAControllerKeeper.SetInterchainAccountAddress(ctx, connectionID, controllerPort, account)
	source.ICAControllerKeeper.SetParams(ctx, icacontrollertypes.NewParams(false))
	require.True(t, source.ICAControllerKeeper.IsBound(ctx, controllerPort))
	require.Equal(t, []string{controllerPort}, source.ICAControllerKeeper.GetAllPorts(ctx))
	source.ICAHostKeeper.SetActiveChannelID(ctx, connectionID, icatypes.PortID, channelID)
	source.ICAHostKeeper.SetInterchainAccountAddress(ctx, connectionID, icatypes.PortID, account)
	source.ICAHostKeeper.SetParams(ctx, icahosttypes.NewParams(false, []string{"/cosmos.bank.v1beta1.MsgSend"}))

	manager := source.GetModuleManager()
	require.Contains(t, manager.Modules, mock.ModuleName)
	stateful := []string{host.ModuleName, transfertypes.ModuleName, icatypes.ModuleName, capabilitytypes.ModuleName}
	expected := make(map[string]json.RawMessage)
	for _, name := range stateful {
		expected[name] = manager.Modules[name].ExportGenesis(ctx, source.AppCodec())
	}
	streamed := make(map[string]json.RawMessage)
	var transport []byte
	repairM1277Bounded(t, "full simapp streaming genesis export", func() error {
		if err := manager.ProcessGenesisPerModule(ctx, source.AppCodec(), func(name string, data json.RawMessage) error {
			if _, exists := streamed[name]; exists {
				return fmt.Errorf("unexpected duplicate genesis record for %s", name)
			}
			streamed[name] = append(json.RawMessage(nil), data...)
			return nil
		}); err != nil {
			return err
		}
		var err error
		transport, err = json.Marshal(streamed)
		return err
	})
	require.NotContains(t, streamed, mock.ModuleName, "the mock module is stateless")
	for _, name := range stateful {
		require.JSONEq(t, string(expected[name]), string(streamed[name]), name)
	}
	stopSource()

	recovered, _ := repairM1277Open(t, transport)
	require.NotSame(t, source, recovered)
	recoveredCtx := recovered.NewUncachedContext(false, header)
	require.True(t, recovered.ICAControllerKeeper.IsBound(recoveredCtx, controllerPort), "controller port capability must survive the restart")
	require.Equal(t, []string{controllerPort}, recovered.ICAControllerKeeper.GetAllPorts(recoveredCtx), "controller port inventory must survive the restart")
	for _, name := range stateful {
		actual := recovered.GetModuleManager().Modules[name].ExportGenesis(recoveredCtx, recovered.AppCodec())
		require.JSONEq(t, string(expected[name]), string(actual), name)
	}
	actualClient, found := recovered.IBCKeeper.ClientKeeper.GetClientState(recoveredCtx, clientID)
	require.True(t, found)
	require.Equal(t, clientState, actualClient)
	require.Equal(t, commitment, recovered.IBCKeeper.ChannelKeeper.GetPacketCommitment(recoveredCtx, mock.PortID, channelID, 7))
	actualAck, found := recovered.IBCKeeper.ChannelKeeper.GetPacketAcknowledgement(recoveredCtx, mock.PortID, channelID, 3)
	require.True(t, found)
	require.Equal(t, ack, actualAck)
	actualTrace, found := recovered.TransferKeeper.GetDenomTrace(recoveredCtx, trace.Hash())
	require.True(t, found)
	require.Equal(t, trace, actualTrace)
	actualAccount, found := recovered.ICAControllerKeeper.GetInterchainAccountAddress(recoveredCtx, connectionID, controllerPort)
	require.True(t, found)
	require.Equal(t, account, actualAccount)
	actualAccount, found = recovered.ICAHostKeeper.GetInterchainAccountAddress(recoveredCtx, connectionID, icatypes.PortID)
	require.True(t, found)
	require.Equal(t, account, actualAccount)
	_, found = recovered.ScopedIBCMockKeeper.GetCapability(recoveredCtx, host.PortPath(mock.PortID))
	require.True(t, found, "mock port capability must survive the restart")
}

func TestRepairM_1277_ControllerGenesisPortRecovery(t *testing.T) {
	app, _ := repairM1277Open(t, nil)
	ctx := app.NewUncachedContext(false, tmproto.Header{ChainID: "repair-m1277-1", Height: 1, Time: time.Unix(100, 0).UTC()})
	state := icatypes.DefaultControllerGenesis()
	state.Ports = []string{"icacontroller-recovery"}
	portID := state.Ports[0]
	require.False(t, app.ICAControllerKeeper.IsBound(ctx, portID))
	icacontrollerkeeper.InitGenesis(ctx, app.ICAControllerKeeper, state)
	require.True(t, app.ICAControllerKeeper.IsBound(ctx, portID), "genesis must bind ports whose capability is absent")
	require.Equal(t, state, icacontrollerkeeper.ExportGenesis(ctx, app.ICAControllerKeeper))
	index := app.CapabilityKeeper.GetLatestIndex(ctx)
	icacontrollerkeeper.InitGenesis(ctx, app.ICAControllerKeeper, state)
	require.Equal(t, index, app.CapabilityKeeper.GetLatestIndex(ctx), "existing bindings must not allocate another capability")
	require.Equal(t, state, icacontrollerkeeper.ExportGenesis(ctx, app.ICAControllerKeeper))
}
