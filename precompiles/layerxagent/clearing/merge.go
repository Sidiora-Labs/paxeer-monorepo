package clearing

import (
	"fmt"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	ethcommon "github.com/ethereum/go-ethereum/common"
)

const MergeMaxOrders = 256

func init() {
	if err := precompiles.RegisterMergeConstructor(ethcommon.HexToAddress(ClearingAddress), NewMergePrecompile); err != nil {
		panic(err)
	}
}

func NewMergePrecompile(keepers utils.Keepers, planName string) (precompiles.IPrecompile, error) {
	if keepers == nil {
		return nil, fmt.Errorf("%w: precompile keepers are unavailable", agentcommon.ErrNoContext)
	}
	evmKeeper, ok := keepers.EVMK().(*evmkeeper.Keeper)
	if !ok || evmKeeper == nil {
		return nil, fmt.Errorf("%w: evm keeper is unavailable", agentcommon.ErrNoContext)
	}
	p, err := NewPrecompile(evmKeeper.UpgradeKeeper(), planName, MergeMaxOrders)
	if err != nil {
		return nil, err
	}
	return p, nil
}
