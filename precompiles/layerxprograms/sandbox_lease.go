package layerxprograms

import (
	"crypto/sha256"
	"encoding/binary"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

// SandboxLeaseRequest contains the terms and receipt evidence of a requested v3 lease.
// Limits follow the lease codec order, including namespace bytes as the eighth bound.
type SandboxLeaseRequest struct {
	LeaseID, Tenant, Program, CodeHash, Asset [32]byte
	Amount                                    *big.Int
	Limits                                    [8]uint64
	OpenedAt, Expiry                          uint64
	FeeVersion                                uint32
	FeeRates                                  [7]uint64
	ActivityID, ReceiptDigest                 [32]byte
}

// EncodeSandboxLeaseRequest encodes the requested state consumed by sandboxFund.
// Funding and activation append their own authenticated transitions in the host.
func EncodeSandboxLeaseRequest(r SandboxLeaseRequest) ([]byte, error) {
	for _, id := range [][32]byte{r.LeaseID, r.Tenant, r.Program, r.CodeHash, r.Asset, r.ActivityID, r.ReceiptDigest} {
		if id == ([32]byte{}) {
			return nil, errSandboxNonCanonical
		}
	}
	maximum := new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil)
	if r.Amount == nil || r.Amount.Sign() <= 0 || r.Amount.Cmp(maximum) > 0 {
		return nil, errSandboxOverflow
	}
	if r.Expiry <= r.OpenedAt || r.Expiry-r.OpenedAt > 1000000 || r.FeeVersion == 0 {
		return nil, errSandboxNonCanonical
	}
	for _, rate := range r.FeeRates {
		if rate == 0 {
			return nil, errSandboxNonCanonical
		}
	}
	bounds := [8]uint64{1000000000, 1 << 30, 1 << 30, 1 << 30, 65536, 1 << 30, 1 << 20, 1 << 30}
	for i, limit := range r.Limits {
		if limit == 0 || limit > bounds[i] {
			return nil, errSandboxNonCanonical
		}
	}
	namespaceMaterial := append([]byte("LayerX/programs/sandbox/namespace/v1\x00"), r.Program[:]...)
	namespace := sha256.Sum256(append(namespaceMaterial, r.LeaseID[:]...))
	seed := append([]byte("sandbox-lease-escrow/v1\x00"), r.LeaseID[:]...)
	accountMaterial := append([]byte("LayerX/programs/program-account/v1\x00"), r.Program[:]...)
	accountMaterial = binary.BigEndian.AppendUint32(accountMaterial, uint32(len(seed)))
	escrow := sha256.Sum256(append(accountMaterial, seed...))
	fees, err := codec.DeriveAccountID([]byte("system:fees"))
	if err != nil {
		return nil, err
	}
	var terms []byte
	for _, id := range [][32]byte{r.LeaseID, r.Tenant, r.Program, r.CodeHash, namespace, r.Asset, escrow, fees} {
		terms = append(terms, id[:]...)
	}
	terms = append(terms, r.Amount.FillBytes(make([]byte, 16))...)
	for _, limit := range r.Limits {
		terms = binary.BigEndian.AppendUint64(terms, limit)
	}
	terms = binary.BigEndian.AppendUint64(terms, r.OpenedAt)
	terms = binary.BigEndian.AppendUint64(terms, r.Expiry)
	schedule := binary.BigEndian.AppendUint32(nil, r.FeeVersion)
	for _, rate := range r.FeeRates {
		schedule = binary.BigEndian.AppendUint64(schedule, rate)
	}
	bindingMaterial := append([]byte("LayerX/programs/sandbox/request/v3\x00"), terms...)
	binding := sha256.Sum256(append(bindingMaterial, schedule...))
	state := append([]byte("LayerX/programs/sandbox/lease-state/v3\x00"), terms...)
	state = append(state, make([]byte, 64)...)
	state = append(state, schedule...)
	state = append(state, make([]byte, 16)...)
	state = append(state, 0, 1, 0, 0, 0)
	state = append(state, r.ActivityID[:]...)
	state = append(state, binding[:]...)
	state = append(state, r.ReceiptDigest[:]...)
	state = binary.BigEndian.AppendUint64(state, r.OpenedAt)
	return append(state, 0, 0, 0), nil
}
