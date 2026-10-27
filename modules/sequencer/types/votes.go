package types

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
)

func EncodeStoredVotes(votes []abci.VoteInfo) ([]byte, error) {
	if uint64(len(votes)) > math.MaxUint32 {
		return nil, fmt.Errorf("stored votes: count %d exceeds uint32", len(votes))
	}
	size := 4
	for i, vote := range votes {
		addressLen := len(vote.Validator.Address)
		if addressLen > math.MaxUint8 {
			return nil, fmt.Errorf("stored votes: vote %d address length %d exceeds 255", i, addressLen)
		}
		entrySize := 1 + addressLen + 8 + 1
		if size > int(^uint(0)>>1)-entrySize {
			return nil, fmt.Errorf("stored votes: encoded size exceeds int")
		}
		size += entrySize
	}

	bz := make([]byte, 4, size)
	binary.BigEndian.PutUint32(bz, uint32(len(votes)))
	for _, vote := range votes {
		bz = append(bz, byte(len(vote.Validator.Address)))
		bz = append(bz, vote.Validator.Address...)
		bz = binary.BigEndian.AppendUint64(bz, uint64(vote.Validator.Power))
		signed := byte(0)
		if vote.SignedLastBlock {
			signed = 1
		}
		bz = append(bz, signed)
	}
	return bz, nil
}

func DecodeStoredVotes(bz []byte) ([]abci.VoteInfo, error) {
	if len(bz) < 4 {
		return nil, fmt.Errorf("stored votes: truncated count")
	}
	count := binary.BigEndian.Uint32(bz[:4])
	bz = bz[4:]
	if uint64(count) > uint64(len(bz)/10) {
		return nil, fmt.Errorf("stored votes: count %d exceeds available entries", count)
	}
	votes := make([]abci.VoteInfo, int(count))
	for i := range votes {
		if len(bz) < 1 {
			return nil, fmt.Errorf("stored votes: vote %d truncated address length", i)
		}
		addressLen := int(bz[0])
		bz = bz[1:]
		if len(bz) < addressLen+8+1 {
			return nil, fmt.Errorf("stored votes: vote %d truncated entry", i)
		}
		address := bytes.Clone(bz[:addressLen])
		bz = bz[addressLen:]
		power := int64(binary.BigEndian.Uint64(bz[:8]))
		signed := bz[8]
		if signed > 1 {
			return nil, fmt.Errorf("stored votes: vote %d invalid signed flag %d", i, signed)
		}
		bz = bz[9:]
		votes[i] = abci.VoteInfo{
			Validator:       abci.Validator{Address: address, Power: power},
			SignedLastBlock: signed == 1,
		}
	}
	if len(bz) != 0 {
		return nil, fmt.Errorf("stored votes: %d trailing bytes", len(bz))
	}
	return votes, nil
}
