package xweb_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	xwebkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestRepair_130_ProgramEventSemantics(t *testing.T) {
	repair130RoundTrip(t, false)
}

func TestRepair_130_ProgramEVMJournalCollision(t *testing.T) {
	repair130RoundTrip(t, true)
}

func repair130Consumer(t *testing.T) func(map[string]interface{}) []byte {
	t.Helper()
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	dir := t.TempDir()
	manifest := fmt.Sprintf(`[package]
name = "xweb-repair-130"
version = "0.1.0"
edition = "2021"
[workspace]
[dependencies]
x-websearch = { path = %q }
k256 = { version = "=0.13.4", default-features = false, features = ["ecdsa", "std"] }
serde_json = "=1.0.151"
[[bin]]
name = "xweb-repair-130"
path = %q
`, filepath.Join(root, "services/interop/crates/x-websearch"), filepath.Join(root, "precompiles/xweb/testdata/repair_130_consumer.rs"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(manifest), 0600))
	lock, err := os.ReadFile(filepath.Join(root, "services/interop/Cargo.lock"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Cargo.lock"), lock, 0600))
	return func(input map[string]interface{}) []byte {
		t.Helper()
		raw, err := json.Marshal(input)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "cargo", "run", "--quiet", "--manifest-path", filepath.Join(dir, "Cargo.toml"), "--bin", "xweb-repair-130")
		cmd.Dir, cmd.Stdin = root, bytes.NewReader(raw)
		cmd.Env = os.Environ()
		if os.Getenv("CARGO_TARGET_DIR") == "" {
			cmd.Env = append(cmd.Env, "CARGO_TARGET_DIR="+filepath.Join(root, "services/interop/target"))
		}
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		require.NoError(t, err, "real Rust watcher and signer: %s", stderr.String())
		return out
	}
}

func repair130RoundTrip(t *testing.T, includeEVM bool) {
	t.Helper()
	h := newProgramDispatchHarness(t)
	bank := testkeeper.EVMTestApp.BankKeeper
	id := uint64(17)
	var evmRequest xweb.RequestView
	if includeEVM {
		id = h.request(recorder, callbackGas)
		evmRequest = h.requestView(id)
	}
	programs := []struct {
		id       types.Hash32
		asset    types.Hash32
		kind     uint8
		sequence uint64
		denom    string
		payload  []byte
	}{
		{types.Hash32{0x91, 1}, types.Hash32{0xa1}, types.KindFetch, 0x0102030405060708, types.ProgramFeeDenomPAX, []byte("https://paxeer.app/fetch?raw=\x00\xff")},
		{types.Hash32{0x91, 2}, types.Hash32{0xa2}, types.KindSearch, 0xfedcba9876543210, types.ProgramFeeDenomUSDL, []byte("repair 130 search \x00\xfe")},
	}
	expected := make([]map[string]interface{}, 0, len(programs))
	for _, program := range programs {
		principal := h.account(stranger)
		coins := sdk.NewCoins(sdk.NewInt64Coin(program.denom, 75))
		require.NoError(t, bank.MintCoins(h.ctx(), "evm", coins))
		require.NoError(t, bank.SendCoinsFromModuleToAccount(h.ctx(), "evm", principal, coins))
		leg := xwebkeeper.ProgramLeg{From: principal, To: xwebkeeper.ProgramFeeAccount(program.asset), Asset: program.asset, Denom: program.denom, Amount: sdk.NewInt(75)}
		require.NoError(t, bank.SendCoins(h.ctx(), leg.From, leg.To, coins))
		call := &xwebkeeper.ProgramCall{ProgramID: program.id, Principal: principal, Sequence: program.sequence, Legs: []xwebkeeper.ProgramLeg{leg}}
		data := binary.BigEndian.AppendUint64(nil, id)
		data = append(data, program.kind)
		data = binary.BigEndian.AppendUint32(data, uint32(len(program.payload)))
		data = append(data, program.payload...)
		require.NoError(t, h.precompile.RequestFromProgram(h.evm, call, []byte(xwebkeeper.ProgramRequestTopic), data, false))
		record, found, err := h.keeper.GetProgramPendingRecord(h.ctx(), program.id, id)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, program.kind, record.Kind)
		require.Equal(t, types.Keccak(program.payload), record.PayloadHash)
		require.Equal(t, program.sequence, record.Sequence)
		require.Equal(t, program.asset, record.FeeAsset)
		require.Equal(t, xwebkeeper.ProgramFeeAccountID(program.asset), record.FeeAccount)
		requireAmount(t, sdk.NewInt(75), record.Amount)
		denom, found := h.keeper.ProgramFeeDenom(h.ctx(), record.FeeAsset)
		require.True(t, found)
		require.Equal(t, program.denom, denom)
		expected = append(expected, map[string]interface{}{"program_id": common.Hash(program.id).Hex(), "request_id": id, "kind": record.Kind, "payload": "0x" + common.Bytes2Hex(program.payload), "sequence": record.Sequence})
	}
	require.Len(t, h.logs("XWebProgramRequested"), 2)
	evmCount := 0
	if includeEVM {
		evmCount = 1
	}
	require.Len(t, h.logs("XWebRequested"), evmCount)
	require.Len(t, h.stateDB.GetAllLogs(), evmCount+2)

	// Expose the actual StateDB logs over JSON-RPC, preserving their ABI bytes.
	// Supply block/transaction identities at the receipt boundary and deliver
	// each receipt twice to exercise the watcher's durable deduplication.
	logs := make([]interface{}, 0, 2*(evmCount+2))
	for i, emitted := range h.stateDB.GetAllLogs() {
		log := *emitted
		log.BlockNumber = uint64(startHeight)
		log.BlockHash = crypto.Keccak256Hash([]byte("repair 130 committed block"))
		log.TxHash = crypto.Keccak256Hash([]byte(fmt.Sprintf("repair 130 transaction %d", i)))
		log.Index = uint(i)
		logs = append(logs, &log, &log)
	}
	chainID := h.evm.ChainConfig().ChainID.Uint64()
	var polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			http.Error(w, "invalid JSON-RPC", http.StatusBadRequest)
			return
		}
		var result interface{}
		switch request.Method {
		case "eth_blockNumber":
			result = fmt.Sprintf("0x%x", startHeight)
		case "eth_chainId":
			result = fmt.Sprintf("0x%x", chainID)
		case "eth_getLogs":
			var filter struct {
				Address string     `json:"address"`
				From    string     `json:"fromBlock"`
				To      string     `json:"toBlock"`
				Topics  [][]string `json:"topics"`
			}
			if len(request.Params) != 1 || json.Unmarshal(request.Params[0], &filter) != nil || common.HexToAddress(filter.Address) != precompile || filter.From != "0x8" || filter.To != "0x8" || len(filter.Topics) != 1 || len(filter.Topics[0]) != 2 {
				http.Error(w, "invalid request log filter", http.StatusBadRequest)
				return
			}
			polls.Add(1)
			result = logs
		default:
			http.Error(w, "unexpected RPC method", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	defer server.Close()
	keys, signers := make([]string, 2), make([]string, len(h.attestors))
	for i, attestor := range h.attestors {
		signers[i] = "0x" + common.Bytes2Hex(attestor.Signer[:])
		if i < 2 {
			keys[i] = "0x" + common.Bytes2Hex(crypto.FromECDSA(attestor.Key))
		}
	}
	answer := []byte("repair 130 watcher answer\x00\xff")
	input := map[string]interface{}{"mode": "observe", "rpc": server.URL, "state": t.TempDir(), "block": uint64(startHeight), "chain_id": chainID, "network_id": kernelNetworkID, "programs": expected, "evm": includeEVM, "request_id": id, "keys": keys, "signers": signers, "response": "0x" + common.Bytes2Hex(answer)}
	if includeEVM {
		input["timeout"], input["callback_gas"] = evmRequest.TimeoutHeight, evmRequest.CallbackGas
		input["paid"], input["evm_payload"] = common.BigToHash(evmRequest.Fee).Hex(), "0x"+common.Bytes2Hex(payload)
	}
	run := repair130Consumer(t)
	var decoded struct {
		Programs []struct {
			ProgramID   common.Hash `json:"program_id"`
			Observation string      `json:"observation"`
		} `json:"programs"`
		EVM *struct {
			Signatures []string `json:"signatures"`
		} `json:"evm"`
	}
	require.NoError(t, json.Unmarshal(run(input), &decoded))
	require.Equal(t, int32(1), polls.Load(), "restart must resume beyond the committed block")
	require.Len(t, decoded.Programs, 2)
	before := make([]sdk.Coins, len(h.attestors))
	for i, attestor := range h.attestors {
		before[i] = bank.GetAllBalances(h.ctx(), attestor.Payout)
	}
	for i, program := range programs {
		require.Equal(t, common.Hash(program.id), decoded.Programs[i].ProgramID)
		raw := common.FromHex(decoded.Programs[i].Observation)
		observation, err := types.DecodeObservation(raw)
		require.NoError(t, err)
		require.Equal(t, program.kind, observation.Kind)
		require.Equal(t, types.Keccak(program.payload), observation.PayloadHash)
		require.Equal(t, program.id, observation.ProgramID)
		require.Equal(t, id, observation.RequestID)
		_, _, reason := h.call(stranger, xweb.FulfilProgramMethod, nil, supplied, raw)
		require.Empty(t, reason)
		require.Equal(t, []interface{}{[32]byte(types.Keccak(answer)), uint32(len(answer)), uint32(len(answer)), answer}, h.view(xweb.GetProgramResultMethod, [32]byte(program.id), id))
		_, _, reason = h.call(stranger, xweb.FulfilProgramMethod, nil, supplied, raw)
		require.Equal(t, "LXP_SEQUENCE_REUSED", reason)
		require.True(t, bank.GetAllBalances(h.ctx(), xwebkeeper.ProgramFeeAccount(program.asset)).IsZero())
		record, found, err := h.keeper.GetProgramPendingRecord(h.ctx(), program.id, id)
		require.NoError(t, err)
		require.True(t, found)
		require.True(t, record.Fulfilled)
		require.Equal(t, program.sequence, record.Sequence)
	}
	if includeEVM {
		require.NotNil(t, decoded.EVM)
		require.Len(t, decoded.EVM.Signatures, 2)
		signatures := make([][]byte, len(decoded.EVM.Signatures))
		for i, sig := range decoded.EVM.Signatures {
			signatures[i] = common.FromHex(sig)
		}
		_, _, reason := h.call(stranger, xweb.FulfilMethod, nil, supplied, id, answer, [32]byte(types.Keccak(answer)), uint32(len(answer)), signatures)
		require.Empty(t, reason)
		require.Equal(t, answer, h.result(id).Response)
		_, _, reason = h.call(stranger, xweb.FulfilMethod, nil, supplied, id, answer, [32]byte(types.Keccak(answer)), uint32(len(answer)), signatures)
		require.NotEmpty(t, reason)
	} else {
		require.Nil(t, decoded.EVM)
	}
	for i, attestor := range h.attestors {
		want := before[i]
		if i < 2 {
			share := int64(38 - i)
			want = want.Add(sdk.NewInt64Coin(types.ProgramFeeDenomPAX, share), sdk.NewInt64Coin(types.ProgramFeeDenomUSDL, share))
			if includeEVM {
				want = want.Add(sdk.NewInt64Coin(sdk.MustGetBaseDenom(), fee/2+int64(1-i)))
			}
		}
		require.True(t, want.IsEqual(bank.GetAllBalances(h.ctx(), attestor.Payout)), "attestor %d must receive each fee once in its recorded denomination", i)
	}
	require.Zero(t, h.keeper.ProgramPendingCount(h.ctx()))
	require.Equal(t, uint64(2), h.keeper.ProgramCommittedCount(h.ctx()))
	input["mode"] = "retire"
	var retired struct {
		Retired bool `json:"retired"`
	}
	require.NoError(t, json.Unmarshal(run(input), &retired))
	require.True(t, retired.Retired)
	require.Equal(t, int32(1), polls.Load(), "retired work must stay retired after process restart")
}
