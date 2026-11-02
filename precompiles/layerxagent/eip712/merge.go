package eip712

import (
	"fmt"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/ethereum/go-ethereum/common"
)

const (
	MergeMaxNameBytes    = 256
	MergeMaxVersionBytes = 256
)

func init() {
	if err := precompiles.RegisterMergeConstructor(common.HexToAddress(Eip712Address), NewMergePrecompile); err != nil {
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
	p, err := NewPrecompile(evmKeeper.UpgradeKeeper(), planName, MergeMaxNameBytes, MergeMaxVersionBytes)
	if err != nil {
		return nil, err
	}
	return p, nil
}
