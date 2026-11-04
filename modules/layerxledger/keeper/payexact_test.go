package keeper_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func payExactFixture(t *testing.T) (*mirrorFixture, ledgerkeeper.Actor, types.PayExactArgs) {
	t.Helper()
	fixture := newFundedSendFixture(t, 25)
	f, actor := fixture.mirrorFixture, fixture.actor
	from, to := fixture.from, fixture.to
	return f, actor, types.PayExactArgs{
		From: from, To: to, Asset: f.asset, Amount: sdk.NewInt(3),
		ExpiresAt:        uint64(f.ctx.BlockTime().UnixMilli()) + 1000,
		PaymentReference: sha256.Sum256([]byte("exact-payment-reference")),
	}
}

func TestPayExact(t *testing.T) {
	f, actor, args := payExactFixture(t)
	denom := types.MirrorDenom(args.Asset)
	assertBalance := func(account [32]byte, want int64) {
		t.Helper()
		balance, err := f.keeper.BalanceOf(f.ctx, account, args.Asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(want), balance)
		require.Equal(t, balance, f.bank.GetBalance(f.ctx, lxphost.BankAddress(account), denom).Amount)
	}
	for i := uint64(0); i < 3; i++ {
		if i == 1 {
			args.PaymentReference = sha256.Sum256([]byte("next-exact-payment"))
		}
		if i == 2 {
			args.PaymentReference = sha256.Sum256([]byte("second-exact-payment"))
		}
		sequence, err := f.keeper.NextSequence(f.ctx, actor.DIDKey, args.Asset)
		require.NoError(t, err)
		require.Equal(t, i, sequence)
		eventsBefore := len(f.ctx.EventManager().Events())
		result, err := f.keeper.PayExact(f.ctx, actor, args)
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), result.Receipt.ResultCode)
		require.Equal(t, result.Receipt.Events, result.Events)
		require.Equal(t, f.global+1+i, result.Receipt.GlobalSequence)
		require.Len(t, result.Receipt.Transfers, 1)
		require.Equal(t, lxphost.TransferLeg{Asset: args.Asset, From: args.From, To: args.To, Amount: args.Amount}, result.Receipt.Transfers[0])
		material := append([]byte("LXP/v1/context-hash\x00"), args.From[:]...)
		material = append(material, args.To[:]...)
		material = append(material, args.Asset[:]...)
		amount := make([]byte, 16)
		args.Amount.BigInt().FillBytes(amount)
		material = append(material, amount...)
		material = append(material, args.PaymentReference[:]...)
		contextHash := sha256.Sum256(material)
		paymentEvents := 0
		for _, event := range f.ctx.EventManager().Events()[eventsBefore:] {
			if event.Type != "PaymentExact" {
				continue
			}
			paymentEvents++
			fields := map[string]string{}
			for _, attribute := range event.Attributes {
				fields[string(attribute.Key)] = string(attribute.Value)
			}
			require.Equal(t, hex.EncodeToString(args.From[:]), fields["from"])
			require.Equal(t, hex.EncodeToString(args.To[:]), fields["to"])
			require.Equal(t, hex.EncodeToString(args.Asset[:]), fields["assetId"])
			require.Equal(t, args.Amount.String(), fields["amount"])
			require.Equal(t, hex.EncodeToString(args.PaymentReference[:]), fields["paymentReference"])
			require.Equal(t, hex.EncodeToString(contextHash[:]), fields["contextHash"])
			require.Len(t, fields, 6)
		}
		require.Equal(t, 1, paymentEvents)
		sequence, err = f.keeper.NextSequence(f.ctx, actor.DIDKey, args.Asset)
		require.NoError(t, err)
		require.Equal(t, i+1, sequence)
		assertBalance(args.From, 25-3*int64(i+1))
		assertBalance(args.To, 3*int64(i+1))

		hostBefore := ledgerViewStore(t, f.ctx, f.hostKey)
		bankKey := testkeeper.EVMTestApp.GetKey(banktypes.StoreKey)
		bankBefore := ledgerViewStore(t, f.ctx, bankKey)
		eventsBefore = len(f.ctx.EventManager().Events())
		replayed, err := f.keeper.PayExact(f.ctx, actor, args)
		require.ErrorIs(t, err, types.ErrIdempotentReplay)
		require.Equal(t, ledgerkeeper.Result{}, replayed)
		require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.hostKey))
		require.Equal(t, bankBefore, ledgerViewStore(t, f.ctx, bankKey))
		require.Len(t, f.ctx.EventManager().Events(), eventsBefore)
		sequence, err = f.keeper.NextSequence(f.ctx, actor.DIDKey, args.Asset)
		require.NoError(t, err)
		require.Equal(t, i+1, sequence)
		assertBalance(args.From, 25-3*int64(i+1))
		assertBalance(args.To, 3*int64(i+1))
	}
	require.Equal(t, sdk.NewInt(1000), f.bank.GetSupply(f.ctx, denom).Amount)

	t.Run("actor_authority", func(t *testing.T) {
		hostBefore := ledgerViewStore(t, f.ctx, f.hostKey)
		bankKey := testkeeper.EVMTestApp.GetKey(banktypes.StoreKey)
		bankBefore := ledgerViewStore(t, f.ctx, bankKey)
		eventsBefore := len(f.ctx.EventManager().Events())
		for _, invalid := range []ledgerkeeper.Actor{
			{},
			{DIDKey: [32]byte{}, DID: actor.DID, Authority: actor.Authority},
		} {
			result, err := f.keeper.PayExact(f.ctx, invalid, args)
			require.ErrorIs(t, err, types.ErrUnauthorizedDebit)
			require.Equal(t, ledgerkeeper.Result{}, result)
			require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.hostKey))
			require.Equal(t, bankBefore, ledgerViewStore(t, f.ctx, bankKey))
			require.Len(t, f.ctx.EventManager().Events(), eventsBefore)
		}
	})

	t.Run("pause_checked_first", func(t *testing.T) {
		require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{Paused: true}))
		hostBefore := ledgerViewStore(t, f.ctx, f.hostKey)
		_, err := f.keeper.PayExact(f.ctx, actor, types.PayExactArgs{})
		require.ErrorIs(t, err, ledgerkeeper.ErrPausedScope)
		require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.hostKey))
	})

	t.Run("mirror_failure_is_atomic", func(t *testing.T) {
		f, actor, args := payExactFixture(t)
		denom := types.MirrorDenom(args.Asset)
		require.NoError(t, f.bank.SendCoins(f.ctx, lxphost.BankAddress(args.From),
			lxphost.BankAddress(args.To), sdk.NewCoins(sdk.NewInt64Coin(denom, 25))))
		hostBefore := ledgerViewStore(t, f.ctx, f.hostKey)
		bankKey := testkeeper.EVMTestApp.GetKey(banktypes.StoreKey)
		bankBefore := ledgerViewStore(t, f.ctx, bankKey)
		eventsBefore := len(f.ctx.EventManager().Events())
		_, err := f.keeper.PayExact(f.ctx, actor, args)
		require.ErrorIs(t, err, types.ErrFatalSupplyMismatch)
		require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.hostKey))
		require.Equal(t, bankBefore, ledgerViewStore(t, f.ctx, bankKey))
		require.Len(t, f.ctx.EventManager().Events(), eventsBefore)
	})
}

func TestPayExactExpired(t *testing.T) {
	f, actor, args := payExactFixture(t)
	now := uint64(f.ctx.BlockTime().UnixMilli())
	bankKey := testkeeper.EVMTestApp.GetKey(banktypes.StoreKey)
	hostBefore := ledgerViewStore(t, f.ctx, f.hostKey)
	ledgerBefore := ledgerViewStore(t, f.ctx, f.storeKey)
	bankBefore := ledgerViewStore(t, f.ctx, bankKey)
	eventsBefore := len(f.ctx.EventManager().Events())
	for _, expiresAt := range []uint64{now, now - 1, 0} {
		args.ExpiresAt = expiresAt
		result, err := f.keeper.PayExact(f.ctx, actor, args)
		require.ErrorIs(t, err, types.ErrExpired)
		require.Equal(t, ledgerkeeper.Result{}, result)
		require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.hostKey))
		require.Equal(t, ledgerBefore, ledgerViewStore(t, f.ctx, f.storeKey))
		require.Equal(t, bankBefore, ledgerViewStore(t, f.ctx, bankKey))
		require.Len(t, f.ctx.EventManager().Events(), eventsBefore)
		from, err := f.keeper.BalanceOf(f.ctx, args.From, args.Asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(25), from)
		to, err := f.keeper.BalanceOf(f.ctx, args.To, args.Asset)
		require.NoError(t, err)
		require.True(t, to.IsZero())
	}
}

func TestPayExactNextSequenceKeyReuse(t *testing.T) {
	f, actor, args := payExactFixture(t)
	first, err := f.keeper.PayExact(f.ctx, actor, args)
	require.NoError(t, err)
	sequence, err := f.keeper.NextSequenceByAccountID(f.ctx, args.From, args.Asset)
	require.NoError(t, err)
	require.Equal(t, uint64(1), sequence)
	hostBefore := ledgerViewStore(t, f.ctx, f.hostKey)
	bankKey := testkeeper.EVMTestApp.GetKey(banktypes.StoreKey)
	bankBefore := ledgerViewStore(t, f.ctx, bankKey)
	eventsBefore := len(f.ctx.EventManager().Events())
	second, err := f.keeper.PayExact(f.ctx, actor, args)
	require.ErrorIs(t, err, types.ErrIdempotentReplay)
	require.Equal(t, ledgerkeeper.Result{}, second)
	require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.hostKey))
	require.Equal(t, bankBefore, ledgerViewStore(t, f.ctx, bankKey))
	require.Len(t, f.ctx.EventManager().Events(), eventsBefore)
	sequence, err = f.keeper.NextSequenceByAccountID(f.ctx, args.From, args.Asset)
	require.NoError(t, err)
	require.Equal(t, uint64(1), sequence)
	args.PaymentReference = sha256.Sum256([]byte("next-sequence-payment-reference"))
	second, err = f.keeper.PayExact(f.ctx, actor, args)
	require.NoError(t, err)
	require.Equal(t, first.Receipt.GlobalSequence+1, second.Receipt.GlobalSequence)
	sequence, err = f.keeper.NextSequenceByAccountID(f.ctx, args.From, args.Asset)
	require.NoError(t, err)
	require.Equal(t, uint64(2), sequence)
	for _, item := range []struct {
		id     [32]byte
		amount int64
	}{{args.From, 19}, {args.To, 6}} {
		balance, err := f.keeper.BalanceOf(f.ctx, item.id, args.Asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(item.amount), balance)
		require.Equal(t, balance, f.bank.GetBalance(f.ctx, lxphost.BankAddress(item.id), types.MirrorDenom(args.Asset)).Amount)
	}
}
