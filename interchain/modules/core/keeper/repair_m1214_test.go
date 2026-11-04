package keeper_test

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	transfertypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/types"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	connectiontypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/03-connection/types"
	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/exported"
	ibckeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/keeper"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	metrics "github.com/armon/go-metrics"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	googleproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func repairM1214RequireProtoEqual(t *testing.T, expected, actual proto.Message) {
	t.Helper()
	// Compare complete wire messages without reflecting into SDK Any caches or time.Time.
	pack := func(message proto.Message) *anypb.Any {
		bz, err := proto.Marshal(message)
		require.NoError(t, err)
		return &anypb.Any{TypeUrl: "/" + proto.MessageName(message), Value: bz}
	}
	require.True(t, googleproto.Equal(pack(expected), pack(actual)), "aggregate service must preserve the complete protobuf message")
}

func TestRepairM_1214_AggregateQueryRegistration(t *testing.T) {
	keeperType := reflect.TypeOf(ibckeeper.Keeper{})
	for i := 0; i < keeperType.NumField(); i++ {
		field := keeperType.Field(i)
		require.False(t, field.Anonymous && field.Type.Kind() == reflect.Interface, "query handlers must not inherit an uninitialized interface: %s", field.Name)
	}

	// This fixture registers the production AppModule and serves generated queries over gRPC.
	chain, conn := repair333QueryTransport(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sdkCtx := sdk.WrapSDKContext(chain.GetContext())
	consensus := chain.LastHeader.ConsensusState()
	require.NoError(t, chain.GetSimApp().UpgradeKeeper.SetUpgradedConsensusState(chain.GetContext(), chain.GetContext().BlockHeight(), clienttypes.MustMarshalConsensusState(chain.Codec, consensus)))

	services := []struct {
		name   string
		typeOf reflect.Type
		keeper interface{}
	}{
		{"ibc.core.client.v1.Query", reflect.TypeOf((*clienttypes.QueryServer)(nil)).Elem(), chain.GetSimApp().IBCKeeper.ClientKeeper},
		{"ibc.core.connection.v1.Query", reflect.TypeOf((*connectiontypes.QueryServer)(nil)).Elem(), chain.GetSimApp().IBCKeeper.ConnectionKeeper},
		{"ibc.core.channel.v1.Query", reflect.TypeOf((*channeltypes.QueryServer)(nil)).Elem(), chain.GetSimApp().IBCKeeper.ChannelKeeper},
	}
	for _, service := range services {
		for i := 0; i < service.typeOf.NumMethod(); i++ {
			method := service.typeOf.Method(i)
			t.Run(service.name+"/"+method.Name, func(t *testing.T) {
				request := reflect.New(method.Type.In(1).Elem())
				response := reflect.New(method.Type.Out(0).Elem()).Interface().(proto.Message)
				want := reflect.ValueOf(service.keeper).MethodByName(method.Name).Call([]reflect.Value{reflect.ValueOf(sdkCtx), request})
				var wantErr error
				if !want[1].IsNil() {
					wantErr = want[1].Interface().(error)
				}
				err := conn.Invoke(ctx, "/"+service.name+"/"+method.Name, request.Interface(), response)
				require.Equal(t, status.Code(wantErr), status.Code(err))
				if wantErr != nil {
					require.Equal(t, status.Convert(wantErr).Message(), status.Convert(err).Message())
					return
				}
				require.NoError(t, err)
				if upgraded, ok := response.(*clienttypes.QueryUpgradedConsensusStateResponse); ok {
					var actual exported.ConsensusState
					require.NoError(t, chain.GetSimApp().InterfaceRegistry().UnpackAny(upgraded.UpgradedConsensusState, &actual))
					repairM1214RequireProtoEqual(t, consensus, actual)
				}
				repairM1214RequireProtoEqual(t, want[0].Interface().(proto.Message), response)
			})
		}
	}

	response, err := clienttypes.NewQueryClient(conn).UpgradedConsensusState(ctx, &clienttypes.QueryUpgradedConsensusStateRequest{})
	require.NoError(t, err)
	require.True(t, clienttypes.MustPackConsensusState(consensus).Equal(response.UpgradedConsensusState))
	chain.GetSimApp().UpgradeKeeper.ClearIBCState(chain.GetContext(), chain.GetContext().BlockHeight())
	_, err = clienttypes.NewQueryClient(conn).UpgradedConsensusState(ctx, &clienttypes.QueryUpgradedConsensusStateRequest{})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestRepairM_1214_TimeoutHeightMetricContract(t *testing.T) {
	repairM1214TimeoutMetric(t, "height")
}

func TestRepairM_1214_TimeoutCloseMetricContract(t *testing.T) {
	repairM1214TimeoutMetric(t, "channel-closed")
}

func repairM1214TimeoutMetric(t *testing.T, timeoutType string) {
	t.Helper()
	coordinator := ibctesting.NewCoordinator(t, 2)
	chainA := coordinator.GetChain(ibctesting.GetChainID(1))
	chainB := coordinator.GetChain(ibctesting.GetChainID(2))
	t.Cleanup(func() {
		require.NoError(t, chainA.GetSimApp().Close())
		require.NoError(t, chainB.GetSimApp().Close())
	})
	path := ibctesting.NewPath(chainA, chainB)
	for _, endpoint := range []*ibctesting.Endpoint{path.EndpointA, path.EndpointB} {
		endpoint.ChannelConfig.PortID = transfertypes.PortID
		endpoint.ChannelConfig.Version = transfertypes.Version
	}
	coordinator.Setup(path)

	// Route the real telemetry wrapper into the same sink used by application telemetry.
	previous := metrics.Default()
	config := metrics.DefaultConfig("")
	config.EnableHostname = false
	config.EnableRuntimeMetrics = false
	sink := metrics.NewInmemSink(time.Minute, 10*time.Minute)
	_, err := metrics.NewGlobal(config, sink)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := metrics.NewGlobal(config, previous)
		require.NoError(t, err)
	})

	contract, err := os.ReadFile("../../../../spec/layerx-merge/contracts/ibc.md")
	require.NoError(t, err)
	interfaceID := "interface.15.met-ibc-interchain-modules-core-keeper-ibc-timeout-packet-" + timeoutType + "-j57"
	var contractRow string
	for _, row := range strings.Split(string(contract), "\n") {
		if strings.Contains(row, "<code>"+interfaceID+"</code> | <code>metric</code>") {
			contractRow = row
			break
		}
	}
	require.NotEmpty(t, contractRow)
	require.Contains(t, contractRow, "keys ['ibc', 'timeout', 'packet']; labels [source_port, source_channel, destination_port, destination_channel, timeout_type="+timeoutType+"]")

	amount := sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(100))
	sender := chainA.SenderAccount.GetAddress()
	initialBalance := chainA.GetSimApp().BankKeeper.GetBalance(chainA.GetContext(), sender, amount.Denom)
	selfHeight := clienttypes.GetSelfHeight(chainB.GetContext())
	timeoutHeight := clienttypes.NewHeight(selfHeight.RevisionNumber, selfHeight.RevisionHeight+10)
	result, err := chainA.SendMsgs(transfertypes.NewMsgTransfer(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, amount, sender.String(), chainB.SenderAccount.GetAddress().String(), timeoutHeight, 0))
	require.NoError(t, err)
	packet, err := ibctesting.ParsePacketFromEvents(result.GetEvents())
	require.NoError(t, err)
	require.Equal(t, timeoutHeight, packet.TimeoutHeight)
	require.Equal(t, initialBalance.Sub(amount), chainA.GetSimApp().BankKeeper.GetBalance(chainA.GetContext(), sender, amount.Denom))
	require.True(t, chainA.GetSimApp().IBCKeeper.ChannelKeeper.HasPacketCommitment(chainA.GetContext(), packet.SourcePort, packet.SourceChannel, packet.Sequence))
	require.NoError(t, path.EndpointA.UpdateClient())

	wantLabels := []metrics.Label{
		{Name: "source_port", Value: packet.SourcePort},
		{Name: "source_channel", Value: packet.SourceChannel},
		{Name: "destination_port", Value: packet.DestinationPort},
		{Name: "destination_channel", Value: packet.DestinationChannel},
		{Name: "timeout_type", Value: timeoutType},
	}
	assertCounter := func(wantCount int) {
		t.Helper()
		count := 0
		sum := float64(0)
		for _, interval := range sink.Data() {
			for _, counter := range interval.Counters {
				if strings.HasPrefix(counter.Name, "ibc.timeout.packet") {
					require.Equal(t, "ibc.timeout.packet", counter.Name, "timeout type is a label, never a metric suffix")
					require.ElementsMatch(t, wantLabels, counter.Labels)
					count += counter.Count
					sum += counter.Sum
				}
			}
		}
		require.Equal(t, wantCount, count)
		require.Equal(t, float64(wantCount), sum)
	}
	assertCounter(0)
	// An unexpired packet on an open channel must not emit a successful timeout metric.
	proof, proofHeight := path.EndpointB.QueryProof(host.PacketReceiptKey(packet.DestinationPort, packet.DestinationChannel, packet.Sequence))
	if timeoutType == "height" {
		_, err = chainA.GetSimApp().IBCKeeper.Timeout(sdk.WrapSDKContext(chainA.GetContext()), channeltypes.NewMsgTimeout(packet, 1, proof, proofHeight, sender.String()))
	} else {
		proofClose, _ := path.EndpointB.QueryProof(host.ChannelKey(packet.DestinationPort, packet.DestinationChannel))
		_, err = chainA.GetSimApp().IBCKeeper.TimeoutOnClose(sdk.WrapSDKContext(chainA.GetContext()), channeltypes.NewMsgTimeoutOnClose(packet, 1, proof, proofClose, proofHeight, sender.String()))
	}
	require.Error(t, err)
	assertCounter(0)

	if timeoutType == "height" {
		coordinator.CommitNBlocks(chainB, 11)
		require.True(t, clienttypes.GetSelfHeight(chainB.GetContext()).GTE(timeoutHeight))
		require.NoError(t, path.EndpointA.UpdateClient())
		require.NoError(t, path.EndpointA.TimeoutPacket(packet))
	} else {
		// The committed counterparty store supplies a genuine closed-channel membership proof.
		require.NoError(t, path.EndpointB.SetChannelClosed())
		require.True(t, clienttypes.GetSelfHeight(chainB.GetContext()).LT(timeoutHeight))
		require.NoError(t, path.EndpointA.TimeoutOnClose(packet))
	}
	assertCounter(1)
	require.False(t, chainA.GetSimApp().IBCKeeper.ChannelKeeper.HasPacketCommitment(chainA.GetContext(), packet.SourcePort, packet.SourceChannel, packet.Sequence))
	require.Equal(t, initialBalance, chainA.GetSimApp().BankKeeper.GetBalance(chainA.GetContext(), sender, amount.Denom))
	if timeoutType == "height" {
		require.NoError(t, path.EndpointA.TimeoutPacket(packet))
	} else {
		require.NoError(t, path.EndpointA.TimeoutOnClose(packet))
	}
	assertCounter(1)
	require.Equal(t, initialBalance, chainA.GetSimApp().BankKeeper.GetBalance(chainA.GetContext(), sender, amount.Denom))
}
