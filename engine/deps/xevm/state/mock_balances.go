package state

/*
==============================================================================
============================= !!! WARNING !!! ================================
==============================================================================
== This file is ONLY for TESTING/BENCHMARKING.                              ==
== It enables automatic top-off of EVM accounts with insufficient funds.    ==
== DO NOT USE IN PRODUCTION OR MAINNET BUILDS.                              ==
== This is enabled only when the 'mock_balances' build tag is set.          ==
==============================================================================
*/

import (
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

// TopOffAmount is the amount to mint when an account needs more funds (100 ETH)
var TopOffAmount = new(big.Int).Mul(big.NewInt(100), big.NewInt(1_000_000_000_000_000_000))

// mockBalanceFunding is selected by the tagged hooks in mock_balances_enabled.go.
// Keeping the implementation shared lets the ordinary test gate exercise the
// same funding path without rebuilding itself with a different set of tags.
type mockBalanceFunding struct {
	*DBImpl
}

func (s mockBalanceFunding) requireDisposableChain() {
	switch s.ctx.ChainID() {
	case "mock-balances-local-1":
		return
	default:
		panic("FATAL: mock_balances requires the disposable mock-balances-local-1 chain")
	}
}

// ensureMinimumBalance tops off the account if balance is low.
// Called from GetBalance to ensure preCheck passes in StateTransition.
func (s mockBalanceFunding) ensureMinimumBalance(evmAddr common.Address) {
	s.requireDisposableChain()

	paxAddr := s.getPaxAddress(evmAddr)
	currentBalance := s.k.GetBalance(s.ctx, paxAddr)

	if currentBalance.Cmp(TopOffAmount) < 0 {
		s.topOffAccount(paxAddr, TopOffAmount)
	}
}

// ensureSufficientBalance tops off the account if it doesn't have enough for the operation.
// Called from SubBalance before actually subtracting.
func (s mockBalanceFunding) ensureSufficientBalance(evmAddr common.Address, amt *big.Int) {
	s.requireDisposableChain()
	paxAddr := s.getPaxAddress(evmAddr)
	currentBalance := s.k.GetBalance(s.ctx, paxAddr)

	if currentBalance.Cmp(amt) < 0 {
		needed := new(big.Int).Sub(amt, currentBalance)
		needed = needed.Add(needed, TopOffAmount)
		s.topOffAccount(paxAddr, needed)
	}
}

// topOffAccount mints funds to an account.
func (s mockBalanceFunding) topOffAccount(paxAddr sdk.AccAddress, amt *big.Int) {
	s.requireDisposableChain()
	// Ensure account exists
	if !s.k.AccountKeeper().HasAccount(s.ctx, paxAddr) {
		s.k.AccountKeeper().SetAccount(s.ctx, s.k.AccountKeeper().NewAccountWithAddress(s.ctx, paxAddr))
	}

	// Mint and send (use NopLogger to suppress log spam)
	uhpx, wei := SplitUhpxWeiAmount(amt)
	coinsAmt := sdk.NewCoins(sdk.NewCoin(s.k.GetBaseDenom(s.ctx), uhpx.Add(sdk.OneInt())))
	ctx := s.ctx
	if err := s.k.BankKeeper().MintCoins(ctx, types.ModuleName, coinsAmt); err != nil {
		return
	}
	moduleAddr := s.k.AccountKeeper().GetModuleAddress(types.ModuleName)
	_ = s.k.BankKeeper().SendCoinsAndWei(ctx, moduleAddr, paxAddr, uhpx, wei)
}
