package chainfixture

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRecordedTrafficRetainsRepliesAndPersistsNewCalls(t *testing.T) {
	t.Setenv(RecordEnv, "")
	original := New(t)
	if original.Recording() || len(original.entries) == 0 {
		t.Fatal("recorded traffic unavailable")
	}
	endpoint := original.Serve(t)
	downstream := &Fixture{entries: map[string]recorded{}, upstream: endpoint, path: filepath.Join(t.TempDir(), "rpc.jsonl.gz")}
	if !downstream.Recording() {
		t.Fatal("recording peer not identified")
	}
	call := rpcCall{ID: json.RawMessage("7"), Method: "eth_chainId", Params: []json.RawMessage{}}
	result, rpcError, err := downstream.answer(call)
	if err != nil || len(rpcError) != 0 || string(result) != `"0x7d"` {
		t.Fatalf("chain identity reply %s %s %v", result, rpcError, err)
	}
	if !downstream.dirty || len(downstream.entries) != 1 {
		t.Fatal("new response not retained")
	}
	if err = downstream.save(); err != nil {
		t.Fatal(err)
	}
	if downstream.dirty {
		t.Fatal("saved recording remains dirty")
	}
	// A second recording includes the persisted calls from the same snapshot.
	another := &Fixture{entries: map[string]recorded{}, path: downstream.path, dirty: true}
	if err = another.save(); err != nil || len(another.entries) != 1 {
		t.Fatalf("recording merge %d %v", len(another.entries), err)
	}
	file, err := os.Open(another.path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var persisted recorded
	if err = json.NewDecoder(reader).Decode(&persisted); err != nil || persisted.Key != Key(call.Method, call.Params) || !bytes.Equal(persisted.Result, result) {
		t.Fatalf("persisted traffic %+v %v", persisted, err)
	}
	downstream.Mutate = func(key string, body json.RawMessage) json.RawMessage {
		if key != persisted.Key {
			t.Errorf("mutated a different call %s", key)
		}
		return append(json.RawMessage(nil), body...)
	}
	reply, _, err := downstream.answer(call)
	if err != nil || !bytes.Equal(reply, result) {
		t.Fatalf("recorded mutation hook %s %v", reply, err)
	}
	raw, err := json.Marshal(call)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	original.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw)))
	var envelope struct {
		ID     int
		Result string
	}
	if err = json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || response.Code != 200 || envelope.ID != 7 || envelope.Result != "0x7d" {
		t.Fatalf("HTTP reply %d %+v %v", response.Code, envelope, err)
	}
}
func TestRecordedTrafficRejectsMissingAndMalformedRequests(t *testing.T) {
	t.Setenv(RecordEnv, "")
	fixture := New(t)
	response := httptest.NewRecorder()
	fixture.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{`)))
	if response.Code != 400 {
		t.Fatalf("malformed request status %d", response.Code)
	}
	response = httptest.NewRecorder()
	fixture.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"id":1,"method":"unrecorded","params":[]}`)))
	if response.Code != 502 || len(fixture.missing) != 1 {
		t.Fatalf("missing request %d %v", response.Code, fixture.missing)
	}
	file, err := os.CreateTemp(t.TempDir(), "request")
	if err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	fixture.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", file))
	if response.Code != 400 {
		t.Fatalf("unreadable request status %d", response.Code)
	}
	if got := Key("call", []json.RawMessage{json.RawMessage(`{`)}); got != `call ["{"]` {
		t.Fatalf("malformed argument key %s", got)
	}
	for key, entry := range fixture.entries {
		if len(entry.Error) > 0 {
			var call rpcCall
			method, params, ok := bytes.Cut([]byte(key), []byte(" "))
			if !ok {
				t.Fatal("recorded key malformed")
			}
			call.Method = string(method)
			if err = json.Unmarshal(params, &call.Params); err != nil {
				t.Fatal(err)
			}
			_, rpcError, err := fixture.answer(call)
			if err != nil || !bytes.Equal(rpcError, entry.Error) {
				t.Fatalf("recorded RPC error %s %v", rpcError, err)
			}
			break
		}
	}
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{`) }))
	defer local.Close()
	forward := &Fixture{entries: map[string]recorded{}, upstream: local.URL}
	if _, _, err = forward.answer(rpcCall{Method: "eth_chainId"}); err == nil {
		t.Fatal("malformed upstream accepted")
	}
	fixture.path = filepath.Join(file.Name(), "recording")
	fixture.dirty = true
	if err = fixture.save(); err == nil {
		t.Fatal("recording persisted through a regular file")
	}
}
