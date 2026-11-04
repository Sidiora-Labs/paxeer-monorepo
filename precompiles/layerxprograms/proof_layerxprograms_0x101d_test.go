// Proves: lxphost.Host.Execute/ReadProgramStateHead -> vm.EVM.Call/StaticCall at layerxprograms.Address -> ai_market SDK encoders/decoders and ai-evaluator programs_wire.
//
// Repairs services/platform/hosted/ai-evaluator/src/main.rs Chain::require_active:
// getStateHead is a storage manifest head, whereas getProgram supplies lifecycle.
// Records an out-of-scope defect in modules/lxphost/host.go Host.Execute and
// modules/lxphost/simulate.go Host.Simulate: ProgramOutcome.Response contains the
// entire terminal execution frame, where the ABI consumer expects guest bytes.
// Assert the exact current producer frame against independent cached execution;
// SDK consumers must refuse it at the unchanged 64-byte guest response capacity.
// Also repairs decode_terminal accepting noncanonical sign extension, offsets,
// kinds, padding and trailing data; Python lacked a terminal-result decoder.
// The proof compiles the evaluator's dependency-free programs_wire module verbatim,
// the same module used by Chain and receipt_outcome, and calls the SDKs' own code.
// No successful return or log is constructed by the proof. Malformed cases mutate
// only returns from real EVM calls. Host.Close/NewHost reopens the same SDK store.
package layerxprograms_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type proofProgramsFrames struct {
	ExpectedResponses []string          `json:"expectedResponses"`
	CallInput         string            `json:"callInput"`
	Calls             []string          `json:"calls"`
	Simulations       []string          `json:"simulations"`
	Heads             []string          `json:"heads"`
	StateHead         string            `json:"stateHead"`
	Refusals          map[string]string `json:"refusals"`
	MalformedCalls    []string          `json:"malformedCalls"`
	MalformedHeads    []string          `json:"malformedHeads"`
	Transfers         []*types.Log      `json:"transfers"`
	TransferProgram   string            `json:"transferProgram"`
}

func proofProgramsRun(t *testing.T, root string, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s %v: %s", name, args, output)
	return strings.TrimSpace(string(output))
}

func proofProgramsWrite(t *testing.T, path string, value interface{}) {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))
}

func proofProgramsReopen(t *testing.T, h *harness) {
	t.Helper()
	before := h.dump(h.stateDB.Ctx())
	h.host.Close()
	h.host = h.newHost(1000000)
	h.executor.SetTestHost(h.host)
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
}

func call(t *testing.T, h *harness, input, payload []byte, limit *big.Int) ([]byte, []byte) {
	t.Helper()
	reference, _ := h.stateDB.Ctx().CacheContext()
	method := lp.ProgramsABI.Methods["call"]
	var fee [16]byte
	limit.FillBytes(fee[:])
	receipt, err := lp.Submit(reference, h.executor, lp.NewCallContext(h.caller, &method), 0x00090003, payload, fee)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	require.NotNil(t, receipt.ProgramOutcome)
	expectedResponse := receipt.ProgramOutcome.Response
	// Current host behavior: the terminal frame is exposed instead of guest response bytes.
	require.True(t, bytes.HasPrefix(expectedResponse, []byte("LXP/programs/terminal-applied-legs/v1\x00")))
	require.Greater(t, len(expectedResponse), 64)
	output, _, err := h.evm.Call(h.caller, lp.Address, input, 20000000, uint256.NewInt(0))
	require.NoError(t, err, "call output %x", output)
	values, err := method.Outputs.Unpack(output)
	require.NoError(t, err)
	require.Equal(t, []interface{}{int32(0), uint8(1), expectedResponse}, values)
	require.Equal(t, h.dump(reference), h.dump(h.stateDB.Ctx()))
	return output, expectedResponse
}

func simulateCall(t *testing.T, h *harness, args []interface{}) []byte {
	t.Helper()
	before := h.dump(h.stateDB.Ctx())
	input, err := lp.ProgramsABI.Pack("simulateCall", args...)
	require.NoError(t, err)
	decodedArgs, err := lp.ProgramsABI.Methods["simulateCall"].Inputs.Unpack(input[4:])
	require.NoError(t, err)
	t.Logf("simulateCall decoded arguments: %#v; expected: %#v", decodedArgs, args)
	require.Equal(t, args, decodedArgs)
	output, _, err := h.evm.StaticCall(h.caller, lp.Address, input, 20000000)
	reason, _ := abi.UnpackRevert(output)
	require.NoError(t, err, "simulateCall output=%x reason=%s", output, reason)
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	return output
}

func getStateHead(t *testing.T, h *harness, program [32]byte) []byte {
	t.Helper()
	input, err := lp.ProgramsABI.Pack("getStateHead", program)
	require.NoError(t, err)
	output, _, err := h.evm.StaticCall(h.caller, lp.Address, input, 2000000)
	require.NoError(t, err)
	values, err := lp.ProgramsABI.Methods["getStateHead"].Outputs.Unpack(output)
	require.NoError(t, err)
	head := values[0].([]byte)
	// Empty storage is a version/count/digest manifest head, not LXPS1 lifecycle.
	digest := sha256.Sum256([]byte{0, 1, 0, 0, 0, 0})
	require.Equal(t, append([]byte{0, 1, 0, 0, 0, 0}, digest[:]...), head)
	return output
}

func quoteCallFee(t *testing.T, h *harness, budget [7]uint64) {
	t.Helper()
	record, err := h.host.ReadFeeSchedule(h.stateDB.Ctx())
	require.NoError(t, err)
	asset := [32]byte(record[65:97])
	expected := new(big.Int)
	for i, limit := range budget {
		price := binary.BigEndian.Uint64(record[9+i*8 : 17+i*8])
		expected.Add(expected, new(big.Int).Mul(new(big.Int).SetUint64(price), new(big.Int).SetUint64(limit)))
	}
	input, err := lp.ProgramsABI.Pack("quoteCallFee", budget)
	require.NoError(t, err)
	before := h.dump(h.stateDB.Ctx())
	for i := 0; i < 2; i++ {
		output, _, err := h.evm.StaticCall(h.caller, lp.Address, input, 2000000)
		require.NoError(t, err)
		values, err := lp.ProgramsABI.Methods["quoteCallFee"].Outputs.Unpack(output)
		require.NoError(t, err)
		require.Equal(t, []interface{}{binary.BigEndian.Uint32(record[5:9]), expected, asset}, values)
		require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	}
	// The ABI refuses malformed fixed-array input before kernel state can change.
	output, _, err := h.evm.StaticCall(h.caller, lp.Address, input[:len(input)-32], 2000000)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.NotEmpty(t, output)
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
}

func transfer(t *testing.T) (*types.Log, [32]byte) {
	t.Helper()
	h, program, legs := newProgramsTransferHarness(t)
	// An overdraft must be atomic and must emit no success log.
	refused := append([]lp.TransferLeg(nil), legs...)
	refused[1].Amount = big.NewInt(71)
	input, err := lp.ProgramsABI.Pack("transfer", program, refused)
	require.NoError(t, err)
	before := h.dump(h.stateDB.Ctx())
	output, _, err := h.evm.Call(h.caller, lp.Address, input, 2000000, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(output)
	require.NoError(t, err)
	require.Equal(t, lp.KernelError(lxphost.Code(-400), "transfer").Error(), reason)
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	require.Empty(t, h.stateDB.GetAllLogs())
	input, err = lp.ProgramsABI.Pack("transfer", program, legs)
	require.NoError(t, err)
	output, _, err = h.evm.Call(h.caller, lp.Address, input, 2000000, uint256.NewInt(0))
	require.NoError(t, err)
	require.Empty(t, output)
	proofProgramsReopen(t, h)
	assertProgramsTransferBalances(t, h, legs, 30, 30, 40)
	logs := h.stateDB.GetAllLogs()
	require.Len(t, logs, 1)
	event, err := lp.DecodeProgramTransferred(logs[0])
	require.NoError(t, err)
	require.Equal(t, common.Hash(program), event.ProgramID)
	require.NotEqual(t, common.Hash{}, event.TransferSetRoot)
	malformed := *logs[0]
	malformed.Data = append(bytes.Clone(malformed.Data), 0)
	_, err = lp.DecodeProgramTransferred(&malformed)
	require.Error(t, err)
	malformed = *logs[0]
	malformed.Address = common.Address{}
	_, err = lp.DecodeProgramTransferred(&malformed)
	require.Error(t, err)
	return logs[0], program
}

func retireStorage(t *testing.T) {
	t.Helper()
	for repeat := 0; repeat < 2; repeat++ {
		h := newHarness(t)
		request, manifest, digest := retirementRequest(t, h)
		input, err := lp.ProgramsABI.Pack("retireStorage", request)
		require.NoError(t, err)
		output, _, err := h.evm.Call(h.caller, lp.Address, input, 2000000, uint256.NewInt(0))
		require.NoError(t, err)
		require.Empty(t, output)
		logs := h.stateDB.GetAllLogs()
		require.Len(t, logs, 2)
		for i, log := range logs {
			event := lp.ProgramsABI.Events["StorageRetired"]
			require.Equal(t, lp.Address, log.Address)
			require.Equal(t, event.ID, log.Topics[0])
			values, err := event.Inputs.NonIndexed().Unpack(log.Data)
			require.NoError(t, err)
			require.Equal(t, uint8(i), values[0])
			frame := values[1].([]byte)
			require.Equal(t, []byte{'L', 'X', 'R', 'T', byte(i), 2, 1}, frame[:7])
			if i == 1 {
				require.Equal(t, digest[:], frame[9:41])
			}
		}
		proofProgramsReopen(t, h)
		require.False(t, h.stateDB.Ctx().KVStore(h.key).Has(append([]byte{5, 0, 9}, digest[:]...)))
		control, err := h.host.ReadRetirementControl(h.stateDB.Ctx())
		require.NoError(t, err)
		require.Equal(t, uint64(len(manifest)), binary.BigEndian.Uint64(control[203:211]))
		before := h.dump(h.stateDB.Ctx())
		input, err = lp.ProgramsABI.Pack("retireStorage", make([]byte, 32769))
		require.NoError(t, err)
		output, _, err = h.evm.Call(h.caller, lp.Address, input, 2000000, uint256.NewInt(0))
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		reason, err := abi.UnpackRevert(output)
		require.NoError(t, err)
		require.Equal(t, "LXP_LENGTH_LIMIT", reason)
		require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	}
}

func TestProofLayerxprograms0x101d(t *testing.T) {
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	dir := t.TempDir()
	fixturePath := filepath.Join(dir, "producer.json")
	h, args, payload := programsCallFixture(t)
	require.Equal(t, lp.Address, h.precompile.Address())
	require.Equal(t, common.HexToAddress("0x101d"), lp.Address)
	input, err := lp.ProgramsABI.Pack("call", args...)
	require.NoError(t, err)
	frames := proofProgramsFrames{CallInput: hex.EncodeToString(input), Refusals: make(map[string]string)}
	proofProgramsWrite(t, fixturePath, frames)
	// Use the actual SDK encoders as the source of the bytes entering EVM.Call.
	python := "python3"
	if configured := os.Getenv("LAYERX_PROOF_PYTHON"); configured != "" {
		python = configured
	}
	pythonArgs := []string{"tests/agent/sdk/python/test_evm_transaction.py", "--programs-proof", fixturePath, "produce"}
	pythonInput, err := hex.DecodeString(proofProgramsRun(t, root, python, pythonArgs...))
	require.NoError(t, err)
	require.Equal(t, input, pythonInput)
	// Transpile only modules loaded by this consumer, without a workspace build.
	loader := filepath.Join(dir, "typescript-loader.mjs")
	loaderSource := `import {createRequire} from 'node:module';
import {readFile} from 'node:fs/promises';
const require = createRequire(` + fmtJS(filepath.Join(root, "services/agent/sdk/typescript/package.json")) + `);
const ts = require('typescript');
export async function resolve(s,c,n) {
 try { return await n(s,c); } catch(e) {
  if ((s.startsWith('.') || s.startsWith('/')) && s.endsWith('.js')) return n(s.slice(0,-3)+'.ts',c);
  throw e;
 }
}
export async function load(u,c,n) {
 if (!u.endsWith('.ts')) return n(u,c);
 return {format:'module',shortCircuit:true,source:ts.transpileModule(await readFile(new URL(u),'utf8'),
  {compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.ESNext}}).outputText};
}`
	require.NoError(t, os.WriteFile(loader, []byte(loaderSource), 0600))
	tsArgs := []string{"--no-warnings", "--loader", loader, "services/agent/sdk/typescript/test/ai-market.test.ts", "--programs-proof", fixturePath, "produce"}
	tsInput, err := hex.DecodeString(proofProgramsRun(t, root, "node", tsArgs...))
	require.NoError(t, err)
	require.Equal(t, input, tsInput)
	proofProgramsReopen(t, h)
	for _, sdkInput := range [][]byte{pythonInput, tsInput} {
		simulated := simulateCall(t, h, args)
		output, expectedResponse := call(t, h, sdkInput, payload, args[8].(*big.Int))
		frames.ExpectedResponses = append(frames.ExpectedResponses, hex.EncodeToString(expectedResponse))
		require.Equal(t, simulated, output)
		frames.Calls = append(frames.Calls, hex.EncodeToString(output))
		frames.Simulations = append(frames.Simulations, hex.EncodeToString(simulated))
		head := getStateHead(t, h, args[0].([32]byte))
		frames.Heads = append(frames.Heads, hex.EncodeToString(head))
		values, err := lp.ProgramsABI.Methods["getStateHead"].Outputs.Unpack(head)
		require.NoError(t, err)
		frames.StateHead = hex.EncodeToString(values[0].([]byte))
		proofProgramsReopen(t, h)
	}
	// Refusals go through the same EVM entry point and preserve every producer key.
	for _, method := range []string{"getStateHead", "getProgram", "simulateCall", "call"} {
		var bad []byte
		caller := h.caller
		expectedCode := "LXP_NOT_FOUND"
		if method == "call" || method == "simulateCall" {
			caller = common.HexToAddress("0xb333")
			expectedCode = "LXP_ADDR_EVM_NOT_BOUND"
			bad, err = lp.ProgramsABI.Pack(method, args...)
		} else {
			bad, err = lp.ProgramsABI.Pack(method, [32]byte{0xff})
		}
		require.NoError(t, err)
		before := h.dump(h.stateDB.Ctx())
		var output []byte
		if method == "call" {
			output, _, err = h.evm.Call(caller, lp.Address, bad, 20000000, uint256.NewInt(0))
		} else {
			output, _, err = h.evm.StaticCall(caller, lp.Address, bad, 20000000)
		}
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		reason, decodeErr := abi.UnpackRevert(output)
		require.NoError(t, decodeErr)
		require.Contains(t, reason, expectedCode)
		frames.Refusals[expectedCode] = hex.EncodeToString(output)
		require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	}
	quoteCallFee(t, h, args[7].([7]uint64))
	for i := 0; i < 2; i++ {
		log, program := transfer(t)
		frames.Transfers = append(frames.Transfers, log)
		frames.TransferProgram = hex.EncodeToString(program[:])
	}
	retireStorage(t)
	original, err := hex.DecodeString(frames.Calls[0])
	require.NoError(t, err)
	for _, index := range []int{0, 32, 63, 95, 127} {
		malformed := bytes.Clone(original)
		malformed[index] ^= 0x80
		frames.MalformedCalls = append(frames.MalformedCalls, hex.EncodeToString(malformed))
	}
	frames.MalformedCalls = append(frames.MalformedCalls, hex.EncodeToString(append(bytes.Clone(original), make([]byte, 32)...)))
	head, err := hex.DecodeString(frames.Heads[0])
	require.NoError(t, err)
	for _, index := range []int{31, 63, len(head) - 1} {
		malformed := bytes.Clone(head)
		if index == 63 {
			malformed[index] ^= 0x80
		} else {
			malformed[index] ^= 1
		}
		frames.MalformedHeads = append(frames.MalformedHeads, hex.EncodeToString(malformed))
	}
	proofProgramsWrite(t, fixturePath, frames)
	pythonArgs[len(pythonArgs)-1] = "observe"
	require.Contains(t, proofProgramsRun(t, root, python, pythonArgs...), "python observed")
	tsArgs[len(tsArgs)-1] = "observe"
	require.Contains(t, proofProgramsRun(t, root, "node", tsArgs...), "typescript observed")
	proofProgramsEvaluator(t, root, dir, h, args[0].([32]byte), frames)
}

func fmtJS(value string) string { encoded, _ := json.Marshal(value); return string(encoded) }

func proofProgramsEvaluator(t *testing.T, root, dir string, h *harness, program [32]byte, frames proofProgramsFrames) {
	t.Helper()
	source, err := os.ReadFile(filepath.Join(root, "services/platform/hosted/ai-evaluator/src/main.rs"))
	require.NoError(t, err)
	text := string(source)
	begin := strings.Index(text, "pub mod programs_wire {")
	end := strings.Index(text, "// PROGRAMS_WIRE_END")
	require.GreaterOrEqual(t, begin, 0)
	require.Greater(t, end, begin)
	// Compile the production module verbatim, never another decoder implementation.
	driver := text[begin:end] + `
use std::io::{self, BufRead};
fn unhex(s:&str)->Vec<u8>{ (0..s.len()).step_by(2).map(|i|u8::from_str_radix(&s[i..i+2],16).unwrap()).collect() }
fn main(){
 let expected=unhex(&std::env::args().nth(1).unwrap());
 let mut counts=[0;6];
 let responses:Vec<Vec<u8>>=std::env::args().nth(2).unwrap().split(',').map(unhex).collect();
 for line in io::stdin().lock().lines(){let line=line.unwrap(); let data=unhex(&line[1..]);
 match &line[..1] {
 "C"=>{assert_eq!(programs_wire::decode_terminal(&data).unwrap(),(0,1,responses[counts[0]%2].clone()));counts[0]+=1;},
 "H"=>{assert_eq!(programs_wire::abi_bytes(&data,0).unwrap(),expected);counts[1]+=1;},
 "c"=>{assert!(programs_wire::decode_terminal(&data).is_err());counts[2]+=1;},
 "h"=>{assert!(programs_wire::abi_bytes(&data,0).is_err());counts[3]+=1;},
 "P"=>{programs_wire::require_active(&data).unwrap();counts[4]+=1;},
 "p"=>{assert!(programs_wire::require_active(&data).is_err());counts[5]+=1;},
 "S"=>assert_eq!(data,programs_wire::PROGRAM_SELECTOR),
 _=>panic!("unknown frame")}}
 assert_eq!(counts,[4,2,6,3,2,2]);
 println!("evaluator observed real program records, terminal outputs and storage heads");
}`
	path := filepath.Join(dir, "evaluator-consumer.rs")
	require.NoError(t, os.WriteFile(path, []byte(driver), 0600))
	binaryPath := filepath.Join(dir, "evaluator-consumer")
	proofProgramsRun(t, root, "rustc", "--edition=2021", path, "-o", binaryPath)
	var stream strings.Builder
	for _, values := range []struct {
		prefix string
		rows   []string
	}{
		{"C", frames.Calls}, {"C", frames.Simulations}, {"H", frames.Heads},
		{"c", frames.MalformedCalls}, {"h", frames.MalformedHeads},
	} {
		for _, row := range values.rows {
			stream.WriteString(values.prefix + row + "\n")
		}
	}
	input, err := lp.ProgramsABI.Pack("getProgram", program)
	require.NoError(t, err)
	stream.WriteString("S" + hex.EncodeToString(input[:4]) + "\n")
	for i := 0; i < 2; i++ {
		output, _, err := h.evm.StaticCall(h.caller, lp.Address, input, 2000000)
		require.NoError(t, err)
		stream.WriteString("P" + hex.EncodeToString(output) + "\n")
		malformed := bytes.Clone(output)
		malformed[len(malformed)-1] = 0
		stream.WriteString("p" + hex.EncodeToString(malformed) + "\n")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, frames.StateHead, strings.Join(frames.ExpectedResponses, ","))
	cmd.Stdin = strings.NewReader(stream.String())
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "evaluator consumer: %s", output)
	require.Contains(t, string(output), "evaluator observed")
}
