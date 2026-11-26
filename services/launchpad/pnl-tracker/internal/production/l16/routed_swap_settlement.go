package l16

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/core/types"
)

type receiptSettlementCandidate struct {
	fill             accounting.FillV1
	token, quote     movement
	payer, recipient string
}

func routedSettlementWallet(sell, buy receiptSettlementCandidate) string {
	wallet, intermediate := sell.token.From, sell.quote.To
	if sell.fill.Side != sideSell || buy.fill.Side != sideBuy || sell.fill.Token.Address == buy.fill.Token.Address || sell.fill.Settlement != buy.fill.Settlement || wallet == zeroAddress || buy.token.To != wallet || intermediate == wallet || intermediate == zeroAddress || buy.quote.From != intermediate || sell.fill.USDLAmountRaw != buy.fill.USDLAmountRaw {
		return ""
	}
	return wallet
}

func receiptSettlementForFills(fill accounting.FillV1, fills []accounting.FillV1, receipt *types.Receipt, requirePair bool) (receiptSettlement, error) {
	var result receiptSettlement
	used := map[uint32]bool{}
	candidates := make([]receiptSettlementCandidate, 0, len(fills))
	for _, f := range fills {
		end, e := strconv.ParseUint(f.Position.LogIndex, 10, 32)
		if e != nil {
			return result, e
		}
		holder := settlementAssetHolder(f)
		var tokenCandidates, quoteCandidates []movement
		for _, log := range receipt.Logs {
			if log == nil {
				return result, delivery.ErrIntegrity
			}
			if uint64(log.Index) >= end || uint64(log.Index) > uint64(^uint32(0)) || used[uint32(log.Index)] || len(log.Topics) == 0 || log.Topics[0] != transferTopic {
				continue
			}
			emitter := strings.ToLower(log.Address.Hex())
			if emitter != f.Token.Address && emitter != f.Settlement.Address {
				continue
			}
			m, e := decodeTransfer(*log)
			if e != nil {
				return result, e
			}
			if m.TransactionHash != f.Position.TransactionHash || m.BlockHash != f.Position.BlockHash || strconv.FormatInt(m.Height, 10) != f.Position.Height || strconv.FormatUint(uint64(m.TransactionIndex), 10) != f.Position.TransactionIndex {
				return result, delivery.ErrIntegrity
			}
			if emitter == f.Token.Address && m.Amount == f.TokenAmountRaw && (f.Side == sideBuy && m.From == f.PoolAddress || f.Side == sideSell && m.To == f.PoolAddress) {
				tokenCandidates = append(tokenCandidates, m)
			}
			if emitter == f.Settlement.Address && m.Amount == f.USDLAmountRaw && (f.Side == sideBuy && m.To == holder || f.Side == sideSell && m.From == holder) {
				quoteCandidates = append(quoteCandidates, m)
			}
		}
		if len(tokenCandidates) != 1 || len(quoteCandidates) != 1 {
			return result, ErrCostAllocationUnavailable
		}
		token, quote := tokenCandidates[0], quoteCandidates[0]
		used[token.LogIndex] = true
		used[quote.LogIndex] = true
		payer, recipient := quote.From, token.To
		if f.Side == sideSell {
			payer, recipient = token.From, quote.To
		}
		if payer == zeroAddress || recipient == zeroAddress {
			return result, ErrCostAllocationUnavailable
		}
		candidates = append(candidates, receiptSettlementCandidate{fill: f, token: token, quote: quote, payer: payer, recipient: recipient})
	}
	paired := make([]bool, len(candidates))
	for i := 0; i+1 < len(candidates); i++ {
		sell, buy := &candidates[i], &candidates[i+1]
		wallet := routedSettlementWallet(*sell, *buy)
		if wallet == "" {
			if sell.fill.Side == sideSell && buy.fill.Side == sideBuy && (sell.payer != sell.recipient || buy.payer != buy.recipient) {
				return result, ErrCostAllocationUnavailable
			}
			continue
		}
		sell.payer, sell.recipient = wallet, wallet
		buy.payer, buy.recipient = wallet, wallet
		paired[i], paired[i+1] = true, true
		i++
	}
	if requirePair && (len(candidates) != 2 || !paired[0] || !paired[1]) {
		return result, ErrCostAllocationUnavailable
	}
	type settlementAccount struct{ asset, wallet string }
	expectedNet := map[settlementAccount]*big.Int{}
	found := false
	for _, candidate := range candidates {
		f, token := candidate.fill, candidate.token
		payer, recipient := candidate.payer, candidate.recipient
		if payer == recipient {
			account := settlementAccount{f.Settlement.Address, payer}
			if expectedNet[account] == nil {
				expectedNet[account] = new(big.Int)
			}
			amount, parseErr := Uint256(f.USDLAmountRaw)
			if parseErr != nil {
				return result, parseErr
			}
			if f.Side == sideBuy {
				expectedNet[account].Sub(expectedNet[account], amount)
			} else {
				expectedNet[account].Add(expectedNet[account], amount)
			}
		}
		if f.SourceFillID == fill.SourceFillID {
			if found {
				return result, delivery.ErrIntegrity
			}
			found = true
			if receipt.EffectiveGasPrice == nil || receipt.EffectiveGasPrice.Sign() < 0 {
				return result, delivery.ErrIntegrity
			}
			result = receiptSettlement{fill: f, tokenMovement: token, payer: payer, recipient: recipient, gas: new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasUsed), receipt.EffectiveGasPrice).String(), digest: f.Provenance.ReceiptSHA256}
		}
	}
	if !found {
		return result, delivery.ErrGap
	}
	actualNet := map[settlementAccount]*big.Int{}
	settlementAssets := map[string]bool{}
	for account := range expectedNet {
		actualNet[account] = new(big.Int)
		settlementAssets[account.asset] = true
	}
	for _, log := range receipt.Logs {
		if log == nil {
			return result, delivery.ErrIntegrity
		}
		if len(log.Topics) == 0 || log.Topics[0] != transferTopic {
			continue
		}
		asset := strings.ToLower(log.Address.Hex())
		if !settlementAssets[asset] {
			continue
		}
		m, decodeErr := decodeTransfer(*log)
		if decodeErr != nil || m.TransactionHash != fill.Position.TransactionHash || m.BlockHash != fill.Position.BlockHash || strconv.FormatInt(m.Height, 10) != fill.Position.Height || strconv.FormatUint(uint64(m.TransactionIndex), 10) != fill.Position.TransactionIndex {
			return result, delivery.ErrIntegrity
		}
		amount, parseErr := Uint256(m.Amount)
		if parseErr != nil {
			return result, parseErr
		}
		if balance := actualNet[settlementAccount{asset, m.To}]; balance != nil {
			balance.Add(balance, amount)
		}
		if balance := actualNet[settlementAccount{asset, m.From}]; balance != nil {
			balance.Sub(balance, amount)
		}
	}
	for account, expected := range expectedNet {
		if expected.Cmp(actualNet[account]) != 0 {
			return result, ErrCostAllocationUnavailable
		}
	}
	return result, nil
}
