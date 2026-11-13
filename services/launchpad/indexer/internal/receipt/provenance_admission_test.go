package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/ethereum/go-ethereum/common"
)

func TestHistoricalConfiguredEpochRetainsOriginalDigest(t *testing.T) {
	ctx := context.Background()
	input := repair53InputsFromProducer(t)
	a, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	checked := 0
	for _, generation := range a.options.History {
		historical, err := a.historicalAuthority(ctx, generation.ActivationHeight)
		if err != nil {
			t.Fatal(err)
		}
		for _, original := range generation.Epochs {
			expected, err := historical.codeEpochDigest(original)
			if err != nil {
				t.Fatal(err)
			}
			bounded := original
			bounded.Deployment.Until = &decode.Order{Block: a.options.Manifest.ActivationHeight}
			actual, err := a.codeEpochDigest(bounded)
			if err != nil || actual != expected {
				t.Fatalf("configured epoch provenance changed: %s %s %v", actual, expected, err)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("configured historical epochs absent")
	}
	invalid := a.options.Epochs[0]
	invalid.Deployment.From.Block = 0
	if _, err := a.codeEpochDigest(invalid); !errors.Is(err, ErrQualification) {
		t.Fatalf("epoch before approved history: %v", err)
	}
}

func TestHistoricalProxyCodeRequiresAvailableStorageAndBeaconReads(t *testing.T) {
	ctx := context.Background()
	input := repair53InputsFromProducer(t)
	a, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	historical, err := a.historicalAuthority(ctx, input.Markets[0].Height)
	if err != nil {
		t.Fatal(err)
	}
	epoch, err := historical.discoveryEpoch(historical.options, input.Markets[0].Pool, valueSidioraPool, decode.Order{Block: input.Markets[0].Height})
	if err != nil {
		t.Fatal(err)
	}
	block := input.Markets[0].Creation.BlockHash
	expected, err := historical.validateCodeUncached(ctx, epoch, block)
	if err != nil || expected == "" {
		t.Fatalf("historical pool code: %v", err)
	}
	for _, mode := range []string{"storage unavailable", "storage width", "beacon getter unavailable"} {
		t.Run(mode, func(t *testing.T) {
			changed, e := clone(input)
			if e != nil {
				t.Fatal(e)
			}
			count := 0
			for key, response := range changed.Responses {
				match := strings.HasPrefix(key, "eth_getStorageAt ") && strings.Contains(strings.ToLower(key), strings.ToLower(epoch.Deployment.Address.Hex()))
				if mode == "beacon getter unavailable" {
					match = strings.HasPrefix(key, "eth_call ") && strings.Contains(strings.ToLower(key), strings.ToLower(epoch.Identity.BeaconAddress))
				}
				if !match || !strings.Contains(strings.ToLower(key), strings.ToLower(block.Hex())) {
					continue
				}
				if mode == "storage width" {
					response.Result = json.RawMessage(`"0x01"`)
					response.Error = nil
				} else {
					response.Result = nil
					response.Error = json.RawMessage(`{"code":-32000,"message":"state unavailable"}`)
				}
				changed.Responses[key] = response
				count++
			}
			if count == 0 {
				t.Fatal("recorded proxy state absent")
			}
			changed = repair53Replay(t, changed)
			client, closeChanged, e := NewBoundedRPCClient(ctx, changed.Endpoint, "")
			if e != nil {
				t.Fatal(e)
			}
			defer closeChanged()
			h := &RPCQualification{client: client, options: historical.options, registry: historical.registry, manifestHash: historical.manifestHash, privateFixture: historical.privateFixture, identityVerified: historical.identityVerified, reads: historical.reads}
			if digest, e := h.validateCodeUncached(ctx, epoch, block); e == nil || digest != "" {
				t.Fatalf("unavailable proxy identity admitted: %s %v", digest, e)
			}
		})
	}
}

func TestPoolMetadataRequiresEveryEpochAndOriginalGeneration(t *testing.T) {
	ctx := context.Background()
	input := repair53InputsFromProducer(t)
	a, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	if err := a.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	a = a.current()
	bundle, err := a.Bundle(ctx, input.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	pool := input.Markets[0].Pool
	baseline, err := a.StableMetadata(ctx, pool, bundle.BlockHash)
	if err != nil || baseline.Token != input.Markets[0].Token {
		t.Fatalf("recorded metadata: %+v %v", baseline, err)
	}
	for _, mode := range []string{"token epoch", "pool runtime", "native emitter", "source generation"} {
		t.Run(mode, func(t *testing.T) {
			options, e := clone(a.options)
			if e != nil {
				t.Fatal(e)
			}
			for i := range options.Epochs {
				epoch := &options.Epochs[i]
				switch mode {
				case "token epoch":
					if epoch.Deployment.Address == baseline.Token {
						epoch.Deployment.Address = common.HexToAddress("0xbad")
					}
				case "pool runtime":
					if epoch.Deployment.Address == pool {
						epoch.Identity.RuntimeHash = common.HexToHash("0xbad").Hex()
					}
				case "native emitter":
					if epoch.Deployment.Role == "LaunchpadPrecompile" {
						epoch.Identity.RuntimeHash = common.HexToHash("0xbad").Hex()
					}
				case "source generation":
					if epoch.Deployment.Address == pool {
						epoch.SourceActivationHeight = options.Manifest.ActivationHeight
					}
				}
			}
			changed := &RPCQualification{client: a.client, options: options, registry: a.registry, manifestHash: a.manifestHash, privateFixture: a.privateFixture, identityVerified: a.identityVerified, reads: &rpcReadState{calls: make(chan struct{}, 16), decimals: map[string]*rpcDecimalRead{}}}
			if _, e = changed.StableMetadata(ctx, pool, bundle.BlockHash); e == nil {
				t.Fatal("unbound metadata admitted")
			}
		})
	}
}
