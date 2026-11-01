package types

import "crypto/sha256"

func TxRoot(activityIDs [][32]byte) [32]byte {
	root := sha256.New()
	for _, id := range activityIDs {
		root.Write(id[:])
	}
	var out [32]byte
	copy(out[:], root.Sum(nil))
	return out
}
