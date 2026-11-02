package layerxchains

import (
	"math/big"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

func (p *Executor) emitEvent(evm *vm.EVM, name string, id uint32, secondary *common.Address, values ...interface{}) error {
	event := p.abi.Events[name]
	topics := []common.Hash{event.ID, common.BigToHash(new(big.Int).SetUint64(uint64(id)))}
	if secondary != nil {
		topics = append(topics, common.BytesToHash(secondary.Bytes()))
	}
	data, err := event.Inputs.NonIndexed().Pack(values...)
	if err != nil {
		return err
	}
	return pcommon.EmitEVMLog(evm, common.HexToAddress(ChainsAddress), topics, data)
}
