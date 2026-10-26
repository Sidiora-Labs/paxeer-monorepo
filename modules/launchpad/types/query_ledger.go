package types

import sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"

func ParseQuoteLedgerKind(kind string) (QuoteLedgerKind, error) {
	switch kind {
	case "held":
		return QuoteLedgerKind_QUOTE_LEDGER_KIND_HELD, nil
	case "fee-held":
		return QuoteLedgerKind_QUOTE_LEDGER_KIND_FEE_HELD, nil
	case "treasury-held":
		return QuoteLedgerKind_QUOTE_LEDGER_KIND_TREASURY_HELD, nil
	default:
		return QuoteLedgerKind_QUOTE_LEDGER_KIND_UNSPECIFIED, sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "unknown quote ledger kind %q", kind)
	}
}

func FeeAccountingInfoOf(a FeeAccounting) FeeAccountingInfo {
	return FeeAccountingInfo{
		PendingProtocolFees:       a.PendingProtocolFees,
		TotalLiabilities:          a.TotalLiabilities,
		AccountingInitialized:     a.AccountingInitialized,
		AccountingMigrationCursor: a.AccountingMigrationCursor,
		PoolCount:                 uint64(len(a.Pools)),
		EpochCount:                uint64(len(a.Epochs)),
		ClaimCount:                uint64(len(a.Claims)),
	}
}
