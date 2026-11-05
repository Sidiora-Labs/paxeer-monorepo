package testutil

import (
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxchains"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
)

type LayerXPayKeepers = layerxpay.LayerXPayKeepers

var _ LayerXPayKeepers = (*app.PrecompileKeepers)(nil)
var _ layerxchains.LayerXChainsKeepers = (*app.PrecompileKeepers)(nil)

func Keepers(a *app.App) *app.PrecompileKeepers {
	return a.GetPrecompileKeepers().(*app.PrecompileKeepers)
}

func Install(a *app.App) error {
	custom, err := precompiles.GetCustomPrecompilesE(app.LatestUpgrade, Keepers(a))
	if err != nil {
		return err
	}
	a.EvmKeeper.SetCustomPrecompiles(custom, app.LatestUpgrade)
	return nil
}
