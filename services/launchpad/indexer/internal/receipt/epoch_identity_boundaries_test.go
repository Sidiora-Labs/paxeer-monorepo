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

func TestAnchorIdentityRequiresAvailableMatchingRecordedHeader(t *testing.T) {
	ctx := context.Background()
	original := repair53InputsFromProducer(t)
	base, closeBase := repair53Open(t, ctx, original)
	defer closeBase()
	bundle, err := base.Bundle(ctx, original.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"matching", "missing anchor", "zero anchor", "changed identity", "unavailable", "pruned"} {
		t.Run(mode, func(t *testing.T) {
			input, err := clone(original)
			if err != nil {
				t.Fatal(err)
			}
			a, closeClient := repair53Open(t, ctx, input)
			defer closeClient()
			a.options.GenesisHash = common.Hash{}
			a.options.Anchor = &BlockAnchor{Height: bundle.Header.Number.Uint64(), Hash: bundle.BlockHash}
			switch mode {
			case "missing anchor":
				a.options.Anchor = nil
			case "zero anchor":
				a.options.Anchor.Hash = common.Hash{}
			case "changed identity":
				a.options.Anchor.Hash = common.HexToHash("0x123")
			case "unavailable", "pruned":
				changed := 0
				for key, response := range input.Responses {
					if strings.HasPrefix(key, "eth_getBlockByNumber ") && strings.Contains(key, `"0x`+bundle.Header.Number.Text(16)+`"`) {
						response.Result = nil
						message := "pinned header unavailable"
						if mode == "pruned" {
							message = "header pruned"
						}
						response.Error = json.RawMessage(`{"code":-32000,"message":"` + message + `"}`)
						input.Responses[key] = response
						changed++
					}
				}
				if changed == 0 {
					t.Fatal("recorded numbered header absent")
				}
				input = repair53Replay(t, input)
				replacement, closeReplacement := repair53Open(t, ctx, input)
				defer closeReplacement()
				replacement.options.GenesisHash = common.Hash{}
				replacement.options.Anchor = a.options.Anchor
				a = replacement
			}
			err = a.verifyChainIdentity(ctx, nil)
			if mode == "matching" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("unverified chain anchor admitted")
			}
			if mode == "pruned" && !errors.Is(err, ErrAnchorUnavailable) {
				t.Fatalf("pruned anchor classification %v", err)
			}
		})
	}
}

func TestHistoricalEpochBindingsRejectMissingApprovedGeneration(t *testing.T) {
	ctx := context.Background()
	input := repair53InputsFromProducer(t)
	a, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	if len(input.Options.History) == 0 || len(input.Options.Epochs) == 0 {
		t.Fatal("approved source history absent")
	}
	historicalHeight := input.Markets[0].Height
	if _, err := a.historicalAuthority(ctx, 0); err == nil {
		t.Fatal("predeployment source history admitted")
	}
	if _, err := a.historicalAuthority(ctx, a.options.Manifest.ActivationHeight); err == nil {
		t.Fatal("current source accepted as historical")
	}
	original := a.options.History
	a.options.History = nil
	if _, err := a.historicalAuthority(ctx, historicalHeight); err == nil {
		t.Fatal("unbound historical generation admitted")
	}
	epoch := a.options.Epochs[0]
	epoch.Deployment.From = decode.Order{Block: historicalHeight}
	if _, err := a.epochManifest(epoch); err == nil {
		t.Fatal("epoch without approved generation admitted")
	}
	if a.epochDiscovery(epoch) != nil {
		t.Fatal("discovery without source generation admitted")
	}
	epoch.SourceActivationHeight = historicalHeight
	epoch.Deployment.From.Block = a.options.Manifest.ActivationHeight
	if _, err := a.sourceManifest(epoch); err == nil {
		t.Fatal("unapproved historical activation admitted")
	}
	epoch.SourceActivationHeight = a.options.Manifest.ActivationHeight
	if _, err := a.sourceManifest(epoch); err == nil {
		t.Fatal("current source relabeled as historical")
	}
	epoch.SourceActivationHeight = original[0].ActivationHeight
	epoch.Deployment.From.Block = historicalHeight
	if _, err := a.sourceManifest(epoch); err == nil {
		t.Fatal("historical source crossing activation backwards admitted")
	}
	epoch.Deployment.From.Block = a.options.Manifest.ActivationHeight
	if _, err := a.sourceManifest(epoch); err == nil {
		t.Fatal("historical source without configured generation admitted")
	}
	a.options.History = original
	manifest, err := a.sourceManifest(epoch)
	if err != nil || manifest.ActivationHeight != epoch.SourceActivationHeight {
		t.Fatalf("approved historical source %v %v", manifest.ActivationHeight, err)
	}
	a.privateFixture = true
	if _, err := a.historicalAuthority(ctx, historicalHeight); err == nil {
		t.Fatal("private source accepted public historical authority")
	}
	for _, tc := range []struct {
		role   string
		ranges []CodeRange
	}{
		{"SidioraPool", []CodeRange{{Start: 0, Length: 1}}},
		{"SidioraERC20", make([]CodeRange, 65)},
		{"SidioraERC20", []CodeRange{{Start: 0, Length: 0}}},
		{"SidioraERC20", []CodeRange{{Start: 0, Length: 33}}},
		{"SidioraERC20", []CodeRange{{Start: 2, Length: 2}, {Start: 3, Length: 1}}},
	} {
		if validCodeRanges(tc.role, tc.ranges) {
			t.Fatal("invalid immutable code ranges admitted")
		}
	}
	if bundleBlockHash(Bundle{}) != (common.Hash{}) {
		t.Fatal("empty source bundle has block identity")
	}
}
