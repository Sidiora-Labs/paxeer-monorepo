// Proves: xwebkeeper.Keeper and teeattestor.PrecompileExecutor -> vm.EVM.StaticCall/Call and JSON-RPC -> wallet teeAttestor, teeVerificationResponse and agentcommon.MethodGas.
//
// Repairs services/wallet/gateway/src/precompiles.ts: the frozen event metadata
// was missing and attestation decoding accepted noncanonical or mismatched-family
// results. Repairs teeattestor.PrecompileExecutor.RequiredGas: a missing method
// must use the shared unknown-selector charge, not MaxUint64.
//
// spec/layerx-merge/contracts/agent-functions.md decision.233 and
// teeattestor.PrecompileExecutor.Execute reserve AttestationVerified but never
// emit it, despite this task's inventory describing an emitted event. This proof
// asserts that current contract for both CALL and STATICCALL and compares the
// real producer's event metadata with the wallet ABI; it fabricates no logs.
// The anchor owns no store. Every repeat traverses the real EVM, including after
// reconstruction; all endpoints and configuration are supplied by this process,
// without inheriting operator configuration. Changing the keeper threshold
// invalidates a previously valid quote. Damaged copies of actual producer return bytes reach the wallet decoder.
// precompiles/layerxagent/common/common.go boundExecutor.RequiredGas still returns
// MaxUint64 for a nil method; this out-of-scope behavior is asserted explicitly.
package teeattestor_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	xwebtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/teeattestor"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestProofTeeAttestor0x0907(t *testing.T) {
	h := newHarness(t, 5)
	producerABI := h.precompile.GetABI()
	original := h.keeper.ExportGenesis(h.stateDB.Ctx())
	quotes := make([]string, 4)
	for family := range quotes {
		quotes[family] = "0x" + ethcommon.Bytes2Hex(h.quote(uint8(family), blockOpen-60, false, uint64(family), h.keys[:3]...))
	}
	var outsider xwebtestutil.Attestor
	set := h.keeper.GetAttestorSet(h.stateDB.Ctx())
	for _, key := range xwebtestutil.Attestors(6) {
		if !set.Has(key.Signer) {
			outsider = key
		}
	}
	require.NotEqual(t, xwebtypes.Address20{}, outsider.Signer)
	badQuote := h.quote(0, blockOpen-60, false, 9, h.keys[0], h.keys[1], outsider)
	roots := make([]string, len(h.keys))
	for index, key := range h.keys {
		roots[index] = "0x" + ethcommon.Bytes2Hex(key.Signer[:])
	}
	event := producerABI.Events["AttestationVerified"]
	require.Equal(t, "AttestationVerified(uint8,bytes32,bytes32,uint64,bool)", event.Sig)
	require.Equal(t, "0x936ac9b70043ceb389e1dd6abcc51ed17a45cfd257a5b3379b934eed92f52316", event.ID.Hex())
	eventInputs := make([]map[string]interface{}, 0, len(event.Inputs))
	for _, input := range event.Inputs {
		eventInputs = append(eventInputs, map[string]interface{}{"name": input.Name, "type": input.Type.String(), "indexed": input.Indexed})
	}
	eventJSON, err := json.Marshal(map[string]interface{}{"name": event.RawName, "anonymous": event.Anonymous, "inputs": eventInputs})
	require.NoError(t, err)
	// Check all four selectors against the actual producer, not just table strings.
	for _, row := range []struct {
		name string
		args []interface{}
		base uint64
	}{
		{"verify", []interface{}{uint8(0), ethcommon.FromHex(quotes[0])}, 46000},
		{"verifyAndExpect", []interface{}{uint8(0), ethcommon.FromHex(quotes[0]), reportData}, 46000},
		{"rootOf", []interface{}{uint8(0), big.NewInt(0)}, 1000},
		{"rootCount", []interface{}{uint8(0)}, 1000},
	} {
		input := h.input(row.name, row.args...)
		method := producerABI.Methods[row.name]
		cost := row.base + 3*uint64(len(input))
		require.Equal(t, cost, agentcommon.MethodGas(method.ID, len(input)), row.name)
		require.Equal(t, cost, h.precompile.RequiredGas(input), row.name)
		require.Equal(t, cost, h.precompile.GetExecutor().RequiredGas(input[4:], &method), row.name)
		ret, left, callErr := h.evm.StaticCall(caller, h.precompile.Address(), input, cost)
		require.NoError(t, callErr, row.name)
		require.Zero(t, left, row.name)
		require.NotEmpty(t, ret, row.name)
		_, left, callErr = h.evm.StaticCall(caller, h.precompile.Address(), input, cost-1)
		require.ErrorIs(t, callErr, vm.ErrOutOfGas, row.name)
		require.Zero(t, left, row.name)
	}
	require.Equal(t, uint64(math.MaxUint64), h.precompile.GetExecutor().RequiredGas(nil, nil))
	executor := &teeattestor.PrecompileExecutor{}
	require.Equal(t, pcommon.UnknownMethodCallGas, executor.RequiredGas(nil, nil))
	// A regular EVM CALL must also retain the frozen view/no-event behavior.
	result, _, err := h.evm.Call(caller, h.precompile.Address(), h.input("verify", uint8(0), ethcommon.FromHex(quotes[0])), suppliedGas, uint256.NewInt(0))
	require.NoError(t, err)
	require.NotEmpty(t, result)
	require.Empty(t, h.stateDB.GetAllLogs())

	type rpcCall struct {
		To   ethcommon.Address `json:"to"`
		Data string            `json:"data"`
	}
	var mu sync.Mutex
	counts := make(map[string]int)
	reopens, thresholdChanges := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var request struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		reply := func(value interface{}, callErr error, data []byte) {
			response := map[string]interface{}{"jsonrpc": "2.0", "id": request.ID}
			if callErr == nil {
				response["result"] = value
			} else {
				message := callErr.Error()
				if reason, decodeErr := abi.UnpackRevert(data); decodeErr == nil {
					message = reason
				}
				response["error"] = map[string]interface{}{"code": 3, "message": message, "data": "0x" + ethcommon.Bytes2Hex(data)}
			}
			_ = json.NewEncoder(w).Encode(response)
		}
		switch request.Method {
		case "proof_reopen":
			app := testkeeper.EVMTestApp
			next, openErr := teeattestor.NewPrecompile(h.keeper, &app.EvmKeeper, &app.UpgradeKeeper, upgradePlan)
			if openErr == nil {
				h.precompile = next
				h.resetEVM(h.stateDB.Ctx())
				reopens++
			}
			reply(true, openErr, nil)
		case "proof_threshold":
			governance, commit := h.stateDB.Ctx().CacheContext()
			changeErr := h.keeper.SetThreshold(governance, xwebtypes.MsgSetThreshold{Authority: xwebtypes.DefaultAuthority(), Threshold: 4})
			if changeErr == nil {
				commit()
				thresholdChanges++
			}
			reply(true, changeErr, nil)
		case "eth_call":
			if len(request.Params) != 2 {
				reply(nil, fmt.Errorf("eth_call requires call and block"), nil)
				return
			}
			var call rpcCall
			if decodeErr := json.Unmarshal(request.Params[0], &call); decodeErr != nil {
				reply(nil, decodeErr, nil)
				return
			}
			if call.To != h.precompile.Address() {
				reply(nil, fmt.Errorf("unexpected address %s", call.To), nil)
				return
			}
			input := ethcommon.FromHex(call.Data)
			if len(input) < 4 {
				reply(nil, fmt.Errorf("missing selector"), nil)
				return
			}
			counts[ethcommon.Bytes2Hex(input[:4])]++
			output, left, callErr := h.evm.StaticCall(caller, call.To, input, suppliedGas)
			if callErr == nil && suppliedGas-left != agentcommon.MethodGas(input[:4], len(input)) {
				callErr = fmt.Errorf("gas consumer disagrees with EVM")
			}
			if callErr == nil && len(h.stateDB.GetAllLogs()) != 0 {
				callErr = fmt.Errorf("view emitted a log")
			}
			reply("0x"+ethcommon.Bytes2Hex(output), callErr, output)
		default:
			reply(nil, fmt.Errorf("unsupported RPC method %s", request.Method), nil)
		}
	}))
	defer server.Close()
	config, err := json.Marshal(map[string]interface{}{
		"quotes": quotes, "badQuote": "0x" + ethcommon.Bytes2Hex(badQuote), "roots": roots,
		"event": json.RawMessage(eventJSON), "eventTopic": event.ID.Hex(),
		"mrtd": "0x" + ethcommon.Bytes2Hex(mrtd[:]), "reportData": "0x" + ethcommon.Bytes2Hex(reportData[:]),
		"timestamp": fmt.Sprint(blockOpen - 60),
	})
	require.NoError(t, err)
	gateway, err := filepath.Abs("../../../services/wallet/gateway")
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "--import", "tsx", "--input-type=module", "-e", proofTeeWallet, server.URL, string(config))
	command.Dir = gateway
	command.Env = []string{"NODE_ENV=test", "HYPERPAXEER_RPC_URL=" + server.URL,
		"SUPABASE_URL=https://supabase.test.invalid", "DATABASE_URL=postgres://localhost:5432/test",
		"CORS_ORIGINS=http://localhost:3000", "WALLET_MASTER_KEY=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}
	output, err := command.CombinedOutput()
	require.NoError(t, err, "services/wallet/gateway/src/precompiles.ts and routes/agentPrecompiles.ts consumer: %s", output)
	require.Contains(t, string(output), "tee attestor proof consumed")
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, reopens)
	require.Equal(t, 1, thresholdChanges)
	require.Equal(t, 1, counts["ffffffff"])
	// Each observation reached the EVM; the wallet cached only its serving probe.
	require.Equal(t, 9, counts["26f02502"])
	require.Equal(t, 43, counts["01ec24b8"])
	require.Equal(t, 14, counts["5fa55ca7"])
	require.Equal(t, 10, counts["71ded4b3"])
	require.Empty(t, h.stateDB.GetAllLogs())
	// Governance was the only write. Restore its expected value for comparison.
	final := h.keeper.ExportGenesis(h.stateDB.Ctx())
	require.Equal(t, uint32(4), final.Attestors.Threshold)
	final.Attestors.Threshold = original.Attestors.Threshold
	require.Equal(t, original, final)
}

const proofTeeWallet = `
import assert from 'node:assert/strict';
import { teeAttestor, TEE_ATTESTOR_ABI, AttestationVerified, decodeTeeAttestationResult, PRECOMPILE_ADDRESSES } from './src/precompiles.ts';
import { teeVerificationResponse } from './src/routes/agentPrecompiles.ts';
import { encodeFunctionData, toEventSelector, toFunctionSelector } from 'viem';
const [rpcURL, configJSON] = process.argv.slice(1);
const config = JSON.parse(configJSON);
let id = 0;
async function rpc(method, params = []) {
  const response = await fetch(rpcURL, {
    method:'POST', headers:{'content-type':'application/json'},
    body:JSON.stringify({jsonrpc:'2.0', id:++id, method, params}),
  });
  assert.equal(response.status,200);
  const body = await response.json();
  if (body.error) throw new Error(body.error.message);
  return body.result;
}
const producerEvent = config.event;
assert.equal(AttestationVerified.name, producerEvent.name);
assert.equal(AttestationVerified.anonymous, producerEvent.anonymous);
assert.deepEqual(AttestationVerified.inputs, producerEvent.inputs.map(({name,type,indexed}) => ({name,type,indexed})));
assert.equal(toEventSelector(AttestationVerified), config.eventTopic);
assert.equal(TEE_ATTESTOR_ABI.filter(item => item.type === 'event').length, 1);
const names = ['intel_tdx','amd_sev_snp','nvidia_h100','intel_sgx'];
const selectors = {verify:'0x5fa55ca7',verifyAndExpect:'0x71ded4b3',rootOf:'0x01ec24b8',rootCount:'0x26f02502'};
for (const item of TEE_ATTESTOR_ABI.filter(item => item.type === 'function')) {
  assert.equal(toFunctionSelector(item), selectors[item.name]);
  assert.equal(item.stateMutability,'view');
}
for (let observation=0; observation<2; observation++) {
  if (observation) await rpc('proof_reopen');
  for (let family=0; family<4; family++) {
    assert.equal(await teeAttestor.rootCount(family),'5');
    for (let index=0; index<5; index++) assert.equal(await teeAttestor.rootOf(family,BigInt(index)),config.roots[index]);
    const expected = {family,family_name:names[family],mrtd:config.mrtd,report_data:config.reportData,timestamp:config.timestamp,debug:false};
    assert.deepEqual(await teeAttestor.verify(family,config.quotes[family]),expected);
    assert.deepEqual(await teeAttestor.verifyAndExpect(family,config.quotes[family],config.reportData),expected);
  }
}
assert.equal(await teeAttestor.rootCount(255),'0');
assert.equal(await teeAttestor.rootOf(255,0n),'0x');
assert.equal(await teeAttestor.rootOf(0,5n),'0x');
assert.equal(await teeAttestor.rootOf(0,1n<<64n),'0x');
await assert.rejects(() => teeAttestor.verify(4,config.quotes[0]),/LXP_AGENT_TEE_UNKNOWN_FAMILY/);
await assert.rejects(() => teeAttestor.verify(0,config.badQuote),/LXP_AGENT_TEE_UNTRUSTED_CHAIN/);
await assert.rejects(() => teeAttestor.verifyAndExpect(0,config.quotes[0],'0x'+'ff'.repeat(32)),/LXP_AGENT_TEE_REPORT_DATA_MISMATCH/);
const expected = {family:0,family_name:names[0],mrtd:config.mrtd,report_data:config.reportData,timestamp:config.timestamp,debug:false};
for (const expected_report_data of [undefined, config.reportData]) {
  assert.deepEqual(await teeVerificationResponse({family:'intel_tdx',quote:config.quotes[0],expected_report_data}),{status:200,body:{attestation:expected}});
}
const refused = await teeVerificationResponse({family:0,quote:config.badQuote});
assert.equal(refused.status,422);
assert.equal(refused.body.error,'attestation_failed');
assert.match(refused.body.detail,/LXP_AGENT_TEE_UNTRUSTED_CHAIN/);
assert.equal((await teeVerificationResponse({family:9,quote:'0x'})).status,400);
// Corrupt actual EVM output only after it crosses the production transport.
const data = encodeFunctionData({abi:TEE_ATTESTOR_ABI,functionName:'verify',args:[0,config.quotes[0]]});
const raw = await rpc('eth_call',[{to:PRECOMPILE_ADDRESSES.teeAttestor,data},'latest']);
assert.deepEqual(decodeTeeAttestationResult(raw,0),expected);
assert.throws(() => decodeTeeAttestationResult(raw.slice(0,-2),0));
assert.throws(() => decodeTeeAttestationResult(raw+'00',0),/invalid attestation result/);
assert.throws(() => decodeTeeAttestationResult(raw,1),/invalid attestation result/);
await rpc('proof_threshold');
await assert.rejects(() => teeAttestor.verify(0,config.quotes[0]),/LXP_AGENT_TEE_UNTRUSTED_CHAIN/);
console.log('tee attestor proof consumed');
`
