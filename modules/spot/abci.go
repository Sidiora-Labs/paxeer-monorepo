package spot

import (
	"strconv"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func EndBlocker(ctx sdk.Context, k *keeper.Keeper) error {
	work, err := k.ExecuteResting(evmkeeper.SyntheticReceiptContext(ctx, evmkeeper.SyntheticReceiptSpot))
	if err != nil {
		ctx.EventManager().EmitEvent(sdk.NewEvent(types.EventTypeRestingFailed,
			sdk.NewAttribute(types.AttributeKeyError, err.Error()),
		))
		return err
	}
	ctx.EventManager().EmitEvent(sdk.NewEvent(types.EventTypeResting,
		sdk.NewAttribute(types.AttributeKeySamples, strconv.FormatUint(work.Samples, 10)),
		sdk.NewAttribute(types.AttributeKeyExpired, strconv.FormatUint(work.Expired, 10)),
		sdk.NewAttribute(types.AttributeKeyActivated, strconv.FormatUint(work.Activated, 10)),
		sdk.NewAttribute(types.AttributeKeyFilled, strconv.FormatUint(work.Filled, 10)),
	))
	return nil
}
