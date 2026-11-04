package dotnet

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	proof "github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
)

// The Go gate executes the production ABI, EIP-1559 signer and receipt codec
// against the same peer evidence consumed by the C# tests. C# member checks
// below establish test wiring, not execution of the .NET runtime.
type repairM1369Vector struct {
	ID             string          `json:"id"`
	RawTransaction string          `json:"rawTransaction"`
	ReceiptJSON    json.RawMessage `json:"receiptJson"`
	SignedReceipt  string          `json:"signedReceipt"`
	ReceiptDigest  string          `json:"receiptDigest"`
	PinnedKey      struct {
		KeyID       string `json:"keyId"`
		PublicKey   string `json:"publicKey"`
		FirstHeight uint64 `json:"firstHeight"`
		LastHeight  uint64 `json:"lastHeight"`
	} `json:"pinnedKey"`
}

type repairM1369Call struct {
	Source           string    `json:"source"`
	TransactionHash  string    `json:"transaction_hash"`
	ProgramID        string    `json:"program_id_hex"`
	GuestABI         uint16    `json:"guest_abi"`
	Entrypoint       string    `json:"entrypoint"`
	Calldata         string    `json:"calldata_hex"`
	Capabilities     string    `json:"capabilities_hex"`
	Access           string    `json:"access_declaration_hex"`
	ResponseCapacity uint32    `json:"response_capacity"`
	Resources        [7]uint64 `json:"resources"`
	FeeLimit         string    `json:"fee_limit"`
	EVMCalldata      string    `json:"evm_calldata_hex"`
}

type repairM1369Discovery struct {
	Lifecycle         string `json:"lifecycle"`
	Policy            uint8  `json:"policy"`
	Authority         string `json:"authority_hex"`
	CodeHash          string `json:"code_hash_hex"`
	ABI               uint16 `json:"abi_version"`
	Version           uint32 `json:"version"`
	Active            bool   `json:"active"`
	Status            uint8  `json:"status"`
	ExitProgram       string `json:"exit_program_hex"`
	Deadline          uint64 `json:"deadline"`
	EffectiveSequence uint64 `json:"effective_sequence"`
	AccountCount      uint16 `json:"account_count"`
	LiveCount         uint16 `json:"live_count"`
	Head              string `json:"get_program_output_hex"`
	WindDown          string `json:"get_wind_down_output_hex"`
}

func repairM1369Decode(t *testing.T, path string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(repairM1368Read(t, path)), target); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func repairM1369Bytes(t *testing.T, value string) []byte {
	t.Helper()
	raw, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func repairM1369Hash(t *testing.T, value string) [32]byte {
	t.Helper()
	raw := repairM1369Bytes(t, value)
	if len(raw) != 32 {
		t.Fatalf("expected bytes32, got %d bytes", len(raw))
	}
	return [32]byte(raw)
}

func repairM1369ABI(t *testing.T) abi.ABI {
	t.Helper()
	producer, err := abi.JSON(strings.NewReader(repairM1368Read(t, "../../../precompiles/layerxprograms/abi.json")))
	if err != nil {
		t.Fatal(err)
	}
	return producer
}

func repairM1369Vectors(t *testing.T) []repairM1369Vector {
	t.Helper()
	var inventory struct {
		Cases []repairM1369Vector `json:"cases"`
	}
	repairM1369Decode(t, "../conformance/fixtures/vector-v1-receipted-positive.json", &inventory)
	return inventory.Cases
}

func repairM1369ProgramVector(t *testing.T) repairM1369Vector {
	t.Helper()
	var matches []repairM1369Vector
	for _, vector := range repairM1369Vectors(t) {
		if vector.ID == "V1-layerxprograms-call" {
			matches = append(matches, vector)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("expected one genuine Programs producer case, got %d", len(matches))
	}
	return matches[0]
}

func repairM1369Member(t *testing.T, file, declaration string, fragments ...string) string {
	t.Helper()
	path := "tests/LayerX.Sdk.Tests/" + file
	source := repairM1368Read(t, path)
	if strings.Contains(source, "Skip =") || strings.Contains(source, "Skip=") {
		t.Fatalf("disabled C# test in %s", file)
	}
	body := repairM1368Member(t, path, declaration)
	repairM1368Contains(t, body, fragments...)
	return body
}

func TestRepairM_1369_programs_contract_tests_stale(t *testing.T) {
	producer := repairM1369ABI(t)
	vector := repairM1369ProgramVector(t)
	var facts repairM1369Call
	repairM1369Decode(t, "tests/LayerX.Sdk.Tests/Fixtures/repair-m1369-recorded-call.json", &facts)
	if facts.Source != "daemon/committed-fixture/vectors/vector-v1-receipted-positive.json#V1-layerxprograms-call" {
		t.Fatal("native call must retain its real producer provenance")
	}
	var recorded types.Transaction
	if err := recorded.UnmarshalBinary(repairM1369Bytes(t, vector.RawTransaction)); err != nil {
		t.Fatal(err)
	}
	if recorded.Hash() != common.BytesToHash(repairM1369Bytes(t, facts.TransactionHash)) || recorded.ChainId().Cmp(big.NewInt(125)) != 0 || recorded.Type() != types.DynamicFeeTxType || recorded.To() == nil || *recorded.To() != common.HexToAddress("0x101d") {
		t.Fatal("recorded call lost its EIP-1559 transaction, endpoint or identity binding")
	}
	fee, ok := new(big.Int).SetString(facts.FeeLimit, 10)
	if !ok {
		t.Fatal("invalid recorded fee limit")
	}
	calldata, err := producer.Pack("call", repairM1369Hash(t, facts.ProgramID), facts.GuestABI, facts.Entrypoint,
		repairM1369Bytes(t, facts.Calldata), repairM1369Bytes(t, facts.Capabilities), repairM1369Bytes(t, facts.Access),
		facts.ResponseCapacity, facts.Resources, fee)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(recorded.Data(), calldata) || !bytes.Equal(calldata, repairM1369Bytes(t, facts.EVMCalldata)) {
		t.Fatal("C# fixture fields do not reconstruct the actual producer transaction")
	}
	sender, err := types.Sender(types.NewLondonSigner(big.NewInt(125)), &recorded)
	if err != nil || sender == (common.Address{}) {
		t.Fatalf("recorded transaction signature: %v", err)
	}
	// Execute the real transaction signer, canonical encoder, decoder and
	// recovery path, with the same public scalar-one test key as the C# signer.
	key, err := crypto.HexToECDSA(strings.Repeat("0", 63) + "1")
	if err != nil {
		t.Fatal(err)
	}
	to := common.HexToAddress("0x101d")
	signer := types.NewLondonSigner(big.NewInt(125))
	unsigned := types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(125), Nonce: 7, GasTipCap: big.NewInt(2),
		GasFeeCap: big.NewInt(100), Gas: 150000, To: &to, Value: new(big.Int), Data: calldata})
	signed, err := types.SignTx(unsigned, signer, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var decoded types.Transaction
	if err := decoded.UnmarshalBinary(raw); err != nil {
		t.Fatal(err)
	}
	recovered, err := types.Sender(signer, &decoded)
	if err != nil || recovered.Hex() != common.HexToAddress("0x7e5f4552091a69125d5dfcb7b8c2659029395bdf").Hex() {
		t.Fatalf("real transaction signer/recovery failed: %s %v", recovered, err)
	}
	if decoded.Hash() != crypto.Keccak256Hash(raw) || !bytes.Equal(decoded.Data(), calldata) {
		t.Fatal("real signed transaction did not retain calldata and expected activity identity")
	}
	changedFee, err := producer.Pack("call", repairM1369Hash(t, facts.ProgramID), facts.GuestABI, facts.Entrypoint,
		repairM1369Bytes(t, facts.Calldata), repairM1369Bytes(t, facts.Capabilities), repairM1369Bytes(t, facts.Access),
		facts.ResponseCapacity, facts.Resources, new(big.Int).Add(fee, big.NewInt(1)))
	if err != nil || bytes.Equal(changedFee, decoded.Data()) {
		t.Fatalf("fee limit not bound in signed calldata: %v", err)
	}

	pending := repairM1369Member(t, "ProgramsContractTests.cs", "public async Task PendingReceiptMayOmitRetainedBytesButMustBindExpectedActivity()",
		"RepairM1369Native.RecordedVector()", "Programs.TransactionHash(raw)", "programs.ReceiptAsync(",
		"Assert.True(pending.IsUnknown)", "Assert.Equal(activity, pending.ActivityId)", "Assert.Null(pending.RetainedSignedActivity)",
		"Assert.Equal(SdkErrorCode.VerificationFailure, error.Code)")
	signedCase := repairM1369Member(t, "ProgramsContractTests.cs", "public async Task NativeSignedFixturesPreserveCanonicalCallBindings(string name)",
		"RepairM1369Native.Sign(unsigned)", "programs.SubmitAsync(call, key)", "eth_sendRawTransaction", "pax_sequencerReceipt",
		"Assert.Equal(call.TransactionHash, pending.ActivityId)", "Assert.Equal(signed, pending.RetainedSignedActivity)",
		"ResponseCapacity = native.ResponseCapacity + 1", "fee.Value + 1", "signed_activity_hex", "new ushort[] { 0, 6, ushort.MaxValue }", "new ushort[] { 3, 4 }")
	repairM1369Member(t, "ProgramsContractTests.cs", "public async Task RecordedProgramsSubmissionAuthenticatesReceiptAndRetainedTransaction()",
		"programs.SubmitAsync(call, key)", "programs.ReceiptAsync(key, expected,", "Assert.Equal(expected, result.ActivityId)",
		"Assert.Equal(raw, result.RetainedSignedTransaction)", "receipt.CanonicalBytes", "receipt.ReceiptDigest", "receipt.ReturnData")
	lifecycle := repairM1369Member(t, "ProgramsContractTests.cs", "public void NativeLifecycleFixturesPreserveSignedBindingAcrossNamedAbiPolicies(string name, int ordinal)",
		"RepairM1369Native.Sign(unsigned)", "new NativeProgramLifecycleRequest(operation, signed)", "Programs.TransactionHash(signed)",
		"new NativeProgramLifecycleRequest(updated, signed)")
	for _, body := range []string{pending, signedCase, lifecycle} {
		for _, legacy := range []string{"Invoke(\"Encode\"", "Invoke(\"DecodeSignedCall\"", "new ProgramsClient(client, Enumerable"} {
			if strings.Contains(body, legacy) {
				t.Fatalf("native contract scenario still relies on %s", legacy)
			}
		}
	}
	repairM1369Member(t, "RepairM1369Native.cs", "internal static byte[] Sign(UnsignedProgramTransaction transaction)",
		"new ECDsaSigner(new HMacDsaKCalculator(new Sha256Digest()))", "signer.GenerateSignature(digest)",
		"verifier.VerifySignature(digest, r, s)", "r.ModInverse(domain.N)", "Assert.InRange(parity, 0, 1)",
		"Assert.Equal(Keccak(signed), Programs.TransactionHash(signed))")
}

func TestRepairM_1369_programs_evm_producer_gap(t *testing.T) {
	producer := repairM1369ABI(t)
	vector := repairM1369ProgramVector(t)
	var view struct {
		ReturnData string `json:"returnData"`
	}
	if err := json.Unmarshal(vector.ReceiptJSON, &view); err != nil {
		t.Fatal(err)
	}
	output := repairM1369Bytes(t, view.ReturnData)
	decoded, err := producer.Methods["simulateCall"].Outputs.Unpack(output)
	if err != nil {
		t.Fatal(err)
	}
	if decoded[0] != int32(0) || decoded[1] != uint8(1) || len(decoded[2].([]byte)) == 0 {
		t.Fatalf("native recorded terminal output lost its status, kind or response: %#v", decoded)
	}
	packed, err := producer.Methods["simulateCall"].Outputs.Pack(decoded...)
	if err != nil || !bytes.Equal(output, packed) {
		t.Fatalf("producer ABI output changed: %v", err)
	}
	var facts repairM1369Call
	repairM1369Decode(t, "tests/LayerX.Sdk.Tests/Fixtures/repair-m1369-recorded-call.json", &facts)
	simulationInput := append(append([]byte{}, producer.Methods["simulateCall"].ID...), repairM1369Bytes(t, facts.EVMCalldata)[4:]...)
	args, err := producer.Methods["simulateCall"].Inputs.Unpack(simulationInput[4:])
	if err != nil || args[0] != repairM1369Hash(t, facts.ProgramID) {
		t.Fatalf("native simulation input: %#v %v", args, err)
	}
	repairM1369Exchange(t, simulationInput, output, func(answer []byte) {
		values, err := producer.Methods["simulateCall"].Outputs.Unpack(answer)
		if err != nil || !reflect.DeepEqual(values, decoded) {
			t.Fatalf("native response bytes changed across root RPC: %v", err)
		}
	})
	// Execute exactly the native precompile's ABI output codec. The committed
	// discovery vectors are codec vectors, not claims of native host execution.
	var discovery struct {
		Cases []repairM1369Discovery `json:"cases"`
	}
	repairM1369Decode(t, "tests/LayerX.Sdk.Tests/Fixtures/repair-m1369-discovery-abi.json", &discovery)
	if len(discovery.Cases) != 3 {
		t.Fatalf("expected all three native lifecycle codec vectors, got %d", len(discovery.Cases))
	}
	seen := map[string]bool{}
	for _, row := range discovery.Cases {
		if seen[row.Lifecycle] {
			t.Fatalf("duplicate lifecycle %s", row.Lifecycle)
		}
		seen[row.Lifecycle] = true
		head, err := producer.Methods["getProgram"].Outputs.Pack(row.Policy, repairM1369Hash(t, row.Authority),
			repairM1369Hash(t, row.CodeHash), row.ABI, row.Version, row.Active)
		if err != nil || !bytes.Equal(head, repairM1369Bytes(t, row.Head)) {
			t.Fatalf("getProgram producer codec mismatch: %v", err)
		}
		wind, err := producer.Methods["getWindDown"].Outputs.Pack(row.Status, repairM1369Hash(t, row.ExitProgram),
			row.Deadline, row.EffectiveSequence, row.AccountCount, row.LiveCount)
		if err != nil || !bytes.Equal(wind, repairM1369Bytes(t, row.WindDown)) {
			t.Fatalf("getWindDown producer codec mismatch: %v", err)
		}
		expectedLifecycle := map[uint8]string{1: "Active", 2: "Deprecated", 3: "Tombstoned"}[row.Status]
		if expectedLifecycle != row.Lifecycle || row.Active != (row.Lifecycle == "Active") {
			t.Fatalf("inconsistent lifecycle vector: %+v", row)
		}
		id := [32]byte{0x42}
		input, err := producer.Pack("getProgram", id)
		if err != nil {
			t.Fatal(err)
		}
		repairM1369Exchange(t, input, head, func(answer []byte) {
			values, err := producer.Methods["getProgram"].Outputs.Unpack(answer)
			if err != nil || values[2] != repairM1369Hash(t, row.CodeHash) || values[3] != row.ABI || values[4] != row.Version || values[5] != row.Active {
				t.Fatalf("root RPC changed program head: %#v %v", values, err)
			}
		})
		if !row.Active {
			input, err = producer.Pack("getWindDown", id)
			if err != nil {
				t.Fatal(err)
			}
			repairM1369Exchange(t, input, wind, func(answer []byte) {
				values, err := producer.Methods["getWindDown"].Outputs.Unpack(answer)
				if err != nil || values[0] != row.Status {
					t.Fatalf("root RPC changed wind-down lifecycle: %#v %v", values, err)
				}
			})
		}
	}
	if !seen["Active"] || !seen["Deprecated"] || !seen["Tombstoned"] {
		t.Fatal("missing lifecycle case")
	}
	evmSource := repairM1368Read(t, "tests/LayerX.Sdk.Tests/ProgramsEvmTests.cs")
	if strings.Contains(evmSource, "/rpc") || strings.Contains(evmSource, "var head = new byte[192]") {
		t.Fatal("native scenarios must use root RPC and producer-codec snapshots")
	}
	repairM1369Member(t, "ProgramsEvmTests.cs", "public async Task ProgramSimulateUsesPendingEthCall()",
		"SdkErrorCode.TransportFailure", "POST / HTTP/1.1", "Assert.Equal(\"pending\", parameters[1].GetString())")
	repairM1369Member(t, "ProgramsEvmTests.cs", "public async Task ProgramSimulateDecodesRecordedNativeProducerOutput()",
		"RepairM1369Native.RecordedCall()", "programs.SimulateAsync(call, Sender)", "Assert.Equal(0, result.TerminalResult)",
		"Assert.Equal((byte)1, result.TerminalKind)", "Assert.Equal(output.AsSpan(128, responseLength).ToArray(), result.Response)")
	repairM1369Member(t, "ProgramsEvmTests.cs", "public async Task ProgramSimulateRefusesMalformedNativeOutput()",
		"\"truncated\", \"offset\", \"padding\", \"length\", \"kind\", \"result\"", "Assert.ThrowsAsync<ArgumentException>",
		"Assert.ThrowsAsync<OverflowException>", "SdkErrorCode.DecodeFailure")
	repairM1369Member(t, "ProgramsEvmTests.cs", "public async Task ProgramDiscoverReadsWindDownLifecycle()",
		"DiscoveryFixture()", "get_program_output_hex", "get_wind_down_output_hex", "Assert.Equal(scenario.Lifecycle.Value, result.Lifecycle)",
		"Assert.Equal(\"pending-eth-call\", result.Verification)", "Assert.Empty(result.ReceiptDigest)", "SdkErrorCode.DecodeFailure",
		"(false, 1, 192, null)", "(false, 0, 192, null)", "(false, 4, 192, null)", "(false, 258, 192, null)",
		"(false, 2, 0, null)", "(false, 2, 31, null)", "(false, 2, 32, null)", "(false, 3, 191, null)")
	repairM1369Member(t, "ProgramsEvmTests.cs", "public async Task UpgradeSubmissionBindsPreparedFeeLimit()",
		"RepairM1369Native.Sign(transaction)", "new ProtocolAmount(\"1000\")", "new ProtocolAmount(\"0\")", "SdkErrorCode.UnknownOutcome")
}

func repairM1369Exchange(t *testing.T, input, output []byte, consume func([]byte)) {
	t.Helper()
	received := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/" {
			t.Error("native request did not use POST /")
			http.Error(w, "route", 400)
			return
		}
		var request struct {
			JSONRPC string            `json:"jsonrpc"`
			ID      json.RawMessage   `json:"id"`
			Method  string            `json:"method"`
			Params  []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "JSON", 400)
			return
		}
		if request.JSONRPC != "2.0" || request.Method != "eth_call" || len(request.Params) != 2 || string(request.Params[1]) != `"pending"` {
			t.Errorf("incorrect native JSON-RPC envelope: %+v", request)
			http.Error(w, "envelope", 400)
			return
		}
		var call map[string]string
		if err := json.Unmarshal(request.Params[0], &call); err != nil {
			t.Error(err)
			http.Error(w, "params", 400)
			return
		}
		if call["to"] != "0x000000000000000000000000000000000000101d" || call["data"] != "0x"+hex.EncodeToString(input) {
			t.Errorf("native calldata or precompile target changed: %v", call)
			http.Error(w, "target", 400)
			return
		}
		select {
		case received <- struct{}{}:
		default:
			t.Error("unexpected extra native request")
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": "0x" + hex.EncodeToString(output)}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, err := rpc.DialHTTP(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var result string
	err = client.CallContext(ctx, &result, "eth_call", map[string]string{
		"to": "0x000000000000000000000000000000000000101d", "data": "0x" + hex.EncodeToString(input),
	}, "pending")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-received:
	default:
		t.Fatal("native request did not reach the loopback peer")
	}
	answer := repairM1369Bytes(t, result)
	if !bytes.Equal(output, answer) {
		t.Fatal("native output bytes changed in transport")
	}
	consume(answer)
}

func TestRepairM_1369_receipt_positive_vector_count(t *testing.T) {
	vectors := repairM1369Vectors(t)
	expected := []string{"V1-pax-transfer", "V1-erc20-approve", "V1-layerxledger-pay-exact", "V1-layerxbridge-bridge-out", "V1-layerxprograms-call"}
	var names []string
	for _, vector := range vectors {
		names = append(names, vector.ID)
	}
	if !reflect.DeepEqual(names, expected) {
		t.Fatalf("producer inventory = %v, want %v", names, expected)
	}
	// The node recorder envelope includes additional top-level receipt and
	// recorder metadata. The SDK publishes its cases as a conformance corpus.
	// Compare every field of every complete case, preserving JSON numbers and
	// exact embedded transaction, receipt, signature, log and return-data strings.
	corpus := func(path string) []any {
		decoder := json.NewDecoder(strings.NewReader(repairM1368Read(t, path)))
		decoder.UseNumber()
		var envelope struct {
			Cases []any `json:"cases"`
		}
		if err := decoder.Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Cases
	}
	producerCopy := corpus("../../../daemon/committed-fixture/vectors/vector-v1-receipted-positive.json")
	sdkCopy := corpus("../conformance/fixtures/vector-v1-receipted-positive.json")
	if !reflect.DeepEqual(sdkCopy, producerCopy) {
		t.Fatal("SDK receipt cases differ from the real node producer corpus")
	}
	producerABI := repairM1369ABI(t)
	for _, vector := range vectors {
		t.Run(vector.ID, func(t *testing.T) {
			canonical := repairM1369Bytes(t, vector.SignedReceipt)
			verified, err := proof.ReceiptSignature(canonical, repairM1369Hash(t, vector.PinnedKey.PublicKey))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(verified.Receipt.CanonicalBytes(), canonical) || verified.Digest != repairM1369Hash(t, vector.ReceiptDigest) || verified.Receipt.ResultCode != 0 {
				t.Fatal("producer receipt authenticity, bytes, digest or result changed")
			}
			var transaction types.Transaction
			if err := transaction.UnmarshalBinary(repairM1369Bytes(t, vector.RawTransaction)); err != nil {
				t.Fatal(err)
			}
			if common.Hash(verified.Receipt.ActivityID) != transaction.Hash() {
				t.Fatal("receipt does not bind the real signed EVM transaction")
			}
			height := binary.BigEndian.Uint64(verified.Receipt.BatchID[24:])
			if height < vector.PinnedKey.FirstHeight || height > vector.PinnedKey.LastHeight {
				t.Fatal("real receipt lies outside the independent key's height range")
			}
			bad := append([]byte(nil), canonical...)
			bad[len(bad)-1] ^= 1
			if _, err := proof.ReceiptSignature(bad, repairM1369Hash(t, vector.PinnedKey.PublicKey)); err == nil {
				t.Fatal("real receipt verifier accepted signature tampering")
			}
			wrongPin := repairM1369Hash(t, vector.PinnedKey.PublicKey)
			wrongPin[0] ^= 1
			if _, err := proof.ReceiptSignature(canonical, wrongPin); err == nil {
				t.Fatal("real receipt verifier accepted a substituted independent key")
			}
			if vector.ID != "V1-layerxprograms-call" {
				return
			}
			var view struct {
				ReturnData string `json:"returnData"`
				Logs       []struct {
					Address string   `json:"address"`
					Topics  []string `json:"topics"`
					Data    string   `json:"data"`
				} `json:"logs"`
			}
			if err := json.Unmarshal(vector.ReceiptJSON, &view); err != nil {
				t.Fatal(err)
			}
			if len(view.Logs) != 1 {
				t.Fatalf("expected the actual Programs outcome log, got %d", len(view.Logs))
			}
			log := view.Logs[0]
			event := producerABI.Events["ProgramCallOutcome"]
			if log.Address != "0x000000000000000000000000000000000000101d" || len(log.Topics) != 2 || common.HexToHash(log.Topics[0]) != event.ID {
				t.Fatal("native outcome event lost its authenticated address or topics")
			}
			logBytes := repairM1369Bytes(t, log.Data)
			values, err := event.Inputs.NonIndexed().Unpack(logBytes)
			if err != nil || len(values) != 13 || values[2] != uint64(0) || values[3] != uint8(0) || values[4] != uint16(1) || values[5] != uint16(1) || values[8] != int32(0) {
				t.Fatalf("actual Programs outcome identity, frame, ABI or result changed: %#v %v", values, err)
			}
			var facts repairM1369Call
			repairM1369Decode(t, "tests/LayerX.Sdk.Tests/Fixtures/repair-m1369-recorded-call.json", &facts)
			if common.HexToHash(log.Topics[1]) != common.Hash(repairM1369Hash(t, facts.ProgramID)) {
				t.Fatal("outcome log belongs to another program")
			}
			// The production codec exposes the signed log effect; compare its
			// digest to the exact producer bytes rather than inventing a log.
			if len(verified.Receipt.Effects) != 2 {
				t.Fatal("expected summary and native outcome effects")
			}
			effect := verified.Receipt.Effects[1]
			digest := sha256.Sum256(logBytes)
			if effect.Kind != 3 || !bytes.Equal(effect.Body[len(effect.Body)-32:], digest[:]) {
				t.Fatal("producer log is not authenticated by the signed receipt effect")
			}
			outputs, err := producerABI.Methods["call"].Outputs.Unpack(repairM1369Bytes(t, view.ReturnData))
			if err != nil || outputs[0] != int32(0) || outputs[1] != uint8(1) {
				t.Fatalf("native return data changed: %v", err)
			}
		})
	}
	source := repairM1369Member(t, "SequencerReceiptTests.cs", "public void SequencerReceiptV1CasesVerifyReceipted()",
		"Assert.Equal(5, cases.Length)", "V1-layerxprograms-call", "Assert.Equal(Programs.AddressValue, log.GetProperty(\"address\").GetString())",
		"0xcfe5297f03a2951065e463e9e0c2e3686bc0204f727eeec1a52357b5dd4a27a0", "Assert.Equal(13 * 32, Hex(log, \"data\").Length)",
		"AssertCheck(ReceiptCheck.Logs", "result.CanonicalBytes", "result.ReceiptDigest", "result.ReturnData", "ReceiptCheck.KeyHeight")
	for _, name := range expected {
		if !strings.Contains(source, fmt.Sprintf("%q", name)) {
			t.Errorf("C# inventory omits %s", name)
		}
	}
	for _, declaration := range []string{"public void SequencerReceiptV2RevertDecodesReason()", "public void SequencerReceiptV3RefusalsNameTheirCheck()"} {
		repairM1369Member(t, "SequencerReceiptTests.cs", declaration)
	}
	record := repairM1368Read(t, "repair_m1369.kvx")
	for _, name := range []string{"programs-contract-tests-stale", "programs-evm-producer-gap", "receipt-positive-vector-count"} {
		if !strings.Contains(record, "[finding."+name+"]") || !strings.Contains(record, "TestRepairM_1369_"+strings.ReplaceAll(name, "-", "_")) {
			t.Errorf("repair record must map finding %s to its proof", name)
		}
	}
}
