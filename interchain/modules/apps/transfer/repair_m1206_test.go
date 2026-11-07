package transfer_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	transfer "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/types"
	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/gogo/gateway"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	metric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func repairM1206Path(t *testing.T) (*ibctesting.Coordinator, *ibctesting.Path) {
	t.Helper()
	coord := ibctesting.NewCoordinator(t, 2)
	path := NewTransferPath(coord.GetChain(ibctesting.GetChainID(1)), coord.GetChain(ibctesting.GetChainID(2)))
	coord.Setup(path)
	return coord, path
}

func repairM1206Doc(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("spec/" + name + ".md")
	require.NoError(t, err)
	return string(b)
}

func repairM1206Send(t *testing.T, from *ibctesting.Endpoint, coin sdk.Coin) channeltypes.Packet {
	t.Helper()
	msg := types.NewMsgTransfer(from.ChannelConfig.PortID, from.ChannelID, coin,
		from.Chain.SenderAccount.GetAddress().String(), from.Counterparty.Chain.SenderAccount.GetAddress().String(),
		from.Counterparty.Chain.GetTimeoutHeight(), 0)
	msg.Memo = "repair-m1206"
	res, err := from.Chain.SendMsgs(msg)
	require.NoError(t, err)
	packet, err := ibctesting.ParsePacketFromEvents(res.GetEvents())
	require.NoError(t, err)
	return packet
}

// Relay the actual packet and acknowledgement with core commitment proofs, retaining
// transaction events so the consumer assertions cover the application callbacks.
func repairM1206Relay(t *testing.T, from *ibctesting.Endpoint, packet channeltypes.Packet) (*sdk.Result, *sdk.Result, channeltypes.Acknowledgement) {
	t.Helper()
	to := from.Counterparty
	require.NoError(t, to.UpdateClient())
	received, err := to.RecvPacketWithResult(packet)
	require.NoError(t, err)
	ackBytes, err := ibctesting.ParseAckFromEvents(received.GetEvents())
	require.NoError(t, err)
	var ack channeltypes.Acknowledgement
	require.NoError(t, types.ModuleCdc.UnmarshalAsJSON(ackBytes, &ack))
	proof, height := to.QueryProof(host.PacketAcknowledgementKey(packet.DestinationPort, packet.DestinationChannel, packet.Sequence))
	msg := channeltypes.NewMsgAcknowledgement(packet, ackBytes, proof, height, from.Chain.SenderAccount.GetAddress().String())
	acknowledged, err := from.Chain.SendMsgs(msg)
	require.NoError(t, err)
	return received, acknowledged, ack
}

func repairM1206Balance(t *testing.T, chain *ibctesting.TestChain, addr sdk.AccAddress, denom string, want sdk.Int) {
	t.Helper()
	got := chain.GetSimApp().BankKeeper.GetBalance(chain.GetContext(), addr, denom)
	require.True(t, got.Amount.Equal(want), "%s: got %s, want %s", denom, got.Amount, want)
}

func repairM1206Events(res *sdk.Result, eventType string) []map[string]string {
	var result []map[string]string
	for _, event := range res.Events {
		if event.Type != eventType {
			continue
		}
		attrs := map[string]string{}
		for _, a := range event.Attributes {
			attrs[string(a.Key)] = string(a.Value)
		}
		result = append(result, attrs)
	}
	return result
}

// Observe rejected callback events before core discards its cached application
// context. The packet still comes from a real send and is relayed separately.
func repairM1206RejectedReceiveCallback(t *testing.T, to *ibctesting.Endpoint, packet channeltypes.Packet) (*sdk.Result, channeltypes.Acknowledgement) {
	t.Helper()
	ctx, _ := to.Chain.GetContext().CacheContext()
	ack := transfer.NewIBCModule(to.Chain.GetSimApp().TransferKeeper).OnRecvPacket(ctx, packet, to.Chain.SenderAccount.GetAddress())
	require.False(t, ack.Success())
	var decoded channeltypes.Acknowledgement
	require.NoError(t, types.ModuleCdc.UnmarshalAsJSON(ack.Acknowledgement(), &decoded))
	return &sdk.Result{Events: ctx.EventManager().ABCIEvents()}, decoded
}

func TestRepairM_1206_QueryShape(t *testing.T) {
	coord, path := repairM1206Path(t)
	a, b := path.EndpointA.Chain, path.EndpointB.Chain
	coin := sdk.NewInt64Coin("uatom", 19)
	require.NoError(t, a.GetSimApp().BankKeeper.MintCoins(a.GetContext(), types.ModuleName, sdk.NewCoins(coin)))
	require.NoError(t, a.GetSimApp().BankKeeper.SendCoinsFromModuleToAccount(a.GetContext(), types.ModuleName, a.SenderAccount.GetAddress(), sdk.NewCoins(coin)))
	coord.CommitBlock(a)
	packet := repairM1206Send(t, path.EndpointA, coin)
	_, _, ack := repairM1206Relay(t, path.EndpointA, packet)
	require.True(t, ack.Success())
	trace := types.ParseDenomTrace(types.GetPrefixedDenom(packet.DestinationPort, packet.DestinationChannel, "uatom"))
	require.Equal(t, "transfer/channel-0", trace.Path)
	require.Equal(t, "27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2", trace.Hash().String())
	repairM1206Balance(t, b, b.SenderAccount.GetAddress(), trace.IBCDenom(), coin.Amount)

	query := baseapp.NewQueryServerTestHelper(b.GetContext(), b.GetSimApp().InterfaceRegistry())
	types.RegisterQueryServer(query, b.GetSimApp().TransferKeeper)
	// Match the production API server's gogo marshaler and field-name policy.
	mux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, &gateway.JSONPb{OrigName: true, EmitDefaults: true}))
	require.NoError(t, types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(query)))
	route := "/ibc/apps/transfer/v1/denom_traces/" + trace.Hash().String()
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var decoded types.QueryDenomTraceResponse
	require.NoError(t, types.ModuleCdc.UnmarshalAsJSON(response.Body.Bytes(), &decoded))
	require.Equal(t, trace, *decoded.DenomTrace)
	// This is the worked response in the guide, produced through the registered gateway.
	expected := `{"denom_trace":{"path":"transfer/channel-0","base_denom":"uatom"}}`
	require.JSONEq(t, expected, response.Body.String())
	doc := repairM1206Doc(t, "01_concepts")
	require.Contains(t, doc, "GET "+route)
	require.Contains(t, doc, expected)
	require.Contains(t, doc, "Tenant namespaces inside Paxeer are not additional IBC chains")
	multi := types.ParseDenomTrace("transfer/channel-7/" + trace.GetFullDenomPath())
	require.Equal(t, "transfer/channel-7/transfer/channel-0", multi.Path)
	require.Equal(t, "uatom", multi.BaseDenom)
	require.Contains(t, doc, multi.GetFullDenomPath())
	for _, suffix := range []string{"not-a-hash", strings.Repeat("0", 64)} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ibc/apps/transfer/v1/denom_traces/"+suffix, nil))
		if suffix == "not-a-hash" {
			require.Equal(t, http.StatusBadRequest, rr.Code)
		} else {
			require.Equal(t, http.StatusNotFound, rr.Code)
		}
	}
}

func TestRepairM_1206_PortStorage(t *testing.T) {
	_, path := repairM1206Path(t)
	chain := path.EndpointA.Chain
	app, ctx := chain.GetSimApp(), chain.GetContext()
	k := app.TransferKeeper
	store := ctx.KVStore(app.GetKey(types.StoreKey))
	require.Equal(t, []byte{0x01}, types.PortKey)
	require.Equal(t, []byte{0x02}, types.DenomTraceKey)
	// The initialized port was written by production genesis, then read by the keeper.
	require.Equal(t, []byte{0x74, 0x72, 0x61, 0x6e, 0x73, 0x66, 0x65, 0x72}, store.Get(types.PortKey))
	require.Equal(t, "transfer", k.GetPort(ctx))
	k.SetPort(ctx, "transfer-port")
	require.Equal(t, []byte("transfer-port"), store.Get(types.PortKey))
	require.Equal(t, "transfer-port", k.GetPort(ctx))
	k.SetPort(ctx, "transfer")
	trace := types.ParseDenomTrace("transfer/channel-7/transfer/channel-0/uatom")
	k.SetDenomTrace(ctx, trace)
	key := append(append([]byte{}, types.DenomTraceKey...), trace.Hash()...)
	raw := store.Get(key)
	require.Len(t, key, 33)
	require.Equal(t, k.MustMarshalDenomTrace(trace), raw)
	var decoded types.DenomTrace
	require.NoError(t, app.AppCodec().Unmarshal(raw, &decoded))
	require.Equal(t, trace, decoded)
	got, found := k.GetDenomTrace(ctx, trace.Hash())
	require.True(t, found)
	require.Equal(t, trace, got)
	require.Nil(t, store.Get(append(append([]byte{}, types.DenomTraceKey...), []byte(trace.Hash().String())...)))
	doc := repairM1206Doc(t, "02_state")
	for _, text := range []string{"`0x01`", "Raw `[]byte(portID)`", "74 72 61 6e 73 66 65 72", "Protobuf-encoded `DenomTrace`", "../keeper/encoding.go"} {
		require.Contains(t, doc, text)
	}
}

func TestRepairM_1206_CallbackEvents(t *testing.T) {
	coord, path := repairM1206Path(t)
	a, b := path.EndpointA.Chain, path.EndpointB.Chain
	coin := sdk.NewInt64Coin(sdk.DefaultBondDenom, 23)
	original := a.GetSimApp().BankKeeper.GetBalance(a.GetContext(), a.SenderAccount.GetAddress(), coin.Denom).Amount
	packet := repairM1206Send(t, path.EndpointA, coin)
	received, acknowledged, ack := repairM1206Relay(t, path.EndpointA, packet)
	require.True(t, ack.Success())
	common := map[string]string{"module": "transfer", "sender": a.SenderAccount.GetAddress().String(), "receiver": b.SenderAccount.GetAddress().String(), "denom": coin.Denom, "amount": "23", "memo": "repair-m1206"}
	recvAttrs := map[string]string{}
	for k, v := range common {
		recvAttrs[k] = v
	}
	recvAttrs["success"] = "true"
	require.Equal(t, []map[string]string{recvAttrs}, repairM1206Events(received, "fungible_token_packet"))
	ackAttrs := map[string]string{}
	for k, v := range common {
		ackAttrs[k] = v
	}
	ackAttrs["acknowledgement"] = ack.String()
	require.Equal(t, []map[string]string{ackAttrs, {"success": string([]byte{1})}}, repairM1206Events(acknowledged, "fungible_token_packet"))
	trace := types.ParseDenomTrace(types.GetPrefixedDenom(packet.DestinationPort, packet.DestinationChannel, coin.Denom))
	require.Equal(t, []map[string]string{{"trace_hash": trace.Hash().String(), "denom": trace.IBCDenom()}}, repairM1206Events(received, "denomination_trace"))

	b.GetSimApp().TransferKeeper.SetParams(b.GetContext(), types.NewParams(true, false))
	coord.CommitBlock(b)
	failedPacket := repairM1206Send(t, path.EndpointA, coin)
	failedRecv, callbackAck := repairM1206RejectedReceiveCallback(t, path.EndpointB, failedPacket)
	failedTxRecv, failedAck, errorAck := repairM1206Relay(t, path.EndpointA, failedPacket)
	require.False(t, errorAck.Success())
	require.Equal(t, callbackAck, errorAck)
	require.Empty(t, repairM1206Events(failedTxRecv, "fungible_token_packet"), "core discards rejected application events")
	recvAttrs["success"] = "false"
	require.Equal(t, []map[string]string{recvAttrs}, repairM1206Events(failedRecv, "fungible_token_packet"))
	ackAttrs["acknowledgement"] = errorAck.String()
	require.Equal(t, []map[string]string{ackAttrs, {"error": errorAck.GetError()}}, repairM1206Events(failedAck, "fungible_token_packet"))
	repairM1206Balance(t, a, a.SenderAccount.GetAddress(), coin.Denom, original.Sub(coin.Amount))

	// A production send supplies the packet and real escrow for the timeout callback.
	timeoutPacket := repairM1206Send(t, path.EndpointA, coin)
	ctx := a.GetContext().WithEventManager(sdk.NewEventManager())
	require.NoError(t, transfer.NewIBCModule(a.GetSimApp().TransferKeeper).OnTimeoutPacket(ctx, timeoutPacket, a.SenderAccount.GetAddress()))
	timeoutResult := &sdk.Result{Events: ctx.EventManager().ABCIEvents()}
	require.Equal(t, []map[string]string{{"module": "transfer", "refund_receiver": common["sender"], "refund_denom": coin.Denom, "refund_amount": "23", "memo": "repair-m1206"}}, repairM1206Events(timeoutResult, "timeout"))
	repairM1206Balance(t, a, a.SenderAccount.GetAddress(), coin.Denom, original.Sub(coin.Amount))
	repairM1206Balance(t, a, types.GetEscrowAddress(timeoutPacket.SourcePort, timeoutPacket.SourceChannel), coin.Denom, coin.Amount)
	doc := repairM1206Doc(t, "05_events")
	for _, text := range []string{"| fungible_token_packet | sender | `data.Sender` |", "| timeout | refund_receiver |", "| timeout | refund_denom |", "| timeout | refund_amount |", "| Result | success | `string(resp.Result)` |", "| Error | error | `resp.Error` |"} {
		require.Contains(t, doc, text)
	}
}

func TestRepairM_1206_ReceiveGating(t *testing.T) {
	coord, path := repairM1206Path(t)
	a, b := path.EndpointA.Chain, path.EndpointB.Chain
	coin := sdk.NewInt64Coin(sdk.DefaultBondDenom, 31)
	original := a.GetSimApp().BankKeeper.GetBalance(a.GetContext(), a.SenderAccount.GetAddress(), coin.Denom).Amount
	voucher := types.GetTransferCoin(path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, coin.Denom, coin.Amount)
	// Bank MsgSend policy is disabled for exactly the denomination B will mint.
	bankB := b.GetSimApp().BankKeeper
	bankB.SetParams(b.GetContext(), bankB.GetParams(b.GetContext()).SetSendEnabledParam(voucher.Denom, false))
	coord.CommitBlock(b)
	packet := repairM1206Send(t, path.EndpointA, coin)
	_, _, ack := repairM1206Relay(t, path.EndpointA, packet)
	require.True(t, ack.Success(), "bank send policy does not gate inbound voucher minting")
	repairM1206Balance(t, b, b.SenderAccount.GetAddress(), voucher.Denom, coin.Amount)
	_, err := bankkeeper.NewMsgServerImpl(bankB).Send(sdk.WrapSDKContext(b.GetContext()), &banktypes.MsgSend{
		FromAddress: b.SenderAccount.GetAddress().String(), ToAddress: b.SenderAccounts[1].SenderAccount.GetAddress().String(), Amount: sdk.NewCoins(voucher),
	})
	require.ErrorIs(t, err, banktypes.ErrSendDisabled)
	_, err = b.GetSimApp().TransferKeeper.Transfer(sdk.WrapSDKContext(b.GetContext()), types.NewMsgTransfer(
		path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, voucher, b.SenderAccount.GetAddress().String(), a.SenderAccount.GetAddress().String(), a.GetTimeoutHeight(), 0))
	require.ErrorIs(t, err, types.ErrSendDisabled)

	bankB.SetParams(b.GetContext(), bankB.GetParams(b.GetContext()).SetSendEnabledParam(voucher.Denom, true))
	bankA := a.GetSimApp().BankKeeper
	bankA.SetParams(a.GetContext(), bankA.GetParams(a.GetContext()).SetSendEnabledParam(coin.Denom, false))
	coord.CommitBlock(a, b)
	returned := repairM1206Send(t, path.EndpointB, voucher)
	_, _, ack = repairM1206Relay(t, path.EndpointB, returned)
	require.True(t, ack.Success(), "bank send policy does not gate inbound unescrow")
	repairM1206Balance(t, a, a.SenderAccount.GetAddress(), coin.Denom, original)
	repairM1206Balance(t, b, b.SenderAccount.GetAddress(), voucher.Denom, sdk.ZeroInt())

	bankA.SetParams(a.GetContext(), bankA.GetParams(a.GetContext()).SetSendEnabledParam(coin.Denom, true))
	b.GetSimApp().TransferKeeper.SetParams(b.GetContext(), types.NewParams(true, false))
	coord.CommitBlock(a, b)
	refused := repairM1206Send(t, path.EndpointA, coin)
	// Refunds must remain possible with new sends, receives and bank MsgSend disabled.
	a.GetSimApp().TransferKeeper.SetParams(a.GetContext(), types.NewParams(false, false))
	bankA.SetParams(a.GetContext(), bankA.GetParams(a.GetContext()).SetSendEnabledParam(coin.Denom, false))
	coord.CommitBlock(a)
	received, callbackAck := repairM1206RejectedReceiveCallback(t, path.EndpointB, refused)
	receivedTx, _, ack := repairM1206Relay(t, path.EndpointA, refused)
	require.False(t, ack.Success())
	require.Equal(t, callbackAck, ack)
	require.Empty(t, repairM1206Events(receivedTx, "fungible_token_packet"), "core discards rejected application events")
	require.Contains(t, ack.GetError(), "ABCI code: 8:")
	receiveEvents := repairM1206Events(received, "fungible_token_packet")
	require.Len(t, receiveEvents, 1)
	require.Equal(t, "false", receiveEvents[0]["success"])
	repairM1206Balance(t, b, b.SenderAccount.GetAddress(), voucher.Denom, sdk.ZeroInt())
	repairM1206Balance(t, a, a.SenderAccount.GetAddress(), coin.Denom, original)
	repairM1206Balance(t, a, types.GetEscrowAddress(refused.SourcePort, refused.SourceChannel), coin.Denom, sdk.ZeroInt())
	doc := repairM1206Doc(t, "07_params")
	for _, text := range []string{"`ReceiveEnabled = false`", "all denominations", "MsgSend", "MsgMultiSend", "SendCoinsFromModuleToAccount", "separate domain-6 task", "Neither transfer parameter gates"} {
		require.Contains(t, doc, text)
	}
}

func TestRepairM_1206_MultihopBalanceProof(t *testing.T) {
	// Execute the repaired integration proof itself, so it cannot drift away from
	// the focused gate. It sends 2^63 through A -> B -> C -> B using real IBC proofs.
	s := new(TransferTestSuite)
	s.SetT(t)
	s.SetupTest()
	s.TestHandleMsgTransfer()
}

func repairM1206Metric(t *testing.T, reader *metric.ManualReader, name string) metricdata.Metrics {
	t.Helper()
	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &data))
	for _, scope := range data.ScopeMetrics {
		if scope.Scope.Name != "ibc_transfer_keeper" {
			continue
		}
		for _, m := range scope.Metrics {
			if m.Name == name {
				return m
			}
		}
	}
	t.Fatalf("missing production metric %s", name)
	return metricdata.Metrics{}
}

func repairM1206Gauge(t *testing.T, reader *metric.ManualReader, name, class string, want int64) {
	t.Helper()
	m := repairM1206Metric(t, reader, name)
	gauge, ok := m.Data.(metricdata.Gauge[int64])
	require.True(t, ok, "%s must be an int64 gauge", name)
	for _, point := range gauge.DataPoints {
		v, found := point.Attributes.Value(attribute.Key("denom_class"))
		if found && v.AsString() == class {
			require.Equal(t, want, point.Value)
			require.Equal(t, 1, point.Attributes.Len(), "gauges have only denom_class")
			return
		}
	}
	t.Fatalf("missing %s denom_class=%s", name, class)
}

func repairM1206Counter(t *testing.T, reader *metric.ManualReader, name string, attrs map[string]string, source bool, want int64) {
	t.Helper()
	m := repairM1206Metric(t, reader, name)
	sum, ok := m.Data.(metricdata.Sum[int64])
	require.True(t, ok)
	require.True(t, sum.IsMonotonic)
	for _, point := range sum.DataPoints {
		s, found := point.Attributes.Value(attribute.Key("source"))
		if !found || s.AsBool() != source {
			continue
		}
		matches := true
		for k, v := range attrs {
			got, found := point.Attributes.Value(attribute.Key(k))
			matches = matches && found && got.AsString() == v
		}
		if matches {
			require.Equal(t, want, point.Value)
			require.Equal(t, len(attrs)+1, point.Attributes.Len())
			return
		}
	}
	t.Fatalf("missing %s labels=%v source=%t", name, attrs, source)
}

func TestRepairM_1206_Metrics(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	old := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() { otel.SetMeterProvider(old); require.NoError(t, provider.Shutdown(context.Background())) })
	legacy, err := telemetry.New(telemetry.Config{Enabled: true})
	require.NoError(t, err)
	_, path := repairM1206Path(t)
	amounts := []sdk.Int{sdk.NewInt(7), sdk.NewInt(11), sdk.NewInt(math.MaxInt64)}
	huge, ok := sdk.NewIntFromString("9223372036854775808")
	require.True(t, ok)
	amounts = append(amounts, huge)
	// Existing genesis balances cannot cover two near-int64 sends, so fund the
	// actual bank account through the production module mint and bank transfer.
	a := path.EndpointA.Chain
	extra := sdk.NewCoin(sdk.DefaultBondDenom, huge)
	require.NoError(t, a.GetSimApp().BankKeeper.MintCoins(a.GetContext(), types.ModuleName, sdk.NewCoins(extra)))
	require.NoError(t, a.GetSimApp().BankKeeper.SendCoinsFromModuleToAccount(a.GetContext(), types.ModuleName, a.SenderAccount.GetAddress(), sdk.NewCoins(extra)))
	a.Coordinator.CommitBlock(a)
	for i, amount := range amounts {
		packet := repairM1206Send(t, path.EndpointA, sdk.NewCoin(sdk.DefaultBondDenom, amount))
		_, _, ack := repairM1206Relay(t, path.EndpointA, packet)
		require.True(t, ack.Success())
		want := amount
		if !amount.IsInt64() {
			want = amounts[i-1]
		}
		repairM1206Gauge(t, reader, "ibc_transfer_tx_msg", "uhpx", want.Int64())
		repairM1206Gauge(t, reader, "ibc_transfer_packet_receive", "uhpx", want.Int64())
		// Legacy gauges coexist and keep the same last representable amount,
		// with the original denomination label and float32 precision.
		gathered, err := legacy.Gather(telemetry.FormatDefault)
		require.NoError(t, err)
		var snapshot struct {
			Gauges []struct {
				Name   string
				Value  float32
				Labels map[string]string
			}
		}
		require.NoError(t, json.Unmarshal(gathered.Metrics, &snapshot))
		for _, name := range []string{"tx.msg.ibc.transfer", "ibc.transfer.packet.receive"} {
			found := false
			for _, g := range snapshot.Gauges {
				if g.Name != name {
					continue
				}
				if g.Labels["denom"] == sdk.DefaultBondDenom {
					require.Equal(t, float32(want.Int64()), g.Value)
					found = true
				}
			}
			require.True(t, found, "missing legacy %s: %s", name, gathered.Metrics)
		}
	}
	repairM1206Counter(t, reader, "ibc_transfer_send", map[string]string{"destination_port": path.EndpointB.ChannelConfig.PortID, "destination_channel": path.EndpointB.ChannelID}, true, 4)
	repairM1206Counter(t, reader, "ibc_transfer_receive", map[string]string{"source_port": path.EndpointA.ChannelConfig.PortID, "source_channel": path.EndpointA.ChannelID}, false, 4)
	// Burning and unescrowing exercise the complementary source roles and the
	// expanded trace's bounded class ("other", despite the bank's ibc/ denom).
	voucher := types.GetTransferCoin(path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, sdk.DefaultBondDenom, sdk.NewInt(5))
	returned := repairM1206Send(t, path.EndpointB, voucher)
	_, _, ack := repairM1206Relay(t, path.EndpointB, returned)
	require.True(t, ack.Success())
	repairM1206Gauge(t, reader, "ibc_transfer_tx_msg", "other", 5)
	repairM1206Gauge(t, reader, "ibc_transfer_packet_receive", "uhpx", 5)
	repairM1206Counter(t, reader, "ibc_transfer_send", map[string]string{"destination_port": path.EndpointA.ChannelConfig.PortID, "destination_channel": path.EndpointA.ChannelID}, false, 1)
	repairM1206Counter(t, reader, "ibc_transfer_receive", map[string]string{"source_port": path.EndpointB.ChannelConfig.PortID, "source_channel": path.EndpointB.ChannelID}, true, 1)
	doc := repairM1206Doc(t, "06_metrics")
	for _, text := range []string{"tx_msg_ibc_transfer", "ibc_transfer_tx_msg", "Last representable", "denom_class", "destination_channel", "source_channel", "9223372036854775807", "leaves the preceding representable"} {
		require.Contains(t, doc, text)
	}
}
