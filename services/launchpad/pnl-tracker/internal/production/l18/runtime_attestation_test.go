package l18

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestCommittedCreatorRejectsUnattestedRuntime(t *testing.T) {
	f := repair62Chain(t)
	ctx := context.Background()
	origin := httptest.NewServer(http.HandlerFunc(f.db.rpcResponse))
	defer origin.Close()
	block, err := f.creator.checkpoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.ToLower(crypto.Keccak256Hash([]byte{0x60, 0, 0x60, 0}).Hex())
	for _, tc := range []struct {
		name, method string
		result       any
		rpcError     bool
		hashes       map[string]string
	}{
		{name: "missing runtime", hashes: map[string]string{}},
		{name: "different runtime", method: "eth_getCode", result: "0x6001"},
		{name: "runtime read", method: "eth_getCode", rpcError: true},
		{name: "storage read", method: "eth_getStorageAt", rpcError: true},
		{name: "short storage", method: "eth_getStorageAt", result: "0x01"},
		{name: "unadmitted implementation", method: "eth_getStorageAt", result: "0x" + strings.Repeat("0", 24) + strings.Repeat("1", 40)},
		{name: "implementation read", method: "eth_getStorageAt", result: "0x" + strings.Repeat("0", 24) + strings.Repeat("1", 40), hashes: map[string]string{actorA: digest}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hashes := map[string]string{NFT: digest, Accumulator: digest, FeesRouter: digest, USDL: digest}
			if tc.name == "missing runtime" {
				hashes = tc.hashes
			} else {
				for key, value := range tc.hashes {
					hashes[key] = value
				}
			}
			variant := rewardReceiptVariant(t, origin.URL, func(method string, params []json.RawMessage, out map[string]any) {
				if method == tc.method {
					if tc.rpcError {
						delete(out, "result")
						out["error"] = map[string]any{"code": -32000, "message": "runtime unavailable"}
					} else {
						out["result"] = tc.result
					}
				}
				if tc.name == "implementation read" && method == "eth_getCode" {
					var target string
					if err := json.Unmarshal(params[0], &target); err != nil {
						t.Error(err)
					}
					if strings.EqualFold(target, actorA) {
						delete(out, "result")
						out["error"] = map[string]any{"code": -32000, "message": "implementation unavailable"}
					}
				}
			})
			if err := verifyRuntime(ctx, variant.rpc, block, hashes); err == nil {
				t.Fatal("unattested runtime accepted")
			}
		})
	}
	for _, tc := range []struct {
		rpc       RPC
		now       func() time.Time
		committed func(context.Context) (Checkpoint, error)
		digest    string
	}{
		{nil, time.Now, f.creator.committed, ManifestDigest}, {f.rpc, nil, f.creator.committed, ManifestDigest}, {f.rpc, time.Now, nil, ManifestDigest}, {f.rpc, time.Now, f.creator.committed, "invalid"},
	} {
		if _, err := NewCommittedCreator(ctx, tc.rpc, tc.now, tc.committed, tc.digest, nil); !errors.Is(err, ErrUnavailable) {
			t.Fatal(err)
		}
	}
	if _, err := NewCommittedCreator(ctx, f.rpc, time.Now, func(context.Context) (Checkpoint, error) { return Checkpoint{}, ErrUnavailable }, ManifestDigest, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := NewJourneyService(nil, f.creator, []byte(strings.Repeat("s", 32)), time.Now, strings.Repeat("a", 64), strings.Repeat("b", 64)); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
