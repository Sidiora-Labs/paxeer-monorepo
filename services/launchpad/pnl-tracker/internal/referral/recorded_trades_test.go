package referral

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
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l18"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/google/uuid"
)

func TestRecordedTradesCreateUnfundedReferralAndCashback(t *testing.T) {
	ctx := context.Background()
	var fill accounting.FillV1
	var sourceLog types.Log
	s, _, alice, _ := productionReferralFixture(t, func(client *rpc.Client, chain internaltest.ChainFixture) {
		transfer := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
		var logs []types.Log
		if err := client.CallContext(ctx, &logs, "eth_getLogs", map[string]any{"fromBlock": "0x0", "toBlock": "latest", "address": chain.Token, "topics": []any{transfer.Hex(), common.BytesToHash(common.HexToAddress(chain.Pool).Bytes()).Hex()}}); err != nil || len(logs) != 1 {
			t.Fatal(logs, err)
		}
		movement := logs[0]
		sourceLog = movement
		var receipt, header json.RawMessage
		if err := client.CallContext(ctx, &receipt, "eth_getTransactionReceipt", movement.TxHash); err != nil {
			t.Fatal(err)
		}
		if err := client.CallContext(ctx, &header, "eth_getBlockByNumber", hexutil.EncodeUint64(movement.BlockNumber), false); err != nil {
			t.Fatal(err)
		}
		var block struct{ Timestamp hexutil.Uint64 }
		if err := json.Unmarshal(header, &block); err != nil {
			t.Fatal(err)
		}
		var transaction struct{ From string }
		if err := client.CallContext(ctx, &transaction, "eth_getTransactionByHash", movement.TxHash); err != nil {
			t.Fatal(err)
		}
		var deposits []types.Log
		if err := client.CallContext(ctx, &deposits, "eth_getLogs", map[string]any{"fromBlock": "0x0", "toBlock": hexutil.EncodeUint64(movement.BlockNumber), "address": l18.USDL, "topics": []any{transfer.Hex(), common.Hash{}.Hex(), common.BytesToHash(common.HexToAddress(chain.Pool).Bytes()).Hex()}}); err != nil || len(deposits) != 1 {
			t.Fatal(deposits, err)
		}
		paid := new(big.Int).SetBytes(deposits[0].Data)
		fee := new(big.Int).Div(new(big.Int).Set(paid), big.NewInt(100))
		net := new(big.Int).Sub(new(big.Int).Set(paid), fee).String()
		priced := paid.String()
		hash := func(raw []byte) string { digest := sha256.Sum256(raw); return hex.EncodeToString(digest[:]) }
		digest := accounting.Digest(hash(receipt))
		headerDigest := accounting.Digest(hash(header))
		bps := uint16(100)
		sender := strings.ToLower(transaction.From)
		recipient := strings.ToLower(common.BytesToAddress(movement.Topics[2].Bytes()).Hex())
		index := strconv.FormatUint(uint64(movement.Index), 10)
		evidence := []accounting.EvidenceRef{{Kind: "output-transfer", ArtifactSHA256: hash(receipt), ArtifactID: strings.ToLower(movement.TxHash.Hex()), LogIndex: &index}}
		fill = accounting.FillV1{Version: accounting.FillVersion, Position: accounting.SourcePosition{ChainID: "125", Height: strconv.FormatUint(movement.BlockNumber, 10), TransactionIndex: strconv.FormatUint(uint64(movement.TxIndex), 10), LogIndex: index, TransactionHash: strings.ToLower(movement.TxHash.Hex()), BlockHash: strings.ToLower(movement.BlockHash.Hex())}, ActualEmitter: strings.ToLower(chain.Pool), PoolAddress: strings.ToLower(chain.Pool), Token: accounting.Asset{Address: strings.ToLower(chain.Token), Decimals: 18}, Settlement: accounting.Asset{Address: l18.USDL, Decimals: 6}, Side: "buy", TokenAmountRaw: new(big.Int).SetBytes(movement.Data).String(), USDLAmountRaw: paid.String(), PaidInputRaw: paid.String(), FeeRaw: fee.String(), PricedInputRaw: &priced, AMMInputAfterFeeRaw: &net, InputDerivationQuality: "exact", FeeDisposition: "distributed_usdl", Attribution: accounting.Attribution{TransactionSender: sender, Trader: &sender, Payer: &sender, Recipient: &recipient, Quality: "exact_receipt", Evidence: evidence}, Execution: accounting.ExecutionPolicy{Quality: "exact", FeeBPS: &bps, ConfigDigest: &headerDigest, PreTransactionStateDigest: &headerDigest, ReplayEvidenceDigest: &digest, Evidence: evidence}, Provenance: accounting.Provenance{DeploymentID: "fixture", ABIRevision: "pool-v1", EmitterCodeEpoch: "pool-v1", PoolCodeEpoch: "pool-v1", ReceiptSHA256: hash(receipt), HeaderSHA256: hash(header)}, BlockTimestampSeconds: strconv.FormatUint(uint64(block.Timestamp), 10)}
		fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
		if err := accounting.ValidateFillV1(fill); err != nil {
			t.Fatal(err)
		}
	})
	raw, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	topics, err := json.Marshal(sourceLog.Topics)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8)`, fill.Position.TransactionHash, fill.Position.LogIndex, fill.Position.Height, fill.Position.TransactionIndex, fill.Position.BlockHash, strings.ToLower(sourceLog.Address.Hex()), topics, []byte(sourceLog.Data)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO indexer.indexer_accounting_fills(source_fill_id,source_fact_digest,chain_id,transaction_hash,log_index,pool_address,payload) VALUES($1,$2,125,$3,$4,$5,$6)`, fill.SourceFillID, string(digest), fill.Position.TransactionHash, fill.Position.LogIndex, fill.PoolAddress, raw); err != nil {
		t.Fatal(err)
	}
	trader := *fill.Attribution.Trader
	seconds, err := strconv.ParseInt(fill.BlockTimestampSeconds, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	attribution, err := l18.NewAttribution(s.ledger, []byte(strings.Repeat("k", 32)), func() time.Time { return time.Unix(seconds-60, 0) }, s.campaign, s.version)
	if err != nil {
		t.Fatal(err)
	}
	s.attribution = attribution
	token, err := attribution.IssueClick(alice, "canonical-click")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := attribution.Bind(ctx, uuid.NewString(), trader, true, []string{token})
	if err != nil || binding.Referrer != alice {
		t.Fatal(binding, err)
	}
	provenance := "125:" + fill.Position.TransactionHash + ":" + fill.Position.LogIndex
	for _, call := range []func() (l18.Liability, error){func() (l18.Liability, error) { return s.Convert(ctx, l18.Actor{Wallet: alice}, provenance) }, func() (l18.Liability, error) { return s.Cashback(ctx, l18.Actor{Wallet: alice}, provenance) }} {
		if _, err = call(); !errors.Is(err, l18.ErrUnauthorized) {
			t.Fatal(err)
		}
	}
	if err = s.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	liability, err := s.Convert(ctx, l18.Actor{Wallet: trader}, provenance)
	if err != nil || liability.Kind != "referral" || liability.Beneficiary != alice || liability.Referred != trader || liability.EligibleRaw != "1000" || liability.PayableRaw != "0" || liability.Status != "unfunded" {
		t.Fatal(liability, err)
	}
	if err = s.CashbackRunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	cashback, err := s.Cashback(ctx, l18.Actor{Wallet: trader}, provenance)
	p, programErr := s.authority.Program(s.campaign, s.version)
	if programErr != nil {
		t.Fatal(programErr)
	}
	expected, amountErr := l18.Cashback(*fill.AMMInputAfterFeeRaw, p.CashbackBPS, p.AwardCapRaw)
	if err != nil || amountErr != nil || cashback.Kind != "cashback" || cashback.Beneficiary != trader || cashback.EligibleRaw != expected || cashback.PayableRaw != "0" || cashback.Status != "unfunded" {
		t.Fatal(cashback, err, amountErr)
	}
	if err = s.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.CashbackRunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var count, payments int
	if err = s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM pnl.l18_awards),(SELECT count(*) FROM pnl.l18_entries)`).Scan(&count, &payments); err != nil || count != 2 || payments != 0 {
		t.Fatal(count, payments, err)
	}
}
