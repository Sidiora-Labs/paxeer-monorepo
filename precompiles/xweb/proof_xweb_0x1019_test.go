// Proves: xwebkeeper.Keeper.RequestFromProgram and Attestor::attest_program -> application vm.EVM at 0x1019 -> XWebProgramProofConsumer, Answer::try_from and policy.Policy.Evaluate.
//
// The Rust test obtains the network through live EVM-backed JSON-RPC, signs
// independently fetched search results and emits program::observation_bytes.
// Solidity's own functions forward that observation and decode the result.
// Fresh EVM/precompile instances read the result twice, without a consumer cache.
// Repairs XWebProgramProofConsumer.fulfilProgram in contracts/test/XWebConsumer.t.sol:
// a typed void call rejects the native precompile because it has no bytecode;
// the consumer now calls it directly and bubbles the exact revert data. The
// network-word decoder in attest.rs shares watch.rs::kernel_network_id. That out-of-scope
// function currently admits a zero network (it should refuse it); the Rust proof
// records that behavior and proves Answer::try_from still refuses the real zero
// word emitted by PrecompileExecutor.getKernelNetworkId from corrupted params.
package xweb_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	xwebkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// proofConsumer compiles the declared Solidity consumer itself, together with
// its production imports. The unrelated Foundry unit tests need no cheatcodes
// or replacement precompile in this application-level execution.
func proofConsumer(t *testing.T, root string) (abi.ABI, []byte) {
	t.Helper()
	file, err := os.ReadFile(filepath.Join(root, "contracts/test/XWebConsumer.t.sol"))
	require.NoError(t, err)
	marker := "contract XWebProgramProofConsumer is XWebConsumer {"
	start := bytes.Index(file, []byte(marker))
	require.NotEqual(t, -1, start)
	source := "pragma solidity ^0.8.27;\nimport {IXWeb, XWEB_PRECOMPILE_ADDRESS} from '../src/precompiles/IXWeb.sol';\nimport {XWebConsumer} from '../src/xweb/XWebConsumer.sol';\n" + string(file[start:])
	sources := map[string]interface{}{"contracts/test/XWebConsumer.t.sol": map[string]string{"content": source}}
	for _, path := range []string{"contracts/src/precompiles/IXWeb.sol", "contracts/src/xweb/XWebConsumer.sol"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		require.NoError(t, err)
		sources[path] = map[string]string{"content": string(data)}
	}
	input, err := json.Marshal(map[string]interface{}{
		"language": "Solidity", "sources": sources,
		"settings": map[string]interface{}{"evmVersion": "cancun", "optimizer": map[string]interface{}{"enabled": true, "runs": 1000},
			"outputSelection": map[string]interface{}{"*": map[string]interface{}{"*": []string{"abi", "evm.deployedBytecode.object"}}}},
	})
	require.NoError(t, err)
	// A regular file avoids solcjs's synchronous stdin read returning EAGAIN.
	inPath := filepath.Join(t.TempDir(), "solc-input.json")
	require.NoError(t, os.WriteFile(inPath, input, 0600))
	in, err := os.Open(inPath)
	require.NoError(t, err)
	defer in.Close()
	cmd := exec.Command("npx", "--yes", "solc@0.8.27", "--standard-json")
	cmd.Dir, cmd.Stdin = root, in
	outputPath := filepath.Join(t.TempDir(), "solc-output.json")
	output, err := os.Create(outputPath)
	require.NoError(t, err)
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = output, &stderr
	runErr := cmd.Run()
	require.NoError(t, output.Close())
	require.NoError(t, runErr, "%s", stderr.String())
	out, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	begin := bytes.IndexByte(out, '{')
	require.NotEqual(t, -1, begin, "%s", out)
	var result struct {
		Contracts map[string]map[string]struct {
			ABI json.RawMessage `json:"abi"`
			EVM struct {
				DeployedBytecode struct {
					Object string `json:"object"`
				} `json:"deployedBytecode"`
			} `json:"evm"`
		} `json:"contracts"`
		Errors []struct {
			Severity string `json:"severity"`
			Message  string `json:"formattedMessage"`
		} `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(out[begin:], &result), "%s", out)
	for _, problem := range result.Errors {
		require.NotEqual(t, "error", problem.Severity, problem.Message)
	}
	compiled, ok := result.Contracts["contracts/test/XWebConsumer.t.sol"]["XWebProgramProofConsumer"]
	require.True(t, ok)
	contract, err := abi.JSON(bytes.NewReader(compiled.ABI))
	require.NoError(t, err)
	code := common.FromHex(compiled.EVM.DeployedBytecode.Object)
	require.NotEmpty(t, code)
	return contract, code
}

func TestProofXweb0x1019(t *testing.T) {
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	h := newProgramDispatchHarness(t)
	app := testkeeper.EVMTestApp
	// Own the real store key so fault injection changes the producer's state,
	// rather than intercepting or fabricating a transport response.
	key := sdk.NewKVStoreKey(types.StoreKey)
	k := xwebkeeper.NewKeeper(key, app.BankKeeper, &app.EvmKeeper)
	ctx := xwebtestutil.WithXWebStore(h.ctx(), key)
	params := h.keeper.GetParams(h.ctx())
	k.InitGenesis(ctx, *types.DefaultGenesis())
	require.NoError(t, k.SetParams(ctx, params))
	for _, attestor := range h.attestors {
		require.NoError(t, k.RegisterAttestor(ctx, types.MsgRegisterAttestor{Authority: authority, Attestor: attestor.Registration()}))
	}
	require.NoError(t, k.Unpause(ctx, types.MsgUnpause{Authority: authority}))
	h.keeper = &k
	h.stateDB.WithCtx(ctx)
	reopen := func() {
		// The application context owns state; these adapters have no cache.
		h.precompile, err = xweb.NewPrecompile(keepers{Keepers: app.GetPrecompileKeepers(), xweb: &k})
		require.NoError(t, err)
		contracts := maps.Clone(app.EvmKeeper.CustomPrecompiles(h.ctx()))
		contracts[precompile] = h.precompile
		h.evm = vm.NewEVM(h.evm.Context, h.stateDB, h.evm.ChainConfig(), vm.Config{}, contracts)
	}
	reopen()
	require.True(t, k.ProgramOriginActive(h.ctx()))
	observation, _ := programDispatchObservation(t, h, response, 2)
	pending, found, err := k.GetProgramPendingRecord(h.ctx(), observation.ProgramID, observation.RequestID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, types.Keccak(payload), pending.PayloadHash)
	require.Equal(t, observation.Kind, pending.Kind)
	contract, code := proofConsumer(t, root)
	consumer := common.HexToAddress("0xc0de1019")
	h.stateDB.SetCode(consumer, code)
	invoke := func(name string, readOnly bool, args ...interface{}) ([]byte, error) {
		t.Helper()
		data, packErr := contract.Pack(name, args...)
		require.NoError(t, packErr)
		if readOnly {
			out, _, callErr := h.evm.StaticCall(stranger, consumer, data, supplied)
			return out, callErr
		}
		out, _, callErr := h.evm.Call(stranger, consumer, data, supplied, uint256.NewInt(0))
		return out, callErr
	}
	refused := func(name, reason string, readOnly bool, args ...interface{}) {
		t.Helper()
		out, callErr := invoke(name, readOnly, args...)
		require.ErrorIs(t, callErr, vm.ErrExecutionReverted)
		actual, decodeErr := abi.UnpackRevert(out)
		require.NoError(t, decodeErr)
		require.Equal(t, reason, actual)
	}
	for i := 0; i < 2; i++ {
		out, callErr := invoke("getKernelNetworkId", true)
		require.NoError(t, callErr)
		decoded, decodeErr := contract.Methods["getKernelNetworkId"].Outputs.Unpack(out)
		require.NoError(t, decodeErr)
		require.Equal(t, []interface{}{uint32(1)}, decoded)
	}
	refused("getProgramResult", "LXP_UNKNOWN_FIELD", true, [32]byte(observation.ProgramID), observation.RequestID)
	// A valid ABI uint32 word with an invalid domain value reaches the
	// consumer decoder; its semantic validation must reject that real output.
	store := h.ctx().KVStore(key)
	savedParams := bytes.Clone(store.Get(types.ParamsKey))
	badParams := k.GetParams(h.ctx())
	badParams.KernelNetworkId = 0
	badJSON, err := json.Marshal(badParams)
	require.NoError(t, err)
	store.Set(types.ParamsKey, badJSON)
	out, callErr := invoke("getKernelNetworkId", true)
	require.ErrorIs(t, callErr, vm.ErrExecutionReverted)
	malformed, ok := contract.Errors["MalformedProgramResult"]
	require.True(t, ok)
	require.Equal(t, malformed.ID.Bytes()[:4], out)
	store.Set(types.ParamsKey, savedParams)

	var mu sync.Mutex
	normalCalls, zeroCalls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var request struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || request.Method != "eth_call" || len(request.Params) != 2 {
			http.Error(w, "invalid eth_call", http.StatusBadRequest)
			return
		}
		var call struct {
			To   string `json:"to"`
			Data string `json:"data"`
		}
		if json.Unmarshal(request.Params[0], &call) != nil || common.HexToAddress(call.To) != precompile || call.Data != "0xfc0d26f4" {
			http.Error(w, "unexpected destination or selector", http.StatusBadRequest)
			return
		}
		store := h.ctx().KVStore(key)
		if r.URL.Path == "/zero" {
			zeroCalls++
			saved := bytes.Clone(store.Get(types.ParamsKey))
			defer store.Set(types.ParamsKey, saved)
			bad := k.GetParams(h.ctx())
			bad.KernelNetworkId = 0
			encoded, encodeErr := json.Marshal(bad)
			if encodeErr != nil {
				http.Error(w, encodeErr.Error(), 500)
				return
			}
			store.Set(types.ParamsKey, encoded)
		} else {
			normalCalls++
		}
		out, _, callErr := h.evm.StaticCall(stranger, precompile, common.FromHex(call.Data), supplied)
		w.Header().Set("Content-Type", "application/json")
		reply := map[string]interface{}{"jsonrpc": "2.0", "id": request.ID}
		if callErr != nil {
			reply["error"] = map[string]interface{}{"code": -32000, "message": callErr.Error()}
		} else {
			reply["result"] = "0x" + common.Bytes2Hex(out)
		}
		_ = json.NewEncoder(w).Encode(reply)
	}))
	defer server.Close()
	// Pass producer-owned values directly to each subprocess; no operator
	// configuration or environment-provided endpoint/file is needed.
	requestJSON, err := json.Marshal(map[string]interface{}{
		"rpc":        server.URL,
		"program_id": "0x" + common.Bytes2Hex(observation.ProgramID[:]),
		"sequence":   pending.Sequence, "request_id": observation.RequestID,
		"kind": pending.Kind, "payload": "0x" + common.Bytes2Hex(payload),
	})
	require.NoError(t, err)
	rust := exec.Command("cargo", "test", "--manifest-path", "services/interop/Cargo.toml", "-p", "x-websearch", "--test", "attest", "xweb_0x1019_application_proof", "--", "--exact", "--nocapture")
	rust.Dir, rust.Stdin = root, bytes.NewReader(requestJSON)
	rustLog, err := rust.CombinedOutput()
	require.NoError(t, err, "Rust consumer: %s", rustLog)
	require.Contains(t, string(rustLog), "1 passed")
	mu.Lock()
	calls, malformedCalls := normalCalls, zeroCalls
	mu.Unlock()
	require.Equal(t, 6, calls, "each Answer::try_from must query the live producer again")
	require.Equal(t, 6, malformedCalls)
	var answerJSON []byte
	for _, line := range bytes.Split(rustLog, []byte{'\n'}) {
		if bytes.HasPrefix(line, []byte("XWEB_PROOF_ANSWER=")) {
			require.Nil(t, answerJSON, "Rust producer must emit exactly one answer")
			answerJSON = bytes.TrimPrefix(line, []byte("XWEB_PROOF_ANSWER="))
		}
	}
	require.NotEmpty(t, answerJSON, "services/interop/crates/x-websearch/tests/attest.rs produced no answer: %s", rustLog)
	var answer struct {
		Observation string `json:"observation"`
		Response    string `json:"response"`
		Digest      string `json:"content_digest"`
		FullLength  uint32 `json:"full_length"`
	}
	require.NoError(t, json.Unmarshal(answerJSON, &answer))
	raw := common.FromHex(answer.Observation)
	require.Greater(t, len(raw), types.ObservationHeaderBytes)
	refused("fulfilProgram", "LXP_NON_CANONICAL", false, raw[:types.ObservationHeaderBytes])
	requireProgramDispatchPending(t, h, observation)
	out, err = invoke("fulfilProgram", false, raw)
	require.NoError(t, err)
	require.Empty(t, out)
	require.Equal(t, uint64(1), k.ProgramCommittedCount(h.ctx()))
	require.Zero(t, k.ProgramPendingCount(h.ctx()))
	for i := 0; i < 2; i++ {
		reopen()
		out, err = invoke("getProgramResult", true, [32]byte(observation.ProgramID), observation.RequestID)
		require.NoError(t, err)
		observed, decodeErr := contract.Methods["getProgramResult"].Outputs.Unpack(out)
		require.NoError(t, decodeErr)
		require.Equal(t, []interface{}{[32]byte(common.HexToHash(answer.Digest)), answer.FullLength, uint32(2), []byte("[]")}, observed)
		require.Equal(t, []byte("[]"), common.FromHex(answer.Response))
	}
	refused("fulfilProgram", "LXP_SEQUENCE_REUSED", false, raw)
	refused("getProgramResult", "LXP_UNKNOWN_FIELD", true, [32]byte(observation.ProgramID), observation.RequestID+1)
	// Corrupt the producer's committed header, and observe refusal through the
	// Solidity consumer rather than calling the keeper as a substitute.
	store = h.ctx().KVStore(key)
	headerKey := xwebkeeper.ProgramAnswerKey(observation.ProgramID, observation.RequestID, 0)
	header := bytes.Clone(store.Get(headerKey))
	require.Len(t, header, xwebkeeper.ProgramAnswerHeaderBytes)
	store.Set(headerKey, header[:len(header)-1])
	refused("getProgramResult", "LXP_NON_CANONICAL", true, [32]byte(observation.ProgramID), observation.RequestID)
	store.Set(headerKey, header)

	calldata, err := h.precompile.GetABI().Pack("fulfilProgram", raw)
	require.NoError(t, err)
	policy := exec.Command("go", "test", "-count=1", "./internal/policy", "-run", "^TestXweb0x1019ApplicationPolicy$", "-v", "-args", "-xweb-proof-calldata=0x"+common.Bytes2Hex(calldata))
	policy.Dir = filepath.Join(root, "services/wallet/attestor")
	policyLog, err := policy.CombinedOutput()
	require.NoError(t, err, "wallet consumer: %s", policyLog)
	require.Contains(t, string(policyLog), "--- PASS: TestXweb0x1019ApplicationPolicy")
	t.Logf("real consumers executed; RPC calls=%d malformed=%d; %s", calls, malformedCalls, fmt.Sprint(strings.TrimSpace(string(policyLog))))
}
