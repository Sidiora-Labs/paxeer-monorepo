package spot_test

import (
	"testing"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type v6120Contracts struct {
	app *app.App
}

func (c v6120Contracts) RefreshPrecompiles(ctx sdk.Context) {
	c.app.RefreshPrecompiles(ctx)
}

func (c v6120Contracts) CustomPrecompiles(ctx sdk.Context) map[common.Address]vm.PrecompiledContract {
	return c.app.EvmKeeper.CustomPrecompiles(ctx)
}

func TestProductionSpotAddressesServeDynamicGas(t *testing.T) {
	a := app.Setup(t, false, true, false)
	ctx := a.GetContextForDeliverTx([]byte{}).WithBlockHeight(1)
	a.UpgradeKeeper.SetDone(ctx, app.V6120Upgrade)
	production := v6120Contracts{app: a}
	production.RefreshPrecompiles(ctx)
	contracts := production.CustomPrecompiles(ctx.WithBlockHeight(2))

	for _, address := range []string{"0x1021", "0x1022", "0x1023", "0x1024", "0x1025"} {
		t.Run(address, func(t *testing.T) {
			addr := common.HexToAddress(address)
			require.Contains(t, contracts, addr)
			contract := contracts[addr]
			require.IsType(t, (*spot.Precompile)(nil), contract)
			require.NotNil(t, contract)
			require.Implements(t, (*vm.DynamicGasPrecompiledContract)(nil), contract)
		})
	}
}
