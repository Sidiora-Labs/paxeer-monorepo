package baseapp

import (
	"crypto/sha256"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/testdata"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
)

func runTxDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount("paxcosmos_baseapp", "run_tx_duration")
}

func runMsgsDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount("paxcosmos_baseapp", "run_msgs_duration")
}

func runMsgLatency(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount("paxcosmos_baseapp", "run_msg_latency")
}

func TestRunTxRecordsRunTxRunMsgsAndMsgLatency(t *testing.T) {
	reader := oteltest.Install()
	deliverKey := []byte("otel-runtx-counter")
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

	afterRunTx := runTxDuration(reader)
	afterRunMsgs := runMsgsDuration(reader)
	afterRunMsg := runMsgLatency(reader)
	require.Equal(t, uint64(1), afterRunTx-beforeRunTx, "run_tx_duration")
	require.Equal(t, uint64(1), afterRunMsgs-beforeRunMsgs, "run_msgs_duration")
	require.Equal(t, uint64(2), afterRunMsg-beforeRunMsg, "run_msg_latency")
}
