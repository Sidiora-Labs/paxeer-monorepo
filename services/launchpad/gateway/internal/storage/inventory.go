package storage

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/cleanup"
	sharedstorage "github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type InventoryReport struct {
	Checked     int       `json:"checked"`
	Orphans     []string  `json:"orphans"`
	Truncated   bool      `json:"truncated"`
	GeneratedAt time.Time `json:"generatedAt"`
}

type InventoryStore interface {
	BeginInventory(context.Context, string, string) (InventoryPass, error)
}

type InventoryPass interface {
	Cursor(context.Context) (string, error)
	Lineage(context.Context, string) (string, string, error)
	Advance(context.Context, string) error
	Commit(context.Context) error
	Rollback(context.Context) error
}

func (a *Authority) ReconcileInventory(ctx context.Context, client *s3.Client, bucket, prefix string, limit int) (InventoryReport, error) {
	return ReconcileInventory(ctx, postgresInventory{authority: a}, client, bucket, prefix, limit)
}

func ReconcileInventory(ctx context.Context, store InventoryStore, client *s3.Client, bucket, prefix string, limit int) (InventoryReport, error) {
	report := InventoryReport{Orphans: []string{}, GeneratedAt: time.Now().UTC()}
	if store == nil || client == nil || bucket == "" || limit < 1 || limit > 100000 || (prefix != approvedPrefix && prefix != privatePrefix && prefix != "quarantine/" && prefix != temporaryPrefix) {
		return report, ErrConflict
	}
	pass, err := store.BeginInventory(ctx, bucket, prefix)
	if err != nil {
		return report, ErrUnavailable
	}
	defer cleanup.Rollback(context.WithoutCancel(ctx), pass)
	token, err := pass.Cursor(ctx)
	if err != nil {
		return report, ErrUnavailable
	}
	for report.Checked < limit {
		maxKeys := min(1000, limit-report.Checked)
		// maxKeys is bounded to 1000 by the page limit above.
		input := &s3.ListObjectsV2Input{Bucket: aws.String(bucket), Prefix: aws.String(prefix), MaxKeys: aws.Int32(int32(maxKeys))} //nolint:gosec // S3's page field is int32 and this value is at most 1000.
		if token != "" {
			input.ContinuationToken = aws.String(token)
		}
		page, err := client.ListObjectsV2(ctx, input)
		if err != nil {
			return report, ErrUnavailable
		}
		if page == nil || len(page.Contents) > maxKeys {
			return report, ErrConflict
		}
		next := aws.ToString(page.NextContinuationToken)
		truncated := aws.ToBool(page.IsTruncated)
		if truncated && (len(page.Contents) == 0 || next == "" || next == token) {
			return report, ErrConflict
		}
		for _, object := range page.Contents {
			key := aws.ToString(object.Key)
			report.Checked++
			if !strings.HasPrefix(key, prefix) {
				return report, ErrConflict
			}
			hash, kind, err := pass.Lineage(ctx, key)
			if errors.Is(err, pgx.ErrNoRows) {
				report.Orphans = append(report.Orphans, key)
				continue
			}
			if err != nil {
				return report, ErrUnavailable
			}
			if kind == variantKind || kind == originalKind {
				expected := hash
				if kind == variantKind {
					if !ValidDeliveryKey(key) {
						return report, ErrConflict
					}
					expected = strings.TrimSuffix(strings.Split(key, "/")[3], ".png")
				}
				bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
				err = verifyInventoryHash(bounded, client, bucket, key, expected)
				cancel()
				if err != nil {
					return report, err
				}
			}
		}
		token = next
		report.Truncated = truncated
		if !truncated {
			token = ""
			break
		}
	}
	if err = pass.Advance(ctx, token); err != nil {
		return report, ErrUnavailable
	}
	if err = pass.Commit(ctx); err != nil {
		return report, ErrUnavailable
	}
	return report, nil
}

type postgresInventory struct{ authority *Authority }
type postgresInventoryPass struct {
	authority      *Authority
	bucket, prefix string
	pgx.Tx
}

func (s postgresInventory) BeginInventory(ctx context.Context, bucket, prefix string) (InventoryPass, error) {
	tx, err := s.authority.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", fmt.Sprintf("media-inventory:%d:%s:%s", len(bucket), bucket, prefix)); err != nil {
		_ = tx.Rollback(context.WithoutCancel(ctx))
		return nil, err
	}
	return &postgresInventoryPass{s.authority, bucket, prefix, tx}, nil
}

func (p *postgresInventoryPass) Cursor(ctx context.Context) (string, error) {
	var token string
	err := p.QueryRow(ctx, "SELECT continuation FROM "+p.authority.table("inventory_cursors")+" WHERE bucket=$1 AND prefix=$2", p.bucket, p.prefix).Scan(&token)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return token, err
}

func (p *postgresInventoryPass) Lineage(ctx context.Context, key string) (string, string, error) {
	var hash, kind string
	err := p.QueryRow(ctx, "SELECT hash,kind FROM "+p.authority.table("lineage")+" WHERE object_key=$1 AND removed_at IS NULL ORDER BY hash LIMIT 1", key).Scan(&hash, &kind)
	return hash, kind, err
}

func (p *postgresInventoryPass) Advance(ctx context.Context, token string) error {
	_, err := p.Exec(ctx, "INSERT INTO "+p.authority.table("inventory_cursors")+"(bucket,prefix,continuation) VALUES($1,$2,$3) ON CONFLICT(bucket,prefix) DO UPDATE SET continuation=EXCLUDED.continuation", p.bucket, p.prefix, token)
	return err
}

func (a *Authority) InventoryReportSchemaDDL() string {
	return "CREATE TABLE " + a.table("inventory_reports") + "(id uuid PRIMARY KEY,bucket text NOT NULL,prefix text NOT NULL,checked integer NOT NULL CHECK(checked>=0),orphans jsonb NOT NULL,truncated boolean NOT NULL,generated_at timestamptz NOT NULL);" + a.InventoryCursorSchemaDDL()
}

func (a *Authority) InventoryCursorSchemaDDL() string {
	return "CREATE TABLE " + a.table("inventory_cursors") + "(bucket text NOT NULL,prefix text NOT NULL,continuation text NOT NULL,PRIMARY KEY(bucket,prefix));"
}

func NewScopedObjectClient(ctx context.Context, config sharedstorage.Config) (*sharedstorage.Client, *s3.Client, error) {
	endpoint, e := url.Parse(config.Endpoint)
	if e != nil || endpoint.Scheme != httpsScheme || endpoint.Host == "" || endpoint.User != nil || endpoint.Path != "" || endpoint.RawQuery != "" || endpoint.Fragment != "" || config.Bucket == "" || config.Region == "" || config.AccessKeyID == "" || config.SecretAccessKey == "" {
		return nil, nil, ErrUnavailable
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	transport.TLSHandshakeTimeout = 2 * time.Second
	transport.ResponseHeaderTimeout = 3 * time.Second
	transport.MaxConnsPerHost = 4
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	cfg, e := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(config.Region), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.SecretAccessKey, "")), awsconfig.WithHTTPClient(client))
	if e != nil {
		return nil, nil, ErrUnavailable
	}
	raw := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(config.Endpoint)
		o.UsePathStyle = true
		o.RetryMaxAttempts = 2
	})
	return sharedstorage.NewFromS3(raw, config.Bucket), raw, nil
}
func (a *Authority) PersistInventory(ctx context.Context, bucket, prefix string, report InventoryReport) error {
	if bucket == "" || (prefix != approvedPrefix && prefix != privatePrefix && prefix != temporaryPrefix && prefix != "quarantine/") || report.Checked < 0 || report.Checked > 100000 || len(report.Orphans) > 100000 || report.GeneratedAt.IsZero() {
		return ErrConflict
	}
	raw, e := json.Marshal(report.Orphans)
	if e != nil {
		return ErrConflict
	}
	if _, e = a.Pool.Exec(ctx, "INSERT INTO "+a.table("inventory_reports")+"(id,bucket,prefix,checked,orphans,truncated,generated_at) VALUES($1,$2,$3,$4,$5,$6,$7)", uuid.NewString(), bucket, prefix, report.Checked, raw, report.Truncated, report.GeneratedAt); e != nil {
		return ErrUnavailable
	}
	return nil
}

func verifyInventoryHash(ctx context.Context, client *s3.Client, bucket, key, expected string) error {
	object, e := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if e != nil {
		return ErrUnavailable
	}
	if object == nil || object.Body == nil {
		return ErrUnavailable
	}
	defer func() {
		if closeErr := object.Body.Close(); closeErr != nil {
			cleanup.Report("close response body", closeErr)
		}
	}()
	size := aws.ToInt64(object.ContentLength)
	if size < 1 || size > 32<<20 || !digest.MatchString(expected) {
		return ErrConflict
	}
	hash := sha256.New()
	read, e := io.Copy(hash, io.LimitReader(object.Body, (32<<20)+1))
	if e != nil {
		return ErrUnavailable
	}
	if read != size || fmt.Sprintf("%x", hash.Sum(nil)) != expected {
		return ErrConflict
	}
	return nil
}
