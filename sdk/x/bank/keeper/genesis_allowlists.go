package keeper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

const DenomAllowListsVersion uint32 = 1

// GenesisState carries the complete bank genesis until the bank types owner
// regenerates genesis.pb.go and wires the module to the extended schema.
type GenesisState struct {
	Bank                   types.GenesisState `json:"-"`
	DenomAllowListsVersion uint32             `json:"denom_allow_lists_version"`
	DenomAllowLists        []DenomAllowList   `json:"denom_allow_lists"`
}

type DenomAllowList struct {
	Denom     string          `json:"denom"`
	AllowList types.AllowList `json:"allow_list"`
}

func validateDenomAllowLists(version uint32, lists []DenomAllowList) error {
	if version != DenomAllowListsVersion && (version != 0 || len(lists) != 0) {
		return fmt.Errorf("unsupported denom allowlists version %d", version)
	}
	denoms := make(map[string]bool, len(lists))
	for _, entry := range lists {
		if err := sdk.ValidateDenom(entry.Denom); err != nil {
			return fmt.Errorf("invalid allowlist denom %q: %w", entry.Denom, err)
		}
		if denoms[entry.Denom] {
			return fmt.Errorf("duplicate allowlist denom %q", entry.Denom)
		}
		denoms[entry.Denom] = true
		addresses := make(map[string]bool, len(entry.AllowList.Addresses))
		for _, address := range entry.AllowList.Addresses {
			addr, err := sdk.AccAddressFromBech32(address)
			if err != nil {
				return fmt.Errorf("invalid allowlist address for %s: %w", entry.Denom, err)
			}
			if address != addr.String() {
				return fmt.Errorf("non-canonical allowlist address %q", address)
			}
			if addresses[string(addr)] {
				return fmt.Errorf("duplicate allowlist address %q for %s", address, entry.Denom)
			}
			addresses[string(addr)] = true
		}
	}
	return nil
}

func (gs *GenesisState) Validate() error {
	if gs == nil {
		return fmt.Errorf("nil bank genesis")
	}
	if err := validateDenomAllowLists(gs.DenomAllowListsVersion, gs.DenomAllowLists); err != nil {
		return err
	}
	return gs.Bank.Validate()
}

func (k BaseKeeper) ExportGenesisWithAllowLists(ctx sdk.Context) *GenesisState {
	lists := []DenomAllowList{}
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.DenomAllowListPrefix)
	iterator := store.Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		// SetDenomAllowList stores prefix || denom || denom, without a delimiter.
		key := iterator.Key()
		mid := len(key) / 2
		if len(key)%2 != 0 || !bytes.Equal(key[:mid], key[mid:]) {
			panic(fmt.Errorf("invalid denom allowlist store key %x", key))
		}
		var list types.AllowList
		k.cdc.MustUnmarshal(iterator.Value(), &list)
		lists = append(lists, DenomAllowList{Denom: string(key[:mid]), AllowList: list})
	}
	sort.Slice(lists, func(i, j int) bool { return lists[i].Denom < lists[j].Denom })
	if err := validateDenomAllowLists(DenomAllowListsVersion, lists); err != nil {
		panic(err)
	}
	return &GenesisState{
		Bank:                   *k.exportGenesisBalances(ctx),
		DenomAllowListsVersion: DenomAllowListsVersion,
		DenomAllowLists:        lists,
	}
}

func (k BaseKeeper) InitGenesisWithAllowLists(ctx sdk.Context, gs *GenesisState) {
	if err := gs.Validate(); err != nil {
		panic(err)
	}
	k.InitGenesis(ctx, &gs.Bank)
	for _, entry := range gs.DenomAllowLists {
		k.SetDenomAllowList(ctx, entry.Denom, entry.AllowList)
	}
}

// MarshalGenesisJSON uses the bank codec for existing fields, preserving the
// protobuf JSON contract rather than encoding SDK numeric types independently.
func MarshalGenesisJSON(cdc codec.JSONCodec, gs *GenesisState) ([]byte, error) {
	if err := gs.Validate(); err != nil {
		return nil, err
	}
	base, err := cdc.MarshalAsJSON(&gs.Bank)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(base, &fields); err != nil {
		return nil, err
	}
	fields["denom_allow_lists_version"], err = json.Marshal(gs.DenomAllowListsVersion)
	if err != nil {
		return nil, err
	}
	fields["denom_allow_lists"], err = json.Marshal(gs.DenomAllowLists)
	if err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}

func UnmarshalGenesisJSON(cdc codec.JSONCodec, data []byte) (*GenesisState, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	var gs GenesisState
	if raw, ok := fields["denom_allow_lists_version"]; ok {
		if err := json.Unmarshal(raw, &gs.DenomAllowListsVersion); err != nil {
			return nil, err
		}
		delete(fields, "denom_allow_lists_version")
	}
	if raw, ok := fields["denom_allow_lists"]; ok {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&gs.DenomAllowLists); err != nil {
			return nil, err
		}
		delete(fields, "denom_allow_lists")
	}
	base, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	if err := cdc.UnmarshalAsJSON(base, &gs.Bank); err != nil {
		return nil, err
	}
	if err := gs.Validate(); err != nil {
		return nil, err
	}
	return &gs, nil
}
