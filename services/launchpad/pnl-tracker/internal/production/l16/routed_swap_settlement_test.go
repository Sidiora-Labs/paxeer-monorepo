package l16

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/jackc/pgx/v5"
)

func routedSwapReceipt(t *testing.T, fixture nativeSettlementFixture, native bool) ([]accounting.FillV1, *types.Receipt) {
	t.Helper()
	buy, receipt := fixture.nativeBuy(t)
	sell := buy
	sell.Side, sell.PaidInputRaw, sell.FeeDisposition = sideSell, sell.TokenAmountRaw, "retained_token_reserve"
	sell.Token.Address = "0x" + strings.Repeat("a", 40)
	sell.PoolAddress = "0x" + strings.Repeat("b", 40)
	if !native {
		sell.ActualEmitter, buy.ActualEmitter = sell.PoolAddress, buy.PoolAddress
	}
	sender := buy.Attribution.TransactionSender
	intermediate := buy.PoolAddress
	if !native {
		intermediate = "0x" + strings.Repeat("c", 40)
	}
	transfer := func(asset, from, to, amount string, index uint) *types.Log {
		log := *receipt.Logs[fixture.tokenIndex]
		log.Address = common.HexToAddress(asset)
		log.Topics = []common.Hash{transferTopic, common.HexToHash(from), common.HexToHash(to)}
		value, err := Uint256(amount)
		if err != nil {
			t.Fatal(err)
		}
		log.Data, log.Index = value.FillBytes(make([]byte, 32)), index
		return &log
	}
	event := func(fill accounting.FillV1, index uint) *types.Log {
		log := *receipt.Logs[len(receipt.Logs)-1]
		log.Address, log.Index = common.HexToAddress(fill.ActualEmitter), index
		return &log
	}
	sellHolder, buyHolder := settlementAssetHolder(sell), settlementAssetHolder(buy)
	receipt.Logs = []*types.Log{
		transfer(sell.Token.Address, sender, sell.PoolAddress, sell.TokenAmountRaw, 0),
		transfer(sell.Settlement.Address, sellHolder, intermediate, sell.USDLAmountRaw, 1),
		event(sell, 2),
		transfer(buy.Settlement.Address, intermediate, buyHolder, buy.USDLAmountRaw, 3),
		transfer(buy.Token.Address, buy.PoolAddress, sender, buy.TokenAmountRaw, 4),
		event(buy, 7),
	}
	sell.Position.LogIndex, buy.Position.LogIndex = "2", "7"
	if !native {
		receipt.Logs = []*types.Log{receipt.Logs[0], receipt.Logs[1], receipt.Logs[3], receipt.Logs[4],
			transfer(buy.Settlement.Address, buy.PoolAddress, "0x"+strings.Repeat("d", 40), "1", 4), event(buy, 5), event(buy, 6)}
		for i, log := range receipt.Logs {
			log.Index = uint(i)
		}
		sell.Position.LogIndex, buy.Position.LogIndex = "2", "6"
	}
	for _, fill := range []*accounting.FillV1{&sell, &buy} {
		fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
		fill.Attribution = accounting.Attribution{TransactionSender: sender, Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}
	}
	return []accounting.FillV1{sell, buy}, receipt
}

func bindRoutedSwap(t *testing.T, ctx context.Context, fixture nativeSettlementFixture, tx pgx.Tx, fills []accounting.FillV1, receipt *types.Receipt) (*BaselineAuthority, []accounting.FillV1) {
	t.Helper()
	a, _ := fixture.bind(t, ctx, tx, fixture.fill, receipt)
	if _, err := tx.Exec(ctx, `DELETE FROM indexer.indexer_accounting_fills WHERE transaction_hash=$1`, fixture.fill.Position.TransactionHash); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	for _, log := range receipt.Logs {
		topics, err := json.Marshal(log.Topics)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`, strings.ToLower(log.TxHash.Hex()), int64(log.Index), int64(log.BlockNumber), int64(log.TxIndex), strings.ToLower(log.BlockHash.Hex()), strings.ToLower(log.Address.Hex()), topics, log.Data); err != nil {
			t.Fatal(err)
		}
	}
	bound := append([]accounting.FillV1(nil), fills...)
	for i := range bound {
		fill := &bound[i]
		fill.Provenance.ReceiptSHA256 = delivery.Hash(raw)
		payload, err := json.Marshal(fill)
		if err != nil {
			t.Fatal(err)
		}
		digest, err := accounting.FillDigest(*fill)
		if err != nil {
			t.Fatal(err)
		}
		index, err := strconv.ParseInt(fill.Position.LogIndex, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO indexer.indexer_accounting_fills(source_fill_id,source_fact_digest,chain_id,transaction_hash,log_index,pool_address,payload) VALUES($1,$2,125,$3,$4,$5,$6)`, fill.SourceFillID, string(digest), fill.Position.TransactionHash, index, fill.PoolAddress, payload); err != nil {
			t.Fatal(err)
		}
	}
	return a, bound
}

func TestRoutedSwapSettlement(t *testing.T) {
	parallelDatabaseTest(t)
	fixture := newNativeSettlementFixture(t, context.Background())
	t.Run("OwnTrades", func(t *testing.T) { routedSwapSettlementOwnTrades(t, fixture) })
	t.Run("RejectsUnallocatedRoutes", func(t *testing.T) { routedSwapSettlementRejectsUnallocatedRoutes(t, fixture) })
	t.Run("PlainTrades", func(t *testing.T) { routedSwapSettlementPlainTrades(t, fixture) })
}

func routedSwapSettlementOwnTrades(t *testing.T, fixture nativeSettlementFixture) {
	t.Helper()
	ctx := context.Background()
	for _, native := range []bool{true, false} {
		t.Run(map[bool]string{true: "native", false: "contract"}[native], func(t *testing.T) {
			fills, receipt := routedSwapReceipt(t, fixture, native)
			tx, err := fixture.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer inventoryRollback(tx)
			a, fills := bindRoutedSwap(t, ctx, fixture, tx, fills, receipt)
			for _, lookup := range []func(context.Context, pgx.Tx, accounting.FillV1) (receiptSettlement, error){a.receiptSettlement, a.liveReceiptSettlement} {
				for _, fill := range fills {
					settlement, err := lookup(ctx, tx, fill)
					if err != nil {
						t.Fatal(err)
					}
					sender := fill.Attribution.TransactionSender
					var token movement
					for _, log := range receipt.Logs {
						if strings.ToLower(log.Address.Hex()) == fill.Token.Address {
							token, err = decodeTransfer(*log)
							if err != nil {
								t.Fatal(err)
							}
						}
					}
					gas := new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasUsed), receipt.EffectiveGasPrice).String()
					if settlement.payer != sender || settlement.recipient != sender || settlement.tokenMovement != token || settlement.fill.SourceFillID != fill.SourceFillID || settlement.digest != fill.Provenance.ReceiptSHA256 || settlement.gas != gas {
						t.Fatalf("settlement = %+v, want own %s with token movement %+v", settlement, fill.Side, token)
					}
					seconds, err := strconv.ParseInt(fill.BlockTimestampSeconds, 10, 64)
					if err != nil {
						t.Fatal(err)
					}
					record, ok, err := normalizeMovement(sender, fill.Token.Address, token, &settlement, time.Unix(seconds, 0), settlement.digest)
					if err != nil || !ok || record.Leg.Kind != fill.Side || record.Leg.Wallet != sender || record.Leg.Token != fill.Token.Address || record.Leg.Quantity != fill.TokenAmountRaw || record.Leg.ActualUSDL == nil || *record.Leg.ActualUSDL != fill.USDLAmountRaw {
						t.Fatalf("normalized trade = %+v, ok = %v, error = %v", record, ok, err)
					}
				}
			}
		})
	}
}

func routedSwapSettlementRejectsUnallocatedRoutes(t *testing.T, fixture nativeSettlementFixture) {
	t.Helper()
	ctx := context.Background()
	for _, native := range []bool{true, false} {
		t.Run(map[bool]string{true: "native", false: "contract"}[native], func(t *testing.T) {
			for _, change := range []string{"settlement amount", "settlement sender", "token recipient", "additional settlement", "third fill"} {
				t.Run(change, func(t *testing.T) {
					fills, receipt := routedSwapReceipt(t, fixture, native)
					var buyQuote, buyToken *types.Log
					for _, log := range receipt.Logs {
						if log.Topics[0] != transferTopic {
							continue
						}
						if log.Address == common.HexToAddress(fills[1].Token.Address) {
							buyToken = log
						}
						if log.Address == common.HexToAddress(fills[1].Settlement.Address) && log.Topics[2] == common.HexToHash(settlementAssetHolder(fills[1])) {
							buyQuote = log
						}
					}
					if buyQuote == nil || buyToken == nil {
						t.Fatal("routed buy transfers absent")
					}
					switch change {
					case "settlement amount":
						amount := new(big.Int).Add(new(big.Int).SetBytes(buyQuote.Data), big.NewInt(1))
						buyQuote.Data = amount.FillBytes(make([]byte, 32))
						fills[1].USDLAmountRaw, fills[1].PaidInputRaw = amount.String(), amount.String()
					case "settlement sender":
						buyQuote.Topics[1] = common.HexToHash("0x" + strings.Repeat("e", 40))
					case "token recipient":
						buyToken.Topics[2] = common.HexToHash("0x" + strings.Repeat("e", 40))
					case "additional settlement":
						extra := *buyQuote
						extra.Topics = []common.Hash{transferTopic, common.HexToHash("0x" + strings.Repeat("e", 40)), common.HexToHash(fills[0].Attribution.TransactionSender)}
						extra.Data, extra.Index = big.NewInt(1).FillBytes(make([]byte, 32)), 8
						receipt.Logs = append(receipt.Logs, &extra)
					case "third fill":
						extra := fills[1]
						extra.Position.LogIndex = "8"
						extra.SourceFillID = accounting.SourceFillIdentity(extra.Position, extra.Provenance.DeploymentID)
						fills = append(fills, extra)
						log := *receipt.Logs[len(receipt.Logs)-1]
						log.Index = 8
						receipt.Logs = append(receipt.Logs, &log)
					}
					tx, err := fixture.pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer inventoryRollback(tx)
					a, fills := bindRoutedSwap(t, ctx, fixture, tx, fills, receipt)
					lookups := []func(context.Context, pgx.Tx, accounting.FillV1) (receiptSettlement, error){a.receiptSettlement}
					if change != "third fill" {
						lookups = append(lookups, a.liveReceiptSettlement)
					}
					for _, lookup := range lookups {
						for _, fill := range fills[:2] {
							if _, err := lookup(ctx, tx, fill); !errors.Is(err, ErrCostAllocationUnavailable) {
								t.Fatalf("settlement error = %v, want %v", err, ErrCostAllocationUnavailable)
							}
						}
					}
				})
			}
		})
	}
}

func routedSwapSettlementPlainTrades(t *testing.T, fixture nativeSettlementFixture) {
	t.Helper()
	ctx := context.Background()
	for _, native := range []bool{true, false} {
		t.Run(map[bool]string{true: "native", false: "contract"}[native], func(t *testing.T) {
			fills, receipt := routedSwapReceipt(t, fixture, native)
			wallet := fills[0].Attribution.TransactionSender
			var sellQuote, buyQuote *types.Log
			for _, log := range receipt.Logs {
				if log.Address != common.HexToAddress(fills[0].Settlement.Address) || log.Topics[0] != transferTopic {
					continue
				}
				if log.Index == 1 {
					log.Topics[2], sellQuote = common.HexToHash(wallet), log
				} else if log.Topics[2] == common.HexToHash(settlementAssetHolder(fills[1])) {
					log.Topics[1], buyQuote = common.HexToHash(wallet), log
				}
			}
			if sellQuote == nil || buyQuote == nil {
				t.Fatal("plain settlement transfers absent")
			}
			sellToken, err := decodeTransfer(*receipt.Logs[0])
			if err != nil {
				t.Fatal(err)
			}
			var buyToken movement
			for _, log := range receipt.Logs {
				if log.Address == common.HexToAddress(fills[1].Token.Address) {
					buyToken, err = decodeTransfer(*log)
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			sellSettlement, err := decodeTransfer(*sellQuote)
			if err != nil {
				t.Fatal(err)
			}
			buySettlement, err := decodeTransfer(*buyQuote)
			if err != nil {
				t.Fatal(err)
			}
			if got := routedSettlementWallet(receiptSettlementCandidate{fill: fills[0], token: sellToken, quote: sellSettlement}, receiptSettlementCandidate{fill: fills[1], token: buyToken, quote: buySettlement}); got != "" {
				t.Fatal("plain trades classified as routed", got)
			}
			tx, err := fixture.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer inventoryRollback(tx)
			a, fills := bindRoutedSwap(t, ctx, fixture, tx, fills, receipt)
			for i, fill := range fills {
				if _, err = a.receiptSettlement(ctx, tx, fill); !errors.Is(err, ErrCostAllocationUnavailable) {
					t.Fatalf("two plain fills error = %v, want %v", err, ErrCostAllocationUnavailable)
				}
				settlement, err := a.liveReceiptSettlement(ctx, tx, fill)
				if err != nil || settlement.payer != wallet || settlement.recipient != wallet || settlement.tokenMovement != []movement{sellToken, buyToken}[i] {
					t.Fatalf("plain settlement = %+v, error = %v", settlement, err)
				}
			}
		})
	}
}
