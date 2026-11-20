package receipt

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

func qualificationLabel(value string) string {
	if value == "" || len(value) > 64 {
		return "unknown"
	}
	for _, ch := range value {
		if ch != '_' && (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') {
			return "unknown"
		}
	}
	return value
}

type qualificationIdentityError struct {
	role, kind, cause string
	address           common.Address
}

func (e *qualificationIdentityError) Error() string {
	return fmt.Sprintf("%s: role=%s kind=%s address=%s cause=%s", ErrQualification, e.role, e.kind, strings.ToLower(e.address.Hex()), e.cause)
}

func (e *qualificationIdentityError) Unwrap() error { return ErrQualification }

func qualificationRefusal(role, kind string, address common.Address, cause string) error {
	return &qualificationIdentityError{role: qualificationLabel(role), kind: qualificationLabel(kind), address: address, cause: cause}
}

func qualificationContext(err error, role, kind string, address common.Address, cause string) error {
	var refused *qualificationIdentityError
	if errors.As(err, &refused) {
		return refused
	}
	return qualificationRefusal(role, kind, address, cause)
}

func (e RPCCodeEpoch) refusal(cause string) error {
	return qualificationRefusal(e.Deployment.Role, e.Identity.Kind, e.Deployment.Address, cause)
}

func discoveryKind(role, kind string) string {
	if role != valueSidioraPool {
		return valueDirect
	}
	if kind == "" {
		return valueBeaconProxy
	}
	return kind
}

func (a *RPCQualification) discoveryKind(role string) string {
	for _, template := range a.options.Discovery {
		if template.Role == role {
			return discoveryKind(role, template.Kind)
		}
	}
	return "unknown"
}
