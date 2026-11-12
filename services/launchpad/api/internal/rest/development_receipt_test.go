package rest

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

func TestDevelopmentReceiptChecksIndexedSwap(t *testing.T) {
	pool := "0x" + strings.Repeat("a1", 20)
	token := "0x" + strings.Repeat("a2", 20)
	sender := "0x" + strings.Repeat("b1", 20)
	transaction := "0x" + strings.Repeat("c1", 32)
	blockHash := "0x" + strings.Repeat("d1", 32)
	poolID := "0x" + strings.Repeat("e1", 32)
	word := func(value int64) []byte { return new(big.Int).SetInt64(value).FillBytes(make([]byte, 32)) }
	data := []byte{}
	for _, value := range []int64{1, 1000000, 400, 100, 12, 1760000000, 44} {
		data = append(data, word(value)...)
	}
	swap := developmentSwap{id: "swap-1", poolID: poolID, pool: pool, token: token, sender: sender, isBuy: true, amountIn: "1000000", amountOut: "400", fee: "100", price: "12", optical: zeroAddressHex, block: 44, timestamp: 1760000000, txHash: transaction, logIndex: 2}
	receipt := &streamReceipt{Status: "0x1", BlockHash: blockHash, BlockNumber: "0x2c", TransactionHash: transaction, TransactionIndex: "0x1"}
	receipt.Logs = append(receipt.Logs, struct {
		Address  string   `json:"address"`
		Topics   []string `json:"topics"`
		Data     string   `json:"data"`
		LogIndex string   `json:"logIndex"`
		Removed  bool     `json:"removed"`
	}{Address: pool, Topics: []string{swapEventTopic, poolID, "0x" + strings.Repeat("0", 24) + sender[2:]}, Data: "0x" + hex.EncodeToString(data), LogIndex: "0x2"})
	projected, reason := swapPosition(swap, receipt)
	if reason != "" || projected.position.Height != "44" || projected.position.TransactionIndex != "1" || projected.emitter != pool {
		t.Fatalf("position = %+v, reason = %q", projected, reason)
	}
	projected.epoch = strings.Repeat("a", 64)
	projected.tokenDecimals = 18
	trade, price, err := developmentTradeDocuments(projected)
	if err != nil || trade["side"] != "buy" || trade["ammInputAfterFeeRaw"] != "999900" || price == nil || price["numerator"] != "2500000000000000" {
		t.Fatalf("trade = %v, price = %v, err = %v", trade, price, err)
	}
	checks := []struct {
		name   string
		change func(*developmentSwap, *streamReceipt)
		want   string
	}{
		{"unindexed pool", func(s *developmentSwap, _ *streamReceipt) { s.pool = "" }, "pool not indexed"},
		{"receipt height", func(_ *developmentSwap, r *streamReceipt) { r.BlockNumber = "0x2d" }, "receipt does not match the indexed swap"},
		{"removed log", func(_ *developmentSwap, r *streamReceipt) { r.Logs[0].Removed = true }, "log is not the indexed Swap"},
		{"malformed data", func(_ *developmentSwap, r *streamReceipt) { r.Logs[0].Data = "0x01" }, "Swap data malformed"},
		{"amount difference", func(s *developmentSwap, _ *streamReceipt) { s.amountOut = "401" }, "Swap amounts differ from the indexed row"},
		{"price difference", func(s *developmentSwap, _ *streamReceipt) { s.price = "13" }, "Swap price or time differs from committed candle source"},
		{"missing log", func(_ *developmentSwap, r *streamReceipt) { r.Logs[0].LogIndex = "0x3" }, "Swap log absent from receipt"},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			row := swap
			copyReceipt := *receipt
			copyReceipt.Logs = append(copyReceipt.Logs[:0:0], receipt.Logs...)
			check.change(&row, &copyReceipt)
			_, got := swapPosition(row, &copyReceipt)
			if got != check.want {
				t.Fatalf("reason = %q, want %q", got, check.want)
			}
		})
	}
	if hexutil.EncodeUint64(44) != receipt.BlockNumber {
		t.Fatal("receipt block does not describe the indexed height")
	}
}

func TestDevelopmentTradeDocumentAmounts(t *testing.T) {
	base := projectedSwap{swap: developmentSwap{
		pool: "0x" + strings.Repeat("a1", 20), token: "0x" + strings.Repeat("a2", 20), sender: "0x" + strings.Repeat("b1", 20),
		amountIn: "200", amountOut: "100", fee: "5", optical: zeroAddressHex, timestamp: 1760000000,
	}, position: marketDocumentPosition(), emitter: "0x" + strings.Repeat("a1", 20), epoch: strings.Repeat("a", 64), tokenDecimals: 18}
	for _, check := range []struct {
		name    string
		change  func(*projectedSwap)
		side    string
		quality string
		price   bool
	}{
		{"sell", func(*projectedSwap) {}, "sell", "exact", true},
		{"buy", func(p *projectedSwap) { p.swap.isBuy = true }, "buy", "exact", true},
		{"optical", func(p *projectedSwap) { p.swap.optical = p.swap.pool }, "sell", "receipt_incomplete", true},
		{"zero token output", func(p *projectedSwap) { p.swap.isBuy = true; p.swap.amountOut = "0" }, "buy", "exact", false},
	} {
		t.Run(check.name, func(t *testing.T) {
			row := base
			check.change(&row)
			trade, price, err := developmentTradeDocuments(row)
			if err != nil || trade["side"] != check.side || trade["inputDerivationQuality"] != check.quality || (price != nil) != check.price {
				t.Fatalf("trade = %v, price = %v, err = %v", trade, price, err)
			}
		})
	}
	for _, check := range []struct {
		name   string
		change func(*projectedSwap)
	}{
		{"amount", func(p *projectedSwap) { p.swap.amountIn = "-1" }},
		{"sender", func(p *projectedSwap) { p.swap.sender = "invalid" }},
		{"fee", func(p *projectedSwap) { p.swap.fee = "201" }},
	} {
		t.Run(check.name, func(t *testing.T) {
			row := base
			check.change(&row)
			if trade, price, err := developmentTradeDocuments(row); err == nil || trade != nil || price != nil {
				t.Fatalf("invalid trade = %v, price = %v, err = %v", trade, price, err)
			}
		})
	}
}
