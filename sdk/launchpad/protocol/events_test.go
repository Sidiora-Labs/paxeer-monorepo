package protocol

import (
	"encoding/json"
	"math/big"
	"reflect"
	"slices"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol/bindings"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestRegistryNonEmpty(t *testing.T) {
	t.Parallel()
	if Topic0Count() == 0 {
		t.Fatal("event registry is empty")
	}
	if got := len(Events()); got != Topic0Count() {
		t.Fatalf("Events() returned %d, Topic0Count() = %d", got, Topic0Count())
	}
}

// TestEveryTopicIsKeccakOfSig independently recomputes each topic0 from its
// canonical signature via crypto.Keccak256 and checks it equals both the
// registry key and the ABI event ID — verifying the registry through a second
// code path rather than trusting abigen alone.
func TestEveryTopicIsKeccakOfSig(t *testing.T) {
	t.Parallel()
	for _, d := range Events() {
		want := crypto.Keccak256Hash([]byte(d.Sig))
		if d.Topic0 != want {
			t.Errorf("%s (%s): Topic0 = %s, keccak(sig) = %s", d.Name, d.Sig, d.Topic0.Hex(), want.Hex())
		}
		if d.Event.ID != d.Topic0 {
			t.Errorf("%s: Event.ID %s != Topic0 %s", d.Name, d.Event.ID.Hex(), d.Topic0.Hex())
		}
		if len(d.Contracts) == 0 {
			t.Errorf("%s: no declaring contracts recorded", d.Name)
		}
	}
}

// TestKnownTopics pins the registry to well-known external golden topic0 values
// (OpenZeppelin Ownable + ERC-1967 proxy), both declared by PoolBeacon.
func TestKnownTopics(t *testing.T) {
	t.Parallel()
	cases := []struct {
		topic0   string
		name     string
		contract string
	}{
		{"0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0", "OwnershipTransferred", "PoolBeacon"},
		{"0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b", "Upgraded", "PoolBeacon"},
	}
	for _, tc := range cases {
		d, ok := LookupEvent(common.HexToHash(tc.topic0))
		if !ok {
			t.Errorf("LookupEvent(%s) not found", tc.topic0)
			continue
		}
		if d.Name != tc.name {
			t.Errorf("topic %s: name = %q, want %q", tc.topic0, d.Name, tc.name)
		}
		found := false
		for _, c := range d.Contracts {
			if c == tc.contract {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("topic %s: %q not among declaring contracts %v", tc.topic0, tc.contract, d.Contracts)
		}
	}
}

func TestLookupUnknownEvent(t *testing.T) {
	t.Parallel()
	// A topic0 that is not keccak of any real event signature.
	if d, ok := LookupEvent(common.HexToHash("0xdeadbeef")); ok {
		t.Errorf("unexpected hit for unknown topic: %+v", d)
	}
}

func TestEventsDeterministicOrder(t *testing.T) {
	t.Parallel()
	a, b := Events(), Events()
	if len(a) != len(b) {
		t.Fatalf("Events() len unstable: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Topic0 != b[i].Topic0 {
			t.Fatalf("Events() order unstable at %d: %s vs %s", i, a[i].Topic0.Hex(), b[i].Topic0.Hex())
		}
	}
}

func TestFeeStrategyChangedRoleSchemaPath(t *testing.T) {
	testFeeStrategyChangedRoleSchemaPath(t)
}

// The metadata is the producer for this SDK schema test. Contract execution
// and the indexer's internal handler belong to the owning-domain wiring proof.
func testFeeStrategyChangedRoleSchemaPath(t *testing.T) {
	t.Helper()
	for _, role := range feeStrategyChangedTestRoles() {
		t.Run(role.name, func(t *testing.T) {
			original, err := role.meta.GetAbi()
			if err != nil {
				t.Fatal(err)
			}
			event := original.Events["FeeStrategyChanged"]
			if event.Inputs[0].Name != role.identity {
				t.Fatalf("producer identity name = %q, want %q", event.Inputs[0].Name, role.identity)
			}
			for i, id := range []int64{1, 9007199254740993, 9223372036854775807} {
				old, next := uint8(i), uint8((i+1)%3)
				log := feeStrategyChangedTestLog(t, role, big.NewInt(id), old, next)
				def, ok := LookupEvent(log.Topics[0])
				if !ok || def == nil {
					t.Fatal("role event absent from exported registry")
				}
				if def.Name != "FeeStrategyChanged" || def.Sig != "FeeStrategyChanged(uint256,uint8,uint8)" || def.Event.ID != event.ID || !slices.Equal(def.Contracts, []string{"SidioraNFT", "FeesRouter"}) {
					t.Fatalf("wrong registry definition: %+v", def)
				}
				wantInputs := slices.Clone(event.Inputs)
				wantInputs[0].Name = "nftId"
				if !reflect.DeepEqual(def.Event.Inputs, wantInputs) {
					t.Fatalf("registry changed types, indexing or strategy names: %+v", def.Event.Inputs)
				}
				consumer := feeStrategyChangedTestConsumer(log.Address, def)
				args := make(map[string]any)
				if err := consumer.UnpackLogIntoMap(args, def.Name, log); err != nil {
					t.Fatal(err)
				}
				want := map[string]any{"nftId": big.NewInt(id), "oldStrategy": old, "newStrategy": next}
				if !reflect.DeepEqual(args, want) {
					t.Fatalf("processor field contract = %#v, want %#v", args, want)
				}
				// Check the generated role consumer independently; it must retain
				// its original identity name while the shared registry is canonical.
				if role.name == "SidioraNFT" {
					filterer, err := bindings.NewSidioraNFTFilterer(log.Address, nil)
					if err != nil {
						t.Fatal(err)
					}
					got, err := filterer.ParseFeeStrategyChanged(log)
					if err != nil {
						t.Fatal(err)
					}
					if got.TokenId.Cmp(big.NewInt(id)) != 0 || got.OldStrategy != old || got.NewStrategy != next || !reflect.DeepEqual(got.Raw, log) {
						t.Fatalf("role parser lost producer values: %+v", got)
					}
				} else {
					filterer, err := bindings.NewFeesRouterFilterer(log.Address, nil)
					if err != nil {
						t.Fatal(err)
					}
					got, err := filterer.ParseFeeStrategyChanged(log)
					if err != nil {
						t.Fatal(err)
					}
					if got.NftId.Cmp(big.NewInt(id)) != 0 || got.OldStrategy != old || got.NewStrategy != next || !reflect.DeepEqual(got.Raw, log) {
						t.Fatalf("role parser lost producer values: %+v", got)
					}
				}
			}
		})
	}
}

type feeStrategyChangedTestRole struct {
	name, identity string
	address        common.Address
	meta           *bind.MetaData
}

func feeStrategyChangedTestRoles() []feeStrategyChangedTestRole {
	return []feeStrategyChangedTestRole{
		{"SidioraNFT", "tokenId", common.HexToAddress(SidioraNFTHex), bindings.SidioraNFTMetaData},
		{"FeesRouter", "nftId", common.HexToAddress(FeesRouterHex), bindings.FeesRouterMetaData},
	}
}

func feeStrategyChangedTestLog(t *testing.T, role feeStrategyChangedTestRole, id *big.Int, old, next uint8) types.Log {
	t.Helper()
	parsed, err := role.meta.GetAbi()
	if err != nil {
		t.Fatal(err)
	}
	event := parsed.Events["FeeStrategyChanged"]
	data, err := event.Inputs.NonIndexed().Pack(old, next)
	if err != nil {
		t.Fatal(err)
	}
	indexed, err := abi.MakeTopics([]any{id})
	if err != nil {
		t.Fatal(err)
	}
	original := types.Log{
		Address: role.address, Topics: []common.Hash{event.ID, indexed[0][0]}, Data: data,
		BlockNumber: 42, TxHash: common.HexToHash("0x1234"), BlockHash: common.HexToHash("0x5678"), TxIndex: 3, Index: 7,
	}
	wire, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var received types.Log
	if err := json.Unmarshal(wire, &received); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(received, original) {
		t.Fatalf("log JSON transport changed values: %+v vs %+v", received, original)
	}
	return received
}

func feeStrategyChangedTestConsumer(address common.Address, def *EventDef) *bind.BoundContract {
	return bind.NewBoundContract(address, abi.ABI{Events: map[string]abi.Event{def.Name: def.Event}}, nil, nil, nil)
}
