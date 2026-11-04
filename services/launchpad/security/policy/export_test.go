package policy

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAuditExportMissingProviderCannotSucceed(t *testing.T) {
	if _, err := (AuditExporter{}).Export(context.Background(), time.Now().Add(-48*time.Hour)); err == nil {
		t.Fatal("missing retained storage authority reported success")
	}
}

type retainedAuditObject struct {
	Body    []byte
	Until   time.Time
	Version string
}

type retainedAuditPeer struct {
	mu           sync.Mutex
	Directory    string
	Objects      map[string]retainedAuditObject
	Puts         int
	FailAt       int
	FailAfter    bool
	CorruptRead  bool
	BadRetention bool
}

func openRetainedAuditPeer(t *testing.T, directory string) *retainedAuditPeer {
	t.Helper()
	peer := &retainedAuditPeer{Directory: directory, Objects: map[string]retainedAuditObject{}}
	wire, err := os.ReadFile(filepath.Join(directory, "objects.json"))
	if err == nil {
		if err := json.Unmarshal(wire, &peer.Objects); err != nil {
			t.Fatal(err)
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return peer
}

func (p *retainedAuditPeer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	w.Header().Set("Content-Type", "application/xml")
	if r.URL.Query().Has("object-lock") {
		fmt.Fprint(w, `<ObjectLockConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><ObjectLockEnabled>Enabled</ObjectLockEnabled></ObjectLockConfiguration>`)
		return
	}
	key := r.URL.Path
	object, exists := p.Objects[key]
	failure := func() {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `<Error><Code>ServiceUnavailable</Code><Message>interrupted upload</Message></Error>`)
	}
	if r.Method == http.MethodPut {
		p.Puts++
		if p.Puts == p.FailAt && !p.FailAfter {
			failure()
			return
		}
		if exists || r.Header.Get("If-None-Match") != "*" || r.Header.Get("X-Amz-Object-Lock-Mode") != "COMPLIANCE" {
			w.WriteHeader(http.StatusPreconditionFailed)
			fmt.Fprint(w, `<Error><Code>PreconditionFailed</Code></Error>`)
			return
		}
		until, err := time.Parse(time.RFC3339, r.Header.Get("X-Amz-Object-Lock-Retain-Until-Date"))
		if err != nil || !until.After(time.Now().Add(365*24*time.Hour)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		object = retainedAuditObject{Body: body, Until: until, Version: "retained-v1"}
		p.Objects[key] = object
		wire, err := json.Marshal(p.Objects)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(filepath.Join(p.Directory, "objects.json"), wire, 0600); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if p.Puts == p.FailAt && p.FailAfter {
			failure()
			return
		}
		w.Header().Set("X-Amz-Version-Id", object.Version)
		w.WriteHeader(http.StatusOK)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `<Error><Code>NoSuchKey</Code><Message>not retained</Message></Error>`)
		return
	}
	if version := r.URL.Query().Get("versionId"); version != "" && version != object.Version {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("X-Amz-Version-Id", object.Version)
	if r.URL.Query().Has("retention") {
		mode := "COMPLIANCE"
		if p.BadRetention {
			mode = "GOVERNANCE"
		}
		fmt.Fprintf(w, `<Retention xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Mode>%s</Mode><RetainUntilDate>%s</RetainUntilDate></Retention>`, mode, object.Until.Format(time.RFC3339))
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	body := object.Body
	if p.CorruptRead {
		body = append(append([]byte{}, body...), 'x')
	}
	w.Write(body)
}

func retainedAuditClient(server *httptest.Server) *s3.Client {
	return s3.New(s3.Options{Region: "us-east-1", BaseEndpoint: aws.String(server.URL), UsePathStyle: true,
		Credentials: aws.AnonymousCredentials{}, Retryer: aws.NopRetryer{},
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired})
}

func TestRepairRetainedExportRecoveryAndCommittedCoverage(t *testing.T) {
	pool, schema := realSecurityDatabase(t)
	ctx := context.Background()
	ddl, err := AuditExportDDL(schema)
	if err != nil {
		t.Fatal(err)
	}
	day := time.Now().UTC().Truncate(24 * time.Hour).Add(-24 * time.Hour)
	table := pgx.Identifier{schema, "audit_outbox"}.Sanitize()
	if _, err = pool.Exec(ctx, ddl+"ALTER TABLE "+table+" ALTER COLUMN at SET DEFAULT '"+day.Add(time.Hour).Format(time.RFC3339)+"'::timestamptz;CREATE TABLE "+pgx.Identifier{schema, "export_mutations"}.Sanitize()+"(id integer)"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		err := MutateAuditedV2(ctx, pool, pgx.Identifier{schema, "audit_outbox"}, Audit{Actor: "operator-a", Action: "export-proof", Target: fmt.Sprint(i), BeforeDigest: strings.Repeat("a", 64), AfterDigest: strings.Repeat("b", 64), PolicyVersion: "v3"}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO "+pgx.Identifier{schema, "export_mutations"}.Sanitize()+" VALUES($1)", i)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	for boundary := 1; boundary <= 3; boundary++ {
		for _, after := range []bool{false, true} {
			t.Run(fmt.Sprintf("boundary-%d-after-%t", boundary, after), func(t *testing.T) {
				directory := t.TempDir()
				peer := openRetainedAuditPeer(t, directory)
				peer.FailAt, peer.FailAfter = boundary, after
				server := httptest.NewServer(peer)
				exporter := AuditExporter{Pool: pool, StatePool: pool, Schema: schema, Bucket: "audit", Prefix: fmt.Sprintf("proof-%d-%t", boundary, after), KeyID: "audit-key-v1", Client: retainedAuditClient(server), Signer: key}
				if err := exporter.CatchUp(ctx, day); err == nil {
					t.Fatal("interrupted boundary reported completed export")
				}
				var complete bool
				if err := pool.QueryRow(ctx, "SELECT complete FROM "+pgx.Identifier{schema, "audit_export_days"}.Sanitize()+" WHERE bucket=$1 AND prefix=$2 AND day=$3", exporter.Bucket, exporter.Prefix, day).Scan(&complete); err != nil || complete {
					t.Fatal("incomplete upload advanced durable progress", complete, err)
				}
				server.Close()
				// Stop and reopen both the retained peer and exporter. Only durable
				// PostgreSQL progress and on-disk retained objects survive.
				reopened := openRetainedAuditPeer(t, directory)
				server = httptest.NewServer(reopened)
				defer server.Close()
				exporter = AuditExporter{Pool: pool, StatePool: pool, Schema: schema, Bucket: exporter.Bucket, Prefix: exporter.Prefix, KeyID: exporter.KeyID, Client: retainedAuditClient(server), Signer: key}
				if err := exporter.CatchUp(ctx, day); err != nil {
					t.Fatal("restart did not resume completed day", err)
				}
				manifest, err := exporter.Export(ctx, day)
				if err != nil || manifest.Rows != 3 || manifest.First != 1 || manifest.Last != 3 {
					t.Fatal("committed producer coverage differs", manifest, err)
				}
				base := "/audit/" + exporter.Prefix + "/" + manifest.Day + "/" + manifest.SHA256
				reopened.mu.Lock()
				data := reopened.Objects[base+".jsonl"]
				wire := reopened.Objects[base+".manifest.json"]
				signature := reopened.Objects[base+".signature"]
				count := len(reopened.Objects)
				reopened.mu.Unlock()
				if count != 3 || !ed25519.Verify(public, wire.Body, signature.Body) || fmt.Sprintf("%x", sha256.Sum256(data.Body)) != manifest.SHA256 {
					t.Fatal("retained bytes/signature/unique object coverage invalid", count)
				}
				lines := bytes.Split(bytes.TrimSpace(data.Body), []byte{'\n'})
				if len(lines) != 3 || data.Until.Before(manifest.RetainUntil) {
					t.Fatal("retained journal coverage or retention differs")
				}
				for i, line := range lines {
					var record struct {
						Sequence int64 `json:"sequence"`
						Record   Audit `json:"record"`
					}
					if err := json.Unmarshal(line, &record); err != nil || record.Sequence != int64(i+1) || record.Record.Target != fmt.Sprint(i) {
						t.Fatal("producer record was lost or duplicated", i, record, err)
					}
				}
				if err := exporter.CatchUp(ctx, day); err != nil || len(reopened.Objects) != 3 {
					t.Fatal("completed export was duplicated", err)
				}
				reopened.CorruptRead = true
				if _, err := exporter.Export(ctx, day); err == nil {
					t.Fatal("corrupt retained readback accepted")
				}
				reopened.CorruptRead, reopened.BadRetention = false, true
				if _, err := exporter.Export(ctx, day); err == nil {
					t.Fatal("governance retention accepted as immutable compliance")
				}
			})
		}
	}
}

func TestRepairAuditSchedulerStartupCatchUpAndAlerts(t *testing.T) {
	pool, schema := realSecurityDatabase(t)
	ctx := context.Background()
	ddl, err := AuditExportDDL(schema)
	if err != nil {
		t.Fatal(err)
	}
	role := pgx.Identifier{schema + "_export"}.Sanitize()
	schemaID := pgx.Identifier{schema}.Sanitize()
	if _, err := pool.Exec(ctx, ddl+"CREATE ROLE "+role+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;GRANT USAGE ON SCHEMA "+schemaID+" TO "+role+";GRANT SELECT ON "+schemaID+".audit_outbox TO "+role+";GRANT SELECT,INSERT,UPDATE ON "+schemaID+".audit_export_days TO "+role); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DROP OWNED BY "+role+";DROP ROLE "+role); err != nil {
			t.Error(err)
		}
	})
	config := pool.Config().Copy()
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE "+role)
		return err
	}
	limited, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer limited.Close()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	peer := openRetainedAuditPeer(t, t.TempDir())
	server := httptest.NewServer(peer)
	defer server.Close()
	day := time.Now().UTC().Truncate(24 * time.Hour).Add(-24 * time.Hour)
	exporter := AuditExporter{Pool: limited, StatePool: limited, Schema: schema, Bucket: "audit", Prefix: "scheduled", KeyID: "key-v1", Signer: key, Client: retainedAuditClient(server)}
	peer.FailAt = 1
	alerts := 0
	err = exporter.Run(ctx, day, func(ctx context.Context, err error) error {
		if err == nil {
			t.Error("empty export alert")
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Error("alert delivery has no bound")
		}
		alerts++
		return nil
	})
	if err == nil || alerts != 1 {
		t.Fatal("startup upload failure was not surfaced", alerts, err)
	}
	peer.mu.Lock()
	peer.FailAt = 0
	peer.mu.Unlock()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		finished <- exporter.Run(runCtx, day, func(context.Context, error) error { return errors.New("unexpected export failure") })
	}()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		var complete bool
		if err := pool.QueryRow(ctx, "SELECT complete FROM "+pgx.Identifier{schema, "audit_export_days"}.Sanitize()+" WHERE bucket='audit' AND prefix='scheduled' AND day=$1", day).Scan(&complete); err == nil && complete {
			break
		}
		select {
		case err := <-finished:
			t.Fatal("scheduler stopped before startup catch-up", err)
		case <-deadline.C:
			t.Fatal("startup catch-up did not finish")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Fatal("scheduler cancellation", err)
	}
	var manifests int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{schema, "audit_export_days"}.Sanitize()+" WHERE complete").Scan(&manifests); err != nil || manifests != 1 {
		t.Fatal("completed-day progress", manifests, err)
	}
}
