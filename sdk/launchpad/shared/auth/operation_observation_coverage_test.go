package auth

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
)

func TestOperationTransactionsReportDurableReplayOutcomes(t *testing.T) {
	pool, _ := auditCoverageDatabase(t)
	ctx := context.Background()
	client, err := ethclient.DialContext(ctx, os.Getenv("L34_ANVIL_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	req, _ := signedOperation(t)
	observations := []string{}
	a := OperationAuthorizer{Ledger: Ledger{Pool: pool, Environment: "test", AuditKey: bytes.Repeat([]byte("a"), 32)}, Policies: DefaultPolicies(), Verifier: WalletOperationVerifier{Backend: NewReportedHashBackend(client.Client())}, Observe: func(category string) { observations = append(observations, category) }}
	mutation := func(ctx context.Context, tx pgx.Tx, body []byte) ([]byte, error) {
		var size int
		if err := tx.QueryRow(ctx, "SELECT octet_length($1::bytea)", body).Scan(&size); err != nil {
			return nil, err
		}
		if size != len(body) {
			return nil, ErrScopeDenied
		}
		return []byte(`{"updated":true}`), nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	response, duplicate, err := a.ExecuteTx(ctx, tx, req, mutation)
	if err != nil || duplicate || !bytes.Equal(response, []byte(`{"updated":true}`)) || len(observations) != 0 {
		t.Fatalf("new operation: %s %t %v %v", response, duplicate, observations, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"duplicate", "nonce binding", "response integrity", "database", "signature", "policy", "verifier"} {
		t.Run(mode, func(t *testing.T) {
			candidate, authorizer := req, a
			work, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer work.Rollback(ctx)
			want := ""
			observations = nil
			switch mode {
			case "duplicate":
				want = "duplicate_accepted"
			case "nonce binding":
				candidate.Envelope.ExpiresAt--
				resignOperation(t, &candidate)
				want = "replay_mismatch"
			case "response integrity":
				_, err = work.Exec(ctx, "UPDATE operation_ledger SET response='changed'")
				want = "replay_integrity"
			case "database":
				_, err = work.Exec(ctx, "ALTER TABLE operation_ledger RENAME COLUMN response_digest TO unbound_digest")
				want = "operation_dependency_unavailable"
			case "signature":
				candidate.Signature = "invalid"
				want = "operation_rejected"
			case "policy":
				candidate.PolicyVersion = "different"
			case "verifier":
				authorizer.Verifier = nil
			}
			if err != nil {
				t.Fatal(err)
			}
			response, duplicate, err := authorizer.ExecuteTx(ctx, work, candidate, mutation)
			if mode == "duplicate" {
				if err != nil || !duplicate || !bytes.Equal(response, []byte(`{"updated":true}`)) {
					t.Fatalf("stored response: %s %t %v", response, duplicate, err)
				}
			} else if err == nil || duplicate || len(response) != 0 {
				t.Fatalf("unbound operation admitted: %s %t %v", response, duplicate, err)
			}
			if want != "" {
				if len(observations) != 1 || observations[0] != want {
					t.Fatalf("operation category: %v want %s", observations, want)
				}
			} else if len(observations) != 0 {
				t.Fatalf("invalid configuration recorded operation outcome: %v", observations)
			}
			if mode == "nonce binding" && !errors.Is(err, ErrReplayMismatch) {
				t.Fatalf("nonce identity: %v", err)
			}
			var n int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM operation_ledger").Scan(&n); err != nil || n != 1 {
				t.Fatalf("durable operation count: %d %v", n, err)
			}
		})
	}
	if err := a.Verifier.Verify(ctx, "invalid", "message", "0x"); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("invalid wallet verification: %v", err)
	}
	if err := (WalletOperationVerifier{}).Verify(ctx, strings.ToLower(goldenAddr), "message", "0x"); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("missing chain verification: %v", err)
	}
}
