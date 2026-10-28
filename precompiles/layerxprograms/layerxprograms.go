package layerxprograms

import (
	"fmt"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	xwebkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
)

const PrecompileName = "layerxprograms"

type HostKeepers interface {
	utils.Keepers
	LayerXHost() *lxphost.Host
	XWebKeeper() *xwebkeeper.Keeper
}

type hostKeepersAdapter struct {
	utils.Keepers
	host *lxphost.Host
	xweb *xwebkeeper.Keeper
}

func (k hostKeepersAdapter) LayerXHost() *lxphost.Host      { return k.host }
func (k hostKeepersAdapter) XWebKeeper() *xwebkeeper.Keeper { return k.xweb }

func NewPrecompile(host *lxphost.Host, evm EVMKeeper, upgrade UpgradeKeeper, planName string) (*pcommon.Precompile, error) {
	if host == nil || evm == nil || upgrade == nil || planName == "" {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: programs host, EVM keeper and merge activation reader are required")
	}
	evmKeeper, ok := evm.(*evmkeeper.Keeper)
	if !ok || evmKeeper == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: EVM keeper is unavailable")
	}
	executor := &PrecompileExecutor{host: host, evmKeeper: evmKeeper}
	configureXWeb(executor, nil)
	return pcommon.NewPrecompile(programsABI, executor, Address, PrecompileName).
		WithLXPReasons().WithGasTable(gasTable), nil
}

func NewMergePrecompile(keepers utils.Keepers, planName string) (precompiles.IPrecompile, error) {
	wired, ok := keepers.(HostKeepers)
	if !ok {
		if legacy, present := keepers.(interface {
			utils.Keepers
			LayerXHost() *lxphost.Host
			XWebK() *xwebkeeper.Keeper
		}); present {
			wired, ok = hostKeepersAdapter{Keepers: keepers, host: legacy.LayerXHost(), xweb: legacy.XWebK()}, true
		}
	}
	if !ok || keepers == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: LayerX host keepers are unavailable")
	}
	evm, ok := keepers.EVMK().(*evmkeeper.Keeper)
	if !ok || evm == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: EVM keeper is unavailable")
	}
	upgrade := evm.UpgradeKeeper()
	if upgrade == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: merge activation reader is unavailable")
	}
	if wired.XWebKeeper() == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: xweb keeper is unavailable")
	}
	p, err := NewPrecompile(wired.LayerXHost(), evm, upgrade, planName)
	if err != nil {
		return nil, err
	}
	executor := &PrecompileExecutor{host: wired.LayerXHost(), evmKeeper: evm}
	configureXWeb(executor, wired.XWebKeeper())
	return pcommon.NewPrecompile(programsABI, executor, p.Address(), PrecompileName).
		WithLXPReasons().WithGasTable(gasTable), nil
}

func init() {
	if err := precompiles.RegisterMergeConstructor(Address, NewMergePrecompile); err != nil {
		panic(err)
	}
}
