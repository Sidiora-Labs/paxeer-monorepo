// Proves: lxphost.Host and keeper.FromKernel -> layerxledger.RevertError and common.Precompile.Run -> abi.UnpackRevert and state.DBImpl.GetPrecompileError.
//
// The consumer is this test: the error contract lists no external caller yet.
// Registration crosses the real keeper and native kernel, then the host is
// closed and recreated over the retained SDK store before consumer reads and
// duplicate/refusal checks. Both registered ledger errors and raw kernel codes
// cross the exported Go and Solidity Error(string) surfaces.
//
// Fix: precompiles/layerxledger/errors.go RevertError used to wrap an already
// converted error again when Execute received it from a handler. Conversion is
// now idempotent, preserving the direct original cause for Go consumers.
package layerxledger_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestProofLayerxledgerErrorsRestart(t *testing.T) {
	h := newAssetHarness(t)
	assetID, salt, actor := registerTestAsset(t, h, false)
	other, _ := bindAssetHolder(t, h)
	wantAsset := assetView(t, h, "getAsset", assetID)
	require.Equal(t, assetID, wantAsset[0])
	require.Equal(t, "Token", wantAsset[2])
	require.Equal(t, false, wantAsset[10])
	wantStore := storeSnapshot(t, h.ctx.KVStore(h.hostKey))
	wantSequence, err := lxphost.NextSequence(h.ctx, h.hostKey)
	require.NoError(t, err)
	require.Equal(t, uint64(2), wantSequence)

	// Stop the actual native component and discard both producer and consumer
	// objects. Only the SDK stores survive; no snapshot is copied into a fake.
	h.host.Close()
	_, err = h.host.ReadAsset(h.ctx, assetID)
	require.Error(t, err)
	h.keeper, h.keepers.ledger, h.precompile = nil, nil, nil
	h.host, err = lxphost.NewHost(lxphost.Config{
		StoreKey: h.hostKey, NetworkID: 125, Epoch: 1, ParameterVersion: 1,
		TimestampWindowMs: 60_000, GasLimit: 10_000_000,
	})
	require.NoError(t, err)
	t.Cleanup(h.host.Close)
	a := testkeeper.EVMTestApp
	auth := authkeeper.NewAccountKeeper(a.AppCodec(), a.GetKey(authtypes.StoreKey),
		a.GetSubspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{types.ModuleName: {authtypes.Minter}})
	bank := bankkeeper.NewBaseKeeper(a.AppCodec(), a.GetKey(banktypes.StoreKey), auth,
		a.GetSubspace(banktypes.ModuleName), nil)
	h.keeper = ledgerkeeper.NewKeeper(a.AppCodec(), h.storeKey, h.host, bank, &a.EvmKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String())
	h.keepers.ledger = h.keeper
	h.precompile, err = layerxledger.NewPrecompile(h.keepers)
	require.NoError(t, err)
	h.stateDB = state.NewDBImpl(h.ctx.WithEventManager(sdk.NewEventManager()), &a.EvmKeeper, true)
	h.ctx = h.stateDB.Ctx()
	h.evm = &vm.EVM{StateDB: h.stateDB, TxContext: vm.TxContext{Origin: h.caller}}
	require.Equal(t, wantAsset, assetView(t, h, "getAsset", assetID))
	require.Nil(t, h.stateDB.GetPrecompileError())
	restoredActor, err := h.keeper.Actor(h.ctx, h.caller)
	require.NoError(t, err)
	require.Equal(t, actor, restoredActor)

	// This real duplicate receipt exercises the contract's FromKernel export.
	branch, _ := h.ctx.CacheContext()
	duplicate, producerErr := h.keeper.RegisterAsset(branch, restoredActor, types.RegisterArgs{
		AssetID: assetID, Salt: salt, Symbol: "TKN", Name: "Token", Decimals: 6,
		SupplyCap: sdk.NewInt(1000), IssuerKind: types.AssetIssuerNative,
	}, [32]byte{})
	require.ErrorIs(t, producerErr, types.ErrAssetAlreadyRegistered)
	require.Equal(t, lxphost.Code(-411), duplicate.Receipt.ResultCode)
	require.ErrorIs(t, ledgerkeeper.FromKernel(duplicate.Receipt.ResultCode), types.ErrAssetAlreadyRegistered)
	converted := layerxledger.RevertError(producerErr)
	require.Equal(t, converted, layerxledger.RevertError(converted))
	require.Same(t, producerErr, errors.Unwrap(converted))
	require.Same(t, producerErr, converted.(interface{ Cause() error }).Cause())
	reason, err := abi.UnpackRevert(pcommon.LXPReason(converted))
	require.NoError(t, err)
	require.Equal(t, "LXP_ASSET_ALREADY_REGISTERED", reason)

	for _, refusal := range []struct {
		name   string
		caller common.Address
		method string
		args   []interface{}
		cause  error
		reason string
	}{
		{"duplicate", h.caller, "register", []interface{}{assetID, salt, "TKN", "Token", uint8(6), big.NewInt(1000), uint8(types.AssetIssuerNative), []byte{}}, types.ErrAssetAlreadyRegistered, "LXP_ASSET_ALREADY_REGISTERED"},
		{"wrong signer", other, "pause", []interface{}{assetID}, types.ErrUnauthorizedDebit, "LXP_UNAUTHORIZED_DEBIT"},
		{"unknown asset", h.caller, "getAsset", []interface{}{[32]byte{}}, types.ErrAssetMismatch, "LXP_ASSET_MISMATCH"},
		{"unknown account", h.caller, "accountOwner", []interface{}{[32]byte{}}, lxphost.Code(-208), "LXP_UNKNOWN_ACCOUNT_NAMESPACE"},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			beforeLogs := len(h.stateDB.GetAllLogs())
			beforeEvents := len(h.ctx.EventManager().Events())
			snapshot := h.stateDB.Snapshot()
			data, callErr := assetCall(t, h, refusal.caller, refusal.method, refusal.args...)
			require.ErrorIs(t, callErr, vm.ErrExecutionReverted)
			require.GreaterOrEqual(t, len(data), 4)
			require.Equal(t, []byte{0x08, 0xc3, 0x79, 0xa0}, data[:4])
			observed, decodeErr := abi.UnpackRevert(data)
			require.NoError(t, decodeErr)
			require.Equal(t, refusal.reason, observed)
			goErr := h.stateDB.GetPrecompileError()
			require.ErrorIs(t, goErr, refusal.cause)
			// Handler -> Execute must leave exactly one conversion wrapper.
			require.Equal(t, refusal.cause, errors.Unwrap(goErr))
			require.Equal(t, refusal.cause, goErr.(interface{ Cause() error }).Cause())
			require.Equal(t, goErr, layerxledger.RevertError(goErr))
			require.Equal(t, data, pcommon.LXPReason(goErr))
			if refusal.method == "accountOwner" {
				var code lxphost.Code
				require.ErrorAs(t, goErr, &code)
				require.Equal(t, lxphost.Code(-208), code)
				require.Equal(t, observed, layerxledger.KernelCodeText(code))
			} else {
				var registered *sdkerrors.Error
				require.ErrorAs(t, goErr, &registered)
				require.Same(t, refusal.cause, registered)
				require.Equal(t, types.ModuleName, registered.Codespace())
			}
			// The EVM consumer rolls back a reverted call through the real
			// StateDB; kernel refusal receipts can advance host metadata.
			h.stateDB.RevertToSnapshot(snapshot)
			h.ctx = h.stateDB.Ctx()
			require.Len(t, h.stateDB.GetAllLogs(), beforeLogs)
			require.Len(t, h.ctx.EventManager().Events(), beforeEvents)
			require.Equal(t, wantStore, storeSnapshot(t, h.ctx.KVStore(h.hostKey)))
			require.Equal(t, wantAsset, assetView(t, h, "getAsset", assetID))
		})
	}
	next, err := lxphost.NextSequence(h.ctx, h.hostKey)
	require.NoError(t, err)
	require.Equal(t, wantSequence, next)
	_, issuance, err := lxphost.IssuanceAccountID(assetID)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(1000), bank.GetSupply(h.ctx, types.MirrorDenom(assetID)).Amount)
	require.Equal(t, sdk.NewInt(1000), bank.GetBalance(h.ctx, lxphost.BankAddress(issuance), types.MirrorDenom(assetID)).Amount)
	require.Nil(t, layerxledger.RevertError(nil))
	require.Nil(t, ledgerkeeper.FromKernel(0))
}
