package multisig

import (
	"fmt"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
)

// AminoMultisignature is used to represent amino multi-signatures for StdTx's.
// It is assumed that all signatures were made with SIGN_MODE_LEGACY_AMINO_JSON.
// Sigs is a list of signatures, sorted by corresponding index.
type AminoMultisignature struct {
	BitArray *types.CompactBitArray
	Sigs     [][]byte
}

// NewMultisig returns a new MultiSignatureData
func NewMultisig(n int) *signing.MultiSignatureData {
	return &signing.MultiSignatureData{
		BitArray:   types.NewCompactBitArray(n),
		Signatures: make([]signing.SignatureData, 0, n),
	}
}

// GetIndex returns the index of pk in keys. Returns -1 if not found
func getIndex(pk types.PubKey, keys []types.PubKey) int {
	for i := 0; i < len(keys); i++ {
		if pk.Equals(keys[i]) {
			return i
		}
	}
	return -1
}

// AddSignature adds a signature to the multisig, at the corresponding index. The index must
// represent the pubkey index in the LegacyAmingPubKey structure, which verifies this signature.
// If the signature already exists, replace it.
// Invalid target dimensions or indices return an error without modifying the multisig.
func AddSignature(mSig *signing.MultiSignatureData, sig signing.SignatureData, index int) error {
	if mSig == nil || mSig.BitArray == nil {
		return fmt.Errorf("multisignature and bit array must be nonnil")
	}
	if sig == nil {
		return fmt.Errorf("signature must be nonnil")
	}
	bitArray := mSig.BitArray
	if len(bitArray.Elems) == 0 || bitArray.ExtraBitsStored > 7 {
		return fmt.Errorf("invalid multisignature bit array dimensions")
	}
	size := bitArray.Count()
	if index < 0 || index >= size {
		return fmt.Errorf("signature index %d out of range for %d keys", index, size)
	}
	if extra := bitArray.ExtraBitsStored; extra != 0 && bitArray.Elems[len(bitArray.Elems)-1]&byte((1<<(8-extra))-1) != 0 {
		return fmt.Errorf("multisignature bit array has nonzero padding bits")
	}
	if selected := bitArray.NumTrueBitsBefore(size); selected != len(mSig.Signatures) {
		return fmt.Errorf("multisignature signature count %d does not match selected key count %d", len(mSig.Signatures), selected)
	}
	newSigIndex := mSig.BitArray.NumTrueBitsBefore(index)
	// Signature already exists, just replace the value there
	if mSig.BitArray.GetIndex(index) {
		mSig.Signatures[newSigIndex] = sig
		return nil
	}
	if !mSig.BitArray.SetIndex(index, true) {
		return fmt.Errorf("unable to set signature index %d", index)
	}
	// Optimization if the index is the greatest index
	if newSigIndex == len(mSig.Signatures) {
		mSig.Signatures = append(mSig.Signatures, sig)
		return nil
	}
	// Expand slice by one with a dummy element, move all elements after i
	// over by one, then place the new signature in that gap.
	mSig.Signatures = append(mSig.Signatures, &signing.SingleSignatureData{})
	copy(mSig.Signatures[newSigIndex+1:], mSig.Signatures[newSigIndex:])
	mSig.Signatures[newSigIndex] = sig
	return nil
}

// AddSignatureFromPubKey adds a signature to the multisig, at the index in
// keys corresponding to the provided pubkey.
func AddSignatureFromPubKey(mSig *signing.MultiSignatureData, sig signing.SignatureData, pubkey types.PubKey, keys []types.PubKey) error {
	if mSig == nil {
		return fmt.Errorf("value of mSig is nil %v", mSig)
	}
	if sig == nil {
		return fmt.Errorf("value of sig is nil %v", sig)
	}

	if pubkey == nil || keys == nil {
		return fmt.Errorf("pubkey or keys can't be nil %v %v", pubkey, keys)
	}
	if mSig.BitArray == nil || mSig.BitArray.Count() != len(keys) {
		return fmt.Errorf("multisignature bit array size does not match public key count %d", len(keys))
	}
	index := getIndex(pubkey, keys)
	if index == -1 {
		keysStr := make([]string, len(keys))
		for i, k := range keys {
			keysStr[i] = fmt.Sprintf("%X", k.Bytes())
		}

		return fmt.Errorf("provided key %X doesn't exist in pubkeys: \n%s", pubkey.Bytes(), strings.Join(keysStr, "\n"))
	}

	return AddSignature(mSig, sig, index)
}

func AddSignatureV2(mSig *signing.MultiSignatureData, sig signing.SignatureV2, keys []types.PubKey) error {
	return AddSignatureFromPubKey(mSig, sig.Data, sig.PubKey, keys)
}
