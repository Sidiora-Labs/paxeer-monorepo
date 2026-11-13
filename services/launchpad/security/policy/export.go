package policy

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/md5" //nolint:gosec // S3 Object Lock requires the MD5 transport checksum.
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditManifest struct {
	Version     int    `json:"version"`
	Day         string `json:"day"`
	Schema      string `json:"schema"`
	Rows        int64  `json:"rows"`
	First, Last int64
	SHA256      string    `json:"sha256"`
	KeyID       string    `json:"keyId"`
	RetainUntil time.Time `json:"retainUntil"`
}
type AuditExporter struct {
	Pool                          *pgxpool.Pool
	StatePool                     *pgxpool.Pool
	Schema, Bucket, Prefix, KeyID string
	Client                        *s3.Client
	Signer                        ed25519.PrivateKey
}

func (e AuditExporter) Export(ctx context.Context, day time.Time) (manifest AuditManifest, resultErr error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if e.Pool == nil || e.StatePool == nil || e.Client == nil || !safeIdentifier(e.Schema) || e.Bucket == "" || !safeExportPrefix(e.Prefix) || e.KeyID == "" || len(e.Signer) != ed25519.PrivateKeySize {
		return manifest, ErrDenied
	}
	start := time.Date(day.UTC().Year(), day.UTC().Month(), day.UTC().Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	if end.After(time.Now()) {
		return manifest, ErrDenied
	}
	config, err := e.Client.GetObjectLockConfiguration(ctx, &s3.GetObjectLockConfigurationInput{Bucket: aws.String(e.Bucket)})
	if err != nil || config.ObjectLockConfiguration == nil || config.ObjectLockConfiguration.ObjectLockEnabled != types.ObjectLockEnabledEnabled {
		return manifest, errors.New("policy: retention authority unavailable")
	}
	tx, err := e.Pool.Begin(ctx)
	if err != nil {
		return manifest, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if rollbackErr := tx.Rollback(cleanup); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			resultErr = errors.Join(resultErr, rollbackErr)
		}
	}()
	table := pgx.Identifier{e.Schema, auditOutboxTable}.Sanitize()
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", table); err != nil {
		return manifest, err
	}
	rows, err := tx.Query(ctx, "SELECT seq,at,record FROM "+table+" WHERE at>=$1 AND at<$2 ORDER BY seq", start, end)
	if err != nil {
		return manifest, err
	}
	defer rows.Close()
	var buf bytes.Buffer
	manifest = AuditManifest{Version: 1, Day: start.Format("2006-01-02"), Schema: e.Schema, KeyID: e.KeyID, RetainUntil: time.Now().UTC().Truncate(time.Second).Add(366 * 24 * time.Hour)}
	for rows.Next() {
		var seq int64
		var at time.Time
		var record json.RawMessage
		if err = rows.Scan(&seq, &at, &record); err != nil {
			return manifest, err
		}
		if manifest.Rows == 0 {
			manifest.First = seq
		}
		manifest.Last = seq
		manifest.Rows++
		if err = json.NewEncoder(&buf).Encode(struct {
			Sequence int64           `json:"sequence"`
			At       time.Time       `json:"at"`
			Record   json.RawMessage `json:"record"`
		}{seq, at, record}); err != nil {
			return manifest, err
		}
		if buf.Len() > 64<<20 {
			return manifest, errors.New("policy: audit export requires partitioning")
		}
	}
	if err = rows.Err(); err != nil {
		return manifest, err
	}
	rows.Close()
	if err = tx.Commit(ctx); err != nil {
		return manifest, err
	}
	sum := sha256.Sum256(buf.Bytes())
	manifest.SHA256 = hex.EncodeToString(sum[:])
	wire, err := json.Marshal(manifest)
	if err != nil {
		return manifest, err
	}
	// Persist the exact signed plan before the first upload. Retries reuse its
	// retention timestamp and bytes, including after a process restart.
	state := pgx.Identifier{e.Schema, auditExportDaysTable}.Sanitize()
	if _, err = e.StatePool.Exec(ctx, "INSERT INTO "+state+"(bucket,prefix,day,manifest) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING", e.Bucket, e.Prefix, start, wire); err != nil {
		return manifest, err
	}
	var planned []byte
	if err = e.StatePool.QueryRow(ctx, "SELECT manifest FROM "+state+" WHERE bucket=$1 AND prefix=$2 AND day=$3", e.Bucket, e.Prefix, start).Scan(&planned); err != nil {
		return manifest, err
	}
	var saved AuditManifest
	if err = json.Unmarshal(planned, &saved); err != nil {
		return manifest, err
	}
	if saved.Version != manifest.Version || saved.Day != manifest.Day || saved.Schema != manifest.Schema || saved.KeyID != manifest.KeyID || saved.SHA256 != manifest.SHA256 || saved.Rows != manifest.Rows || saved.First != manifest.First || saved.Last != manifest.Last || saved.RetainUntil.Before(end.Add(365*24*time.Hour)) || !saved.RetainUntil.After(time.Now()) {
		return manifest, errors.New("policy: committed audit coverage differs from durable export plan")
	}
	manifest, wire = saved, planned
	signature := ed25519.Sign(e.Signer, wire)
	key := e.Prefix + "/" + manifest.Day + "/" + manifest.SHA256
	for _, artifact := range []struct {
		suffix string
		body   []byte
	}{{".jsonl", buf.Bytes()}, {".signature", signature}, {".manifest.json", wire}} {
		body := artifact.body
		object := key + artifact.suffix
		if err = e.retainObject(ctx, object, body, manifest.RetainUntil); err != nil {
			return manifest, err
		}
	}
	if _, err = e.StatePool.Exec(ctx, "UPDATE "+state+" SET complete=true WHERE bucket=$1 AND prefix=$2 AND day=$3 AND manifest=$4", e.Bucket, e.Prefix, start, wire); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func safeExportPrefix(prefix string) bool {
	if prefix == "" || strings.HasPrefix(prefix, "/") || strings.HasSuffix(prefix, "/") || strings.ContainsAny(prefix, "\\\r\n\x00*") {
		return false
	}
	for _, part := range strings.Split(prefix, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func (e AuditExporter) retainObject(ctx context.Context, key string, body []byte, until time.Time) (resultErr error) {
	object, err := e.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(e.Bucket), Key: aws.String(key)})
	if err != nil {
		var api smithy.APIError
		if !errors.As(err, &api) || api.ErrorCode() != "NoSuchKey" {
			return fmt.Errorf("policy: retained audit readback: %w", err)
		}
		checksum := md5.Sum(body) //nolint:gosec // S3 Object Lock requires the MD5 transport checksum.
		output, putErr := e.Client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(e.Bucket), Key: aws.String(key), Body: bytes.NewReader(body), IfNoneMatch: aws.String("*"), ContentMD5: aws.String(base64.StdEncoding.EncodeToString(checksum[:])), ObjectLockMode: types.ObjectLockModeCompliance, ObjectLockRetainUntilDate: &until, ContentType: aws.String("application/octet-stream")})
		if putErr != nil {
			return fmt.Errorf("policy: retained audit upload: %w", putErr)
		}
		if output.VersionId == nil || *output.VersionId == "" {
			return errors.New("policy: retained audit version absent")
		}
		object, err = e.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(e.Bucket), Key: aws.String(key), VersionId: output.VersionId})
		if err != nil {
			return err
		}
	}
	defer func() { resultErr = errors.Join(resultErr, object.Body.Close()) }()
	readback, err := io.ReadAll(io.LimitReader(object.Body, int64(len(body))+1))
	if err != nil || !bytes.Equal(readback, body) || object.VersionId == nil || *object.VersionId == "" {
		return errors.New("policy: retained audit bytes or version differ")
	}
	retention, err := e.Client.GetObjectRetention(ctx, &s3.GetObjectRetentionInput{Bucket: aws.String(e.Bucket), Key: aws.String(key), VersionId: object.VersionId})
	if err != nil || retention.Retention == nil || retention.Retention.Mode != types.ObjectLockRetentionModeCompliance || retention.Retention.RetainUntilDate == nil || retention.Retention.RetainUntilDate.Before(until) {
		return errors.New("policy: audit retention not confirmed")
	}
	return nil
}

func AuditExportDDL(schema string) (string, error) {
	if !safeIdentifier(schema) {
		return "", ErrDenied
	}
	return "CREATE TABLE IF NOT EXISTS " + pgx.Identifier{schema, auditExportDaysTable}.Sanitize() + "(bucket text NOT NULL,prefix text NOT NULL,day date NOT NULL,manifest bytea NOT NULL,complete boolean NOT NULL DEFAULT false,PRIMARY KEY(bucket,prefix,day));", nil
}

// Run catches up completed UTC days before sleeping. firstDay is the durable
// journal inception day supplied by the owning runtime, never the boot date.
func (e AuditExporter) Run(ctx context.Context, firstDay time.Time, alert func(context.Context, error) error) error {
	if alert == nil || e.Pool == nil || e.StatePool == nil || e.Client == nil || firstDay.IsZero() || !safeIdentifier(e.Schema) || e.Bucket == "" || !safeExportPrefix(e.Prefix) || e.KeyID == "" || len(e.Signer) != ed25519.PrivateKeySize {
		return ErrDenied
	}
	report := func(err error) error {
		alertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return errors.Join(err, alert(alertCtx, err))
	}
	if err := ValidateDatabaseRole(ctx, e.Pool, e.Schema); err != nil {
		return report(err)
	}
	if err := ValidateDatabaseRole(ctx, e.StatePool, e.Schema); err != nil {
		return report(err)
	}
	var sourceWrite bool
	if err := e.Pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,$1,'INSERT,UPDATE,DELETE,TRUNCATE') OR has_any_column_privilege(current_user,$1,'INSERT,UPDATE')`, pgx.Identifier{e.Schema, auditOutboxTable}.Sanitize()).Scan(&sourceWrite); err != nil || sourceWrite {
		return report(ErrDenied)
	}
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		if err := e.CatchUp(ctx, firstDay); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return report(err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (e AuditExporter) CatchUp(ctx context.Context, firstDay time.Time) error {
	if e.Pool == nil || e.StatePool == nil || e.Client == nil || !safeIdentifier(e.Schema) || e.Bucket == "" || !safeExportPrefix(e.Prefix) || e.KeyID == "" || len(e.Signer) != ed25519.PrivateKeySize || firstDay.IsZero() || firstDay.After(time.Now()) {
		return ErrDenied
	}
	day := time.Date(firstDay.UTC().Year(), firstDay.UTC().Month(), firstDay.UTC().Day(), 0, 0, 0, 0, time.UTC)
	for !day.Add(24 * time.Hour).After(time.Now().UTC()) {
		var complete bool
		err := e.StatePool.QueryRow(ctx, "SELECT complete FROM "+pgx.Identifier{e.Schema, auditExportDaysTable}.Sanitize()+" WHERE bucket=$1 AND prefix=$2 AND day=$3", e.Bucket, e.Prefix, day).Scan(&complete)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !complete {
			if _, err = e.Export(ctx, day); err != nil {
				return err
			}
		}
		day = day.Add(24 * time.Hour)
	}
	return nil
}
