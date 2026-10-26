package types_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
)

func TestBankKeeperMethodSetFrozen(t *testing.T) {
	var _ types.BankKeeper = bankkeeper.BaseKeeper{}

	keeperType := reflect.TypeOf((*types.BankKeeper)(nil)).Elem()
	methods := make([]string, keeperType.NumMethod())
	for i := range methods {
		methods[i] = keeperType.Method(i).Name
	}

	require.Equal(t, []string{
		"BurnCoins",
		"GetDenomAllowList",
		"GetDenomMetaData",
		"HasSupply",
		"MintCoins",
		"SendCoinsFromAccountToModule",
		"SendCoinsFromModuleToAccount",
		"SetDenomAllowList",
		"SetDenomMetaData",
	}, methods)
}
