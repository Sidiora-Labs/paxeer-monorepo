package types

import (
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

func (gs GenesisState) Validate() error {
	if err := validateGenesisParams(gs.Params); err != nil {
		return err
	}
	return gs.validateEntries()
}

func genesisParamDecimals(p Params) []sdk.Dec {
	return []sdk.Dec{p.PriorityNormalizer, p.BaseFeePerGas, p.MinimumFeePerGas,
		p.MaximumFeePerGas, p.MaxDynamicBaseFeeUpwardAdjustment,
		p.MaxDynamicBaseFeeDownwardAdjustment, p.MaxFeeTokenSpread}
}

func validateGenesisParams(p Params) error {
	for _, value := range genesisParamDecimals(p) {
		if value.IsNil() {
			return fmt.Errorf("genesis parameters contain an uninitialized decimal")
		}
	}
	return p.Validate()
}

// Stream chunks may omit parameters; at least one chunk must supply them.
// Decimal zero values also cover omitted parameters after JSON/protobuf transport.
func emptyGenesisParams(p Params) bool {
	for _, value := range genesisParamDecimals(p) {
		if !value.IsNil() && !value.IsZero() {
			return false
		}
	}
	return len(p.WhitelistedCwCodeHashesForDelegateCall) == 0 &&
		p.DeliverTxHookWasmGasLimit == 0 && p.TargetGasUsedPerBlock == 0 &&
		p.PaxSstoreSetGasEip2200 == 0 && !p.RegisterPointerDisabled &&
		len(p.AllowedFeeDenoms) == 0 && !p.FeeTokenEnabled &&
		!p.FeeTokenDistribution && p.MaxFeeTokenRateAge == 0
}

func (gs GenesisState) validateEntries() error {
	for i, association := range gs.AddressAssociations {
		if association == nil {
			return fmt.Errorf("address association %d is nil", i)
		}
		if _, err := sdk.AccAddressFromBech32(association.PaxAddress); err != nil {
			return fmt.Errorf("address association %d: %w", i, err)
		}
		if err := ethtx.ValidateAddress(association.EthAddress); err != nil {
			return fmt.Errorf("address association %d: %w", i, err)
		}
	}
	for i, code := range gs.Codes {
		if code == nil {
			return fmt.Errorf("code %d is nil", i)
		}
		if err := ethtx.ValidateAddress(code.Address); err != nil {
			return fmt.Errorf("code %d: %w", i, err)
		}
	}
	for i, state := range gs.States {
		if state == nil {
			return fmt.Errorf("state %d is nil", i)
		}
		if err := ethtx.ValidateAddress(state.Address); err != nil {
			return fmt.Errorf("state %d: %w", i, err)
		}
		if len(state.Key) != common.HashLength || len(state.Value) != common.HashLength {
			return fmt.Errorf("state %d key and value must each be %d bytes", i, common.HashLength)
		}
	}
	for i, nonce := range gs.Nonces {
		if nonce == nil {
			return fmt.Errorf("nonce %d is nil", i)
		}
		if err := ethtx.ValidateAddress(nonce.Address); err != nil {
			return fmt.Errorf("nonce %d: %w", i, err)
		}
	}
	for i, entry := range gs.Serialized {
		if entry == nil || len(entry.Prefix)+len(entry.Key) == 0 || entry.Value == nil {
			return fmt.Errorf("serialized entry %d must have a storage key and non-nil value", i)
		}
		if err := ValidateLayerXGenesisEntry(entry.Prefix, entry.Key, entry.Value); err != nil {
			return fmt.Errorf("serialized entry %d: %w", i, err)
		}
		if err := ValidateAccountFeeDenomGenesisEntry(entry.Prefix, entry.Key, entry.Value); err != nil {
			return fmt.Errorf("serialized entry %d: %w", i, err)
		}
	}
	return nil
}

func ValidateStream(genesisStateCh <-chan GenesisState) error {
	passedParamCheck := false
	var firstErr error
	for genesisState := range genesisStateCh {
		if !emptyGenesisParams(genesisState.Params) {
			if err := validateGenesisParams(genesisState.Params); err != nil {
				if firstErr == nil {
					firstErr = err
				}
			} else {
				passedParamCheck = true
			}
		}
		if err := genesisState.validateEntries(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return firstErr
	}
	if !passedParamCheck {
		return fmt.Errorf("genesis stream must supply valid parameters")
	}
	return nil
}
