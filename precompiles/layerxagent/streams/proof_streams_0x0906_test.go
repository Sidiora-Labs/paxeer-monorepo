// Proves: paykeeper.Keeper and streams.NewMergePrecompile -> vm.EVM.Call/StaticCall and JSON-RPC -> wallet streams, decodeStreamEvent and agentcommon.MethodGas.
// Repairs services/wallet/gateway/src/precompiles.ts STREAMS_ABI: the wallet
// omitted all four native events, so consumers could not decode producer logs.
// The RPC adapter below only forwards calls to the application's real EVM; it
// never substitutes return values, keeper operations, logs or ABI encodings.
// Reopening commits and closes the stores and host, then reconstructs the pay
// keeper, both precompiles and EVM from disk. Truncation of an actual emitted event
// is refused by the wallet's own strict decoder.
// modules/layerxpay/keeper/stream_views.go StreamAccrued returns cumulative
// accrual; the wallet computes unpaid value by subtracting getStream.settled.
package streams_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/streams"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestProofStreams0x0906(t *testing.T) {
	persisted := newProofStreamsStore(t)
	h := persisted.h
	keepers := payKeepers{testkeeper.EVMTestApp.GetPrecompileKeepers(), h.keeper}
	var producer precompiles.IPrecompile = h.precompile
	reopen := func() error {
		persisted.reopen()
		producer = h.precompile
		return nil
	}
	require.Equal(t, common.HexToAddress(streams.StreamsAddress), producer.Address())
	require.Equal(t, streams.PrecompileName, producer.GetName())
	_, err := streams.NewMergePrecompile(keepers, "")
	require.ErrorIs(t, err, agentcommon.ErrNotActive)
	require.NotContains(t, precompiles.MissingMergeConstructors(), precompiles.MergeAddress{Address: producer.Address(), Name: streams.PrecompileName})

	type call struct {
		From common.Address `json:"from"`
		To   common.Address `json:"to"`
		Data string         `json:"data"`
	}
	type wireLog struct {
		Address string        `json:"address"`
		Topics  []common.Hash `json:"topics"`
		Data    string        `json:"data"`
	}
	var mu sync.Mutex
	counts := make(map[string]int)
	var producedLogs []*types.Log
	reopenCount := 0
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
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		reply := func(result interface{}, err error, data []byte) {
			response := map[string]interface{}{"jsonrpc": "2.0", "id": request.ID}
			if err != nil {
				message := err.Error()
				if reason, decodeErr := abi.UnpackRevert(data); decodeErr == nil {
					message = reason
				}
				response["error"] = map[string]interface{}{"code": 3, "message": message, "data": common.Bytes2Hex(data)}
			} else {
				response["result"] = result
			}
			_ = json.NewEncoder(w).Encode(response)
		}
		switch request.Method {
		case "proof_advance":
			var seconds int64
			if len(request.Params) != 1 {
				reply(nil, fmt.Errorf("advance requires seconds"), nil)
				return
			}
			if err := json.Unmarshal(request.Params[0], &seconds); err != nil {
				reply(nil, err, nil)
				return
			}
			h.advance(seconds)
			reply(true, nil, nil)
		case "proof_reopen":
			err := reopen()
			reopenCount++
			reply(true, err, nil)
		case "eth_call", "proof_execute":
			var c call
			if len(request.Params) < 1 {
				reply(nil, fmt.Errorf("missing call"), nil)
				return
			}
			if err := json.Unmarshal(request.Params[0], &c); err != nil {
				reply(nil, err, nil)
				return
			}
			if c.To != producer.Address() {
				reply(nil, fmt.Errorf("unexpected precompile %s", c.To), nil)
				return
			}
			input := common.FromHex(c.Data)
			selector := common.Bytes2Hex(input)
			if len(input) >= 4 {
				selector = common.Bytes2Hex(input[:4])
			}
			counts[request.Method+":"+selector]++
			before := len(h.db.GetAllLogs())
			var output []byte
			var err error
			if request.Method == "eth_call" {
				output, _, err = h.evm.StaticCall(h.other.caller, c.To, input, 2000000)
			} else {
				output, _, err = h.evm.Call(c.From, c.To, input, 2000000, uint256.NewInt(0))
			}
			if err == nil {
				err = h.db.Err()
			}
			if err != nil {
				reply(nil, err, output)
				return
			}
			if request.Method == "eth_call" {
				reply("0x"+common.Bytes2Hex(output), nil, nil)
				return
			}
			logs := make([]wireLog, 0)
			for _, entry := range h.db.GetAllLogs()[before:] {
				producedLogs = append(producedLogs, entry)
				logs = append(logs, wireLog{entry.Address.Hex(), entry.Topics, "0x" + common.Bytes2Hex(entry.Data)})
			}
			reply(map[string]interface{}{"output": "0x" + common.Bytes2Hex(output), "logs": logs, "gas": producer.RequiredGas(input)}, nil, nil)
		default:
			reply(nil, fmt.Errorf("unsupported proof RPC method %s", request.Method), nil)
		}
	}))
	defer server.Close()

	gateway, err := filepath.Abs("../../../services/wallet/gateway")
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "--import", "tsx", "--input-type=module", "-e", proofStreamsWallet)
	command.Dir = gateway
	command.Env = append(os.Environ(),
		"NODE_ENV=test", "HYPERPAXEER_RPC_URL="+server.URL,
		"SUPABASE_URL=https://supabase.test.invalid", "DATABASE_URL=postgres://localhost:5432/test",
		"CORS_ORIGINS=http://localhost:3000", "WALLET_MASTER_KEY=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"PROOF_PAYER="+h.payer.caller.Hex(), "PROOF_PAYEE="+h.payee.caller.Hex(),
		"PROOF_OTHER="+h.other.caller.Hex(), "PROOF_TOKEN="+h.token.Hex(),
	)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "wallet consumer failed: %s", output)
	require.Contains(t, string(output), "streams proof consumed")

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, reopenCount)
	for selector, writes := range map[string]int{"93e5b08b": 1, "8df82800": 3, "405abb41": 1, "0aebeb4e": 1} {
		require.Equal(t, writes, counts["proof_execute:"+selector], selector)
	}
	require.Equal(t, 1, counts["eth_call:ffffffff"])
	require.Equal(t, 9, counts["eth_call:894e9a0d"])
	require.Equal(t, 5, counts["eth_call:f52981f4"])

	id := new(big.Int).SetBytes(producedLogs[0].Topics[1].Bytes())
	record := h.record(id)
	require.True(t, record.Closed)
	require.Equal(t, codec.U128{Lo: 3}, record.Rate)
	require.Equal(t, codec.U128{Lo: 19}, record.SettledTotal)
	for account, expected := range map[[32]byte]string{record.Payer: "981", record.Recipient: "19", record.StreamAccount: "0"} {
		balance, err := h.keeper.Host().ReadBalance(h.db.Ctx(), account, h.asset)
		require.NoError(t, err)
		require.Equal(t, expected, balance.String())
	}
	require.Len(t, producedLogs, 6)
	require.Equal(t, common.HexToAddress("0x101b"), producedLogs[4].Address)

	// The same real producer ABI feeds the gas consumer; charge full calldata.
	for _, row := range []struct {
		name string
		args []interface{}
		want uint64
	}{
		{"open", []interface{}{h.payee.caller, h.token, big.NewInt(2), streamsStart, streamsStart + 100, big.NewInt(100)}, 55880},
		{"settle", []interface{}{id}, 61080},
		{"close", []interface{}{id}, 81080},
		{"updateRate", []interface{}{id, big.NewInt(3)}, 62040},
		{"accrued", []interface{}{id}, 1108},
		{"getStream", []interface{}{id}, 1108},
	} {
		input := h.input(row.name, row.args...)
		require.Equal(t, row.want, agentcommon.MethodGas(input[:4], len(input)), row.name)
		require.Equal(t, row.want, producer.RequiredGas(input), row.name)
	}
}

const proofStreamsWallet = `
import assert from 'node:assert/strict';
import { streams, STREAMS_ABI, PRECOMPILE_ADDRESSES, decodeStreamEvent } from './src/precompiles.ts';
import { decodeFunctionResult } from 'viem';
const { PROOF_PAYER:payer, PROOF_PAYEE:payee, PROOF_OTHER:other, PROOF_TOKEN:token } = process.env;
let requestID = 0;
async function rpc(method, params = []) {
  const response = await fetch(process.env.HYPERPAXEER_RPC_URL, {
    method: 'POST', headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ jsonrpc:'2.0', id:++requestID, method, params }),
  });
  assert.equal(response.status, 200);
  const body = await response.json();
  if (body.error) throw new Error(body.error.message);
  return body.result;
}
const address = '0x0000000000000000000000000000000000000906';
assert.equal(PRECOMPILE_ADDRESSES.streams, address);
async function send(name, call, caller, gas) {
  assert.equal(call.to, address);
  const result = await rpc('proof_execute', [{ ...call, from:caller }]);
  assert.equal(result.gas, gas);
  const value = decodeFunctionResult({ abi:STREAMS_ABI, functionName:name, data:result.output });
  const events = result.logs.filter(log => log.address.toLowerCase() === address).map(decodeStreamEvent);
  return { value, events, logs:result.logs };
}
function event(result, name, expected) {
  assert.equal(result.events.length, 1);
  assert.equal(result.events[0].eventName, name);
  assert.deepEqual(result.events[0].args, expected);
}
async function observe(id, rate, settled, active) {
  const first = await streams.getStream(id);
  assert.deepEqual(await streams.getStream(id), first);
  assert.deepEqual(first, { id:id.toString(), payer, payee, token, ratePerSecond:rate,
    cap:'100', startTime:'1700000000', stopTime:'1700000100', settled, active });
  return first;
}
const opened = await send('open', streams.open({payee,token,ratePerSecond:2n,
  startTime:1700000000n,stopTime:1700000100n,cap:100n}), payer,55880);
const id = opened.value;
assert(id > (1n << 160n));
event(opened,'StreamOpened',{streamId:id,payer,payee,token,ratePerSecond:2n,cap:100n});
// Decode a damaged real producer log at the actual wallet decoder boundary.
assert.throws(() => decodeStreamEvent({...opened.logs[0],data:opened.logs[0].data.slice(0,-2)}));
await rpc('proof_reopen');
await observe(id,'2','0',true);
await rpc('proof_advance',[5]);
assert.equal(await streams.accrued(id),'10');
assert.equal(await streams.accrued(id),'10');
await assert.rejects(() => send('settle',streams.settle(id),other,61080), /LXP_UNAUTHORIZED_DEBIT/);
await observe(id,'2','0',true);
const settled = await send('settle',streams.settle(id),payee,61080);
assert.equal(settled.value,10n);
event(settled,'StreamSettled',{streamId:id,payee,paid:10n,committed:10n});
const duplicate = await send('settle',streams.settle(id),payee,61080);
assert.equal(duplicate.value,0n);
event(duplicate,'StreamSettled',{streamId:id,payee,paid:0n,committed:0n});
await rpc('proof_advance',[8]);
const updated = await send('updateRate',streams.updateRate(id,3n),payer,62040);
assert.equal(updated.value,undefined);
event(updated,'StreamRateUpdated',{streamId:id,oldRate:2n,newRate:3n});
assert.equal(updated.logs.length,2);
const afterUpdate = await observe(id,'3','16',true);
await rpc('proof_advance',[9]);
for (let observation = 0; observation < 2; observation++) {
  const accrued = await streams.accrued(id);
  assert.equal(accrued,'19');
  assert.equal(BigInt(accrued) - BigInt(afterUpdate.settled),3n);
}
const closed = await send('close',streams.close(id),payer,81080);
assert.equal(closed.value,3n);
event(closed,'StreamClosed',{streamId:id,finalPaid:3n,refund:81n});
await observe(id,'3','19',false);
assert.equal(await streams.getStream(0n),null);
assert.equal(await streams.accrued(0n),'0');
console.log('streams proof consumed');
`
