package layerxpay

import (
	"fmt"
	"reflect"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

func (p *Executor) budgetHandlers(ctx sdk.Context, method *abi.Method, caller common.Address, args []interface{}, key [32]byte, evm *vm.EVM) ([]byte, error) {
	id := args[0].([32]byte)
	switch method.Name {
	case "getBudget":
		record, found, err := p.keeper.GetBudget(ctx, id)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("LXP_UNKNOWN_FIELD: budget not found")
		}
		tuple, err := payTuple(method.Outputs[0].Type, reflect.ValueOf(record))
		if err != nil {
			return nil, err
		}
		return method.Outputs.Pack(tuple.Interface())
	case "budgetRemaining":
		remaining, err := p.keeper.BudgetRemaining(ctx, id)
		if err != nil {
			return nil, err
		}
		return method.Outputs.Pack(remaining.BigInt())
	}
	var drained codec.U128
	if method.Name == "budgetClose" || method.Name == "budgetRevoke" {
		before, found, err := p.keeper.GetBudget(ctx, id)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("LXP_UNKNOWN_FIELD: budget not found")
		}
		balance, err := p.keeper.Host().ReadBalance(ctx, before.BudgetAccount, before.AssetID)
		if err != nil {
			return nil, err
		}
		if balance.IsNegative() || balance.BigInt().BitLen() > 128 {
			return nil, fmt.Errorf("LXP_FATAL_INVARIANT: invalid budget balance")
		}
		drained = payU128(balance.BigInt())
	}
	var event string
	var err error
	switch method.Name {
	case "budgetCreate", "budgetCreateNative":
		a := paykeeper.BudgetCreateArgs{
			BudgetID: id, BudgetAccount: args[1].([32]byte), AssetID: args[2].([32]byte), PurposeHash: args[3].([32]byte),
			PerPeriodLimit: payU128(args[4]), CarryCap: payU128(args[5]), Amount: payU128(args[6]),
			PeriodLengthMs: args[7].(uint64), PeriodStartMs: args[8].(uint64), ExpiryMs: args[9].(uint64),
			RevocationSequence: args[10].(uint64), RolloverPolicy: args[11].(uint8),
		}
		if method.Name == "budgetCreateNative" {
			_, err = p.keeper.BudgetCreateNative(ctx, caller, a, args[12].([32]byte), args[13].(uint64))
		} else {
			_, err = p.keeper.BudgetCreate(ctx, caller, a)
		}
		event = "BudgetCreated"
	case "budgetFund":
		_, err = p.keeper.BudgetFund(ctx, caller, id, payU128(args[1]), key)
		event = "BudgetFunded"
	case "budgetFundNative":
		_, err = p.keeper.BudgetFundNative(ctx, caller, id, payU128(args[1]), args[2].(uint64), key)
		event = "BudgetFunded"
	case "budgetAmend":
		_, err = p.keeper.BudgetAmend(ctx, caller, id, payU128(args[1]), payU128(args[2]), args[3].(uint64), args[4].(uint8), key)
		event = "BudgetAmended"
	case "budgetDelegateAdd":
		_, err = p.keeper.BudgetDelegateAdd(ctx, caller, id, args[1].([32]byte))
		event = "BudgetDelegateAdded"
	case "budgetDelegateRemove":
		_, err = p.keeper.BudgetDelegateRemove(ctx, caller, id, args[1].([32]byte))
		event = "BudgetDelegateRemoved"
	case "budgetSpend":
		_, err = p.keeper.BudgetSpend(ctx, caller, id, args[1].([32]byte), payU128(args[2]))
		event = "BudgetSpent"
	case "budgetDelegateSpend", "budgetPull":
		if ctx.EVMTxHash() == (common.Hash{}) {
			return nil, fmt.Errorf("LXP_NON_CANONICAL: layerxpay missing transaction hash")
		}
		var payload []byte
		if method.Name == "budgetDelegateSpend" {
			payload, err = paytypes.EncodeBudgetDelegateSpend(id, args[1].([32]byte), payU128(args[2]), key)
		} else {
			payload, err = paytypes.EncodeBudgetPull(id, args[1].([32]byte), payU128(args[2]), key, args[3].([]byte))
		}
		if err == nil {
			_, err = p.keeper.Submit(ctx, caller, method.Name, payload, key)
		}
		event = "BudgetSpent"
	case "budgetDefund":
		_, err = p.keeper.BudgetDefund(ctx, caller, id, payU128(args[1]), key)
		event = "BudgetDefunded"
	case "budgetClose":
		_, err = p.keeper.BudgetClose(ctx, caller, id, args[1].(uint64), key)
		event = "BudgetClosed"
	case "budgetRevoke":
		_, err = p.keeper.BudgetRevoke(ctx, caller, id, args[1].(uint64), key)
		event = "BudgetRevoked"
	default:
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: %s", method.Name)
	}
	if err != nil {
		return nil, err
	}
	record, found, err := p.keeper.GetBudget(ctx, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("LXP_FATAL_INVARIANT: budget missing after %s", method.Name)
	}
	var eventRecord interface{} = record
	if event == "BudgetClosed" || event == "BudgetRevoked" {
		eventRecord = payBudgetDrainRecord{Budget: record, Drained: drained}
	}
	values, err := payEvents[event].pack(args, eventRecord)
	if err != nil {
		return nil, err
	}
	if err := emitPayEvent(evm, event, id, values); err != nil {
		return nil, err
	}
	return method.Outputs.Pack()
}
