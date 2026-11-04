package layerx

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRepairM_1385_program_corpus_third_shape(t *testing.T) {
	t.Run("public-native-rpc-corpus", TestProgramTerminalV5NativeCorpus)
	t.Run("terminal-codec", TestCanonicalProgramTerminalBindsResponseUsageAndGraph)
	t.Run("terminal-wrapper-refusal", TestProgramTerminalWrapperOrderAndUniquenessAreClosed)
	t.Run("outcome-codec", TestProgramOutcomeV3Vector)
}

func TestRepairM_1385_secret_callback_deadlock(t *testing.T) {
	input := []byte("retained-secret-material")
	secret, err := NewSecretBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	defer secret.Destroy()
	stored := secret.value
	failure := errors.New("consumer cleanup")
	var outer, inner []byte
	finished := make(chan error, 1)
	go func() {
		finished <- secret.Expose(func(borrow []byte) error {
			outer = borrow
			if !bytes.Equal(borrow, input) {
				return errors.New("initial borrow changed")
			}
			if err := secret.Expose(func(nested []byte) error {
				inner = nested
				nested[0] ^= 1
				if !bytes.Equal(borrow, input) {
					return errors.New("nested borrow aliases outer")
				}
				return nil
			}); err != nil {
				return err
			}
			secret.Destroy()
			secret.Destroy()
			if !bytes.Equal(stored, make([]byte, len(stored))) {
				return errors.New("Destroy did not wipe storage")
			}
			if !bytes.Equal(borrow, input) {
				return errors.New("Destroy changed active borrow")
			}
			if err := secret.Expose(func([]byte) error { return errors.New("destroyed callback ran") }); err == nil {
				return errors.New("Destroy admitted a new borrow")
			}
			return failure
		})
	}()
	select {
	case err := <-finished:
		if !errors.Is(err, failure) {
			t.Fatalf("callback result lost: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("callback re-entry or destruction deadlocked")
	}
	for _, value := range [][]byte{stored, outer, inner} {
		if len(value) == 0 || !bytes.Equal(value, make([]byte, len(value))) {
			t.Fatal("secret copy was not zeroized")
		}
	}
	if string(input) != "retained-secret-material" || secret.value != nil || !secret.destroyed {
		t.Fatal("secret ownership or destruction state changed")
	}
	if err := secret.Expose(nil); err == nil {
		t.Fatal("nil consumer accepted")
	}
	var missing *SecretBytes
	if err := missing.Expose(func([]byte) error { return nil }); err == nil {
		t.Fatal("nil secret accepted")
	}

	t.Run("panic-wipes-borrow", func(t *testing.T) {
		secret, err := NewSecretBytes(input)
		if err != nil {
			t.Fatal(err)
		}
		defer secret.Destroy()
		var retained []byte
		func() {
			defer func() {
				if recover() != failure {
					t.Fatal("consumer panic was swallowed or changed")
				}
			}()
			_ = secret.Expose(func(value []byte) error { retained = value; panic(failure) })
		}()
		if !bytes.Equal(retained, make([]byte, len(input))) {
			t.Fatal("panic leaked borrow")
		}
		if err := secret.Expose(func(value []byte) error {
			if !bytes.Equal(value, input) {
				return errors.New("callback mutation changed storage")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestRepairM_1385_secret_concurrent_cleanup(t *testing.T) {
	const borrowers = 32
	input := []byte("parallel-secret-material")
	secret, err := NewSecretBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	defer secret.Destroy()
	stored := secret.value
	entered := make(chan []byte, borrowers)
	release := make(chan struct{})
	defer close(release)
	results := make(chan error, borrowers)
	for n := 0; n < borrowers; n++ {
		go func() {
			results <- secret.Expose(func(value []byte) error {
				entered <- value
				<-release
				if !bytes.Equal(value, input) {
					return errors.New("cleanup invalidated active borrow")
				}
				value[0] ^= 1
				return nil
			})
		}()
	}
	var borrows [][]byte
	for n := 0; n < borrowers; n++ {
		select {
		case borrow := <-entered:
			borrows = append(borrows, borrow)
		case <-time.After(3 * time.Second):
			t.Fatal("concurrent Expose serialized callbacks or deadlocked")
		}
	}
	cleaned := make(chan struct{})
	go func() {
		var workers sync.WaitGroup
		for n := 0; n < borrowers; n++ {
			workers.Add(1)
			go func() { defer workers.Done(); secret.Destroy() }()
		}
		workers.Wait()
		close(cleaned)
	}()
	select {
	case <-cleaned:
	case <-time.After(3 * time.Second):
		t.Fatal("concurrent Destroy waited for callbacks")
	}
	if secret.value != nil || !secret.destroyed || !bytes.Equal(stored, make([]byte, len(stored))) {
		t.Fatal("concurrent Destroy did not wipe storage")
	}
	if err := secret.Expose(func([]byte) error { t.Error("callback entered after Destroy"); return nil }); err == nil {
		t.Fatal("new borrow admitted after cleanup")
	}
	// All callbacks may finish independently of the destroyed owner.
	for n := 0; n < borrowers; n++ {
		release <- struct{}{}
	}
	for n := 0; n < borrowers; n++ {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("borrow did not finish after cleanup")
		}
	}
	for _, borrow := range borrows {
		if !bytes.Equal(borrow, make([]byte, len(input))) {
			t.Fatal("completed concurrent borrow retained material")
		}
	}
}

func TestRepairM_1385_stream_unbounded_history(t *testing.T) {
	if maximumStreamReplayCursors != 256 {
		t.Fatal("acknowledged replay window must remain bounded to 256 cursors")
	}
	data, err := os.ReadFile("../../../services/human/schema/human-api/golden/stream.next.response.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Body struct {
			Result StreamPage `json:"result"`
			Trace  string     `json:"trace"`
		} `json:"body"`
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if len(golden.Body.Result.Events) != 2 {
		t.Fatal("recorded Human producer page missing")
	}
	const pages, pageSize, start = 2048, 4, 42
	cursorText := func(n int) string { return fmt.Sprintf("cur-%018d", n) }
	// The loopback peer keeps a finite event store with the recorded Human event
	// bodies. Production HTTP, HumanStreamSource and ResumableStream handle every page.
	store := make([]StreamEvent, pages*pageSize)
	for n := range store {
		store[n] = golden.Body.Result.Events[n%2]
		store[n].Cursor = cursorText(start + n + 1)
	}
	var requests atomic.Int64
	var mu sync.Mutex
	var override *StreamPage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/v1/stream/") {
			t.Error("unexpected Human stream request")
			http.Error(w, "route", 400)
			return
		}
		text := strings.TrimPrefix(r.URL.Path, "/v1/stream/")
		n, err := strconv.Atoi(strings.TrimPrefix(text, "cur-"))
		if err != nil || n < start || n > start+len(store) {
			t.Error("unknown checkpoint")
			http.Error(w, "cursor", 400)
			return
		}
		mu.Lock()
		page := StreamPage{Events: []StreamEvent{}, NextCursor: text}
		if override != nil {
			page = *override
		} else if n-start < len(store) {
			page.Events = store[n-start : n-start+pageSize]
			page.NextCursor = page.Events[len(page.Events)-1].Cursor
		}
		mu.Unlock()
		if err := json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": page, "trace": golden.Body.Trace}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
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
	initial, _ := NewStreamCursor(cursorText(start))
	stream, err := NewResumableStream(initial, source)
	if err != nil {
		t.Fatal(err)
	}
	for p := 0; p < pages; p++ {
		before := stream.Cursor()
		events, err := stream.Next(context.Background())
		if err != nil || len(events) != pageSize || stream.Cursor() != before {
			t.Fatalf("page %d lost checkpoint: %v", p, err)
		}
		count := requests.Load()
		replay, err := stream.Next(context.Background())
		if err != nil || !reflect.DeepEqual(replay, events) || requests.Load() != count {
			t.Fatal("pending page was refetched or changed")
		}
		last, _ := NewStreamCursor(events[len(events)-1].Cursor)
		if err := stream.Acknowledge(last); err == nil || stream.Cursor() != before {
			t.Fatal("out-of-order checkpoint accepted")
		}
		for n, event := range events {
			if event.Cursor != store[p*pageSize+n].Cursor {
				t.Fatal("cursor gap or duplicate")
			}
			cursor, _ := NewStreamCursor(event.Cursor)
			if err := stream.Acknowledge(cursor); err != nil {
				t.Fatal(err)
			}
			if len(stream.seen) > 256 || len(stream.replay) != 256 || len(stream.pending) != pageSize-n-1 {
				t.Fatal("retained stream state exceeded its bound")
			}
			if n == 0 {
				pending, err := stream.Next(context.Background())
				if err != nil || !reflect.DeepEqual(pending, events[1:]) || requests.Load() != count {
					t.Fatal("partial acknowledgment lost pending suffix")
				}
			}
		}
	}
	if requests.Load() != pages || len(stream.seen) != 256 || stream.pending != nil || stream.Cursor().String() != store[len(store)-1].Cursor {
		t.Fatal("long-running stream did not retain a fixed acknowledged window")
	}
	if _, retained := stream.seen[store[0].Cursor]; retained {
		t.Fatal("old acknowledged cursor retained forever")
	}
	if events, err := stream.Next(context.Background()); err != nil || len(events) != 0 {
		t.Fatalf("empty page failed: %v", err)
	}
	checkpoint, fetched := stream.Cursor(), stream.fetched
	next := StreamEvent{Cursor: cursorText(start + len(store) + 1), Kind: store[0].Kind, ObservedAt: store[0].ObservedAt}
	for name, page := range map[string]StreamPage{
		"recent-replay":     {Events: []StreamEvent{store[len(store)-2]}, NextCursor: store[len(store)-2].Cursor},
		"checkpoint-replay": {Events: []StreamEvent{store[len(store)-1]}, NextCursor: checkpoint.String()},
		"page-duplicate":    {Events: []StreamEvent{next, next}, NextCursor: next.Cursor},
		"wrong-next":        {Events: []StreamEvent{next}, NextCursor: checkpoint.String()},
		"invalid-cursor":    {Events: []StreamEvent{{Cursor: "bad\x00cursor"}}, NextCursor: "bad\x00cursor"},
	} {
		t.Run(name, func(t *testing.T) {
			mu.Lock()
			override = &page
			mu.Unlock()
			if events, err := stream.Next(context.Background()); err == nil || events != nil || programErrorCode(err) != ErrorDecodeFailure {
				t.Fatalf("invalid page accepted: %+v %v", events, err)
			}
			if stream.Cursor() != checkpoint || stream.fetched != fetched || len(stream.pending) != 0 || len(stream.seen) != 256 {
				t.Fatal("invalid page changed stream state")
			}
		})
	}
	t.Run("checkpoint-recovery", TestRepair_141_stream_checkpoint_before_consumption)
}

func TestRepairM_1385_exported_kernel_lifecycle_verifier(t *testing.T) {
	// The control is an unchanged, correctly signed deploy receipt from layerxd.
	data, err := os.ReadFile("../../../services/platform/hosted/authority/tests/fixtures/real-program-deploy-receipt.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Receipt string `json:"receipt_hex"`
		Key     string `json:"sequencer_public_key_hex"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	canonical, key := fixtureBytes(t, fixture.Receipt), fixture32(t, fixture.Key)
	decoded, err := decodeProtocolReceipt(canonical)
	if err != nil {
		t.Fatal(err)
	}
	receipt := decoded.protocol
	if receipt.ProtocolVersion != 3 || receipt.ModuleID != 9 || receipt.ModuleVersion != 4 || receipt.Operation != 0 || receipt.ProgramOutcome != nil {
		t.Fatal("control is not a legacy lifecycle receipt")
	}
	digest := domainDigest(receiptDomain, decoded.unsigned)
	if !ed25519.Verify(key[:], digest[:], decoded.signature[:]) {
		t.Fatal("recorded deploy signature is invalid")
	}
	verified, err := VerifyProgramLifecycleReceipt(canonical, receipt.ActivityID, key)
	var refusal *VerificationError
	if !errors.As(err, &refusal) || !errors.Is(err, ErrVerification) || refusal.Check != ReceiptCheckReceiptShape || !reflect.DeepEqual(verified, VerifiedReceipt{}) {
		t.Fatalf("retired signed lifecycle receipt accepted: %+v %v", verified, err)
	}
	legacy, err := json.Marshal(map[string]string{"state": "executed", "activity_id": fmt.Sprintf("%x", receipt.ActivityID), "receipt": fixture.Receipt, "terminal_payload": "", "call_graph": ""})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := decodeLifecycleSubmission(legacy, receipt.ActivityID, key); programErrorCode(err) != ErrorUnknownOutcome || !reflect.DeepEqual(got, VerifiedReceipt{}) {
		t.Fatal("legacy lifecycle decoder returned success")
	}
	t.Run("call-outcome-refusal", TestLifecycleVerifierRejectsCallOutcomeReceipt)
	t.Run("current-verification-levels", TestVerificationRungsAreReceiptedAndCommittedOnly)
	// The native lifecycle consumer resolves its signed EVM hash through Activity;
	// the common receipt path authenticates the original producer bytes and pins.
	for _, vector := range sequencerReceiptCases(t) {
		if vector.Vector != "V1" && vector.Vector != "V2" {
			continue
		}
		t.Run("native-"+vector.ID, func(t *testing.T) {
			raw, keys := sequencerFixtureBytes(t, vector.RawTransaction), sequencerFixtureKeys(t, vector)
			programs := committedRecordedPrograms(t, vector.ReceiptJSON, raw, keys)
			request := NativeProgramLifecycleRequest{TransactionHash: receiptKeccak256(raw)}
			got, err := programs.LifecycleReceipt(context.Background(), request)
			if err != nil || got.Level != "receipted" || got.Receipt.ActivityID != request.TransactionHash || !bytes.Equal(got.CanonicalBytes, sequencerFixtureBytes(t, vector.SignedReceipt)) {
				t.Fatalf("native lifecycle receipt: %+v %v", got, err)
			}
			keys.Pins = append([]ReceiptKey(nil), keys.Pins...)
			keys.Pins[0].PublicKey[0] ^= 1
			untrusted := committedRecordedPrograms(t, vector.ReceiptJSON, raw, keys)
			if got, err := untrusted.LifecycleReceipt(context.Background(), request); err == nil || !reflect.DeepEqual(got, VerifiedReceipt{}) {
				t.Fatal("native lifecycle accepted foreign key")
			}
			if got, err := programs.LifecycleReceipt(context.Background(), NativeProgramLifecycleRequest{}); programErrorCode(err) != ErrorInvalidArgument || got.Level != "" {
				t.Fatal("native lifecycle accepted missing transaction identity")
			}
		})
	}
}
