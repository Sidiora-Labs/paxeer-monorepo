package bringup

import "errors"

func IdentityAction(state string, days, renewDays int64) (string, error) {
	if renewDays < 0 {
		return "", errors.New("invalid renewal window")
	}
	switch state {
	case "absent":
		return "issue", nil
	case "expired", "incompatible":
		return "renew", nil
	case "present":
		if days > renewDays {
			return "reuse", nil
		}
		return "renew", nil
	case "foreign", "unreadable", "partial", "inconsistent":
		return "", errors.New("retained identity must be reconciled before issuance")
	default:
		return "", errors.New("unknown retained identity state")
	}
}
