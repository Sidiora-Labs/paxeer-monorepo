package baseapp

import (
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	scheduler "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/occ"
	"github.com/stretchr/testify/require"
)

func TestRunTxMapsAbortToErrOCCAbortWhenOCCEnabled(t *testing.T) {
	for _, stage := range []string{"ante", "message"} {
		t.Run(stage, func(t *testing.T) {
			res, err := runTxWithSchedulerAbort(t, true, stage)
			require.ErrorIs(t, err, sdkerrors.ErrOCCAbort)
			require.NotErrorIs(t, err, sdkerrors.ErrPanic)
			require.ErrorContains(t, err, "dependent index 7")
			require.ErrorContains(t, err, scheduler.ErrReadEstimate.Error())
			require.Nil(t, res.result)
		})
	}
}

func TestRunTxTreatsAbortAsPanicWhenOCCDisabled(t *testing.T) {
	for _, stage := range []string{"ante", "message"} {
		t.Run(stage, func(t *testing.T) {
			res, err := runTxWithSchedulerAbort(t, false, stage)
			require.ErrorIs(t, err, sdkerrors.ErrPanic)
			require.NotErrorIs(t, err, sdkerrors.ErrOCCAbort)
			require.ErrorContains(t, err, "recovered:")
			require.ErrorContains(t, err, "stack:")
			require.ErrorContains(t, err, scheduler.ErrReadEstimate.Error())
			require.Nil(t, res.result)
		})
	}
}

func runTxWithSchedulerAbort(t *testing.T, occEnabled bool, stage string) (runTxResult, error) {
	t.Helper()
	ante := anteHandlerTxTest(t, capKey1, []byte("ante-key"))
	handler := handlerMsgCounter(t, capKey1, []byte("deliver-key"))
	anteCalled, handlerCalled := false, false
	app := setupBaseApp(t, func(app *BaseApp) {
		app.SetAnteHandler(func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			newCtx, err := ante(ctx, tx, simulate)
			require.NoError(t, err)
			anteCalled = true
			if stage == "ante" {
				panic(scheduler.NewEstimateAbort(7))
			}
			return newCtx, err
		})
		app.Router().AddRoute(sdk.NewRoute(routeMsgCounter, func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
			_, err := handler(ctx, msg)
			require.NoError(t, err)
			handlerCalled = true
			panic(scheduler.NewEstimateAbort(7))
		}))
	})
	app.setDeliverState(tmproto.Header{Height: 1})
	ctx := app.deliverState.ctx.WithIsOCCEnabled(occEnabled)
	res, err := app.runTx(ctx, runTxModeDeliver, *newTxCounter(0, 0), [32]byte{})
	require.True(t, anteCalled)
	require.Equal(t, stage == "message", handlerCalled)
	return res, err
}
