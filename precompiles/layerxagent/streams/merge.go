package streams

import (
	"fmt"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/ethereum/go-ethereum/common"
)

func init() {
	if err := precompiles.RegisterMergeConstructor(common.HexToAddress(StreamsAddress), NewMergePrecompile); err != nil {
		panic(err)
	}
}

func NewMergePrecompile(keepers utils.Keepers, planName string) (precompiles.IPrecompile, error) {
	if keepers == nil {
		return nil, fmt.Errorf("%w: precompile keepers are unavailable", agentcommon.ErrNoContext)
	}
	wired, ok := keepers.(layerxpay.LayerXPayKeepers)
	if !ok || wired.LayerXPayK() == nil {
		return nil, fmt.Errorf("%w: pay keeper is unavailable", agentcommon.ErrNoContext)
	}
	evmKeeper, ok := keepers.EVMK().(*evmkeeper.Keeper)
	if !ok || evmKeeper == nil {
		return nil, fmt.Errorf("%w: evm keeper is unavailable", agentcommon.ErrNoContext)
	}
	p, err := NewPrecompile(wired.LayerXPayK(), evmKeeper.UpgradeKeeper(), planName)
	if err != nil {
		return nil, err
	}
	return p, nil
}
