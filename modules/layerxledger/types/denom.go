package types

import (
	"encoding/hex"
	"strings"
)

const MirrorDenomPrefix = "layerx/"

// MirrorDenom returns the native bank denom for a kernel asset. Tokenfactory
// subdenoms cap at 44 characters (MaxSubdenomLength), so the 64-digit asset ID
// requires a native denom.
func MirrorDenom(assetID [32]byte) string {
	return MirrorDenomPrefix + hex.EncodeToString(assetID[:])
}

// ParseMirrorDenom decodes a canonical native bank mirror denom.
func ParseMirrorDenom(denom string) ([32]byte, error) {
	var assetID [32]byte
	if len(denom) != 71 || !strings.HasPrefix(denom, MirrorDenomPrefix) {
		return assetID, ErrNonCanonical
	}
	encoded := denom[len(MirrorDenomPrefix):]
	for i := range encoded {
		c := encoded[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return assetID, ErrNonCanonical
		}
	}
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		return assetID, ErrNonCanonical
	}
	copy(assetID[:], decoded)
	return assetID, nil
}
