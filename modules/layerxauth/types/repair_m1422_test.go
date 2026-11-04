package types_test

import (
	"bytes"
	"testing"
	"time"

	types "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
)

type repairM1422Chain struct {
	*repair193Chain
}

func newRepairM1422Chain(t *testing.T) *repairM1422Chain {
	f := &repairM1422Chain{newRepair193Chain(t)}
	authz.RegisterMsgServer(f.router, f.keeper)
	return f
}

func (f *repairM1422Chain) grant(a types.LayerXScopeAuthorization) {
	f.t.Helper()
	require.NoError(f.t, a.ValidateBasic())
	// Keep SDK expiration later than the scope interval so refusals at NotAfter
	// prove the scope's runtime check rather than SDK grant cleanup.
	expiration := time.UnixMilli(int64(a.NotAfter) + 10000).UTC()
	msg, err := authz.NewMsgGrant(f.granter, f.grantee, &a, expiration)
	require.NoError(f.t, err)
	decoded := f.signed(msg, f.granterKey)
	grant, ok := decoded.(*authz.MsgGrant)
	require.True(f.t, ok, "SDK decoder returned %T", decoded)
	storedAuth, ok := grant.GetAuthorization().(*types.LayerXScopeAuthorization)
	require.True(f.t, ok)
	require.True(f.t, proto.Equal(&a, storedAuth))
	handler := f.router.Handler(decoded)
	require.NotNil(f.t, handler)
	result, err := handler(f.ctx, decoded)
	require.NoError(f.t, err)
	require.NotNil(f.t, result)
	require.True(f.t, proto.Equal(&a, f.stored()))
	_, storedExpiration := f.keeper.GetCleanAuthorization(f.ctx, f.grantee, f.granter, a.MsgTypeUrl)
	require.Equal(f.t, expiration, storedExpiration)
}

func (f *repairM1422Chain) exec(ctx sdk.Context, msgs ...sdk.Msg) error {
	f.t.Helper()
	msg := authz.NewMsgExec(f.grantee, msgs)
	decoded := f.signed(&msg, f.granteeKey)
	exec, ok := decoded.(*authz.MsgExec)
	require.True(f.t, ok, "SDK decoder returned %T", decoded)
	payments, err := exec.GetMessages()
	require.NoError(f.t, err)
	require.Len(f.t, payments, len(msgs))
	for i, payment := range payments {
		require.IsType(f.t, msgs[i], payment)
		switch expected := msgs[i].(type) {
		case *banktypes.MsgSend:
			actual := payment.(*banktypes.MsgSend)
			require.Equal(f.t, expected.FromAddress, actual.FromAddress)
			require.Equal(f.t, expected.ToAddress, actual.ToAddress)
			require.True(f.t, expected.Amount.IsEqual(actual.Amount), "decoded payment amount differs")
		case *banktypes.MsgMultiSend:
			actual := payment.(*banktypes.MsgMultiSend)
			require.Len(f.t, actual.Inputs, len(expected.Inputs))
			require.Len(f.t, actual.Outputs, len(expected.Outputs))
			for j, input := range expected.Inputs {
				require.Equal(f.t, input.Address, actual.Inputs[j].Address)
				require.True(f.t, input.Coins.IsEqual(actual.Inputs[j].Coins), "decoded input coins differ")
			}
			for j, output := range expected.Outputs {
				require.Equal(f.t, output.Address, actual.Outputs[j].Address)
				require.True(f.t, output.Coins.IsEqual(actual.Outputs[j].Coins), "decoded output coins differ")
			}
		default:
			require.FailNow(f.t, "unsupported payment comparison", "message type %T", expected)
		}
		_, wrapped := payment.(types.ScopedActivity)
		require.False(f.t, wrapped, "decoded production payment must not use the test scope wrapper")
		require.NoError(f.t, payment.ValidateBasic())
	}
	handler := f.router.Handler(decoded)
	require.NotNil(f.t, handler)
	cache, write := ctx.CacheContext()
	result, err := handler(cache, decoded)
	if err == nil {
		require.NotNil(f.t, result)
		write()
	}
	return err
}

func (f *repairM1422Chain) liveContext() sdk.Context {
	return types.WithGrantorRevocation(f.ctx, f.granter, f.authorization().RevocationEpoch)
}

func (f *repairM1422Chain) assertSpending(total, period string, start uint64, transferred int64) {
	f.t.Helper()
	stored := f.stored()
	require.Equal(f.t, total, stored.SpentTotal)
	require.Equal(f.t, period, stored.SpentThisPeriod)
	require.Equal(f.t, start, stored.PeriodStart)
	denom := ledgertypes.MirrorDenom(f.asset)
	require.True(f.t, sdk.NewInt(transferred).Equal(f.bank.GetBalance(f.ctx, f.recipient, denom).Amount), "recipient balance differs")
	require.True(f.t, sdk.NewInt(1000-transferred).Equal(f.bank.GetBalance(f.ctx, f.granter, denom).Amount), "granter balance differs")
}

func TestRepairM_1422_RuntimeRefusal(t *testing.T) {
	cases := []struct {
		name   string
		change func(*repairM1422Chain, *types.LayerXScopeAuthorization, *banktypes.MsgSend)
		offset int64
		want   error
		reason string
	}{
		{name: "inclusive_not_before"},
		{name: "last_valid_millisecond", offset: 999},
		{name: "before_validity", offset: -1, want: types.ErrAuthScope, reason: "outside validity interval"},
		{name: "exclusive_not_after", offset: 1000, want: types.ErrAuthScope, reason: "outside validity interval"},
		{name: "module", change: func(_ *repairM1422Chain, a *types.LayerXScopeAuthorization, _ *banktypes.MsgSend) {
			a.ModuleMask = 1 << 2
		}, want: types.ErrAuthScope, reason: "outside module mask"},
		{name: "ordinal", change: func(_ *repairM1422Chain, a *types.LayerXScopeAuthorization, _ *banktypes.MsgSend) {
			a.OrdinalMin++
			a.OrdinalMax++
		}, want: types.ErrAuthScope, reason: "outside"},
		{name: "asset", change: func(_ *repairM1422Chain, a *types.LayerXScopeAuthorization, _ *banktypes.MsgSend) {
			a.AssetId = bytes.Repeat([]byte{0x24}, 32)
		}, want: types.ErrAuthScope, reason: "activity asset differs"},
		{name: "per_activity", change: func(_ *repairM1422Chain, a *types.LayerXScopeAuthorization, _ *banktypes.MsgSend) {
			a.MaxPerActivity = "4"
		}, want: types.ErrAuthScope, reason: "above max per activity"},
		{name: "purpose_amount", change: func(f *repairM1422Chain, a *types.LayerXScopeAuthorization, msg *banktypes.MsgSend) {
			purpose, err := types.BankSendPurpose(msg)
			require.NoError(f.t, err)
			a.PurposeHash = purpose[:]
			msg.Amount = f.send(6).Amount
		}, want: types.ErrAuthScope, reason: "activity purpose differs"},
		{name: "purpose_recipient", change: func(f *repairM1422Chain, a *types.LayerXScopeAuthorization, msg *banktypes.MsgSend) {
			purpose, err := types.BankSendPurpose(msg)
			require.NoError(f.t, err)
			a.PurposeHash = purpose[:]
			msg.ToAddress = sdk.AccAddress(bytes.Repeat([]byte{0x25}, 20)).String()
		}, want: types.ErrAuthScope, reason: "activity purpose differs"},
		{name: "unmapped_denomination", change: func(_ *repairM1422Chain, _ *types.LayerXScopeAuthorization, msg *banktypes.MsgSend) {
			msg.Amount = sdk.NewCoins(sdk.NewInt64Coin("upax", 5))
		}, want: types.ErrAuthScope, reason: "no canonical kernel asset"},
		{name: "multiple_assets", change: func(_ *repairM1422Chain, _ *types.LayerXScopeAuthorization, msg *banktypes.MsgSend) {
			msg.Amount = msg.Amount.Add(sdk.NewInt64Coin("upax", 1))
		}, want: types.ErrAuthScope, reason: "requires one asset"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRepairM1422Chain(t)
			a := f.authorization()
			msg := f.send(5)
			if tc.change != nil {
				tc.change(f, &a, msg)
			}
			f.grant(a)
			before := proto.Clone(f.stored())
			f.ctx = f.ctx.WithBlockTime(time.UnixMilli(repair193Now + tc.offset))
			err := f.exec(f.liveContext(), msg)
			if tc.want == nil {
				require.NoError(t, err)
				f.assertSpending("5", "", 0, 5)
				return
			}
			require.ErrorIs(t, err, tc.want)
			require.ErrorContains(t, err, tc.reason)
			require.True(t, proto.Equal(before, f.stored()), "refused runtime action changed stored authorization")
			f.assertSpending("", "", 0, 0)
			recipient, err := sdk.AccAddressFromBech32(msg.ToAddress)
			require.NoError(t, err)
			for _, coin := range msg.Amount {
				require.True(t, f.bank.GetBalance(f.ctx, recipient, coin.Denom).IsZero())
			}
		})
	}
}

func TestRepairM_1422_PersistedSpendingAndRollover(t *testing.T) {
	f := newRepairM1422Chain(t)
	a := f.authorization()
	a.PeriodLength = 100
	a.MaxPerPeriod = "10"
	f.grant(a)
	require.NoError(t, f.exec(f.liveContext(), f.send(6)))
	f.assertSpending("6", "6", a.NotBefore, 6)
	require.ErrorIs(t, f.exec(f.liveContext(), f.send(5)), types.ErrAuthScope)
	f.assertSpending("6", "6", a.NotBefore, 6)
	require.NoError(t, f.exec(f.liveContext(), f.send(4)))
	f.assertSpending("10", "10", a.NotBefore, 10)
	f.reopen()
	f.assertSpending("10", "10", a.NotBefore, 10)
	f.ctx = f.ctx.WithBlockTime(time.UnixMilli(repair193Now + 99))
	require.ErrorIs(t, f.exec(f.liveContext(), f.send(1)), types.ErrAuthScope)
	f.assertSpending("10", "10", a.NotBefore, 10)
	f.ctx = f.ctx.WithBlockTime(time.UnixMilli(repair193Now + 100))
	require.NoError(t, f.exec(f.liveContext(), f.send(6)))
	f.assertSpending("16", "6", a.NotBefore+100, 16)
	require.NoError(t, f.exec(f.liveContext(), f.send(4)))
	f.assertSpending("20", "10", a.NotBefore+100, 20)
	f.ctx = f.ctx.WithBlockTime(time.UnixMilli(repair193Now + 300))
	err := f.exec(f.liveContext(), f.send(1))
	require.ErrorIs(t, err, types.ErrAuthScope)
	require.ErrorContains(t, err, "above max total")
	f.assertSpending("20", "10", a.NotBefore+100, 20)

	t.Run("whole_elapsed_periods", func(t *testing.T) {
		f := newRepairM1422Chain(t)
		a := f.authorization()
		a.PeriodLength = 100
		a.MaxPerPeriod = "10"
		f.grant(a)
		require.NoError(t, f.exec(f.liveContext(), f.send(7)))
		f.ctx = f.ctx.WithBlockTime(time.UnixMilli(repair193Now + 350))
		require.NoError(t, f.exec(f.liveContext(), f.send(8)))
		f.assertSpending("15", "8", a.NotBefore+300, 15)
		f.reopen()
		f.ctx = f.ctx.WithBlockTime(time.UnixMilli(repair193Now + 399))
		f.assertSpending("15", "8", a.NotBefore+300, 15)
		require.ErrorIs(t, f.exec(f.liveContext(), f.send(3)), types.ErrAuthScope)
		f.assertSpending("15", "8", a.NotBefore+300, 15)
	})
}

func TestRepairM_1422_PurposeRoutingAndAtomicity(t *testing.T) {
	f := newRepairM1422Chain(t)
	a := f.authorization()
	a.PeriodLength = 100
	a.MaxPerPeriod = "10"
	purpose, err := types.BankSendPurpose(f.send(6))
	require.NoError(t, err)
	a.PurposeHash = purpose[:]
	f.grant(a)
	before := proto.Clone(f.stored())
	// Both decoded actions enter one real SDK transaction cache. The second
	// exceeds the period budget and must roll back the first transfer and charge.
	require.ErrorIs(t, f.exec(f.liveContext(), f.send(6), f.send(6)), types.ErrAuthScope)
	require.True(t, proto.Equal(before, f.stored()))
	f.assertSpending("", "", 0, 0)
	require.NoError(t, f.exec(f.liveContext(), f.send(6)))
	f.assertSpending("6", "6", a.NotBefore, 6)

	coins := f.send(1).Amount
	multi := banktypes.NewMsgMultiSend(
		[]banktypes.Input{banktypes.NewInput(f.granter, coins)},
		[]banktypes.Output{banktypes.NewOutput(f.recipient, coins)},
	)
	require.ErrorIs(t, f.exec(f.liveContext(), multi), sdkerrors.ErrUnauthorized)
	f.assertSpending("6", "6", a.NotBefore, 6)
	revoked := types.WithGrantorRevocation(f.ctx, f.granter, a.RevocationEpoch+1)
	require.ErrorIs(t, f.exec(revoked, f.send(6)), types.ErrAuthRevoked)
	require.ErrorIs(t, f.exec(f.ctx, f.send(6)), types.ErrAuthRevoked)
	f.assertSpending("6", "6", a.NotBefore, 6)

	t.Run("bank_refusal_rolls_back_spending", func(t *testing.T) {
		f := newRepairM1422Chain(t)
		a := f.authorization()
		a.MaxPerActivity = "2000"
		a.MaxTotal = "3000"
		a.PeriodLength = 100
		a.MaxPerPeriod = "3000"
		f.grant(a)
		before := proto.Clone(f.stored())
		require.ErrorIs(t, f.exec(f.liveContext(), f.send(1500)), sdkerrors.ErrInsufficientFunds)
		require.True(t, proto.Equal(before, f.stored()))
		f.assertSpending("", "", 0, 0)
	})
}
