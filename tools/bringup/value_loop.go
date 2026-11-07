package bringup

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/url"

	"github.com/ethereum/go-ethereum/core/types"
)

// ReadValueLoopTransaction binds committed evidence to the retained signed
// transaction and the explicitly selected root chain.
func ReadValueLoopTransaction(ctx context.Context, client *http.Client, endpoint, hash, chainID string, raw []byte) (CommittedTransaction, error) {
	invalid := errors.New("value-loop transaction or chain identity mismatch")
	expected, ok := new(big.Int).SetString(chainID, 10)
	if !ok || expected.Sign() <= 0 || expected.String() != chainID {
		return CommittedTransaction{}, invalid
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Path != "" && u.Path != "/") {
		return CommittedTransaction{}, errors.New("value-loop requires the root RPC")
	}
	var tx types.Transaction
	if tx.UnmarshalBinary(raw) != nil || !tx.Protected() || tx.ChainId().Cmp(expected) != 0 || tx.Hash().Hex() != hash {
		return CommittedTransaction{}, invalid
	}
	if _, err := types.Sender(types.LatestSignerForChainID(expected), &tx); err != nil {
		return CommittedTransaction{}, invalid
	}
	checkChain := func() error {
		encoded, err := callRPC(ctx, client, endpoint, "eth_chainId", []any{}, "", false)
		var text string
		if err != nil || json.Unmarshal(encoded, &text) != nil {
			return invalid
		}
		actual, err := quantity(text)
		if err != nil || actual.Cmp(expected) != 0 {
			return invalid
		}
		return nil
	}
	if err := checkChain(); err != nil {
		return CommittedTransaction{}, err
	}
	evidence, err := ReadCommittedTransaction(ctx, client, endpoint, hash)
	if err != nil {
		return CommittedTransaction{}, err
	}
	encoded, err := callRPC(ctx, client, endpoint, "eth_blockNumber", []any{}, "", false)
	var text string
	if err != nil || json.Unmarshal(encoded, &text) != nil {
		return CommittedTransaction{}, errors.New("value-loop committed head unavailable")
	}
	head, err := quantity(text)
	height, heightErr := quantity(evidence.Height)
	if err != nil || heightErr != nil || head.Cmp(height) < 0 {
		return CommittedTransaction{}, errors.New("value-loop committed head precedes inclusion")
	}
	if err := checkChain(); err != nil {
		return CommittedTransaction{}, err
	}
	return evidence, nil
}
