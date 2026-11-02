package common

import (
	"errors"
	"fmt"

	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
)

func RevertError(err error) error {
	var code ErrorCode
	if errors.As(err, &code) {
		return err
	}
	if code := paytypes.PayRevertCode(err); code != "" {
		return fmt.Errorf("%s: %w", code, err)
	}
	return err
}
