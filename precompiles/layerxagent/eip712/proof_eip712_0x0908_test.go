// Proves: eip712.NewMergePrecompile/Eip712Precompile.Execute -> vm.EVM.StaticCall -> gateway eip712.hashTypedData/domainSeparator/recoverTypedSigner.
//
// The node application supplies the real keepers, upgrade record, precompile
// registry and StateDB. The HTTP endpoint below forwards every eth_call into a
// new EVM over that state; it never substitutes a precompile or its output. The
// gateway's own helpers encode the calls and decode the producer's bytes.
// No EIP-712 state persists, so each input is consumed twice and a second signed
// message proves that a cached result cannot satisfy the consumer.
//
// Repairs services/wallet/gateway/src/precompiles.ts: EIP-712 reads previously
// accepted trailing return bytes and nonzero address padding through permissive
// ABI decoding. decodeEip712Result now refuses both, as well as truncated words.
// Damaged responses in this proof are explicit mutations of real EVM output at
// the consumer decoder, never a replacement producer or transport.
package eip712_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/eip712"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/stretchr/testify/require"
)

func TestProofEip7120x0908(t *testing.T) {
	application := app.Setup(t, false, true, false)
	ctx, _ := application.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(100).WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	application.UpgradeKeeper.SetDone(ctx, precompiles.MergeUpgrade)
	address := common.HexToAddress(eip712.Eip712Address)
	p, err := eip712.NewMergePrecompile(application.GetPrecompileKeepers(), precompiles.MergeUpgrade)
	require.NoError(t, err)
	require.Equal(t, address, p.Address())
	require.Equal(t, "eip712", p.GetName())
	unavailable, err := eip712.NewMergePrecompile(application.GetPrecompileKeepers(), "")
	require.Nil(t, unavailable)
	require.ErrorIs(t, err, agentcommon.ErrNotActive)
	registered := application.EvmKeeper.CustomPrecompiles(ctx)
	require.Contains(t, registered, address)
	require.IsType(t, p, registered[address])
	require.NotContains(t, application.EvmKeeper.CustomPrecompiles(ctx.WithBlockHeight(99)), address)
	surface := p.GetABI()
	block, err := application.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(application.EvmKeeper.ChainID(ctx))

	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	signer := crypto.PubkeyToAddress(key.PublicKey)
	chainID := ethmath.HexOrDecimal256(*big.NewInt(125))
	typed := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {{Name: "name", Type: "string"}, {Name: "version", Type: "string"}, {Name: "chainId", Type: "uint256"}, {Name: "verifyingContract", Type: "address"}},
			"Request":      {{Name: "amount", Type: "uint256"}},
		},
		PrimaryType: "Request",
		Domain:      apitypes.TypedDataDomain{Name: "Paxeer", Version: "1", ChainId: &chainID, VerifyingContract: address.Hex()},
	}
	separator, err := typed.HashStruct("EIP712Domain", typed.Domain.Map())
	require.NoError(t, err)
	vectors := make([]map[string]string, 0, 2)
	for _, amount := range []string{"42", "43"} {
		typed.Message = apitypes.TypedDataMessage{"amount": amount}
		structHash, err := typed.HashStruct("Request", typed.Message)
		require.NoError(t, err)
		digest, _, err := apitypes.TypedDataAndHash(typed)
		require.NoError(t, err)
		signature, err := crypto.Sign(digest, key)
		require.NoError(t, err)
		signature[64] += 27
		vectors = append(vectors, map[string]string{
			"domain": hexutil.Encode(separator), "structHash": hexutil.Encode(structHash),
			"digest": hexutil.Encode(digest), "signature": hexutil.Encode(signature), "signer": signer.Hex(),
		})
	}

	var mu sync.Mutex
	calls := map[string]int{}
	var executions []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		var request struct {
			JSONRPC string            `json:"jsonrpc"`
			ID      json.RawMessage   `json:"id"`
			Method  string            `json:"method"`
			Params  []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		response := map[string]interface{}{"jsonrpc": "2.0", "id": request.ID}
		fail := func(code int, message string, data string) {
			response["error"] = map[string]interface{}{"code": code, "message": message, "data": data}
		}
		if request.Method != "eth_call" || len(request.Params) != 2 {
			fail(-32601, "only real EVM eth_call is supported", "0x")
		} else {
			var call struct {
				To   common.Address `json:"to"`
				Data hexutil.Bytes  `json:"data"`
			}
			if err := json.Unmarshal(request.Params[0], &call); err != nil || call.To != address {
				fail(-32602, "invalid precompile call", "0x")
			} else {
				db := state.NewDBImpl(ctx.WithEventManager(sdk.NewEventManager()), &application.EvmKeeper, false)
				evm := vm.NewEVM(*block, db, chain, vm.Config{}, registered)
				output, left, callErr := evm.StaticCall(common.Address{}, address, call.Data, 2000000)
				if method, err := surface.MethodById(call.Data); err == nil {
					calls[method.Name]++
					if callErr == nil {
						base := uint64(1000)
						if method.Name == eip712.RecoverTypedSignerMethod {
							base = 4000
						}
						expectedGas := base + 3*uint64(len(call.Data))
						if quoted := agentcommon.MethodGas(method.ID, len(call.Data)); quoted != expectedGas {
							executions = append(executions, fmt.Sprintf("%s gas quote: got %d want %d", method.Name, quoted, expectedGas))
						}
						if used := uint64(2000000) - left; used != expectedGas {
							executions = append(executions, fmt.Sprintf("%s gas: got %d want %d", method.Name, used, expectedGas))
						}
						if len(output) != 32 {
							executions = append(executions, fmt.Sprintf("%s returned %d bytes", method.Name, len(output)))
						}
					}
				} else {
					calls["probe"]++
				}
				if callErr != nil {
					reason, decodeErr := abi.UnpackRevert(output)
					if decodeErr != nil {
						executions = append(executions, decodeErr.Error())
					}
					fail(3, reason, hexutil.Encode(output))
				} else {
					response["result"] = hexutil.Encode(output)
				}
			}
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			executions = append(executions, err.Error())
		}
	}))
	defer server.Close()

	payload, err := json.Marshal(vectors)
	require.NoError(t, err)
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../../.."))
	gateway := filepath.Join(root, "services/wallet/gateway")
	deadline, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(deadline, "node", "--import", "tsx", "--input-type=module", "--eval", proofEip712Consumer)
	command.Dir = gateway
	command.Env = append(os.Environ(), "HYPERPAXEER_RPC_URL="+server.URL, "EIP712_PROOF_VECTORS="+string(payload))
	output, err := command.CombinedOutput()
	require.NoError(t, err, "real gateway consumer failed:\n%s", output)
	require.Contains(t, string(output), "EIP712_CONSUMER_PROVED")
	mu.Lock()
	defer mu.Unlock()
	require.Empty(t, executions)
	require.Equal(t, map[string]int{"probe": 1, "domainSeparator": 5, "hashTypedData": 4, "recoverTypedSigner": 7}, calls,
		"every gateway read must execute again; only the service probe may be cached")
	require.Empty(t, ctx.EventManager().Events(), "pure/view helpers must not emit application events")
}

const proofEip712Consumer = `
import assert from 'node:assert/strict';
await import('./test/setup.ts');
const { eip712, decodeEip712Result, EIP712_ABI, PRECOMPILE_ADDRESSES } = await import('./src/precompiles.ts');
const { publicClient } = await import('./src/chainReads.ts');
const { encodeFunctionData } = await import('viem');
const vectors = JSON.parse(process.env.EIP712_PROOF_VECTORS);
assert.notEqual(vectors[0].digest, vectors[1].digest);
for (const vector of vectors) {
  for (let repeat = 0; repeat < 2; repeat++) {
    const domain = await eip712.domainSeparator({ name:'Paxeer', version:'1', chainId:125n, verifyingContract:PRECOMPILE_ADDRESSES.eip712 });
    assert.equal(domain, vector.domain);
    const digest = await eip712.hashTypedData(domain, vector.structHash);
    assert.equal(digest, vector.digest);
    const signer = await eip712.recoverTypedSigner(domain, vector.structHash, vector.signature);
    assert.equal(signer.toLowerCase(), vector.signer.toLowerCase());
    for (const method of ['hashTypedData', 'domainSeparator']) {
      const word = method === 'hashTypedData' ? digest : domain;
      for (const damaged of [undefined, '0x', word.slice(0, -2), word + '00']) {
        assert.throws(() => decodeEip712Result(method, damaged), /expected one ABI word/);
      }
    }
  }
}
const vector = vectors[0];
assert.equal(await eip712.recoverTypedSigner(vector.domain, vector.structHash, '0x'), '0x' + '00'.repeat(20));
assert.notEqual((await eip712.recoverTypedSigner(vector.domain, vectors[1].structHash, vector.signature)).toLowerCase(), vector.signer.toLowerCase());
await assert.rejects(eip712.domainSeparator({ name:'n'.repeat(257), version:'1', chainId:125n, verifyingContract:PRECOMPILE_ADDRESSES.eip712 }), /LXP_AGENT_BATCH_TOO_LARGE/);
const { data } = await publicClient().call({ to:PRECOMPILE_ADDRESSES.eip712, data:encodeFunctionData({ abi:EIP712_ABI, functionName:'recoverTypedSigner', args:[vector.domain, vector.structHash, vector.signature] }) });
assert.equal(decodeEip712Result('recoverTypedSigner', data).toLowerCase(), vector.signer.toLowerCase());
for (const damaged of [undefined, '0x', data.slice(0, -2), data + '00']) {
  assert.throws(() => decodeEip712Result('recoverTypedSigner', damaged), /expected one ABI word/);
}
assert.throws(() => decodeEip712Result('recoverTypedSigner', '0x01' + data.slice(4)), /nonzero address padding/);
console.log('EIP712_CONSUMER_PROVED');
`
