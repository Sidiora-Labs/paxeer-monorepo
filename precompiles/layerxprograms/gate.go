package layerxprograms

import (
	"fmt"
	"math/big"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

type UpgradeKeeper interface {
	GetDoneHeight(ctx sdk.Context, name string) int64
}

type programsGate struct {
	upgrade  UpgradeKeeper
	planName string
}

func (g programsGate) checkActive(ctx sdk.Context) error {
	if g.upgrade == nil || g.planName == "" || ctx.MultiStore() == nil || ctx.GasMeter() == nil {
		return ErrModuleDisabled
	}
	readCtx := ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	doneHeight := g.upgrade.GetDoneHeight(readCtx, g.planName)
	if doneHeight <= 0 || ctx.BlockHeight() < doneHeight {
		return ErrModuleDisabled
	}
	return nil
}

const errGateUnknownMethod refusalCode = "LXP_PRECOMPILE_UNKNOWN_METHOD: unknown method"

func checkCall(method *abi.Method, value *big.Int, readOnly bool) error {
	if err := pcommon.ValidateNonPayable(value); err != nil {
		return fmt.Errorf("LXP_PRECOMPILE_NON_PAYABLE: %w", err)
	}
	if method == nil {
		return errGateUnknownMethod
	}
	row, ok := gasTable[method.Name]
	if !ok {
		return errGateUnknownMethod
	}
	if readOnly && row.Write {
		return ErrReadOnlyWrite
	}
	return nil
}
