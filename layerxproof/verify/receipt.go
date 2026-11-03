package verify

import (
	"errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

var (
	ErrReceiptProtocolVersion  = errors.New("layerx verify: receipt protocol version")
	ErrReceiptActivityID       = errors.New("layerx verify: receipt activity identifier")
	ErrReceiptMissingSignature = errors.New("layerx verify: receipt signature absent")
	ErrSequencerSignature      = errors.New("layerx verify: sequencer signature")

	ErrHeaderCanonical     = errors.New("layerx verify: batch header canonical form")
	ErrHeaderBatchNumber   = errors.New("layerx verify: batch outside authorised range")
	ErrSequencerIdentity   = errors.New("layerx verify: sequencer identity")
	ErrHeaderSignature     = errors.New("layerx verify: batch header signature")
	ErrReceiptBatchBinding = errors.New("layerx verify: receipt outside batch sequence range")
)

// SequencerAuthorization is the sequencer authority valid for an inclusive
// batch range. It is the trust anchor of every header-bound proof.
type SequencerAuthorization struct {
	SequencerID      [32]byte
	PublicKey        [32]byte
	FirstBatchNumber uint64
	LastBatchNumber  uint64
}

// VerifiedReceipt is a receipt whose canonical form and sequencer signature
// passed under the pinned sequencer key.
type VerifiedReceipt struct {
	Receipt *codec.Receipt
	Digest  [32]byte
}

// VerifiedBatchHeader is a canonical header whose authority range, sequencer
// identity and signature passed.
type VerifiedBatchHeader struct {
	Header *codec.BatchHeader
	Digest [32]byte
}

// ReceiptSignature is the receipt-authenticity primitive
// (layerx-proof receipt::verify_sequencer_signature): canonical decode,
// supported protocol, non-zero activity, present signature, and a strict
// Ed25519 signature over the receipt digest under sequencerPublicKey.
func ReceiptSignature(receiptBytes []byte, sequencerPublicKey [32]byte) (*VerifiedReceipt, error) {
	receipt, err := codec.DecodeReceipt(receiptBytes)
	if err != nil {
		return nil, err
	}
	if !codec.ProtocolVersionUsesOccupancy(receipt.ProtocolVersion) {
		return nil, ErrReceiptProtocolVersion
	}
	if receipt.ActivityID == ([32]byte{}) {
		return nil, ErrReceiptActivityID
	}
	if receipt.SequencerSignature == nil {
		return nil, ErrReceiptMissingSignature
	}
	digest := receipt.Digest()
	if err := Ed25519Digest(sequencerPublicKey, *receipt.SequencerSignature, digest); err != nil {
		return nil, ErrSequencerSignature
	}
	return &VerifiedReceipt{Receipt: receipt, Digest: digest}, nil
}

// ReceiptAtRoot proves canonical receipt bytes included under a trusted
// receipt Merkle root and authenticates the receipt under the sequencer key.
func ReceiptAtRoot(receiptBytes []byte, proof *codec.MerkleProof, receiptRoot [32]byte, sequencerPublicKey [32]byte) (*VerifiedReceipt, error) {
	verified, err := ReceiptSignature(receiptBytes, sequencerPublicKey)
	if err != nil {
		return nil, err
	}
	if err := codec.VerifyMerklePath(receiptBytes, proof, receiptRoot); err != nil {
		return nil, err
	}
	return verified, nil
}
