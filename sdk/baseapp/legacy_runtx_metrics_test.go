package baseapp

import (
	"crypto/sha256"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/testdata"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
)

func TestBaseappRunTxEmitsNoLegacyTelemetry(t *testing.T) {
	reader := oteltest.Install()
	sink := oteltest.InstallLegacySink(t)
	telemetry.IncrCounter(1, "baseapp", "legacy_probe")
	require.Contains(t, sink.Keys(), "oteltest.baseapp.legacy_probe")

	deliverKey := []byte("legacy-runtx-counter")
	app := setupBaseApp(t, func(app *BaseApp) {
		app.SetInterfaceRegistry(testdata.NewTestInterfaceRegistry())
		testdata.RegisterMsgServer(app.MsgServiceRouter(), testdata.MsgServerImpl{})
		app.Router().AddRoute(sdk.NewRoute(routeMsgCounter, handlerMsgCounter(t, capKey1, deliverKey)))
	})
	app.setDeliverState(tmproto.Header{Height: 1})
	ctx := app.deliverState.ctx
	tx := &txTest{Msgs: []sdk.Msg{
		&testdata.MsgCreateDog{Dog: &testdata.Dog{Name: "Spot"}},
		&msgCounter{Counter: 0},
	}}
	require.Len(t, tx.GetMsgs(), 2)
	require.NotNil(t, app.MsgServiceRouter().Handler(tx.Msgs[0]))
	require.Nil(t, app.MsgServiceRouter().Handler(tx.Msgs[1]))
	require.Zero(t, getIntFromStore(ctx.KVStore(capKey1), deliverKey))

	beforeRunTx := runTxDuration(reader)
	beforeRunMsgs := runMsgsDuration(reader)
	beforeRunMsg := runMsgLatency(reader)

	res, err := app.runTx(ctx, runTxModeDeliver, tx, sha256.Sum256([]byte(t.Name())))

	require.NoError(t, err)
	require.NotNil(t, res.result)
	require.Equal(t, int64(1), getIntFromStore(ctx.KVStore(capKey1), deliverKey))
	var msgData sdk.TxMsgData
	require.NoError(t, proto.Unmarshal(res.result.Data, &msgData))
	require.Len(t, msgData.Data, 2)
	require.Equal(t, sdk.MsgTypeURL(tx.Msgs[0]), msgData.Data[0].MsgType)
	var dogResponse testdata.MsgCreateDogResponse
	require.NoError(t, proto.Unmarshal(msgData.Data[0].Data, &dogResponse))
	require.Equal(t, "Spot", dogResponse.Name)
	require.Equal(t, sdk.MsgTypeURL(tx.Msgs[1]), msgData.Data[1].MsgType)

	require.Equal(t, uint64(1), runTxDuration(reader)-beforeRunTx, "run_tx_duration")
	require.Equal(t, uint64(1), runMsgsDuration(reader)-beforeRunMsgs, "run_msgs_duration")
	require.Equal(t, uint64(2), runMsgLatency(reader)-beforeRunMsg, "run_msg_latency")

	for _, key := range sink.Keys() {
		for _, legacyKey := range []string{
			"pax.cosmos.throughput.transaction",
			"pax.cosmos.throughput.message",
			"pax.cosmos.run.msg.latency",
			"cosmos.run.msg.latency",
		} {
			require.NotContains(t, key, legacyKey, "legacy runTx metric emitted")
		}
	}
}
