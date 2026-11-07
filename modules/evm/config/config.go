package config

import (
	"fmt"
	"math/big"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const DefaultChainID = int64(125)

// ChainIDMapping is a mapping of cosmos chain IDs to their respective chain IDs.
var ChainIDMapping = map[string]int64{
	// hyperpax_125-1 (Paxeer mainnet) EVM chain ID == 0x7d
	"hyperpax_125-1": int64(125),
}

// EVMChainIDMapping is a mapping of cosmos chain IDs to their respective chain IDs.
var EVMChainIDMapping = map[int64]string{
	// hyperpax_125-1 (Paxeer mainnet) EVM chain ID == 0x7d
	int64(125): "hyperpax_125-1",
}

func GetEVMChainID(cosmosChainID string) *big.Int {
	if evmChainID, ok := ChainIDMapping[cosmosChainID]; ok {
		return big.NewInt(evmChainID)
	}
	return big.NewInt(DefaultChainID)
}

func GetVersionWthDefault(ctx sdk.Context, override uint16, defaultVersion uint16) uint16 {
	// overrides are only available on non-live chain IDs
	if override > 0 && !IsLiveChainID(ctx) {
		return override
	}
	return defaultVersion
}

// IsLiveChainID return true if one of the live chainIDs
func IsLiveChainID(ctx sdk.Context) bool {
	_, ok := ChainIDMapping[ctx.ChainID()]
	return ok
}

// IsLiveEVMChainID returns true is this chainID is reserved for one of the live chains.
func IsLiveEVMChainID(evmChainID int64) bool {
	_, ok := EVMChainIDMapping[evmChainID]
	return ok
}

// ValidateMockBalancesChainID rejects test minting on every configured live chain.
func ValidateMockBalancesChainID(ctx sdk.Context) error {
	if IsLiveChainID(ctx) {
		return fmt.Errorf("mock_balances is forbidden on live chain %q", ctx.ChainID())
	}
	return nil
}
