package layerx

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

type programRecordingTransport struct {
	next     http.RoundTripper
	requests []*http.Request
}

func (transport *programRecordingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.requests = append(transport.requests, request.Clone(request.Context()))
	return transport.next.RoundTrip(request)
}

func programRoutes() []string {
	var operations []string
	for _, operation := range AllAgentOperations() {
		if strings.HasPrefix(string(operation), "program.") {
			operations = append(operations, string(operation))
		}
	}
	return operations
}

func sendProgram(t *testing.T, programs *Programs, operation string) error {
	t.Helper()
	ctx := context.Background()
	program, transaction := [32]byte{1}, [32]byte{2}
	key, err := NewIdempotencyKey(strings.Repeat("02", 32))
	if err != nil {
		t.Fatal(err)
	}
	switch operation {
	case "program.discover":
		_, err = programs.Discover(ctx, program)
	case "program.interface":
		_, err = programs.Interface(ctx, program)
	case "program.simulate", "program.call":
		call := programEVMCall(t, readProgramEVMVector(t, "native-program-call-v4"))
		if operation == "program.simulate" {
			_, err = programs.Simulate(ctx, call)
		} else {
			_, err = programs.Submit(ctx, call, key)
		}
	case "program.receipt":
		_, err = programs.Receipt(ctx, key, transaction)
	case "program.activity":
		_, err = programs.Activity(ctx, transaction)
	case "program.deploy", "program.upgrade":
		name := strings.TrimPrefix(operation, "program.")
		_, payload, _, _ := lifecycleFixture(t, name)
		if operation == "program.deploy" {
			value, decodeErr := DecodeNativeProgramDeploy(payload)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			_, err = programs.Deploy(ctx, value, nil, key)
		} else {
			value, decodeErr := DecodeNativeProgramUpgrade(payload)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			_, err = programs.Upgrade(ctx, value, nil, key)
		}
	case "program.wind-down":
		_, payload, signed, fixture := lifecycleFixture(t, "wind-down-route")
		value, decodeErr := DecodeNativeProgramWindDown(payload)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		key, err = NewIdempotencyKey(hex.EncodeToString(lifecycleFixtureBytes(t, fixture, "idempotency_key_hex")))
		if err != nil {
			t.Fatal(err)
		}
		_, err = programs.WindDown(ctx, value, signed, key)
	case "lifecycle-receipt":
		_, err = programs.LifecycleReceipt(ctx, NativeProgramLifecycleRequest{TransactionHash: transaction})
	default:
		t.Fatalf("program operation has no retirement coverage: %s", operation)
	}
	return err
}

func TestProgramOperationsNeverCallProgramsRestIngress(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	operations := programRoutes()
	if len(operations) != 9 {
		t.Fatalf("expected all nine program operations, got %v", operations)
	}
	for _, useRPC := range []bool{false, true} {
		name := "human"
		if useRPC {
			name = "rpc"
		}
		t.Run(name, func(t *testing.T) {
			for _, operation := range append(append([]string{}, operations...), "lifecycle-receipt") {
				t.Run(operation, func(t *testing.T) {
					recording := &programRecordingTransport{next: server.Client().Transport}
					httpClient := &http.Client{Transport: recording}
					var transport Transport
					var err error
					if useRPC {
						transport, err = NewProgramRPCTransport(server.URL+"/", httpClient)
					} else {
						transport, err = NewHumanHTTPTransport(server.URL, httpClient, nil)
					}
					if err != nil {
						t.Fatal(err)
					}
					client, err := NewClient(transport, nil)
					if err != nil {
						t.Fatal(err)
					}
					programs, err := NewProgramsWithTrustOptions(client, [32]byte{1}, ProgramTrustOptions{
						Sender: [20]byte{1}, DIDID: [32]byte{1}, ProtocolVersion: 3,
						ClockMilliseconds: func() uint64 { return 1000 }, MaximumSimulationAgeMilliseconds: 300,
					})
					if err != nil {
						t.Fatal(err)
					}
					err = sendProgram(t, programs, operation)
					wantCode, wantRequests := ErrorUnavailableCapability, 0
					if useRPC {
						wantCode, wantRequests = ErrorTransportFailure, 1
					}
					if code := programErrorCode(err); code != wantCode {
						t.Fatalf("operation error = %v, want %s", err, wantCode)
					}
					for _, request := range recording.requests {
						if strings.HasPrefix(request.URL.Path, "/v1/programs") {
							t.Fatalf("program operation dispatched retired ingress: %s", request.URL.Path)
						}
						if request.URL.Path != "/" || request.Method != http.MethodPost {
							t.Fatalf("unexpected RPC request: %s %s", request.Method, request.URL.Path)
						}
					}
					if len(recording.requests) != wantRequests {
						t.Fatalf("HTTP requests = %d, want %d", len(recording.requests), wantRequests)
					}
				})
			}
		})
	}
	for _, operation := range operations {
		t.Run("retired/"+operation, func(t *testing.T) {
			recording := &programRecordingTransport{next: server.Client().Transport}
			transport, err := NewHumanHTTPTransport(server.URL, &http.Client{Transport: recording}, nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = transport.Call(context.Background(), TransportCall{
				Plane: PlaneAgent, Operation: operation, Request: json.RawMessage(`{}`),
			})
			if programErrorCode(err) != ErrorUnavailableCapability || len(recording.requests) != 0 {
				t.Fatalf("retired operation dispatched: error=%v requests=%d", err, len(recording.requests))
			}
		})
	}
}

func TestProgramWindDownPreparesBroadcastsAndObservesNativeActivity(t *testing.T) {
	data, err := os.ReadFile("testdata/repair141/native-deprecate.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		From, Calldata, RawTransaction, TransactionHash, R, S string
		YParity                                               uint64
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	signed := sequencerFixtureBytes(t, fixture.RawTransaction)
	expectedHash := receiptKeccak256(signed)
	var methods []string
	programs := repair141Programs(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" || r.Method != http.MethodPost || strings.HasPrefix(r.URL.Path, "/v1/programs") {
			t.Error("retired or unexpected ingress")
			http.Error(w, "route", 400)
			return
		}
		var request programRPCRecord
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		methods = append(methods, request.Method)
		var result any
		switch request.Method {
		case "eth_chainId":
			result = "0x7d"
		case "eth_getTransactionCount":
			result = "0x7"
		case "eth_gasPrice":
			result = "0x64"
		case "eth_maxPriorityFeePerGas":
			result = "0x2"
		case "eth_estimateGas":
			var tx ProgramTransaction
			if len(request.Params) != 2 || json.Unmarshal(request.Params[0], &tx) != nil || tx.Data != fixture.Calldata || tx.To != layerxProgramsAddress || tx.ChainID != "0x7d" || tx.From != fixture.From || string(request.Params[1]) != `"pending"` {
				t.Error("deprecate transaction not authorized at native precompile")
			}
			result = "0x493e0"
		case "eth_sendRawTransaction":
			if len(request.Params) != 1 || string(request.Params[0]) != fmt.Sprintf("%q", fixture.RawTransaction) {
				t.Error("externally signed bytes changed")
			}
			result = fixture.TransactionHash
		case "eth_getTransactionByHash", "pax_sequencerReceipt":
			if len(request.Params) != 1 || string(request.Params[0]) != fmt.Sprintf("%q", fixture.TransactionHash) {
				t.Error("activity identity changed")
			}
			if request.Method == "eth_getTransactionByHash" {
				result = map[string]string{"hash": fixture.TransactionHash}
			} else {
				result = map[string]string{"rung": "submitted"}
			}
		default:
			t.Errorf("unexpected native wind-down method %s", request.Method)
			http.Error(w, "method", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}), ReceiptKeys{})
	copy(programs.sender[:], sequencerFixtureBytes(t, fixture.From))
	value := NativeProgramWindDown{ProgramID: [32]byte{0x42}, Operation: 2, ExitProgram: [32]byte{0x42}, DeadlineBatch: 1000}
	prepared, err := programs.WindDown(context.Background(), value, nil, IdempotencyKey{})
	if err != nil || prepared.State != ProgramSubmissionPrepared || prepared.Transaction == nil {
		t.Fatalf("native deprecate preparation: %+v %v", prepared, err)
	}
	tx := prepared.Transaction
	if tx.Data != fixture.Calldata || !strings.HasPrefix(tx.Data, "0x86ba1a99") || tx.To != layerxProgramsAddress {
		t.Fatalf("wrong deprecate selector: %+v", tx)
	}
	// Bind all prepared signing fields to the externally signed EIP-1559 fixture.
	var envelope []byte
	for _, quantity := range []string{tx.ChainID, tx.Nonce, tx.MaxPriorityFeePerGas, tx.MaxFeePerGas, tx.Gas} {
		number, ok := receiptQuantity(quantity, 256)
		if !ok {
			t.Fatal("invalid prepared quantity")
		}
		envelope = append(envelope, committedRLP(number.Bytes(), false)...)
	}
	envelope = append(envelope, committedRLP(sequencerFixtureBytes(t, tx.To), false)...)
	if tx.Value != "0x0" || len(tx.AccessList) != 0 {
		t.Fatal("signed fixture value or access list mismatch")
	}
	envelope = append(envelope, committedRLP(nil, false)...)
	envelope = append(envelope, committedRLP(sequencerFixtureBytes(t, tx.Data), false)...)
	envelope = append(envelope, committedRLP(nil, true)...)
	for _, quantity := range []string{fmt.Sprintf("0x%x", fixture.YParity), fixture.R, fixture.S} {
		number, ok := receiptQuantity(quantity, 256)
		if !ok {
			t.Fatal("invalid fixture signature")
		}
		envelope = append(envelope, committedRLP(number.Bytes(), false)...)
	}
	if !bytes.Equal(signed, append([]byte{2}, committedRLP(envelope, true)...)) {
		t.Fatal("signed transaction differs from native preparation")
	}
	broadcast, err := programs.Broadcast(context.Background(), signed)
	if err != nil || broadcast.State != ProgramSubmissionPending || broadcast.TransactionHash != expectedHash || fixture.TransactionHash != portableHex(expectedHash[:]) {
		t.Fatalf("native deprecate broadcast: %+v %v", broadcast, err)
	}
	activity, err := programs.Activity(context.Background(), expectedHash)
	if err != nil || activity.State != ProgramSubmissionPending || activity.TransactionHash != expectedHash || activity.SequencerReceipt != nil {
		t.Fatalf("native deprecate activity: %+v %v", activity, err)
	}
	expected := []string{"eth_chainId", "eth_getTransactionCount", "eth_gasPrice", "eth_maxPriorityFeePerGas", "eth_estimateGas", "eth_sendRawTransaction", "eth_getTransactionByHash", "pax_sequencerReceipt"}
	if !reflect.DeepEqual(methods, expected) {
		t.Fatalf("native wind-down RPC path: %v", methods)
	}
}
