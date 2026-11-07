package layerxledger

import (
	"crypto/sha256"
	"fmt"

	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

const AccountOpened = "AccountOpened"

func init() {
	ledgerMethods["openAccount"] = (*PrecompileExecutor).openAccount
	ledgerMethods["send"] = (*PrecompileExecutor).send
}

func (p *PrecompileExecutor) openAccount(ctx sdk.Context, _ *abi.Method, actor ledgerkeeper.Actor,
	args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	assetID := args[0].([32]byte)
	key := sha256.Sum256(types.EncodeOpenAccountPayload(assetID))
	result, err := p.keeper.OpenAccount(ctx, actor, assetID, key)
	if err != nil {
		return nil, RevertError(err)
	}
	events, err := ledgerkeeper.LedgerEvents(0x00010004, result.Receipt, ledgerkeeper.EventArgs{})
	if err != nil {
		return nil, RevertError(err)
	}
	id := events[0].Fields["accountId"].([32]byte)
	if err := p.log(evm, AccountOpened, []common.Hash{common.Hash(id)}); err != nil {
		return nil, err
	}
	return append([]byte(nil), id[:]...), nil
}

func (p *PrecompileExecutor) send(ctx sdk.Context, method *abi.Method, actor ledgerkeeper.Actor,
	args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	envelope := args[0].([]byte)
	if err := types.ValidateSendEnvelope(envelope); err != nil {
		return nil, RevertError(err)
	}
	var key [32]byte
	copy(key[:], envelope[124:156])
	result, err := p.keeper.Send(ctx, actor, envelope, key)
	if err != nil {
		return nil, RevertError(err)
	}
	events, err := ledgerkeeper.LedgerEvents(0x00010005, result.Receipt, ledgerkeeper.EventArgs{})
	if err != nil {
		return nil, RevertError(err)
	}
	for _, event := range events {
		fields := event.Fields
		topics := []common.Hash{common.Hash(fields["from"].([32]byte)),
			common.Hash(fields["to"].([32]byte)), common.Hash(fields["assetId"].([32]byte))}
		if err := p.log(evm, event.Name, topics, fields["amount"], fields["kind"]); err != nil {
			return nil, err
		}
	}
	return method.Outputs.Pack()
}

func (p *PrecompileExecutor) log(evm *vm.EVM, name string, topics []common.Hash, data ...interface{}) error {
	event, ok := p.abi.Events[name]
	if !ok {
		return fmt.Errorf("layerxledger: unknown event %s", name)
	}
	packed, err := event.Inputs.NonIndexed().Pack(data...)
	if err != nil {
		return err
	}
	return pcommon.EmitEVMLog(evm, p.address, append([]common.Hash{event.ID}, topics...), packed)
}
