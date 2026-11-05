package layerxprograms

import (
	"encoding/binary"
	"errors"
	"math/big"
)

const (
	maxTransferSetLegs  = 256
	transferLegBytes    = 112
	maxAccountSeedBytes = 128
)

type TransferLeg struct {
	From, Asset, To [32]byte
	Amount          *big.Int
}

func EncodeTransferPayload(programID [32]byte, legs []TransferLeg) ([]byte, error) {
	if len(legs) == 0 {
		return nil, errors.New("LXP_NON_CANONICAL")
	}
	if len(legs) > maxTransferSetLegs {
		return nil, errors.New("LXP_LENGTH_LIMIT")
	}
	for _, leg := range legs {
		if leg.Amount == nil || leg.Amount.Sign() < 0 {
			return nil, errors.New("LXP_INVALID_AMOUNT")
		}
		if leg.Amount.BitLen() > 128 {
			return nil, errors.New("LXP_OVERFLOW")
		}
	}
	payload := make([]byte, 34+transferLegBytes*len(legs))
	copy(payload[:32], programID[:])
	binary.BigEndian.PutUint16(payload[32:34], uint16(len(legs)))
	for i, leg := range legs {
		encoded := payload[34+i*transferLegBytes : 34+(i+1)*transferLegBytes]
		copy(encoded[:32], leg.From[:])
		copy(encoded[32:64], leg.Asset[:])
		copy(encoded[64:96], leg.To[:])
		leg.Amount.FillBytes(encoded[96:112])
	}
	return payload, nil
}

func EncodeAccountPayload(programID [32]byte, profile uint8, assetID [32]byte, seed []byte) ([]byte, error) {
	if profile != 1 && profile != 2 {
		return nil, errors.New("LXP_NON_CANONICAL")
	}
	if len(seed) > maxAccountSeedBytes {
		return nil, errors.New("LXP_NON_CANONICAL")
	}
	payload := make([]byte, 73+len(seed))
	copy(payload[:32], programID[:])
	copy(payload[32:37], "LXPA")
	payload[36] = '0' + profile
	copy(payload[37:69], assetID[:])
	binary.BigEndian.PutUint32(payload[69:73], uint32(len(seed)))
	copy(payload[73:], seed)
	return payload, nil
}
