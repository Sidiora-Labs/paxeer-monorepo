package mirror

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/ethereum/go-ethereum/rpc"
)

type repairM110Key struct {
	ID     string `json:"keyId"`
	Public string `json:"publicKey"`
	First  uint64 `json:"firstHeight"`
	Last   uint64 `json:"lastHeight"`
}

type repairM110Pin struct {
	ID     string `json:"sequencer_id_hex"`
	Public string `json:"public_key_hex"`
	First  uint64 `json:"first_height"`
	Last   uint64 `json:"last_height"`
}

type repairM110Endpoint struct {
	URL     string `json:"url"`
	Backend string `json:"independent_backend"`
	CA      string `json:"ca_certificate_der"`
	Token   string `json:"bearer_token_file"`
}

type repairM110Target struct {
	RPC struct {
		Endpoints []repairM110Endpoint `json:"endpoints"`
		Quorum    int                  `json:"quorum"`
	} `json:"rpc"`
	Keys []repairM110Pin `json:"receipt_keys"`
}

type repairM110Config struct {
	Paxeer   repairM110Target  `json:"paxeer"`
	Ethereum repairM110Target  `json:"ethereum"`
	Solana   *repairM110Target `json:"solana"`
}

type repairM110Policy struct {
	Name   string          `json:"name"`
	Input  json.RawMessage `json:"input"`
	Mirror struct {
		Keys  []repairM110Pin `json:"keys"`
		Error string          `json:"error"`
	} `json:"mirror"`
	Gateway struct {
		Keys  []repairM110Key `json:"keys"`
		Error string          `json:"error"`
	} `json:"gateway"`
}

type repairM110Transcript struct {
	Hashes      map[string]string  `json:"source_hashes"`
	Renderings  []repairM110Config `json:"renderings"`
	GatewayKeys []repairM110Key    `json:"gateway_keys"`
	Gateway     struct {
		Supported struct {
			Signers map[string][]string `json:"signers"`
		} `json:"x402_supported"`
	} `json:"gateway"`
	Policies []repairM110Policy `json:"policies"`
	Refusals []struct {
		URL   string          `json:"url"`
		Error string          `json:"error"`
		RPC   json.RawMessage `json:"rpc"`
	} `json:"endpoint_refusals"`
}

func repairM110Read(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func repairM110JSON(t *testing.T, data []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}

func repairM110Record(t *testing.T) repairM110Transcript {
	t.Helper()
	var record repairM110Transcript
	repairM110JSON(t, repairM110Read(t, "testdata/repair_m110/rendered.json"), &record)
	if len(record.Hashes) != 8 || len(record.Renderings) != 2 {
		t.Fatal("missing production provenance or Ethereum-only/Solana rendering")
	}
	for name, want := range record.Hashes {
		digest := sha256.Sum256(repairM110Read(t, filepath.Join("../../../..", name)))
		if hex.EncodeToString(digest[:]) != want {
			t.Fatalf("%s changed since recording production outputs; regenerate the transcript", name)
		}
	}
	return record
}

func repairM110Pins(keys []repairM110Key) []repairM110Pin {
	pins := make([]repairM110Pin, len(keys))
	for i, key := range keys {
		pins[i] = repairM110Pin{strings.ToLower(strings.TrimPrefix(key.ID, "0x")), strings.ToLower(strings.TrimPrefix(key.Public, "0x")), key.First, key.Last}
	}
	return pins
}

func TestRepairM_110_MirrorExampleRPC(t *testing.T) {
	record := repairM110Record(t)
	var example repairM110Config
	repairM110JSON(t, repairM110Read(t, "config.example.json"), &example)
	for index, config := range record.Renderings {
		if (config.Solana != nil) != (index == 1) {
			t.Fatal("wrong Solana presence")
		}
		pairs := [][2]repairM110Target{{example.Paxeer, config.Paxeer}, {example.Ethereum, config.Ethereum}}
		if config.Solana != nil {
			pairs = append(pairs, [2]repairM110Target{*example.Solana, *config.Solana})
		}
		for chain, pair := range pairs {
			if len(pair[0].RPC.Endpoints) != 3 || !reflect.DeepEqual(pair[0].RPC, pair[1].RPC) {
				t.Fatalf("chain %d changed example endpoints, TLS/token paths, independent backends or quorum", chain)
			}
			for _, endpoint := range pair[1].RPC.Endpoints {
				parsed, err := url.Parse(endpoint.URL)
				if err != nil {
					t.Fatal(err)
				}
				want := "/rpc"
				if chain == 0 {
					want = "/"
				}
				if parsed.Scheme != "https" || parsed.Host == "" || parsed.Path != want || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
					t.Fatalf("invalid rendered chain %d endpoint: %s", chain, endpoint.URL)
				}
			}
		}
	}
	if len(record.Refusals) != 6 {
		t.Fatal("missing retired/decorated Paxeer URL refusals")
	}
	for _, refusal := range record.Refusals {
		if !strings.Contains(refusal.Error, "root JSON-RPC URL") || len(refusal.RPC) != 0 {
			t.Fatalf("production renderer accepted invalid Paxeer URL %s: %+v", refusal.URL, refusal)
		}
	}

	// The node uses this production HTTP JSON-RPC server and metadata service.
	handler := rpc.NewServer()
	defer handler.Stop()
	type request struct{ Host, Path, Method string }
	requests := make(chan request, 6)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- request{r.Host, r.URL.RequestURI(), r.Method}
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	loopback, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := server.Client().Transport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, loopback.Host)
	}
	transport.TLSClientConfig.ServerName = loopback.Hostname()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	for _, config := range record.Renderings {
		for _, endpoint := range config.Paxeer.RPC.Endpoints {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			connection, err := rpc.DialOptions(ctx, endpoint.URL, rpc.WithHTTPClient(client))
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			var modules map[string]string
			err = connection.CallContext(ctx, &modules, "rpc_modules")
			connection.Close()
			cancel()
			if err != nil {
				t.Fatalf("rendered URL failed real node root RPC: %v", err)
			}
			if !reflect.DeepEqual(modules, map[string]string{"rpc": "1.0"}) {
				t.Fatalf("unexpected metadata: %v", modules)
			}
			parsed, _ := url.Parse(endpoint.URL)
			select {
			case got := <-requests:
				if got != (request{parsed.Host, "/", http.MethodPost}) {
					t.Fatalf("URL changed over transport: %+v", got)
				}
			default:
				t.Fatal("request did not reach production node handler")
			}
		}
	}
}

func TestRepairM_110_MirrorKeyPolicy(t *testing.T) {
	record := repairM110Record(t)
	var canonical []repairM110Key
	repairM110JSON(t, repairM110Read(t, "testdata/repair_m110/receipt-keys.json"), &canonical)
	if len(canonical) != 2 || canonical[1].First != 0 || canonical[0].Last != ^uint64(0) {
		t.Fatal("canonical key file must cover zero and the uint64 maximum")
	}
	for _, config := range record.Renderings {
		if !reflect.DeepEqual(config.Paxeer.Keys, repairM110Pins(canonical)) {
			t.Fatal("mirror changed the canonical key file")
		}
	}
	if !reflect.DeepEqual(record.GatewayKeys, []repairM110Key{canonical[1], canonical[0]}) {
		t.Fatal("gateway changed canonical identities or height windows")
	}
	wantSigners := map[string][]string{"layerx:beta": {"did:layerx:" + strings.TrimPrefix(canonical[0].Public, "0x"), "did:layerx:" + strings.TrimPrefix(canonical[1].Public, "0x")}}
	if !reflect.DeepEqual(record.Gateway.Supported.Signers, wantSigners) {
		t.Fatalf("gateway signer set differs from canonical keys: %v", record.Gateway.Supported.Signers)
	}

	expected := map[string]bool{
		"canonical": true, "zero_only": true, "full_uint64": true, "max_only": true,
		"duplicate_disjoint": false, "duplicate_normalized": false, "overlap": false,
		"negative_first": false, "negative_last": false, "overflow_first": false, "overflow_last": false,
		"reversed": false, "boolean_first": false, "boolean_last": false, "string_first": false, "float_last": false,
		"zero_key": false, "zero_id": false, "short_key": false, "malformed_id": false, "empty": false, "unknown_field": false,
	}
	if len(record.Policies) != len(expected) {
		t.Fatal("missing production key policy cases")
	}
	seen := map[string]bool{}
	for _, policy := range record.Policies {
		accepted, ok := expected[policy.Name]
		if !ok || seen[policy.Name] {
			t.Fatalf("unexpected or duplicated policy case %s", policy.Name)
		}
		seen[policy.Name] = true
		t.Run(policy.Name, func(t *testing.T) {
			if accepted {
				var input []repairM110Key
				repairM110JSON(t, policy.Input, &input)
				if policy.Mirror.Error != "" || policy.Gateway.Error != "" || !reflect.DeepEqual(policy.Mirror.Keys, repairM110Pins(input)) {
					t.Fatalf("valid inclusive uint64 policy refused/changed: %+v", policy)
				}
				gateway := policy.Gateway.Keys
				if policy.Name == "canonical" {
					input = []repairM110Key{canonical[1], canonical[0]}
				}
				if !reflect.DeepEqual(gateway, input) {
					t.Fatal("gateway changed inclusive key policy")
				}
			} else {
				if policy.Mirror.Error == "" || policy.Mirror.Keys != nil {
					t.Fatalf("mirror accepted invalid policy: %+v", policy)
				}
				// The gateway's pre-existing zero-id policy is outside this repair.
				if policy.Name != "zero_id" && (policy.Gateway.Error == "" || policy.Gateway.Keys != nil) {
					t.Fatalf("gateway accepted invalid policy: %+v", policy)
				}
				if strings.HasPrefix(policy.Name, "duplicate_") && !strings.Contains(policy.Mirror.Error, "unique") {
					t.Fatalf("duplicate ID was refused for the wrong reason: %s", policy.Mirror.Error)
				}
				if policy.Name == "overlap" && !strings.Contains(policy.Mirror.Error, "overlapping") {
					t.Fatalf("overlap was refused for the wrong reason: %s", policy.Mirror.Error)
				}
			}
		})
	}
}

func repairM110Hex(t *testing.T, value string) []byte {
	t.Helper()
	data, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRepairM_110_ReceiptVerification(t *testing.T) {
	record := repairM110Record(t)
	var fixture struct {
		Receipts   []json.RawMessage `json:"receipts"`
		Provenance string            `json:"receiptProvenance"`
	}
	repairM110JSON(t, repairM110Read(t, "../../crates/layerx-mirror/tests/fixtures/committed_blocks.json"), &fixture)
	if len(fixture.Receipts) != 2 || !strings.Contains(fixture.Provenance, "rpc/sequencer_receipt_vectors_test.go") {
		t.Fatal("missing real node producer transcript")
	}
	byHash := map[string]json.RawMessage{}
	for _, raw := range fixture.Receipts {
		var receipt struct {
			Hash string `json:"txHash"`
		}
		repairM110JSON(t, raw, &receipt)
		byHash[receipt.Hash] = raw
	}
	// Peer responses are committed node transcripts; decoding and verification
	// below use the production RPC client, receipt codec and strict verifier.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params []string        `json:"params"`
		}
		if r.Method != http.MethodPost || r.URL.Path != "/" {
			http.Error(w, "wrong RPC target", 400)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Method != "pax_sequencerReceipt" || len(request.Params) != 1 {
			http.Error(w, "unexpected transcript request", 400)
			return
		}
		result, ok := byHash[request.Params[0]]
		if !ok {
			http.Error(w, "unrecorded transaction", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(struct {
			Version string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Result  json.RawMessage `json:"result"`
		}{"2.0", request.ID, result}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, err := rpc.DialHTTP(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for hash := range byHash {
		var receipt struct {
			Bytes  string `json:"receipt"`
			Digest string `json:"receiptDigest"`
			KeyID  string `json:"keyId"`
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := client.CallContext(ctx, &receipt, "pax_sequencerReceipt", hash)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		raw := repairM110Hex(t, receipt.Bytes)
		decoded, err := codec.DecodeReceipt(raw)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(decoded.BatchID[:24], make([]byte, 24)) {
			t.Fatal("signed height prefix is not canonical")
		}
		height := binary.BigEndian.Uint64(decoded.BatchID[24:])
		if height != 1000 {
			t.Fatalf("unexpected producer height %d", height)
		}
		for _, config := range record.Renderings {
			matches := 0
			for _, pin := range config.Paxeer.Keys {
				if pin.ID != hex.EncodeToString(decoded.ContextHash[:]) {
					continue
				}
				matches++
				if pin.First != 0 || height != pin.Last || height < pin.First || height > pin.Last || "0x"+pin.ID != receipt.KeyID {
					t.Fatal("signed receipt does not bind the rendered inclusive zero-start key window")
				}
				public := repairM110Hex(t, pin.Public)
				if len(public) != 32 {
					t.Fatal("rendered public key length changed")
				}
				verified, err := verify.ReceiptSignature(raw, [32]byte(public))
				if err != nil {
					t.Fatalf("real receipt consumer refused rendered key: %v", err)
				}
				if !bytes.Equal(verified.Receipt.CanonicalBytes(), raw) || !bytes.Equal(verified.Digest[:], repairM110Hex(t, receipt.Digest)) {
					t.Fatal("consumer changed producer receipt or signed digest")
				}
				altered := bytes.Clone(raw)
				altered[len(altered)-1] ^= 1
				if _, err := verify.ReceiptSignature(altered, [32]byte(public)); !errors.Is(err, verify.ErrSequencerSignature) {
					t.Fatalf("tampered signature not refused: %v", err)
				}
				wrongPublic := repairM110Hex(t, record.GatewayKeys[1].Public)
				if _, err := verify.ReceiptSignature(raw, [32]byte(wrongPublic)); !errors.Is(err, verify.ErrSequencerSignature) {
					t.Fatalf("substituted key not refused: %v", err)
				}
				if _, err := verify.ReceiptSignature(append(bytes.Clone(raw), 0), [32]byte(public)); err == nil {
					t.Fatal("noncanonical receipt accepted")
				}
			}
			if matches != 1 {
				t.Fatalf("expected one unique rendered key for signed receipt, got %d", matches)
			}
		}
	}
}
