package types

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
)

func LeaderPinFromSchedule(activationHeight int64, leaders [][32]byte) (LeaderPin, error) {
	if activationHeight == 0 {
		return LeaderPin{}, nil
	}
	keys := make([]crypto.PubKey, len(leaders))
	for i, leader := range leaders {
		key, err := ed25519.PublicKeyFromBytes(leader[:])
		if err != nil {
			return LeaderPin{}, err
		}
		keys[i] = key
	}
	return NewLeaderPin(activationHeight, keys)
}

func (p LeaderPin) Applies(height int64) bool {
	return len(p.leaders) > 0 && height >= p.activationHeight
}
