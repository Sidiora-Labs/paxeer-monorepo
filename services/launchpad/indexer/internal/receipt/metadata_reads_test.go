package receipt

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestStableMetadataRejectsUnavailablePinnedContractReads(t *testing.T) {
	ctx := context.Background()
	original := receiptReplayInputsFromProducer(t)
	base, closeBase := receiptReplayOpen(t, ctx, original)
	defer closeBase()
	if err := base.Rehydrate(ctx, original.Markets); err != nil {
		t.Fatal(err)
	}
	base = base.current()
	bundle, err := base.Bundle(ctx, original.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	pool := original.Markets[0].Pool
	expected, err := base.StableMetadata(ctx, pool, bundle.BlockHash)
	if err != nil || expected.Pool != pool {
		t.Fatalf("pinned pool metadata: %+v, %v", expected, err)
	}
	for _, tc := range []struct {
		signature string
		address   common.Address
	}{
		{"tokenAddress()", pool}, {"usdlAddress()", pool}, {"eventEmitter()", pool}, {"protocolConfig()", pool},
		{"isRegisteredPool(address)", common.HexToAddress(base.options.Manifest.Roles["PoolRegistry"].Address)},
		{"getPoolByToken(address)", common.HexToAddress(base.options.Manifest.Roles["PoolRegistry"].Address)},
		{"decimals()", expected.Token}, {"decimals()", expected.Settlement},
	} {
		t.Run(tc.signature+"/"+strings.ToLower(tc.address.Hex()), func(t *testing.T) {
			input, err := clone(original)
			if err != nil {
				t.Fatal(err)
			}
			selector := common.Bytes2Hex(crypto.Keccak256([]byte(tc.signature))[:4])
			changed := 0
			for key, response := range input.Responses {
				lower := strings.ToLower(key)
				if strings.HasPrefix(key, "eth_call ") && strings.Contains(lower, strings.ToLower(tc.address.Hex())) && strings.Contains(lower, strings.ToLower(bundle.BlockHash.Hex())) && strings.Contains(lower, selector) {
					response.Result = json.RawMessage(`"0x01"`)
					response.Error = nil
					input.Responses[key] = response
					changed++
				}
			}
			if changed == 0 {
				t.Fatal("pinned contract call absent")
			}
			input = receiptReplayReplay(t, input)
			client, closeClient, err := NewBoundedRPCClient(ctx, input.Endpoint, "")
			if err != nil {
				t.Fatal(err)
			}
			defer closeClient()
			authority := &RPCQualification{client: base.client, options: base.options, registry: base.registry, manifestHash: base.manifestHash, privateFixture: base.privateFixture, identityVerified: base.identityVerified, reads: base.reads}
			authority.client = client
			authority.reads = &rpcReadState{calls: make(chan struct{}, 16), decimals: map[string]*rpcDecimalRead{}}
			if _, err := authority.StableMetadata(ctx, pool, bundle.BlockHash); err == nil {
				t.Fatal("malformed contract word admitted")
			}
		})
	}
	for _, role := range []string{"pool", "token", "config"} {
		t.Run("epoch/"+role, func(t *testing.T) {
			authority := &RPCQualification{client: base.client, options: base.options, registry: base.registry, manifestHash: base.manifestHash, privateFixture: base.privateFixture, identityVerified: base.identityVerified, reads: base.reads}
			options, err := clone(base.options)
			if err != nil {
				t.Fatal(err)
			}
			authority.options = options
			address := pool
			if role == "token" {
				address = expected.Token
			}
			if role == "config" {
				address = common.HexToAddress(base.options.Manifest.Roles["ProtocolConfig"].Address)
			}
			if role == "pool" {
				for i := range authority.options.Epochs {
					if authority.options.Epochs[i].Deployment.Address == address {
						authority.options.Epochs[i].Deployment.Role = "SidioraERC20"
					}
				}
			} else {
				epochs := authority.options.Epochs[:0]
				for _, epoch := range authority.options.Epochs {
					if epoch.Deployment.Address != address {
						epochs = append(epochs, epoch)
					}
				}
				authority.options.Epochs = epochs
			}
			if _, err := authority.StableMetadata(ctx, pool, bundle.BlockHash); err == nil {
				t.Fatal("unbound metadata epoch admitted")
			}
		})
	}
	if _, err := base.StableMetadata(ctx, pool, common.HexToHash("0xbad")); err == nil {
		t.Fatal("unavailable metadata block admitted")
	}
	if _, err := base.SourceProvenance(ctx, bundle, common.HexToAddress("0xbad")); err == nil {
		t.Fatal("absent receipt emitter supplied provenance")
	}
	if _, err := base.ExecutionPolicy(ctx, bundle, ^uint32(0)); err == nil {
		t.Fatal("absent fill supplied execution policy")
	}
}
