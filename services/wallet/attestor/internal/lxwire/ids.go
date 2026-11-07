package lxwire

import (
	"encoding/hex"
	"errors"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

const (
	DIDPrefix             = "did:layerx:"
	MaxAccountNameBytes   = codec.MaxAccountNameBytes
	agentAccountPrefix    = "agent:"
	assetMarker           = ":asset:"
	mainSuffix            = ":main"
	assetIdentifierLength = 64
)

var (
	ErrAccountName = errors.New("lxwire: account name is outside the native namespaces")
	ErrDID         = errors.New("lxwire: did is not did:layerx followed by 64 lowercase hex characters")
)

func DIDFromKey(pub [32]byte) string {
	return DIDPrefix + hex.EncodeToString(pub[:])
}

func KeyFromDID(did string) ([32]byte, error) {
	var key [32]byte
	if !strings.HasPrefix(did, DIDPrefix) {
		return key, ErrDID
	}
	body := did[len(DIDPrefix):]
	if len(body) != 64 || !lowerHex(body) {
		return key, ErrDID
	}
	if _, err := hex.Decode(key[:], []byte(body)); err != nil {
		return key, ErrDID
	}
	return key, nil
}

func MainAccountName(did string) string {
	return agentAccountPrefix + did + mainSuffix
}

func AssetAccountName(did string, asset, nativeAsset [32]byte) (string, error) {
	if did == "" || len(did) > MaxDIDBytes || strings.HasPrefix(did, ":") || strings.HasSuffix(did, ":") ||
		strings.Contains(did, "::") || strings.Contains(did, assetMarker) {
		return "", ErrAccountName
	}
	for i := 0; i < len(did); i++ {
		if !nameByte(did[i]) {
			return "", ErrAccountName
		}
	}
	name := MainAccountName(did)
	if asset != nativeAsset {
		name = agentAccountPrefix + did + assetMarker + hex.EncodeToString(asset[:])
	}
	if err := parseAccountName(name); err != nil {
		return "", err
	}
	return name, nil
}

func AccountID(name []byte) ([32]byte, error) {
	id, err := codec.DeriveAccountID(name)
	if err != nil {
		return [32]byte{}, ErrAccountName
	}
	return id, nil
}

func lowerHex(text string) bool {
	for i := 0; i < len(text); i++ {
		c := text[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func nameByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' || c == ':'
}

func parseAccountName(name string) error {
	if _, ok := codec.AccountKind([]byte(name)); !ok {
		return ErrAccountName
	}
	return nil
}
