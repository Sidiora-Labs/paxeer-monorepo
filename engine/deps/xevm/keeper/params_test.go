package keeper_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/testutil/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"
)

func TestStorageGasReaderWithoutValue(t *testing.T) {
	k, ctx := newFeeAndReceiptKeeper(t)
	key := append([]byte(k.Paramstore.Name()+"/"), types.KeyPaxSstoreSetGasEIP2200...)
	ctx.KVStore(k.GetStoreKey()).Delete(key)
	require.False(t, k.Paramstore.Has(ctx, types.KeyPaxSstoreSetGasEIP2200))
	require.Zero(t, k.GetParams(ctx).PaxSstoreSetGasEip2200)
	require.Equal(t, uint64(20000), k.GetSstoreSetGasEIP2200(ctx))
}

func TestStorageGasReaderStoredValue(t *testing.T) {
	k, ctx := newFeeAndReceiptKeeper(t)
	for _, value := range []uint64{72000, 10000, 0} {
		t.Run(strconv.FormatUint(value, 10), func(t *testing.T) {
			k.Paramstore.Set(ctx, types.KeyPaxSstoreSetGasEIP2200, value)
			require.True(t, k.Paramstore.Has(ctx, types.KeyPaxSstoreSetGasEIP2200))
			want := value
			if want == 0 {
				want = 20000
			}
			require.Equal(t, want, k.GetSstoreSetGasEIP2200(ctx))
		})
	}
}

func TestStorageGasReaderMatchesOrdinaryConstant(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(source), "../../../../modules/evm/keeper/params.go"), nil, 0)
	require.NoError(t, err)
	var legacy uint64
	found := false
	for _, decl := range file.Decls {
		constants, ok := decl.(*ast.GenDecl)
		if !ok || constants.Tok != token.CONST {
			continue
		}
		for _, spec := range constants.Specs {
			value := spec.(*ast.ValueSpec)
			for i, name := range value.Names {
				if name.Name != "LegacySstoreSetGasEIP2200" {
					continue
				}
				cast, ok := value.Values[i].(*ast.CallExpr)
				require.True(t, ok)
				require.Equal(t, "uint64", cast.Fun.(*ast.Ident).Name)
				require.Len(t, cast.Args, 1)
				literal, ok := cast.Args[0].(*ast.BasicLit)
				require.True(t, ok)
				legacy, err = strconv.ParseUint(literal.Value, 0, 64)
				require.NoError(t, err)
				found = true
			}
		}
	}
	require.True(t, found)
	k, ctx := newFeeAndReceiptKeeper(t)
	ctx.KVStore(k.GetStoreKey()).Delete(append([]byte(k.Paramstore.Name()+"/"), types.KeyPaxSstoreSetGasEIP2200...))
	require.False(t, k.Paramstore.Has(ctx, types.KeyPaxSstoreSetGasEIP2200))
	require.Equal(t, legacy, k.GetSstoreSetGasEIP2200(ctx))
}

func TestParams(t *testing.T) {
	k, ctx := testkeeper.MockEVMKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())
	require.Equal(t, "uhpx", k.GetBaseDenom(ctx))
	require.Equal(t, types.DefaultPriorityNormalizer, k.GetPriorityNormalizer(ctx))
	require.Equal(t, types.DefaultMinFeePerGas, k.GetNextBaseFeePerGas(ctx))
	require.Equal(t, types.DefaultBaseFeePerGas, k.GetBaseFeePerGas(ctx))
	require.Equal(t, types.DefaultMinFeePerGas, k.GetMinimumFeePerGas(ctx))
	require.Equal(t, types.DefaultMaxFeePerGas, k.GetMaximumFeePerGas(ctx))
	require.True(t, k.GetMinimumFeePerGas(ctx).LTE(k.GetMaximumFeePerGas(ctx)))
	require.Equal(t, types.DefaultDeliverTxHookWasmGasLimit, k.GetDeliverTxHookWasmGasLimit(ctx))
	require.Equal(t, types.DefaultMaxDynamicBaseFeeUpwardAdjustment, k.GetMaxDynamicBaseFeeUpwardAdjustment(ctx))
	require.Equal(t, types.DefaultMaxDynamicBaseFeeDownwardAdjustment, k.GetMaxDynamicBaseFeeDownwardAdjustment(ctx))
	require.Nil(t, k.GetParams(ctx).Validate())
}

func TestGetParamsIfExists(t *testing.T) {
	k, ctx := testkeeper.MockEVMKeeper(t)
	ctx = ctx.WithBlockTime(time.Now())

	// Define the expected parameters
	expectedParams := types.Params{
		PriorityNormalizer: sdk.NewDec(1),
		BaseFeePerGas:      sdk.NewDec(1),
	}

	// Set only a subset of the parameters in the keeper
	k.Paramstore.Set(ctx, types.KeyPriorityNormalizer, expectedParams.PriorityNormalizer)
	k.Paramstore.Set(ctx, types.KeyBaseFeePerGas, expectedParams.BaseFeePerGas)

	// Retrieve the parameters using GetParamsIfExists
	params := k.GetParamsIfExists(ctx)

	// Assert that the retrieved parameters match the expected parameters
	require.Equal(t, expectedParams.PriorityNormalizer, params.PriorityNormalizer)
	require.Equal(t, expectedParams.BaseFeePerGas, params.BaseFeePerGas)

	// Assert that the missing parameter has its default value
	require.Equal(t, types.DefaultParams().DeliverTxHookWasmGasLimit, params.DeliverTxHookWasmGasLimit)
}

func TestParamGettersTracingVersions(t *testing.T) {
	k, baseCtx := testkeeper.MockEVMKeeper(t)

	// custom values to distinguish from defaults
	customBaseFee := sdk.NewDec(123456)
	customMinFee := sdk.NewDec(654321)
	customMaxFee := sdk.NewDec(987654)
	customUpward := sdk.NewDecWithPrec(123, 2)  // 1.23
	customDownward := sdk.NewDecWithPrec(45, 2) // 0.45
	customTargetGas := uint64(111111)
	customDeliverTxGasLimit := uint64(222222)
	customRegisterPointerDisabled := true

	// Populate Paramstore with custom values (these keys are shared across all versioned Param structs)
	k.Paramstore.Set(baseCtx, types.KeyBaseFeePerGas, customBaseFee)
	k.Paramstore.Set(baseCtx, types.KeyMinFeePerGas, customMinFee)
	k.Paramstore.Set(baseCtx, types.KeyMaxFeePerGas, customMaxFee)
	k.Paramstore.Set(baseCtx, types.KeyMaxDynamicBaseFeeUpwardAdjustment, customUpward)
	k.Paramstore.Set(baseCtx, types.KeyMaxDynamicBaseFeeDownwardAdjustment, customDownward)
	k.Paramstore.Set(baseCtx, types.KeyTargetGasUsedPerBlock, customTargetGas)
	k.Paramstore.Set(baseCtx, types.KeyDeliverTxHookWasmGasLimit, customDeliverTxGasLimit)
	k.Paramstore.Set(baseCtx, types.KeyRegisterPointerDisabled, customRegisterPointerDisabled)

	// ---- Pre-v5.8.0 (ParamsPreV580 path) ----
	ctxPre580 := baseCtx.WithIsTracing(true).WithClosestUpgradeName("v5.7.0")

	require.Equal(t, customBaseFee, k.GetBaseFeePerGas(ctxPre580))
	require.Equal(t, customMinFee, k.GetMinimumFeePerGas(ctxPre580))
	// Not supported pre-5.8.0, should fall back to defaults
	require.Equal(t, types.DefaultMaxDynamicBaseFeeUpwardAdjustment, k.GetMaxDynamicBaseFeeUpwardAdjustment(ctxPre580))
	require.Equal(t, types.DefaultMaxDynamicBaseFeeDownwardAdjustment, k.GetMaxDynamicBaseFeeDownwardAdjustment(ctxPre580))
	require.Equal(t, types.DefaultMaxFeePerGas, k.GetMaximumFeePerGas(ctxPre580))
	require.Equal(t, types.DefaultTargetGasUsedPerBlock, k.GetTargetGasUsedPerBlock(ctxPre580))
	require.Equal(t, types.DefaultDeliverTxHookWasmGasLimit, k.GetDeliverTxHookWasmGasLimit(ctxPre580))
	require.Equal(t, types.DefaultRegisterPointerDisabled, k.GetRegisterPointerDisabled(ctxPre580))

	// ---- Between v5.8.0 and v6.0.6 (ParamsPreV606 path) ----
	ctxPre606 := baseCtx.WithIsTracing(true).WithClosestUpgradeName("v6.0.5")

	require.Equal(t, customBaseFee, k.GetBaseFeePerGas(ctxPre606))
	require.Equal(t, customMinFee, k.GetMinimumFeePerGas(ctxPre606))
	require.Equal(t, customUpward, k.GetMaxDynamicBaseFeeUpwardAdjustment(ctxPre606))
	require.Equal(t, customDownward, k.GetMaxDynamicBaseFeeDownwardAdjustment(ctxPre606))
	require.Equal(t, customMaxFee, k.GetMaximumFeePerGas(ctxPre606))
	require.Equal(t, customTargetGas, k.GetTargetGasUsedPerBlock(ctxPre606))
	require.Equal(t, customDeliverTxGasLimit, k.GetDeliverTxHookWasmGasLimit(ctxPre606))
	// RegisterPointerDisabled is unavailable pre-6.0.6 → default
	require.Equal(t, types.DefaultRegisterPointerDisabled, k.GetRegisterPointerDisabled(ctxPre606))

	// ---- v6.0.6 and later (current Params path) ----
	ctxPost606 := baseCtx.WithIsTracing(true).WithClosestUpgradeName("v6.1.0")

	require.Equal(t, customBaseFee, k.GetBaseFeePerGas(ctxPost606))
	require.Equal(t, customMinFee, k.GetMinimumFeePerGas(ctxPost606))
	require.Equal(t, customUpward, k.GetMaxDynamicBaseFeeUpwardAdjustment(ctxPost606))
	require.Equal(t, customDownward, k.GetMaxDynamicBaseFeeDownwardAdjustment(ctxPost606))
	require.Equal(t, customMaxFee, k.GetMaximumFeePerGas(ctxPost606))
	require.Equal(t, customTargetGas, k.GetTargetGasUsedPerBlock(ctxPost606))
	require.Equal(t, customDeliverTxGasLimit, k.GetDeliverTxHookWasmGasLimit(ctxPost606))
	require.Equal(t, customRegisterPointerDisabled, k.GetRegisterPointerDisabled(ctxPost606))
}

func TestDecimalParamDefaults(t *testing.T) {
	k, ctx := newFeeAndReceiptKeeper(t)
	k.SetParams(ctx, types.DefaultParams())
	defaults := evmtypes.DefaultParams()
	for _, param := range []struct {
		key          []byte
		get          func(sdk.Context) sdk.Dec
		defaultValue sdk.Dec
		introduced   string
	}{
		{types.KeyPriorityNormalizer, k.GetPriorityNormalizer, defaults.PriorityNormalizer, "v0.0.0"},
		{types.KeyBaseFeePerGas, k.GetBaseFeePerGas, defaults.BaseFeePerGas, "v0.0.0"},
		{types.KeyMinFeePerGas, k.GetMinimumFeePerGas, defaults.MinimumFeePerGas, "v0.0.0"},
		{types.KeyMaxFeePerGas, k.GetMaximumFeePerGas, defaults.MaximumFeePerGas, "v6.0.1"},
		{types.KeyMaxDynamicBaseFeeUpwardAdjustment, k.GetMaxDynamicBaseFeeUpwardAdjustment, defaults.MaxDynamicBaseFeeUpwardAdjustment, "v6.0.0"},
		{types.KeyMaxDynamicBaseFeeDownwardAdjustment, k.GetMaxDynamicBaseFeeDownwardAdjustment, defaults.MaxDynamicBaseFeeDownwardAdjustment, "v6.0.0"},
	} {
		t.Run(string(param.key), func(t *testing.T) {
			for _, version := range []string{"current", "v5.7.0", "v5.8.0", "v6.0.0", "v6.0.1", "v6.0.6"} {
				t.Run(version, func(t *testing.T) {
					readCtx := ctx
					if version != "current" {
						readCtx = ctx.WithIsTracing(true).WithClosestUpgradeName(version)
					}
					t.Run("missing", func(t *testing.T) {
						k.Paramstore.Set(ctx, param.key, sdk.MustNewDecFromStr("0.125"))
						key := append([]byte(k.Paramstore.Name()+"/"), param.key...)
						ctx.KVStore(k.GetStoreKey()).Delete(key)
						require.False(t, k.Paramstore.Has(ctx, param.key))
						value := param.get(readCtx)
						if version != "current" && semver.Compare(version, "v6.0.6") < 0 && semver.Compare(version, param.introduced) >= 0 {
							require.True(t, value.IsNil())
						} else {
							require.Equal(t, param.defaultValue, value)
						}
						require.False(t, k.Paramstore.Has(ctx, param.key))
					})
					for _, value := range []sdk.Dec{sdk.ZeroDec(), sdk.MustNewDecFromStr("0.125")} {
						t.Run("stored_"+value.String(), func(t *testing.T) {
							k.Paramstore.Set(ctx, param.key, value)
							require.True(t, k.Paramstore.Has(ctx, param.key))
							if version != "current" && semver.Compare(version, param.introduced) < 0 {
								require.Equal(t, param.defaultValue, param.get(readCtx))
							} else {
								require.Equal(t, value, param.get(readCtx))
							}
						})
					}
				})
			}
		})
	}
}
