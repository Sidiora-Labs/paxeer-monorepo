package types

import (
	"fmt"

	controllertypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/controller/types"
	hosttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/host/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

// DefaultGenesis creates and returns the interchain accounts GenesisState
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		ControllerGenesisState: DefaultControllerGenesis(),
		HostGenesisState:       DefaultHostGenesis(),
	}
}

// NewGenesisState creates and returns a new GenesisState instance from the provided controller and host genesis state types
func NewGenesisState(controllerGenesisState ControllerGenesisState, hostGenesisState HostGenesisState) *GenesisState {
	return &GenesisState{
		ControllerGenesisState: controllerGenesisState,
		HostGenesisState:       hostGenesisState,
	}
}

// Validate performs basic validation of the interchain accounts GenesisState
func (gs GenesisState) Validate() error {
	if err := gs.ControllerGenesisState.Validate(); err != nil {
		return err
	}

	if err := gs.HostGenesisState.Validate(); err != nil {
		return err
	}

	return nil
}

// DefaultControllerGenesis creates and returns the default interchain accounts ControllerGenesisState
func DefaultControllerGenesis() ControllerGenesisState {
	return ControllerGenesisState{
		Params: controllertypes.DefaultParams(),
	}
}

// NewControllerGenesisState creates a returns a new ControllerGenesisState instance
func NewControllerGenesisState(channels []ActiveChannel, accounts []RegisteredInterchainAccount, ports []string, controllerParams controllertypes.Params) ControllerGenesisState {
	return ControllerGenesisState{
		ActiveChannels:     channels,
		InterchainAccounts: accounts,
		Ports:              ports,
		Params:             controllerParams,
	}
}

// Validate performs basic validation of the ControllerGenesisState
func (gs ControllerGenesisState) Validate() error {
	for _, ch := range gs.ActiveChannels {
		if err := host.ChannelIdentifierValidator(ch.ChannelId); err != nil {
			return err
		}

		if err := host.PortIdentifierValidator(ch.PortId); err != nil {
			return err
		}
	}

	for _, acc := range gs.InterchainAccounts {
		if err := host.PortIdentifierValidator(acc.PortId); err != nil {
			return err
		}

		if err := ValidateAccountAddress(acc.AccountAddress); err != nil {
			return err
		}
	}

	for _, port := range gs.Ports {
		if err := host.PortIdentifierValidator(port); err != nil {
			return err
		}
	}

	if err := gs.Params.Validate(); err != nil {
		return err
	}

	return nil
}

// DefaultHostGenesis creates and returns the default interchain accounts HostGenesisState
func DefaultHostGenesis() HostGenesisState {
	return HostGenesisState{
		Port:   PortID,
		Params: hosttypes.DefaultParams(),
	}
}

// NewHostGenesisState creates a returns a new HostGenesisState instance
func NewHostGenesisState(channels []ActiveChannel, accounts []RegisteredInterchainAccount, port string, hostParams hosttypes.Params) HostGenesisState {
	return HostGenesisState{
		ActiveChannels:     channels,
		InterchainAccounts: accounts,
		Port:               port,
		Params:             hostParams,
	}
}

// Validate performs basic validation of the HostGenesisState
func (gs HostGenesisState) Validate() error {
	if err := host.PortIdentifierValidator(gs.Port); err != nil {
		return err
	}
	if gs.Port != PortID {
		return sdkerrors.Wrapf(ErrInvalidHostPort, "expected %s, got %s", PortID, gs.Port)
	}

	channels := make(map[string]struct{}, len(gs.ActiveChannels))
	for _, ch := range gs.ActiveChannels {
		if err := host.ConnectionIdentifierValidator(ch.ConnectionId); err != nil {
			return err
		}

		if err := host.ChannelIdentifierValidator(ch.ChannelId); err != nil {
			return err
		}

		if err := host.PortIdentifierValidator(ch.PortId); err != nil {
			return err
		}

		key := string(KeyActiveChannel(ch.PortId, ch.ConnectionId))
		if _, exists := channels[key]; exists {
			return fmt.Errorf("duplicate host active channel mapping for connection %s and port %s", ch.ConnectionId, ch.PortId)
		}
		channels[key] = struct{}{}
	}

	accounts := make(map[string]struct{}, len(gs.InterchainAccounts))
	for _, acc := range gs.InterchainAccounts {
		if err := host.ConnectionIdentifierValidator(acc.ConnectionId); err != nil {
			return err
		}

		if err := host.PortIdentifierValidator(acc.PortId); err != nil {
			return err
		}

		if _, err := sdk.AccAddressFromBech32(acc.AccountAddress); err != nil {
			return sdkerrors.Wrapf(ErrInvalidAccountAddress, "invalid host account address: %s", err)
		}

		key := string(KeyOwnerAccount(acc.PortId, acc.ConnectionId))
		if _, exists := accounts[key]; exists {
			return fmt.Errorf("duplicate host account mapping for connection %s and port %s", acc.ConnectionId, acc.PortId)
		}
		accounts[key] = struct{}{}
	}

	if err := gs.Params.Validate(); err != nil {
		return err
	}

	return nil
}
