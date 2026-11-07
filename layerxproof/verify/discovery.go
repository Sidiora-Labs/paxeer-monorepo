package verify

import (
	"errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

var (
	ErrDiscoveryRequest      = errors.New("layerx verify: discovery request identity")
	ErrDiscoveryProgram      = errors.New("layerx verify: discovery program mismatch")
	ErrDiscoveryFreshness    = errors.New("layerx verify: discovery freshness binding")
	ErrDiscoverySequencerKey = errors.New("layerx verify: discovery sequencer key mismatch")
	ErrDiscoverySignature    = errors.New("layerx verify: discovery signature")
)

// VerifiedDiscovery is a program head attestation verified under the trusted
// sequencer key. Callers compare Head.ValidThrough with their own clock.
type VerifiedDiscovery struct {
	Head              codec.ProgramDiscoveryHead
	HeadReceiptDigest [32]byte
	Digest            [32]byte
}
