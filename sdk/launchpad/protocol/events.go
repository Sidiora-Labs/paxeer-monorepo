package protocol

import (
	"fmt"
	"slices"
	"sort"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol/bindings"
)

// EventDef describes one decoded launchpad event: its topic0 (the keccak256 of
// the canonical signature, i.e. log.Topics[0]), the event name and signature,
// the ABI event used to unpack log data, and the set of contracts that declare
// it (the same signature can appear on several contracts, e.g. OwnershipTransferred).
type EventDef struct {
	Topic0    common.Hash
	Name      string
	Sig       string
	Event     abi.Event
	Contracts []string
}

// abiSources are the contract metadata whose events seed the registry — the ten
// contracts that chain.CreateContracts binds for the indexer and processors.
var abiSources = []struct {
	Name string
	Meta *bind.MetaData
}{
	{roleEventEmitter, bindings.EventEmitterMetaData},
	{rolePoolRegistry, bindings.PoolRegistryMetaData},
	{roleSidioraFactory, bindings.SidioraFactoryMetaData},
	{roleRouter, bindings.RouterMetaData},
	{roleQuoter, bindings.QuoterMetaData},
	{roleProtocolConfig, bindings.ProtocolConfigMetaData},
	{roleFeeAccumulator, bindings.FeeAccumulatorMetaData},
	{roleSidioraNFT, bindings.SidioraNFTMetaData},
	{roleFeesRouter, bindings.FeesRouterMetaData},
	{rolePoolBeacon, bindings.PoolBeaconMetaData},
}

// eventRegistry maps topic0 -> event definition, built once at package init from
// the generated ABIs (so it always tracks the bindings; no hand-maintained list).
var eventRegistry = buildEventRegistry()

func buildEventRegistry() map[common.Hash]*EventDef {
	reg := make(map[common.Hash]*EventDef)
	for _, src := range abiSources {
		parsed, err := src.Meta.GetAbi()
		if err != nil {
			panic(fmt.Sprintf("protocol: parse %s ABI: %v", src.Name, err))
		}
		for name := range parsed.Events {
			ev := parsed.Events[name]
			if ev.Sig == "FeeStrategyChanged(uint256,uint8,uint8)" {
				// Both role events describe the same NFT. The indexer consumes
				// nftId; preserve the role bindings while exposing one schema.
				inputs := slices.Clone(ev.Inputs)
				inputs[0].Name = "nftId"
				ev = abi.NewEvent(ev.Name, ev.RawName, ev.Anonymous, inputs)
			}
			if d, ok := reg[ev.ID]; ok {
				d.Contracts = appendUnique(d.Contracts, src.Name)
				continue
			}
			reg[ev.ID] = &EventDef{
				Topic0:    ev.ID,
				Name:      ev.Name,
				Sig:       ev.Sig,
				Event:     ev,
				Contracts: []string{src.Name},
			}
		}
	}
	return reg
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

// LookupEvent returns the event definition for a log's first topic (topic0), and
// whether it is a known launchpad event. The returned definition belongs to the caller.
// FeeStrategyChanged(uint256,uint8,uint8) uses the canonical nftId argument for
// both SidioraNFT and FeesRouter, independently of their binding argument names.
func LookupEvent(topic0 common.Hash) (*EventDef, bool) {
	d, ok := eventRegistry[topic0]
	if !ok {
		return nil, false
	}
	return cloneEventDef(d), true
}

// Events returns every registered event definition, sorted by name then topic0
// for deterministic output.
func Events() []*EventDef {
	out := make([]*EventDef, 0, len(eventRegistry))
	for _, d := range eventRegistry {
		out = append(out, cloneEventDef(d))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Topic0.Hex() < out[j].Topic0.Hex()
	})
	return out
}

// Topic0Count returns the number of distinct event topics in the registry.
func Topic0Count() int { return len(eventRegistry) }

// cloneEventDef isolates callers from the process-wide registry, including the
// ABI argument slices. A caller may annotate a result without changing how a
// later log is decoded.
func cloneEventDef(d *EventDef) *EventDef {
	out := *d
	out.Contracts = slices.Clone(d.Contracts)
	out.Event.Inputs = slices.Clone(d.Event.Inputs)
	for i := range out.Event.Inputs {
		out.Event.Inputs[i].Type = cloneEventType(out.Event.Inputs[i].Type)
	}
	return &out
}

func cloneEventType(t abi.Type) abi.Type {
	if t.Elem != nil {
		elem := cloneEventType(*t.Elem)
		t.Elem = &elem
	}
	t.TupleRawNames = slices.Clone(t.TupleRawNames)
	t.TupleElems = slices.Clone(t.TupleElems)
	for i, elem := range t.TupleElems {
		copy := cloneEventType(*elem)
		t.TupleElems[i] = &copy
	}
	return t
}
