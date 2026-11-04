package internal

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/executor/precompiles"
	"github.com/ethereum/evmc/v12/bindings/go/evmc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

var _ evmc.HostContext = (*HostContext)(nil)

// StandardSstoreSetGasEIP2200 is the standard EIP-2200 SSTORE gas cost for setting
// storage from zero to non-zero. evmone uses this internally.
const StandardSstoreSetGasEIP2200 = uint64(20000)

// HostContextConfig holds configuration for the HostContext.
// Values are pre-computed from ChainConfig at construction time for efficiency.
type HostContextConfig struct {
	// SstoreGasDelta is the per-SSTORE gas delta (Pax custom cost - standard 20k).
	// This is added to gas consumption for each StorageAdded operation.
	// Can be negative if Pax cost is below standard (results in gas reduction).
	SstoreGasDelta int64
}

// NewHostContextConfig creates a HostContextConfig from the chain config.
// This extracts and pre-computes values needed by HostContext.
func NewHostContextConfig(chainConfig *params.ChainConfig) HostContextConfig {
	var delta int64
	if chainConfig != nil && chainConfig.PaxSstoreSetGasEIP2200 != nil {
		paxSstoreGas := *chainConfig.PaxSstoreSetGasEIP2200

		// Guard against overflow: paxSstoreGas is uint64, and casting to int64 would
		// overflow if value > math.MaxInt64. This is a critical misconfiguration.
		if paxSstoreGas > uint64(math.MaxInt64) {
			panic(fmt.Sprintf("PaxSstoreSetGasEIP2200 (%d) exceeds maximum safe value (%d)",
				paxSstoreGas, uint64(math.MaxInt64)))
		}

		// Delta = Pax cost - standard cost (can be positive or negative)
		// Safe to cast now that we've verified paxSstoreGas <= math.MaxInt64
		delta = int64(paxSstoreGas) - int64(StandardSstoreSetGasEIP2200)
	}
	return HostContextConfig{
		SstoreGasDelta: delta,
	}
}

type HostContext struct {
	vm              *evmc.VM
	evm             *vm.EVM
	config          HostContextConfig
	sstoreGasMu     sync.Mutex
	sstoreGasFrames []sstoreGasFrame
	abortErr        error
	staticFrames    []bool
}

type UnsupportedEVMRevisionError struct{}

func (*UnsupportedEVMRevisionError) Error() string {
	return "evmone does not support the active Prague, Osaka or Verkle rules"
}

func (*UnsupportedEVMRevisionError) IsAbortError() bool { return true }

func (h *HostContext) beginTransaction() int {
	h.abortErr = nil
	return h.evm.StateDB.Snapshot()
}

func (h *HostContext) endTransaction(snapshot int) {
	if h.abortErr != nil {
		h.evm.StateDB.RevertToSnapshot(snapshot)
	}
}

type sstoreGasFrame struct {
	adjustment int64
	overflow   bool
}

// NewHostContext creates a new HostContext with the given configuration.
func NewHostContext(vm *evmc.VM, evm *vm.EVM, config HostContextConfig) *HostContext {
	return &HostContext{
		vm:     vm,
		evm:    evm,
		config: config,
	}
}

func (h *HostContext) BeginSstoreGasAdjustment() {
	h.sstoreGasMu.Lock()
	h.sstoreGasFrames = append(h.sstoreGasFrames, sstoreGasFrame{})
	h.sstoreGasMu.Unlock()
}

func (h *HostContext) addSstoreGasAdjustment(delta int64) {
	h.sstoreGasMu.Lock()
	defer h.sstoreGasMu.Unlock()
	if len(h.sstoreGasFrames) == 0 {
		return
	}
	frame := &h.sstoreGasFrames[len(h.sstoreGasFrames)-1]
	if (delta > 0 && frame.adjustment > math.MaxInt64-delta) ||
		(delta < 0 && frame.adjustment < math.MinInt64-delta) {
		frame.overflow = true
		return
	}
	frame.adjustment += delta
}

func (h *HostContext) EndSstoreGasAdjustment() (int64, bool) {
	h.sstoreGasMu.Lock()
	defer h.sstoreGasMu.Unlock()
	if len(h.sstoreGasFrames) == 0 {
		return 0, true
	}
	last := len(h.sstoreGasFrames) - 1
	frame := h.sstoreGasFrames[last]
	h.sstoreGasFrames = h.sstoreGasFrames[:last]
	return frame.adjustment, frame.overflow
}

func (h *HostContext) AccountExists(addr evmc.Address) bool {
	return h.evm.StateDB.Exist(common.Address(addr))
}

func (h *HostContext) GetStorage(addr evmc.Address, key evmc.Hash) evmc.Hash {
	return evmc.Hash(h.evm.StateDB.GetState(common.Address(addr), common.Hash(key)))
}

func (h *HostContext) SetStorage(addr evmc.Address, key evmc.Hash, value evmc.Hash) evmc.StorageStatus {
	gethAddr := common.Address(addr)
	gethKey := common.Hash(key)

	current := h.evm.StateDB.GetState(gethAddr, gethKey)
	original := h.evm.StateDB.GetCommittedState(gethAddr, gethKey)

	newValue := common.Hash(value)
	status := evmc.StorageAssigned
	if current != newValue {
		switch {
		case original == current:
			switch {
			case original == (common.Hash{}):
				status = evmc.StorageAdded
			case newValue == (common.Hash{}):
				status = evmc.StorageDeleted
			default:
				status = evmc.StorageModified
			}
		case original == newValue:
			switch {
			case original == (common.Hash{}):
				status = evmc.StorageAddedDeleted
			case current == (common.Hash{}):
				status = evmc.StorageDeletedRestored
			default:
				status = evmc.StorageModifiedRestored
			}
		case original != (common.Hash{}):
			if current == (common.Hash{}) {
				status = evmc.StorageDeletedAdded
			} else if newValue == (common.Hash{}) {
				status = evmc.StorageModifiedDeleted
			}
		}
	}

	// Accumulate SSTORE gas adjustment for StorageAdded operations.
	// evmone uses standard EIP-2200 gas (20k), but Pax may have a different cost.
	// We track the delta here and apply it after execution.
	// Delta can be positive (higher cost) or negative (lower cost).
	if status == evmc.StorageAdded && h.config.SstoreGasDelta != 0 {
		h.addSstoreGasAdjustment(h.config.SstoreGasDelta)
	}

	h.evm.StateDB.SetState(gethAddr, gethKey, common.Hash(value))
	return status
}

func (h *HostContext) GetBalance(addr evmc.Address) evmc.Hash {
	return h.evm.StateDB.GetBalance(common.Address(addr)).Bytes32()
}

func (h *HostContext) GetCodeSize(addr evmc.Address) int {
	return h.evm.StateDB.GetCodeSize(common.Address(addr))
}

func (h *HostContext) GetCodeHash(addr evmc.Address) evmc.Hash {
	return evmc.Hash(h.evm.StateDB.GetCodeHash(common.Address(addr)))
}

func (h *HostContext) GetCode(addr evmc.Address) []byte {
	return h.evm.StateDB.GetCode(common.Address(addr))
}

// todo(pdrobnjak): support historical selfdestruct logic as well
func (h *HostContext) Selfdestruct(addr evmc.Address, beneficiary evmc.Address) bool {
	addrKey := common.Address(addr)
	beneficiaryKey := common.Address(beneficiary)
	amt := h.evm.StateDB.GetBalance(addrKey)
	h.evm.StateDB.SubBalance(addrKey, amt, tracing.BalanceDecreaseSelfdestruct)
	h.evm.StateDB.AddBalance(beneficiaryKey, amt, tracing.BalanceIncreaseSelfdestruct)
	h.evm.StateDB.SelfDestruct6780(common.Address(addr))
	return true
}

func (h *HostContext) GetTxContext() evmc.TxContext {
	var gasPrice evmc.Hash
	if h.evm.GasPrice != nil {
		h.evm.GasPrice.FillBytes(gasPrice[:])
	}

	var prevRandao evmc.Hash
	if h.evm.Context.Random != nil {
		prevRandao = evmc.Hash(*h.evm.Context.Random)
	}

	var chainID evmc.Hash
	if h.evm.ChainConfig() != nil && h.evm.ChainConfig().ChainID != nil {
		h.evm.ChainConfig().ChainID.FillBytes(chainID[:])
	}

	var baseFee evmc.Hash
	if h.evm.Context.BaseFee != nil {
		h.evm.Context.BaseFee.FillBytes(baseFee[:])
	}

	var blobBaseFee evmc.Hash
	if h.evm.Context.BlobBaseFee != nil {
		h.evm.Context.BlobBaseFee.FillBytes(blobBaseFee[:])
	}

	//nolint:gosec // G115: safe integer conversions for Time and GasLimit
	return evmc.TxContext{
		GasPrice:    gasPrice,
		Origin:      evmc.Address(h.evm.Origin),
		Coinbase:    evmc.Address(h.evm.Context.Coinbase),
		Number:      h.evm.Context.BlockNumber.Int64(),
		Timestamp:   int64(h.evm.Context.Time),
		GasLimit:    int64(h.evm.Context.GasLimit),
		PrevRandao:  prevRandao,
		ChainID:     chainID,
		BaseFee:     baseFee,
		BlobBaseFee: blobBaseFee,
	}
}

func (h *HostContext) GetBlockHash(number int64) evmc.Hash {
	if number < 0 {
		return evmc.Hash{}
	}
	return evmc.Hash(h.evm.Context.GetHash(uint64(number)))
}

func (h *HostContext) EmitLog(addr evmc.Address, topics []evmc.Hash, data []byte) {
	gethTopics := make([]common.Hash, len(topics))
	for i, topic := range topics {
		gethTopics[i] = common.Hash(topic)
	}
	h.evm.StateDB.AddLog(&ethtypes.Log{Address: common.Address(addr), Topics: gethTopics, Data: data})
}

func (h *HostContext) Execute(kind evmc.CallKind, recipient evmc.Address, sender evmc.Address, value evmc.Hash, input, code []byte, gas int64,
	depth int, static bool) ([]byte, int64, int64, evmc.Address, error) {
	evmRevision, err := h.getEVMRevision()
	if err != nil {
		return nil, 0, 0, evmc.Address{}, err
	}
	if h.abortErr != nil {
		return nil, 0, 0, evmc.Address{}, h.abortErr
	}
	if h.vm == nil {
		return nil, 0, 0, evmc.Address{}, fmt.Errorf("evmone runtime is unavailable")
	}
	h.staticFrames = append(h.staticFrames, static)
	defer func() { h.staticFrames = h.staticFrames[:len(h.staticFrames)-1] }()
	// contract.Code is already resolved by geth, including DELEGATECALL and
	// EIP-7702. The storage recipient is not necessarily the code account.
	result, err := h.vm.Execute(h, evmRevision, kind, static, false, depth,
		gas, recipient, sender, input, value, code)
	if h.abortErr != nil {
		return nil, 0, 0, evmc.Address{}, h.abortErr
	}
	if err != nil && !errors.Is(err, evmc.Revert) {
		return nil, 0, 0, evmc.Address{}, err
	}
	return result.Output, result.GasLeft, result.GasRefund, evmc.Address{}, err
}

func (h *HostContext) callbackError(err error) error {
	var abort vm.AbortError
	if errors.As(err, &abort) && abort.IsAbortError() && h.abortErr == nil {
		h.abortErr = abort
	}
	return toEvmcError(err)
}

func (h *HostContext) Call(
	kind evmc.CallKind, recipient evmc.Address, sender evmc.Address, value evmc.Hash, input []byte, gas int64,
	_ int, static bool, salt evmc.Hash, codeAddress evmc.Address,
) ([]byte, int64, int64, evmc.Address, error) {
	if h.abortErr != nil {
		return nil, 0, 0, evmc.Address{}, evmc.Failure
	}
	if gas < 0 {
		return nil, 0, 0, evmc.Address{}, evmc.Failure
	}
	recipientAddr := common.Address(recipient)
	senderAddr := common.Address(sender)
	valueUint256 := new(uint256.Int).SetBytes(value[:])
	if len(h.staticFrames) > 0 && h.staticFrames[len(h.staticFrames)-1] {
		static = true
	}
	if static && (kind == evmc.Create || kind == evmc.Create2 ||
		(kind == evmc.Call && !valueUint256.IsZero())) {
		return nil, 0, 0, evmc.Address{}, evmc.Error(11) // EVMC_STATIC_MODE_VIOLATION
	}
	var ret []byte
	var leftoverGas uint64
	var err error
	var createAddr common.Address

	//nolint:gosec // G115: safe integer conversions for gas values
	switch kind {
	case evmc.Call:
		if static {
			ret, leftoverGas, err = h.evm.StaticCall(senderAddr, recipientAddr, input, uint64(gas))
		} else {
			ret, leftoverGas, err = h.evm.Call(senderAddr, recipientAddr, input, uint64(gas), valueUint256)
		}
	case evmc.DelegateCall:
		ret, leftoverGas, err = h.evm.DelegateCall(
			senderAddr, recipientAddr, common.Address(codeAddress), input, uint64(gas), valueUint256,
		)
	case evmc.CallCode:
		ret, leftoverGas, err = h.evm.CallCode(recipientAddr, common.Address(codeAddress), input, uint64(gas), valueUint256)
	case evmc.Create:
		ret, createAddr, leftoverGas, err = h.evm.Create(senderAddr, input, uint64(gas), valueUint256)
		return ret, int64(leftoverGas), 0, evmc.Address(createAddr), h.callbackError(err)
	case evmc.Create2:
		saltUint256 := new(uint256.Int).SetBytes(salt[:])
		ret, createAddr, leftoverGas, err = h.evm.Create2(senderAddr, input, uint64(gas), valueUint256, saltUint256)
		return ret, int64(leftoverGas), 0, evmc.Address(createAddr), h.callbackError(err)
	default:
		return nil, 0, 0, evmc.Address{}, evmc.Failure
	}

	//nolint:gosec // G115: safe, leftoverGas won't exceed int64 max
	return ret, int64(leftoverGas), 0, evmc.Address{}, h.callbackError(err)
}

func (h *HostContext) AccessAccount(addr evmc.Address) evmc.AccessStatus {
	gethAddr := common.Address(addr)
	addrInAccessList := h.evm.StateDB.AddressInAccessList(gethAddr)
	if addrInAccessList {
		return evmc.WarmAccess
	}
	// After a cold access, add address to access list so subsequent accesses are warm
	h.evm.StateDB.AddAddressToAccessList(gethAddr)
	return evmc.ColdAccess
}

func (h *HostContext) AccessStorage(addr evmc.Address, key evmc.Hash) evmc.AccessStatus {
	gethAddr := common.Address(addr)
	gethKey := common.Hash(key)
	addrInAccessList, slotInAccessList := h.evm.StateDB.SlotInAccessList(gethAddr, gethKey)
	if addrInAccessList && slotInAccessList {
		return evmc.WarmAccess
	}
	// After a cold access, add slot to access list so subsequent accesses are warm
	h.evm.StateDB.AddSlotToAccessList(gethAddr, gethKey)
	return evmc.ColdAccess
}

func (h *HostContext) GetTransientStorage(addr evmc.Address, key evmc.Hash) evmc.Hash {
	return evmc.Hash(h.evm.StateDB.GetTransientState(common.Address(addr), common.Hash(key)))
}

func (h *HostContext) SetTransientStorage(addr evmc.Address, key evmc.Hash, value evmc.Hash) {
	h.evm.StateDB.SetTransientState(common.Address(addr), common.Hash(key), common.Hash(value))
}

// getEVMRevision determines the EVM revision based on the current chain configuration
func (h *HostContext) getEVMRevision() (evmc.Revision, error) {
	chainConfig := h.evm.ChainConfig()
	blockNumber := h.evm.Context.BlockNumber
	time := h.evm.Context.Time
	isMerge := h.evm.Context.Random != nil

	// Get the rules for the current block
	rules := chainConfig.Rules(blockNumber, isMerge, time)

	// The packaged runtime targets revisions through Cancun. Never execute a
	// newer consensus revision using an older schedule.
	if rules.IsPrague || rules.IsOsaka || rules.IsVerkle {
		return 0, &UnsupportedEVMRevisionError{}
	}
	if rules.IsCancun {
		return evmc.Cancun, nil
	}
	if rules.IsShanghai {
		return evmc.Shanghai, nil
	}
	if rules.IsMerge {
		return evmc.Paris, nil
	}
	if rules.IsLondon {
		return evmc.London, nil
	}
	if rules.IsBerlin {
		return evmc.Berlin, nil
	}
	if rules.IsIstanbul {
		return evmc.Istanbul, nil
	}
	if rules.IsPetersburg {
		return evmc.Petersburg, nil
	}
	if rules.IsConstantinople {
		return evmc.Constantinople, nil
	}
	if rules.IsByzantium {
		return evmc.Byzantium, nil
	}
	if rules.IsEIP158 {
		return evmc.SpuriousDragon, nil
	}
	if rules.IsEIP150 {
		return evmc.TangerineWhistle, nil
	}
	if rules.IsHomestead {
		return evmc.Homestead, nil
	}
	return evmc.Frontier, nil
}

// toEvmcError converts a Go error to an evmc.Error.
// The EVMC bindings expect Call() to return evmc.Error type, not standard Go errors.
//
// Note: The evmc Go bindings currently only expose evmc.Failure and evmc.Revert constants.
// Additional error codes like EVMC_OUT_OF_GAS (3), EVMC_INVALID_INSTRUCTION (4), etc.
// are defined in the C header (evmc/evmc.h) but not exported in the Go bindings.
// To add proper mapping for vm.ErrOutOfGas -> evmc.OutOfGas, the Go bindings in
// github.com/ethereum/evmc would need to be extended first.
func toEvmcError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.As(err, new(evmc.Error)):
		// Already an evmc.Error, return as-is
		return err
	case errors.Is(err, vm.ErrExecutionReverted):
		return evmc.Revert
	default:
		// All other errors map to generic failure
		// TODO: Add evmc.OutOfGas mapping once the Go bindings expose it
		return evmc.Failure
	}
}

// To be called by an exported EVM create function which knows how to instantiate params like statedb.
func createEVMWithFailFastPrecompile(blockContext vm.BlockContext, statedb vm.StateDB, chainConfig *params.ChainConfig, vmConfig vm.Config) *vm.EVM {
	return vm.NewEVM(blockContext, statedb, chainConfig, vmConfig, precompiles.AllCustomPrecompilesFailFast)
}
