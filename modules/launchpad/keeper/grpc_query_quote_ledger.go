package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/common"
)

func (q Querier) QuoteLedger(goCtx context.Context, req *types.QueryQuoteLedgerRequest) (*types.QueryQuoteLedgerResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	kind, err := types.ParseQuoteLedgerKind(req.Kind)
	if err != nil {
		return nil, err
	}
	var ledger QuoteLedger
	switch kind {
	case types.QuoteLedgerKind_QUOTE_LEDGER_KIND_HELD, types.QuoteLedgerKind_QUOTE_LEDGER_KIND_FEE_HELD:
		if !common.IsHexAddress(req.Address) {
			return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid quote ledger address %q", req.Address)
		}
		owner := common.HexToAddress(req.Address)
		if kind == types.QuoteLedgerKind_QUOTE_LEDGER_KIND_HELD {
			ledger = HeldQuoteLedger(owner)
		} else {
			ledger = FeeHeldLedger(owner)
		}
	case types.QuoteLedgerKind_QUOTE_LEDGER_KIND_TREASURY_HELD:
		ledger = TreasuryHeldLedger()
	}
	amount, err := q.Keeper.GetQuoteLedger(ctx, ledger)
	if err != nil {
		return nil, err
	}
	return &types.QueryQuoteLedgerResponse{Amount: amount.String()}, nil
}
