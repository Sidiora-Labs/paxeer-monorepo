package layerx

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
						if operation == "program.wind-down" {
							wantCode, wantRequests = ErrorInvalidArgument, 0
						}
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
