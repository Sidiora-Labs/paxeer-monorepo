package types

func ValidateLeaders(leaders [][32]byte) error {
	if len(leaders) == 0 {
		return ErrInvalidGenesis.Wrap("leader list is empty")
	}
	seen := make(map[[32]byte]struct{}, len(leaders))
	for i, leader := range leaders {
		if leader == ([32]byte{}) {
			return ErrInvalidGenesis.Wrapf("leader %d is the zero key", i)
		}
		if _, ok := seen[leader]; ok {
			return ErrInvalidGenesis.Wrapf("leader %d repeats an earlier leader", i)
		}
		seen[leader] = struct{}{}
	}
	return nil
}
