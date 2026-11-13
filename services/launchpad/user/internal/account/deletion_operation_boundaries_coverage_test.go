package account

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
)

func TestDeletionOperationValidatesOwnerInputBeforeLedger(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 9671)
	other := owner(t, s, 9672)
	verify := func(_ context.Context, actor, statement, signature string) error {
		_, err := sharedauth.VerifyEOA(actor, statement, signature, nil)
		return err
	}
	body, err := json.Marshal(DeletionBody{ConfirmAccountID: p.AccountID.String(), ExpectedRevision: "0", ReauthProof: "proof"})
	if err != nil {
		t.Fatal(err)
	}
	req := sharedauth.OperationRequest{Actor: "0x00000000000000000000000000000000000025c7", Action: "account.delete", Method: "POST", Path: "/v1/me/deletion-requests", RequestID: uuid.NewString(), Body: body}
	ledger := sharedauth.Ledger{Pool: s.DB}
	for _, tc := range []struct {
		name      string
		principal Principal
		request   sharedauth.OperationRequest
		verify    func(context.Context, string, string, string) error
		want      error
	}{
		{"missing verifier", p, req, nil, nil},
		{"wrong action", p, func() sharedauth.OperationRequest { v := req; v.Action = "account.export"; return v }(), verify, ErrInvalid},
		{"invalid request ID", p, func() sharedauth.OperationRequest { v := req; v.RequestID = "invalid"; return v }(), verify, ErrInvalid},
		{"malformed body", p, func() sharedauth.OperationRequest { v := req; v.Body = []byte("{"); return v }(), verify, ErrInvalid},
		{"different owner", other, req, verify, ErrDenied},
		{"expired proof", Principal{AccountID: p.AccountID, ProofAt: s.now().Add(-24 * time.Hour)}, req, verify, ErrDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, replayed, err := s.RequestDeletionOperation(context.Background(), tc.principal, ledger, tc.request, uuid.New(), uuid.New(), tc.verify)
			if err == nil || replayed || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("deletion operation = replayed %t, %v; want %v", replayed, err, tc.want)
			}
		})
	}
}

func TestExportSnapshotRefusesAnonymousAndDeletedAccounts(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 9673)
	ctx := context.Background()
	if _, err := s.ExportSnapshot(ctx, Principal{}); !errors.Is(err, ErrDenied) {
		t.Fatalf("anonymous account export = %v", err)
	}
	if _, err := s.DB.Exec(ctx, `UPDATE account_state SET deleted=true WHERE account_id=$1`, p.AccountID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExportSnapshot(ctx, p); !errors.Is(err, ErrState) {
		t.Fatalf("deleted account export = %v", err)
	}
}

func TestBackfillQuarantinesAnAccountWalletConflict(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 9674)
	ctx := context.Background()
	digest := sha256.Sum256([]byte("legacy source"))
	if err := s.Backfill(ctx, "legacy-invalid", p.AccountID, "invalid", digest[:]); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid source wallet = %v", err)
	}
	if err := s.Backfill(ctx, "legacy-invalid", p.AccountID, "0x2222222222222222222222222222222222222222", digest[:31]); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short source digest = %v", err)
	}
	const wallet = "0x2222222222222222222222222222222222222222"
	if err := s.Backfill(ctx, "legacy-wallet-conflict", p.AccountID, wallet, digest[:]); err != nil {
		t.Fatal(err)
	}
	var state, reason string
	if err := s.DB.QueryRow(ctx, `SELECT state,reason FROM account_backfill WHERE source_key='legacy-wallet-conflict'`).Scan(&state, &reason); err != nil || state != "quarantined" || reason == "" {
		t.Fatalf("conflicting account source = %q %q, %v", state, reason, err)
	}
	var count int
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM account_state WHERE wallet=$1`, wallet).Scan(&count); err != nil || count != 0 {
		t.Fatalf("conflicting source created account = %d, %v", count, err)
	}
}
