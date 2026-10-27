package layerxprograms

import (
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

type CallContext = callContext
type ProgramsGate = programsGate

var (
	GasTable                   = gasTable
	ProgramsABI                = programsABI
	KernelError                = kernelError
	NextOrdinal                = nextOrdinal
	PrincipalOf                = principalOf
	Submit                     = submit
	EmitSandboxEvents          = emitSandboxEvents
	DecodeProgramsReceiptEvent = decodeProgramsReceiptEvent
	EmitReceiptEvents          = emitReceiptEvents
	CheckCall                  = checkCall
	ErrGateUnknownMethod       = errGateUnknownMethod
)

func NewTestExecutor(host *lxphost.Host, evm *evmkeeper.Keeper) *PrecompileExecutor {
	return &PrecompileExecutor{host: host, evmKeeper: evm}
}

func (p *PrecompileExecutor) TestEVMKeeper() *evmkeeper.Keeper { return p.evmKeeper }
func (p *PrecompileExecutor) SetTestHost(host *lxphost.Host)   { p.host = host }

func NewCallContext(caller common.Address, method *abi.Method) CallContext {
	return callContext{caller: caller, method: method}
}

func NewProgramsGate(upgrade UpgradeKeeper, planName string) ProgramsGate {
	return programsGate{upgrade: upgrade, planName: planName}
}

func (g programsGate) CheckActive(ctx sdk.Context) error { return g.checkActive(ctx) }

func (log programsReceiptLog) Name() string          { return log.name }
func (log programsReceiptLog) Values() []interface{} { return log.values }
func (log programsReceiptLog) HasProgramID() bool    { return log.hasProgramID }
