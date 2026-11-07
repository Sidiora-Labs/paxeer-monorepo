package state

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// RepairM1127MockBalanceHooks exposes the production tagged hook bodies only
// to tests; it neither changes the selected build mode nor replaces a keeper.
func RepairM1127MockBalanceHooks(db *DBImpl) (func(common.Address), func(common.Address, *big.Int)) {
	funding := mockBalanceFunding{db}
	return funding.ensureMinimumBalance, funding.ensureSufficientBalance
}
