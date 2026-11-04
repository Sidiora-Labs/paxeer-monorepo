package internal

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/executor/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/executor/utils"
	"github.com/ethereum/evmc/v12/bindings/go/evmc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

var repairM1131Caller = common.HexToAddress("0xa1")
var repairM1131Recipient = common.HexToAddress("0xb2")
var repairM1131Code = common.HexToAddress("0xc3")

func repairM1131Host(t *testing.T, prague bool) (*HostContext, *state.StateDB) {
	t.Helper()
	db, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	require.NoError(t, err)
	cfg := *params.AllDevChainProtocolChanges
	cfg.OsakaTime, cfg.VerkleTime = nil, nil
	if !prague {
		cfg.PragueTime = nil
	}
	ctx := vm.BlockContext{
		CanTransfer: core.CanTransfer, Transfer: core.Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		BlockNumber: big.NewInt(1), Time: 1, GasLimit: 30000000,
		BaseFee: big.NewInt(0), BlobBaseFee: big.NewInt(1), Random: new(common.Hash),
	}
	evm := vm.NewEVM(ctx, db, &cfg, vm.Config{}, nil)
	evm.SetTxContext(vm.TxContext{Origin: common.HexToAddress("0xdead"), GasPrice: big.NewInt(0)})
	db.SetBalance(repairM1131Caller, uint256.NewInt(1000000), tracing.BalanceChangeUnspecified)
	db.SetBalance(repairM1131Recipient, uint256.NewInt(1000000), tracing.BalanceChangeUnspecified)
	rules := cfg.Rules(ctx.BlockNumber, true, ctx.Time)
	db.Prepare(rules, repairM1131Caller, ctx.Coinbase, &repairM1131Recipient, vm.ActivePrecompiles(rules), nil)
	return NewHostContext(nil, evm, HostContextConfig{}), db
}

func TestRepairM_1131_022_028_StorageTransitions(t *testing.T) {
	// Exhaust all equality partitions, including three distinct nonzero words.
	// Each row is current=0..3, each column is new=0..3, for original=0..3.
	const A = evmc.StorageAssigned
	const N = evmc.StorageAdded
	const D = evmc.StorageDeleted
	const M = evmc.StorageModified
	const DA = evmc.StorageDeletedAdded
	const MD = evmc.StorageModifiedDeleted
	const DR = evmc.StorageDeletedRestored
	const AD = evmc.StorageAddedDeleted
	const MR = evmc.StorageModifiedRestored
	expected := [4][4][4]evmc.StorageStatus{
		{{A, N, N, N}, {AD, A, A, A}, {AD, A, A, A}, {AD, A, A, A}},
		{{A, DR, DA, DA}, {D, A, M, M}, {MD, MR, A, A}, {MD, MR, A, A}},
		{{A, DA, DR, DA}, {MD, A, MR, A}, {D, M, A, M}, {MD, A, MR, A}},
		{{A, DA, DA, DR}, {MD, A, A, MR}, {MD, A, A, MR}, {D, M, M, A}},
	}
	for o := 0; o < 4; o++ {
		for c := 0; c < 4; c++ {
			for n := 0; n < 4; n++ {
				t.Run(fmt.Sprintf("%d_%d_%d", o, c, n), func(t *testing.T) {
					h, db := repairM1131Host(t, false)
					addr, key := repairM1131Recipient, common.Hash{}
					original := common.BigToHash(big.NewInt(int64(o)))
					current := common.BigToHash(big.NewInt(int64(c)))
					next := common.BigToHash(big.NewInt(int64(n)))
					db.SetState(addr, key, original)
					db.Finalise(false)
					require.Equal(t, original, db.GetCommittedState(addr, key))
					db.SetState(addr, key, current)
					snapshot := db.Snapshot()
					h.config.SstoreGasDelta = 52000
					h.BeginSstoreGasAdjustment()
					got := h.SetStorage(evmc.Address(addr), evmc.Hash(key), evmc.Hash(next))
					delta, overflow := h.EndSstoreGasAdjustment()
					require.Equal(t, expected[o][c][n], got)
					require.Equal(t, next, db.GetState(addr, key))
					require.Equal(t, original, db.GetCommittedState(addr, key))
					require.False(t, overflow)
					wantDelta := int64(0)
					if got == evmc.StorageAdded {
						wantDelta = 52000
					}
					require.Equal(t, wantDelta, delta)
					db.RevertToSnapshot(snapshot)
					require.Equal(t, current, db.GetState(addr, key))
				})
			}
		}
	}
	TestStorageStatusLogic(t)
}

func TestRepairM_1131_022_StorageGasReference(t *testing.T) {
	// Independent EIP-2200 Istanbul vectors. These are geth reference proofs;
	// evmone differential qualification remains an explicit wiring request.
	cases := []struct {
		o, c, n     byte
		gas, refund uint64
	}{
		{0, 0, 0, 1612, 0}, {0, 0, 1, 20812, 0}, {0, 1, 0, 20812, 19200},
		{0, 1, 2, 20812, 0}, {0, 1, 1, 20812, 0}, {1, 0, 0, 5812, 15000},
		{1, 0, 1, 5812, 4200}, {1, 0, 2, 5812, 0}, {1, 2, 0, 5812, 15000},
		{1, 2, 3, 5812, 0}, {1, 2, 1, 5812, 4200}, {1, 2, 2, 5812, 0},
		{1, 1, 0, 5812, 15000}, {1, 1, 2, 5812, 0}, {1, 1, 1, 1612, 0},
	}
	for _, tt := range cases {
		t.Run(fmt.Sprintf("%d_%d_%d", tt.o, tt.c, tt.n), func(t *testing.T) {
			h, db := repairM1131Host(t, false)
			cfg := *params.AllEthashProtocolChanges
			cfg.BerlinBlock, cfg.LondonBlock = nil, nil
			h.evm = vm.NewEVM(h.evm.Context, db, &cfg, vm.Config{}, nil)
			code := []byte{0x60, tt.c, 0x60, 0, 0x55, 0x60, tt.n, 0x60, 0, 0x55}
			db.SetCode(repairM1131Recipient, code)
			db.SetState(repairM1131Recipient, common.Hash{}, common.BytesToHash([]byte{tt.o}))
			db.Finalise(false)
			_, left, err := h.evm.Call(repairM1131Caller, repairM1131Recipient, nil, 100000, new(uint256.Int))
			require.NoError(t, err)
			require.Equal(t, tt.gas, 100000-left)
			require.Equal(t, tt.refund, db.GetRefund())
		})
	}
}

func repairM1131CreationAndCalls(t *testing.T) {
	for _, kind := range []evmc.CallKind{evmc.Create, evmc.Create2} {
		t.Run(callKindName(kind), func(t *testing.T) {
			h, db := repairM1131Host(t, false)
			init := common.FromHex("0x600060005360016000f3")
			salt := evmc.Hash{31: 7}
			output, left, refund, addr, err := h.Call(kind, evmc.Address{}, evmc.Address(repairM1131Caller), evmc.Hash{}, init, 100000, 0, false, salt, evmc.Address{})
			require.NoError(t, err)
			require.Equal(t, []byte{0}, output)
			require.Equal(t, int64(0), refund)
			expected := crypto.CreateAddress(repairM1131Caller, 0)
			if kind == evmc.Create2 {
				expected = crypto.CreateAddress2(repairM1131Caller, [32]byte(salt), crypto.Keccak256(init))
			}
			require.Equal(t, expected, common.Address(addr))
			require.Equal(t, []byte{0}, db.GetCode(expected))
			ref, _ := repairM1131Host(t, false)
			var refGas uint64
			var refAddr common.Address
			if kind == evmc.Create {
				_, refAddr, refGas, err = ref.evm.Create(repairM1131Caller, init, 100000, new(uint256.Int))
			} else {
				_, refAddr, refGas, err = ref.evm.Create2(repairM1131Caller, init, 100000, new(uint256.Int), uint256.NewInt(7))
			}
			require.NoError(t, err)
			require.Equal(t, refAddr, common.Address(addr))
			require.Equal(t, int64(refGas), left)
		})
	}
	h, db := repairM1131Host(t, false)
	// Echo calldata: distinct code and input catch accidental calldata-as-code.
	db.SetCode(repairM1131Recipient, common.FromHex("0x366000600037366000f3"))
	input := []byte{0xfe, 0xca, 0xfe}
	output, left, _, _, err := h.Call(evmc.Call, evmc.Address(repairM1131Recipient), evmc.Address(repairM1131Caller), evmc.Hash{}, input, 100000, 0, false, evmc.Hash{}, evmc.Address(repairM1131Recipient))
	require.NoError(t, err)
	require.Equal(t, input, output)
	require.Less(t, left, int64(100000))
}

func TestRepairM_1131_029_CreationAndCalls(t *testing.T) { repairM1131CreationAndCalls(t) }

func repairM1131DelegatedCode(t *testing.T) {
	for _, prague := range []bool{false, true} {
		t.Run(fmt.Sprint(prague), func(t *testing.T) {
			h, db := repairM1131Host(t, prague)
			target := repairM1131Code
			// Store caller, address, callvalue and first calldata word in the recipient.
			code := common.FromHex("0x33600055306001553460025560003560035500")
			db.SetCode(target, code)
			if prague {
				delegate := common.HexToAddress("0xd4")
				db.SetCode(delegate, code)
				db.SetCode(target, types.AddressToDelegation(delegate))
			}
			db.SetCode(repairM1131Recipient, []byte{0xfe})
			input := common.LeftPadBytes([]byte{42}, 32)
			_, left, _, _, err := h.Call(evmc.DelegateCall, evmc.Address(repairM1131Recipient), evmc.Address(repairM1131Caller), evmc.Hash{31: 9}, input, 200000, 1, false, evmc.Hash{}, evmc.Address(target))
			require.NoError(t, err)
			require.Equal(t, common.BytesToHash(repairM1131Caller.Bytes()), db.GetState(repairM1131Recipient, common.Hash{}))
			require.Equal(t, common.BytesToHash(repairM1131Recipient.Bytes()), db.GetState(repairM1131Recipient, common.Hash{31: 1}))
			require.Equal(t, common.Hash{31: 9}, db.GetState(repairM1131Recipient, common.Hash{31: 2}))
			require.Equal(t, common.Hash{31: 42}, db.GetState(repairM1131Recipient, common.Hash{31: 3}))
			require.Equal(t, common.Hash{}, db.GetState(target, common.Hash{}))
			ref, refDB := repairM1131Host(t, prague)
			refDB.SetCode(target, db.GetCode(target))
			if prague {
				refDB.SetCode(common.HexToAddress("0xd4"), code)
			}
			_, refGas, err := ref.evm.DelegateCall(repairM1131Caller, repairM1131Recipient, target, input, 200000, uint256.NewInt(9))
			require.NoError(t, err)
			require.Equal(t, int64(refGas), left)
		})
	}
	// CALLCODE uses the recipient's storage and caller identity too.
	h, db := repairM1131Host(t, false)
	db.SetCode(repairM1131Code, common.FromHex("0x3360005500"))
	_, _, _, _, err := h.Call(evmc.CallCode, evmc.Address(repairM1131Recipient), evmc.Address(repairM1131Caller), evmc.Hash{}, nil, 100000, 1, false, evmc.Hash{}, evmc.Address(repairM1131Code))
	require.NoError(t, err)
	require.Equal(t, common.BytesToHash(repairM1131Recipient.Bytes()), db.GetState(repairM1131Recipient, common.Hash{}))
}

func TestRepairM_1131_024_DelegatedCode(t *testing.T) { repairM1131DelegatedCode(t) }

func repairM1131NestedCall(op vm.OpCode, target common.Address, value byte) []byte {
	// retSize, retOffset, argsSize, argsOffset, [value], target, gas.
	code := common.FromHex("0x6000600060006000")
	if op == vm.CALL || op == vm.CALLCODE {
		code = append(code, 0x60, value)
	}
	code = append(code, 0x73)
	code = append(code, target[:]...)
	code = append(code, 0x62, 0x01, 0x86, 0xa0, byte(op))
	return code
}

func repairM1131StaticDescendants(t *testing.T) {
	for _, op := range []vm.OpCode{vm.CALL, vm.DELEGATECALL} {
		for name, child := range map[string][]byte{
			"sstore": common.FromHex("0x600160005500"),
			"log":    common.FromHex("0x60006000a000"),
			"create": common.FromHex("0x600060006000f000"),
			"value":  append(repairM1131NestedCall(vm.CALL, common.HexToAddress("0xe5"), 1), 0x00),
		} {
			t.Run(op.String()+"_"+name, func(t *testing.T) {
				h, db := repairM1131Host(t, false)
				db.SetCode(repairM1131Code, child)
				// Return the nested call's success bit. A denied child returns zero.
				parent := append(repairM1131NestedCall(op, repairM1131Code, 0), common.FromHex("0x60005260206000f3")...)
				db.SetCode(repairM1131Recipient, parent)
				before := db.GetBalance(repairM1131Recipient).Clone()
				out, _, _, _, err := h.Call(evmc.Call, evmc.Address(repairM1131Recipient), evmc.Address(repairM1131Caller), evmc.Hash{}, nil, 300000, 0, true, evmc.Hash{}, evmc.Address(repairM1131Recipient))
				require.NoError(t, err)
				require.Equal(t, make([]byte, 32), out)
				require.Equal(t, common.Hash{}, db.GetState(repairM1131Recipient, common.Hash{}))
				require.Equal(t, common.Hash{}, db.GetState(repairM1131Code, common.Hash{}))
				require.Empty(t, db.Logs())
				require.Equal(t, before, db.GetBalance(repairM1131Recipient))
				require.Zero(t, db.GetNonce(repairM1131Recipient))
				interpreter := NewEVMInterpreter(h, h.evm)
				interpreter.readOnly = true
				_, static, err := interpreter.callMode(op)
				require.NoError(t, err)
				require.True(t, static)
			})
		}
	}
	h, db := repairM1131Host(t, false)
	before := db.GetBalance(repairM1131Caller).Clone()
	for _, kind := range []evmc.CallKind{evmc.Call, evmc.Create, evmc.Create2} {
		_, _, _, _, err := h.Call(kind, evmc.Address(repairM1131Recipient), evmc.Address(repairM1131Caller), evmc.Hash{31: 1}, nil, 100000, 0, true, evmc.Hash{}, evmc.Address(repairM1131Recipient))
		require.Equal(t, evmc.Error(11), err)
	}
	require.Equal(t, before, db.GetBalance(repairM1131Caller))
}

func TestRepairM_1131_023_StaticDescendants(t *testing.T) { repairM1131StaticDescendants(t) }

func repairM1131RevertAndGas(t *testing.T) {
	h, db := repairM1131Host(t, false)
	db.SetCode(repairM1131Code, common.FromHex("0x6001600055602a60005360016000fd"))
	output, left, refund, _, err := h.Call(evmc.Call, evmc.Address(repairM1131Code), evmc.Address(repairM1131Caller), evmc.Hash{}, nil, 100000, 0, false, evmc.Hash{}, evmc.Address(repairM1131Code))
	require.Equal(t, evmc.Revert, err)
	require.Equal(t, []byte{42}, output)
	require.Positive(t, left)
	require.Zero(t, refund)
	require.Equal(t, common.Hash{}, db.GetState(repairM1131Code, common.Hash{}))
	interpreter := NewEVMInterpreter(h, h.evm)
	contract := vm.NewContract(repairM1131Caller, repairM1131Code, new(uint256.Int), 100000, nil)
	result, resultErr := interpreter.applyExecutionResult(contract, output, left, refund, 0, false, err)
	require.ErrorIs(t, resultErr, vm.ErrExecutionReverted)
	require.Equal(t, output, result)
	require.Equal(t, uint64(left), contract.Gas)
	require.Zero(t, db.GetRefund())
	// Catch the child revert and return RETURNDATA from the parent.
	parent := append(repairM1131NestedCall(vm.CALL, repairM1131Code, 0), common.FromHex("0x503d600060003e3d6000f3")...)
	db.SetCode(repairM1131Recipient, parent)
	out, _, _, _, err := h.Call(evmc.Call, evmc.Address(repairM1131Recipient), evmc.Address(repairM1131Caller), evmc.Hash{}, nil, 300000, 0, false, evmc.Hash{}, evmc.Address(repairM1131Recipient))
	require.NoError(t, err)
	require.Equal(t, []byte{42}, out)
	require.Equal(t, common.Hash{}, db.GetState(repairM1131Code, common.Hash{}))
}

func TestRepairM_1131_025_RevertAndGas(t *testing.T) { repairM1131RevertAndGas(t) }

func repairM1131Abort(t *testing.T) {
	h, db := repairM1131Host(t, false)
	native := common.HexToAddress("0x101a")
	h.evm.SetPrecompiles(vm.PrecompiledContracts{native: precompiles.FailFastSingleton})
	snapshot := h.beginTransaction()
	db.SetState(repairM1131Recipient, common.Hash{}, common.Hash{31: 1})
	_, _, _, _, err := h.Call(evmc.Call, evmc.Address(native), evmc.Address(repairM1131Recipient), evmc.Hash{}, nil, 100000, 1, false, evmc.Hash{}, evmc.Address(native))
	require.Equal(t, evmc.Failure, err)
	// An EVMC parent is allowed to catch FAILURE and keep writing. Its result
	// cannot clear the transaction abort carried outside the EVMC status code.
	h.SetStorage(evmc.Address(repairM1131Recipient), evmc.Hash{31: 1}, evmc.Hash{31: 2})
	interpreter := NewEVMInterpreter(h, h.evm)
	contract := vm.NewContract(repairM1131Caller, repairM1131Recipient, new(uint256.Int), 100000, nil)
	_, err = interpreter.applyExecutionResult(contract, nil, 90000, 0, 0, false, nil)
	require.Same(t, precompiles.ErrInvalidPrecompileCall, err)
	require.True(t, utils.ShouldExecutionAbort(err))
	h.endTransaction(snapshot)
	require.Equal(t, common.Hash{}, db.GetState(repairM1131Recipient, common.Hash{}))
	require.Equal(t, common.Hash{}, db.GetState(repairM1131Recipient, common.Hash{31: 1}))
	// Retry on the ordinary interpreter with a real native SHA256 precompile.
	h.evm.SetPrecompiles(vm.PrecompiledContracts{native: vm.PrecompiledContractsBerlin[common.BytesToAddress([]byte{2})]})
	code := append(repairM1131NestedCall(vm.CALL, native, 0), common.FromHex("0x50600160005500")...)
	db.SetCode(repairM1131Recipient, code)
	_, _, err = h.evm.Call(repairM1131Caller, repairM1131Recipient, nil, 300000, new(uint256.Int))
	require.NoError(t, err)
	require.Equal(t, common.Hash{31: 1}, db.GetState(repairM1131Recipient, common.Hash{}))
	next := h.beginTransaction()
	require.Nil(t, h.abortErr)
	h.endTransaction(next)
}

func TestRepairM_1131_027_AbortRetry(t *testing.T) { repairM1131Abort(t) }

func TestRepairM_1131_026_ForkBoundary(t *testing.T) {
	for _, at := range []uint64{9, 10, 11} {
		t.Run(fmt.Sprint(at), func(t *testing.T) {
			h, db := repairM1131Host(t, false)
			cfg := *h.evm.ChainConfig()
			activation := uint64(10)
			cfg.PragueTime = &activation
			ctx := h.evm.Context
			ctx.Time = at
			h.evm = vm.NewEVM(ctx, db, &cfg, vm.Config{}, nil)
			e := NewEVMInterpreter(h, h.evm)
			rev, err := h.getEVMRevision()
			if at < activation {
				require.NoError(t, err)
				require.Equal(t, evmc.Cancun, rev)
			} else {
				require.Error(t, err)
				require.True(t, utils.ShouldExecutionAbort(err))
				h.evm.EVMInterpreter = e
				contract := vm.NewContract(repairM1131Caller, repairM1131Recipient, new(uint256.Int), 100000, nil)
				contract.Code = common.FromHex("0x6001600055")
				_, err = e.Run(vm.CALL, contract, nil, false)
				require.IsType(t, &UnsupportedEVMRevisionError{}, err)
				require.Equal(t, uint64(100000), contract.Gas)
				require.Equal(t, common.Hash{}, db.GetState(repairM1131Recipient, common.Hash{}))
				require.Zero(t, h.evm.Depth)
			}
		})
	}
}

func TestRepairM_1131_029_InterpreterBoundary(t *testing.T) {
	for _, op := range []vm.OpCode{vm.CALL, vm.CALLCODE, vm.DELEGATECALL, vm.STATICCALL, vm.CREATE, vm.CREATE2} {
		for _, inherited := range []bool{false, true} {
			t.Run(op.String()+fmt.Sprint(inherited), func(t *testing.T) {
				h, db := repairM1131Host(t, false)
				interpreter := NewEVMInterpreter(h, h.evm)
				h.evm.EVMInterpreter = interpreter
				interpreter.readOnly = inherited
				contract := vm.NewContract(repairM1131Caller, repairM1131Recipient, uint256.NewInt(7), 100000, nil)
				contract.Code = common.FromHex("0x3360005260206000f3")
				db.SetCode(repairM1131Recipient, []byte{0xfe})
				input := []byte{0xca, 0xfe}
				message, err := interpreter.executionMessage(op, contract, input)
				require.NoError(t, err)
				require.Equal(t, contract.Code, message.code)
				require.Equal(t, input, message.input)
				require.Equal(t, evmc.Address(repairM1131Caller), message.sender)
				require.Equal(t, evmc.Address(repairM1131Recipient), message.recipient)
				require.Equal(t, evmc.Hash{31: 7}, message.value)
				require.Equal(t, inherited || op == vm.STATICCALL, message.static)
				// No native runtime is loaded in this gate. Run must refuse explicitly,
				// restore inherited state and never silently choose another interpreter.
				_, err = interpreter.Run(op, contract, input, op == vm.STATICCALL)
				require.ErrorContains(t, err, "evmone runtime is unavailable")
				require.Equal(t, inherited, interpreter.ReadOnly())
				require.Zero(t, h.evm.Depth)
				require.Empty(t, h.sstoreGasFrames)
				require.Equal(t, uint64(100000), contract.Gas)
				require.Equal(t, common.Hash{}, db.GetState(repairM1131Recipient, common.Hash{}))
			})
		}
	}
	TestCallKindMapping(t)
	TestCodeToExecuteForCreate(t)
	TestStaticCallFlag(t)
	TestExecuteCodeSelection(t)
}
