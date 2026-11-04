package keeper_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/erc1155"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/erc20"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/erc721"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	receiptstore "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	wasmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1242_013_FeeConservationAndRecovery(t *testing.T) {
	proveOperatorFeeSettlement(t)
}

func TestRepairM_1242_010_RegisteredDelegateCall(t *testing.T) {
	proveRegisteredDelegateCall(t)
}

func TestRepairM_1242_014_PrecompileVersions(t *testing.T) {
	TestGetCustomPrecompiles(t)
}

func TestRepairM_1242_018_ReceiptRetryContract(t *testing.T) {
	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	k := &a.EvmKeeper
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(1)
	hash := common.HexToHash("0x124218")
	for _, attempts := range []int{0, -1, -200} {
		t.Run(fmt.Sprint(attempts), func(t *testing.T) {
			r, err := k.GetReceiptWithRetry(ctx, hash, attempts)
			require.Nil(t, r)
			require.ErrorContains(t, err, "attempts must be positive")
		})
	}
	r, err := k.GetReceiptWithRetry(ctx, hash, 1)
	require.Nil(t, r)
	require.ErrorIs(t, err, receiptstore.ErrNotFound)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	r, err = k.GetReceiptWithRetry(ctx.WithContext(cancelled), hash, 3)
	require.Nil(t, r)
	require.ErrorIs(t, err, context.Canceled)
	deadline, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	started := time.Now()
	r, err = k.GetReceiptWithRetry(ctx.WithContext(deadline), hash, 100)
	require.Nil(t, r)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(started), 200*time.Millisecond)
	// Use the real receipt producer, flush and persistent consumer for success.
	produced := &types.Receipt{TxHashHex: hash.Hex(), BlockNumber: 1, Status: 1, GasUsed: 21000}
	require.NoError(t, k.SetTransientReceipt(ctx, hash, produced))
	require.NoError(t, k.FlushTransientReceipts(ctx))
	require.Eventually(t, func() bool {
		r, err = k.GetReceiptWithRetry(ctx, hash, 1)
		return err == nil
	}, 2*time.Second, 10*time.Millisecond)
	require.NotNil(t, r)
	require.Equal(t, produced.TxHashHex, r.TxHashHex)
	require.Equal(t, produced.Status, r.Status)
	require.Equal(t, produced.GasUsed, r.GasUsed)
	r, err = k.GetReceiptWithRetry(ctx.WithContext(cancelled), hash, 1)
	require.Nil(t, r)
	require.ErrorIs(t, err, context.Canceled)
}

func TestRepairM_1242_015_MetadataDecodeErrors(t *testing.T) {
	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	k := &a.EvmKeeper
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(1)
	address := common.HexToAddress("0x124215")
	for _, tc := range []struct{ name, code, query, message string }{
		{"empty", "0x00", "totalSupply", "unpack"},
		{"short", "0x602a60005360016000f3", "totalSupply", "unpack"},
		{"malformed-string", "0x602a60005260206000f3", "name", "unpack"},
		{"unknown-method", "0x00", "missingMetadataMethod", "pack"},
		{"missing-input", "0x00", "balanceOf", "pack"},
		{"revert", "0x60006000fd", "totalSupply", "execution reverted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k.SetCode(ctx, address, common.FromHex(tc.code))
			output, err := k.QueryERCSingleOutput(ctx, "native", address, tc.query)
			require.Nil(t, output)
			require.ErrorContains(t, err, tc.message)
		})
	}
	k.SetCode(ctx, address, common.FromHex("0x602a60005260206000f3"))
	output, err := k.QueryERCSingleOutput(ctx, "native", address, "totalSupply")
	require.NoError(t, err)
	require.Equal(t, "42", fmt.Sprint(output))
}

func TestRepairM_1242_022_GenesisInstallationsAreAtomic(t *testing.T) {
	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	k := &a.EvmKeeper
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(1)
	codes := []struct {
		name string
		kind types.PointerType
		bin  []byte
	}{
		{"CWERC20", types.PointerType_ERC20, erc20.GetBin()},
		{"CWERC721", types.PointerType_ERC721, erc721.GetBin()},
		{"CWERC1155", types.PointerType_ERC1155, erc1155.GetBin()},
	}
	// Start each initialization with no usable pointer-code registry.
	pointerStore := k.PrefixStore(ctx, types.PointerCWCodePrefix)
	iter := pointerStore.Iterator(nil, nil)
	var keys [][]byte
	for ; iter.Valid(); iter.Next() {
		keys = append(keys, append([]byte(nil), iter.Key()...))
	}
	require.NoError(t, iter.Close())
	for _, key := range keys {
		pointerStore.Delete(key)
	}
	genesis := *types.DefaultGenesis()
	genesis.Params.MinimumFeePerGas = sdk.NewDec(60_000_000)
	require.NoError(t, genesis.Params.Validate())
	association := sdk.AccAddress(common.HexToAddress("0x124222").Bytes())
	associatedEVM := common.HexToAddress("0x124223")
	genesis.AddressAssociations = append(genesis.AddressAssociations, &types.AddressAssociation{PaxAddress: association.String(), EthAddress: associatedEVM.Hex()})
	beforeParams := k.GetParams(ctx)
	nextCodeID := a.WasmKeeper.PeekAutoIncrementID(ctx, wasmtypes.KeyLastCodeID)
	assertUnpublished := func() {
		require.Equal(t, beforeParams, k.GetParams(ctx))
		_, exists := k.GetEVMAddress(ctx, association)
		require.False(t, exists)
		require.Equal(t, nextCodeID, a.WasmKeeper.PeekAutoIncrementID(ctx, wasmtypes.KeyLastCodeID))
		for _, code := range codes {
			require.Zero(t, k.GetStoredPointerCodeID(ctx, code.kind))
		}
	}
	// A real authorization error exercises the returned-error branch, preserving its cause.
	t.Run("upload-error", func(t *testing.T) {
		params := a.WasmKeeper.GetParams(ctx)
		blocked := params
		blocked.CodeUploadAccess = wasmtypes.AllowNobody
		a.WasmKeeper.SetParams(ctx, blocked)
		original := *k.WasmKeeper()
		*k.WasmKeeper() = *wasmkeeper.NewDefaultPermissionKeeper(&a.WasmKeeper)
		defer func() { *k.WasmKeeper() = original; a.WasmKeeper.SetParams(ctx, params) }()
		var failure interface{}
		func() { defer func() { failure = recover() }(); k.InitGenesis(ctx, genesis) }()
		err, ok := failure.(error)
		require.True(t, ok, "expected initialization error, got %v", failure)
		require.ErrorContains(t, err, "install CWERC20 pointer code")
		require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
		assertUnpublished()
	})
	// Calibrate real upload costs in discarded caches to exhaust gas at each
	// installation, including after earlier code uploads have already succeeded.
	uploadCosts := make([]uint64, len(codes))
	for i, code := range codes {
		probe, _ := ctx.CacheContext()
		probe = probe.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
		_, err := k.WasmKeeper().Create(probe, k.AccountKeeper().GetModuleAddress(types.ModuleName), code.bin, nil)
		require.NoError(t, err)
		uploadCosts[i] = probe.GasMeter().GasConsumed()
	}
	probe, _ := ctx.CacheContext()
	probe = probe.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	k.InitGenesis(probe, genesis)
	total := probe.GasMeter().GasConsumed()
	var precedingConsumption uint64
	for i, code := range codes {
		t.Run(code.name+"-interrupted", func(t *testing.T) {
			remaining := uint64(0)
			for _, cost := range uploadCosts[i:] {
				remaining += cost
			}
			require.Greater(t, total, remaining)
			limited := ctx.WithGasMeter(sdk.NewGasMeter(total-remaining, 1, 1))
			var failure interface{}
			func() { defer func() { failure = recover() }(); k.InitGenesis(limited, genesis) }()
			outOfGas, ok := failure.(sdk.ErrorOutOfGas)
			require.True(t, ok, "expected upload gas exhaustion, got %v", failure)
			require.Equal(t, "Compiling WASM Bytecode", outOfGas.Descriptor)
			require.Greater(t, limited.GasMeter().GasConsumed(), precedingConsumption)
			precedingConsumption = limited.GasMeter().GasConsumed()
			assertUnpublished()
			registrationCtx, _ := ctx.CacheContext()
			_, err := keeper.NewMsgServerImpl(k).RegisterPointer(sdk.WrapSDKContext(registrationCtx), &types.MsgRegisterPointer{Sender: association.String(), PointerType: code.kind, ErcAddress: associatedEVM.Hex()})
			require.Error(t, err, "failed initialization cannot serve pointer registration")
		})
	}
	// The successful producer installs all IDs; RegisterPointer consumes each
	// ID through the real wasm instantiator, and the registry reports that code.
	k.InitGenesis(ctx, genesis)
	for i, code := range codes {
		t.Run(code.name+"-registered", func(t *testing.T) {
			codeID := k.GetStoredPointerCodeID(ctx, code.kind)
			require.Equal(t, nextCodeID+uint64(i), codeID)
			require.NotNil(t, a.WasmKeeper.GetCodeInfo(ctx, codeID))
			res, err := keeper.NewMsgServerImpl(k).RegisterPointer(sdk.WrapSDKContext(ctx), &types.MsgRegisterPointer{Sender: association.String(), PointerType: code.kind, ErcAddress: associatedEVM.Hex()})
			require.NoError(t, err)
			pointer, err := sdk.AccAddressFromBech32(res.PointerAddress)
			require.NoError(t, err)
			info := a.WasmKeeper.GetContractInfo(ctx, pointer)
			require.NotNil(t, info)
			require.Equal(t, codeID, info.CodeID)
		})
	}
}
