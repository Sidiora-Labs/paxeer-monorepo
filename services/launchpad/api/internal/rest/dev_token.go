package rest

import (
	"context"
	"math/big"
	"strconv"
	"strings"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

const developmentTokenReadABI = `[
 {"type":"function","name":"name","inputs":[],"outputs":[{"type":"string"}]},
 {"type":"function","name":"symbol","inputs":[],"outputs":[{"type":"string"}]},
 {"type":"function","name":"decimals","inputs":[],"outputs":[{"type":"uint8"}]},
 {"type":"function","name":"totalSupply","inputs":[],"outputs":[{"type":"uint256"}]}
]`

// tokenDocument reads each ERC20 value at the response's exact canonical
// watermark. Unsupported methods remain unavailable independently. The last
// transaction/log of that block anchors the block-end read to a real position.
func (c *developmentChain) tokenDocument(ctx context.Context, token string, wm store.ChainWatermark) (map[string]any, error) {
	row := store.PoolRegistryRow{Token: token}
	doc := tokenDocument(row)
	if c == nil || c.client == nil {
		return doc, nil
	}
	var block *struct {
		Hash         string   `json:"hash"`
		Number       string   `json:"number"`
		Transactions []string `json:"transactions"`
	}
	if err := c.client.CallContext(ctx, &block, "eth_getBlockByHash", wm.BlockHash, false); err != nil {
		return nil, errChainReferenceUnavailable
	}
	if block == nil || !strings.EqualFold(block.Hash, wm.BlockHash) {
		return nil, errChainReferenceUnavailable
	}
	height, err := hexutil.DecodeUint64(block.Number)
	if err != nil || strconv.FormatUint(height, 10) != wm.Height {
		return nil, store.ErrDataIntegrity
	}
	if len(block.Transactions) == 0 {
		return doc, nil
	}
	hash := block.Transactions[len(block.Transactions)-1]
	var receipt *struct {
		rpcReceipt
		TransactionHash string   `json:"transactionHash"`
		Logs            []rpcLog `json:"logs"`
	}
	if err = c.client.CallContext(ctx, &receipt, "eth_getTransactionReceipt", hash); err != nil || receipt == nil {
		return nil, errChainReferenceUnavailable
	}
	txIndex, err := hexutil.DecodeUint64(receipt.TransactionIndex)
	if err != nil || !strings.EqualFold(receipt.BlockHash, wm.BlockHash) || receipt.BlockNumber != block.Number || !strings.EqualFold(receipt.TransactionHash, hash) {
		return nil, store.ErrDataIntegrity
	}
	logIndex := uint64(0)
	if len(receipt.Logs) > 0 {
		logIndex, err = hexutil.DecodeUint64(receipt.Logs[len(receipt.Logs)-1].LogIndex)
		if err != nil {
			return nil, store.ErrDataIntegrity
		}
	}
	pos := position(height, txIndex, logIndex, hash, wm.BlockHash)
	if accounting.ValidateSourcePosition(pos) != nil {
		return nil, store.ErrDataIntegrity
	}
	contract, err := abi.JSON(strings.NewReader(developmentTokenReadABI))
	if err != nil {
		return nil, err
	}
	names := []string{"name", "symbol", "decimals", "totalSupply"}
	results := make([]hexutil.Bytes, len(names))
	batch := make([]rpc.BatchElem, len(names))
	for i, name := range names {
		batch[i] = rpc.BatchElem{Method: "eth_call", Args: []any{map[string]any{"to": token, "data": hexutil.Encode(contract.Methods[name].ID)}, map[string]any{"blockHash": wm.BlockHash, "requireCanonical": true}}, Result: &results[i]}
	}
	if err = c.client.BatchCallContext(ctx, batch); err != nil {
		return nil, errChainReferenceUnavailable
	}
	values := make(map[string]any, len(names))
	for i, name := range names {
		if batch[i].Error != nil || len(results[i]) == 0 || len(results[i]) > store.PublicJSONMaxBytes {
			continue
		}
		out, decodeErr := contract.Unpack(name, results[i])
		if decodeErr == nil && len(out) == 1 {
			values[name] = out[0]
		}
	}
	name, hasName := values["name"].(string)
	symbol, hasSymbol := values["symbol"].(string)
	decimals, hasDecimals := values["decimals"].(uint8)
	supply, hasSupply := values["totalSupply"].(*big.Int)
	if hasSupply && (supply.Sign() < 0 || supply.BitLen() > 256) {
		return nil, store.ErrDataIntegrity
	}
	if hasName && hasSymbol && hasDecimals && hasSupply {
		row.Metadata = &store.TokenCreationMetadata{Name: name, Symbol: symbol, Decimals: decimals, TotalSupply: supply.String(), Position: pos}
		return tokenDocument(row), nil
	}
	if hasName {
		doc["name"] = name
	}
	if hasSymbol {
		doc["symbol"] = symbol
	}
	if hasDecimals {
		doc["decimals"] = int(decimals)
		doc["decimalsQuality"] = "verified"
	}
	if hasSupply {
		doc["totalSupplyRaw"] = supply.String()
		doc["supplyQuality"] = "verified"
	}
	if len(values) > 0 {
		doc["sourcePosition"] = chainPosition(pos)
	}
	return doc, nil
}
