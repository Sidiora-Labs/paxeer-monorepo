package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type drawFixture struct {
	*mirrorFixture
	payee    ledgerkeeper.Actor
	from, to [32]byte
	args     types.DrawArgs
}

func newDrawFixture(t *testing.T) *drawFixture {
	t.Helper()
	f := &drawFixture{mirrorFixture: newMirrorFixture(t, nil)}
	f.ctx = f.ctx.WithChainID("hyperpax_125-1")
	registered := f.register(t, sdk.NewInt(1000))
	require.NoError(t, f.keeper.Mirror(f.ctx, 0x00010001, registered))
	recipient := newLedgerViewActor(t, 103)
	var err error
	_, f.from, err = lxphost.AssetAccountID(f.actor.key, f.asset)
	require.NoError(t, err)
	_, f.to, err = lxphost.AssetAccountID(recipient.key, f.asset)
	require.NoError(t, err)
	f.execute(t, f.actor, 0x00010004, 1, types.EncodeOpenAccountPayload(f.asset))
	f.execute(t, recipient, 0x00010004, 0, types.EncodeOpenAccountPayload(f.asset))
	mint, err := types.EncodeSupplyPayload(f.asset, f.from, sdk.NewInt(250))
	require.NoError(t, err)
	minted := f.execute(t, f.actor, 0x0001000a, 2, mint)
	require.NoError(t, f.keeper.Mirror(f.ctx, 0x0001000a, minted))
	grantFixture := ledgerViewFixture{
		harness: f.harness, actor: f.actor, recipient: recipient,
		asset: f.asset, account: f.from, to: f.to,
	}
	grant, grantID := grantFixture.payerGrant(t)
	f.execute(t, f.actor, 0x00010007, 3, grant)

	app := testkeeper.EVMTestApp
	caller := common.HexToAddress("0x2470000000000000000000000000000000000001")
	message, err := evmtypes.LayerXBindMessage(app.EvmKeeper.ChainID(f.ctx), caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(recipient.private, message))
	_, err = app.EvmKeeper.BindLayerX(f.ctx, caller, recipient.key, signature)
	require.NoError(t, err)
	f.payee, err = f.keeper.Actor(f.ctx, caller)
	require.NoError(t, err)
	require.Equal(t, recipient.key, f.payee.DIDKey)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, bytes.Repeat([]byte{0x22}, 32))
	require.NoError(t, err)
	f.args = types.DrawArgs{
		From: f.from, Asset: f.asset, Amount: sdk.NewInt(15), GrantID: grantID,
		ReceiverSequence: 0, IdempotencyKey: sha256.Sum256([]byte("keeper-draw-0")),
		ContextHash: contextHash,
	}
	return f
}

func TestDrawWithinGrant(t *testing.T) {
	f := newDrawFixture(t)
	before, err := f.keeper.GetPayerGrant(f.ctx, f.args.GrantID)
	require.NoError(t, err)
	require.Equal(t, sdk.ZeroInt(), before.DrawnTotal)
	allowance := sdk.NewIntFromBigInt(new(big.Int).SetBytes(before.Grant[144:160]))
	require.Equal(t, sdk.NewInt(100), allowance)
	for draw := uint64(0); draw < 2; draw++ {
		args := f.args
		args.ReceiverSequence = draw
		args.IdempotencyKey = sha256.Sum256(binary.BigEndian.AppendUint64([]byte("keeper-draw"), draw))
		result, err := f.keeper.Draw(f.ctx, f.payee, args)
		require.NoError(t, err, "receive result code: %v", result.Receipt.ResultCode)
		require.Equal(t, lxphost.Code(0), result.Receipt.ResultCode)
		require.Equal(t, f.global+draw+1, result.Receipt.GlobalSequence)
		require.NotEmpty(t, result.Receipt.Encoded)
		require.Equal(t, []lxphost.TransferLeg{{
			Asset: f.asset, From: f.from, To: f.to, Amount: args.Amount,
		}}, result.Receipt.Transfers)
		require.Equal(t, result.Receipt.Events, result.Events)
		events, err := ledgerkeeper.LedgerEvents(0x00010006, result.Receipt, ledgerkeeper.EventArgs{
			GrantID: args.GrantID, IdempotencyKey: args.IdempotencyKey, ContextHash: args.ContextHash,
		})
		require.NoError(t, err)
		require.Equal(t, []ledgerkeeper.Event{
			{Name: "Transfer", Fields: map[string]any{
				"from": f.from, "to": f.to, "assetId": f.asset,
				"amount": args.Amount.BigInt(), "kind": uint8(6),
			}},
			{Name: "PaymentDrawn", Fields: map[string]any{
				"grantId": args.GrantID, "from": f.from, "to": f.to,
				"assetId": f.asset, "amount": args.Amount.BigInt(),
				"idempotencyKey": args.IdempotencyKey, "contextHash": args.ContextHash,
			}},
		}, events)
		after, err := f.keeper.GetPayerGrant(f.ctx, args.GrantID)
		require.NoError(t, err)
		require.Equal(t, before.Grant, after.Grant)
		require.Equal(t, before.DrawnTotal.Add(args.Amount), after.DrawnTotal)
		require.Equal(t, allowance.Sub(before.DrawnTotal).Sub(args.Amount), allowance.Sub(after.DrawnTotal))
		before = after
		for _, account := range [][32]byte{f.from, f.to} {
			balance, err := f.keeper.BalanceOf(f.ctx, account, f.asset)
			require.NoError(t, err)
			want := sdk.NewInt(15 * int64(draw+1))
			if account == f.from {
				want = sdk.NewInt(250).Sub(want)
			}
			require.Equal(t, want, balance)
			require.Equal(t, balance, f.bank.GetBalance(f.ctx, lxphost.BankAddress(account), types.MirrorDenom(f.asset)).Amount)
		}
		next, err := f.keeper.NextSequence(f.ctx, f.payee.DIDKey, f.asset)
		require.NoError(t, err)
		require.Equal(t, draw+1, next)
	}
}

func TestDrawWithoutGrant(t *testing.T) {
	f := newDrawFixture(t)
	args := f.args
	args.GrantID = sha256.Sum256([]byte("unknown-payer-grant"))
	hostBefore := ledgerViewStore(t, f.ctx, f.hostKey)
	ledgerBefore := ledgerViewStore(t, f.ctx, f.storeKey)
	result, err := f.keeper.Draw(f.ctx, f.payee, args)
	require.ErrorIs(t, err, types.ErrNoPayerGrant)
	require.Equal(t, ledgerkeeper.Result{}, result)
	require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.hostKey))
	require.Equal(t, ledgerBefore, ledgerViewStore(t, f.ctx, f.storeKey))
	require.Equal(t, sdk.NewInt(250), f.bank.GetBalance(f.ctx, lxphost.BankAddress(f.from), types.MirrorDenom(f.asset)).Amount)
	require.True(t, f.bank.GetBalance(f.ctx, lxphost.BankAddress(f.to), types.MirrorDenom(f.asset)).IsZero())
	grant, err := f.keeper.GetPayerGrant(f.ctx, f.args.GrantID)
	require.NoError(t, err)
	require.Equal(t, sdk.ZeroInt(), grant.DrawnTotal)

	require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{Paused: true}))
	_, err = f.keeper.Draw(f.ctx, ledgerkeeper.Actor{}, args)
	require.ErrorIs(t, err, ledgerkeeper.ErrPausedScope)
}
