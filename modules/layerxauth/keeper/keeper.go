package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type Keeper struct {
	cdc         codec.BinaryCodec
	storeKey    sdk.StoreKey
	host        *lxphost.Host
	authzKeeper types.AuthzKeeper
	evmKeeper   types.EVMKeeper
	authority   string
}

func NewKeeper(cdc codec.BinaryCodec, storeKey sdk.StoreKey, host *lxphost.Host, authz types.AuthzKeeper, evm types.EVMKeeper, authority string) *Keeper {
	return &Keeper{
		cdc: cdc, storeKey: storeKey, host: host,
		authzKeeper: authz, evmKeeper: evm, authority: authority,
	}
}

func (k *Keeper) store(ctx sdk.Context) sdk.KVStore { return ctx.KVStore(k.storeKey) }

func (k *Keeper) GetParams(ctx sdk.Context) types.Params {
	bz := k.store(ctx).Get(types.ParamsKey)
	if bz == nil {
		return types.DefaultParams()
	}
	var params types.Params
	k.cdc.MustUnmarshal(bz, &params)
	return params
}

func (k *Keeper) SetParams(ctx sdk.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}
	k.store(ctx).Set(types.ParamsKey, k.cdc.MustMarshal(&params))
	return nil
}
