package layerxprograms

import (
	"encoding/binary"
	"errors"
	"math"
	"math/big"
)

const (
	sandboxExecuteHeaderBytes   = 236
	sandboxFundHeaderBytes      = 256
	sandboxActivateBytes        = 76
	retireStorageHeaderBytes    = 370
	retireStorageCandidateBytes = 149
	retireStorageMaxCandidates  = 3
	retireStorageMaxBytes       = 32768
)

var (
	errSandboxNonCanonical = errors.New("LXP_NON_CANONICAL")
	errSandboxLengthLimit  = errors.New("LXP_LENGTH_LIMIT")
	errSandboxOverflow     = errors.New("LXP_OVERFLOW")
)

func EncodeSandboxExecutePayload(leaseID [32]byte, expectedSequence uint64, expectedLeaseDigest, escrowAccount, asset, feeDestination [32]byte, feeScheduleVersion uint32, feeSchedule [7]uint64, callPayload []byte) ([]byte, error) {
	if leaseID == ([32]byte{}) || expectedLeaseDigest == ([32]byte{}) ||
		escrowAccount == ([32]byte{}) || asset == ([32]byte{}) ||
		feeDestination == ([32]byte{}) || feeScheduleVersion == 0 || len(callPayload) == 0 {
		return nil, errSandboxNonCanonical
	}
	if uint64(len(callPayload)) > math.MaxUint32 || len(callPayload) > math.MaxInt-sandboxExecuteHeaderBytes {
		return nil, errSandboxLengthLimit
	}
	payload := make([]byte, sandboxExecuteHeaderBytes+len(callPayload))
	payload[0], payload[1] = 1, 1
	copy(payload[4:36], leaseID[:])
	binary.BigEndian.PutUint64(payload[36:44], expectedSequence)
	copy(payload[44:76], expectedLeaseDigest[:])
	copy(payload[76:108], escrowAccount[:])
	copy(payload[108:140], asset[:])
	copy(payload[140:172], feeDestination[:])
	binary.BigEndian.PutUint32(payload[172:176], feeScheduleVersion)
	for i, rate := range feeSchedule {
		binary.BigEndian.PutUint64(payload[176+8*i:184+8*i], rate)
	}
	binary.BigEndian.PutUint32(payload[232:236], uint32(len(callPayload)))
	copy(payload[236:], callPayload)
	return payload, nil
}

func EncodeSandboxFundPayload(leaseID, tenant, hostProgram, escrowAccount, asset [32]byte, amount *big.Int, expiry uint64, feeScheduleVersion uint32, feeSchedule [7]uint64, leaseRequest []byte, transferProgramID [32]byte, fundingLegs []TransferLeg) ([]byte, error) {
	if len(leaseRequest) == 0 {
		return nil, errSandboxNonCanonical
	}
	if uint64(len(leaseRequest)) > math.MaxUint32 || len(leaseRequest) > math.MaxInt-sandboxFundHeaderBytes {
		return nil, errSandboxLengthLimit
	}
	if amount == nil || amount.Sign() < 0 || amount.BitLen() > 128 {
		return nil, errSandboxOverflow
	}
	transferPayload, err := EncodeTransferPayload(transferProgramID, fundingLegs)
	if err != nil {
		return nil, err
	}
	if len(transferPayload) == 0 {
		return nil, errSandboxNonCanonical
	}
	if uint64(len(transferPayload)) > math.MaxUint32 || len(transferPayload) > math.MaxInt-sandboxFundHeaderBytes-len(leaseRequest) {
		return nil, errSandboxLengthLimit
	}
	payload := make([]byte, sandboxFundHeaderBytes+len(leaseRequest)+len(transferPayload))
	payload[0], payload[1] = 1, 2
	copy(payload[4:36], leaseID[:])
	copy(payload[36:68], tenant[:])
	copy(payload[68:100], hostProgram[:])
	copy(payload[100:132], escrowAccount[:])
	copy(payload[132:164], asset[:])
	amount.FillBytes(payload[164:180])
	binary.BigEndian.PutUint64(payload[180:188], expiry)
	binary.BigEndian.PutUint32(payload[188:192], feeScheduleVersion)
	for i, rate := range feeSchedule {
		binary.BigEndian.PutUint64(payload[192+8*i:200+8*i], rate)
	}
	binary.BigEndian.PutUint32(payload[248:252], uint32(len(leaseRequest)))
	copy(payload[252:], leaseRequest)
	cursor := 252 + len(leaseRequest)
	binary.BigEndian.PutUint32(payload[cursor:cursor+4], uint32(len(transferPayload)))
	copy(payload[cursor+4:], transferPayload)
	return payload, nil
}

func EncodeSandboxActivatePayload(leaseID, expectedLeaseDigest [32]byte, expectedSequence uint64) ([]byte, error) {
	payload := make([]byte, sandboxActivateBytes)
	payload[0], payload[1] = 1, 3
	copy(payload[4:36], leaseID[:])
	copy(payload[36:68], expectedLeaseDigest[:])
	binary.BigEndian.PutUint64(payload[68:76], expectedSequence)
	return payload, nil
}

func CheckRetireStorageRequest(request []byte) error {
	if len(request) < retireStorageHeaderBytes+retireStorageCandidateBytes || len(request) > retireStorageMaxBytes {
		return errSandboxLengthLimit
	}
	count := int(request[1])
	if count < 1 || count > retireStorageMaxCandidates || len(request) < retireStorageHeaderBytes+count*retireStorageCandidateBytes {
		return errSandboxLengthLimit
	}
	return nil
}
