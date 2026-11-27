package processor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func routedInventoryFills(t *testing.T, ctx context.Context, contract bool) ([]accounting.FillV1, receipt.Bundle) {
	t.Helper()
	first, bundle := nativeHolderProducerFill(t, ctx, false)
	raw, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	var second accounting.FillV1
	if err := json.Unmarshal(raw, &second); err != nil {
		t.Fatal(err)
	}
	first.Side, first.FeeDisposition = "sell", "retained_token_reserve"
	first.PaidInputRaw = first.TokenAmountRaw
	first.PricedInputRaw, first.AMMInputAfterFeeRaw = nil, nil
	first.InputDerivationQuality = "receipt_incomplete"
	second.Token.Address = strings.ToLower(common.BytesToAddress(crypto.Keccak256(common.HexToAddress(first.Token.Address).Bytes())).Hex())
	second.PoolAddress = strings.ToLower(common.BytesToAddress(crypto.Keccak256(common.HexToAddress(first.PoolAddress).Bytes())).Hex())
	if contract {
		first.ActualEmitter = strings.ToLower(nativeAccountingReceiptOptions(t, nativeProjectionReceiptInputs(t)).Manifest.Roles["EventEmitter"].Address)
		second.ActualEmitter = first.ActualEmitter
	}
	fills := []accounting.FillV1{first, second}
	sender := first.Attribution.TransactionSender
	holderOut, holderIn, relay := first.PoolAddress, second.PoolAddress, strings.ToLower(bundle.Transaction.To().Hex())
	if !contract {
		holderOut, holderIn, relay = strings.ToLower(decode.LaunchpadModuleAddress.Hex()), strings.ToLower(decode.LaunchpadModuleAddress.Hex()), second.PoolAddress
	}
	logs := make([]*types.Log, 8)
	for i := range logs {
		log := *bundle.Receipt.Logs[0]
		log.Index = uint(i)
		log.Topics = nil
		log.Data = []byte{}
		logs[i] = &log
	}
	for _, transfer := range []struct {
		index                   int
		asset, from, to, amount string
	}{
		{0, first.Token.Address, sender, first.PoolAddress, first.TokenAmountRaw},
		{1, first.Settlement.Address, holderOut, relay, first.USDLAmountRaw},
		{3, first.Settlement.Address, relay, holderIn, first.USDLAmountRaw},
		{4, second.Token.Address, second.PoolAddress, sender, second.TokenAmountRaw},
	} {
		log := logs[transfer.index]
		log.Address = common.HexToAddress(transfer.asset)
		log.Topics = []common.Hash{crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")), common.HexToHash(transfer.from), common.HexToHash(transfer.to)}
		amount, err := accounting.Uint(transfer.amount)
		if err != nil {
			t.Fatal(err)
		}
		log.Data = amount.FillBytes(make([]byte, 32))
	}
	for i, index := range []int{2, 7} {
		fills[i].Position.LogIndex = strconv.Itoa(index)
		fills[i].SourceFillID = accounting.SourceFillIdentity(fills[i].Position, fills[i].Provenance.DeploymentID)
		logs[index].Address = common.HexToAddress(fills[i].ActualEmitter)
	}
	bundle.Receipt.Logs = logs
	raw, err = json.Marshal(bundle.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	for i := range fills {
		fill := &fills[i]
		fill.Provenance.ReceiptSHA256 = hex.EncodeToString(digest[:])
		input, output := [2]string{"0", "3"}[i], [2]string{"1", "4"}[i]
		fill.Attribution = accounting.Attribution{TransactionSender: sender, RouterCaller: &sender, Trader: &sender, Payer: &sender, Recipient: &sender, Quality: "qualified_calldata", Evidence: []accounting.EvidenceRef{
			{Kind: "router-calldata", ArtifactSHA256: first.Attribution.Evidence[0].ArtifactSHA256, ArtifactID: fill.Position.TransactionHash},
			{Kind: "input-transfer", ArtifactSHA256: fill.Provenance.ReceiptSHA256, ArtifactID: fill.Position.TransactionHash, LogIndex: &input},
			{Kind: "output-transfer", ArtifactSHA256: fill.Provenance.ReceiptSHA256, ArtifactID: fill.Position.TransactionHash, LogIndex: &output},
		}}
	}
	return fills, bundle
}

func TestRoutedSwapAttributionInventorySameOwner(t *testing.T) {
	ctx := context.Background()
	h := newInventoryHarness(t, ctx)
	for _, contract := range []bool{false, true} {
		t.Run(strconv.FormatBool(contract), func(t *testing.T) {
			fills, bundle := routedInventoryFills(t, ctx, contract)
			tx, err := h.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			canonical := store.CanonicalTx{Tx: tx}
			height := bundle.Header.Number.Int64()
			batch := store.CanonicalBatch{ChainID: 125, StreamID: realStream, Owner: inventoryTestOwner, Fence: 1, PreviousHeight: height - 1, Blocks: []store.BlockCompleteness{{Height: height}}}
			for _, log := range bundle.Receipt.Logs {
				topics := make([]string, len(log.Topics))
				for i, topic := range log.Topics {
					topics[i] = strings.ToLower(topic.Hex())
				}
				rawTopics, err := json.Marshal(topics)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := tx.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8)`, fills[0].Position.TransactionHash, int64(log.Index), height, int64(log.TxIndex), fills[0].Position.BlockHash, strings.ToLower(log.Address.Hex()), rawTopics, log.Data); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := tx.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,$4,true,true)`, realStream, height, fills[0].Position.BlockHash, len(bundle.Receipt.Logs)); err != nil {
				t.Fatal(err)
			}
			for i, fill := range fills {
				if _, err := canonical.InsertAccountingFill(ctx, fill); err != nil {
					t.Fatal(err)
				}
				_, _, err := qualifiedInventoryEnrichment(ctx, canonical, batch, fill, pnlGeneration, pnlInventoryVersion)
				var refusal *inventoryRefusal
				if !errors.As(err, &refusal) || !refusal.pending || refusal.code != "baseline-unissued" {
					t.Fatalf("routed %s must reach baseline qualification: %v", fill.Side, err)
				}
				wallet := fill.Attribution.TransactionSender
				creation := fill.Position
				creation.LogIndex = strconv.Itoa(5 + i)
				if _, err := tx.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$2,$2,1,$4,$5,$6,0,$7,$8,'{}','{}')`, fill.Token.Address, fill.PoolAddress, wallet, height, fill.Position.BlockHash, fill.Position.TransactionHash, creation.LogIndex, bundle.Header.Time); err != nil {
					t.Fatal(err)
				}
				if _, err := tx.Exec(ctx, `INSERT INTO pnl.inventory_l16_baselines(baseline_digest,consumer_id,projection_generation,partition,wallet,token,creation_height,creation_block_hash,source_stream,source_owner,source_epoch,movement_digest,observed_balance_raw,attestation) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,1,$1,'0',$11)`, fill.Provenance.ReceiptSHA256[:63]+strconv.Itoa(i), pnlConsumerID, pnlGeneration, "125:"+wallet+":"+fill.Token.Address, wallet, fill.Token.Address, height, fill.Position.BlockHash, realStream, inventoryTestOwner, []byte{}); err != nil {
					t.Fatal(err)
				}
				index := [2]int{0, 4}[i]
				log := bundle.Receipt.Logs[index]
				position := fill.Position
				position.LogIndex = strconv.Itoa(index)
				movement := store.TransferFact{Token: fill.Token.Address, From: strings.ToLower(common.BytesToAddress(log.Topics[1].Bytes()).Hex()), To: strings.ToLower(common.BytesToAddress(log.Topics[2].Bytes()).Hex()), AmountRaw: new(big.Int).SetBytes(log.Data).String(), Position: position, Provenance: fill.Provenance}
				if err := canonical.InsertTokenTransfer(ctx, movement); err != nil {
					t.Fatal(err)
				}
			}
			for _, fill := range fills {
				enrichment, prior, err := qualifiedInventoryEnrichment(ctx, canonical, batch, fill, pnlGeneration, pnlInventoryVersion)
				if err != nil || enrichment.SourceFillID != fill.SourceFillID || enrichment.Attribution == nil || enrichment.Attribution.Payer == nil || *enrichment.Attribution.Payer != fill.Attribution.TransactionSender || len(prior) != 0 {
					t.Fatalf("routed %s inventory for %s: %+v, %v, %v", fill.Side, fill.Token.Address, enrichment, prior, err)
				}
				unavailable := fill
				unavailable.Attribution = accounting.Attribution{TransactionSender: fill.Attribution.TransactionSender, Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}
				_, _, err = qualifiedInventoryEnrichment(ctx, canonical, batch, unavailable, pnlGeneration, pnlInventoryVersion)
				var refusal *inventoryRefusal
				if !errors.As(err, &refusal) || refusal.pending || refusal.code != "not-same-owner" {
					t.Fatalf("unavailable routed %s inventory: %v", fill.Side, err)
				}
				raw, err := json.Marshal(unavailable)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := tx.Exec(ctx, `UPDATE indexer.indexer_accounting_fills SET payload=$2 WHERE source_fill_id=$1`, fill.SourceFillID, raw); err != nil {
					t.Fatal(err)
				}
				_, _, err = qualifiedInventoryEnrichment(ctx, canonical, batch, fill, pnlGeneration, pnlInventoryVersion)
				if !errors.As(err, &refusal) || refusal.pending || refusal.code != "unresolved-movement" {
					t.Fatalf("unavailable routed %s history: %v", fill.Side, err)
				}
			}
		})
	}
}
