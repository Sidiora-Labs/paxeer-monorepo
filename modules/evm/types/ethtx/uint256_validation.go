package ethtx

import (
	"fmt"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func validateRequiredUint256(name string, value *sdk.Int) error {
	if value == nil || value.IsNil() {
		return fmt.Errorf("%s must be present", name)
	}
	if value.IsNegative() || !IsValidInt256(value.BigInt()) {
		return fmt.Errorf("%s must be an unsigned 256-bit integer", name)
	}
	return nil
}
