package keeper_test

import (
	"crypto/ed25519"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type supplyFixture struct {
	*mirrorFixture
	issuer, holder               ledgerkeeper.Actor
	issuerAccount, holderAccount [32]byte
}

func newSupplyFixture(t *testing.T) *supplyFixture {
	t.Helper()
	f := &supplyFixture{mirrorFixture: newMirrorFixture(t, nil)}
	f.ctx = f.ctx.WithChainID("hyperpax_125-1")
	registered := f.register(t, sdk.NewInt(1000))
	require.NoError(t, f.keeper.Mirror(f.ctx, 0x00010001, registered))
	holder := newLedgerViewActor(t, 103)
	app := testkeeper.EVMTestApp
	bind := func(actor ledgerViewActor, caller common.Address) ledgerkeeper.Actor {
		t.Helper()
		message, err := evmtypes.LayerXBindMessage(app.EvmKeeper.ChainID(f.ctx), caller, 0)
		require.NoError(t, err)
		var signature [ed25519.SignatureSize]byte
		copy(signature[:], ed25519.Sign(actor.private, message))
		_, err = app.EvmKeeper.BindLayerX(f.ctx, caller, actor.key, signature)
		require.NoError(t, err)
		bound, err := f.keeper.Actor(f.ctx, caller)
		require.NoError(t, err)
		require.Equal(t, actor.id, bound.Authority.Actor)
		return bound
	}
	f.issuer = bind(f.actor, common.Address{0x24, 0x02, 1})
	f.holder = bind(holder, common.Address{0x24, 0x02, 2})
	f.execute(t, f.actor, 0x00010004, 1, types.EncodeOpenAccountPayload(f.asset))
	f.execute(t, holder, 0x00010004, 0, types.EncodeOpenAccountPayload(f.asset))
	var err error
	_, f.issuerAccount, err = lxphost.AssetAccountID(f.actor.key, f.asset)
	require.NoError(t, err)
	_, f.holderAccount, err = lxphost.AssetAccountID(holder.key, f.asset)
	require.NoError(t, err)
	return f
}

func (f *supplyFixture) assertUnits(t *testing.T, issuer, holder, total int64) {
	t.Helper()
	denom := types.MirrorDenom(f.asset)
	for account, want := range map[[32]byte]int64{
		f.issuerAccount: issuer, f.holderAccount: holder, f.issuance: 1000 - total,
	} {
		balance, err := f.host.ReadBalance(f.ctx, account, f.asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(want), balance)
		require.Equal(t, balance, f.bank.GetBalance(f.ctx, lxphost.BankAddress(account), denom).Amount)
	}
	units, err := f.host.ReadTotalUnits(f.ctx, f.asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(total), units)
	require.Equal(t, sdk.NewInt(1000), f.bank.GetSupply(f.ctx, denom).Amount)
}

func TestMintWithinCap(t *testing.T) {
	f := newSupplyFixture(t)
	f.assertUnits(t, 0, 0, 0)
	first, err := f.keeper.Mint(f.ctx, f.issuer, f.asset, f.holderAccount, sdk.NewInt(600), [32]byte{0x51})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), first.Receipt.ResultCode)
	require.NotEmpty(t, first.Receipt.Encoded)
	require.Empty(t, first.Events)
	require.Equal(t, first.Receipt.Events, first.Events)
	events, err := ledgerkeeper.LedgerEvents(0x0001000a, first.Receipt, ledgerkeeper.EventArgs{})
	require.NoError(t, err)
	require.Equal(t, []ledgerkeeper.Event{{Name: "Transfer", Fields: map[string]any{
		"from": f.issuance, "to": f.holderAccount, "assetId": f.asset,
		"amount": sdk.NewInt(600).BigInt(), "kind": uint8(10),
	}}}, events)
	require.Len(t, first.Receipt.Transfers, 1)
	require.Equal(t, f.issuance, first.Receipt.Transfers[0].From)
	require.Equal(t, f.holderAccount, first.Receipt.Transfers[0].To)
	require.Equal(t, f.asset, first.Receipt.Transfers[0].Asset)
	require.Equal(t, sdk.NewInt(600), first.Receipt.Transfers[0].Amount)
	f.assertUnits(t, 0, 600, 600)
	refused, err := f.keeper.Mint(f.ctx, f.issuer, f.asset, f.holderAccount, sdk.NewInt(500), [32]byte{0x52})
	require.ErrorIs(t, err, types.ErrInsufficientBalance)
	require.Equal(t, lxphost.Code(-400), refused.Receipt.ResultCode)
	require.ErrorIs(t, err, ledgerkeeper.FromKernel(refused.Receipt.ResultCode))
	require.Equal(t, first.Receipt.GlobalSequence+1, refused.Receipt.GlobalSequence)
	f.assertUnits(t, 0, 600, 600)
	sequence, err := f.host.ReadIdentitySequence(f.ctx, f.issuer.DID)
	require.NoError(t, err)
	second, err := f.keeper.Mint(f.ctx, f.issuer, f.asset, f.issuerAccount, sdk.NewInt(400), [32]byte{0x51})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), second.Receipt.ResultCode)
	require.Equal(t, refused.Receipt.GlobalSequence+1, second.Receipt.GlobalSequence)
	require.Empty(t, second.Events)
	nextSequence, err := f.host.ReadIdentitySequence(f.ctx, f.issuer.DID)
	require.NoError(t, err)
	require.Equal(t, sequence+1, nextSequence)
	f.assertUnits(t, 400, 600, 1000)
	refused, err = f.keeper.Mint(f.ctx, f.issuer, f.asset, f.holderAccount, sdk.OneInt(), [32]byte{0x53})
	require.ErrorIs(t, err, types.ErrInsufficientBalance)
	require.Equal(t, lxphost.Code(-400), refused.Receipt.ResultCode)
	require.ErrorIs(t, err, ledgerkeeper.FromKernel(refused.Receipt.ResultCode))
	f.assertUnits(t, 400, 600, 1000)
	refused, err = f.keeper.Mint(f.ctx, f.issuer, f.asset, f.holderAccount, sdk.NewInt(600), [32]byte{0x51})
	require.ErrorIs(t, err, types.ErrInsufficientBalance)
	require.Equal(t, lxphost.Code(-400), refused.Receipt.ResultCode)
	require.ErrorIs(t, err, ledgerkeeper.FromKernel(refused.Receipt.ResultCode))
	f.assertUnits(t, 400, 600, 1000)
	require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{Paused: true}))
	before := ledgerViewStore(t, f.ctx, f.hostKey)
	_, err = f.keeper.Mint(f.ctx, f.issuer, f.asset, f.holderAccount, sdk.ZeroInt(), [32]byte{0x54})
	require.ErrorIs(t, err, ledgerkeeper.ErrPausedScope)
	require.Contains(t, err.Error(), "layerxledger.mint")
	require.Equal(t, before, ledgerViewStore(t, f.ctx, f.hostKey))
	f.assertUnits(t, 400, 600, 1000)
}

func TestBurnReducesBalance(t *testing.T) {
	f := newSupplyFixture(t)
	_, err := f.keeper.Mint(f.ctx, f.issuer, f.asset, f.holderAccount, sdk.NewInt(80), [32]byte{0x61})
	require.NoError(t, err)
	_, err = f.keeper.Mint(f.ctx, f.issuer, f.asset, f.issuerAccount, sdk.NewInt(50), [32]byte{0x62})
	require.NoError(t, err)
	f.assertUnits(t, 50, 80, 130)
	_, err = f.keeper.Burn(f.ctx, f.issuer, f.asset, f.holderAccount, sdk.OneInt(), [32]byte{0x63})
	require.ErrorIs(t, err, types.ErrUnauthorizedDebit)
	f.assertUnits(t, 50, 80, 130)
	burned, err := f.keeper.Burn(f.ctx, f.holder, f.asset, f.holderAccount, sdk.NewInt(30), [32]byte{0x64})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), burned.Receipt.ResultCode)
	require.NotEmpty(t, burned.Receipt.Encoded)
	require.Empty(t, burned.Events)
	require.Equal(t, burned.Receipt.Events, burned.Events)
	events, err := ledgerkeeper.LedgerEvents(0x0001000b, burned.Receipt, ledgerkeeper.EventArgs{})
	require.NoError(t, err)
	require.Equal(t, []ledgerkeeper.Event{{Name: "Transfer", Fields: map[string]any{
		"from": f.holderAccount, "to": f.issuance, "assetId": f.asset,
		"amount": sdk.NewInt(30).BigInt(), "kind": uint8(11),
	}}}, events)
	require.Len(t, burned.Receipt.Transfers, 1)
	require.Equal(t, f.holderAccount, burned.Receipt.Transfers[0].From)
	require.Equal(t, f.issuance, burned.Receipt.Transfers[0].To)
	require.Equal(t, f.asset, burned.Receipt.Transfers[0].Asset)
	require.Equal(t, sdk.NewInt(30), burned.Receipt.Transfers[0].Amount)
	f.assertUnits(t, 50, 50, 100)
	refused, err := f.keeper.Burn(f.ctx, f.holder, f.asset, f.holderAccount, sdk.NewInt(51), [32]byte{0x65})
	require.ErrorIs(t, err, types.ErrInsufficientBalance)
	require.Equal(t, lxphost.Code(-400), refused.Receipt.ResultCode)
	require.ErrorIs(t, err, ledgerkeeper.FromKernel(refused.Receipt.ResultCode))
	f.assertUnits(t, 50, 50, 100)
	_, err = f.keeper.Burn(f.ctx, f.holder, f.asset, f.holderAccount, sdk.NewInt(50), [32]byte{0x66})
	require.NoError(t, err)
	f.assertUnits(t, 50, 0, 50)
	require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{Paused: true}))
	before := ledgerViewStore(t, f.ctx, f.hostKey)
	_, err = f.keeper.Burn(f.ctx, f.issuer, f.asset, f.issuerAccount, sdk.ZeroInt(), [32]byte{0x67})
	require.ErrorIs(t, err, ledgerkeeper.ErrPausedScope)
	require.Contains(t, err.Error(), "layerxledger.burn")
	require.Equal(t, before, ledgerViewStore(t, f.ctx, f.hostKey))
	f.assertUnits(t, 50, 0, 50)
}
