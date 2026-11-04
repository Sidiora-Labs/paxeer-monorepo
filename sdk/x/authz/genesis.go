package authz

import (
	"fmt"
	"reflect"
	"strings"

	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/gogo/protobuf/proto"
)

// NewGenesisState creates new GenesisState object
func NewGenesisState(entries []GrantAuthorization) *GenesisState {
	return &GenesisState{
		Authorization: entries,
	}
}

// ValidateGenesis check the given genesis state has no integrity issues
// Expiration is preserved without comparison to wall time or genesis block time:
// an export can contain expired records, which clean lookup refuses after import.
func ValidateGenesis(data GenesisState) (err error) {
	// An authorization with an unsupported enum may panic in MsgTypeURL.
	// Report malformed input at validation, before InitGenesis writes any state.
	defer func() {
		if r := recover(); r != nil {
			err = sdkerrors.ErrInvalidType.Wrapf("invalid genesis authorization: %v", r)
		}
	}()
	type tuple struct{ granter, grantee, msgType string }
	seen := make(map[tuple]struct{}, len(data.Authorization))
	for i, entry := range data.Authorization {
		granter, err := sdk.AccAddressFromBech32(entry.Granter)
		if err != nil {
			return sdkerrors.ErrInvalidAddress.Wrapf("grant %d granter: %v", i, err)
		}
		grantee, err := sdk.AccAddressFromBech32(entry.Grantee)
		if err != nil {
			return sdkerrors.ErrInvalidAddress.Wrapf("grant %d grantee: %v", i, err)
		}
		if granter.Equals(grantee) {
			return sdkerrors.ErrInvalidRequest.Wrapf("grant %d: granter and grantee cannot be same", i)
		}
		grant := Grant{Authorization: entry.Authorization, Expiration: entry.Expiration}
		if err := grant.ValidateBasic(); err != nil {
			return fmt.Errorf("grant %d: %w", i, err)
		}
		msgType := grant.GetAuthorization().MsgTypeURL()
		if !strings.HasPrefix(msgType, "/") {
			return sdkerrors.ErrInvalidType.Wrapf("grant %d: invalid message type %q", i, msgType)
		}
		mt := proto.MessageType(strings.TrimPrefix(msgType, "/"))
		if mt == nil || !mt.Implements(reflect.TypeOf((*sdk.Msg)(nil)).Elem()) {
			return sdkerrors.ErrInvalidType.Wrapf("grant %d: unsupported message type %q", i, msgType)
		}
		key := tuple{string(granter), string(grantee), msgType}
		if _, ok := seen[key]; ok {
			return sdkerrors.ErrInvalidRequest.Wrapf("grant %d: duplicate storage tuple", i)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// DefaultGenesisState - Return a default genesis state
func DefaultGenesisState() *GenesisState {
	return &GenesisState{}
}

var _ cdctypes.UnpackInterfacesMessage = GenesisState{}

// UnpackInterfaces implements UnpackInterfacesMessage.UnpackInterfaces
func (data GenesisState) UnpackInterfaces(unpacker cdctypes.AnyUnpacker) error {
	for _, a := range data.Authorization {
		err := a.UnpackInterfaces(unpacker)
		if err != nil {
			return err
		}
	}
	return nil
}

// UnpackInterfaces implements UnpackInterfacesMessage.UnpackInterfaces
func (msg GrantAuthorization) UnpackInterfaces(unpacker cdctypes.AnyUnpacker) error {
	var a Authorization
	return unpacker.UnpackAny(msg.Authorization, &a)
}
