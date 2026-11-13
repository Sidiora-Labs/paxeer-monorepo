package types

import (
	"fmt"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
)

// LaunchedChain binds a chain registry entry to its launchpad market.
type LaunchedChain struct {
	RegistryNumber uint64 `json:"registry_number"`
	ChainID        uint64 `json:"chain_id"`
	Pool           string `json:"pool"`
	Token          string `json:"token"`
	FeeRightID     uint64 `json:"fee_right_id"`
	Creator        string `json:"creator"`
	LaunchBlock    int64  `json:"launch_block"`
}

// ChainRevenueEpoch fixes the claim basis and payout amount for one chain epoch.
type ChainRevenueEpoch struct {
	Chain          uint64  `json:"chain"`
	Epoch          uint64  `json:"epoch"`
	Amount         sdk.Int `json:"amount"`
	SnapshotBlock  uint64  `json:"snapshot_block"`
	EligibleSupply sdk.Int `json:"eligible_supply"`
	ClaimedTotal   sdk.Int `json:"claimed_total"`
	OpenedAt       int64   `json:"opened_at"`
}

// ChainRevenueClaim records a holder's completed claim for one epoch.
type ChainRevenueClaim struct {
	Chain  uint64 `json:"chain"`
	Epoch  uint64 `json:"epoch"`
	Holder string `json:"holder"`
}

// PoolHomeChain records the chain where a pool's trades take place.
type PoolHomeChain struct {
	Pool  string `json:"pool"`
	Chain uint64 `json:"chain"`
}

// ChainRevenueAccount is the module account holding one chain's operator revenue.
func ChainRevenueAccount(registryNumber uint64) sdk.AccAddress {
	return authtypes.NewModuleAddress(fmt.Sprintf("%s_chain_revenue_%d", ModuleName, registryNumber))
}

func (c LaunchedChain) Validate() error {
	if c.RegistryNumber == 0 || c.ChainID == 0 || c.FeeRightID == 0 || c.LaunchBlock < 0 ||
		!canonicalAddress(c.Pool) || !canonicalAddress(c.Token) || !canonicalAddress(c.Creator) ||
		c.Pool == c.Token {
		return invalid("launched chain %d", c.RegistryNumber)
	}
	return nil
}

func (e ChainRevenueEpoch) Validate() error {
	if e.Chain == 0 || e.Epoch == 0 || !amount(e.Amount) || e.Amount.IsZero() ||
		!amount(e.EligibleSupply) || e.EligibleSupply.IsZero() || !amount(e.ClaimedTotal) ||
		e.ClaimedTotal.GT(e.Amount) || e.OpenedAt < 0 {
		return invalid("chain revenue epoch %d/%d", e.Chain, e.Epoch)
	}
	return nil
}
