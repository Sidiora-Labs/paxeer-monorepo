package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const programFeeDenomPrefix = "web/feedenom"

func ProgramFeeDenomKey(asset types.Hash32) []byte {
	key := make([]byte, 0, len(programFeeDenomPrefix)+len(asset))
	key = append(key, programFeeDenomPrefix...)
	return append(key, asset[:]...)
}

func (k Keeper) ProgramFeeDenom(ctx sdk.Context, asset types.Hash32) (string, bool) {
	denom := ctx.KVStore(k.storeKey).Get(ProgramFeeDenomKey(asset))
	if denom == nil {
		return "", false
	}
	return string(denom), true
}

func (k Keeper) setProgramFeeDenom(ctx sdk.Context, asset types.Hash32, denom string) {
	ctx.KVStore(k.storeKey).Set(ProgramFeeDenomKey(asset), []byte(denom))
}
