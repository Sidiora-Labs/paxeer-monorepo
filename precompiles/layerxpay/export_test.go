package layerxpay

import (
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

type EscrowTuple = escrowTuple
type PayBudgetDrainRecord = payBudgetDrainRecord
type PayStreamChangeRecord = payStreamChangeRecord
type PayServiceSequence[T any] = payServiceSequence[T]

var (
	PayU128      = payU128
	PayEvents    = payEvents
	PayAmount    = payAmount
	EmitPayEvent = emitPayEvent
	GasABI       = gasABI
	MethodGas    = methodGas
	ABITestFS    = abiTestFS
	ZeroArgument = zeroArgument
)

func (event payEvent) Name() string { return event.name }

func (event payEvent) Pack(args []interface{}, record any) ([]interface{}, error) {
	return event.pack(args, record)
}

func (p *Executor) ServiceViewHandlers(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, bool, error) {
	return p.serviceViewHandlers(ctx, method, args)
}
