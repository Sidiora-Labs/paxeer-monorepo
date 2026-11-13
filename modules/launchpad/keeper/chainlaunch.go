package keeper

import (
	"fmt"
	"strconv"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

// LaunchChain creates a market and its home chain in one state transition.
func (k *Keeper) LaunchChain(ctx sdk.Context, caller NestedCaller, evm *vm.EVM, creator common.Address,
	name, symbol string, strategy types.FeeStrategy, optical common.Address, chainName string,
	gasAllowance, byteAllowance uint64, feeShareBps uint16, value sdk.Int) (sdk.Context, types.Market, types.LaunchedChain, error) {
	if k.chainRegistry == nil {
		return ctx, types.Market{}, types.LaunchedChain{}, fmt.Errorf("launchpad: chain registry is unavailable")
	}
	cache, write := ctx.CacheContext()
	marketCtx, market, err := k.CreateMarket(cache, caller, evm, creator, name, symbol, strategy, optical)
	if err != nil {
		return ctx, types.Market{}, types.LaunchedChain{}, err
	}

	nextID := k.chainRegistry.NextID(marketCtx)
	operator := common.BytesToAddress(types.ChainRevenueAccount(uint64(nextID)))
	payer := k.evmKeeper.GetPaxAddressOrDefault(marketCtx, creator)
	domain, err := k.chainRegistry.CreateWithPayer(marketCtx, payer, operator, chainName,
		gasAllowance, byteAllowance, feeShareBps, value, evmtypes.DefaultMinFeePerGas)
	if err != nil {
		return ctx, types.Market{}, types.LaunchedChain{}, err
	}
	if domain.Id != nextID {
		return ctx, types.Market{}, types.LaunchedChain{}, fmt.Errorf("launchpad: registry assigned chain %d after reserving %d", domain.Id, nextID)
	}
	record := types.LaunchedChain{
		RegistryNumber: uint64(domain.Id), ChainID: chainstypes.ChainIDOf(domain.Id).Uint64(),
		Pool: market.PoolAddress, Token: market.TokenAddress, FeeRightID: market.FeeRightID,
		Creator: creator.Hex(), LaunchBlock: marketCtx.BlockHeight(),
	}
	if err := k.SetLaunchedChain(marketCtx, record); err != nil {
		return ctx, types.Market{}, types.LaunchedChain{}, err
	}
	marketCtx.EventManager().EmitEvent(sdk.NewEvent(types.EventTypeChainLaunched,
		sdk.NewAttribute("registry_number", strconv.FormatUint(record.RegistryNumber, 10)),
		sdk.NewAttribute("chain_id", strconv.FormatUint(record.ChainID, 10)),
		sdk.NewAttribute("pool", record.Pool),
		sdk.NewAttribute("token", record.Token),
		sdk.NewAttribute("fee_right_id", strconv.FormatUint(record.FeeRightID, 10)),
		sdk.NewAttribute(types.AttributeKeyCreator, record.Creator)))
	write()
	ctx.EventManager().EmitEvents(marketCtx.EventManager().Events())
	return ctx, market, record, nil
}
