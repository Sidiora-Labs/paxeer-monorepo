package layerx

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type repair141Recorded struct {
	Authorization struct {
		PublicKey        [32]byte
		FirstBatchNumber uint64
		LastBatchNumber  uint64
	} `json:"authorization"`
	Settlement struct {
		RawTransaction string          `json:"raw_transaction"`
		Receipt        json.RawMessage `json:"receipt"`
		Batch          struct {
			PreviousSequenceHash []byte `json:"PrevSeqHash"`
			KeyID                []byte `json:"KeyId"`
		} `json:"sequence_batch"`
		BatchProof       committedRecordedTxProof `json:"sequence_batch_proof"`
		TransactionProof committedRecordedTxProof `json:"transaction_proof"`
		Result           struct {
			Code      uint32 `json:"code"`
			Data      []byte `json:"data"`
			GasWanted int64  `json:"gas_wanted"`
			GasUsed   int64  `json:"gas_used"`
		} `json:"exec_tx_result"`
		ResultProof     committedRecordedProof  `json:"exec_tx_result_proof"`
		ExecutionHeader committedRecordedHeader `json:"execution_header"`
		ExecutionCommit committedRecordedCommit `json:"execution_commit"`
		Header          committedRecordedHeader `json:"header"`
		Commit          committedRecordedCommit `json:"commit"`
		Validators      []struct {
			Key struct {
				Type  string `json:"type"`
				Value []byte `json:"value"`
			} `json:"pub_key"`
			Power int64 `json:"voting_power,string"`
		} `json:"validators"`
	} `json:"settlement"`
}

func repair141LoadRecorded(t *testing.T) (json.RawMessage, []byte, ReceiptKeys, PortableCommittedEvidence, CommittedTrust) {
	t.Helper()
	data, err := os.ReadFile("../../../daemon/committed-fixture/vectors/vector-v4-committed.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector repair141Recorded
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	s := vector.Settlement
	var keyID, previous [32]byte
	if len(s.Batch.KeyID) != 32 || len(s.Batch.PreviousSequenceHash) != 32 {
		t.Fatal("invalid producer receipt key")
	}
	copy(keyID[:], s.Batch.KeyID)
	copy(previous[:], s.Batch.PreviousSequenceHash)
	keys := ReceiptKeys{PreviousSequenceHash: previous, Pins: []ReceiptKey{{KeyID: keyID, PublicKey: vector.Authorization.PublicKey, FirstHeight: vector.Authorization.FirstBatchNumber, LastHeight: vector.Authorization.LastBatchNumber}}}
	var validators []CometValidator
	for _, v := range s.Validators {
		if v.Key.Type != "tendermint/PubKeyEd25519" || len(v.Key.Value) != 32 {
			t.Fatal("invalid producer validator")
		}
		var key [32]byte
		copy(key[:], v.Key.Value)
		validators = append(validators, CometValidator{PublicKey: key, VotingPower: v.Power})
	}
	header := s.ExecutionHeader.decode(t)
	evidence := PortableCommittedEvidence{BlockHeader: header, Commit: s.ExecutionCommit.decode(t), ValidatorSet: validators,
		ReceiptInclusion: ReceiptInclusion{Transaction: s.TransactionProof.Data, TransactionProof: s.TransactionProof.Proof.decode(t), Result: ExecTxResult{Code: s.Result.Code, Data: s.Result.Data, GasWanted: s.Result.GasWanted, GasUsed: s.Result.GasUsed}, ResultProof: s.ResultProof.decode(t), BatchTransaction: s.BatchProof.Data, BatchProof: s.BatchProof.Proof.decode(t), NextHeader: s.Header.decode(t), NextCommit: s.Commit.decode(t), NextValidators: validators}}
	trust := CommittedTrust{ChainID: header.ChainID, Height: header.Height, BlockHash: header.Hash()}
	return s.Receipt, sequencerFixtureBytes(t, s.RawTransaction), keys, evidence, trust
}

func repair141Programs(t *testing.T, handler http.Handler, keys ReceiptKeys) *Programs {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	transport, err := NewProgramRPCTransport(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	programs, err := NewProgramsWithTrustOptions(client, [32]byte{1}, ProgramTrustOptions{Sender: [20]byte{0x12}, ReceiptKeys: keys, ClockMilliseconds: func() uint64 { return 1000 }, MaximumSimulationAgeMilliseconds: 300000})
	if err != nil {
		t.Fatal(err)
	}
	return programs
}

func TestRepair_141_committed_rpc_rung(t *testing.T) {
	receipt, raw, keys, evidence, trust := repair141LoadRecorded(t)
	hash := receiptKeccak256(raw)
	var view sequencerReceiptJSON
	if err := json.Unmarshal(receipt, &view); err != nil {
		t.Fatal(err)
	}
	if view.Rung != "committed" || view.BlockNumber == nil || view.BlockHash == nil {
		t.Fatal("producer recording lacks committed transport view")
	}
	var methods []string
	programs := repair141Programs(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request programRPCRecord
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "JSON", 400)
			return
		}
		if len(request.Params) != 1 || string(request.Params[0]) != fmt.Sprintf("%q", portableHex(hash[:])) {
			t.Error("transaction identity changed")
		}
		methods = append(methods, request.Method)
		var result any
		switch request.Method {
		case "eth_getTransactionByHash":
			result = map[string]string{"hash": portableHex(hash[:])}
		case "pax_sequencerReceipt":
			result = receipt
		case "eth_getRawTransactionByHash":
			result = portableHex(raw)
		default:
			t.Errorf("unexpected method %s", request.Method)
			http.Error(w, "method", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}), keys)
	submission, err := programs.Activity(context.Background(), hash)
	if err != nil {
		t.Fatal(err)
	}
	authenticated := submission.SequencerReceipt
	if submission.State != ProgramSubmissionExecuted || authenticated == nil || authenticated.Level != "receipted" || authenticated.Rung != "receipted" || authenticated.TransportBlock == nil || authenticated.TransportBlock.Height != uint64(evidence.BlockHeader.Height) || authenticated.TransportBlock.BlockHash != trust.BlockHash {
		t.Fatalf("transport rung was confused with independently verified finality: %+v", submission)
	}
	if !reflect.DeepEqual(methods, []string{"eth_getTransactionByHash", "pax_sequencerReceipt", "eth_getRawTransactionByHash"}) {
		t.Fatal(methods)
	}
	bundle, err := ExportPortableReceipt(receipt, raw, keys, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bundle.ReceiptJSON, receipt) || bundle.VerificationLevel != "receipted" {
		t.Fatal("portable export rewrote the RPC response or promoted finality")
	}
	encoded, err := bundle.Export()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := VerifyPortableReceipt(encoded, keys)
	if err != nil || restored.TransportBlock == nil || *restored.TransportBlock != *authenticated.TransportBlock {
		t.Fatalf("lost RPC coordinates: %+v %v", restored, err)
	}
	committed, err := programs.ActivityCommitted(context.Background(), hash, evidence, trust)
	if err != nil || committed.SequencerReceipt == nil || committed.SequencerReceipt.Level != "committed" {
		t.Fatalf("trusted inclusion failed: %+v %v", committed, err)
	}
	bundle, err = ExportPortableReceipt(receipt, raw, keys, &evidence, trust)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err = bundle.Export()
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyPortableReceipt(encoded, keys, trust)
	if err != nil || verified.Level != "committed" {
		t.Fatalf("portable commitment failed: %+v %v", verified, err)
	}
	// Coordinates are transport claims until they match the independently anchored proof.
	view.BlockHash = func() *string { value := "0x" + strings.Repeat("ff", 32); return &value }()
	changed, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExportPortableReceipt(changed, raw, keys, &evidence, trust); !errors.Is(err, ErrVerification) {
		t.Fatalf("substituted RPC coordinate accepted: %v", err)
	}
	for _, mutate := range []func(*sequencerReceiptJSON){func(v *sequencerReceiptJSON) { v.BlockHash = nil }, func(v *sequencerReceiptJSON) { v.BlockNumber = nil }, func(v *sequencerReceiptJSON) { value := "0x00"; v.BlockNumber = &value }} {
		var v sequencerReceiptJSON
		_ = json.Unmarshal(receipt, &v)
		mutate(&v)
		data, _ := json.Marshal(v)
		if result, err := VerifySequencerReceipt(data, raw, keys); !errors.Is(err, ErrVerification) || result.Rung != "" {
			t.Fatal("malformed committed coordinates were accepted")
		}
	}
}

func TestRepair_141_portable_unanchored_finality(t *testing.T) {
	c := sequencerReceiptCases(t)[0]
	keys := sequencerFixtureKeys(t, c)
	f := newCommittedFixture(t, c.ID, [4]int64{1, 1, 1, 1}, 0)
	trusted := f.trust
	evidence := PortableCommittedEvidence{BlockHeader: f.header, ReceiptInclusion: f.inclusion, Commit: f.commit, ValidatorSet: f.validators}
	bundle, err := ExportPortableReceipt(c.ReceiptJSON, sequencerFixtureBytes(t, c.RawTransaction), keys, &evidence, trusted)
	if err != nil {
		t.Fatal(err)
	}
	data, err := bundle.Export()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyPortableReceipt(data, keys); !errors.Is(err, ErrVerification) {
		t.Fatalf("missing external checkpoint was accepted: %v", err)
	}
	wrong := trusted
	wrong.ChainID = "fabricated-network"
	if _, err := VerifyPortableReceipt(data, keys, wrong); !errors.Is(err, ErrVerification) {
		t.Fatal("wrong network accepted")
	}
	for n := range f.keys {
		f.keys[n] = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(0x80 + n)}, 32))
		copy(f.validators[n].PublicKey[:], f.keys[n].Public().(ed25519.PublicKey))
	}
	f.reanchor(t)
	fabricated := CommittedTrust{ChainID: f.header.ChainID, Height: f.header.Height, BlockHash: f.header.Hash()}
	// All receipt signatures, transaction/result proofs and attacker commits are valid.
	if _, err := VerifyCommittedReceipt(f.receipt, f.header, f.inclusion, f.commit, f.validators, fabricated); err != nil {
		t.Fatalf("attack did not preserve consistency: %v", err)
	}
	bundle.BlockHeader = &f.header
	bundle.ReceiptInclusion = &f.inclusion
	bundle.Commit = &f.commit
	bundle.ValidatorSet = f.validators
	data, err = bundle.Export()
	if err != nil {
		t.Fatal(err)
	}
	result, err := VerifyPortableReceipt(data, keys, trusted)
	var refusal *VerificationError
	if !errors.As(err, &refusal) || refusal.Check != "committed-trust-anchor" || result.Rung != "" {
		t.Fatalf("private validator chain established network finality: %+v %v", result, err)
	}
	if _, err := ExportPortableReceipt(c.ReceiptJSON, sequencerFixtureBytes(t, c.RawTransaction), keys, &evidence); !errors.Is(err, ErrVerification) {
		t.Fatal("export accepted committed evidence without external trust")
	}
}

func TestRepair_141_validator_evolution(t *testing.T) {
	c := sequencerReceiptCases(t)[0]
	f := newCommittedFixture(t, c.ID, [4]int64{1, 1, 1, 1}, 0)
	var previousKeys []ed25519.PrivateKey
	var previousValidators []CometValidator
	for n := 0; n < 4; n++ {
		key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(n + 0x50)}, 32))
		previousKeys = append(previousKeys, key)
		var public [32]byte
		copy(public[:], key.Public().(ed25519.PublicKey))
		previousValidators = append(previousValidators, CometValidator{PublicKey: public, VotingPower: 1})
	}
	previous := f.header
	previous.Height--
	previous.Time = previous.Time.Add(-time.Second)
	previousHash := committedValidatorsHash(previousValidators)
	previous.ValidatorsHash = previousHash[:]
	previousCommit := committedFixtureCommit(previous, previousKeys, 3)
	f.header.LastBlockID = previousCommit.BlockID
	f.commit = committedFixtureCommit(f.header, f.keys, 3)
	f.inclusion.NextHeader.LastBlockID = f.commit.BlockID
	f.inclusion.NextCommit = committedFixtureCommit(f.inclusion.NextHeader, f.keys, 3)
	f.inclusion.ValidatorHistory = []CommittedLightBlock{{Header: previous, Commit: previousCommit, Validators: previousValidators}}
	trust := CommittedTrust{ChainID: previous.ChainID, Height: previous.Height, BlockHash: previous.Hash()}
	result, err := VerifyCommittedReceipt(f.receipt, f.header, f.inclusion, f.commit, f.validators, trust)
	if err != nil || result.Level != "committed" {
		t.Fatalf("authenticated validator rotation refused: %+v %v", result, err)
	}
	for _, mutation := range []string{"missing-history", "signature", "validator-transition", "network"} {
		t.Run(mutation, func(t *testing.T) {
			encoded, _ := json.Marshal(f.inclusion)
			var changed ReceiptInclusion
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "missing-history":
				changed.ValidatorHistory = nil
			case "signature":
				changed.ValidatorHistory[0].Commit.Signatures[0].Signature[0] ^= 1
			case "validator-transition":
				changed.ValidatorHistory[0].Header.NextValidatorsHash = bytes.Repeat([]byte{0x99}, 32)
				changed.ValidatorHistory[0].Commit = committedFixtureCommit(changed.ValidatorHistory[0].Header, previousKeys, 3)
			case "network":
				changed.ValidatorHistory[0].Header.ChainID = "private-network"
				changed.ValidatorHistory[0].Commit = committedFixtureCommit(changed.ValidatorHistory[0].Header, previousKeys, 3)
			}
			if value, err := VerifyCommittedReceipt(f.receipt, f.header, changed, f.commit, f.validators, trust); !errors.Is(err, ErrVerification) || value.Rung != "" {
				t.Fatalf("unauthenticated validator evolution accepted: %+v %v", value, err)
			}
		})
	}
}

func TestRepair_141_stream_checkpoint_before_consumption(t *testing.T) {
	golden, err := os.ReadFile("../../../services/human/schema/human-api/golden/stream.next.response.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Body struct {
			OK     bool       `json:"ok"`
			Result StreamPage `json:"result"`
			Trace  string     `json:"trace"`
		} `json:"body"`
	}
	if err := json.Unmarshal(golden, &fixture); err != nil {
		t.Fatal(err)
	}
	page := fixture.Body.Result
	if len(page.Events) != 2 {
		t.Fatal("need the real two-event producer page")
	}
	initial, _ := NewStreamCursor("cur-000000000000000042")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := strings.TrimPrefix(r.URL.Path, "/v1/stream/")
		next := page
		switch cursor {
		case initial.String():
		case page.Events[0].Cursor:
			next.Events = page.Events[1:]
		case page.Events[1].Cursor:
			next.Events = []StreamEvent{}
		default:
			t.Errorf("unexpected consumer checkpoint %s", cursor)
			http.Error(w, "cursor", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": next, "trace": fixture.Body.Trace})
	}))
	defer server.Close()
	open := func(cursor StreamCursor) *ResumableStream {
		transport, err := NewHumanHTTPTransport(server.URL, server.Client(), nil)
		if err != nil {
			t.Fatal(err)
		}
		client, err := NewClient(transport, nil)
		if err != nil {
			t.Fatal(err)
		}
		source, err := NewHumanStreamSource(client)
		if err != nil {
			t.Fatal(err)
		}
		stream, err := NewResumableStream(cursor, source)
		if err != nil {
			t.Fatal(err)
		}
		return stream
	}
	stream := open(initial)
	events, err := stream.Next(context.Background())
	if err != nil || len(events) != 2 || stream.Cursor() != initial {
		t.Fatalf("fetch advanced consumption checkpoint: %v", err)
	}
	wrong, _ := NewStreamCursor(events[1].Cursor)
	if err := stream.Acknowledge(wrong); err == nil || stream.Cursor() != initial {
		t.Fatal("out-of-order acknowledgment accepted")
	}
	failed := errors.New("consumer failed")
	if err := stream.Run(context.Background(), func(StreamEvent) error { return failed }); !errors.Is(err, failed) || stream.Cursor() != initial {
		t.Fatal("failed first callback lost events")
	}
	if replay, err := stream.Next(context.Background()); err != nil || !reflect.DeepEqual(replay, events) || stream.Cursor() != initial {
		t.Fatalf("retry before restart lost the unacknowledged page: %v %v", replay, err)
	}
	checkpoint := filepath.Join(t.TempDir(), "cursor")
	reopen := func(old *ResumableStream) *ResumableStream {
		if err := os.WriteFile(checkpoint, []byte(old.Cursor().String()), 0600); err != nil {
			t.Fatal(err)
		}
		saved, err := os.ReadFile(checkpoint)
		if err != nil {
			t.Fatal(err)
		}
		cursor, err := NewStreamCursor(string(saved))
		if err != nil {
			t.Fatal(err)
		}
		return open(cursor)
	}
	stream = reopen(stream)
	var consumed []string
	if err := stream.Run(context.Background(), func(event StreamEvent) error {
		if event.Cursor == page.Events[1].Cursor {
			return failed
		}
		consumed = append(consumed, event.Cursor)
		return nil
	}); !errors.Is(err, failed) || stream.Cursor().String() != page.Events[0].Cursor {
		t.Fatal("checkpoint did not stop at successfully consumed event")
	}
	if replay, err := stream.Next(context.Background()); err != nil || !reflect.DeepEqual(replay, page.Events[1:]) || stream.Cursor().String() != page.Events[0].Cursor {
		t.Fatalf("partial acknowledgment did not retain only the failed event: %v %v", replay, err)
	}
	stream = reopen(stream)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = stream.Run(ctx, func(event StreamEvent) error { consumed = append(consumed, event.Cursor); cancel(); return nil })
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(consumed, []string{page.Events[0].Cursor, page.Events[1].Cursor}) || stream.Cursor().String() != page.NextCursor {
		t.Fatalf("restart lost or duplicated events: %v %v %s", consumed, err, stream.Cursor())
	}
	stream = reopen(stream)
	if events, err := stream.Next(context.Background()); err != nil || len(events) != 0 || stream.Cursor().String() != page.NextCursor {
		t.Fatalf("fully acknowledged restart replayed events: %v %v", events, err)
	}
}

func TestRepair_141_wind_down_native_route(t *testing.T) {
	var last struct {
		From            string
		Calldata        string
		RawTransaction  string
		TransactionHash string
		R               string
		S               string
		YParity         uint64
		Authorization   string
		Register        string
		WindDown        string
	}
	fixture, err := os.ReadFile("testdata/repair141/native-deprecate.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fixture, &last); err != nil {
		t.Fatal(err)
	}
	programs, records := programEVMEndpoint(t, "")
	copy(programs.sender[:], sequencerFixtureBytes(t, last.From))
	var transactions []ProgramTransaction
	for _, name := range []string{"route", "tombstone", "exit", "deprecate"} {
		t.Run(name, func(t *testing.T) {
			vector := readProgramEVMVector(t, "native-program-wind-down-"+name+"-v4")
			value, err := DecodeNativeProgramWindDown(programEVMVectorBytes(t, vector.PayloadHex))
			if err != nil {
				t.Fatal(err)
			}
			if name == "deprecate" {
				value = NativeProgramWindDown{ProgramID: [32]byte{0x42}, Operation: 2, ExitProgram: [32]byte{0x42}, DeadlineBatch: 1000}
				vector.EVM.Calldata = last.Calldata
			}
			prepared, err := programs.WindDown(context.Background(), value, nil, IdempotencyKey{})
			if err != nil {
				t.Fatal(err)
			}
			all := records()
			assertProgramEVMTransaction(t, prepared, vector, all[len(all)-5:])
			payload, err := EncodeNativeProgramWindDown(value)
			if err != nil {
				t.Fatal(err)
			}
			if name != "deprecate" && !bytes.Equal(payload, programEVMVectorBytes(t, vector.PayloadHex)) {
				t.Fatal("wind-down payload differs from native producer vector")
			}
			transactions = append(transactions, *prepared.Transaction)
		})
	}
	if len(records()) != 20 || len(transactions) != 4 {
		t.Fatalf("native preparation did not use nonce, fee and estimate RPC: %v", records())
	}
	// The signed fixture must contain exactly the prepared EIP-1559 transaction.
	// Rebuild its envelope with the fixture's external signer output.
	tx := transactions[len(transactions)-1]
	var encoded []byte
	for _, quantity := range []string{tx.ChainID, tx.Nonce, tx.MaxPriorityFeePerGas, tx.MaxFeePerGas, tx.Gas} {
		number, ok := receiptQuantity(quantity, 256)
		if !ok {
			t.Fatal("invalid prepared quantity")
		}
		encoded = append(encoded, committedRLP(number.Bytes(), false)...)
	}
	encoded = append(encoded, committedRLP(sequencerFixtureBytes(t, tx.To), false)...)
	encoded = append(encoded, committedRLP(nil, false)...)
	encoded = append(encoded, committedRLP(sequencerFixtureBytes(t, tx.Data), false)...)
	encoded = append(encoded, committedRLP(nil, true)...)
	for _, quantity := range []string{fmt.Sprintf("0x%x", last.YParity), last.R, last.S} {
		number, ok := receiptQuantity(quantity, 256)
		if !ok {
			t.Fatal("invalid signer output")
		}
		encoded = append(encoded, committedRLP(number.Bytes(), false)...)
	}
	signed := sequencerFixtureBytes(t, last.RawTransaction)
	if !bytes.Equal(signed, append([]byte{2}, committedRLP(encoded, true)...)) {
		t.Fatal("external signature envelope differs from prepared native transaction")
	}
	expectedHash := receiptKeccak256(signed)
	if last.TransactionHash != portableHex(expectedHash[:]) || signed[0] != 2 {
		t.Fatal("signed native deprecation identity differs")
	}
	broadcaster := repair141Programs(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request programRPCRecord
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		var result string
		switch request.Method {
		case "eth_sendRawTransaction":
			if len(request.Params) != 1 || string(request.Params[0]) != fmt.Sprintf("%q", last.RawTransaction) {
				t.Error("signed transaction changed in transport")
			}
			result = last.TransactionHash
		case "eth_call":
			var call map[string]string
			if len(request.Params) != 2 || json.Unmarshal(request.Params[0], &call) != nil || call["to"] != layerxProgramsAddress || string(request.Params[1]) != `"pending"` {
				t.Error("native lifecycle state read changed")
				return
			}
			switch call["data"] {
			case "0xb1ffb3d4" + hex.EncodeToString(append([]byte{0x42}, make([]byte, 31)...)):
				result = last.Register
			case "0x98e4e41d" + hex.EncodeToString(append([]byte{0x42}, make([]byte, 31)...)):
				result = last.WindDown
			default:
				t.Errorf("unexpected native lifecycle selector: %s", call["data"])
				return
			}
		default:
			t.Errorf("unexpected native method: %s", request.Method)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}), ReceiptKeys{})
	submission, err := broadcaster.Broadcast(context.Background(), signed)
	if err != nil || submission.State != ProgramSubmissionPending || submission.TransactionHash != expectedHash {
		t.Fatalf("native broadcast lost transaction identity: %+v %v", submission, err)
	}
	discovery, err := broadcaster.Discover(context.Background(), [32]byte{0x42})
	if err != nil || discovery.Lifecycle != "deprecated" || discovery.Verification != "pending-state-read" {
		t.Fatalf("native lifecycle state response was lost: %+v %v", discovery, err)
	}
	authorization := repair141Programs(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request programRPCRecord
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		var answer any = "0x7"
		if request.Method == "eth_chainId" {
			answer = "0x7d"
		}
		if request.Method == "eth_estimateGas" {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "error": map[string]any{"code": 3, "message": last.Authorization}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": answer})
	}), ReceiptKeys{})
	_, err = authorization.WindDown(context.Background(), NativeProgramWindDown{ProgramID: [32]byte{0x42}, Operation: 2, ExitProgram: [32]byte{0x42}, DeadlineBatch: 1000}, nil, IdempotencyKey{})
	var rpcError *ProgramRPCError
	if !errors.As(err, &rpcError) || rpcError.Code != 3 || rpcError.Message != last.Authorization {
		t.Fatalf("native operation authorization refusal was lost: %v", err)
	}
	if _, err := programs.WindDown(context.Background(), NativeProgramWindDown{ProgramID: [32]byte{1}, Operation: 2, ExitProgram: [32]byte{2}, DeadlineBatch: 1000}, nil, IdempotencyKey{}); err == nil {
		t.Fatal("unsupported exit program route accepted")
	}
}

type repair141MirrorExchange struct {
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response"`
}

type repair141MirrorFixture struct {
	verifier   *MirrorVerifier
	coordinate CommittedCoordinate
	policy     MirrorPolicy
	receipts   []json.RawMessage
	callsMu    sync.Mutex
	calls      map[string]int
	exchanges  []repair141MirrorExchange
}

func repair141OpenMirror(t *testing.T) *repair141MirrorFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/repair141/mirror-transcript.json")
	if err != nil {
		t.Fatal(err)
	}
	var transcript struct {
		Coordinate CommittedCoordinate       `json:"coordinate"`
		Commitment string                    `json:"commitment"`
		Exchanges  []repair141MirrorExchange `json:"exchanges"`
	}
	if err := json.Unmarshal(data, &transcript); err != nil {
		t.Fatal(err)
	}
	commitment, err := fixedDigest(transcript.Commitment)
	if err != nil {
		t.Fatal(err)
	}
	state := &repair141MirrorFixture{coordinate: transcript.Coordinate, policy: MirrorPolicy{Kind: MirrorExact, Candidates: []MirrorCandidate{{Source: 0, Commitment: commitment}}}, calls: map[string]int{}, exchanges: transcript.Exchanges}
	data, err = os.ReadFile("../../../services/interop/crates/layerx-mirror/tests/fixtures/committed_blocks.json")
	if err != nil {
		t.Fatal(err)
	}
	var producer struct {
		Receipts []json.RawMessage `json:"receipts"`
	}
	if err := json.Unmarshal(data, &producer); err != nil {
		t.Fatal(err)
	}
	state.receipts = producer.Receipts
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/verify" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("mirror transport changed HTTP request")
			http.Error(w, "request", 400)
			return
		}
		var actual any
		if err := json.NewDecoder(r.Body).Decode(&actual); err != nil {
			t.Error(err)
			http.Error(w, "JSON", 400)
			return
		}
		for _, exchange := range state.exchanges {
			var expected any
			if err := json.Unmarshal(exchange.Request, &expected); err != nil {
				t.Error(err)
				return
			}
			if reflect.DeepEqual(actual, expected) {
				key := string(exchange.Request)
				state.callsMu.Lock()
				state.calls[key]++
				state.callsMu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(exchange.Response)
				return
			}
		}
		t.Errorf("mirror request differs from verify transcript: %v", actual)
		http.Error(w, "unrecorded request", 400)
	}))
	t.Cleanup(server.Close)
	state.verifier, err = NewMirrorHTTPVerifier(server.URL+"/verify", server.Client(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestRepair_141_mirror_request_shape(t *testing.T) {
	fixture := repair141OpenMirror(t)
	for n, receiptJSON := range fixture.receipts {
		verified, err := fixture.verifier.VerifyReceipt(context.Background(), fixture.coordinate, fixture.policy, uint32(n))
		if err != nil {
			t.Fatalf("receipt index %d: %v", n, err)
		}
		var view struct {
			Receipt string `json:"receipt"`
		}
		if err := json.Unmarshal(receiptJSON, &view); err != nil {
			t.Fatal(err)
		}
		canonical := sequencerFixtureBytes(t, view.Receipt)
		decoded, err := decodeReceiptProfile(canonical, false)
		if err != nil {
			t.Fatal(err)
		}
		digest := domainDigest([]byte(ReceiptDomain+"\x00"), encodeSequencerReceipt(decoded.protocol, false))
		if verified.EvidenceDigest == nil || *verified.EvidenceDigest != digest {
			t.Fatalf("mirror did not select the requested archived receipt %d", n)
		}
	}
	if _, err := fixture.verifier.VerifyReceipt(context.Background(), fixture.coordinate, fixture.policy, maxMirrorReceiptIndex); err == nil {
		t.Fatal("out-of-bounds receipt index reached transport")
	}
	if result, err := fixture.verifier.VerifyReceipt(context.Background(), fixture.coordinate, fixture.policy, uint32(len(fixture.receipts))); err == nil || result.Level != "" {
		t.Fatal("missing archive receipt was accepted")
	}
	if _, err := fixture.verifier.VerifyCommitted(context.Background(), fixture.coordinate, fixture.policy); err != nil {
		t.Fatal(err)
	}
	fixture.callsMu.Lock()
	defer fixture.callsMu.Unlock()
	for _, exchange := range fixture.exchanges {
		if fixture.calls[string(exchange.Request)] != 1 {
			t.Fatal("both mirror request variants and each receipt index must match the transcript exactly once")
		}
	}
}

func TestRepair_141_mirror_response_shape(t *testing.T) {
	fixture := repair141OpenMirror(t)
	committed, err := fixture.verifier.VerifyCommitted(context.Background(), fixture.coordinate, fixture.policy)
	if err != nil {
		t.Fatal(err)
	}
	if committed.Level != "Committed" || committed.CommittedHeight != fixture.coordinate.Height || committed.CommittedBlockHash != fixture.coordinate.BlockHash || committed.EvidenceDigest != nil || committed.SourceID != "repair141-ethereum" || committed.AgreeingSources != 1 || committed.FailoverCount != 0 || committed.Provenance != "Canonical" || committed.LatestBatch != nil || committed.BatchLag != "Unknown" || !strings.HasPrefix(committed.Target, "ethereum:") || !strings.HasPrefix(committed.CanonicalPosition, "ethereum:block=50;") {
		t.Fatalf("SDK invented or lost fields from committed CLI report: %+v", committed)
	}
	receipt, err := fixture.verifier.VerifyReceipt(context.Background(), fixture.coordinate, fixture.policy, 0)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Level != committed.Level || receipt.CommittedHeight != committed.CommittedHeight || receipt.CommittedBlockHash != committed.CommittedBlockHash || receipt.EvidenceDigest == nil {
		t.Fatal("receipt response lost committed coordinate or evidence digest")
	}
	changed := fixture.coordinate
	changed.BlockHash[0] ^= 1
	if value, err := fixture.verifier.VerifyCommitted(context.Background(), changed, fixture.policy); err == nil || value.Level != "" {
		t.Fatal("CLI success for a different committed coordinate accepted")
	}
	changed = fixture.coordinate
	changed.Height++
	// Reuse the successful recorded response at a different requested height.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fixture.exchanges[len(fixture.exchanges)-2].Response)
	}))
	defer server.Close()
	verifier, err := NewMirrorHTTPVerifier(server.URL, server.Client(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if value, err := verifier.VerifyCommitted(context.Background(), changed, fixture.policy); err == nil || value.Level != "" {
		t.Fatal("CLI success for a different height accepted")
	}
}

func TestRepair_141_mirror_http_bounds(t *testing.T) {
	fixture := repair141OpenMirror(t)
	for _, endpoint := range []string{"http://example.com/verify", "https://user:pass@example.com/verify", "file:///verify", "https://example.com/#fragment"} {
		if _, err := NewMirrorHTTPVerifier(endpoint, nil, time.Second); err == nil {
			t.Fatalf("unprotected or malformed mirror endpoint accepted: %s", endpoint)
		}
	}
	for _, timeout := range []time.Duration{0, 99 * time.Millisecond, 121 * time.Second} {
		if _, err := NewMirrorHTTPVerifier("https://example.com/verify", nil, timeout); err == nil {
			t.Fatal("mirror timeout bound was ignored")
		}
	}
	for _, name := range []string{"oversized", "redirect", "status", "malformed", "receipt-without-digest", "committed-with-digest", "legacy-level"} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch name {
				case "oversized":
					_, _ = w.Write([]byte(strings.Repeat("x", maxMirrorResponseBytes+1)))
				case "redirect":
					w.Header().Set("Location", fixture.verifier.endpoint)
					w.WriteHeader(http.StatusTemporaryRedirect)
				case "status":
					w.WriteHeader(http.StatusServiceUnavailable)
				case "malformed":
					_, _ = w.Write([]byte(`{"ok":true,"verification":{}}`))
				default:
					var report map[string]any
					data := fixture.exchanges[len(fixture.exchanges)-2].Response
					if err := json.Unmarshal(data, &report); err != nil {
						t.Error(err)
						return
					}
					value := report["verification"].(map[string]any)
					if name == "committed-with-digest" {
						value["evidenceDigest"] = strings.Repeat("11", 32)
					}
					if name == "legacy-level" {
						value["level"] = "receipted"
					}
					_ = json.NewEncoder(w).Encode(report)
				}
			}))
			defer server.Close()
			verifier, err := NewMirrorHTTPVerifier(server.URL, server.Client(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			var result MirrorVerification
			if name == "receipt-without-digest" {
				result, err = verifier.VerifyReceipt(context.Background(), fixture.coordinate, fixture.policy, 0)
			} else {
				result, err = verifier.VerifyCommitted(context.Background(), fixture.coordinate, fixture.policy)
			}
			var refusal MirrorVerificationError
			if !errors.As(err, &refusal) || result.Level != "" {
				t.Fatalf("untrusted report accepted: %+v %v", result, err)
			}
			want := MirrorMalformed
			if name == "oversized" {
				want = MirrorBounds
			}
			if name == "redirect" || name == "status" {
				want = MirrorUnavailable
			}
			if refusal.Code != want {
				t.Fatalf("refusal %s, want %s", refusal.Code, want)
			}
		})
	}
	fixture.callsMu.Lock()
	defer fixture.callsMu.Unlock()
	if len(fixture.calls) != 0 {
		t.Fatal("mirror HTTP redirect followed before verification")
	}
}
