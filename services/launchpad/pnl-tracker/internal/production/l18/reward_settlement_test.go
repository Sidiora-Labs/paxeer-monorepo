package l18

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var rewardTransferABI = parseABI(`[{"type":"function","name":"transfer","inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"type":"bool"}]}]`)

func TestRewardSettlementReceiptsRetryAndInvestigation(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(localChainFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	var chain struct{ RPC, Admin, Alice, Bob string }
	if err = json.Unmarshal(raw, &chain); err != nil {
		t.Fatal(err)
	}
	rpc, err := newFixtureRPC(chain.RPC)
	if err != nil {
		t.Fatal(err)
	}
	legacy := NewCreator(rpc, time.Now)
	creator := NewCreator(rpc, time.Now)
	creator.committed = legacy.checkpoint
	db := coordinatortest.Bootstrap(t, os.Getenv("PNL_TEST_POSTGRES_DSN"), "indexer_runtime", "pnl_runtime")
	pool, err := pgxpool.New(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	b, err := creator.checkpoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var code hexutil.Bytes
	if err = rpc.CallContext(ctx, &code, "eth_getCode", USDL, map[string]any{"blockHash": b.Hash, "requireCanonical": true}); err != nil {
		t.Fatal(err)
	}
	program := Program{ID: "campaign", Version: "v1", DeploymentID: "settlement-domain", Asset: USDL, ReferralRaw: "1000000", AwardCapRaw: "1000000", Treasury: strings.ToLower(chain.Admin), Custody: USDL, Adapter: "usdl-contract-custody", AdapterCodeHash: strings.ToLower(crypto.Keccak256Hash(code).Hex()), DailyCapRaw: "10000000", BeneficiaryCapRaw: "10000000"}
	authority, err := NewCanonicalAuthority(pool, "pnl", creator, []Program{program})
	if err != nil {
		t.Fatal(err)
	}
	guard, err := PostgresEpochGuard("recovery", coordinatortest.Epoch, coordinatortest.Region)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := NewRuntimeLedger(pool, "pnl", "reward-settlement", authority, guard)
	if err != nil {
		t.Fatal(err)
	}
	settlement, err := NewSettlementService(ledger, authority)
	if err != nil {
		t.Fatal(err)
	}
	if !ledger.MatchesAuthority(authority) || ledger.RequireEpochFence() != nil {
		t.Fatal("runtime authority not bound")
	}
	mustSend(t, rpc, chain.Admin, USDL, mintABI, "mint", common.HexToAddress(chain.Admin), big.NewInt(10000000))
	fundingTx := mustSend(t, rpc, chain.Admin, USDL, rewardTransferABI, "transfer", common.HexToAddress(USDL), big.NewInt(10000000))
	evidence, err := creator.tokenTransfer(ctx, fundingTx, program.Treasury, USDL)
	if err != nil || evidence.Raw != "10000000" {
		t.Fatal(evidence, err)
	}
	funding := Funding{Program: program.ID, Version: program.Version, Day: time.Unix(evidence.Timestamp, 0).UTC().Format("2006-01-02"), Asset: USDL, Treasury: program.Treasury, Adapter: program.Adapter, Proof: fundingTx, FundedRaw: evidence.Raw, DailyCapRaw: program.DailyCapRaw, BeneficiaryCapRaw: program.BeneficiaryCapRaw}
	for i := 0; i < 2; i++ {
		if err = ledger.Fund(ctx, funding); err != nil {
			t.Fatal(err)
		}
	}
	badFunding := funding
	badFunding.FundedRaw = "1"
	if err = authority.VerifyFunding(ctx, badFunding); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	badFunding = funding
	badFunding.Adapter = "different-adapter"
	if err = authority.VerifyFunding(ctx, badFunding); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	// Stored liabilities are the settlement service's input; receipt confirmation
	// must preserve their exact beneficiary and amount.
	for i, id := range []string{"settlement-paid", "settlement-retry", "settlement-pending"} {
		if _, err = pool.Exec(ctx, `INSERT INTO pnl.l18_awards(identity,program,version,day,beneficiary,asset,kind,eligible,payable,status,provenance) VALUES($1,'campaign','v1',$2,$3,$4,'cashback',1000000,1000000,'funded',$5)`, id, funding.Day, strings.ToLower(chain.Alice), USDL, fmt.Sprintf("125:%s:%d", fundingTx, i)); err != nil {
			t.Fatal(err)
		}
	}
	actor := Actor{Wallet: chain.Admin, Roles: map[string]bool{"reward-settlement": true, "reward-retry": true}}
	// The local chain owns this custody address and executes the token contract's
	// transfer implementation with the custody's actual token balance.
	var accepted bool
	if err = rpc.CallContext(ctx, &accepted, "anvil_impersonateAccount", USDL); err != nil {
		t.Fatal(err)
	}
	if err = rpc.CallContext(ctx, &accepted, "anvil_setBalance", USDL, "0x100000000000000000"); err != nil {
		t.Fatal(err)
	}
	paid := mustSend(t, rpc, USDL, USDL, rewardTransferABI, "transfer", common.HexToAddress(chain.Alice), big.NewInt(1000000))
	assertSettlementDatabaseCancellation(t, authority, program, actor, strings.ToLower(chain.Alice), funding.Day, paid)
	assertSettlementEvidenceBindings(t, settlement, actor, program, funding.Day, strings.ToLower(chain.Alice), chain.RPC, func(t *testing.T) string {
		return mustSend(t, rpc, USDL, USDL, rewardTransferABI, "transfer", common.HexToAddress(chain.Alice), big.NewInt(1))
	})
	request := uuid.NewString()
	for i := 0; i < 2; i++ {
		if err = settlement.RegisterIntent(ctx, actor, request, "settlement-paid", paid, "beneficiary payout"); err != nil {
			t.Fatal(err)
		}
	}
	if err = settlement.RegisterIntent(ctx, actor, uuid.NewString(), "settlement-paid", paid, "beneficiary payout"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		state, err := settlement.Reconcile(ctx, "settlement-paid")
		if err != nil || state != statusIncludedSuccess {
			t.Fatal(state, err)
		}
	}
	if err = settlement.RegisterIntent(ctx, actor, uuid.NewString(), "settlement-paid", paid, "beneficiary payout"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if got := query(t, ledger, `SELECT status FROM l18_awards WHERE identity='settlement-paid'`); got != "paid" {
		t.Fatal(got)
	}
	if got := query(t, ledger, `SELECT raw::text FROM l18_entries WHERE identity='settlement-paid'`); got != "1000000" {
		t.Fatal(got)
	}
	failed := send(t, rpc, USDL, USDL, rewardTransferABI, "transfer", common.HexToAddress(chain.Alice), new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil))
	if receiptStatus(t, rpc, failed) != 0 {
		t.Fatal("insolvent transfer included successfully")
	}
	if err = settlement.RegisterIntent(ctx, actor, uuid.NewString(), "settlement-retry", failed, "initial payout"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		state, err := settlement.Reconcile(ctx, "settlement-retry")
		if err != nil || state != statusIncludedReverted {
			t.Fatal(state, err)
		}
	}
	if err = settlement.markAttempt(ctx, "settlement-retry", 1, failed, statusIncludedReverted); err != nil {
		t.Fatal(err)
	}
	if err = settlement.markAttempt(ctx, "settlement-retry", 1, failed, statusPendingInvestigation); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	retry := mustSend(t, rpc, USDL, USDL, rewardTransferABI, "transfer", common.HexToAddress(chain.Alice), big.NewInt(1000000))
	noRetry := actor
	noRetry.Roles = map[string]bool{"reward-settlement": true}
	if err = settlement.RegisterIntent(ctx, noRetry, uuid.NewString(), "settlement-retry", retry, "retry payout"); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err = settlement.RegisterIntent(ctx, actor, uuid.NewString(), "settlement-retry", retry, "retry payout"); err != nil {
		t.Fatal(err)
	}
	if state, err := settlement.Reconcile(ctx, "settlement-retry"); err != nil || state != statusIncludedSuccess {
		t.Fatal(state, err)
	}
	pending := "0x" + strings.Repeat("e", 64)
	if err = settlement.RegisterIntent(ctx, actor, uuid.NewString(), "settlement-pending", pending, "pending payout"); err != nil {
		t.Fatal(err)
	}
	if state, err := settlement.Reconcile(ctx, "settlement-pending"); !errors.Is(err, ErrUnavailable) || state != statusPendingInvestigation {
		t.Fatal(state, err)
	}
	if err = settlement.markAttempt(ctx, "settlement-pending", 1, pending, statusPendingInvestigation); err != nil {
		t.Fatal(err)
	}
	if err = settlement.markAttempt(ctx, "settlement-pending", 2, pending, statusIncludedReverted); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err = settlement.markAttempt(ctx, "settlement-pending", 1, pending, "unknown"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err = settlement.RegisterIntent(ctx, Actor{Wallet: chain.Admin}, uuid.NewString(), "settlement-pending", pending, "pending payout"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if err = settlement.RegisterIntent(ctx, actor, "invalid", "settlement-pending", pending, "pending payout"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = settlement.Reconcile(ctx, "invalid id"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err = settlement.markAttempt(ctx, "settlement-paid", 1, paid, statusIncludedReverted); err != nil {
		t.Fatal(err)
	}
	if got := query(t, ledger, `SELECT state FROM l18_payout_attempts WHERE award_id='settlement-paid'`); got != statusIncludedSuccess {
		t.Fatal(got)
	}
}
