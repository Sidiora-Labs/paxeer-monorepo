package layerxpay

import (
	"errors"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

func (p *Executor) streamHandlers(ctx sdk.Context, method *abi.Method, caller common.Address, args []interface{}, key [32]byte, evm *vm.EVM) ([]byte, error) {
	id := args[0].([32]byte)
	switch method.Name {
	case "getStream":
		record, err := p.keeper.GetStream(ctx, id)
		if err != nil {
			if errors.Is(err, lxphost.Code(-7)) {
				return nil, fmt.Errorf("LXP_UNKNOWN_FIELD: stream not found")
			}
			return nil, err
		}
		return method.Outputs.Pack(record.ABITuple())
	case "getStreamResult":
		result, err := p.keeper.GetStreamResult(ctx, id)
		if err != nil {
			if errors.Is(err, lxphost.Code(-7)) {
				return nil, fmt.Errorf("LXP_UNKNOWN_FIELD: stream result not found")
			}
			return nil, err
		}
		return method.Outputs.Pack(result.ABITuple())
	case "streamAccrued":
		accrued, unsettled, err := p.keeper.StreamAccrued(ctx, id)
		if err != nil {
			if errors.Is(err, lxphost.Code(-7)) {
				return nil, fmt.Errorf("LXP_UNKNOWN_FIELD: stream not found")
			}
			return nil, err
		}
		return method.Outputs.Pack(accrued.BigInt(), unsettled.BigInt())
	}

	var before paytypes.StreamRecord
	var err error
	if method.Name == "streamTopUp" || method.Name == "streamMeter" {
		before, err = p.keeper.GetStream(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	var event string
	switch method.Name {
	case "streamOpen":
		_, err = p.keeper.StreamOpen(ctx, caller, paytypes.StreamOpen{
			StreamID: id, StreamAccount: args[1].([32]byte), Recipient: args[2].([32]byte),
			AssetID: args[3].([32]byte), Mode: args[4].(uint8), Rate: payU128(args[5]),
			RateUnit: args[6].(uint64), StartTimestamp: args[7].(uint64), EndTimestamp: args[8].(uint64),
			TotalCap: payU128(args[9]), InitialFunding: payU128(args[10]), MeterAuthorities: args[11].([][32]byte),
		})
		event = "StreamOpened"
	case "streamTopUp":
		_, err = p.keeper.StreamTopUp(ctx, caller, id, payU128(args[1]), key)
		event = "StreamToppedUp"
	case "streamMeter":
		signature := args[3].([]byte)
		if len(signature) != 64 {
			return nil, fmt.Errorf("LXP_NON_CANONICAL: meter signature must be 64 bytes")
		}
		_, err = p.keeper.StreamMeter(ctx, caller, id, args[1].(uint64), args[2].([32]byte), [64]byte(signature), key)
		event = "StreamMetered"
	case "streamSettle":
		key = args[1].([32]byte)
		_, err = p.keeper.StreamSettle(ctx, caller, id, key)
		event = "StreamSettled"
	case "streamPause":
		_, err = p.keeper.StreamPause(ctx, caller, id, key)
		event = "StreamPaused"
	case "streamResume":
		_, err = p.keeper.StreamResume(ctx, caller, id, key)
		event = "StreamResumed"
	case "streamClose":
		key = args[1].([32]byte)
		_, err = p.keeper.StreamClose(ctx, caller, id, key)
		event = "StreamClosed"
	default:
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: %s", method.Name)
	}
	if err != nil {
		return nil, err
	}
	after, err := p.keeper.GetStream(ctx, id)
	if err != nil {
		return nil, err
	}
	change := payStreamChangeRecord{Before: before, After: after}
	var record any = after
	switch method.Name {
	case "streamTopUp", "streamMeter":
		record = change
	case "streamSettle", "streamClose":
		change.Result, err = p.keeper.GetStreamResult(ctx, key)
		if err != nil {
			return nil, err
		}
		record = change
		if method.Name == "streamClose" {
			record = change.Result
		}
	}
	values, err := payEvents[event].pack(args, record)
	if err != nil {
		return nil, err
	}
	if err := emitPayEvent(evm, event, id, values); err != nil {
		return nil, err
	}
	return method.Outputs.Pack()
}

func EmitStreamRateUpdated(evm *vm.EVM, streamID [32]byte, oldRate, newRate, paid codec.U128) error {
	change := payStreamChangeRecord{
		Before: paytypes.StreamRecord{Rate: oldRate},
		After:  paytypes.StreamRecord{Rate: newRate},
		Result: paytypes.StreamResult{Paid: paid},
	}
	values, err := payEvents["StreamRateUpdated"].pack([]interface{}{streamID}, change)
	if err != nil {
		return err
	}
	return emitPayEvent(evm, "StreamRateUpdated", streamID, values)
}
