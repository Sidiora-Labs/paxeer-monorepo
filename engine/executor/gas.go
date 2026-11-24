package executor

import sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"

func TransactionGas(ctx sdk.Context, priorityNormalizer sdk.Dec, gasLimit, usedGas uint64) (int64, int64) {
	gasWanted := priorityNormalizer.MulInt64(int64(gasLimit)).TruncateInt().Int64()                         //nolint:gosec
	meter := sdk.NewGasMeterWithMultiplier(ctx, uint64(gasWanted))                                          //nolint:gosec
	meter.ConsumeGas(priorityNormalizer.MulInt64(int64(usedGas)).TruncateInt().Uint64(), "evm transaction") //nolint:gosec
	return gasWanted, int64(meter.GasConsumed())                                                            //nolint:gosec
}
