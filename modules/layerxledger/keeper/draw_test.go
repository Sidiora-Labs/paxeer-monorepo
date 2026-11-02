package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
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
		From: f.from, Asset: f.asset, Amount: sdk.NewInt(15), GrantID: sha256.Sum256([]byte("retired-payer-grant")),
		ReceiverSequence: 0, IdempotencyKey: sha256.Sum256([]byte("keeper-draw-0")),
		ContextHash: contextHash,
	}
	return f
}

func TestDrawRefusesWithoutPayerGrants(t *testing.T) {
	f := newDrawFixture(t)
	grantIDs := [][32]byte{f.args.GrantID, sha256.Sum256([]byte("unknown-payer-grant"))}
	require.NotEqual(t, grantIDs[0], grantIDs[1])
	balances := map[[32]byte]sdk.Int{
		f.issuance: sdk.NewInt(750),
		f.from:     sdk.NewInt(250),
		f.to:       sdk.ZeroInt(),
	}
	assertBalances := func() {
		t.Helper()
		for account, want := range balances {
			balance, err := f.host.ReadBalance(f.ctx, account, f.asset)
			require.NoError(t, err)
			require.Equal(t, want, balance)
			viewBalance, err := f.keeper.BalanceOf(f.ctx, account, f.asset)
			require.NoError(t, err)
			require.Equal(t, balance, viewBalance)
			require.Equal(t, balance, f.bank.GetBalance(f.ctx, lxphost.BankAddress(account), types.MirrorDenom(f.asset)).Amount)
		}
	}
	assertBalances()
	hostBefore := ledgerViewStore(t, f.ctx, f.hostKey)
	ledgerBefore := ledgerViewStore(t, f.ctx, f.storeKey)
	for _, grantID := range grantIDs {
		args := f.args
		args.GrantID = grantID
		result, err := f.keeper.Draw(f.ctx, f.payee, args)
		require.ErrorIs(t, err, types.ErrNoPayerGrant)
		require.Equal(t, ledgerkeeper.Result{}, result)
		require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.hostKey))
		require.Equal(t, ledgerBefore, ledgerViewStore(t, f.ctx, f.storeKey))
		assertBalances()
	}

	require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{Paused: true}))
	_, err := f.keeper.Draw(f.ctx, ledgerkeeper.Actor{}, f.args)
	require.ErrorIs(t, err, ledgerkeeper.ErrPausedScope)
}
