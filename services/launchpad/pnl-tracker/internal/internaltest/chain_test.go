package internaltest

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func TestOwnedChainFixtureDeploysCanonicalMarket(t *testing.T) {
	ctx := context.Background()
	rpc, fixture := NewAnvilFixture(t, ctx, coordinatortest.RepositoryRoot(t))
	var chain hexutil.Uint64
	if err := rpc.CallContext(ctx, &chain, "eth_chainId"); err != nil || chain != 125 {
		t.Fatal(chain, err)
	}
	if fixture.Pool == "" || fixture.Token == "" || fixture.Admin == "" || fixture.Alice == fixture.Bob {
		t.Fatal(fixture)
	}
	for _, target := range []string{fixture.Pool, fixture.Token} {
		var code hexutil.Bytes
		if err := rpc.CallContext(ctx, &code, "eth_getCode", target, "latest"); err != nil || len(code) == 0 {
			t.Fatal(target, len(code), err)
		}
	}
	var balance hexutil.Big
	if err := rpc.CallContext(ctx, &balance, "eth_getBalance", fixture.Alice, "latest"); err != nil || balance.ToInt().Sign() <= 0 {
		t.Fatal(balance, err)
	}
}
