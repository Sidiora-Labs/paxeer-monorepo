package state

import (
	"math/big"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

// NewMockBalancesDBForTest selects the tagged configuration on the real DB
// without requiring a second compiler invocation in the focused test gate.
func NewMockBalancesDBForTest(ctx sdk.Context, k EVMKeeper) *DBImpl {
	db := NewDBImpl(ctx, k, false)
	db.mockBalances = true
	return db
}

func TopOffAccountForTest(db *DBImpl, addr sdk.AccAddress, amount *big.Int) {
	db.topOffAccount(addr, amount)
}
