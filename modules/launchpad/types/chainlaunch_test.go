package types_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestChainRevenueAccount(t *testing.T) {
	market := poolMarket()
	first := types.ChainRevenueAccount(7)
	require.Equal(t, first, types.ChainRevenueAccount(7))
	require.NotEqual(t, first, types.ChainRevenueAccount(8))
	for _, address := range []common.Address{
		common.HexToAddress(market.PoolAddress),
		common.HexToAddress(market.TokenAddress),
		common.BytesToAddress(authtypes.NewModuleAddress(types.ModuleName)),
		common.BytesToAddress(authtypes.NewModuleAddress(types.TreasuryName)),
	} {
		require.NotEqual(t, address, common.BytesToAddress(first))
	}
}
