// Proves: paykeeper.Keeper and Python encode_call/Type2Transaction -> vm.EVM.Call at PayAddress -> paxeer::decode_block, Store::commit and api::route.
// Repairs layerxpay.Precompile.RunAndCalculateGas accepting noncanonical trailing
// calldata after common.Precompile.Prepare decoded it. Both malformed calldata
// and a truncated producer event are refused at their actual decoding boundary.
// precompiles/common/precompiles.go: Precompile.Prepare still accepts trailing
// data; the payment wrapper must reject it before any keeper execution.
// services/platform/hosted/indexer/src/paxeer.rs: decode_log quarantines
// malformed event data as undecodable, with no holder projection.
// src/modules/budget/lx_budget_module.c: budget_execute_revoke reduces the
// period limit to spent_this_period; src/modules/budget/lx_budget_create.c:
// lx_budget_record_validate then rejects zero. An unused budget should be
// revocable, but currently refuses atomically with LXP_ERR_NON_CANONICAL.
// Assert that behaviour, then spend and refund through SDK calls to prove the
// successful revocation and its unchanged 200-unit drain across the consumer.
// budgetPull is retired: its current typed retirement refusal is asserted rather
// than treating the retained ABI selector as an active payment operation.
package layerxpay_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

var _ lp.LayerXPayKeepers = payPrecompileKeepers{}
var _ utils.Keepers = payPrecompileKeepers{}

type layerxpayProofCall struct {
	Method string        `json:"method"`
	Types  []string      `json:"types"`
	Args   []interface{} `json:"args"`
}

type layerxpayProofDispute struct {
	DisputeId          [32]byte
	ActivityId         [32]byte
	AgreementId        [32]byte
	Raiser             [32]byte
	EvidenceHashes     [][32]byte
	GlobalSequence     uint64
	Resolved           bool
	Ruling             uint16
	ProviderBps        uint32
	EscrowResolutionId [32]byte
	ResolutionSequence uint64
}

func layerxpayProofArgument(value interface{}) interface{} {
	switch v := value.(type) {
	case [32]byte:
		return hex.EncodeToString(v[:])
	case []byte:
		return hex.EncodeToString(v)
	case *big.Int:
		return v.String()
	case [][32]byte:
		out := make([]interface{}, len(v))
		for i := range v {
			out[i] = layerxpayProofArgument(v[i])
		}
		return out
	default:
		return fmt.Sprint(value)
	}
}

func layerxpayProofCommand(t *testing.T, root string, input []byte, name string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = root
	command.Stdin = bytes.NewReader(input)
	var diagnostics bytes.Buffer
	command.Stderr = &diagnostics
	output, err := command.Output()
	require.NoError(t, err, "%s %v: %s\n%s", name, args, output, diagnostics.String())
	return output
}

// layerxpayProofReopen closes the real kernel and reconstructs its keeper and
// EVM state DB over the committed application stores, without InitGenesis.
func layerxpayProofReopen(t *testing.T, h *payPrecompileHarness) {
	t.Helper()
	parent := h.db.Ctx().MultiStore()
	_, err := h.db.Finalize()
	require.NoError(t, err)
	ctx := h.db.Ctx().WithMultiStore(parent).WithEventManager(sdk.NewEventManager())
	h.keeper.Host().Close()
	hostKey := escrowTestHostKey(t, h)
	var payKey sdk.StoreKey
	for _, key := range parent.StoreKeys() {
		if key.Name() == paytypes.StoreKey {
			payKey = key
		}
	}
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: hostKey, NetworkID: 7, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	application := testkeeper.EVMTestApp
	keeper, err := paykeeper.NewKeeper(application.AppCodec(), payKey, hostKey, host, &application.EvmKeeper, application.LayerXCustodyKeeper, "pay-governance")
	require.NoError(t, err)
	h.keeper, h.keepers.pay = &keeper, &keeper
	merged, err := lp.NewMergePrecompile(h.keepers, pcommon.MergeUpgrade)
	require.NoError(t, err)
	h.precompile = merged.(*lp.Precompile)
	h.db = state.NewDBImpl(ctx, &application.EvmKeeper, false)
	h.evm.StateDB = h.db
	h.evm.SetPrecompiles(map[common.Address]vm.PrecompiledContract{h.precompile.Address(): h.precompile})
}

func TestProofLayerxpay0x101b(t *testing.T) {
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	directory := t.TempDir()
	f := newServiceTestFixture(t)
	h := f.h
	h.keeper.SetParams(h.db.Ctx(), paytypes.Params{BudgetEnabled: true, EscrowEnabled: true, ServiceEnabled: true})
	serviceTestForm(t, f, 2)
	serviceTestCommit(t, f)
	serviceTestDeliver(t, f)
	serviceTestCall(t, f, f.owner.caller, "serviceReject", f.agreement, uint16(7), [][32]byte{f.spec})

	// Exercise all exported constructor/keeper surfaces used by the application.
	base, err := lp.NewPrecompile(h.keepers.LayerXPayK())
	require.NoError(t, err)
	require.Equal(t, lp.PrecompileName, base.GetName())
	require.Equal(t, common.HexToAddress(lp.PayAddress), base.Address())
	_, err = lp.NewPrecompile(nil)
	require.ErrorContains(t, err, "LXP_PRECOMPILE_NO_CONTEXT")
	merged, err := lp.NewMergePrecompile(h.keepers, pcommon.MergeUpgrade)
	require.NoError(t, err)
	h.precompile = merged.(*lp.Precompile)
	h.evm.SetPrecompiles(map[common.Address]vm.PrecompiledContract{h.precompile.Address(): h.precompile})
	contract := h.precompile.GetABI()
	require.Equal(t, "budgetPull(bytes32,bytes32,uint128,bytes)", contract.Methods["budgetPull"].Sig)
	require.Equal(t, "serviceDisputeOpen(bytes32,bytes32,bytes32[])", contract.Methods["serviceDisputeOpen"].Sig)
	for _, name := range []string{"budgetPull", "serviceDisputeOpen"} {
		require.Equal(t, "nonpayable", contract.Methods[name].StateMutability)
	}
	for _, name := range []string{lp.BudgetCreated, lp.BudgetRevoked} {
		require.True(t, contract.Events[name].Inputs[0].Indexed)
	}

	id, disputeID := [32]byte{0xd1}, [32]byte{0xd2}
	create := budgetTestArgs(t, f.escrowTestFixture, id)
	calls := []struct {
		name string
		args []interface{}
	}{
		{"budgetCreate", create},
		{"budgetRemaining", []interface{}{id}},
		{"budgetRevoke", []interface{}{id, uint64(1)}},
		{"budgetPull", []interface{}{id, f.beneficiary.account, big.NewInt(5), []byte{1, 2}}},
		{"serviceDisputeOpen", []interface{}{disputeID, f.agreement, [][32]byte{{0xa2}, {0xa1}}}},
		{"budgetSpend", []interface{}{id, f.beneficiary.account, big.NewInt(1)}},
		{"budgetFund", []interface{}{id, big.NewInt(1)}},
	}
	requests := make([]layerxpayProofCall, len(calls))
	for i, call := range calls {
		request := &requests[i]
		request.Method = call.name
		for j, input := range contract.Methods[call.name].Inputs {
			request.Types = append(request.Types, input.Type.String())
			request.Args = append(request.Args, layerxpayProofArgument(call.args[j]))
		}
	}
	requestJSON, err := json.Marshal(requests)
	require.NoError(t, err)
	requestPath := filepath.Join(directory, "sdk-calls.json")
	require.NoError(t, os.WriteFile(requestPath, requestJSON, 0600))
	encoded := layerxpayProofCommand(t, root, nil, "uv", "run", "--quiet", "--no-project", "--with", "cryptography>=41", "python3", "tests/agent/sdk/python/test_evm_transaction.py", "--layerxpay-proof", requestPath)
	var signed []struct {
		Raw    string `json:"raw"`
		Sender string `json:"sender"`
		Data   string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(encoded, &signed))
	require.Len(t, signed, len(calls))
	transactions := make([]*ethtypes.Transaction, len(signed))
	for i, row := range signed {
		raw, err := hex.DecodeString(row.Raw)
		require.NoError(t, err)
		transaction := new(ethtypes.Transaction)
		require.NoError(t, transaction.UnmarshalBinary(raw))
		sender, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(transaction.ChainId()), transaction)
		require.NoError(t, err)
		require.Equal(t, common.HexToAddress(row.Sender), sender)
		require.Equal(t, h.precompile.Address(), *transaction.To())
		require.Zero(t, transaction.Value().Sign())
		want, err := contract.Pack(calls[i].name, calls[i].args...)
		require.NoError(t, err)
		require.Equal(t, want, transaction.Data(), "Python SDK and Go producer disagree")
		transactions[i] = transaction
	}
	sender := common.HexToAddress(signed[0].Sender)
	message, err := evmtypes.LayerXBindMessage(testkeeper.EVMTestApp.EvmKeeper.ChainID(h.db.Ctx()), sender, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(f.owner.private, message))
	// A DID has exactly one EVM binding. Preserve that refusal, then move the
	// fixture owner's binding through the real keeper before using SDK calls.
	_, err = testkeeper.EVMTestApp.EvmKeeper.BindLayerX(h.db.Ctx(), sender, f.owner.public, signature)
	require.ErrorIs(t, err, evmtypes.ErrLayerXDidBound)
	removed, _, err := testkeeper.EVMTestApp.EvmKeeper.UnbindLayerX(h.db.Ctx(), f.owner.caller)
	require.NoError(t, err)
	require.Equal(t, f.owner.public, removed)
	_, err = testkeeper.EVMTestApp.EvmKeeper.BindLayerX(h.db.Ctx(), sender, f.owner.public, signature)
	require.NoError(t, err)
	bound, found := testkeeper.EVMTestApp.EvmKeeper.GetLayerXDid(h.db.Ctx(), sender)
	require.True(t, found)
	require.Equal(t, f.owner.public, bound)

	invoke := func(index int, caller common.Address) ([]byte, error) {
		transaction := transactions[index]
		h.db.WithCtx(h.db.Ctx().WithEVMTxHash(transaction.Hash()))
		output, _, err := h.evm.Call(caller, *transaction.To(), transaction.Data(), transaction.Gas(), uint256.NewInt(0))
		return output, err
	}
	var envelopes []interface{}
	recordBlock := func(index int, emitted *ethtypes.Log) {
		transaction := transactions[index]
		height := uint64(100 + len(envelopes))
		blockHash := crypto.Keccak256Hash([]byte(fmt.Sprintf("layerxpay-proof-block-%d", height)))
		parentHash := crypto.Keccak256Hash([]byte(fmt.Sprintf("layerxpay-proof-block-%d", height-1)))
		entry := *emitted
		entry.BlockNumber, entry.BlockHash, entry.TxHash, entry.TxIndex, entry.Index = height, blockHash, transaction.Hash(), 0, 0
		receipt := &ethtypes.Receipt{Type: transaction.Type(), Status: ethtypes.ReceiptStatusSuccessful, TxHash: transaction.Hash(), BlockHash: blockHash, BlockNumber: new(big.Int).SetUint64(height), Logs: []*ethtypes.Log{&entry}}
		envelopes = append(envelopes, map[string]interface{}{
			"block":    map[string]interface{}{"number": fmt.Sprintf("0x%x", height), "hash": blockHash, "parentHash": parentHash, "transactions": []interface{}{map[string]interface{}{"hash": transaction.Hash(), "from": sender, "to": transaction.To(), "value": "0x0", "transactionIndex": "0x0"}}},
			"receipts": map[string]interface{}{transaction.Hash().Hex(): receipt},
		})
	}

	// Successful creation and wrong-signer refusal use the very same SDK bytes.
	output, err := invoke(0, sender)
	require.NoError(t, err, "%x", output)
	require.Empty(t, output)
	created := h.db.GetAllLogs()[len(h.db.GetAllLogs())-1]
	require.Equal(t, contract.Events[lp.BudgetCreated].ID, created.Topics[0])
	recordBlock(0, created)
	before := h.keeper.ExportGenesis(h.db.Ctx())
	logs := len(h.db.GetAllLogs())
	_, err = invoke(2, f.arbiter.caller)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Len(t, h.db.GetAllLogs(), logs)
	layerxpayProofReopen(t, h)
	for i := 0; i < 2; i++ {
		output, _, err = h.evm.StaticCall(sender, h.precompile.Address(), transactions[1].Data(), 2000000)
		require.NoError(t, err)
		values, decodeErr := contract.Unpack("budgetRemaining", output)
		require.NoError(t, decodeErr)
		require.Equal(t, big.NewInt(200), values[0])
	}
	// Prepare itself accepts these bytes; the repaired wrapper must reject them.
	malformed := append(append([]byte{}, transactions[1].Data()...), 0)
	_, _, _, err = h.precompile.Precompile.Prepare(h.evm, malformed)
	require.NoError(t, err)
	before = h.keeper.ExportGenesis(h.db.Ctx())
	output, _, err = h.evm.StaticCall(sender, h.precompile.Address(), malformed, 2000000)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(output)
	require.NoError(t, err)
	require.Contains(t, reason, "LXP_NON_CANONICAL")
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	// Preserve the out-of-scope kernel defect as an explicit atomic refusal.
	before = h.keeper.ExportGenesis(h.db.Ctx())
	logs = len(h.db.GetAllLogs())
	output, err = invoke(2, sender)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err = abi.UnpackRevert(output)
	require.NoError(t, err)
	require.Contains(t, reason, "LXP_ERR_NON_CANONICAL")
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Len(t, h.db.GetAllLogs(), logs)
	// A genuine spend makes the terminal period limit nonzero. Refund the
	// amount via budgetFund so the original 200-unit drain remains required.
	output, err = invoke(5, sender)
	require.NoError(t, err, "%x", output)
	output, err = invoke(6, sender)
	require.NoError(t, err, "%x", output)
	output, err = invoke(2, sender)
	require.NoError(t, err, "%x", output)
	require.True(t, budgetTestRecord(t, h, id).Revoked)
	revoked := h.db.GetAllLogs()[len(h.db.GetAllLogs())-1]
	require.Equal(t, contract.Events[lp.BudgetRevoked].ID, revoked.Topics[0])
	recordBlock(2, revoked)
	for i := 0; i < 2; i++ {
		before = h.keeper.ExportGenesis(h.db.Ctx())
		output, err = invoke(3, sender)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.Equal(t, pcommon.RetiredRevert(lp.PrecompileName), output)
		require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	}
	output, err = invoke(4, sender)
	require.NoError(t, err, "%x", output)
	dispute, found := h.keeper.GetDispute(h.db.Ctx(), disputeID)
	require.True(t, found)
	require.Equal(t, [][32]byte{{0xa1}, {0xa2}}, dispute.EvidenceHashes)
	require.Equal(t, f.owner.account, dispute.Raiser)
	before = h.keeper.ExportGenesis(h.db.Ctx())
	_, err = invoke(4, sender)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	layerxpayProofReopen(t, h)
	for i := 0; i < 2; i++ {
		values := budgetTestView(t, h, "getDispute", disputeID)
		observed := abi.ConvertType(values[0], new(layerxpayProofDispute)).(*layerxpayProofDispute)
		require.Equal(t, disputeID, observed.DisputeId)
		require.Equal(t, f.agreement, observed.AgreementId)
		require.Equal(t, dispute.EvidenceHashes, observed.EvidenceHashes)
		require.Equal(t, dispute.Raiser, observed.Raiser)
		recovered, found := h.keeper.GetDispute(h.db.Ctx(), disputeID)
		require.True(t, found)
		require.Equal(t, dispute, recovered)
		require.True(t, budgetTestRecord(t, h, id).Revoked)
	}

	consumerInput, err := json.Marshal(map[string]interface{}{"abi": filepath.Join(root, "precompiles/layerxpay/abi.json"), "store": filepath.Join(directory, "indexer.sqlite"), "owner": sender.Hex(), "budget": hex.EncodeToString(id[:]), "asset": "0x" + hex.EncodeToString(f.asset[:]), "blocks": envelopes})
	require.NoError(t, err)
	result := layerxpayProofCommand(t, root, consumerInput, "cargo", "test", "--manifest-path", "services/platform/Cargo.toml", "-p", "layerx-indexer", "--test", "history_position", "proof_layerxpay_0x101b", "--", "--exact", "--nocapture")
	require.Contains(t, string(result), "1 passed")
}
