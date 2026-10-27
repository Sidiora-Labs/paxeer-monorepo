package layerxpay

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	PayAddress     = "0x000000000000000000000000000000000000101B"
	PrecompileName = "layerxpay"
)

//go:embed abi.json
var abiJSON []byte

type Executor struct {
	abi    abi.ABI
	keeper *paykeeper.Keeper
}

var _ pcommon.PrecompileExecutor = (*Executor)(nil)

func NewPrecompile(keeper *paykeeper.Keeper) (*pcommon.Precompile, error) {
	if keeper == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: layerxpay keeper is unavailable")
	}
	a, err := abi.JSON(bytes.NewReader(abiJSON))
	if err != nil {
		return nil, err
	}
	executor := &Executor{abi: a, keeper: keeper}
	return pcommon.NewPrecompile(a, executor, common.HexToAddress(PayAddress), PrecompileName).WithLXPReasons().WithGasTable(methodGas), nil
}

func NewMergePrecompile(keepers utils.Keepers, _ string) (precompiles.IPrecompile, error) {
	wired, ok := keepers.(LayerXPayKeepers)
	if !ok {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: LayerXPayKeepers is required")
	}
	p, err := NewPrecompile(wired.LayerXPayK())
	if err != nil {
		return nil, err
	}
	return p, nil
}

func init() {
	if err := precompiles.RegisterMergeConstructor(common.HexToAddress(PayAddress), NewMergePrecompile); err != nil {
		panic(err)
	}
}

func (p *Executor) RequiredGas(input []byte, method *abi.Method) uint64 {
	if method != nil {
		if row, ok := methodGas[method.Name]; ok {
			return row.Gas(input)
		}
	}
	return pcommon.UnknownMethodCallGas
}

func (p *Executor) Execute(ctx sdk.Context, method *abi.Method, caller common.Address, _ common.Address, args []interface{}, value *big.Int, readOnly bool, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	if err := pcommon.ValidateNonPayable(value); err != nil {
		return nil, err
	}
	if ctx.EVMPrecompileCalledFromDelegateCall() {
		return nil, fmt.Errorf("LXP_UNAUTHORIZED_DEBIT: cannot delegatecall layerxpay")
	}
	if p == nil || p.keeper == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: layerxpay keeper is unavailable")
	}
	if method == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: missing layerxpay method")
	}
	canonical, err := p.abi.MethodById(method.ID)
	if err != nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: %w", err)
	}
	method = canonical
	if readOnly && !method.IsConstant() {
		return nil, fmt.Errorf("LXP_UNAUTHORIZED_DEBIT: layerxpay write under staticcall")
	}
	if err := pcommon.ValidateArgsLength(args, len(method.Inputs)); err != nil {
		return nil, err
	}
	if _, err := method.Inputs.Pack(args...); err != nil {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: %w", err)
	}
	for i, input := range method.Inputs {
		if input.Type.T == abi.UintTy && input.Type.Size == 128 {
			n := args[i].(*big.Int)
			if n == nil || n.Sign() < 0 || n.BitLen() > 128 {
				return nil, fmt.Errorf("LXP_NON_CANONICAL: %s must be uint128", input.Name)
			}
		}
	}
	if method.IsConstant() {
		return p.view(ctx, method, args)
	}
	if err := p.keeper.CheckWrite(ctx, method.Name); err != nil {
		return nil, err
	}
	if _, err := p.keeper.AuthorityFor(ctx, caller); err != nil {
		return nil, err
	}
	var selector [4]byte
	copy(selector[:], method.ID)
	if evm == nil || evm.StateDB == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: layerxpay EVM is unavailable")
	}
	ordinalKey := crypto.Keccak256Hash([]byte("layerxpay/call-ordinal/v1"))
	address := common.HexToAddress(PayAddress)
	counter := evm.StateDB.GetTransientState(address, ordinalKey).Big()
	if !counter.IsUint64() || counter.Uint64() >= 1<<32 {
		return nil, fmt.Errorf("LXP_OVERFLOW: layerxpay call ordinal")
	}
	ordinal := uint32(counter.Uint64())
	key := paytypes.DeriveIdempotencyKey([32]byte(ctx.EVMTxHash()), ordinal, selector)
	var output []byte
	switch method.Name {
	case "escrowOpen", "escrowCapture", "escrowPartialCapture", "escrowRelease", "escrowTimeout", "escrowDisputeOpen", "escrowDisputeResolve":
		output, err = p.escrowHandlers(ctx, method, caller, args, key, evm)
	default:
		var payload []byte
		payload, key, err = payPayload(method.Name, args, key)
		if err == nil {
			_, err = p.keeper.Submit(ctx, caller, method.Name, payload, key)
		}
		if err == nil {
			output, err = method.Outputs.Pack()
		}
	}
	if err != nil {
		return nil, err
	}
	evm.StateDB.SetTransientState(address, ordinalKey, common.BigToHash(counter.Add(counter, big.NewInt(1))))
	return output, nil
}

func payU128(v interface{}) codec.U128 {
	var encoded [16]byte
	v.(*big.Int).FillBytes(encoded[:])
	return codec.U128{Hi: binary.BigEndian.Uint64(encoded[:8]), Lo: binary.BigEndian.Uint64(encoded[8:])}
}

func payPayload(name string, a []interface{}, key [32]byte) ([]byte, [32]byte, error) {
	id := func(i int) [32]byte { return a[i].([32]byte) }
	amount := func(i int) codec.U128 { return payU128(a[i]) }
	u64 := func(i int) uint64 { return a[i].(uint64) }
	var payload []byte
	var err error
	switch name {
	case "escrowOpen":
		payload = paytypes.EncodeEscrowOpen(id(0), id(1), id(2), id(3), id(4), id(5), amount(6), u64(7), u64(8), id(9), id(10))
	case "escrowCapture", "escrowPartialCapture":
		key = id(2)
		payload = paytypes.EncodeEscrowCapture(id(0), amount(1), key)
	case "escrowRelease", "escrowTimeout":
		key = id(1)
		payload = paytypes.EncodeEscrowRelease(id(0), key)
	case "escrowDisputeOpen":
		payload = paytypes.EncodeEscrowDisputeOpen(id(0))
	case "escrowDisputeResolve":
		key = id(2)
		payload = paytypes.EncodeEscrowDisputeResolve(id(0), a[1].(uint32), key)
	case "budgetCreate":
		payload, err = paytypes.EncodeBudgetCreate(id(0), id(1), id(2), id(3), amount(4), amount(5), amount(6), u64(7), u64(8), u64(9), u64(10), a[11].(uint8))
	case "budgetCreateNative":
		payload, err = paytypes.EncodeBudgetCreateNative(id(0), id(1), id(2), id(3), amount(4), amount(5), amount(6), u64(7), u64(8), u64(9), u64(10), a[11].(uint8), id(12), u64(13))
	case "budgetFund":
		payload, err = paytypes.EncodeBudgetFund(id(0), amount(1))
	case "budgetFundNative":
		payload, err = paytypes.EncodeBudgetFundNative(id(0), amount(1), u64(2))
	case "budgetAmend":
		payload, err = paytypes.EncodeBudgetAmend(id(0), amount(1), amount(2), u64(3), a[4].(uint8))
	case "budgetDelegateAdd", "budgetDelegateRemove":
		payload, err = paytypes.EncodeBudgetDelegate(id(0), id(1))
	case "budgetSpend":
		payload, err = paytypes.EncodeBudgetSpend(id(0), id(1), amount(2))
	case "budgetClose":
		payload, err = paytypes.EncodeBudgetClose(id(0), u64(1))
	case "budgetDefund":
		payload, err = paytypes.EncodeBudgetDefund(id(0), amount(1))
	case "budgetRevoke":
		payload, err = paytypes.EncodeBudgetRevoke(id(0), u64(1))
	case "budgetDelegateSpend":
		payload, err = paytypes.EncodeBudgetDelegateSpend(id(0), id(1), amount(2), key)
	case "budgetPull":
		payload, err = paytypes.EncodeBudgetPull(id(0), id(1), amount(2), key, a[3].([]byte))
	case "streamOpen":
		payload, err = paytypes.EncodeStreamOpen(paytypes.StreamOpen{StreamID: id(0), StreamAccount: id(1), Recipient: id(2), AssetID: id(3), Mode: a[4].(uint8), Rate: amount(5), RateUnit: u64(6), StartTimestamp: u64(7), EndTimestamp: u64(8), TotalCap: amount(9), InitialFunding: amount(10), MeterAuthorities: a[11].([][32]byte)})
	case "streamTopUp":
		payload, err = paytypes.EncodeStreamTopUp(id(0), amount(1))
	case "streamMeter":
		signature := a[3].([]byte)
		if len(signature) != 64 {
			return nil, key, fmt.Errorf("LXP_NON_CANONICAL: meter signature must be 64 bytes")
		}
		payload, err = paytypes.EncodeStreamMeter(id(0), u64(1), id(2), [64]byte(signature))
	case "streamSettle", "streamClose":
		key = id(1)
		payload, err = paytypes.EncodeStreamKeyed(id(0), key)
	case "streamPause", "streamResume":
		payload, err = paytypes.EncodeStreamID(id(0))
	case "serviceOfferPublish":
		payload, err = paytypes.EncodeServiceOffer(id(0), id(1), amount(2), id(3), id(4), u64(5), u64(6), u64(7), a[8].(uint8), u64(9))
	case "serviceOfferWithdraw", "serviceAgreementAccept", "serviceAccept":
		payload, err = paytypes.EncodeServiceIdentifier(id(0))
	case "serviceAgreementPropose":
		payload, err = paytypes.EncodeServicePropose(id(0), id(1), id(2), id(3))
	case "serviceCommitTask":
		payload, err = paytypes.EncodeServiceCommit(id(0), id(1), id(2), id(3), u64(4), u64(5))
	case "serviceCommitAbandon":
		payload, err = paytypes.EncodeServiceAbandon(id(0), a[1].(uint16))
	case "serviceToolExecAttest":
		payload, err = paytypes.EncodeServiceToolExecAttest(a[0].([]byte))
	case "serviceProgressReport":
		payload, err = paytypes.EncodeServiceProgress(id(0), id(1), id(2), id(3), a[4].(uint32))
	case "serviceDeliver":
		items := abi.ConvertType(a[2], new([]paytypes.ServiceDeliverable)).(*[]paytypes.ServiceDeliverable)
		payload, err = paytypes.EncodeServiceDeliver(id(0), id(1), *items)
	case "serviceReject":
		payload, err = paytypes.EncodeServiceReject(id(0), a[1].(uint16), a[2].([][32]byte))
	case "serviceDisputeOpen":
		payload, err = paytypes.EncodeServiceDisputeOpen(id(0), id(1), a[2].([][32]byte))
	case "serviceDisputeResolve":
		payload, err = paytypes.EncodeServiceDisputeResolve(id(0), a[1].(uint16), a[2].(uint32), id(3))
	default:
		err = fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: %s", name)
	}
	return payload, key, err
}

func (p *Executor) view(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	if output, handled, err := p.serviceViewHandlers(ctx, method, args); handled {
		return output, err
	}
	id := args[0].([32]byte)
	var record interface{}
	found := true
	var err error
	switch method.Name {
	case "getEscrow", "escrowRemaining", "getEscrowResult":
		return p.escrowHandlers(ctx, method, common.Address{}, args, [32]byte{}, nil)
	case "getStream":
		r, e := p.keeper.GetStream(ctx, id)
		if e != nil {
			return nil, e
		}
		return method.Outputs.Pack(r.ABITuple())
	case "getStreamResult":
		r, e := p.keeper.GetStreamResult(ctx, id)
		if e != nil {
			return nil, e
		}
		return method.Outputs.Pack(r.ABITuple())
	case "streamAccrued":
		accrued, unsettled, e := p.keeper.StreamAccrued(ctx, id)
		if e != nil {
			return nil, e
		}
		return method.Outputs.Pack(accrued.BigInt(), unsettled.BigInt())
	case "getBudget", "budgetRemaining":
		key := paytypes.KernelKVKey(paytypes.BudgetModuleID, append([]byte("budget:"), id[:]...))
		found = false
		for _, pair := range p.keeper.ExportGenesis(ctx).KernelRecords {
			if bytes.Equal(pair.Key, key) {
				record, err = paytypes.DecodeBudgetRecord(pair.Value)
				found = true
				break
			}
		}
		if err != nil {
			return nil, err
		}
		if found && method.Name == "budgetRemaining" {
			r := record.(paytypes.BudgetRecord)
			remaining := new(big.Int).Sub(payAmount(r.PerPeriodLimit), payAmount(r.SpentThisPeriod))
			if remaining.Sign() < 0 {
				return nil, fmt.Errorf("LXP_FATAL_INVARIANT: negative budget remaining")
			}
			return method.Outputs.Pack(remaining)
		}
	default:
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: %s", method.Name)
	}
	if !found {
		return nil, fmt.Errorf("LXP_UNKNOWN_FIELD: %s record not found", method.Name)
	}
	tuple, err := payTuple(method.Outputs[0].Type, reflect.ValueOf(record))
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(tuple.Interface())
}

func payTuple(t abi.Type, record reflect.Value) (reflect.Value, error) {
	tuple := reflect.New(t.GetType()).Elem()
	for i, name := range t.TupleRawNames {
		field := record.FieldByNameFunc(func(candidate string) bool { return strings.EqualFold(candidate, name) })
		if !field.IsValid() {
			return reflect.Value{}, fmt.Errorf("LXP_NON_CANONICAL: missing tuple field %s", name)
		}
		if amount, ok := field.Interface().(codec.U128); ok {
			field = reflect.ValueOf(payAmount(amount))
		}
		target := tuple.Field(i)
		if field.Type().AssignableTo(target.Type()) {
			target.Set(field)
		} else if field.Kind() == reflect.Slice && t.TupleElems[i].Elem.T == abi.TupleTy {
			items := reflect.MakeSlice(target.Type(), field.Len(), field.Len())
			for j := 0; j < field.Len(); j++ {
				item, err := payTuple(*t.TupleElems[i].Elem, field.Index(j))
				if err != nil {
					return reflect.Value{}, err
				}
				items.Index(j).Set(item)
			}
			target.Set(items)
		} else {
			return reflect.Value{}, fmt.Errorf("LXP_NON_CANONICAL: tuple field %s has type %s", name, field.Type())
		}
	}
	return tuple, nil
}
