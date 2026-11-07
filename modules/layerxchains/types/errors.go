package types

import sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"

var (
	ErrInvalidName            = sdkerrors.Register(ModuleName, 2, "invalid chain name")
	ErrNameTaken              = sdkerrors.Register(ModuleName, 3, "chain name already reserved")
	ErrNotOperator            = sdkerrors.Register(ModuleName, 4, "caller is not the operator")
	ErrNotAuthority           = sdkerrors.Register(ModuleName, 5, "caller is not the authority")
	ErrBondTooLow             = sdkerrors.Register(ModuleName, 6, "insufficient bond or payment")
	ErrAllowanceOutOfRange    = sdkerrors.Register(ModuleName, 7, "allowance out of range")
	ErrTotalAllowanceExceeded = sdkerrors.Register(ModuleName, 8, "total active gas allowance exceeded")
	ErrUnknownDomain          = sdkerrors.Register(ModuleName, 9, "unknown domain")
	ErrDomainNotActive        = sdkerrors.Register(ModuleName, 10, "domain is not active")
	ErrDomainRetired          = sdkerrors.Register(ModuleName, 11, "domain is retired")
	ErrTooManyDomains         = sdkerrors.Register(ModuleName, 12, "maximum chain count reached")
	ErrInvalidGenesis         = sdkerrors.Register(ModuleName, 13, "invalid genesis")
)
