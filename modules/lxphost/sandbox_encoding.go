//go:build cgo

package lxphost

/*
#include "layerx/programs.h"
*/
import "C"

import (
	"crypto/sha256"
	"encoding/binary"
	"math/big"
)

// SandboxFunding describes a requested lease and the account funding it.
type SandboxFunding struct {
	LeaseID, Tenant, ProgramID, ImageCodeHash, Asset, Source [32]byte
	RequestActivityID, RequestReceiptDigest                  [32]byte
	Amount                                                   [16]byte
	Limits                                                   [8]uint64
	OpenedAt, Expiry                                         uint64
	FeeScheduleVersion                                       uint32
	Prices                                                   [7]uint64
}

// EncodeSandboxFunding produces the requested lease-state/v3 and the canonical
// FUND payload. The native FUND activity produces the durable lifecycle records.
func EncodeSandboxFunding(f SandboxFunding) ([]byte, error) {
	for _, id := range [][32]byte{f.LeaseID, f.Tenant, f.ProgramID, f.ImageCodeHash, f.Asset, f.Source, f.RequestActivityID, f.RequestReceiptDigest} {
		if id == ([32]byte{}) {
			return nil, codeNonCanonical
		}
	}
	amount := new(big.Int).SetBytes(f.Amount[:])
	maximum := new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil)
	if amount.Sign() <= 0 || amount.Cmp(maximum) > 0 || f.OpenedAt == 0 || f.Expiry <= f.OpenedAt ||
		f.Expiry-f.OpenedAt > 1000000 || f.FeeScheduleVersion == 0 {
		return nil, codeNonCanonical
	}
	for i, maximum := range [8]uint64{1000000000, 1 << 30, 1 << 30, 1 << 30, 65536, 1 << 30, 1 << 20, 1 << 30} {
		if f.Limits[i] == 0 || f.Limits[i] > maximum {
			return nil, codeNonCanonical
		}
	}
	schedule := binary.BigEndian.AppendUint32(nil, f.FeeScheduleVersion)
	for _, price := range f.Prices {
		if price == 0 {
			return nil, codeNonCanonical
		}
		schedule = binary.BigEndian.AppendUint64(schedule, price)
	}
	seed := append([]byte("sandbox-lease-escrow/v1\x00"), f.LeaseID[:]...)
	var account [32]byte
	if err := result(C.lxp_programs_account_derive(bytes32(&f.ProgramID), bytesPtr(seed), C.size_t(len(seed)), bytes32(&account))); err != nil {
		return nil, err
	}
	treasury, err := kernelAccountID([]byte("system:fees"))
	if err != nil {
		return nil, err
	}
	namespaceMaterial := append([]byte("LayerX/programs/sandbox/namespace/v1\x00"), f.ProgramID[:]...)
	namespace := sha256.Sum256(append(namespaceMaterial, f.LeaseID[:]...))
	fields := make([]byte, 0, 352)
	for _, id := range [][32]byte{f.LeaseID, f.Tenant, f.ProgramID, f.ImageCodeHash, namespace, f.Asset, account, treasury} {
		fields = append(fields, id[:]...)
	}
	fields = append(fields, f.Amount[:]...)
	for _, limit := range f.Limits {
		fields = binary.BigEndian.AppendUint64(fields, limit)
	}
	fields = binary.BigEndian.AppendUint64(fields, f.OpenedAt)
	fields = binary.BigEndian.AppendUint64(fields, f.Expiry)
	lease := append([]byte("LayerX/programs/sandbox/lease-state/v3\x00"), fields...)
	lease = append(lease, make([]byte, 64)...)
	lease = append(lease, schedule...)
	lease = append(lease, make([]byte, 16)...)
	requestMaterial := append([]byte("LayerX/programs/sandbox/request/v3\x00"), fields...)
	requestDigest := sha256.Sum256(append(requestMaterial, schedule...))
	lease = append(lease, 0, 1, 0, 0, 0)
	lease = append(lease, f.RequestActivityID[:]...)
	lease = append(lease, requestDigest[:]...)
	lease = append(lease, f.RequestReceiptDigest[:]...)
	lease = binary.BigEndian.AppendUint64(lease, f.OpenedAt)
	lease = append(lease, 0, 0, 0)
	transfer := append([]byte(nil), f.ProgramID[:]...)
	transfer = binary.BigEndian.AppendUint16(transfer, 1)
	transfer = append(transfer, f.Source[:]...)
	transfer = append(transfer, f.Asset[:]...)
	transfer = append(transfer, account[:]...)
	transfer = append(transfer, f.Amount[:]...)
	out := []byte{1, 2, 0, 0}
	for _, id := range [][32]byte{f.LeaseID, f.Tenant, f.ProgramID, account, f.Asset} {
		out = append(out, id[:]...)
	}
	out = append(out, f.Amount[:]...)
	out = binary.BigEndian.AppendUint64(out, f.Expiry)
	out = append(out, schedule...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(lease)))
	out = append(out, lease...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(transfer)))
	return append(out, transfer...), nil
}
