package types

import sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"

type GenesisState struct {
	Params       Params             `json:"params"`
	Domains      []Domain           `json:"domains"`
	NextID       uint32             `json:"next_id"`
	NextBaseFees map[uint32]sdk.Dec `json:"next_base_fees"`
}

func DefaultGenesis() *GenesisState {
	return &GenesisState{Params: DefaultParams(), Domains: []Domain{}, NextID: 1, NextBaseFees: map[uint32]sdk.Dec{}}
}
func (g GenesisState) Validate() error {
	if err := g.Params.Validate(); err != nil {
		return ErrInvalidGenesis.Wrap(err.Error())
	}
	if g.NextID == 0 || uint64(len(g.Domains)) > uint64(g.Params.MaxChains) {
		return ErrInvalidGenesis.Wrap("invalid next id or chain count")
	}
	ids := make(map[uint32]bool)
	names := make(map[string]bool)
	var total uint64
	for _, d := range g.Domains {
		if d.Id == 0 || d.Id >= g.NextID || ids[d.Id] || names[d.Name] {
			return ErrInvalidGenesis.Wrap("duplicate id/name or invalid next id")
		}
		if err := ValidateName(d.Name); err != nil {
			return ErrInvalidGenesis.Wrap(err.Error())
		}
		if err := g.Params.ValidateAllowances(d.GasAllowance, d.ByteAllowance, d.FeeShareBps); err != nil {
			return ErrInvalidGenesis.Wrap(err.Error())
		}
		if d.Status < StatusActive || d.Status > StatusRetired || d.Bond.IsNil() || d.Bond.IsNegative() || d.TargetGasPerBlock != d.GasAllowance/2 || d.CreatedHeight < 0 {
			return ErrInvalidGenesis.Wrap("invalid domain fields")
		}
		if d.Status == StatusRetired && !d.Bond.IsZero() {
			return ErrInvalidGenesis.Wrap("retired domain has a bond")
		}
		if d.Status == StatusActive {
			if d.GasAllowance > g.Params.MaxTotalGasAllowance-total {
				return ErrInvalidGenesis.Wrap("total active gas allowance exceeded")
			}
			total += d.GasAllowance
		}
		ids[d.Id], names[d.Name] = true, true
	}
	for id, fee := range g.NextBaseFees {
		if !ids[id] || fee.IsNil() || fee.IsNegative() {
			return ErrInvalidGenesis.Wrap("invalid next base fee")
		}
	}
	for id := range ids {
		if _, ok := g.NextBaseFees[id]; !ok {
			return ErrInvalidGenesis.Wrap("missing next base fee")
		}
	}
	return nil
}
