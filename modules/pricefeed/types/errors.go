package types

import (
	"errors"

	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

const ModuleName = "pricefeed"

var (
	ErrUnauthorized        = sdkerrors.Register(ModuleName, 2, "unauthorized")
	ErrInvalidParams       = sdkerrors.Register(ModuleName, 3, "invalid params")
	ErrInvalidGenesis      = sdkerrors.Register(ModuleName, 4, "invalid genesis")
	ErrNotValidator        = sdkerrors.Register(ModuleName, 5, "not a bonded validator")
	ErrInvalidKey          = sdkerrors.Register(ModuleName, 6, "feed key must be a 33-byte compressed secp256k1 public key")
	ErrKeyInUse            = sdkerrors.Register(ModuleName, 7, "feed account is registered to another validator")
	ErrNoKey               = sdkerrors.Register(ModuleName, 8, "validator has no feed key")
	ErrUnknownMarket       = sdkerrors.Register(ModuleName, 9, "market is not listed")
	ErrUnregisteredFeeder  = sdkerrors.Register(ModuleName, 10, "feeder is not a registered feed account")
	ErrRateLimited         = sdkerrors.Register(ModuleName, 11, "the validator has already submitted prices at this height")
	ErrTooManyObservations = sdkerrors.Register(ModuleName, 12, "too many observations in one submission")
	ErrSubmissionTooLarge  = sdkerrors.Register(ModuleName, 13, "submission is over the byte bound")
	ErrDuplicateMarket     = sdkerrors.Register(ModuleName, 14, "a submission carries one observation per market")
	ErrSubmissionAlone     = sdkerrors.Register(ModuleName, 15, "price submissions cannot be in the same tx as other messages")
	ErrInvalidObservation  = sdkerrors.Register(ModuleName, 16, "invalid observation")
)

type LXPCode struct {
	Err  *sdkerrors.Error
	Name string
}

func LXPCodes() []LXPCode {
	return []LXPCode{
		{ErrUnauthorized, "LXP_PRICEFEED_UNAUTHORIZED"},
		{ErrInvalidParams, "LXP_PRICEFEED_INVALID_PARAMS"},
		{ErrInvalidGenesis, "LXP_PRICEFEED_INVALID_GENESIS"},
		{ErrNotValidator, "LXP_PRICEFEED_NOT_VALIDATOR"},
		{ErrInvalidKey, "LXP_PRICEFEED_INVALID_KEY"},
		{ErrKeyInUse, "LXP_PRICEFEED_KEY_IN_USE"},
		{ErrNoKey, "LXP_PRICEFEED_NO_KEY"},
		{ErrUnknownMarket, "LXP_PRICEFEED_UNKNOWN_MARKET"},
		{ErrUnregisteredFeeder, "LXP_PRICEFEED_UNREGISTERED_FEEDER"},
		{ErrRateLimited, "LXP_PRICEFEED_RATE_LIMITED"},
		{ErrTooManyObservations, "LXP_PRICEFEED_TOO_MANY_OBSERVATIONS"},
		{ErrSubmissionTooLarge, "LXP_PRICEFEED_SUBMISSION_TOO_LARGE"},
		{ErrDuplicateMarket, "LXP_PRICEFEED_DUPLICATE_MARKET"},
		{ErrSubmissionAlone, "LXP_PRICEFEED_SUBMISSION_ALONE"},
		{ErrInvalidObservation, "LXP_PRICEFEED_INVALID_OBSERVATION"},
	}
}

func LXPName(err error) (string, bool) {
	for _, code := range LXPCodes() {
		if errors.Is(err, code.Err) {
			return code.Name, true
		}
	}
	return "", false
}
