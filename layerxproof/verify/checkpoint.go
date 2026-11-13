package verify

import (
	"errors"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	ErrAttestationBinding   = errors.New("layerx verify: attestation does not bind the checkpoint")
	ErrAttestationDomain    = errors.New("layerx verify: attestation settlement domain")
	ErrAttestationDuty      = errors.New("layerx verify: attestation replay or availability duty")
	ErrAttestationFreshness = errors.New("layerx verify: attestation outside the freshness window")
	ErrGuarantorSignature   = errors.New("layerx verify: guarantor signature")
	ErrGuarantorUnknown     = errors.New("layerx verify: guarantor not bonded and active")
	ErrSettlementReference  = errors.New("layerx verify: settlement reference")
	ErrEquivocationShape    = errors.New("layerx verify: statements do not contradict")
)

var secp256k1HalfOrder = new(big.Int).Rsh(crypto.S256().Params().N, 1)

// GuarantorSigner resolves the signer address a bonded, active guarantor is
// authorised to attest with at the given epoch.
type GuarantorSigner func(guarantorID [32]byte, epoch uint64) ([20]byte, bool)

// GuarantorSignature checks a historical attestation's secp256k1 signature:
// v is 27 or 28, S is in the lower half order, and the recovered address is
// the declared, non-zero signer. It does not establish block finality.
func GuarantorSignature(attestation *codec.GuarantorAttestation) error {
	if attestation.SignatureV != 27 && attestation.SignatureV != 28 {
		return ErrGuarantorSignature
	}
	if attestation.Signer == ([20]byte{}) {
		return ErrGuarantorSignature
	}
	r := new(big.Int).SetBytes(attestation.Signature[:32])
	s := new(big.Int).SetBytes(attestation.Signature[32:])
	if r.Sign() == 0 || s.Sign() == 0 || s.Cmp(secp256k1HalfOrder) > 0 {
		return ErrGuarantorSignature
	}
	digest := attestation.Digest()
	var compact [65]byte
	copy(compact[:64], attestation.Signature[:])
	compact[64] = attestation.SignatureV - 27
	public, err := crypto.SigToPub(digest[:], compact[:])
	if err != nil {
		return ErrGuarantorSignature
	}
	if crypto.PubkeyToAddress(*public) != attestation.Signer {
		return ErrGuarantorSignature
	}
	return nil
}
