package layerxchains

import (
	"errors"

	chains "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
)

func RevertError(err error) []byte {
	if err == nil {
		return nil
	}
	for _, entry := range []struct {
		err  error
		code string
	}{
		{chains.ErrNotOperator, "LXP_UNAUTHORIZED_DEBIT"},
		{chains.ErrNotAuthority, "LXP_AUTH_SCOPE"},
		{chains.ErrBondTooLow, "LXP_INVALID_AMOUNT"},
		{chains.ErrInvalidName, "LXP_NON_CANONICAL"},
		{chains.ErrNameTaken, "LXP_DUPLICATE_ENTRY"},
		{chains.ErrAllowanceOutOfRange, "LXP_LENGTH_LIMIT"},
		{chains.ErrTotalAllowanceExceeded, "LXP_LENGTH_LIMIT"},
		{chains.ErrTooManyDomains, "LXP_LENGTH_LIMIT"},
		{chains.ErrUnknownDomain, "LXP_UNKNOWN_FIELD"},
		{chains.ErrDomainNotActive, "LXP_AUTH_SCOPE"},
		{chains.ErrDomainRetired, "LXP_AUTH_REVOKED"},
	} {
		if errors.Is(err, entry.err) {
			return pcommon.LXPRevert(entry.code, err.Error())
		}
	}
	return pcommon.LXPReason(err)
}
