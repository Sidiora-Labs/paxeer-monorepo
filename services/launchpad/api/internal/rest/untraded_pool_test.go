package rest_test

import (
	"encoding/json"
	"math/big"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/go-chi/chi/v5"
)

func untradedPoolValues(base, minimum, maximum, decay string) map[string]string {
	return map[string]string{"baseFeeBps()": base, "minFeeBps()": minimum, "maxFeeBps()": maximum, "feeDecayRate()": decay}
}

func TestUntradedPoolDetailAndList(t *testing.T) {
	f := newPoolReadsFixture(t, "")
	if _, err := f.pool.Exec(t.Context(), `DELETE FROM stats.accounting_market_commits WHERE pool=$1`, f.first); err != nil {
		t.Fatal(err)
	}
	var policy store.PoolFeePolicy
	admission, err := rest.NewAdmission(100, 100, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	if err = rest.RegisterProductionReads(router, rest.ProductionReadOptions{Model: f.model, Store: store.New(f.pool, f.rdb), Admission: admission, PoolFeePolicy: func() store.PoolFeePolicy { return policy }}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	const head = int64(1761000000)
	for _, tc := range []struct {
		name                          string
		age                           int64
		base, minimum, maximum, decay string
		want                          float64
	}{
		{"zero hours", 0, "30", "10", "300", "199", 229},
		{"partial hour", 3599, "30", "10", "300", "199", 229},
		{"one hour", 3600, "30", "10", "300", "199", 129},
		{"beyond one hour", 7201, "30", "10", "300", "199", 96},
		{"maximum", 0, "30", "10", "300", "500", 300},
		{"minimum", 3600, "0", "10", "300", "1", 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy = store.NewPoolFeePolicy(untradedPoolValues(tc.base, tc.minimum, tc.maximum, tc.decay), time.Unix(head, 0))
			if _, err := f.pool.Exec(t.Context(), `UPDATE indexer.pools SET created_at=$1 WHERE lower(pool_address)=$2`, head-tc.age, f.first); err != nil {
				t.Fatal(err)
			}
			detail := assertV2Success(t, getV2(t, srv.URL+"/v2/pools/"+f.first), "PoolDetailResponse")
			pool := detail["data"].(map[string]any)["pool"].(map[string]any)
			if pool["feeBps"] != tc.want || pool["poolAddress"] != f.first {
				t.Fatalf("detail pool: %v, want fee %v", pool, tc.want)
			}
			list := assertV2Success(t, getV2(t, srv.URL+"/v2/pools?sort=new"), "PoolListResponse")
			items := list["data"].(map[string]any)["items"].([]any)
			if len(items) != 2 {
				t.Fatalf("list pools: %v", items)
			}
			for i, want := range []struct {
				address string
				fee     float64
			}{{poolReadsSecond, 100}, {f.first, tc.want}} {
				item := items[i].(map[string]any)
				if item["poolAddress"] != want.address || item["feeBps"] != want.fee {
					t.Fatalf("list pool: %v, want %v", item, want)
				}
			}
		})
	}
	policy = store.NewPoolFeePolicy(untradedPoolValues("30", "10", "300", "500"), time.Unix(head, 0))
	assertV2Error(t, getV2(t, srv.URL+"/v2/pools/"+poolReadsStateless), 503, "DEPENDENCY_UNAVAILABLE")
	assertV2Error(t, getV2(t, srv.URL+"/v2/pools?sort=trending"), 503, "DEPENDENCY_UNAVAILABLE")
	checkUnavailable := func(t *testing.T) {
		t.Helper()
		assertV2Error(t, getV2(t, srv.URL+"/v2/pools/"+f.first), 503, "DEPENDENCY_UNAVAILABLE")
		assertV2Error(t, getV2(t, srv.URL+"/v2/pools?sort=new"), 503, "DEPENDENCY_UNAVAILABLE")
	}
	t.Run("missing attested values", func(t *testing.T) {
		policy = store.NewPoolFeePolicy(nil, time.Unix(head, 0))
		checkUnavailable(t)
	})
	for _, getter := range []string{"baseFeeBps()", "minFeeBps()", "maxFeeBps()", "feeDecayRate()"} {
		for _, raw := range []string{"", "-1", "1.5", "1e2", "+1", "01", " 1", new(big.Int).Lsh(big.NewInt(1), 256).String()} {
			t.Run(getter+"/"+raw, func(t *testing.T) {
				values := untradedPoolValues("30", "10", "300", "500")
				values[getter] = raw
				policy = store.NewPoolFeePolicy(values, time.Unix(head, 0))
				checkUnavailable(t)
			})
		}
		t.Run(getter+"/absent", func(t *testing.T) {
			values := untradedPoolValues("30", "10", "300", "500")
			delete(values, getter)
			policy = store.NewPoolFeePolicy(values, time.Unix(head, 0))
			checkUnavailable(t)
		})
	}
	for _, values := range []map[string]string{
		untradedPoolValues("30", "301", "300", "500"),
		untradedPoolValues("30", "10", "10000", "500"),
		untradedPoolValues("30", "10", "10001", "500"),
	} {
		policy = store.NewPoolFeePolicy(values, time.Unix(head, 0))
		checkUnavailable(t)
	}
	for _, blockTime := range []time.Time{{}, time.Unix(head-3601, 0)} {
		policy = store.NewPoolFeePolicy(untradedPoolValues("30", "10", "300", "500"), blockTime)
		checkUnavailable(t)
	}
	policy = store.PoolFeePolicy{}
	insertQualifiedFill(t, f.pool, f.first, 3, 75)
	detail := assertV2Success(t, getV2(t, srv.URL+"/v2/pools/"+f.first), "PoolDetailResponse")
	if detail["data"].(map[string]any)["pool"].(map[string]any)["feeBps"] != float64(75) {
		t.Fatalf("observed fee without policy: %v", detail)
	}
}

func TestUntradedPoolFeeLibVectors(t *testing.T) {
	raw, err := os.ReadFile("../../../../../contracts/launchpad/artifacts/contracts/test/FeeLibWrapper.sol/FeeLibWrapper.json")
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		ABI     json.RawMessage `json:"abi"`
		Runtime string          `json:"deployedBytecode"`
	}
	if err = json.Unmarshal(raw, &artifact); err != nil {
		t.Fatal(err)
	}
	contract, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		t.Fatal(err)
	}
	code := common.FromHex(artifact.Runtime)
	if len(code) == 0 {
		t.Fatal("FeeLib wrapper runtime required")
	}
	maximumUint := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String()
	for _, tc := range []struct {
		name, base, minimum, maximum, decay string
		age                                 int64
		want                                int
	}{
		{"zero age", "30", "10", "300", "199", 0, 229},
		{"before hour", "30", "10", "300", "199", 3599, 229},
		{"hour boundary", "30", "10", "300", "199", 3600, 129},
		{"whole hours", "30", "10", "300", "199", 7201, 96},
		{"age cap", "30", "10", "300", "500", 0, 300},
		{"exact age cap", "30", "10", "300", "270", 0, 300},
		{"minimum floor", "0", "10", "300", "1", 3600, 10},
		{"base cap", "300", "10", "300", "0", 0, 300},
		{"uint256 base", maximumUint, "10", "300", maximumUint, 0, 300},
		{"uint256 age", "30", "10", "300", maximumUint, 7201, 300},
		{"zero bounds", "0", "0", "0", "0", 0, 0},
		{"equal bounds", "0", "300", "300", "1", 0, 300},
		{"largest bound", "9998", "0", "9999", "0", 0, 9998},
		{"old pool", "30", "10", "300", "500", 3600 * 1000000, 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := untradedPoolValues(tc.base, tc.minimum, tc.maximum, tc.decay)
			args := make([]any, 9)
			for i, value := range []string{tc.base, tc.minimum, tc.maximum, tc.decay, maximumUint, maximumUint} {
				n, ok := new(big.Int).SetString(value, 10)
				if !ok {
					t.Fatal(value)
				}
				args[i] = n
			}
			args[6], args[7], args[8] = big.NewInt(tc.age), new(big.Int), new(big.Int)
			input, err := contract.Pack("calculateDynamicFee", args...)
			if err != nil {
				t.Fatal(err)
			}
			output, _, err := runtime.Execute(code, input, &runtime.Config{GasLimit: 30000000})
			if err != nil {
				t.Fatal(err)
			}
			if len(output) != 32 || new(big.Int).SetBytes(output).Cmp(big.NewInt(int64(tc.want))) != 0 {
				t.Fatalf("library fee %x, want %d", output, tc.want)
			}
			const createdAt = int64(1760000000)
			policy := store.NewPoolFeePolicy(values, time.Unix(createdAt+tc.age, 0))
			if got, ok := policy.FeeBps(createdAt); !ok || got != tc.want {
				t.Fatalf("fee %d available %t, library %d", got, ok, tc.want)
			}
		})
	}
}
