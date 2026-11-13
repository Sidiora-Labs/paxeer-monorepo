package l18

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestNativeEpochDatabaseCancellationIsAtomic(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(localChainFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	var chain struct{ RPC, Pool, Admin, Alice, Bob string }
	if err = json.Unmarshal(raw, &chain); err != nil {
		t.Fatal(err)
	}
	rpc, err := newFixtureRPC(chain.RPC)
	if err != nil {
		t.Fatal(err)
	}
	creator := NewCreator(rpc, time.Now)
	feeClaim := mustSend(t, rpc, chain.Alice, FeesRouter, router, "claimFees", big.NewInt(1))
	mustSend(t, rpc, chain.Alice, FeesRouter, router, "setFeeStrategy", big.NewInt(1), uint8(2))
	mustSend(t, rpc, chain.Admin, USDL, mintABI, "mint", common.HexToAddress(chain.Pool), big.NewInt(1000))
	mustSend(t, rpc, chain.Admin, chain.Pool, swapABI, "swap", big.NewInt(1000), big.NewInt(0), true, common.HexToAddress("0x000000000000000000000000000000000000dead"), big.NewInt(2000000000))
	triggered := mustSend(t, rpc, chain.Alice, FeesRouter, router, "executeAirdrop", big.NewInt(1))
	claimed := mustSend(t, rpc, chain.Alice, FeesRouter, router, "claimAirdropEpoch", big.NewInt(1), big.NewInt(1))
	for _, action := range []string{"register", "register replay", "claim", "claim replay", "entitlement", "creator claim", "creator replay"} {
		t.Run(action, func(t *testing.T) {
			rewardCancellationMatrix(t, func(t *testing.T, l *Ledger) func() error {
				if action != "register" && action != "creator claim" && action != "creator replay" {
					epoch, err := l.RegisterEpoch(ctx, creator, triggered, chain.Pool)
					if err != nil || epoch.FundedRaw != "10" {
						t.Fatal(epoch, err)
					}
				}
				if action == "claim replay" {
					result, err := l.RecordEpochClaim(ctx, creator, claimed, chain.Pool, chain.Alice, "1")
					if err != nil || result.Raw != "6" {
						t.Fatal(result, err)
					}
				}
				if action == "creator replay" {
					result, err := l.RecordCreatorClaim(ctx, creator, feeClaim, chain.Pool, chain.Alice)
					if err != nil || result.Raw != "10000000" {
						t.Fatal(result, err)
					}
				}
				return func() error {
					switch action {
					case "register", "register replay":
						epoch, err := l.RegisterEpoch(ctx, creator, triggered, chain.Pool)
						if err == nil && (epoch.ID != "1" || epoch.FundedRaw != "10") {
							t.Fatal(epoch)
						}
						return err
					case "claim", "claim replay":
						result, err := l.RecordEpochClaim(ctx, creator, claimed, chain.Pool, chain.Alice, "1")
						if err == nil && result.Raw != "6" {
							t.Fatal(result)
						}
						return err
					case "entitlement":
						result, err := l.EpochEntitlement(ctx, creator, chain.Pool, "1", chain.Bob)
						if err == nil && (result.ClaimableRaw != "4" || result.AlreadyClaimed) {
							t.Fatal(result)
						}
						return err
					default:
						result, err := l.RecordCreatorClaim(ctx, creator, feeClaim, chain.Pool, chain.Alice)
						if err == nil && result.Raw != "10000000" {
							t.Fatal(result)
						}
						return err
					}
				}
			})
		})
	}
	l := storage(t)
	for _, pool := range []string{"invalid", chain.Pool} {
		if _, err = l.RegisterEpoch(ctx, nil, triggered, pool); err == nil {
			t.Fatal("missing epoch authority accepted")
		}
		if _, err = l.RecordEpochClaim(ctx, nil, claimed, pool, chain.Alice, "1"); err == nil {
			t.Fatal("missing claim authority accepted")
		}
		if _, err = l.EpochEntitlement(ctx, nil, pool, "1", chain.Alice); err == nil {
			t.Fatal("missing entitlement authority accepted")
		}
	}
	if _, err = l.RecordEpochClaim(ctx, creator, claimed, chain.Pool, "invalid", "1"); err == nil {
		t.Fatal("invalid beneficiary accepted")
	}
	if _, err = l.RecordEpochClaim(ctx, creator, claimed, chain.Pool, chain.Alice, "0"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	for _, beneficiary := range []string{"invalid", chain.Pool, "0x000000000000000000000000000000000000dead"} {
		if _, err = l.EpochEntitlement(ctx, creator, chain.Pool, "1", beneficiary); err == nil {
			t.Fatal(beneficiary)
		}
	}
	if _, err = l.EpochEntitlement(ctx, creator, chain.Pool, "0", chain.Alice); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = l.PrepareEpochClaim(ctx, nil, "1", "1", chain.Alice); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	for _, tc := range []struct{ id, epoch, caller string }{{"invalid", "1", chain.Alice}, {"1", "0", chain.Alice}, {"1", "1", "invalid"}} {
		if _, err = l.PrepareEpochClaim(ctx, creator, tc.id, tc.epoch, tc.caller); err == nil {
			t.Fatal(tc)
		}
	}
}
