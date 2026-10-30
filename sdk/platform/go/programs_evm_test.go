package layerx

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type programEVMVector struct {
	PayloadHex string `json:"payload_hex"`
	FeeLimit   string `json:"fee_limit"`
	EVM        struct {
		To       string `json:"to"`
		ChainID  uint64 `json:"chainId"`
		Calldata string `json:"calldata"`
	} `json:"evm"`
}

func readProgramEVMVector(t *testing.T, name string) programEVMVector {
	t.Helper()
	raw, err := os.ReadFile("../conformance/fixtures/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var vector programEVMVector
	if err := json.Unmarshal(raw, &vector); err != nil {
		t.Fatal(err)
	}
	if vector.EVM.ChainID != 125 || vector.EVM.Calldata == "" {
		t.Fatal("missing EVM conformance vector")
	}
	return vector
}

func programEVMVectorBytes(t *testing.T, encoded string) []byte {
	t.Helper()
	raw, err := hex.DecodeString(strings.TrimPrefix(encoded, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

type programRPCRecord struct {
	Method  string            `json:"method"`
	Params  []json.RawMessage `json:"params"`
	JSONRPC string            `json:"jsonrpc"`
	ID      uint64            `json:"id"`
}

func programEVMEndpoint(t *testing.T, simulation string) (*Programs, func() []programRPCRecord) {
	t.Helper()
	var mu sync.Mutex
	records := []programRPCRecord{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/" {
			t.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected route", 400)
			return
		}
		var record programRPCRecord
		if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
			t.Error(err)
			http.Error(w, "bad JSON", 400)
			return
		}
		if record.JSONRPC != "2.0" || record.ID != 1 {
			t.Errorf("invalid JSON-RPC envelope: %+v", record)
		}
		mu.Lock()
		records = append(records, record)
		mu.Unlock()
		var result string
		switch record.Method {
		case "eth_getTransactionCount":
			result = "0x7"
		case "eth_gasPrice":
			result = "0x64"
		case "eth_maxPriorityFeePerGas":
			result = "0x2"
		case "eth_estimateGas":
			result = "0x493e0"
		case "eth_call":
			result = simulation
		default:
			t.Errorf("unexpected RPC method: %s", record.Method)
			http.Error(w, "unexpected RPC", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": record.ID, "result": result}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	transport, err := NewProgramRPCTransport(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	programs, err := NewProgramsWithTrustOptions(client, [32]byte{1}, ProgramTrustOptions{ProtocolVersion: 3, Sender: [20]byte{0x12}, ClockMilliseconds: func() uint64 { return 1000 }, MaximumSimulationAgeMilliseconds: 300000})
	if err != nil {
		t.Fatal(err)
	}
	return programs, func() []programRPCRecord {
		mu.Lock()
		defer mu.Unlock()
		return append([]programRPCRecord{}, records...)
	}
}

func programEVMCall(t *testing.T, vector programEVMVector) ProgramCall {
	t.Helper()
	native, err := DecodeNativeProgramCall(programEVMVectorBytes(t, vector.PayloadHex))
	if err != nil {
		t.Fatal(err)
	}
	fee, err := ParseUint128(vector.FeeLimit)
	if err != nil {
		t.Fatal(err)
	}
	call, err := (NativeProgramRequest{Call: native, FeeLimit: fee}).ProgramCall()
	if err != nil {
		t.Fatal(err)
	}
	return call
}

func assertProgramEVMTransaction(t *testing.T, submission ProgramSubmission, vector programEVMVector, records []programRPCRecord) {
	t.Helper()
	if submission.State != ProgramSubmissionPrepared || submission.Transaction == nil || submission.TransactionHash != ([32]byte{}) || submission.SequencerReceipt != nil {
		t.Fatalf("unsigned preparation reported a submission: %+v", submission)
	}
	tx := submission.Transaction
	if tx.Type != "0x2" || tx.ChainID != "0x7d" || tx.To != layerxProgramsAddress || tx.Data != vector.EVM.Calldata || tx.Nonce != "0x7" || tx.Gas != "0x493e0" || tx.Value != "0x0" || tx.MaxFeePerGas != "0x64" || tx.MaxPriorityFeePerGas != "0x2" || len(tx.AccessList) != 0 {
		t.Fatalf("unexpected EIP-1559 transaction: %+v", tx)
	}
	expected := []string{"eth_getTransactionCount", "eth_gasPrice", "eth_maxPriorityFeePerGas", "eth_estimateGas"}
	methods := make([]string, len(records))
	for i, record := range records {
		methods[i] = record.Method
	}
	if !reflect.DeepEqual(methods, expected) {
		t.Fatalf("RPC methods: got %v, want %v", methods, expected)
	}
	if len(records[0].Params) != 2 || string(records[0].Params[0]) != `"`+tx.From+`"` || string(records[0].Params[1]) != `"pending"` {
		t.Fatalf("nonce was not read for the sender at pending: %+v", records[0])
	}
	if len(records[3].Params) != 2 || string(records[3].Params[1]) != `"pending"` {
		t.Fatalf("gas was not estimated at pending: %+v", records[3])
	}
	var estimate map[string]json.RawMessage
	if err := json.Unmarshal(records[3].Params[0], &estimate); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"from": tx.From, "to": tx.To, "data": tx.Data, "nonce": tx.Nonce, "type": tx.Type, "chainId": tx.ChainID, "maxFeePerGas": tx.MaxFeePerGas, "maxPriorityFeePerGas": tx.MaxPriorityFeePerGas} {
		var actual string
		if json.Unmarshal(estimate[name], &actual) != nil || actual != value {
			t.Fatalf("estimated %s does not match transaction", name)
		}
	}
	encoded, err := json.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"r", "s", "v", "yParity"} {
		if _, exists := fields[name]; exists {
			t.Fatalf("unsigned transaction carries %s", name)
		}
	}
}

func TestProgramCallBuildsLayerxprogramsCallTransaction(t *testing.T) {
	vector := readProgramEVMVector(t, "native-program-call-v4")
	if vector.EVM.To != layerxProgramsAddress {
		t.Fatal("call vector destination changed")
	}
	programs, records := programEVMEndpoint(t, "")
	call := programEVMCall(t, vector)
	submission, err := programs.Submit(context.Background(), call, IdempotencyKey{})
	if err != nil {
		t.Fatal(err)
	}
	assertProgramEVMTransaction(t, submission, vector, records())
	nativeTx, err := (NativeProgramRequest{Call: *call.NativeCall, FeeLimit: call.Budget.FeeLimit}).Transaction(context.Background(), programs)
	if err != nil || !reflect.DeepEqual(nativeTx, *submission.Transaction) {
		t.Fatalf("native request transaction differs: %+v %v", nativeTx, err)
	}
}

func TestProgramSimulateUsesPendingEthCall(t *testing.T) {
	vector := readProgramEVMVector(t, "native-program-call-v4")
	response := []byte{0, 0x61, 0xff, 0x10}
	result := make([]byte, 128+32)
	result[63] = 1
	result[95] = 96
	result[127] = byte(len(response))
	copy(result[128:], response)
	programs, records := programEVMEndpoint(t, "0x"+hex.EncodeToString(result))
	simulation, err := programs.Simulate(context.Background(), programEVMCall(t, vector))
	if err != nil {
		t.Fatal(err)
	}
	if simulation.Committed || simulation.ResultCode != 0 || simulation.TerminalKind != 1 || !bytes.Equal(simulation.Response, response) {
		t.Fatalf("simulation result changed: %+v", simulation)
	}
	requests := records()
	if len(requests) != 1 || requests[0].Method != "eth_call" || len(requests[0].Params) != 2 || string(requests[0].Params[1]) != `"pending"` {
		t.Fatalf("simulation did not use pending eth_call: %+v", requests)
	}
	var call map[string]string
	if err := json.Unmarshal(requests[0].Params[0], &call); err != nil {
		t.Fatal(err)
	}
	expected := "0x3044bd56" + vector.EVM.Calldata[10:]
	if call["to"] != layerxProgramsAddress || call["data"] != expected || call["from"] != "0x1200000000000000000000000000000000000000" {
		t.Fatalf("simulation call differs from vector: %+v", call)
	}
}

func TestProgramDiscoverReadsWindDownLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name      string
		active    bool
		status    uint64
		length    int
		lifecycle string
	}{
		{"active", true, 1, 192, "active"},
		{"deprecated", false, 2, 192, "deprecated"},
		{"tombstoned", false, 3, 192, "tombstoned"},
		{"inactive-status-one", false, 1, 192, ""},
		{"status-zero", false, 0, 192, ""},
		{"unknown-status", false, 4, 192, ""},
		{"status-overflow", false, 258, 192, ""},
		{"empty", false, 2, 0, ""},
		{"short-status", false, 2, 31, ""},
		{"status-only", false, 2, 32, ""},
		{"short-record", false, 3, 191, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program := [32]byte{0x42}
			head := make([]byte, 192)
			head[63], head[95], head[127], head[159] = 0x11, 0x22, 1, 7
			if tc.active {
				head[191] = 1
			}
			windDown := programABI("", programABIWord(tc.status), bytes.Repeat([]byte{0x33}, 32),
				programABIWord(999), programABIWord(888), programABIWord(7), programABIWord(5))[:tc.length]
			var mu sync.Mutex
			var requests []programRPCRecord
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request programRPCRecord
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					http.Error(w, "bad request", 400)
					return
				}
				mu.Lock()
				requests = append(requests, request)
				mu.Unlock()
				var tx map[string]string
				if len(request.Params) != 2 || json.Unmarshal(request.Params[0], &tx) != nil {
					t.Error("invalid eth_call parameters")
					http.Error(w, "bad parameters", 400)
					return
				}
				var result []byte
				switch tx["data"] {
				case "0xb1ffb3d4" + hex.EncodeToString(program[:]):
					result = head
				case "0x98e4e41d" + hex.EncodeToString(program[:]):
					result = windDown
				default:
					t.Errorf("unexpected calldata: %s", tx["data"])
					http.Error(w, "bad calldata", 400)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": "0x" + hex.EncodeToString(result)}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			transport, err := NewProgramRPCTransport(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			client, err := NewClient(transport, nil)
			if err != nil {
				t.Fatal(err)
			}
			programs, err := NewProgramsWithTrustOptions(client, [32]byte{1}, ProgramTrustOptions{
				ClockMilliseconds: func() uint64 { return 1000 }, MaximumSimulationAgeMilliseconds: 300000,
			})
			if err != nil {
				t.Fatal(err)
			}
			result, err := programs.Discover(context.Background(), program)
			if tc.lifecycle == "" {
				var sdkError *SDKError
				if !errors.As(err, &sdkError) || sdkError.Code != ErrorDecodeFailure {
					t.Fatalf("expected decode failure, got %+v, %v", result, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if result.Lifecycle != tc.lifecycle || result.ProgramID != hex.EncodeToString(program[:]) || result.Version != 7 || result.ABIVersion != 1 || result.CodeHash != hex.EncodeToString(head[64:96]) {
					t.Fatalf("unexpected discovery: %+v", result)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			expectedCalls := 2
			if tc.active {
				expectedCalls = 1
			}
			if len(requests) != expectedCalls {
				t.Fatalf("got %d calls, want %d", len(requests), expectedCalls)
			}
			for i, request := range requests {
				if request.Method != "eth_call" || request.JSONRPC != "2.0" || string(request.Params[1]) != `"pending"` {
					t.Fatalf("unexpected RPC: %+v", request)
				}
				var tx map[string]string
				if err := json.Unmarshal(request.Params[0], &tx); err != nil {
					t.Fatal(err)
				}
				selector := "b1ffb3d4"
				if i == 1 {
					selector = "98e4e41d"
				}
				if tx["to"] != layerxProgramsAddress || tx["data"] != "0x"+selector+hex.EncodeToString(program[:]) {
					t.Fatalf("unexpected call: %+v", tx)
				}
			}
		})
	}
}

func TestProgramDeployBuildsLayerxprogramsDeployTransaction(t *testing.T) {
	vector := readProgramEVMVector(t, "native-program-deploy-v4")
	if !strings.EqualFold(vector.EVM.To, "0x101d") {
		t.Fatal("deploy vector destination changed")
	}
	programs, records := programEVMEndpoint(t, "")
	value, err := DecodeNativeProgramDeploy(programEVMVectorBytes(t, vector.PayloadHex))
	if err != nil {
		t.Fatal(err)
	}
	submission, err := programs.Deploy(context.Background(), value, nil, IdempotencyKey{})
	if err != nil {
		t.Fatal(err)
	}
	assertProgramEVMTransaction(t, submission, vector, records())
	request, err := NewNativeProgramLifecycleRequest(1, programEVMVectorBytes(t, vector.PayloadHex), nil)
	if err != nil {
		t.Fatal(err)
	}
	nativeTx, err := request.Transaction(context.Background(), programs)
	if err != nil || !reflect.DeepEqual(nativeTx, *submission.Transaction) {
		t.Fatalf("lifecycle request transaction differs: %+v %v", nativeTx, err)
	}
}
