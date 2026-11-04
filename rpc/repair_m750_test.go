package evmrpc_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// Task 22.33.750 / audit 22.30.172: delegated-execution-auth is proved by
// TestRepairM_750_UnauthorizedWalletCannotSpend and TestRepairM_750_OwnerSelfCall.
// Baseline verification was not run: the bounded gate order requires complete
// implementation before hf-build and hf-done. No out-of-path wiring is required;
// the existing eth_call parity tests already call from the delegated EOA itself.
// The compiler recording below is solc 0.8.28, optimizer enabled, 200 runs,
// EVM cancun, source name SimpleAccount7702.sol (the maintained Hardhat settings).
// Its metadata binds the recording to the exact Solidity source, and its IPFS
// digest is checked against the deployed code. The gate invokes no toolchain.
const repairM750Creation = "6080604052348015600e575f5ffd5b506102e88061001c5f395ff3fe608060405260043610610020575f3560e01c806334fcd5be1461002b575f5ffd5b3661002757005b5f5ffd5b61003e610039366004610189565b610040565b005b33301461008c5760405162461bcd60e51b81526020600482015260166024820152753ab730baba3437b934bd32b21032bc32b1baba34b7b760511b604482015260640160405180910390fd5b5f5b8181101561015157368383838181106100a9576100a96101fa565b90506020028101906100bb919061020e565b90505f806100cc602084018461022c565b6001600160a01b031660208401356100e76040860186610259565b6040516100f59291906102a3565b5f6040518083038185875af1925050503d805f811461012f576040519150601f19603f3d011682016040523d82523d5f602084013e610134565b606091505b50915091508161014657805160208201fd5b50505060010161008e565b506040518181527fe9cc9bbe7de1cbb097a6357ca3182909488efe8d793e25665befc064a02d10a79060200160405180910390a15050565b5f5f6020838503121561019a575f5ffd5b823567ffffffffffffffff8111156101b0575f5ffd5b8301601f810185136101c0575f5ffd5b803567ffffffffffffffff8111156101d6575f5ffd5b8560208260051b84010111156101ea575f5ffd5b6020919091019590945092505050565b634e487b7160e01b5f52603260045260245ffd5b5f8235605e19833603018112610222575f5ffd5b9190910192915050565b5f6020828403121561023c575f5ffd5b81356001600160a01b0381168114610252575f5ffd5b9392505050565b5f5f8335601e1984360301811261026e575f5ffd5b83018035915067ffffffffffffffff821115610288575f5ffd5b60200191503681900382131561029c575f5ffd5b9250929050565b818382375f910190815291905056fea26469706673582212206792d62973062dfaa8fbb07fcbe2386d2897d9808c8651cb43d8c88a504be11564736f6c634300081c0033"

const repairM750Metadata = "{\"compiler\":{\"version\":\"0.8.28+commit.7893614a\"},\"language\":\"Solidity\",\"output\":{\"abi\":[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"count\",\"type\":\"uint256\"}],\"name\":\"BatchExecuted\",\"type\":\"event\"},{\"inputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"target\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"internalType\":\"struct SimpleAccount7702.Call[]\",\"name\":\"calls\",\"type\":\"tuple[]\"}],\"name\":\"executeBatch\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"stateMutability\":\"payable\",\"type\":\"receive\"}],\"devdoc\":{\"kind\":\"dev\",\"methods\":{},\"version\":1},\"userdoc\":{\"kind\":\"user\",\"methods\":{},\"notice\":\"Minimal EIP-7702 delegation target. An EOA delegates to this implementation via a type-4 SetCode transaction, after which calls to the EOA execute this code in the EOA's context (so `address(this)` is the EOA). `executeBatch` lets the delegated account perform a list of calls atomically \\u2014 enough for the RPC suite's 7702 parity specs.\",\"version\":1}},\"settings\":{\"compilationTarget\":{\"SimpleAccount7702.sol\":\"SimpleAccount7702\"},\"evmVersion\":\"cancun\",\"libraries\":{},\"metadata\":{\"bytecodeHash\":\"ipfs\"},\"optimizer\":{\"enabled\":true,\"runs\":200},\"remappings\":[]},\"sources\":{\"SimpleAccount7702.sol\":{\"keccak256\":\"0x86d97be032784c3e3598519b17e2ca9599fc3a9177912846fe4dc971573663a8\",\"license\":\"MIT\",\"urls\":[\"bzz-raw://c6ad96ad2252d0805f02ce8e3b779ed1acf51d18278b8ad2942efe2311b01ffb\",\"dweb:/ipfs/QmbFwwmnK21EHFvUQyANTxnAv1khc6M3FeqRgGN8bYjYRo\"]}},\"version\":1}"

type repairM750Call struct {
	Target common.Address
	Value  *big.Int
	Data   []byte
}

type repairM750Fixture struct {
	db             *state.StateDB
	config         runtime.Config
	accountABI     abi.ABI
	owner, other   *ecdsa.PrivateKey
	account, actor common.Address
	implementation common.Address
	txIndex        int
}

func repairM750MetadataDigest(t *testing.T) [32]byte {
	t.Helper()
	metadata := []byte(repairM750Metadata)
	require.LessOrEqual(t, len(metadata), 256*1024, "recorded metadata must fit one IPFS block")
	// Solc's IPFS digest hashes a DAG-PB node containing a UnixFS File.
	file := binary.AppendUvarint([]byte{0x08, 0x02, 0x12}, uint64(len(metadata)))
	file = append(file, metadata...)
	file = append(file, 0x18)
	file = binary.AppendUvarint(file, uint64(len(metadata)))
	node := binary.AppendUvarint([]byte{0x0a}, uint64(len(file)))
	node = append(node, file...)
	return sha256.Sum256(node)
}

func newRepairM750Fixture(t *testing.T) *repairM750Fixture {
	t.Helper()
	var metadata struct {
		Compiler struct{ Version string }
		Output   struct{ ABI json.RawMessage }
		Sources  map[string]struct{ Keccak256 string }
		Settings struct {
			EvmVersion string
			Optimizer  struct {
				Enabled bool
				Runs    int
			}
		}
	}
	require.NoError(t, json.Unmarshal([]byte(repairM750Metadata), &metadata))
	require.Equal(t, "0.8.28+commit.7893614a", metadata.Compiler.Version)
	require.Equal(t, "cancun", metadata.Settings.EvmVersion)
	require.True(t, metadata.Settings.Optimizer.Enabled)
	require.Equal(t, 200, metadata.Settings.Optimizer.Runs)
	source, err := os.ReadFile("../tests/integration/rpc_tests/contracts/SimpleAccount7702.sol")
	require.NoError(t, err)
	require.Equal(t, metadata.Sources["SimpleAccount7702.sol"].Keccak256, crypto.Keccak256Hash(source).Hex(), "regenerate the compiler recording when the Solidity source changes")
	accountABI, err := abi.JSON(strings.NewReader(string(metadata.Output.ABI)))
	require.NoError(t, err)
	db, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	require.NoError(t, err)
	owner, err := crypto.HexToECDSA(strings.Repeat("11", 32))
	require.NoError(t, err)
	other, err := crypto.HexToECDSA(strings.Repeat("22", 32))
	require.NoError(t, err)
	chain := *params.AllDevChainProtocolChanges
	chain.ChainID = big.NewInt(125)
	f := &repairM750Fixture{
		db: db, accountABI: accountABI, owner: owner, other: other,
		account: crypto.PubkeyToAddress(owner.PublicKey), actor: crypto.PubkeyToAddress(other.PublicKey),
		config: runtime.Config{State: db, ChainConfig: &chain, BlockNumber: big.NewInt(1), GasLimit: 10_000_000, BaseFee: new(big.Int)},
	}
	runtime.SetDefaults(&f.config)
	for _, address := range []common.Address{f.account, f.actor} {
		db.AddBalance(address, uint256.NewInt(1_000_000_000_000_000_000), tracing.BalanceChangeUnspecified)
	}
	deployment, receipt := f.send(t, other, nil, new(big.Int), common.FromHex(repairM750Creation), nil)
	require.NoError(t, deployment.Err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
	f.implementation = receipt.ContractAddress
	require.NotEmpty(t, db.GetCode(f.implementation))
	digest := repairM750MetadataDigest(t)
	trailer := append(common.FromHex("a2646970667358221220"), digest[:]...)
	trailer = append(trailer, common.FromHex("64736f6c634300081c0033")...)
	require.True(t, bytes.HasSuffix(db.GetCode(f.implementation), trailer), "deployed compiler output must carry the recorded source metadata")
	return f
}

// Signing, binary envelope transport, sender recovery, authorization processing,
// execution, and receipt production all use the maintained geth implementation.
func (f *repairM750Fixture) send(t *testing.T, key *ecdsa.PrivateKey, to *common.Address, value *big.Int, data []byte, authorizations []types.SetCodeAuthorization) (*core.ExecutionResult, *types.Receipt) {
	t.Helper()
	from := crypto.PubkeyToAddress(key.PublicKey)
	var txData types.TxData = &types.DynamicFeeTx{
		ChainID: f.config.ChainConfig.ChainID, Nonce: f.db.GetNonce(from),
		GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(1), Gas: 2_000_000,
		To: to, Value: value, Data: data,
	}
	if authorizations != nil {
		require.NotNil(t, to)
		txData = &types.SetCodeTx{
			ChainID: uint256.MustFromBig(f.config.ChainConfig.ChainID), Nonce: f.db.GetNonce(from),
			GasTipCap: uint256.NewInt(1), GasFeeCap: uint256.NewInt(1), Gas: 2_000_000,
			To: *to, Value: uint256.MustFromBig(value), Data: data, AuthList: authorizations,
		}
	}
	signer := types.MakeSigner(f.config.ChainConfig, f.config.BlockNumber, f.config.Time)
	signed, err := types.SignNewTx(key, signer, txData)
	require.NoError(t, err)
	wire, err := signed.MarshalBinary()
	require.NoError(t, err)
	var received types.Transaction
	require.NoError(t, received.UnmarshalBinary(wire))
	require.Equal(t, signed.Hash(), received.Hash())
	msg, err := core.TransactionToMessage(&received, signer, f.config.BaseFee)
	require.NoError(t, err)
	require.Equal(t, from, msg.From)
	f.db.SetTxContext(received.Hash(), f.txIndex)
	f.txIndex++
	env := runtime.NewEnv(&f.config)
	result, err := core.ApplyMessage(env, msg, new(core.GasPool).AddGas(f.config.GasLimit))
	require.NoError(t, err, "signed transaction must reach EVM execution")
	f.db.Finalise(true)
	receipt := core.MakeReceipt(env, result, f.db, f.config.BlockNumber, common.Hash{}, &received, result.UsedGas, nil)
	return result, receipt
}

func (f *repairM750Fixture) authorization(t *testing.T) []types.SetCodeAuthorization {
	t.Helper()
	auth, err := types.SignSetCode(f.owner, types.SetCodeAuthorization{
		ChainID: *uint256.MustFromBig(f.config.ChainConfig.ChainID), Address: f.implementation, Nonce: f.db.GetNonce(f.account),
	})
	require.NoError(t, err)
	return []types.SetCodeAuthorization{auth}
}

func (f *repairM750Fixture) batch(t *testing.T, calls ...repairM750Call) []byte {
	t.Helper()
	data, err := f.accountABI.Pack("executeBatch", calls)
	require.NoError(t, err)
	return data
}

func repairM750RequireUnauthorized(t *testing.T, result *core.ExecutionResult, receipt *types.Receipt) {
	t.Helper()
	require.ErrorIs(t, result.Err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(result.Revert())
	require.NoError(t, err)
	require.Equal(t, "unauthorized execution", reason)
	require.Equal(t, types.ReceiptStatusFailed, receipt.Status)
	require.Empty(t, receipt.Logs)
}

func TestRepairM_750_UnauthorizedWalletCannotSpend(t *testing.T) {
	f := newRepairM750Fixture(t)
	recipient := common.HexToAddress("0xb001")
	before := f.db.GetBalance(f.account).ToBig()
	data := f.batch(t, repairM750Call{Target: recipient, Value: big.NewInt(12345), Data: []byte{0xab, 0xcd}})
	// A valid delegation signature authorizes the code, not the sponsor's batch.
	result, receipt := f.send(t, f.other, &f.account, big.NewInt(7), data, f.authorization(t))
	repairM750RequireUnauthorized(t, result, receipt)
	require.Equal(t, before, f.db.GetBalance(f.account).ToBig())
	require.True(t, f.db.GetBalance(recipient).IsZero())
	require.Equal(t, types.AddressToDelegation(f.implementation), f.db.GetCode(f.account))
	require.Equal(t, uint64(1), f.db.GetNonce(f.account), "EIP-7702 authorization persists even when execution reverts")
	result, receipt = f.send(t, f.other, &f.account, new(big.Int), data, nil)
	repairM750RequireUnauthorized(t, result, receipt)
	require.Equal(t, before, f.db.GetBalance(f.account).ToBig())
	require.True(t, f.db.GetBalance(recipient).IsZero())
	// Calling the implementation directly must not acquire the EOA's authority.
	result, receipt = f.send(t, f.owner, &f.implementation, new(big.Int), data, nil)
	repairM750RequireUnauthorized(t, result, receipt)
	require.True(t, f.db.GetBalance(recipient).IsZero())
}

func TestRepairM_750_OwnerSelfCall(t *testing.T) {
	f := newRepairM750Fixture(t)
	result, receipt := f.send(t, f.other, &f.account, new(big.Int), nil, f.authorization(t))
	require.NoError(t, result.Err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
	first, second := common.HexToAddress("0xb001"), common.HexToAddress("0xb002")
	before := f.db.GetBalance(f.account).ToBig()
	data := f.batch(t,
		repairM750Call{Target: first, Value: big.NewInt(12345), Data: []byte{0xab, 0xcd}},
		repairM750Call{Target: second, Value: big.NewInt(67890)},
	)
	result, receipt = f.send(t, f.owner, &f.account, new(big.Int), data, nil)
	require.NoError(t, result.Err)
	require.Empty(t, result.ReturnData)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
	require.Equal(t, big.NewInt(12345), f.db.GetBalance(first).ToBig())
	require.Equal(t, big.NewInt(67890), f.db.GetBalance(second).ToBig())
	want := new(big.Int).Sub(before, big.NewInt(12345+67890))
	want.Sub(want, new(big.Int).SetUint64(result.UsedGas))
	require.Equal(t, want, f.db.GetBalance(f.account).ToBig())
	require.Equal(t, types.AddressToDelegation(f.implementation), f.db.GetCode(f.account))
	require.Len(t, receipt.Logs, 1)
	require.Equal(t, f.account, receipt.Logs[0].Address, "event must originate in the delegated EOA context")
	require.Equal(t, []common.Hash{f.accountABI.Events["BatchExecuted"].ID}, receipt.Logs[0].Topics)
	require.Equal(t, common.LeftPadBytes(big.NewInt(2).Bytes(), 32), receipt.Logs[0].Data)
}

func TestRepairM_750_BatchRevertIsAtomic(t *testing.T) {
	f := newRepairM750Fixture(t)
	result, _ := f.send(t, f.other, &f.account, new(big.Int), nil, f.authorization(t))
	require.NoError(t, result.Err)
	recipient := common.HexToAddress("0xb001")
	before := f.db.GetBalance(f.account).ToBig()
	data := f.batch(t,
		repairM750Call{Target: recipient, Value: big.NewInt(12345)},
		repairM750Call{Target: f.implementation, Value: new(big.Int), Data: f.batch(t)},
	)
	result, receipt := f.send(t, f.owner, &f.account, new(big.Int), data, nil)
	repairM750RequireUnauthorized(t, result, receipt)
	require.True(t, f.db.GetBalance(recipient).IsZero(), "a later failure must undo earlier value transfers")
	require.Equal(t, new(big.Int).Sub(before, new(big.Int).SetUint64(result.UsedGas)), f.db.GetBalance(f.account).ToBig())
}

func TestRepairM_750_ReceiveAndEmptyBatchAuthorization(t *testing.T) {
	f := newRepairM750Fixture(t)
	before := f.db.GetBalance(f.account).ToBig()
	result, receipt := f.send(t, f.other, &f.account, big.NewInt(12345), nil, f.authorization(t))
	require.NoError(t, result.Err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
	require.Equal(t, new(big.Int).Add(before, big.NewInt(12345)), f.db.GetBalance(f.account).ToBig())
	before = f.db.GetBalance(f.account).ToBig()
	result, receipt = f.send(t, f.other, &f.account, big.NewInt(99), f.batch(t), nil)
	repairM750RequireUnauthorized(t, result, receipt)
	require.Equal(t, before, f.db.GetBalance(f.account).ToBig())
	result, receipt = f.send(t, f.owner, &f.account, new(big.Int), f.batch(t), nil)
	require.NoError(t, result.Err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
	require.Len(t, receipt.Logs, 1)
	require.Equal(t, make([]byte, 32), receipt.Logs[0].Data)
}
