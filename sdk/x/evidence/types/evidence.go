package types

import (
	"crypto/sha256"
	"fmt"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	tmbytes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/bytes"
	"gopkg.in/yaml.v2"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/exported"
)

// Evidence type constants
const (
	RouteEquivocation = "equivocation"
	TypeEquivocation  = "equivocation"
)

var _ exported.Evidence = &Equivocation{}

// Route returns the Evidence Handler route for an Equivocation type.
func (e *Equivocation) Route() string { return RouteEquivocation }

// Type returns the Evidence Handler type for an Equivocation type.
func (e *Equivocation) Type() string { return TypeEquivocation }

func (e *Equivocation) String() string {
	bz, _ := yaml.Marshal(e)
	return string(bz)
}

// Hash returns the hash of an Equivocation object.
func (e *Equivocation) Hash() tmbytes.HexBytes {
	bz, err := e.Marshal()
	if err != nil {
		panic(err)
	}
	b := sha256.Sum256(bz)
	return b[:]
}

// ValidateBasic performs basic stateless validation checks on an Equivocation object.
func (e *Equivocation) ValidateBasic() error {
	if e.Time.Unix() <= 0 {
		return fmt.Errorf("invalid equivocation time: %s", e.Time)
	}
	if e.Height < 1 {
		return fmt.Errorf("invalid equivocation height: %d", e.Height)
	}
	if e.Power < 1 {
		return fmt.Errorf("invalid equivocation validator power: %d", e.Power)
	}
	if _, err := e.consensusAddress(); err != nil {
		return err
	}

	return nil
}

// GetConsensusAddress returns the validator's consensus address at time of the
// Equivocation infraction. It panics if the address is invalid; callers must
// validate evidence before using it.
func (e Equivocation) GetConsensusAddress() sdk.ConsAddress {
	addr, err := e.consensusAddress()
	if err != nil {
		panic(err)
	}
	return addr
}

func (e Equivocation) consensusAddress() (sdk.ConsAddress, error) {
	addr, err := sdk.ConsAddressFromBech32(e.ConsensusAddress)
	if err != nil {
		return nil, sdkerrors.Wrapf(ErrInvalidEvidence, "invalid equivocation validator consensus address: %s", err)
	}
	if len(addr) != crypto.AddressSize {
		return nil, sdkerrors.Wrapf(ErrInvalidEvidence, "invalid equivocation validator consensus address length: expected %d, got %d", crypto.AddressSize, len(addr))
	}
	return addr, nil
}

// GetHeight returns the height at time of the Equivocation infraction.
func (e Equivocation) GetHeight() int64 {
	return e.Height
}

// GetTime returns the time at time of the Equivocation infraction.
func (e Equivocation) GetTime() time.Time {
	return e.Time
}

// GetValidatorPower returns the validator's power at time of the Equivocation
// infraction.
func (e Equivocation) GetValidatorPower() int64 {
	return e.Power
}

// GetTotalPower is a no-op for the Equivocation type.
func (e Equivocation) GetTotalPower() int64 { return 0 }

// FromABCIEvidence converts a Tendermint concrete Evidence type to
// SDK Evidence using Equivocation as the concrete type.
func FromABCIEvidence(e abci.Evidence) exported.Evidence {
	bech32PrefixConsAddr := sdk.GetConfig().GetBech32ConsensusAddrPrefix()
	consAddr, err := sdk.Bech32ifyAddressBytes(bech32PrefixConsAddr, e.Validator.Address)
	if err != nil {
		panic(err)
	}

	return &Equivocation{
		Height:           e.Height,
		Power:            e.Validator.Power,
		ConsensusAddress: consAddr,
		Time:             e.Time,
	}
}
