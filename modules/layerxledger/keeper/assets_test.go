package keeper_test

import (
	"bytes"
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

func bindAssetActor(t *testing.T, h *harness, signer ledgerViewActor, caller common.Address) ledgerkeeper.Actor {
	t.Helper()
	h.ctx = h.ctx.WithChainID("hyperpax_125-1")
	app := testkeeper.EVMTestApp
	message, err := evmtypes.LayerXBindMessage(app.EvmKeeper.ChainID(h.ctx), caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(signer.private, message))
	_, err = app.EvmKeeper.BindLayerX(h.ctx, caller, signer.key, signature)
	require.NoError(t, err)
	actor, err := h.keeper.Actor(h.ctx, caller)
	require.NoError(t, err)
	require.Equal(t, signer.id, actor.Authority.Actor)
	return actor
}

func newAssetKeeperFixture(t *testing.T) (*mirrorFixture, ledgerkeeper.Actor, types.RegisterArgs) {
	t.Helper()
	f := newMirrorFixture(t, nil)
	actor := bindAssetActor(t, f.harness, f.actor, common.Address{0x24, 0x01})
	return f, actor, types.RegisterArgs{
		AssetID: f.asset, Salt: [32]byte(bytes.Repeat([]byte{0x41}, 32)),
		Symbol: "TKN", Name: "Token", Decimals: 6,
		SupplyCap: sdk.NewInt(1000), IssuerKind: types.AssetIssuerNative, CustodyReference: []byte{},
	}
}

func TestRegisterAsset(t *testing.T) {
	for _, custody := range []bool{false, true} {
		name := "native"
		if custody {
			name = "custody"
		}
		t.Run(name, func(t *testing.T) {
			f, actor, args := newAssetKeeperFixture(t)
			if custody {
				args.IssuerKind = types.AssetIssuerPaxeerCustody
				args.CustodyReference = []byte("reserve")
			}
			key := [32]byte{1}
			result, err := f.keeper.RegisterAsset(f.ctx, actor, args, key)
			require.NoError(t, err)
			require.Equal(t, lxphost.Code(0), result.Receipt.ResultCode)
			require.Equal(t, uint64(1), result.Receipt.GlobalSequence)
			require.NotEmpty(t, result.Receipt.Encoded)
			require.Equal(t, result.Receipt.Events, result.Events)
			require.Contains(t, result.Events, lxphost.ReceiptEvent{Code: 1, Body: args.AssetID[:]})
			asset, err := f.host.ReadAsset(f.ctx, args.AssetID)
			require.NoError(t, err)
			require.Equal(t, args.AssetID, asset.AssetID)
			require.Equal(t, actor.Authority.Actor, asset.IssuerDID)
			require.Equal(t, args.Salt, asset.Salt)
			require.Equal(t, args.Symbol, asset.Symbol)
			require.Equal(t, args.Name, asset.Name)
			require.Equal(t, args.Decimals, asset.Decimals)
			require.Equal(t, args.SupplyCap, asset.SupplyCap)
			require.Equal(t, args.IssuerKind, asset.IssuerKind)
			require.Equal(t, args.CustodyReference, asset.CustodyReference)
			require.False(t, asset.Paused)
			require.True(t, asset.TotalUnits.IsZero())
			denom := types.MirrorDenom(args.AssetID)
			require.Equal(t, args.SupplyCap, f.bank.GetSupply(f.ctx, denom).Amount)
			require.Equal(t, args.SupplyCap, f.bank.GetBalance(f.ctx, lxphost.BankAddress(f.issuance), denom).Amount)
			stored, err := f.host.ReadIdempotentReceipt(f.ctx, actor.DID, key)
			require.NoError(t, err)
			require.Equal(t, stored.Encoded, result.Receipt.Encoded)
			before := ledgerViewStore(t, f.ctx, f.hostKey)
			replay, err := f.keeper.RegisterAsset(f.ctx, actor, args, key)
			require.ErrorIs(t, err, types.ErrIdempotentReplay)
			require.Equal(t, stored, replay.Receipt)
			require.Equal(t, before, ledgerViewStore(t, f.ctx, f.hostKey))
			require.Equal(t, args.SupplyCap, f.bank.GetSupply(f.ctx, denom).Amount)
		})
	}
	t.Run("invalid_payload", func(t *testing.T) {
		f, actor, args := newAssetKeeperFixture(t)
		args.Symbol = ""
		before := ledgerViewStore(t, f.ctx, f.hostKey)
		_, err := f.keeper.RegisterAsset(f.ctx, actor, args, [32]byte{1})
		require.ErrorIs(t, err, types.ErrNonCanonical)
		require.Equal(t, before, ledgerViewStore(t, f.ctx, f.hostKey))
		require.True(t, f.bank.GetSupply(f.ctx, types.MirrorDenom(args.AssetID)).IsZero())
	})
}

func TestRegisterAssetTwiceRefused(t *testing.T) {
	f, actor, args := newAssetKeeperFixture(t)
	_, err := f.keeper.RegisterAsset(f.ctx, actor, args, [32]byte{1})
	require.NoError(t, err)
	original, err := f.host.ReadAsset(f.ctx, args.AssetID)
	require.NoError(t, err)
	args.Name = "Replacement"
	args.SupplyCap = sdk.NewInt(2000)
	result, err := f.keeper.RegisterAsset(f.ctx, actor, args, [32]byte{2})
	require.ErrorIs(t, err, types.ErrAssetAlreadyRegistered)
	require.Equal(t, lxphost.Code(-411), result.Receipt.ResultCode)
	asset, err := f.host.ReadAsset(f.ctx, args.AssetID)
	require.NoError(t, err)
	require.Equal(t, original, asset)
	denom := types.MirrorDenom(args.AssetID)
	require.Equal(t, original.SupplyCap, f.bank.GetSupply(f.ctx, denom).Amount)
	require.Equal(t, original.SupplyCap, f.bank.GetBalance(f.ctx, lxphost.BankAddress(f.issuance), denom).Amount)
}

func TestPauseUnpauseAsset(t *testing.T) {
	f, issuer, args := newAssetKeeperFixture(t)
	other := bindAssetActor(t, f.harness, newLedgerViewActor(t, 102), common.Address{0x24, 0x02})
	_, err := f.keeper.RegisterAsset(f.ctx, issuer, args, [32]byte{1})
	require.NoError(t, err)
	for _, operation := range []struct {
		name   string
		call   func(sdk.Context, ledgerkeeper.Actor, [32]byte, [32]byte) (ledgerkeeper.Result, error)
		paused bool
		code   uint16
	}{
		{"pause", f.keeper.PauseAsset, true, 2},
		{"unpause", f.keeper.UnpauseAsset, false, 3},
	} {
		t.Run(operation.name, func(t *testing.T) {
			result, err := operation.call(f.ctx, other, args.AssetID, [32]byte{byte(operation.code), 1})
			require.ErrorIs(t, err, types.ErrUnauthorizedDebit)
			require.Equal(t, lxphost.Code(-207), result.Receipt.ResultCode)
			asset, err := f.host.ReadAsset(f.ctx, args.AssetID)
			require.NoError(t, err)
			require.Equal(t, !operation.paused, asset.Paused)
			result, err = operation.call(f.ctx, issuer, args.AssetID, [32]byte{byte(operation.code), 2})
			require.NoError(t, err)
			require.Equal(t, lxphost.Code(0), result.Receipt.ResultCode)
			require.Equal(t, result.Receipt.Events, result.Events)
			require.Contains(t, result.Events, lxphost.ReceiptEvent{Code: operation.code, Body: args.AssetID[:]})
			asset, err = f.host.ReadAsset(f.ctx, args.AssetID)
			require.NoError(t, err)
			require.Equal(t, operation.paused, asset.Paused)
			require.Equal(t, issuer.Authority.Actor, asset.IssuerDID)
		})
	}
	denom := types.MirrorDenom(args.AssetID)
	require.Equal(t, args.SupplyCap, f.bank.GetSupply(f.ctx, denom).Amount)
	require.Equal(t, args.SupplyCap, f.bank.GetBalance(f.ctx, lxphost.BankAddress(f.issuance), denom).Amount)
	t.Run("module_pause_first", func(t *testing.T) {
		require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{Paused: true}))
		before := ledgerViewStore(t, f.ctx, f.hostKey)
		_, err := f.keeper.RegisterAsset(f.ctx, ledgerkeeper.Actor{}, types.RegisterArgs{}, [32]byte{9})
		require.ErrorIs(t, err, ledgerkeeper.ErrPausedScope)
		require.Contains(t, err.Error(), "layerxledger.register")
		_, err = f.keeper.PauseAsset(f.ctx, ledgerkeeper.Actor{}, [32]byte{}, [32]byte{10})
		require.ErrorIs(t, err, ledgerkeeper.ErrPausedScope)
		require.Contains(t, err.Error(), "layerxledger.pause")
		_, err = f.keeper.UnpauseAsset(f.ctx, ledgerkeeper.Actor{}, [32]byte{}, [32]byte{11})
		require.ErrorIs(t, err, ledgerkeeper.ErrPausedScope)
		require.Contains(t, err.Error(), "layerxledger.unpause")
		require.Equal(t, before, ledgerViewStore(t, f.ctx, f.hostKey))
	})
}
