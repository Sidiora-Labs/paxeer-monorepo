package receipt

import (
	"context"
	"fmt"
	"strings"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/ethereum/go-ethereum/common"
)

// tenantReceiptActors consumes the same source-attested envelopes and market
// bindings used to issue fills. Domain admission and signature recovery remain
// mandatory; no substitute root-signed transaction is constructed for decoding.
func tenantReceiptActors(ctx context.Context, a Qualification, b Bundle, markets []decode.ActorMarketBinding, decoded []*decode.DecodedEnvelope) ([]*decode.DecodedEnvelope, error) {
	domain, ok := a.(signedDomainQualification)
	if !ok || b.Transaction == nil || !b.Transaction.Protected() || b.Transaction.ChainId().Uint64() == 125 {
		return nil, ErrQualification
	}
	if err := domain.ValidateSignedDomain(ctx, b); err != nil {
		return nil, err
	}
	sender, err := coordinates(b)
	if err != nil || len(markets) > 64 || len(decoded) != len(b.Receipt.Logs) {
		return nil, ErrQualification
	}
	bindings := map[common.Address]decode.ActorMarketBinding{}
	for _, market := range markets {
		if market.Pool == (common.Address{}) || market.Token == (common.Address{}) || market.Settlement == (common.Address{}) || market.Token == market.Settlement {
			return nil, ErrQualification
		}
		if _, exists := bindings[market.Pool]; exists {
			return nil, ErrQualification
		}
		bindings[market.Pool] = market
	}
	out, err := clone(decoded)
	if err != nil {
		return nil, err
	}
	economicCount := 0
	for _, env := range out {
		if env == nil {
			return nil, ErrQualification
		}
		if env.Actors != nil {
			origin := sender
			env.Actors.TransactionOrigin = &origin
			economicCount++
		}
	}
	if economicCount > 0 && len(out) > (1<<20)/economicCount {
		for _, env := range out {
			if env.Actors != nil {
				env.Alerts = append(env.Alerts, "actor_unavailable:comparison_bound")
			}
		}
		return out, nil
	}
	for i, env := range out {
		if env.Actors == nil || env.Event == nil || !qualifiedSwapFamily(env, b.Receipt.Logs[i]) {
			continue
		}
		poolID, ok := env.Event.Args["poolId"].(string)
		if !ok || len(poolID) != 66 {
			return nil, ErrQualification
		}
		word := common.HexToHash(poolID)
		for _, v := range word[:12] {
			if v != 0 {
				return nil, ErrQualification
			}
		}
		market, admitted := bindings[common.BytesToAddress(word[12:])]
		if !admitted {
			return nil, fmt.Errorf("%w: source-attested actor market binding required", ErrQualification)
		}
		outputToken := market.Token
		buy, ok := env.Event.Args["isBuy"].(bool)
		if !ok {
			return nil, ErrQualification
		}
		if !buy {
			outputToken = market.Settlement
		}
		var matches []*decode.DecodedEnvelope
		for _, transfer := range out {
			if transfer.Event == nil || transfer.Status != valueDecoded || transfer.EmitterRole != valueSidioraERC20 || common.HexToAddress(transfer.Raw.Emitter) != outputToken || transfer.EventName != "Transfer" || transfer.Raw.Order.Log >= env.Raw.Order.Log {
				continue
			}
			if transfer.Event.Args["from"] == strings.ToLower(market.Pool.Hex()) && transfer.Event.Args["value"] == env.Event.Args["amountOut"] {
				matches = append(matches, transfer)
			}
		}
		if len(matches) != 1 {
			continue
		}
		recipient, ok := matches[0].Event.Args["to"].(string)
		if !ok || !common.IsHexAddress(recipient) || common.HexToAddress(recipient) == (common.Address{}) {
			continue
		}
		env.Actors.Recipient = &recipient
		env.Actors.Beneficiary = &recipient
		env.Actors.AttributionConfidence = "receipt_evidence"
		env.Actors.AttributionMethod = "unique_pool_output_transfer"
		env.Actors.EvidenceIdentity = append(env.Actors.EvidenceIdentity, matches[0].Raw.Identity)
	}
	return out, nil
}
