package multisig

import (
	"fmt"
	"math"

	types "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

// tmMultisig implements a K of N threshold multisig. It is used for
// Amino JSON marshaling of LegacyAminoPubKey (see below for details).
//
// This struct is copy-pasted from:
// https://github.com/tendermint/tendermint/blob/v0.33.9/crypto/multisig/threshold_pubkey.go
//
// This struct was used in the SDK <=0.39. In 0.40 and the switch to protobuf,
// it has been converted to LegacyAminoPubKey. However, there's one difference:
// the threshold field was an `uint` before, and an `uint32` after. This caused
// amino marshaling to be breaking: amino marshals `uint32` as a JSON number,
// and `uint` as a JSON string.
//
// In this file, we're overriding LegacyAminoPubKey's default JSON Amino
// marshaling by using this struct. Please note that we are NOT overriding the
// Amino binary marshaling, as that _might_ introduce breaking changes in the
// keyring, where multisigs are amino-binary-encoded.
//
// ref: https://github.com/cosmos/cosmos-sdk/issues/8776
type tmMultisig struct {
	K       uint                 `json:"threshold"`
	PubKeys []cryptotypes.PubKey `json:"pubkeys"`
}

// protoToTm converts a LegacyAminoPubKey into a tmMultisig.
func protoToTm(protoPk *LegacyAminoPubKey) (tmMultisig, error) {
	var ok bool
	pks := make([]cryptotypes.PubKey, len(protoPk.PubKeys))
	for i, pk := range protoPk.PubKeys {
		pks[i], ok = pk.GetCachedValue().(cryptotypes.PubKey)
		if !ok {
			return tmMultisig{}, sdkerrors.Wrapf(sdkerrors.ErrInvalidType, "expected %T, got %T", (cryptotypes.PubKey)(nil), pk.GetCachedValue())
		}
	}

	return tmMultisig{
		K:       uint(protoPk.Threshold),
		PubKeys: pks,
	}, nil
}

// tmToProto converts a tmMultisig into a LegacyAminoPubKey.
func tmToProto(tmPk tmMultisig) (*LegacyAminoPubKey, error) {
	if tmPk.K > math.MaxUint32 {
		return nil, fmt.Errorf("threshold %d out of uint32 range", tmPk.K)
	}

	var err error
	pks := make([]*types.Any, len(tmPk.PubKeys))
	for i, pk := range tmPk.PubKeys {
		pks[i], err = types.NewAnyWithValue(pk)
		if err != nil {
			return nil, err
		}
	}

	return &LegacyAminoPubKey{
		Threshold: uint32(tmPk.K), //nolint:gosec // bounds checked above
		PubKeys:   pks,
	}, nil
}

// MarshalAminoJSON overrides amino JSON unmarshaling.
func (m LegacyAminoPubKey) MarshalAminoJSON() (tmMultisig, error) { //nolint:revive
	return protoToTm(&m)
}

// UnmarshalAminoJSON overrides amino JSON unmarshaling.
func (m *LegacyAminoPubKey) UnmarshalAminoJSON(tmPk tmMultisig) error {
	protoPk, err := tmToProto(tmPk)
	if err != nil {
		return err
	}

	// Prepare fresh Anys with the current keys and matching JSON compatibility
	// bytes. Reusing an Any would retain its old cached key or compatibility data.
	for i, pk := range tmPk.PubKeys {
		bz, err := AminoCdc.MarshalAsJSON(pk)
		if err != nil {
			return err
		}
		if err := protoPk.PubKeys[i].UnmarshalJSON(bz); err != nil {
			return err
		}
	}
	m.PubKeys = protoPk.PubKeys
	m.Threshold = protoPk.Threshold

	return nil
}
