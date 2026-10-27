package keeper

import (
	"bytes"

	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k Keeper) CheckWrite(ctx sdk.Context, method string) error {
	activity, write := types.MethodActivity[method]
	if !write {
		return nil
	}
	params := k.GetParams(ctx)
	var enabled bool
	switch activity.Module {
	case types.EscrowModuleID:
		enabled = params.EscrowEnabled
	case types.BudgetModuleID:
		enabled = params.BudgetEnabled
	case types.StreamModuleID:
		enabled = params.StreamEnabled
	case types.ServiceModuleID:
		enabled = params.ServiceEnabled
	}
	if !enabled {
		return types.ErrFamilyDisabled.Wrap(method)
	}
	if err := govtypes.CheckPauseScope(types.ModuleName, params.Paused, method); err != nil {
		return err
	}
	key, err := types.KernelModuleEnableKey(activity.Module)
	if err != nil {
		return types.ErrKernelGateMismatch.Wrapf("%s: %v", method, err)
	}
	var enabledRecord [32]byte
	enabledRecord[31] = 1
	if !bytes.Equal(ctx.KVStore(k.hostKey).Get(key), enabledRecord[:]) {
		return types.ErrKernelGateMismatch.Wrap(method)
	}
	return nil
}
