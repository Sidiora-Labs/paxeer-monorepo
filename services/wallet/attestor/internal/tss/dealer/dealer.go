package dealer

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/server"
	core "github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/tss/dealershare"
	pt "github.com/getamis/alice/crypto/ecpointgrouplaw"
	"math/big"
)

type Curve = core.Curve
type ShareBundle core.ShareBundle

const (
	Participants = core.Participants
	Threshold    = core.Threshold
	Secp256k1    = core.Secp256k1
	Ed25519      = core.Ed25519
)

var (
	ErrUnknownCurve      = core.ErrUnknownCurve
	ErrParticipants      = core.ErrParticipants
	ErrScalarRange       = core.ErrScalarRange
	ErrSeedLength        = core.ErrSeedLength
	ErrPointCurve        = core.ErrPointCurve
	ErrIdentityPoint     = core.ErrIdentityPoint
	ErrBundleIncomplete  = core.ErrBundleIncomplete
	ErrShareMismatch     = core.ErrShareMismatch
	ErrPublicKeyMismatch = core.ErrPublicKeyMismatch
)

func (b ShareBundle) ParticipantIDs() []string  { return core.ShareBundle(b).ParticipantIDs() }
func (b ShareBundle) ValidatePublicData() error { return core.ShareBundle(b).ValidatePublicData() }
func (b ShareBundle) Validate() error           { return core.ShareBundle(b).Validate() }
func (b ShareBundle) Clone() ShareBundle        { return ShareBundle(core.ShareBundle(b).Clone()) }
func Split(curve Curve, secret *big.Int, ids []string) ([]ShareBundle, *pt.ECPoint, error) {
	bundles, pub, err := core.Split(curve, secret, ids)
	if err != nil {
		return nil, pub, err
	}
	out := make([]ShareBundle, len(bundles))
	for i, b := range bundles {
		out[i] = ShareBundle(b)
	}
	return out, pub, nil
}
func Ed25519ScalarFromSeed(seed []byte) (*big.Int, error) { return core.Ed25519ScalarFromSeed(seed) }
func EncodeEd25519(p *pt.ECPoint) ([]byte, error)         { return core.EncodeEd25519(p) }
func Wipe(x *big.Int)                                     { core.Wipe(x) }

// CeremonyNodeConfig exposes the embedded production server constructor through
// the existing public share alias without weakening Go's internal boundary.
func (ShareBundle) CeremonyNodeConfig() server.CeremonyNodeConfig { return server.CeremonyNodeConfig{} }
