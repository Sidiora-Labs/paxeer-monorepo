package keeper

import (
	"encoding/json"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k Keeper) GetParams(ctx sdk.Context) types.Params {
	bz := ctx.KVStore(k.storeKey).Get(types.ParamsKey)
	if bz == nil {
		return types.DefaultParams()
	}
	var params types.Params
	if err := json.Unmarshal(bz, &params); err != nil {
		panic(fmt.Errorf("layerxpay: decode params: %w", err))
	}
	return params
}

func (k Keeper) SetParams(ctx sdk.Context, params types.Params) {
	bz, err := json.Marshal(params)
	if err != nil {
		panic(fmt.Errorf("layerxpay: encode params: %w", err))
	}
	ctx.KVStore(k.storeKey).Set(types.ParamsKey, bz)
}
