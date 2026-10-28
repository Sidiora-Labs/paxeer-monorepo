package keeper

import (
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "paused-denoms", PausedDenomsInvariant(k))
}

func PausedDenomsInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		for _, denom := range k.GetPausedDenoms(ctx) {
			if !k.GetDenomPrefixStore(ctx, denom).Has([]byte(types.DenomAuthorityMetadataKey)) {
				return sdk.FormatInvariant(types.ModuleName, "paused-denoms", fmt.Sprintf(
					"paused denom %s has no authority metadata", denom)), true
			}
			if _, err := k.GetAuthorityMetadata(ctx, denom); err != nil {
				return sdk.FormatInvariant(types.ModuleName, "paused-denoms", fmt.Sprintf(
					"paused denom %s has unreadable authority metadata: %v", denom, err)), true
			}
		}
		return sdk.FormatInvariant(types.ModuleName, "paused-denoms", "all paused denoms have authority metadata"), false
	}
}
