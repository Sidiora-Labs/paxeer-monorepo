package keeper

import (
	"math"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/utils"
	"golang.org/x/mod/semver"
)

const BaseDenom = "uhpx"

var MaxUint64BigInt = new(big.Int).SetUint64(math.MaxUint64)

func (k Keeper) SetParams(ctx sdk.Context, params types.Params) {
	k.Paramstore.SetParamSet(ctx, &params)
}

func (k *Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	return k.GetParamsIfExists(ctx)
}

func (k *Keeper) GetSstoreSetGasEIP2200(ctx sdk.Context) uint64 {
	sstore := k.GetParams(ctx).PaxSstoreSetGasEip2200
	if sstore == 0 {
		return 20000
	}
	return sstore
}

func (k *Keeper) GetParamsPreV580(ctx sdk.Context) (params types.ParamsPreV580) {
	return k.GetParamsPreV580IfExists(ctx)
}

func (k *Keeper) GetParamsPreV600(ctx sdk.Context) (params types.ParamsPreV600) {
	return k.GetParamsPreV600IfExists(ctx)
}

func (k *Keeper) GetParamsPreV601(ctx sdk.Context) (params types.ParamsPreV601) {
	return k.GetParamsPreV601IfExists(ctx)
}

func (k *Keeper) GetParamsPreV606(ctx sdk.Context) (params types.ParamsPreV606) {
	return k.GetParamsPreV606IfExists(ctx)
}

func (k *Keeper) GetParamsIfExists(ctx sdk.Context) types.Params {
	params := types.Params{}
	k.Paramstore.GetParamSetIfExists(ctx, &params)
	return params
}

func (k *Keeper) GetParamsPreV580IfExists(ctx sdk.Context) types.ParamsPreV580 {
	params := types.ParamsPreV580{}
	k.Paramstore.GetParamSetIfExists(ctx, &params)
	return params
}

func (k *Keeper) GetParamsPreV600IfExists(ctx sdk.Context) types.ParamsPreV600 {
	params := types.ParamsPreV600{}
	k.Paramstore.GetParamSetIfExists(ctx, &params)
	return params
}

func (k *Keeper) GetParamsPreV601IfExists(ctx sdk.Context) types.ParamsPreV601 {
	params := types.ParamsPreV601{}
	k.Paramstore.GetParamSetIfExists(ctx, &params)
	return params
}

func (k *Keeper) GetParamsPreV606IfExists(ctx sdk.Context) types.ParamsPreV606 {
	params := types.ParamsPreV606{}
	k.Paramstore.GetParamSetIfExists(ctx, &params)
	return params
}

func (k *Keeper) GetBaseDenom(ctx sdk.Context) string {
	return BaseDenom
}

func decimalParamOrDefault(value, defaultValue sdk.Dec) sdk.Dec {
	if value.IsNil() {
		return defaultValue
	}
	return value
}

func (k *Keeper) GetPriorityNormalizer(ctx sdk.Context) sdk.Dec {
	if !ctx.IsTracing() {
		return decimalParamOrDefault(k.GetParams(ctx).PriorityNormalizer, evmtypes.DefaultPriorityNormalizer)
	}
	switch {
	case semver.Compare(ctx.ClosestUpgradeName(), "v5.8.0") < 0:
		return k.GetParamsPreV580(ctx).PriorityNormalizer
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.0") < 0:
		return k.GetParamsPreV600(ctx).PriorityNormalizer
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.1") < 0:
		return k.GetParamsPreV601(ctx).PriorityNormalizer
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.6") < 0:
		return k.GetParamsPreV606(ctx).PriorityNormalizer
	default:
		return decimalParamOrDefault(k.GetParams(ctx).PriorityNormalizer, evmtypes.DefaultPriorityNormalizer)
	}
}

func (k *Keeper) GetBaseFeePerGas(ctx sdk.Context) sdk.Dec {
	if !ctx.IsTracing() {
		return decimalParamOrDefault(k.GetParams(ctx).BaseFeePerGas, evmtypes.DefaultBaseFeePerGas)
	}
	switch {
	case semver.Compare(ctx.ClosestUpgradeName(), "v5.8.0") < 0:
		return k.GetParamsPreV580(ctx).BaseFeePerGas
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.0") < 0:
		return k.GetParamsPreV600(ctx).BaseFeePerGas
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.1") < 0:
		return k.GetParamsPreV601(ctx).BaseFeePerGas
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.6") < 0:
		return k.GetParamsPreV606(ctx).BaseFeePerGas
	default:
		return decimalParamOrDefault(k.GetParams(ctx).BaseFeePerGas, evmtypes.DefaultBaseFeePerGas)
	}
}

func (k *Keeper) GetMaxDynamicBaseFeeUpwardAdjustment(ctx sdk.Context) sdk.Dec {
	if !ctx.IsTracing() {
		return decimalParamOrDefault(k.GetParams(ctx).MaxDynamicBaseFeeUpwardAdjustment, evmtypes.DefaultMaxDynamicBaseFeeUpwardAdjustment)
	}
	switch {
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.0") < 0:
		// Not present in pre-6.0.0 params; use default
		return types.DefaultMaxDynamicBaseFeeUpwardAdjustment
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.1") < 0:
		return k.GetParamsPreV601(ctx).MaxDynamicBaseFeeUpwardAdjustment
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.6") < 0:
		return k.GetParamsPreV606(ctx).MaxDynamicBaseFeeUpwardAdjustment
	default:
		return decimalParamOrDefault(k.GetParams(ctx).MaxDynamicBaseFeeUpwardAdjustment, evmtypes.DefaultMaxDynamicBaseFeeUpwardAdjustment)
	}
}

func (k *Keeper) GetMaxDynamicBaseFeeDownwardAdjustment(ctx sdk.Context) sdk.Dec {
	if !ctx.IsTracing() {
		return decimalParamOrDefault(k.GetParams(ctx).MaxDynamicBaseFeeDownwardAdjustment, evmtypes.DefaultMaxDynamicBaseFeeDownwardAdjustment)
	}
	switch {
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.0") < 0:
		// Not present in pre-6.0.0 params; use default
		return types.DefaultMaxDynamicBaseFeeDownwardAdjustment
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.1") < 0:
		return k.GetParamsPreV601(ctx).MaxDynamicBaseFeeDownwardAdjustment
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.6") < 0:
		return k.GetParamsPreV606(ctx).MaxDynamicBaseFeeDownwardAdjustment
	default:
		return decimalParamOrDefault(k.GetParams(ctx).MaxDynamicBaseFeeDownwardAdjustment, evmtypes.DefaultMaxDynamicBaseFeeDownwardAdjustment)
	}
}

func (k *Keeper) GetMinimumFeePerGas(ctx sdk.Context) sdk.Dec {
	if !ctx.IsTracing() {
		return decimalParamOrDefault(k.GetParams(ctx).MinimumFeePerGas, evmtypes.DefaultMinFeePerGas)
	}
	switch {
	case semver.Compare(ctx.ClosestUpgradeName(), "v5.8.0") < 0:
		return k.GetParamsPreV580(ctx).MinimumFeePerGas
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.0") < 0:
		return k.GetParamsPreV600(ctx).MinimumFeePerGas
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.1") < 0:
		return k.GetParamsPreV601(ctx).MinimumFeePerGas
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.6") < 0:
		return k.GetParamsPreV606(ctx).MinimumFeePerGas
	default:
		return decimalParamOrDefault(k.GetParams(ctx).MinimumFeePerGas, evmtypes.DefaultMinFeePerGas)
	}
}

// GetMinimumFeePerGasOrDefault also defaults missing historical minimum fees.
func (k *Keeper) GetMinimumFeePerGasOrDefault(ctx sdk.Context) sdk.Dec {
	return decimalParamOrDefault(k.GetMinimumFeePerGas(ctx), evmtypes.DefaultMinFeePerGas)
}

func (k *Keeper) GetMaximumFeePerGas(ctx sdk.Context) sdk.Dec {
	if !ctx.IsTracing() {
		return decimalParamOrDefault(k.GetParams(ctx).MaximumFeePerGas, evmtypes.DefaultMaxFeePerGas)
	}
	switch {
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.1") < 0:
		// Not present in pre-6.0.1 params; use default
		return types.DefaultMaxFeePerGas
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.6") < 0:
		return k.GetParamsPreV606(ctx).MaximumFeePerGas
	default:
		return decimalParamOrDefault(k.GetParams(ctx).MaximumFeePerGas, evmtypes.DefaultMaxFeePerGas)
	}
}

func (k *Keeper) GetTargetGasUsedPerBlock(ctx sdk.Context) uint64 {
	if !ctx.IsTracing() {
		return k.GetParams(ctx).TargetGasUsedPerBlock
	}
	switch {
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.0") < 0:
		// Not present in pre-6.0.0 params; use default
		return types.DefaultTargetGasUsedPerBlock
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.1") < 0:
		return k.GetParamsPreV601(ctx).TargetGasUsedPerBlock
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.6") < 0:
		return k.GetParamsPreV606(ctx).TargetGasUsedPerBlock
	default:
		return k.GetParams(ctx).TargetGasUsedPerBlock
	}
}

func (k *Keeper) GetDeliverTxHookWasmGasLimit(ctx sdk.Context) uint64 {
	if !ctx.IsTracing() {
		return k.GetParams(ctx).DeliverTxHookWasmGasLimit
	}
	switch {
	case semver.Compare(ctx.ClosestUpgradeName(), "v5.8.0") < 0:
		// Not present in pre-5.8.0 params; use default
		return types.DefaultDeliverTxHookWasmGasLimit
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.0") < 0:
		return k.GetParamsPreV600(ctx).DeliverTxHookWasmGasLimit
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.1") < 0:
		return k.GetParamsPreV601(ctx).DeliverTxHookWasmGasLimit
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.6") < 0:
		return k.GetParamsPreV606(ctx).DeliverTxHookWasmGasLimit
	default:
		return k.GetParams(ctx).DeliverTxHookWasmGasLimit
	}
}

func (k *Keeper) GetRegisterPointerDisabled(ctx sdk.Context) bool {
	if !ctx.IsTracing() {
		return k.GetParams(ctx).RegisterPointerDisabled
	}
	switch {
	case semver.Compare(ctx.ClosestUpgradeName(), "v6.0.6") < 0:
		// Not present in pre-5.8.0 params; use default
		return types.DefaultRegisterPointerDisabled
	default:
		return k.GetParams(ctx).RegisterPointerDisabled
	}
}

func (k *Keeper) ChainID(ctx sdk.Context) *big.Int {
	// return mapped chain ID
	return config.GetEVMChainID(ctx.ChainID())

}

/*
*
pax gas = evm gas * multiplier
pax gas price = fee / pax gas = fee / (evm gas * multiplier) = evm gas / multiplier
*/
func (k *Keeper) GetEVMGasLimitFromCtx(ctx sdk.Context) uint64 {
	return k.getEvmGasLimitFromCtx(ctx)
}

func (k *Keeper) GetCosmosGasLimitFromEVMGas(ctx sdk.Context, evmGas uint64) uint64 {
	gasMultipler := k.GetPriorityNormalizer(ctx)
	gasLimitBigInt := sdk.NewDecFromInt(sdk.NewIntFromUint64(evmGas)).Mul(gasMultipler).TruncateInt().BigInt()
	if gasLimitBigInt.Cmp(utils.BigMaxU64) > 0 {
		gasLimitBigInt = utils.BigMaxU64
	}
	return gasLimitBigInt.Uint64()
}

func (k *Keeper) getEvmGasLimitFromCtx(ctx sdk.Context) uint64 {
	paxGasRemaining := ctx.GasMeter().Limit() - ctx.GasMeter().GasConsumedToLimit()
	if ctx.GasMeter().Limit() <= 0 {
		return math.MaxUint64
	}
	if ctx.ChainID() != Pacific1ChainID || ctx.BlockHeight() >= 119821526 {
		ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	}
	evmGasBig := sdk.NewDecFromInt(sdk.NewIntFromUint64(paxGasRemaining)).Quo(k.GetPriorityNormalizer(ctx)).TruncateInt().BigInt()
	if evmGasBig.Cmp(MaxUint64BigInt) > 0 {
		evmGasBig = MaxUint64BigInt
	}
	return evmGasBig.Uint64()
}
