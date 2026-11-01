package oracle

import (
	"fmt"
	"math/big"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/ethereum/go-ethereum/common"
)

const MergeMaxFeeds = 32

var MergeMaxConfidence = big.NewInt(1_000_000_000_000_000_000)

func init() {
	if err := precompiles.RegisterMergeConstructor(common.HexToAddress(OracleAddress), NewMergePrecompile); err != nil {
		panic(err)
	}
}

func NewMergePrecompile(keepers utils.Keepers, planName string) (precompiles.IPrecompile, error) {
	if keepers == nil {
		return nil, fmt.Errorf("%w: precompile keepers are unavailable", agentcommon.ErrNoContext)
	}
	exchange := keepers.LayerXExchangeK()
	if exchange == nil {
		return nil, fmt.Errorf("%w: exchange keeper is unavailable", agentcommon.ErrNoContext)
	}
	evmKeeper, ok := keepers.EVMK().(*evmkeeper.Keeper)
	if !ok || evmKeeper == nil {
		return nil, fmt.Errorf("%w: evm keeper is unavailable", agentcommon.ErrNoContext)
	}
	p, err := NewPrecompile(exchange, evmKeeper, evmKeeper.UpgradeKeeper(), planName, MergeMaxFeeds, MergeMaxConfidence)
	if err != nil {
		return nil, err
	}
	return p, nil
}
