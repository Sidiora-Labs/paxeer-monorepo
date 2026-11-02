package testutil

import (
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
)

func NewEVMBindingReader(cdc codec.BinaryCodec, key sdk.StoreKey) *evmkeeper.Keeper {
	subspace := paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), nil, nil, evmtypes.ModuleName)
	return evmkeeper.NewKeeper(key, nil, subspace, nil, nil, nil, nil, transferkeeper.Keeper{}, nil, nil, nil)
}
