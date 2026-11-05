package types

import (
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

// codespace is the layerxledger module name.
const codespace = "layerxledger"

// AssetPausedCode is the wire code shared by ledger, tokenfactory and bank.
const AssetPausedCode = "LXP_ASSET_PAUSED"

// Each error code is the magnitude of its kernel result code
// (include/layerx/lxp_result.h) and each text is the kernel result name with
// its ERR_ prefix removed.
var (
	ErrNonCanonical      = sdkerrors.Register(codespace, 3, "LXP_NON_CANONICAL")
	ErrMalformedSend     = sdkerrors.Register(codespace, 107, "LXP_MALFORMED_SEND")
	ErrUnauthorizedDebit = sdkerrors.Register(codespace, 207, "LXP_UNAUTHORIZED_DEBIT")
	ErrContextMismatch   = sdkerrors.Register(codespace, 213, "LXP_CONTEXT_MISMATCH")
	// ErrNoPayerGrant also refuses retired issue/revoke/read routes (decision 2340).
	// The shared LXP registry already publishes code -214.
	ErrNoPayerGrant           = sdkerrors.Register(codespace, 214, "LXP_NO_PAYER_GRANT")
	ErrIdempotentReplay       = sdkerrors.Register(codespace, 302, "LXP_IDEMPOTENT_REPLAY")
	ErrExpired                = sdkerrors.Register(codespace, 303, "LXP_EXPIRED")
	ErrSequenceMismatch       = sdkerrors.Register(codespace, 305, "LXP_SEQUENCE_MISMATCH")
	ErrSequenceExhausted      = sdkerrors.Register(codespace, 307, "LXP_SEQUENCE_EXHAUSTED")
	ErrInsufficientBalance    = sdkerrors.Register(codespace, 400, "LXP_INSUFFICIENT_BALANCE")
	ErrZeroAmount             = sdkerrors.Register(codespace, 401, "LXP_ZERO_AMOUNT")
	ErrAssetMismatch          = sdkerrors.Register(codespace, 402, "LXP_ASSET_MISMATCH")
	ErrAssetPaused            = sdkerrors.Register(codespace, 403, AssetPausedCode)
	ErrAssetAlreadyRegistered = sdkerrors.Register(codespace, 411, "LXP_ASSET_ALREADY_REGISTERED")
	ErrPausedScope            = sdkerrors.Register(codespace, 735, "LXP_PAUSED_SCOPE")
	ErrArenaExhausted         = sdkerrors.Register(codespace, 905, "LXP_ARENA_EXHAUSTED")
	ErrFatalSupplyMismatch    = sdkerrors.Register(codespace, 1003, "LXP_FATAL_SUPPLY_MISMATCH")
)
