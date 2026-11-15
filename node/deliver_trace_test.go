package app_test

import (
	"crypto/sha256"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/legacyabci"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/stretchr/testify/require"
)

func TestDeliverTraceFailure(t *testing.T) {
	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(1).WithChainID("pax-test").WithBlockTime(time.Unix(1_800_000_000, 0))
	ctx, _ = ctx.CacheContext()
	private := secp256k1.GenPrivKey()
	sender := sdk.AccAddress(private.PubKey().Address())
	recipient := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	funding := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, funding))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, sender, funding))
	account := a.AccountKeeper.GetAccount(ctx, sender)
	require.NotNil(t, account)
	tx, err := app.GenTx(a.GetTxConfig(), []sdk.Msg{&banktypes.MsgSend{
		FromAddress: sender.String(), ToAddress: recipient.String(),
		Amount: sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 2_000_000)),
	}}, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1_000)), 1_000_000,
		ctx.ChainID(), []uint64{account.GetAccountNumber()}, []uint64{account.GetSequence()}, private)
	require.NoError(t, err)
	txBytes, err := a.GetTxConfig().TxEncoder()(tx)
	require.NoError(t, err)

	for _, tc := range []struct {
		name    string
		tx      sdk.Tx
		bytes   []byte
		failure string
	}{
		{name: "decode", failure: "tx decode error"},
		{name: "message", tx: tx, bytes: txBytes, failure: "failed to execute message; message index: 0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checksum := sha256.Sum256(tc.bytes)
			baselineCtx, _ := ctx.CacheContext()
			gas, result, anteEvents, _, deliveryErr := legacyabci.DeliverTx(
				baselineCtx.WithTxBytes(tc.bytes).WithTxSum(checksum), tc.tx, a.GetTxConfig(), &a.DeliverTxKeepers, checksum,
				func(ctx sdk.Context) (sdk.Context, sdk.CacheMultiStore) { return a.CacheTxContext(ctx, checksum) },
				a.RunMsgs, a.TracingInfo, a.AddCosmosEventsToEVMReceiptIfApplicable,
			)
			require.ErrorContains(t, deliveryErr, tc.failure)
			require.Nil(t, result)
			if tc.name == "message" {
				require.ErrorIs(t, deliveryErr, sdkerrors.ErrInsufficientFunds)
				require.NotZero(t, gas.GasUsed)
				require.NotEmpty(t, anteEvents)
			}
			baseline := sdkerrors.ResponseDeliverTxWithEvents(deliveryErr, gas.GasWanted, gas.GasUsed, sdk.MarkEventsToIndex(anteEvents, a.IndexEvents), false)
			baseapp.SetTrace(false)(a.GetBaseApp())
			offCtx, _ := ctx.CacheContext()
			off := a.DeliverTx(offCtx, abci.RequestDeliverTxV2{Tx: tc.bytes}, tc.tx, checksum)
			expectedBytes, err := baseline.Marshal()
			require.NoError(t, err)
			actualBytes, err := off.Marshal()
			require.NoError(t, err)
			require.Equal(t, expectedBytes, actualBytes)

			baseapp.SetTrace(true)(a.GetBaseApp())
			onCtx, _ := ctx.CacheContext()
			on := a.DeliverTx(onCtx, abci.RequestDeliverTxV2{Tx: tc.bytes}, tc.tx, checksum)
			require.Contains(t, on.Log, deliveryErr.Error())
			require.Contains(t, on.Log, ".go:")
			require.NotEqual(t, off.Log, on.Log)
			on.Log = off.Log
			require.Equal(t, off, on)
		})
	}
}

func TestDeliverTraceRecoveredPanic(t *testing.T) {
	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil)
	var tx *legacytx.StdTx
	baseapp.SetTrace(false)(a.GetBaseApp())
	offCtx, _ := ctx.CacheContext()
	off := a.DeliverTx(offCtx, abci.RequestDeliverTxV2{}, tx, [32]byte{})
	baseline := sdkerrors.ResponseDeliverTxWithEvents(sdkerrors.ErrPanic, 0, 0, nil, false)
	expectedBytes, err := baseline.Marshal()
	require.NoError(t, err)
	actualBytes, err := off.Marshal()
	require.NoError(t, err)
	require.Equal(t, expectedBytes, actualBytes)
	require.Equal(t, "internal error", off.Log)

	baseapp.SetTrace(true)(a.GetBaseApp())
	onCtx, _ := ctx.CacheContext()
	on := a.DeliverTx(onCtx, abci.RequestDeliverTxV2{}, tx, [32]byte{})
	require.Contains(t, on.Log, "recovered:")
	require.Contains(t, on.Log, "recovered: value method github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx.StdTx.GetMsgs called using nil *StdTx pointer")
	require.Contains(t, on.Log, "stack:\n")
	on.Log = off.Log
	require.Equal(t, off, on)
}
