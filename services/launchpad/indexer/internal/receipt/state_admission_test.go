package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestRecordedMarketRecoveryRejectsUnavailableBindings(t *testing.T) {
	ctx := context.Background()
	original := repair53InputsFromProducer(t)
	base, closeBase := repair53Open(t, ctx, original)
	defer closeBase()
	head, headHash, err := base.headerByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	market := original.Markets[0]
	for _, tc := range []struct {
		name, method, signature string
		address                 common.Address
		result                  string
	}{
		{"token code", "eth_getCode", "", market.Token, "empty"},
		{"pool code", "eth_getCode", "", market.Pool, "empty"},
		{"deploying factory", "eth_call", "deploymentFactory()", market.Token, "zero"},
		{"pool registration", "eth_call", "isRegisteredPool(address)", common.HexToAddress(original.Options.Manifest.Roles["PoolRegistry"].Address), "zero"},
		{"pool token", "eth_call", "getPoolByToken(address)", common.HexToAddress(original.Options.Manifest.Roles["PoolRegistry"].Address), "zero"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, e := clone(original)
			if e != nil {
				t.Fatal(e)
			}
			changes := 0
			selector := common.Bytes2Hex(crypto.Keccak256([]byte(tc.signature))[:4])
			for key, response := range input.Responses {
				if !strings.HasPrefix(key, tc.method+" ") || !strings.Contains(strings.ToLower(key), strings.ToLower(tc.address.Hex())) || !strings.Contains(strings.ToLower(key), strings.ToLower(headHash.Hex())) || tc.signature != "" && !strings.Contains(key, selector) {
					continue
				}
				response.Error = nil
				if tc.result == "empty" {
					response.Result = json.RawMessage(`"0x"`)
				} else {
					response.Result = json.RawMessage(`"0x` + strings.Repeat("0", 64) + `"`)
				}
				input.Responses[key] = response
				changes++
			}
			if changes == 0 {
				t.Fatal("recorded recovery read absent")
			}
			input = repair53Replay(t, input)
			a, closeClient := repair53Open(t, ctx, input)
			defer closeClient()
			if e = a.Rehydrate(ctx, input.Markets); !errors.Is(e, ErrQualification) {
				t.Fatalf("unbound market recovered: %v", e)
			}
		})
	}
	for _, tc := range []struct {
		name string
		edit func(*RPCQualification)
	}{
		{"head below activation", func(a *RPCQualification) { a.options.Manifest.ActivationHeight = head.Number.Uint64() + 1 }},
		{"historical generation absent", func(a *RPCQualification) { a.options.History = nil }},
		{"discovery template absent", func(a *RPCQualification) { a.options.Discovery = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, closeClient := repair53Open(t, ctx, original)
			defer closeClient()
			tc.edit(a)
			if e := a.Rehydrate(ctx, original.Markets); !errors.Is(e, ErrQualification) {
				t.Fatalf("unbound recovery admitted: %v", e)
			}
		})
	}
	if err = base.Rehydrate(ctx, original.Markets); err != nil {
		t.Fatal(err)
	}
	if err = base.Rehydrate(ctx, original.Markets); err != nil {
		t.Fatalf("idempotent historical recovery: %v", err)
	}
	if got := len(base.DiscoveredEpochs()); got != 4 {
		t.Fatalf("market epoch count %d", got)
	}
}

func TestPinnedReserveReaderRejectsMalformedState(t *testing.T) {
	ctx := context.Background()
	original := repair53InputsFromProducer(t)
	base, closeBase := repair53Open(t, ctx, original)
	defer closeBase()
	if err := base.Rehydrate(ctx, original.Markets); err != nil {
		t.Fatal(err)
	}
	block := common.HexToHash("0x1cd6b450aa87745be16a8760a928a2e7ddd1097e872bdcd99b31ff66f77c2945")
	_, err := base.headerByHash(ctx, block)
	if err != nil {
		t.Fatal(err)
	}
	pool := original.Markets[0].Pool
	for _, tc := range []struct{ name, signature, result string }{
		{"reserve unavailable", "getReserves()", "error"}, {"reserve width", "getReserves()", "short"}, {"price unavailable", "getPrice()", "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, e := clone(original)
			if e != nil {
				t.Fatal(e)
			}
			changes := 0
			selector := common.Bytes2Hex(crypto.Keccak256([]byte(tc.signature))[:4])
			for key, response := range input.Responses {
				if !strings.HasPrefix(key, "eth_call ") || !strings.Contains(strings.ToLower(key), strings.ToLower(pool.Hex())) || !strings.Contains(key, selector) || !strings.Contains(strings.ToLower(key), strings.ToLower(block.Hex())) {
					continue
				}
				if tc.result == "error" {
					response.Result = nil
					response.Error = json.RawMessage(`{"code":-32000,"message":"state unavailable"}`)
				} else {
					response.Error = nil
					response.Result = json.RawMessage(`"0x01"`)
				}
				input.Responses[key] = response
				changes++
			}
			if changes == 0 {
				t.Fatal("recorded reserve read absent")
			}
			input = repair53Replay(t, input)
			a, closeClient := repair53Open(t, ctx, input)
			defer closeClient()
			if e = a.Rehydrate(ctx, input.Markets); e != nil {
				t.Fatal(e)
			}
			if _, e = a.PoolReserves(ctx, pool, block); e == nil {
				t.Fatal("unbound reserve state admitted")
			}
		})
	}
	if _, err = base.PoolReserves(ctx, common.HexToAddress(original.Options.Manifest.Roles["Router"].Address), block); !errors.Is(err, ErrQualification) {
		t.Fatalf("non-pool reserves: %v", err)
	}
}

func TestHistoricalAuthorityConfigurationKeepsApprovedEpochs(t *testing.T) {
	ctx := context.Background()
	input := repair53InputsFromProducer(t)
	client, closeClient, err := NewBoundedRPCClient(ctx, input.Endpoint, "")
	if err != nil {
		t.Fatal(err)
	}
	defer closeClient()
	for _, mode := range []string{"duplicate generation", "unknown generation", "missing catalog revision"} {
		t.Run(mode, func(t *testing.T) {
			options, e := clone(input.Options)
			if e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "duplicate generation":
				options.History = append(options.History, options.History[0])
			case "unknown generation":
				options.History[0].ActivationHeight++
			case "missing catalog revision":
				options.Discovery[0].ABIRevision = strings.Repeat("0", 64)
			}
			if _, e = newRPCQualification(ctx, client, options, "bound", false, nil); e == nil {
				t.Fatal("unapproved authority configuration admitted")
			}
		})
	}
	for _, generation := range input.Options.History {
		historical, err := approvedManifestAt(generation.ActivationHeight)
		if err != nil {
			t.Fatal(err)
		}
		options := input.Options
		options.Manifest = historical
		options.History = nil
		options.Epochs = generation.Epochs
		options.Discovery = generation.Discovery
		// The historical catalog is selected by the original authority itself.
		a, closeAuthority := repair53Open(t, ctx, input)
		defer closeAuthority()
		h, e := a.historicalAuthority(ctx, generation.ActivationHeight)
		if e != nil {
			t.Fatal(e)
		}
		supplemental, e := historicalEmitterEpochs(h.options.Manifest, h.options.CatalogJSON)
		if e != nil {
			t.Fatal(e)
		}
		if len(supplemental) == 0 {
			t.Fatal("historical emitter epoch absent")
		}
		options = h.options
		options.Epochs = append(append([]RPCCodeEpoch(nil), options.Epochs...), supplemental...)
		// The authority may already carry the supplemental epoch.
		unique := options.Epochs[:0]
		seen := map[common.Address]bool{}
		for _, epoch := range options.Epochs {
			if !seen[epoch.Deployment.Address] {
				unique = append(unique, epoch)
				seen[epoch.Deployment.Address] = true
			}
		}
		options.Epochs = unique
		q, e := newRPCQualification(ctx, client, options, h.manifestHash, false, nil)
		if e != nil || q == nil {
			t.Fatalf("approved configured supplemental epoch: %v", e)
		}
		for i := range options.Epochs {
			if options.Epochs[i].Deployment.Address == supplemental[0].Deployment.Address {
				options.Epochs[i].Deployment.ABIRevision = strings.Repeat("0", 64)
			}
		}
		if _, e = newRPCQualification(ctx, client, options, h.manifestHash, false, nil); !errors.Is(e, ErrQualification) {
			t.Fatalf("changed supplemental epoch: %v", e)
		}
	}
}

func TestRestoredSurfaceMarketsRequireApprovedDiscoveryTemplates(t *testing.T) {
	ctx := context.Background()
	input := repair53InputsFromProducer(t)
	a, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	if err := a.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	options, err := clone(input.Options)
	if err != nil {
		t.Fatal(err)
	}
	var restored RPCCodeEpoch
	for _, epoch := range a.DiscoveredEpochs() {
		if epoch.Deployment.Role == valueSidioraPool && epoch.Deployment.From.Block >= options.Manifest.ActivationHeight {
			restored = epoch
		}
	}
	if restored.SourceActivationHeight == 0 {
		t.Fatal("historical source generation absent")
	}

	for _, mode := range []string{"approved", "changed template", "missing template"} {
		t.Run(mode, func(t *testing.T) {
			changed, e := clone(options)
			if e != nil {
				t.Fatal(e)
			}
			if mode != "approved" {
				epoch := restored
				epoch.SourceActivationHeight = 0
				changed.Epochs = append(changed.Epochs, epoch)
			}
			for i := range changed.Discovery {
				if changed.Discovery[i].Role == valueSidioraPool {
					if mode == "changed template" {
						changed.Discovery[i].RuntimeHash = common.HexToHash("0xbad").Hex()
					}
					if mode == "missing template" {
						changed.Discovery[i].Kind = valueBeaconProxy
					}
				}
			}
			q, e := newRPCQualification(ctx, a.client, changed, a.manifestHash, false, nil)
			if mode == "approved" {
				if e != nil || q == nil {
					t.Fatalf("restored surface admission: %v", e)
				}
			} else if !errors.Is(e, ErrQualification) {
				t.Fatalf("unapproved surface template: %v", e)
			}
		})
	}
}

func TestHistoricalPoolCodeRejectsChangedBeaconBindings(t *testing.T) {
	ctx := context.Background()
	input := repair53InputsFromProducer(t)
	a, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	h, err := a.historicalAuthority(ctx, input.Markets[0].Height)
	if err != nil {
		t.Fatal(err)
	}
	e, err := h.discoveryEpoch(h.options, input.Markets[0].Pool, valueSidioraPool, decode.Order{Block: input.Markets[0].Height})
	if err != nil {
		t.Fatal(err)
	}
	hash := input.Markets[0].Creation.BlockHash
	if _, err = h.validateCodeUncached(ctx, e, hash); err != nil {
		t.Fatalf("actual historical beacon: %v", err)
	}
	for _, mode := range []string{"unknown kind", "beacon address", "beacon code", "implementation address", "implementation code", "precompile address"} {
		t.Run(mode, func(t *testing.T) {
			epoch := e
			options, err := clone(h.options)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "unknown kind":
				epoch.Identity.Kind = "unknown"
			case "beacon address":
				epoch.Identity.BeaconAddress = common.HexToAddress("0xbad").Hex()
			case "beacon code":
				id := options.Manifest.Roles["PoolBeacon"]
				id.RuntimeHash = common.HexToHash("0xbad").Hex()
				options.Manifest.Roles["PoolBeacon"] = id
			case "implementation address":
				epoch.Identity.ImplementationAddress = common.HexToAddress("0xbad").Hex()
			case "implementation code":
				epoch.Identity.ImplementationHash = common.HexToHash("0xbad").Hex()
			case "precompile address":
				epoch.Identity.Kind = valuePrecompile
			}
			authority := &RPCQualification{client: h.client, options: options, reads: h.reads, manifestHash: h.manifestHash}
			if _, err = authority.validateCodeUncached(ctx, epoch, hash); !errors.Is(err, ErrQualification) {
				t.Fatalf("changed beacon binding: %v", err)
			}
		})
	}
}
