package keeper_test

import (
	"context"
	"testing"
	"time"

	transfer "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/types"
	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	metrics "github.com/armon/go-metrics"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func repairM1205Suite(t *testing.T) *KeeperTestSuite {
	s := new(KeeperTestSuite)
	s.SetT(t)
	s.SetupTest()
	t.Cleanup(func() {
		for _, chain := range s.coordinator.Chains {
			require.NoError(t, chain.GetSimApp().Close())
		}
	})
	return s
}

func repairM1205Send(t *testing.T, path *ibctesting.Path, coin sdk.Coin, receiver string) channeltypes.Packet {
	t.Helper()
	sender := path.EndpointA.Chain
	msg := types.NewMsgTransfer(types.PortID, path.EndpointA.ChannelID, coin, sender.SenderAccount.GetAddress().String(), receiver, path.EndpointB.Chain.GetTimeoutHeight(), 0)
	res, err := sender.SendMsgs(msg)
	require.NoError(t, err)
	packet, err := ibctesting.ParsePacketFromEvents(res.GetEvents())
	require.NoError(t, err)
	require.Equal(t, coin.Amount.String(), func() string {
		var data types.FungibleTokenPacketData
		require.NoError(t, types.ModuleCdc.UnmarshalAsJSON(packet.Data, &data))
		return data.Amount
	}())
	return packet
}

func repairM1205Receive(t *testing.T, path *ibctesting.Path, packet channeltypes.Packet) channeltypes.Acknowledgement {
	t.Helper()
	require.NoError(t, path.EndpointB.UpdateClient())
	res, err := path.EndpointB.RecvPacketWithResult(packet)
	require.NoError(t, err)
	bz, err := ibctesting.ParseAckFromEvents(res.GetEvents())
	require.NoError(t, err)
	var ack channeltypes.Acknowledgement
	require.NoError(t, types.ModuleCdc.UnmarshalAsJSON(bz, &ack))
	return ack
}

// Traces come only from committed MsgTransfer/MsgRecvPacket transactions.
func repairM1205Produce(t *testing.T, s *KeeperTestSuite, base string) types.DenomTrace {
	t.Helper()
	ab := NewTransferPath(s.chainA, s.chainB)
	bc := NewTransferPath(s.chainB, s.chainC)
	s.coordinator.Setup(ab)
	s.coordinator.Setup(bc)
	amount := sdk.NewInt(17)
	if base != sdk.DefaultBondDenom {
		require.NoError(t, simapp.FundAccount(s.chainA.GetSimApp(), s.chainA.GetContext(), s.chainA.SenderAccount.GetAddress(), sdk.NewCoins(sdk.NewCoin(base, amount))))
		s.coordinator.CommitBlock(s.chainA)
	}
	packet := repairM1205Send(t, ab, sdk.NewCoin(base, amount), s.chainB.SenderAccount.GetAddress().String())
	require.NoError(t, ab.RelayPacket(packet))
	first := types.ParseDenomTrace(types.GetPrefixedDenom(types.PortID, ab.EndpointB.ChannelID, base))
	packet = repairM1205Send(t, bc, sdk.NewCoin(first.IBCDenom(), amount), s.chainC.SenderAccount.GetAddress().String())
	require.NoError(t, bc.RelayPacket(packet))
	expected := types.ParseDenomTrace(types.GetPrefixedDenom(types.PortID, bc.EndpointB.ChannelID, first.GetFullDenomPath()))
	trace, found := s.chainC.GetSimApp().TransferKeeper.GetDenomTrace(s.chainC.GetContext(), expected.Hash())
	require.True(t, found)
	require.Equal(t, expected, trace)
	require.Equal(t, base, trace.BaseDenom)
	require.NoError(t, trace.Validate())
	require.Equal(t, amount, s.chainC.GetSimApp().BankKeeper.GetBalance(s.chainC.GetContext(), s.chainC.SenderAccount.GetAddress(), trace.IBCDenom()).Amount)
	return trace
}

func TestRepairM_1205_GenesisRecovery(t *testing.T) {
	repairM1205GenesisRecovery(t)
}

func TestRepairM_1205_GenesisRefusals(t *testing.T) {
	s := repairM1205Suite(t)
	app := s.chainA.GetSimApp()
	for _, trace := range []types.DenomTrace{
		{Path: "transfer/channel?0", BaseDenom: "uatom"},
		{Path: "transfer/channel-0/transfer", BaseDenom: "uatom"},
		{Path: "transfer/channel-0", BaseDenom: ""},
	} {
		genesis := types.NewGenesisState(types.PortID, types.Traces{trace}, types.DefaultParams())
		bz := app.AppCodec().MustMarshalJSON(genesis)
		require.Error(t, (transfer.AppModuleBasic{}).ValidateGenesis(app.AppCodec(), app.GetTxConfig(), bz))
	}
}

func TestRepairM_1205_QueryTransport(t *testing.T) {
	repairM1205QueryTransport(t)
}

func TestRepairM_1205_ModelOutcomes(t *testing.T) {
	s := repairM1205Suite(t)
	s.TestModelBasedRelay()
}

func TestRepairM_1205_AtomicRefusals(t *testing.T) {
	for _, refusal := range []string{"receive-disabled", "blocked-after-mint", "invalid-receiver"} {
		t.Run(refusal, func(t *testing.T) {
			s := repairM1205Suite(t)
			path := NewTransferPath(s.chainA, s.chainB)
			s.coordinator.Setup(path)
			receiver := s.chainB.SenderAccount.GetAddress().String()
			switch refusal {
			case "receive-disabled":
				s.chainB.GetSimApp().TransferKeeper.SetParams(s.chainB.GetContext(), types.NewParams(true, false))
				s.coordinator.CommitBlock(s.chainB)
			case "blocked-after-mint":
				receiver = s.chainB.GetSimApp().AccountKeeper.GetModuleAddress(types.ModuleName).String()
			case "invalid-receiver":
				receiver = "invalid-receiver"
			}
			packet := repairM1205Send(t, path, sdk.NewInt64Coin(sdk.DefaultBondDenom, 23), receiver)
			before := BankOfChain(s.chainB)
			traces := s.chainB.GetSimApp().TransferKeeper.GetAllDenomTraces(s.chainB.GetContext())
			supply := s.chainB.GetSimApp().BankKeeper.ExportGenesis(s.chainB.GetContext()).Supply
			ack := repairM1205Receive(t, path, packet)
			require.False(t, ack.Success(), "protocol callback must refuse")
			require.NotEmpty(t, ack.GetError())
			after := BankOfChain(s.chainB)
			delta := after.Sub(&before)
			require.Empty(t, delta.NonZeroString(), "committed balances must be unchanged")
			require.Equal(t, traces, s.chainB.GetSimApp().TransferKeeper.GetAllDenomTraces(s.chainB.GetContext()), "new trace must roll back")
			require.Equal(t, supply, s.chainB.GetSimApp().BankKeeper.ExportGenesis(s.chainB.GetContext()).Supply, "mint must roll back")
			ackHash, found := s.chainB.App.GetIBCKeeper().ChannelKeeper.GetPacketAcknowledgement(s.chainB.GetContext(), packet.DestinationPort, packet.DestinationChannel, packet.Sequence)
			require.True(t, found, "error acknowledgement must commit")
			require.NotEmpty(t, ackHash)
		})
	}
}

func TestRepairM_1205_MetricContract(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	legacy := metrics.NewInmemSink(time.Minute, time.Hour)
	config := metrics.DefaultConfig("")
	config.EnableHostname = false
	config.EnableRuntimeMetrics = false
	_, err := metrics.NewGlobal(config, legacy)
	require.NoError(t, err)
	s := repairM1205Suite(t)
	path := NewTransferPath(s.chainA, s.chainB)
	s.coordinator.Setup(path)
	amount := sdk.NewInt64Coin(sdk.DefaultBondDenom, 31)
	packet := repairM1205Send(t, path, amount, s.chainB.SenderAccount.GetAddress().String())
	require.NoError(t, path.RelayPacket(packet))
	trace := types.ParseDenomTrace(types.GetPrefixedDenom(types.PortID, path.EndpointB.ChannelID, amount.Denom))
	reverse := &ibctesting.Path{EndpointA: path.EndpointB, EndpointB: path.EndpointA}
	packet = repairM1205Send(t, reverse, sdk.NewCoin(trace.IBCDenom(), amount.Amount), s.chainA.SenderAccount.GetAddress().String())
	require.NoError(t, reverse.RelayPacket(packet))
	intervals := legacy.Data()
	receivedGauges := map[string]float32{}
	for _, interval := range intervals {
		for _, gauge := range interval.Gauges {
			if gauge.Name != "ibc.transfer.packet.receive" {
				continue
			}
			require.Len(t, gauge.Labels, 1)
			require.Equal(t, metrics.Label{Name: "denom", Value: amount.Denom}, gauge.Labels[0])
			receivedGauges[gauge.Labels[0].Value] = gauge.Value
		}
	}
	require.Equal(t, map[string]float32{amount.Denom: 31}, receivedGauges)
	for _, key := range []string{"ibc.transfer.send", "ibc.transfer.receive"} {
		for _, source := range []string{"true", "false"} {
			total := float64(0)
			found := false
			for _, interval := range intervals {
				for _, counter := range interval.Counters {
					if counter.Name != key {
						continue
					}
					for _, label := range counter.Labels {
						if label.Name != "source" || label.Value != source {
							continue
						}
						require.Len(t, counter.Labels, 3)
						port, channel := "source_port", "source_channel"
						if key == "ibc.transfer.send" {
							port, channel = "destination_port", "destination_channel"
						}
						require.Contains(t, counter.Labels, metrics.Label{Name: port, Value: types.PortID})
						require.Contains(t, counter.Labels, metrics.Label{Name: channel, Value: path.EndpointA.ChannelID})
						total += counter.Sum
						found = true
					}
				}
			}
			require.True(t, found, "%s source=%s", key, source)
			require.Equal(t, float64(1), total, "%s source=%s", key, source)
		}
	}
	var resource metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &resource))
	instruments := map[string]bool{}
	for _, scope := range resource.ScopeMetrics {
		if scope.Scope.Name != "ibc_transfer_keeper" {
			continue
		}
		for _, m := range scope.Metrics {
			instruments[m.Name] = true
			switch m.Name {
			case "ibc_transfer_tx_msg", "ibc_transfer_packet_receive":
				g, ok := m.Data.(metricdata.Gauge[int64])
				require.True(t, ok)
				require.NotEmpty(t, g.DataPoints)
				classes := map[string]bool{}
				for _, point := range g.DataPoints {
					require.Equal(t, int64(31), point.Value)
					require.Equal(t, 1, point.Attributes.Len())
					class, ok := point.Attributes.Value("denom_class")
					require.True(t, ok)
					classes[class.AsString()] = true
				}
				expected := map[string]bool{telemetry.DenomClass(amount.Denom): true}
				if m.Name == "ibc_transfer_tx_msg" {
					expected[telemetry.DenomClass(trace.GetFullDenomPath())] = true
				}
				require.Equal(t, expected, classes)
				require.Equal(t, "{token}", m.Unit)
			case "ibc_transfer_send", "ibc_transfer_receive":
				sum, ok := m.Data.(metricdata.Sum[int64])
				require.True(t, ok)
				require.True(t, sum.IsMonotonic)
				require.Equal(t, "{count}", m.Unit)
				branches := map[bool]bool{}
				for _, point := range sum.DataPoints {
					source, ok := point.Attributes.Value("source")
					require.True(t, ok)
					branches[source.AsBool()] = true
					require.Equal(t, int64(1), point.Value)
					require.Equal(t, 3, point.Attributes.Len())
					portKey, channelKey := "source_port", "source_channel"
					if m.Name == "ibc_transfer_send" {
						portKey, channelKey = "destination_port", "destination_channel"
					}
					port, ok := point.Attributes.Value(attribute.Key(portKey))
					require.True(t, ok)
					channel, ok := point.Attributes.Value(attribute.Key(channelKey))
					require.True(t, ok)
					require.Equal(t, types.PortID, port.AsString())
					require.Equal(t, path.EndpointA.ChannelID, channel.AsString())
				}
				require.Equal(t, map[bool]bool{true: true, false: true}, branches)
			}
		}
	}
	require.Equal(t, map[string]bool{"ibc_transfer_tx_msg": true, "ibc_transfer_packet_receive": true, "ibc_transfer_send": true, "ibc_transfer_receive": true}, instruments)
}
