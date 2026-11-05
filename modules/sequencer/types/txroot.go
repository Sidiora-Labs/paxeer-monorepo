package types

import "crypto/sha256"

// TxRoot commits to the ordered, fixed-width activity identifiers. With no
// activities it returns sha256 of the empty concatenation, including for nil.
func TxRoot(activityIDs [][32]byte) [32]byte {
	root := sha256.New()
	for _, id := range activityIDs {
		root.Write(id[:])
	}
	var out [32]byte
	copy(out[:], root.Sum(nil))
	return out
}
