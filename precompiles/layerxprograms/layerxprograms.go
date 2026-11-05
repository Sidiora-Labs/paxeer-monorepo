package layerxprograms

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

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
	XWebK() *xwebkeeper.Keeper
}

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
	if wired.XWebK() == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: xweb keeper is unavailable")
	}
	p, err := NewPrecompile(wired.LayerXHost(), evm, upgrade, planName)
	if err != nil {
		return nil, err
	}
	executor := &PrecompileExecutor{host: wired.LayerXHost(), evmKeeper: evm}
	configureXWeb(executor, wired.XWebK())
	return pcommon.NewPrecompile(programsABI, executor, p.Address(), PrecompileName).
		WithLXPReasons().WithGasTable(gasTable), nil
}

func init() {
	if err := precompiles.RegisterMergeConstructor(Address, NewMergePrecompile); err != nil {
		panic(err)
	}
}

// ProgramTransferred is the transfer set root emitted after an atomic transfer set.
type ProgramTransferred struct {
	ProgramID       common.Hash
	TransferSetRoot common.Hash
}

// Address identifies the only emitter accepted by DecodeProgramTransferred.
func (ProgramTransferred) Address() common.Address { return Address }

// DecodeProgramTransferred refuses logs from another contract and noncanonical frames.
func DecodeProgramTransferred(log *types.Log) (ProgramTransferred, error) {
	var event ProgramTransferred
	if log == nil || log.Address != event.Address() || len(log.Topics) != 2 ||
		log.Topics[0] != programsABI.Events["ProgramTransferred"].ID || len(log.Data) != 32 {
		return event, fmt.Errorf("LXP_NON_CANONICAL: invalid ProgramTransferred log")
	}
	event.ProgramID = log.Topics[1]
	copy(event.TransferSetRoot[:], log.Data)
	return event, nil
}
