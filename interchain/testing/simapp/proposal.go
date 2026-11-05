package simapp

import (
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

// newProcessProposalHandler validates decoded transactions and signed ante state
// in an isolated cache. Message execution belongs to FinalizeBlock: a transaction
// whose handler fails is still a valid block transaction with a committed result.
func newProcessProposalHandler(decode sdk.TxDecoder, ante sdk.AnteHandler) sdk.ProcessProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		reject := &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}
		proposalCtx, _ := ctx.WithIsCheckTx(false).WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)).CacheContext()
		var maxGas, totalGas uint64
		if params := ctx.ConsensusParams(); params != nil && params.Block != nil && params.Block.MaxGas > 0 {
			maxGas = uint64(params.Block.MaxGas)
		}
		for _, txBytes := range req.Txs {
			tx, err := decode(txBytes)
			if err != nil || tx == nil {
				return reject, nil
			}
			if err := tx.ValidateBasic(); err != nil {
				return reject, nil
			}
			feeTx, ok := tx.(sdk.FeeTx)
			if !ok {
				return reject, nil
			}
			if maxGas > 0 {
				if feeTx.GetGas() > maxGas-totalGas {
					return reject, nil
				}
				totalGas += feeTx.GetGas()
			}
			txCtx, write := proposalCtx.CacheContext()
			if _, err := ante(txCtx.WithTxBytes(txBytes), tx, false); err != nil {
				return reject, nil
			}
			// Later transactions in this proposal must see earlier ante sequences.
			// This cache is discarded when validation returns, including on rejection.
			write()
		}
		return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
	}
}
