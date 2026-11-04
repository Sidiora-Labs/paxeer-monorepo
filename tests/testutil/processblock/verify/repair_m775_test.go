package verify

import (
	"testing"

	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/processblock"
	"github.com/stretchr/testify/require"
)

func TestRepairM_775_Send(t *testing.T) {
	large := sdk.NewIntFromUint64(uint64(1) << 63)
	for _, tc := range []struct {
		name     string
		initial  sdk.Int
		amount   sdk.Int
		messages int
	}{
		{"large_stored_balance", large.AddRaw(100), sdk.NewInt(7), 1},
		{"large_transfer", large.MulRaw(3), large.AddRaw(1), 1},
		{"large_accumulated_delta", large.MulRaw(3), large.SubRaw(1), 2},
		{"small_balance", sdk.NewInt(100), sdk.NewInt(7), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := repairM775App(t)
			sender, recipient := a.NewAccount(), a.NewAccount()
			repairM775Fund(t, a, sender, sdk.NewCoins(sdk.NewCoin("repaircoin", tc.initial), sdk.NewInt64Coin("othercoin", 100)))
			msgs := make([]sdk.Msg, tc.messages)
			for i := range msgs {
				msgs[i] = banktypes.NewMsgSend(sender, recipient, sdk.NewCoins(sdk.NewCoin("repaircoin", tc.amount), sdk.NewInt64Coin("othercoin", 3)))
			}
			tx := repairM775Tx(t, sender, nil, msgs...)
			codes := Balance(t, a, func() []uint32 { return repairM775Deliver(t, a, tx) }, []signing.Tx{tx})()
			require.Equal(t, []uint32{0}, codes)
			total := tc.amount.MulRaw(int64(tc.messages))
			require.True(t, a.BankKeeper.GetBalance(a.Ctx(), sender, "repaircoin").Amount.Equal(tc.initial.Sub(total)))
			require.True(t, a.BankKeeper.GetBalance(a.Ctx(), recipient, "repaircoin").Amount.Equal(total))
			require.Equal(t, sdk.NewInt(int64(3*tc.messages)), a.BankKeeper.GetBalance(a.Ctx(), recipient, "othercoin").Amount)
		})
	}
}

func TestRepairM_775_MultiSend(t *testing.T) {
	a := repairM775App(t)
	sender, first, second := a.NewAccount(), a.NewAccount(), a.NewAccount()
	large := sdk.NewIntFromUint64(uint64(1) << 63).MulRaw(2)
	repairM775Fund(t, a, sender, sdk.NewCoins(sdk.NewCoin("repaircoin", large.MulRaw(4))))
	msg := banktypes.NewMsgMultiSend(
		[]banktypes.Input{{Address: sender.String(), Coins: sdk.NewCoins(sdk.NewCoin("repaircoin", large.MulRaw(3)))}},
		[]banktypes.Output{
			{Address: first.String(), Coins: sdk.NewCoins(sdk.NewCoin("repaircoin", large))},
			{Address: second.String(), Coins: sdk.NewCoins(sdk.NewCoin("repaircoin", large))},
			{Address: sender.String(), Coins: sdk.NewCoins(sdk.NewCoin("repaircoin", large))},
		})
	tx := repairM775Tx(t, sender, nil, msg)
	require.Equal(t, []uint32{0}, Balance(t, a, func() []uint32 { return repairM775Deliver(t, a, tx) }, []signing.Tx{tx})())
	require.True(t, a.BankKeeper.GetBalance(a.Ctx(), sender, "repaircoin").Amount.Equal(large.MulRaw(2)))
	for _, recipient := range []sdk.AccAddress{first, second} {
		require.True(t, a.BankKeeper.GetBalance(a.Ctx(), recipient, "repaircoin").Amount.Equal(large))
	}
}

func TestRepairM_775_Fees(t *testing.T) {
	a := repairM775App(t)
	payer, sender, recipient := a.NewAccount(), a.NewAccount(), a.NewAccount()
	large := sdk.NewIntFromUint64(uint64(1) << 63)
	fee := sdk.NewCoins(sdk.NewCoin("repaircoin", large.AddRaw(5)))
	repairM775Fund(t, a, payer, sdk.NewCoins(sdk.NewCoin("repaircoin", large.MulRaw(3))))
	repairM775Fund(t, a, sender, sdk.NewCoins(sdk.NewCoin("repaircoin", large.MulRaw(2))))
	tx := repairM775Tx(t, payer, fee, banktypes.NewMsgSend(sender, recipient, sdk.NewCoins(sdk.NewCoin("repaircoin", large))))
	require.Equal(t, []uint32{0}, Balance(t, a, func() []uint32 { return repairM775Deliver(t, a, tx) }, []signing.Tx{tx})())
	require.True(t, a.BankKeeper.GetBalance(a.Ctx(), payer, "repaircoin").Amount.Equal(large.MulRaw(2).SubRaw(5)))
	require.True(t, a.BankKeeper.GetBalance(a.Ctx(), sender, "repaircoin").Amount.Equal(large))
	require.True(t, a.BankKeeper.GetBalance(a.Ctx(), recipient, "repaircoin").Amount.Equal(large))
	collector := a.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	require.True(t, a.BankKeeper.GetBalance(a.Ctx(), collector, "repaircoin").Amount.Equal(fee.AmountOf("repaircoin")))
}

func repairM775App(t *testing.T) *processblock.App {
	t.Helper()
	a := processblock.NewTestApp(t)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	return a
}

func repairM775Fund(t *testing.T, a *processblock.App, account sdk.AccAddress, coins sdk.Coins) {
	t.Helper()
	require.NoError(t, a.BankKeeper.MintCoins(a.Ctx(), minttypes.ModuleName, coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(a.Ctx(), minttypes.ModuleName, account, coins))
}

func repairM775Tx(t *testing.T, payer sdk.AccAddress, fees sdk.Coins, msgs ...sdk.Msg) signing.Tx {
	t.Helper()
	config := node.MakeEncodingConfig().TxConfig
	builder := config.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msgs...))
	feePayerSetter, ok := builder.(interface{ SetFeePayer(sdk.AccAddress) })
	require.True(t, ok, "the protobuf transaction builder supports an explicit fee payer")
	feePayerSetter.SetFeePayer(payer)
	builder.SetFeeAmount(fees)
	bz, err := config.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	decoded, err := config.TxDecoder()(bz)
	require.NoError(t, err)
	tx, ok := decoded.(signing.Tx)
	require.True(t, ok)
	require.Equal(t, payer, tx.FeePayer())
	return tx
}

func repairM775Deliver(t *testing.T, a *processblock.App, tx signing.Tx) []uint32 {
	t.Helper()
	if !tx.GetFee().IsZero() {
		require.NoError(t, a.BankKeeper.SendCoinsFromAccountToModule(a.Ctx(), tx.FeePayer(), authtypes.FeeCollectorName, tx.GetFee()))
	}
	server := bankkeeper.NewMsgServerImpl(a.BankKeeper)
	ctx := sdk.WrapSDKContext(a.Ctx())
	for _, msg := range tx.GetMsgs() {
		require.NoError(t, msg.ValidateBasic())
		switch m := msg.(type) {
		case *banktypes.MsgSend:
			_, err := server.Send(ctx, m)
			require.NoError(t, err)
		case *banktypes.MsgMultiSend:
			_, err := server.MultiSend(ctx, m)
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected message type %T", msg)
		}
	}
	return []uint32{0}
}
