package layerx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRepairM_1384_rewritten_committed_fixture(t *testing.T) {
	TestCommittedInclusionBindsTheWrappedTransaction(t)
}

func TestRepairM_1384_occupancy_public_contract(t *testing.T) {
	TestNativeProgramReceiptRefusesOccupancyPayerClaims(t)
	// Preserve the internal codec checks as distinct historical coverage.
	t.Run("historical-codec", TestStateCommitmentOccupancyPaysFromTheProvenPayerAccount)
}

func TestRepairM_1384_portable_external_anchor(t *testing.T) {
	t.Run("recorded-checkpoint-and-replacement-chain", TestPortableReceiptRequiresExternalNetworkAnchor)
	t.Run("validator-rotation", TestPortableReceiptAuthenticatesValidatorRotation)
}

func TestRepairM_1384_wind_down_native_activity(t *testing.T) {
	TestProgramWindDownPreparesBroadcastsAndObservesNativeActivity(t)
	t.Run("retired-ingress-refusal", TestProgramOperationsNeverCallProgramsRestIngress)
}

func TestRepairM_1384_http_deadlines(t *testing.T) {
	// All timeout cases use real sockets and a peer which accepts the request but
	// never produces a response. No replacement RoundTripper or clock is used.
	for _, entry := range []string{"human", "agent"} {
		for _, budget := range []string{"default", "caller", "explicit", "explicit-zero"} {
			entry, budget := entry, budget
			t.Run(entry+"/"+budget, func(t *testing.T) {
				t.Parallel()
				release := make(chan struct{})
				received := make(chan struct{}, 1)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.Copy(io.Discard, r.Body)
					received <- struct{}{}
					select {
					case <-r.Context().Done():
					case <-release:
					}
				}))
				defer server.Close()
				defer close(release)
				var client *http.Client
				if budget == "explicit" {
					client = &http.Client{Timeout: 100 * time.Millisecond}
				}
				if budget == "explicit-zero" {
					client = &http.Client{}
				}
				var transport Transport
				var timeout time.Duration
				if entry == "human" {
					human, err := NewHumanHTTPTransport(server.URL, client, nil)
					if err != nil {
						t.Fatal(err)
					}
					timeout, transport = human.client.Timeout, human
				} else {
					token, err := NewSecretBytes([]byte(strings.Repeat("ab", 32)))
					if err != nil {
						t.Fatal(err)
					}
					defer token.Destroy()
					credential, err := NewAgentSessionCredential("tenant-a", strings.Repeat("cd", 32), token, 7)
					if err != nil {
						t.Fatal(err)
					}
					authorizer := func(r *http.Request) error {
						r.Header.Set("Authorization", "LayerX-Key key:lxp_live_"+strings.Repeat("ab", 32))
						return nil
					}
					agent, err := NewAgentEnvelopeHTTPTransport(server.URL, client, authorizer, credential)
					if err != nil {
						t.Fatal(err)
					}
					timeout, transport = agent.endpoint.client.Timeout, agent
				}
				want := DefaultHTTPRequestTimeout
				if client != nil {
					want = client.Timeout
				}
				if timeout != want || (client != nil && client.Timeout != want) || DefaultHTTPRequestTimeout != 30*time.Second {
					t.Fatalf("HTTP client timeout = %s, want %s", timeout, want)
				}
				ctx := context.Background()
				var cancel context.CancelFunc
				if budget == "caller" || budget == "explicit-zero" {
					ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
					defer cancel()
				}
				call := TransportCall{Plane: PlaneHuman, Operation: string(HumanOperationProfileGet)}
				if entry == "agent" {
					call = TransportCall{Plane: PlaneAgent, Operation: string(AgentOperationTenantReadiness), Request: json.RawMessage(`{}`)}
				}
				started := time.Now()
				response, err := transport.Call(ctx, call)
				elapsed := time.Since(started)
				select {
				case <-received:
				default:
					t.Fatal("request did not reach stalled peer")
				}
				if err == nil || response != nil {
					t.Fatalf("stalled response succeeded: %s %v", response, err)
				}
				if budget == "default" {
					if elapsed < DefaultHTTPRequestTimeout-500*time.Millisecond || elapsed > DefaultHTTPRequestTimeout+5*time.Second || ctx.Err() != nil {
						t.Fatalf("default request budget not enforced: %s %v", elapsed, ctx.Err())
					}
				} else if elapsed > 3*time.Second {
					t.Fatalf("shorter caller/client budget ignored: %s", elapsed)
				}
				if budget == "caller" || budget == "explicit-zero" {
					if !errors.Is(ctx.Err(), context.DeadlineExceeded) || programErrorCode(err) != ErrorDeadline {
						t.Fatalf("caller deadline lost: %v %v", ctx.Err(), err)
					}
				}
			})
		}
	}
}

func repairM1384Wasm(t *testing.T, size int) []byte {
	t.Helper()
	// A valid empty module padded with one custom section with an empty name.
	for lengthBytes := 1; lengthBytes <= 5; lengthBytes++ {
		sectionSize := size - 9 - lengthBytes
		if sectionSize < 1 {
			continue
		}
		var length []byte
		remaining := uint64(sectionSize)
		for {
			b := byte(remaining & 127)
			remaining >>= 7
			if remaining != 0 {
				b |= 128
			}
			length = append(length, b)
			if remaining == 0 {
				break
			}
		}
		if len(length) != lengthBytes {
			continue
		}
		wasm := append([]byte{0, 97, 115, 109, 1, 0, 0, 0, 0}, length...)
		wasm = append(wasm, make([]byte, sectionSize)...)
		if len(wasm) != size {
			t.Fatal("invalid Wasm fixture length")
		}
		return wasm
	}
	t.Fatal("cannot encode boundary Wasm")
	return nil
}

func TestRepairM_1384_deployment_payload_boundary(t *testing.T) {
	if MaximumNativeProgramPayloadBytes != 524288 {
		t.Fatal("native payload ceiling drift")
	}
	for _, tc := range []struct {
		name        string
		upgrade     bool
		iface, hook int
		clear       bool
	}{
		{name: "deploy-unframed"}, {name: "deploy-interface", iface: 1}, {name: "deploy-max-interface", iface: 952},
		{name: "upgrade-unframed", upgrade: true}, {name: "upgrade-hook", upgrade: true, hook: 65535},
		{name: "upgrade-interface-and-hook", upgrade: true, iface: 952, hook: 65535}, {name: "upgrade-clear-interface", upgrade: true, clear: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixed := 104
			if tc.upgrade {
				fixed = 106
			}
			var iface []byte
			if tc.iface > 0 || tc.clear {
				iface = make([]byte, tc.iface)
				fixed += 4
			}
			wasm := repairM1384Wasm(t, MaximumNativeProgramPayloadBytes-fixed-tc.iface-tc.hook)
			hash := sha256.Sum256(wasm)
			program := [32]byte{1}
			var payload, calldata []byte
			var err error
			ordinal := uint16(1)
			if tc.upgrade {
				ordinal = 2
				value := NativeProgramUpgrade{ProgramID: program, GuestABI: 4, NewHash: hash, OldHash: [32]byte{2}, Interface: iface, MigrationHook: make([]byte, tc.hook), ClearInterface: tc.clear, Wasm: wasm}
				payload, err = EncodeNativeProgramUpgrade(value)
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := DecodeNativeProgramUpgrade(payload)
				if err != nil || !reflect.DeepEqual(decoded.Wasm, wasm) || !bytes.Equal(decoded.Interface, iface) || len(decoded.MigrationHook) != tc.hook || decoded.ClearInterface != tc.clear {
					t.Fatalf("upgrade boundary decode: %+v %v", decoded, err)
				}
				value.Wasm = append(append([]byte(nil), wasm...), 0)
				value.NewHash = sha256.Sum256(value.Wasm)
				if result, err := EncodeNativeProgramUpgrade(value); err == nil || result != nil {
					t.Fatal("first oversized upgrade byte accepted")
				}
			} else {
				value := NativeProgramDeploy{ProgramID: program, GuestABI: 4, NewHash: hash, Interface: iface, Wasm: wasm}
				payload, err = EncodeNativeProgramDeploy(value)
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := DecodeNativeProgramDeploy(payload)
				if err != nil || !bytes.Equal(decoded.Wasm, wasm) || !bytes.Equal(decoded.Interface, iface) {
					t.Fatalf("deploy boundary decode: %v", err)
				}
				value.Wasm = append(append([]byte(nil), wasm...), 0)
				value.NewHash = sha256.Sum256(value.Wasm)
				if result, err := EncodeNativeProgramDeploy(value); err == nil || result != nil {
					t.Fatal("first oversized deployment byte accepted")
				}
			}
			if len(payload) != 524288 {
				t.Fatalf("accepted boundary = %d", len(payload))
			}
			request, err := NewNativeProgramLifecycleRequest(ordinal, payload, nil)
			if err != nil {
				t.Fatal(err)
			}
			calldata, err = request.EVMCalldata()
			if err != nil {
				t.Fatal(err)
			}
			// Decode the exact ABI dynamic Wasm argument sent to 0x101D with the
			// production ABI decoder; the envelope ceiling excludes EVM ABI wrapping.
			slot := 6
			selector := "bde3ff66"
			if tc.upgrade {
				slot = 7
				selector = "a860929f"
			}
			if hex.EncodeToString(calldata[:4]) != selector {
				t.Fatal("wrong lifecycle ABI selector")
			}
			start, err := programABIUint(calldata[4:], slot*32, 64)
			if err != nil {
				t.Fatal(err)
			}
			decodedWasm, err := programABIBytes(calldata[4:], slot*32, int(start))
			if err != nil || !bytes.Equal(decodedWasm, wasm) {
				t.Fatalf("precompile ABI Wasm argument: %v", err)
			}
			// An otherwise well-framed first oversized byte is refused on decode too.
			oversized := append(append([]byte(nil), payload...), 0)
			lengthOffset := 100
			if tc.upgrade {
				lengthOffset = 102
			}
			binary.BigEndian.PutUint32(oversized[lengthOffset:], uint32(len(wasm)+1))
			oversizedHash := sha256.Sum256(append(append([]byte(nil), wasm...), 0))
			copy(oversized[68:100], oversizedHash[:])
			if _, err := lifecycleReencode(ordinal, oversized); err == nil {
				t.Fatal("oversized framed payload decoded")
			}
			if _, err := NewNativeProgramLifecycleRequest(ordinal, oversized, nil); err == nil {
				t.Fatal("oversized native request admitted")
			}
		})
	}
	t.Run("producer-codec-vectors", TestProgramDeployBuildsLayerxprogramsDeployTransaction)
	t.Run("native-lifecycle-codec-vectors", TestNativeLifecycleCFixtures)
}

func TestRepairM_1384_program_authenticated_facts(t *testing.T) {
	execution := nativeProgramReceiptDocument(t)
	programs := committedRecordedPrograms(t, []byte(execution.Receipt), execution.RawTransaction, execution.ReceiptKeys)
	hash := receiptKeccak256(execution.RawTransaction)
	activity, err := programs.Activity(context.Background(), hash)
	if err != nil || activity.SequencerReceipt == nil || len(activity.SequencerReceipt.Logs) != 1 {
		t.Fatalf("recorded program receipt through real transport: %+v %v", activity, err)
	}
	verified, err := VerifyProgramReceipt(execution)
	if err != nil || verified.NativeResult == nil || verified.NativeResult.ProgramID != fixture32(t, execution.ProgramID) || verified.NativeResult.GuestABIVersion != execution.GuestABIVersion || verified.NativeResult.TerminalResult != 0 || verified.NativeResult.ActivityID == hash || !reflect.DeepEqual(verified.SequencerReceipt.Logs, activity.SequencerReceipt.Logs) {
		t.Fatalf("authenticated native log facts: %+v %v", verified, err)
	}
	for name, mutate := range map[string]func(*ProgramExecutionDocument){
		"program": func(e *ProgramExecutionDocument) { e.ProgramID = strings.Repeat("ff", 32) },
		"abi":     func(e *ProgramExecutionDocument) { e.GuestABIVersion = 4 },
		"result":  func(e *ProgramExecutionDocument) { e.ResultCode = -1 },
		"outcome": func(e *ProgramExecutionDocument) {
			code := int32(7)
			e.Outcome = ProgramOutcome{Kind: "completed", Code: &code, Response: []byte("substituted")}
		},
		"terminal":  func(e *ProgramExecutionDocument) { e.TerminalPayload = "00" },
		"graph":     func(e *ProgramExecutionDocument) { e.CallGraph = "00" },
		"usage":     func(e *ProgramExecutionDocument) { e.Usage.CPUFuel = "100" },
		"authority": func(e *ProgramExecutionDocument) { e.Authority.Asset = strings.Repeat("ff", 32) },
		"module":    func(e *ProgramExecutionDocument) { e.ModuleVersion = 4 },
		"activity":  func(e *ProgramExecutionDocument) { e.ActivityID = strings.Repeat("ff", 32) },
		"digest":    func(e *ProgramExecutionDocument) { e.ReceiptDigest = strings.Repeat("ff", 32) },
		"level":     func(e *ProgramExecutionDocument) { e.Verification = "committed" },
		"state":     func(e *ProgramExecutionDocument) { e.State = "refused" },
		"log-program": func(e *ProgramExecutionDocument) {
			var v sequencerReceiptJSON
			_ = json.Unmarshal([]byte(e.Receipt), &v)
			v.Logs[0].Topics[1] = "0x" + strings.Repeat("ff", 32)
			data, _ := json.Marshal(v)
			e.Receipt = string(data)
		},
		"log-result": func(e *ProgramExecutionDocument) {
			var v sequencerReceiptJSON
			_ = json.Unmarshal([]byte(e.Receipt), &v)
			body := sequencerFixtureBytes(t, v.Logs[0].Data)
			body[9*32-1] = 7
			v.Logs[0].Data = portableHex(body)
			data, _ := json.Marshal(v)
			e.Receipt = string(data)
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := execution
			mutate(&changed)
			got, err := VerifyProgramReceipt(changed)
			if err == nil || got.NativeResult != nil || got.SequencerReceipt != nil || got.Verification.Level != "" {
				t.Fatalf("substituted program claim accepted: %+v %v", got, err)
			}
		})
	}
	// A valid ordinary EVM receipt proves no program identity or guest outcome.
	c := sequencerReceiptCases(t)[0]
	var view sequencerReceiptJSON
	if err := json.Unmarshal(c.ReceiptJSON, &view); err != nil {
		t.Fatal(err)
	}
	ordinary := ProgramExecutionDocument{TransactionHash: view.TxHash, Receipt: string(c.ReceiptJSON), RawTransaction: sequencerFixtureBytes(t, c.RawTransaction), ReceiptKeys: sequencerFixtureKeys(t, c)}
	got, err := VerifyProgramReceipt(ordinary)
	if err != nil || got.NativeResult != nil || got.SequencerReceipt == nil {
		t.Fatalf("ordinary receipt contract: %+v %v", got, err)
	}
	ordinary.ProgramID = execution.ProgramID
	if got, err := VerifyProgramReceipt(ordinary); err == nil || got.NativeResult != nil || got.Verification.Level != "" {
		t.Fatal("ordinary receipt authenticated an invented program")
	}
}

func TestRepairM_1384_endpoint_chain_domain(t *testing.T) {
	data, err := os.ReadFile("testdata/repair_m1384/domain-rpc.json")
	if err != nil {
		t.Fatal(err)
	}
	var transcript struct {
		Cases []struct {
			Path    string `json:"path"`
			ChainID string `json:"chain_id"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &transcript); err != nil {
		t.Fatal(err)
	}
	if len(transcript.Cases) != 2 {
		t.Fatal("root and tenant domain producer transcripts required")
	}
	for _, tc := range transcript.Cases {
		t.Run(tc.Path, func(t *testing.T) {
			var methods []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != tc.Path {
					t.Errorf("RPC escaped selected endpoint: %s %s", r.Method, r.URL.Path)
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
					if len(request.Params) != 0 {
						t.Error("domain identity request parameters")
					}
					result = tc.ChainID
				case "eth_getTransactionCount":
					result = "0x7"
				case "eth_gasPrice":
					result = "0x64"
				case "eth_maxPriorityFeePerGas":
					result = "0x2"
				case "eth_estimateGas":
					var tx ProgramTransaction
					if len(request.Params) != 2 || json.Unmarshal(request.Params[0], &tx) != nil || tx.ChainID != tc.ChainID || tx.To != layerxProgramsAddress || string(request.Params[1]) != `"pending"` {
						t.Error("estimate signing domain differs from endpoint")
					}
					result = "0x493e0"
				default:
					t.Errorf("unexpected domain RPC %s", request.Method)
					http.Error(w, "method", 400)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
			}))
			defer server.Close()
			transport, err := NewProgramRPCTransport(server.URL+tc.Path, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			client, err := NewClient(transport, nil)
			if err != nil {
				t.Fatal(err)
			}
			programs, err := NewProgramsWithTrustOptions(client, [32]byte{1}, ProgramTrustOptions{Sender: [20]byte{0x12}, ClockMilliseconds: func() uint64 { return 1000 }, MaximumSimulationAgeMilliseconds: 300000})
			if err != nil {
				t.Fatal(err)
			}
			call := programEVMCall(t, readProgramEVMVector(t, "native-program-call-v4"))
			prepared, err := programs.Submit(context.Background(), call, IdempotencyKey{})
			if err != nil || prepared.Transaction == nil || prepared.Transaction.ChainID != tc.ChainID {
				t.Fatalf("endpoint signing domain: %+v %v", prepared, err)
			}
			if tc.Path == "/" && prepared.Transaction.ChainID != "0x7d" || tc.Path == "/chain/1" && prepared.Transaction.ChainID != "0x7735941" {
				t.Fatal("root and tenant signing domains collapsed")
			}
			// Exercise both native lifecycle entry points using the same endpoint.
			_, payload, _, _ := lifecycleFixture(t, "deploy")
			deploy, err := DecodeNativeProgramDeploy(payload)
			if err != nil {
				t.Fatal(err)
			}
			lifecycle, err := programs.Deploy(context.Background(), deploy, nil, IdempotencyKey{})
			if err != nil || lifecycle.Transaction == nil || lifecycle.Transaction.ChainID != tc.ChainID {
				t.Fatalf("deployment endpoint domain: %+v %v", lifecycle, err)
			}
			_, payload, _, _ = lifecycleFixture(t, "upgrade")
			upgrade, err := DecodeNativeProgramUpgrade(payload)
			if err != nil {
				t.Fatal(err)
			}
			lifecycle, err = programs.Upgrade(context.Background(), upgrade, nil, IdempotencyKey{})
			if err != nil || lifecycle.Transaction == nil || lifecycle.Transaction.ChainID != tc.ChainID {
				t.Fatalf("upgrade endpoint domain: %+v %v", lifecycle, err)
			}
			want := []string{"eth_chainId", "eth_getTransactionCount", "eth_gasPrice", "eth_maxPriorityFeePerGas", "eth_estimateGas"}
			if len(methods) != 15 {
				t.Fatalf("preparation did not resolve chain for every transaction: %v", methods)
			}
			for n := 0; n < 3; n++ {
				if !reflect.DeepEqual(methods[n*5:(n+1)*5], want) {
					t.Fatalf("domain preparation path: %v", methods)
				}
			}
		})
	}
	for _, chain := range []string{`null`, `125`, `""`, `"0x0"`, `"0x00"`, `"0X7d"`, `"0x7D"`, `"0x-1"`, fmt.Sprintf("%q", "0x1"+strings.Repeat("0", 64))} {
		t.Run("refuse-"+string(chain), func(t *testing.T) {
			count := 0
			programs := repair141Programs(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count++
				var request programRPCRecord
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				if request.Method != "eth_chainId" {
					t.Error("continued preparation after invalid chain identity")
				}
				_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":%s}`, request.ID, chain)
			}), ReceiptKeys{})
			call := programEVMCall(t, readProgramEVMVector(t, "native-program-call-v4"))
			prepared, err := programs.Submit(context.Background(), call, IdempotencyKey{})
			if err == nil || prepared.Transaction != nil || count != 1 {
				t.Fatalf("invalid chain identity fell back to root: %+v %v requests=%d", prepared, err, count)
			}
		})
	}
	t.Run("chain-rpc-refusal", func(t *testing.T) {
		programs := repair141Programs(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`)
		}), ReceiptKeys{})
		call := programEVMCall(t, readProgramEVMVector(t, "native-program-call-v4"))
		prepared, err := programs.Submit(context.Background(), call, IdempotencyKey{})
		var rpcError *ProgramRPCError
		if !errors.As(err, &rpcError) || rpcError.Code != -32601 || prepared.Transaction != nil {
			t.Fatalf("missing chain RPC silently fell back: %+v %v", prepared, err)
		}
	})
	t.Run("endpoint-redirect", func(t *testing.T) {
		followed := false
		other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			followed = true
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":"0x7735941"}`)
		}))
		defer other.Close()
		programs := repair141Programs(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, other.URL+"/chain/1", http.StatusTemporaryRedirect)
		}), ReceiptKeys{})
		call := programEVMCall(t, readProgramEVMVector(t, "native-program-call-v4"))
		prepared, err := programs.Submit(context.Background(), call, IdempotencyKey{})
		if err == nil || prepared.Transaction != nil || followed {
			t.Fatalf("preparation escaped endpoint through redirect: %+v %v followed=%v", prepared, err, followed)
		}
	})
}
