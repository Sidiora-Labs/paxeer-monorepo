package evmrpc

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

func (i *InfoAPI) GasPriceHelperForTest(ctx context.Context, baseFee *big.Int, totalGasUsedPrevBlock uint64, medianRewardPrevBlock *big.Int) (*hexutil.Big, error) {
	return i.gasPriceHelper(ctx, baseFee, totalGasUsedPrevBlock, medianRewardPrevBlock)
}

func (i *InfoAPI) CalculateGasUsedRatioForTest(ctx context.Context, blockHeight int64) (float64, error) {
	return i.calculateGasUsedRatio(ctx, blockHeight)
}
