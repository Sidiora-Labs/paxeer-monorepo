package receipt

import (
	"context"
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
)

func TestArchiveRoutingRejectsUnprovenFloorsAndBlockIdentities(t *testing.T) {
	transport := &boundedRPCTransport{floor: 128, hashes: map[string]bool{}}
	for _, body := range []string{`{`, `{}`, `{"error":{"message":"state unavailable"}}`, `{"error":{"message":"earliest available is 18446744073709551616"}}`} {
		transport.learnFloor([]byte(body))
		if transport.floor != 128 {
			t.Fatalf("unproven archive floor: %d", transport.floor)
		}
	}
	for _, body := range []string{`{`, `{}`, `{"result":null}`, `{"result":{"hash":""}}`} {
		transport.learn(valueEthGetBlockByHash, []byte(body))
		if len(transport.hashes) != 0 {
			t.Fatalf("unproven block routing: %+v", transport.hashes)
		}
	}
	identities := []deployment.Role{
		{Address: common.HexToAddress("0x1").Hex(), Kind: valueDirect, RuntimeHash: "bad"},
		{Address: common.HexToAddress("0x1").Hex(), Kind: valueErc1967, RuntimeHash: common.HexToHash("0x1").Hex()},
		{Address: common.HexToAddress("0x1").Hex(), Kind: "unsupported", RuntimeHash: common.HexToHash("0x1").Hex()},
	}
	for _, identity := range identities {
		if validRPCIdentity(identity) {
			t.Fatalf("unbound deployment identity: %+v", identity)
		}
	}
	transport.learnFloor([]byte(`{"error":{"message":"earliest available is 256"}}`))
	if transport.floor != 256 {
		t.Fatalf("archive floor not retained: %d", transport.floor)
	}
	transport.learnFloor([]byte(`{"error":{"message":"earliest available is 64"}}`))
	if transport.floor != 256 {
		t.Fatalf("archive floor moved backwards: %d", transport.floor)
	}
	transport.learn(valueEthGetBlockByHash, []byte(`{"result":{"hash":"0xABC"}}`))
	if !transport.hashes["0xabc"] || len(transport.hashes) != 1 {
		t.Fatalf("block routing: %+v", transport.hashes)
	}
	if !validRPCIdentity(deployment.Role{Address: common.HexToAddress("0x1").Hex(), Kind: valueDirect, RuntimeHash: common.HexToHash("0x1").Hex()}) {
		t.Fatal("valid direct deployment rejected")
	}
	if rpcHash("0x"+strings.Repeat("0", 64)) || validSignedChainID(new(big.Int).SetUint64(math.MaxUint64)) {
		t.Fatal("invalid chain or runtime identity admitted")
	}
}

func TestScalarReadAdmissionCancelsQueuedCallsWithoutRetainingPermits(t *testing.T) {
	ctx := context.Background()
	input := repair53InputsFromProducer(t)
	authority, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	capacity := cap(authority.reads.calls)
	for i := 0; i < capacity; i++ {
		authority.reads.calls <- struct{}{}
	}
	waiting, cancel := context.WithTimeout(ctx, 25*time.Millisecond)
	defer cancel()
	read, release, err := authority.scalarRead(waiting)
	if !errors.Is(err, context.DeadlineExceeded) || read != nil || release != nil || len(authority.reads.calls) != capacity {
		t.Fatalf("queued scalar read: %v %v %d", read, err, len(authority.reads.calls))
	}
	for i := 0; i < capacity; i++ {
		<-authority.reads.calls
	}
	read, release, err = authority.scalarRead(ctx)
	if err != nil || read == nil || release == nil || len(authority.reads.calls) != 1 {
		t.Fatalf("available scalar read: %v %v", read, err)
	}
	release()
	if len(authority.reads.calls) != 0 || !errors.Is(read.Err(), context.Canceled) {
		t.Fatal("released scalar read retained context or permit")
	}
}
