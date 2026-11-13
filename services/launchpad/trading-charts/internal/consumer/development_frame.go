package consumer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
)

// DevelopmentFrame preserves presence separately from zero-valued source indexes.
type DevelopmentFrame struct {
	EventName        string             `json:"eventName"`
	BlockNumber      *uint64            `json:"blockNumber"`
	TransactionIndex *uint64            `json:"transactionIndex"`
	LogIndex         *uint64            `json:"logIndex"`
	Position         *engine.ChainOrder `json:"position,omitempty"`
	BlockTimestamp   int64              `json:"blockTimestamp"`
	TxHash           string             `json:"txHash"`
	Args             map[string]any     `json:"args"`
}

func DecodeDevelopmentSwap(payload []byte) (engine.SwapEvent, error) {
	var data DevelopmentFrame
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		return engine.SwapEvent{}, fmt.Errorf("consumer: invalid swap envelope: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return engine.SwapEvent{}, errors.New("consumer: trailing swap envelope")
	}
	if data.EventName != "Swap" || data.BlockNumber == nil || *data.BlockNumber == 0 || data.LogIndex == nil || *data.LogIndex > math.MaxInt32 {
		return engine.SwapEvent{}, errors.New("consumer: source chain position required")
	}
	var position engine.ChainOrder
	if data.Position != nil {
		position = *data.Position
		if position.Height != *data.BlockNumber || position.LogIndex != *data.LogIndex || (data.TransactionIndex != nil && position.TransactionIndex != *data.TransactionIndex) {
			return engine.SwapEvent{}, errors.New("consumer: source chain position inconsistent")
		}
	} else {
		if data.TransactionIndex == nil {
			return engine.SwapEvent{}, errors.New("consumer: source chain position required")
		}
		position = engine.ChainOrder{Height: *data.BlockNumber, TransactionIndex: *data.TransactionIndex, LogIndex: *data.LogIndex}
	}
	if position.TransactionIndex > math.MaxUint32 {
		return engine.SwapEvent{}, errors.New("consumer: source transaction index out of range")
	}
	swap := engine.SwapEvent{
		PoolID: asString(data.Args["poolId"]), PoolAddress: asString(data.Args["poolAddress"]),
		Sender: asString(data.Args["sender"]), IsBuy: asBool(data.Args["isBuy"]),
		AmountIn: asString(data.Args["amountIn"]), AmountOut: asString(data.Args["amountOut"]),
		Fee: asString(data.Args["fee"]), Price: asString(data.Args["price"]),
		TxHash: data.TxHash, LogIndex: int(*data.LogIndex),
		Position: &position,
	}
	switch {
	case data.BlockTimestamp > 0:
		swap.BlockTimestamp = data.BlockTimestamp
	case asInt64(data.Args["blockTimestamp"]) > 0:
		swap.BlockTimestamp = asInt64(data.Args["blockTimestamp"])
	case asInt64(data.Args["timestamp"]) > 0:
		swap.BlockTimestamp = asInt64(data.Args["timestamp"])
	default:
		return engine.SwapEvent{}, errors.New("consumer: authenticated block timestamp unavailable")
	}
	return swap, nil
}
