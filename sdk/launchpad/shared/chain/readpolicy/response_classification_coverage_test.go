package readpolicy

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/rpc"
)

func TestRPCClassificationRetainsSelectorsAndRangeLimits(t *testing.T) {
	for _, tc := range []struct {
		name     string
		code     int
		data     any
		selector string
		limit    bool
	}{
		{"revert selector", 3, "0xABCDEF012345", "0xabcdef01", false},
		{"malformed selector", 3, "0xnot-a-selector", "", false},
		{"short selector", 3, "0x00", "", false},
		{"structured data", 3, map[string]string{"reason": "reverted"}, "", false},
		{"range limit", -32005, nil, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct{ ID json.RawMessage }
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "error": map[string]any{"code": tc.code, "message": "read rejected", "data": tc.data}}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, err := rpc.DialContext(context.Background(), server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			var value string
			err = client.CallContext(context.Background(), &value, "eth_call")
			if err == nil {
				t.Fatal("RPC error admitted as a value")
			}
			classified := Classify(err, "primary", "eth_call")
			want := "RPC_REJECTED"
			if tc.code == 3 {
				want = "SEMANTIC_REVERT"
			}
			if classified.Code != want || classified.Retryable || classified.RevertSelector != tc.selector || !errors.Is(classified, err) || IsRangeLimit(err) != tc.limit {
				t.Fatalf("classified error: %+v %v", classified, err)
			}
		})
	}
	for _, status := range []int{400, 429, 500, 503} {
		err := rpc.HTTPError{StatusCode: status, Status: http.StatusText(status)}
		got := Classify(err, "fallback", "head")
		if got.Retryable != (status == 429 || status >= 500) {
			t.Fatalf("HTTP status %d: %+v", status, got)
		}
	}
	for _, tc := range []struct {
		ids       []uint64
		responses []BatchResponse
	}{{nil, nil}, {[]uint64{1, 1}, []BatchResponse{{ID: 1}, {ID: 2}}}} {
		if values, err := MatchBatch(tc.ids, tc.responses); err == nil || values != nil {
			t.Fatalf("invalid batch returned values: %x %v", values, err)
		}
	}
	var seen []Range
	if err := WalkRanges(context.Background(), math.MaxUint64-1, math.MaxUint64, 3, func(_ context.Context, r Range) error { seen = append(seen, r); return nil }, IsRangeLimit); err != nil || len(seen) != 1 || seen[0] != (Range{math.MaxUint64 - 1, math.MaxUint64}) {
		t.Fatalf("maximum range: %+v %v", seen, err)
	}
}
