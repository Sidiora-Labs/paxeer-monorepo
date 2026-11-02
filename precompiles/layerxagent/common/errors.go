package common

import (
	"errors"
	"fmt"
	"strings"

	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
)

// RevertError preserves agent errors and adds the wire code to pay errors while
// retaining their cause. Mapping an already mapped error preserves its identity.
func RevertError(err error) error {
	var code ErrorCode
	if errors.As(err, &code) {
		return err
	}
	if code := paytypes.PayRevertCode(err); code != "" {
		if strings.HasPrefix(err.Error(), code+": ") {
			return err
		}
		return fmt.Errorf("%s: %w", code, err)
	}
	return err
}
