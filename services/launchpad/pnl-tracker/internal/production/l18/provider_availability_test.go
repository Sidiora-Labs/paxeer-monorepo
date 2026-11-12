package l18

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func assertRewardProviderAvailability(t *testing.T, origin string, operation func(*Creator) error) {
	t.Helper()
	var mu sync.Mutex
	var methods []string
	target := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var request struct {
			Method string `json:"method"`
		}
		if err = json.Unmarshal(raw, &request); err != nil {
			t.Error(err)
			return
		}
		mu.Lock()
		methods = append(methods, request.Method)
		reject := target > 0 && len(methods) == target
		mu.Unlock()
		if reject {
			http.Error(w, "provider unavailable", http.StatusServiceUnavailable)
			return
		}
		response, err := http.Post(origin, "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Error(err)
			return
		}
		defer response.Body.Close()
		w.WriteHeader(response.StatusCode)
		if _, err = io.Copy(w, response.Body); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	rpc, err := newFixtureRPC(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	creator := NewCreator(rpc, time.Now)
	if err = operation(creator); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	queries := append([]string(nil), methods...)
	mu.Unlock()
	if len(queries) == 0 {
		t.Fatal("provider was not read")
	}
	for i, method := range queries {
		t.Run(fmt.Sprintf("%03d_%s", i+1, method), func(t *testing.T) {
			mu.Lock()
			target = i + 1
			methods = nil
			mu.Unlock()
			err := operation(creator)
			mu.Lock()
			observed := append([]string(nil), methods...)
			mu.Unlock()
			if err == nil || len(observed) != i+1 || observed[i] != method {
				t.Fatal(observed, method, err)
			}
		})
	}
}

func TestNativeRewardReadsRequireAvailableCanonicalProvider(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(localChainFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	var chain struct{ RPC, Pool, Admin, Alice, Bob string }
	if err = json.Unmarshal(raw, &chain); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"preview", "prepare"} {
		t.Run(action, func(t *testing.T) {
			assertRewardProviderAvailability(t, chain.RPC, func(c *Creator) error {
				if action == "preview" {
					result, err := c.Preview(ctx, "1", chain.Alice)
					if err == nil && result.ClaimableRaw != "10000000" {
						t.Fatal(result)
					}
					return err
				}
				result, err := c.PrepareClaim(ctx, "1", chain.Alice, chain.Alice)
				if err == nil && (result.To != FeesRouter || result.Value != "0") {
					t.Fatal(result)
				}
				return err
			})
		})
	}
	rpc, err := newFixtureRPC(chain.RPC)
	if err != nil {
		t.Fatal(err)
	}
	feeClaim := mustSend(t, rpc, chain.Alice, FeesRouter, router, "claimFees", big.NewInt(1))
	mustSend(t, rpc, chain.Alice, FeesRouter, router, "setFeeStrategy", big.NewInt(1), uint8(2))
	mustSend(t, rpc, chain.Admin, USDL, mintABI, "mint", common.HexToAddress(chain.Pool), big.NewInt(1000))
	mustSend(t, rpc, chain.Admin, chain.Pool, swapABI, "swap", big.NewInt(1000), big.NewInt(0), true, common.HexToAddress("0x000000000000000000000000000000000000dead"), big.NewInt(2000000000))
	triggered := mustSend(t, rpc, chain.Alice, FeesRouter, router, "executeAirdrop", big.NewInt(1))
	claimed := mustSend(t, rpc, chain.Alice, FeesRouter, router, "claimAirdropEpoch", big.NewInt(1), big.NewInt(1))
	l := storage(t)
	original := NewCreator(rpc, time.Now)
	epoch, err := l.RegisterEpoch(ctx, original, triggered, chain.Pool)
	if err != nil || epoch.FundedRaw != "10" {
		t.Fatal(epoch, err)
	}
	for _, action := range []string{"native event", "claim receipt", "creator claim", "register epoch", "epoch claim", "entitlement", "prepare epoch", "token transfer"} {
		t.Run(action, func(t *testing.T) {
			assertRewardProviderAvailability(t, chain.RPC, func(c *Creator) error {
				switch action {
				case "native event":
					result, err := c.nativeEvent(ctx, triggered, accumulator.Events["AirdropTriggered"], []string{addressTopic(chain.Pool)})
					if err == nil && result.TxHash != triggered {
						t.Fatal(result)
					}
					return err
				case "claim receipt":
					result, err := c.ClaimReceipt(ctx, feeClaim, chain.Pool, chain.Alice)
					if err == nil && result.Raw != "10000000" {
						t.Fatal(result)
					}
					return err
				case "creator claim":
					result, err := l.RecordCreatorClaim(ctx, c, feeClaim, chain.Pool, chain.Alice)
					if err == nil && result.Raw != "10000000" {
						t.Fatal(result)
					}
					return err
				case "register epoch":
					result, err := l.RegisterEpoch(ctx, c, triggered, chain.Pool)
					if err == nil && result.ID != "1" {
						t.Fatal(result)
					}
					return err
				case "epoch claim":
					result, err := l.RecordEpochClaim(ctx, c, claimed, chain.Pool, chain.Alice, "1")
					if err == nil && result.Raw != "6" {
						t.Fatal(result)
					}
					return err
				case "entitlement":
					result, err := l.EpochEntitlement(ctx, c, chain.Pool, "1", chain.Bob)
					if err == nil && result.ClaimableRaw != "4" {
						t.Fatal(result)
					}
					return err
				case "prepare epoch":
					result, err := l.PrepareEpochClaim(ctx, c, "1", "1", chain.Bob)
					if err == nil && result.Entitlement.ClaimableRaw != "4" {
						t.Fatal(result)
					}
					return err
				default:
					result, err := c.tokenTransfer(ctx, claimed, Accumulator, chain.Alice)
					if err == nil && result.Raw != "6" {
						t.Fatal(result)
					}
					return err
				}
			})
		})
	}
}
