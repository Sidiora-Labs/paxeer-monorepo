//go:build mock_balances

package state

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

func (s *DBImpl) ensureMinimumBalance(evmAddr common.Address) {
	mockBalanceFunding{s}.ensureMinimumBalance(evmAddr)
}

func (s *DBImpl) ensureSufficientBalance(evmAddr common.Address, amt *big.Int) {
	mockBalanceFunding{s}.ensureSufficientBalance(evmAddr, amt)
}
