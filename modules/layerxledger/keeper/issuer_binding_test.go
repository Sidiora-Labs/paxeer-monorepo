package keeper_test

import (
	"encoding/hex"
	"errors"
	"testing"

	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type issuerBindingCall struct {
	method   string
	denom    string
	creator  string
	subdenom string
	did      string
	paused   bool
}

type recordingLedgerIssuer struct {
	calls     []issuerBindingCall
	denom     string
	createErr error
	issuerErr error
	pauseErr  error
}

var _ types.IssuerKeeper = (*recordingLedgerIssuer)(nil)

func (k *recordingLedgerIssuer) CreateDenom(_ sdk.Context, creator, subdenom string) (string, error) {
	k.calls = append(k.calls, issuerBindingCall{method: "create", creator: creator, subdenom: subdenom})
	return k.denom, k.createErr
}

func (k *recordingLedgerIssuer) SetIssuerDid(_ sdk.Context, denom, did string) error {
	k.calls = append(k.calls, issuerBindingCall{method: "issuer", denom: denom, did: did})
	return k.issuerErr
}

func (k *recordingLedgerIssuer) SetDenomPausedByIssuer(_ sdk.Context, denom string, paused bool) error {
	k.calls = append(k.calls, issuerBindingCall{method: "pause", denom: denom, paused: paused})
	return k.pauseErr
}

func TestRegisterBindsTokenfactoryDenom(t *testing.T) {
	f, actor, args := newAssetKeeperFixture(t)
	issuer := &recordingLedgerIssuer{denom: "factory/ledger/bound-asset"}
	k := ledgerkeeper.NewIssuerBindingKeeper(f.keeper, issuer)
	invalid := args
	invalid.Symbol = ""
	before := ledgerViewStore(t, f.ctx, f.hostKey)
	_, err := k.RegisterAsset(f.ctx, actor, invalid, [32]byte{1})
	require.ErrorIs(t, err, types.ErrNonCanonical)
	require.Empty(t, issuer.calls)
	require.Equal(t, before, ledgerViewStore(t, f.ctx, f.hostKey))

	result, err := k.RegisterAsset(f.ctx, actor, args, [32]byte{2})
	require.NoError(t, err)
	require.Zero(t, result.Receipt.ResultCode)
	require.Equal(t, []issuerBindingCall{
		{method: "create", creator: authtypes.NewModuleAddress(types.ModuleName).String(), subdenom: hex.EncodeToString(args.AssetID[:])},
		{method: "issuer", denom: issuer.denom, did: string(actor.DID)},
	}, issuer.calls)
	asset, err := f.host.ReadAsset(f.ctx, args.AssetID)
	require.NoError(t, err)
	require.Equal(t, actor.Authority.Actor, asset.IssuerDID)

	_, err = k.RegisterAsset(f.ctx, actor, args, [32]byte{3})
	require.ErrorIs(t, err, types.ErrAssetAlreadyRegistered)
	require.Len(t, issuer.calls, 2)

	for _, stage := range []string{"create", "issuer"} {
		t.Run(stage+"_failure", func(t *testing.T) {
			f, actor, args := newAssetKeeperFixture(t)
			failure := errors.New("issuer binding refused")
			issuer := &recordingLedgerIssuer{denom: "factory/ledger/bound-asset"}
			if stage == "create" {
				issuer.createErr = failure
			} else {
				issuer.issuerErr = failure
			}
			k := ledgerkeeper.NewIssuerBindingKeeper(f.keeper, issuer)
			hostBefore := ledgerViewStore(t, f.ctx, f.hostKey)
			ledgerBefore := ledgerViewStore(t, f.ctx, f.storeKey)
			_, err := k.RegisterAsset(f.ctx, actor, args, [32]byte{1})
			require.ErrorIs(t, err, failure)
			require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.hostKey))
			require.Equal(t, ledgerBefore, ledgerViewStore(t, f.ctx, f.storeKey))
			require.True(t, f.bank.GetSupply(f.ctx, types.MirrorDenom(args.AssetID)).IsZero())
			if stage == "create" {
				require.Len(t, issuer.calls, 1)
			} else {
				require.Len(t, issuer.calls, 2)
			}
		})
	}

	t.Run("tokenfactory_subdenom_contract", func(t *testing.T) {
		_, err := tokenfactorytypes.GetTokenDenom(authtypes.NewModuleAddress(types.ModuleName).String(), hex.EncodeToString(args.AssetID[:]))
		require.ErrorIs(t, err, tokenfactorytypes.ErrSubdenomTooLong)
	})
}

func TestLedgerPauseMirrorsToDenom(t *testing.T) {
	f, actor, args := newAssetKeeperFixture(t)
	issuer := &recordingLedgerIssuer{denom: "factory/ledger/bound-asset"}
	k := ledgerkeeper.NewIssuerBindingKeeper(f.keeper, issuer)
	_, err := k.RegisterAsset(f.ctx, actor, args, [32]byte{1})
	require.NoError(t, err)
	other := bindAssetActor(t, f.harness, newLedgerViewActor(t, 102), common.Address{0x24, 0x02})

	k = ledgerkeeper.NewIssuerBindingKeeper(f.keeper, issuer)
	for _, operation := range []struct {
		name   string
		call   func(sdk.Context, ledgerkeeper.Actor, [32]byte, [32]byte) (ledgerkeeper.Result, error)
		paused bool
		key    byte
	}{
		{"pause", k.PauseAsset, true, 2},
		{"unpause", k.UnpauseAsset, false, 3},
	} {
		t.Run(operation.name, func(t *testing.T) {
			before := len(issuer.calls)
			_, err := operation.call(f.ctx, other, args.AssetID, [32]byte{operation.key, 1})
			require.ErrorIs(t, err, types.ErrUnauthorizedDebit)
			require.Len(t, issuer.calls, before)

			failure := errors.New("pause mirror refused")
			issuer.pauseErr = failure
			hostBefore := ledgerViewStore(t, f.ctx, f.hostKey)
			_, err = operation.call(f.ctx, actor, args.AssetID, [32]byte{operation.key, 2})
			require.ErrorIs(t, err, failure)
			require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.hostKey))
			asset, err := f.host.ReadAsset(f.ctx, args.AssetID)
			require.NoError(t, err)
			require.Equal(t, !operation.paused, asset.Paused)

			issuer.pauseErr = nil
			_, err = operation.call(f.ctx, actor, args.AssetID, [32]byte{operation.key, 3})
			require.NoError(t, err)
			require.Equal(t, issuerBindingCall{method: "pause", denom: issuer.denom, paused: operation.paused}, issuer.calls[len(issuer.calls)-1])
			asset, err = f.host.ReadAsset(f.ctx, args.AssetID)
			require.NoError(t, err)
			require.Equal(t, operation.paused, asset.Paused)
		})
	}
	require.Equal(t, []issuerBindingCall{
		{method: "pause", denom: issuer.denom, paused: true},
		{method: "pause", denom: issuer.denom, paused: true},
		{method: "pause", denom: issuer.denom, paused: false},
		{method: "pause", denom: issuer.denom, paused: false},
	}, issuer.calls[2:])
}
