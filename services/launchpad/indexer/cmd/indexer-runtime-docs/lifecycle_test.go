package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
)

func recordedCurrentAnchor(t *testing.T) (json.RawMessage, uint64, common.Hash) {
	t.Helper()
	body, err := os.ReadFile("../../internal/receipt/testdata/repair53-producer.json")
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Responses map[string]struct{ Result json.RawMessage }
	}
	if err = json.Unmarshal(body, &packet); err != nil {
		t.Fatal(err)
	}
	height := deployment.Current().ActivationHeight + 1
	key := fmt.Sprintf(`eth_getBlockByNumber ["0x%x",false]`, height)
	raw := packet.Responses[key].Result
	var block struct {
		Number string
		Hash   common.Hash
	}
	if err = json.Unmarshal(raw, &block); err != nil || block.Number != fmt.Sprintf("0x%x", height) || block.Hash == (common.Hash{}) {
		t.Fatalf("current anchor %+v %v", block, err)
	}
	return raw, height, block.Hash
}
func invokeRuntimeDocuments(t *testing.T, args ...string) error {
	t.Helper()
	original, argv := flag.CommandLine, os.Args
	defer func() { flag.CommandLine = original; os.Args = argv }()
	flag.CommandLine = flag.NewFlagSet("indexer-runtime-docs", flag.ContinueOnError)
	os.Args = append([]string{"indexer-runtime-docs"}, args...)
	return run()
}
func TestRuntimeDocumentsCommandWritesBoundCurrentDocuments(t *testing.T) {
	raw, height, hash := recordedCurrentAnchor(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			ID     json.RawMessage
			Method string
			Params []json.RawMessage
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			t.Error(err)
			return
		}
		if q.Method != "eth_getBlockByNumber" || len(q.Params) != 2 || string(q.Params[0]) != fmt.Sprintf(`"0x%x"`, height) || string(q.Params[1]) != "false" {
			t.Errorf("anchor request %+v", q)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": raw}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source location absent")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
	out := filepath.Join(t.TempDir(), "documents")
	args := []string{"-repo", root, "-start-height", fmt.Sprint(height + 1), "-anchor-rpc", server.URL, "-max-delivery-lag-ms", "60000", "-out", out}
	if err := invokeRuntimeDocuments(t, args...); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"receipt.json", "delivery.json", "lifecycle.json"} {
		body, err := os.ReadFile(filepath.Join(out, name))
		if err != nil || len(body) == 0 {
			t.Fatalf("document %s %v", name, err)
		}
		var document map[string]json.RawMessage
		if err = json.Unmarshal(body, &document); err != nil {
			t.Fatal(err)
		}
		if name == "delivery.json" && string(document["maxDeliveryLagMs"]) != "60000" {
			t.Fatalf("delivery lag %s", document["maxDeliveryLagMs"])
		}
		if name == "receipt.json" && !strings.Contains(string(body), hash.Hex()) {
			t.Fatal("receipt document lost recorded anchor")
		}
	}
	for _, tc := range []struct {
		name    string
		prepare func() string
	}{
		{"output collision", func() string {
			path := filepath.Join(t.TempDir(), "occupied")
			if err := os.WriteFile(path, []byte("retained"), 0600); err != nil {
				t.Fatal(err)
			}
			return path
		}},
		{"document collision", func() string {
			path := t.TempDir()
			if err := os.Mkdir(filepath.Join(path, "delivery.json"), 0700); err != nil {
				t.Fatal(err)
			}
			return path
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := append([]string(nil), args...)
			next[len(next)-1] = tc.prepare()
			if err := invokeRuntimeDocuments(t, next...); err == nil {
				t.Fatal("output collision accepted")
			}
		})
	}
	if err := invokeRuntimeDocuments(t, "-max-delivery-lag-ms", "60000"); err == nil {
		t.Fatal("missing source anchor accepted")
	}
	for _, artifact := range []string{"missing", "{", `{"deployedBytecode":"0xzz"}`, `{"deployedBytecode":"0x"}`} {
		t.Run("artifact "+artifact, func(t *testing.T) {
			repo := t.TempDir()
			reviewed, target := inputPaths(repo)
			if err := os.MkdirAll(filepath.Dir(reviewed), 0700); err != nil {
				t.Fatal(err)
			}
			original, _ := inputPaths(root)
			body, err := os.ReadFile(original)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(reviewed, body, 0600); err != nil {
				t.Fatal(err)
			}
			if artifact != "missing" {
				if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(target, []byte(artifact), 0600); err != nil {
					t.Fatal(err)
				}
			}
			next := append([]string(nil), args...)
			next[1] = repo
			if err = invokeRuntimeDocuments(t, next...); err == nil {
				t.Fatal("invalid compiler artifact admitted")
			}
		})
	}
	next := append([]string(nil), args...)
	next[1] = t.TempDir()
	if err := invokeRuntimeDocuments(t, next...); err == nil {
		t.Fatal("missing catalog admitted")
	}
}
func TestReportedAnchorRejectsMalformedAndUnavailableResponses(t *testing.T) {
	raw, height, hash := recordedCurrentAnchor(t)
	for _, body := range []string{"{", `{"result":null}`, `{"error":{"message":"pruned"}}`, `{"result":{"number":"0x1","hash":"` + hash.Hex() + `"}}`, `{"result":{"number":"` + fmt.Sprintf("0x%x", height) + `","hash":"` + (common.Hash{}).Hex() + `"}}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			if _, err := reportedBlockHash(context.Background(), server.URL, height); err == nil {
				t.Fatal("invalid anchor response admitted")
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{"result": raw}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	if got, err := reportedBlockHash(context.Background(), server.URL, height); err != nil || got != hash {
		t.Fatalf("recorded anchor %s %v", got, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reportedBlockHash(canceled, server.URL, height); err == nil {
		t.Fatal("canceled anchor admitted")
	}
	if _, err := reportedBlockHash(context.Background(), ":", height); err == nil {
		t.Fatal("invalid anchor URL admitted")
	}
}
