package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func auditCoverageDatabase(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("L34_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("authorization PostgreSQL fixture is required")
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "kd_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:24] + "_audit"
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	if _, err := pool.Exec(ctx, LedgerDDL+AuditDeliveryDDL); err != nil {
		t.Fatal(err)
	}
	return pool, schema
}

func TestAuditStageAndStateSnapshot(t *testing.T) {
	ctx := context.Background()
	pool, schema := auditCoverageDatabase(t)
	subject := "security.audit.fixture"
	ddl, err := delivery.SchemaDDL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	store, err := delivery.NewStore(pool, schema, delivery.Policy{
		Subjects: map[string]bool{subject: true}, EventTypes: map[string]bool{"security_audit": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	ledger := Ledger{Pool: pool, Environment: "test", AuditKey: bytes.Repeat([]byte("a"), 32)}
	requestID := uuid.NewString()
	event := AuditEvent{RequestID: requestID, Actor: goldenAddr, Action: "sign_in", Result: valueDenied, PolicyVersion: "2", RequestDigest: strings.Repeat("b", 64)}
	if err := ledger.RecordDenial(ctx, event); err != nil {
		t.Fatal(err)
	}
	bridge := AuditBridge{Pool: pool, Delivery: store, Subject: subject, EventType: "security_audit"}
	count, age, err := bridge.Pending(ctx)
	if err != nil || count != 1 || age < 0 {
		t.Fatalf("pending audit: count=%d age=%s err=%v", count, age, err)
	}
	if staged, err := bridge.Stage(ctx, 10); err != nil || staged != 1 {
		t.Fatalf("stage audit: %d, %v", staged, err)
	}
	if staged, err := bridge.Stage(ctx, 10); err != nil || staged != 0 {
		t.Fatalf("duplicate stage: %d, %v", staged, err)
	}
	count, _, err = bridge.Pending(ctx)
	if err != nil || count != 0 {
		t.Fatalf("staged pending audit: %d, %v", count, err)
	}
	stats, err := store.Stats(ctx)
	if err != nil || stats.PendingRows != 1 {
		t.Fatalf("delivery outbox: %+v, %v", stats, err)
	}
	missing, err := bridge.MissingOperationAudits(ctx)
	if err != nil || missing != 0 {
		t.Fatalf("missing accepted audit: %d, %v", missing, err)
	}
	before, err := ReadAuthStateSnapshot(ctx, pool)
	if err != nil || before.AuditRecords != 1 || before.LedgerRecords != 0 || before.MissingOperationAudits != 0 {
		t.Fatalf("audit snapshot: %+v, %v", before, err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO operation_ledger(actor,action,nonce,envelope_digest,body_digest,request_id,response,response_digest) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", goldenAddr, "account.export", "n", strings.Repeat("c", 64), strings.Repeat("d", 64), uuid.NewString(), []byte("ok"), strings.Repeat("e", 64)); err != nil {
		t.Fatal(err)
	}
	after, err := ReadAuthStateSnapshot(ctx, pool)
	if err != nil || after.LedgerRecords != 1 || after.MissingOperationAudits != 1 || after.LedgerSHA256 == before.LedgerSHA256 {
		t.Fatalf("ledger snapshot: %+v, %v", after, err)
	}
	if err := CompareAuthRecovery(before, after); err == nil {
		t.Fatal("snapshot comparison accepted changed durable records")
	}
	missing, err = bridge.MissingOperationAudits(ctx)
	if err != nil || missing != 1 {
		t.Fatalf("missing operation audit: %d, %v", missing, err)
	}
}

func TestAuditPayloadValidationAndConfiguration(t *testing.T) {
	digest := sha256.Sum256([]byte("actor"))
	p := AuditPayload{Version: 1, Revision: "1", RequestID: uuid.NewString(), ActorPseudonym: hex.EncodeToString(digest[:]), Action: "sign_in", Result: valueDenied, PolicyVersion: "2", RequestDigest: hex.EncodeToString(digest[:]), CreatedAt: time.Now()}
	if err := p.validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*AuditPayload)
	}{
		{"version", func(p *AuditPayload) { p.Version = 2 }},
		{"revision", func(p *AuditPayload) { p.Revision = "01" }},
		{"request", func(p *AuditPayload) { p.RequestID = uuid.Nil.String() }},
		{"actor", func(p *AuditPayload) { p.ActorPseudonym = strings.ToUpper(p.ActorPseudonym) }},
		{"digest", func(p *AuditPayload) { p.RequestDigest = "short" }},
		{"time", func(p *AuditPayload) { p.CreatedAt = time.Time{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := p
			tc.edit(&invalid)
			if err := invalid.validate(); !errors.Is(err, ErrScopeDenied) {
				t.Fatalf("invalid audit payload: %v", err)
			}
		})
	}
	if auditEventID(p.RequestID, p.Action) != "audit_"+delivery.Hash([]byte(p.RequestID+"\x00"+p.Action)) || auditEventID(p.RequestID, p.Action) == auditEventID(p.RequestID, "refresh") {
		t.Fatal("audit event identity does not bind action")
	}
	if staged, err := (AuditBridge{}).Stage(context.Background(), 1); err == nil || staged != 0 {
		t.Fatalf("unconfigured bridge staged %d audits: %v", staged, err)
	}
	if _, _, err := (AuditBridge{}).Pending(context.Background()); err == nil {
		t.Fatal("unconfigured bridge reported pending audits")
	}
	if _, err := (AuditBridge{}).MissingOperationAudits(context.Background()); err == nil {
		t.Fatal("unconfigured bridge reported missing audits")
	}
	if err := (AuditSink{}).Consume(context.Background(), nil, nil); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("unconfigured sink: %v", err)
	}
	if err := (Ledger{}).RecordDenial(context.Background(), AuditEvent{Result: valueDenied}); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("unconfigured denial recorder: %v", err)
	}
	if !IsAuditIntegrityError(delivery.ErrIntegrity) || IsAuditIntegrityError(nil) {
		t.Fatal("audit integrity classification is inconsistent")
	}
}
