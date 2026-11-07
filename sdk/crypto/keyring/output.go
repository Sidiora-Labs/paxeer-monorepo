package keyring

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// TODO: Move this file to client/keys
// Use protobuf interface marshaler rather then generic JSON

// KeyOutput defines a structure wrapping around an Info object used for output
// functionality.
type KeyOutput struct {
	Name       string `json:"name" yaml:"name"`
	Type       string `json:"type" yaml:"type"`
	Address    string `json:"address" yaml:"address"`
	EvmAddress string `json:"evm_address" yaml:"evm_address"`
	PubKey     string `json:"pubkey" yaml:"pubkey"`
	Mnemonic   string `json:"mnemonic,omitempty" yaml:"mnemonic"`
}

// NewKeyOutput creates a default KeyOutput instance without Mnemonic, Threshold and PubKeys
func NewKeyOutput(name string, keyType KeyType, a sdk.Address, pk cryptotypes.PubKey) (KeyOutput, error) {
	apk, err := codectypes.NewAnyWithValue(pk)
	if err != nil {
		return KeyOutput{}, err
	}
	bz, err := codec.ProtoMarshalJSON(apk, nil)
	if err != nil {
		return KeyOutput{}, err
	}
	return KeyOutput{
		Name:    name,
		Type:    keyType.String(),
		Address: a.String(),
		PubKey:  string(bz),
	}, nil
}

// MkConsKeyOutput create a KeyOutput in with "cons" Bech32 prefixes.
func MkConsKeyOutput(keyInfo Info) (KeyOutput, error) {
	if err := validateInfo(keyInfo); err != nil {
		entry := "<nil>"
		if !isNilInterface(keyInfo) {
			entry = infoKey(keyInfo.GetName())
		}
		return KeyOutput{}, &EntryError{Entry: entry, Err: err}
	}
	pk := keyInfo.GetPubKey()
	addr := sdk.ConsAddress(pk.Address())
	return NewKeyOutput(keyInfo.GetName(), keyInfo.GetType(), addr, pk)
}

// MkValKeyOutput create a KeyOutput in with "val" Bech32 prefixes.
func MkValKeyOutput(keyInfo Info) (KeyOutput, error) {
	if err := validateInfo(keyInfo); err != nil {
		entry := "<nil>"
		if !isNilInterface(keyInfo) {
			entry = infoKey(keyInfo.GetName())
		}
		return KeyOutput{}, &EntryError{Entry: entry, Err: err}
	}
	pk := keyInfo.GetPubKey()
	addr := sdk.ValAddress(pk.Address())
	return NewKeyOutput(keyInfo.GetName(), keyInfo.GetType(), addr, pk)
}

// MkAccKeyOutput create a KeyOutput in with "acc" Bech32 prefixes. If the
// public key is a multisig public key, then the threshold and constituent
// public keys will be added.
func MkAccKeyOutput(keyInfo Info) (KeyOutput, error) {
	if err := validateInfo(keyInfo); err != nil {
		entry := "<nil>"
		if !isNilInterface(keyInfo) {
			entry = infoKey(keyInfo.GetName())
		}
		return KeyOutput{}, &EntryError{Entry: entry, Err: err}
	}
	pk := keyInfo.GetPubKey()
	addr := sdk.AccAddress(pk.Address())
	return NewKeyOutput(keyInfo.GetName(), keyInfo.GetType(), addr, pk)
}

// MkAccKeysOutput returns a slice of KeyOutput objects, each with the "acc"
// Bech32 prefixes, given a slice of Info objects. It returns an error if any
// call to MkKeyOutput fails. An optional List error is propagated unchanged.
func MkAccKeysOutput(infos []Info, listErrors ...error) ([]KeyOutput, error) {
	for _, err := range listErrors {
		if err != nil {
			return nil, err
		}
	}

	kos := make([]KeyOutput, len(infos))
	var err error
	for i, info := range infos {
		kos[i], err = MkAccKeyOutput(info)
		if err != nil {
			return nil, err
		}
		kos[i], err = PopulateEvmAddrIfApplicable(info, kos[i])
		if err != nil {
			return nil, err
		}
	}

	return kos, nil
}

func PopulateEvmAddrIfApplicable(info Info, o KeyOutput) (KeyOutput, error) {
	if err := validateInfo(info); err != nil {
		return o, err
	}
	pub, ok := info.GetPubKey().(*secp256k1.PubKey)
	if !ok {
		return o, nil
	}
	publicKey, err := crypto.DecompressPubkey(pub.Bytes())
	if err != nil {
		return o, err
	}
	o.EvmAddress = crypto.PubkeyToAddress(*publicKey).Hex()
	return o, nil
}
