package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	raster "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/metadata"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
)

func TestPendingMediaScanPromotesAndRejects(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	const schema = "media_scan"
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	store, err := metadata.NewProductionStore(pool, schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, store.SchemaDDL()+store.V2SchemaDDL()+store.ConnectedSchemaDDL()); err != nil {
		t.Fatal(err)
	}
	const poolAddress = "0x1111111111111111111111111111111111111111"
	const creator = "0x2222222222222222222222222222222222222222"
	const token = "0x3333333333333333333333333333333333333333"
	if _, err := pool.Exec(ctx, "INSERT INTO "+pgx.Identifier{schema, "pools"}.Sanitize()+"(pool,creator,token) VALUES($1,$2,$3)", poolAddress, creator, token); err != nil {
		t.Fatal(err)
	}
	quarantine := internaltest.NewMinIO(t, "metadata-scan-quarantine")
	approved := internaltest.NewMinIO(t, "metadata-scan-approved")
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("us-east-1"), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(approved.AccessKeyID, approved.SecretAccessKey, "")))
	if err != nil {
		t.Fatal(err)
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(approved.Endpoint)
		options.UsePathStyle = true
	})
	host, port := internaltest.NewClamd(t)
	if host == "" {
		t.Fatal("scanner required for media maintenance")
	}
	media := metadata.ConnectedMedia{
		Metadata:   metadata.V2ContractHandlers{Store: store},
		Objects:    metadata.S3AssetObjects{Client: client, QuarantineBucket: quarantine.Bucket, PublicBucket: approved.Bucket, PublicOrigin: "https://metadata.sidiora.test"},
		Scanner:    sharedhttp.ScanOptions{Host: host, Port: port},
		Worker:     raster.InProcessWorker{},
		Moderation: metadata.DevelopmentVisibility{},
	}
	connection, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Release()
	imageBytes := func(width int) []byte {
		t.Helper()
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, image.NewNRGBA(image.Rect(0, 0, width, 12))); err != nil {
			t.Fatal(err)
		}
		return buffer.Bytes()
	}
	create := func(width int, state string) (string, []byte) {
		t.Helper()
		id := uuid.NewString()
		body := imageBytes(width)
		digest := sha256.Sum256(body)
		record := metadata.AssetRecord{ID: id, Pool: poolAddress, Hash: hex.EncodeToString(digest[:]), Size: int64(len(body)), ObjectKey: "quarantine/" + id}
		if err := store.CreateConnectedAsset(ctx, record, "image/png", width, 12); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "UPDATE "+pgx.Identifier{schema, "assets"}.Sanitize()+" SET state=$2 WHERE id=$1", id, state); err != nil {
			t.Fatal(err)
		}
		return id, body
	}
	validID, validBody := create(16, "QUARANTINED")
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(quarantine.Bucket), Key: aws.String("quarantine/" + validID), Body: bytes.NewReader(validBody), ContentType: aws.String("image/png")}); err != nil {
		t.Fatal(err)
	}
	missingID, _ := create(17, "UPLOADING")
	badKeyID, _ := create(18, "UPLOADING")
	if _, err := pool.Exec(ctx, "UPDATE "+pgx.Identifier{schema, "assets"}.Sanitize()+" SET object_key='unscoped/object' WHERE id=$1", badKeyID); err != nil {
		t.Fatal(err)
	}
	changedID, _ := create(19, "QUARANTINED")
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(quarantine.Bucket), Key: aws.String("quarantine/" + changedID), Body: bytes.NewReader(imageBytes(20)), ContentType: aws.String("image/png")}); err != nil {
		t.Fatal(err)
	}
	invalidID, invalidBody := create(21, "VALIDATING")
	invalidBody = invalidBody[:20]
	invalidHash := sha256.Sum256(invalidBody)
	if _, err := pool.Exec(ctx, "UPDATE "+pgx.Identifier{schema, "assets"}.Sanitize()+" SET hash=$2,size=$3 WHERE id=$1", invalidID, hex.EncodeToString(invalidHash[:]), len(invalidBody)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(quarantine.Bucket), Key: aws.String("quarantine/" + invalidID), Body: bytes.NewReader(invalidBody), ContentType: aws.String("image/png")}); err != nil {
		t.Fatal(err)
	}
	if err := scanPendingMedia(ctx, connection, media); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{validID: "APPROVED", missingID: "UPLOADING", badKeyID: "REJECTED", changedID: "REJECTED", invalidID: "REJECTED"} {
		var state string
		if err := pool.QueryRow(ctx, "SELECT state FROM "+pgx.Identifier{schema, "assets"}.Sanitize()+" WHERE id=$1", id).Scan(&state); err != nil || state != want {
			t.Fatalf("asset %s state = %q, %v", id, state, err)
		}
	}
	var variants []byte
	if err := pool.QueryRow(ctx, "SELECT variants FROM "+pgx.Identifier{schema, "asset_details_v2"}.Sanitize()+" WHERE asset_id=$1", validID).Scan(&variants); err != nil || !strings.Contains(string(variants), "approved/") {
		t.Fatalf("approved variants = %s, %v", variants, err)
	}
	outageID, outageBody := create(22, "QUARANTINED")
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(quarantine.Bucket), Key: aws.String("quarantine/" + outageID), Body: bytes.NewReader(outageBody), ContentType: aws.String("image/png")}); err != nil {
		t.Fatal(err)
	}
	outage := media
	outage.Scanner.Port = 1
	if err := scanPendingMedia(ctx, connection, outage); err == nil || !strings.Contains(err.Error(), "metadata_scanner_unavailable") {
		t.Fatalf("scanner outage = %v", err)
	}
	var state string
	if err := pool.QueryRow(ctx, "SELECT state FROM "+pgx.Identifier{schema, "assets"}.Sanitize()+" WHERE id=$1", outageID).Scan(&state); err != nil || state != "VALIDATING" {
		t.Fatalf("scanner outage state = %q, %v", state, err)
	}
	if err := scanPendingMedia(ctx, connection, metadata.ConnectedMedia{}); err == nil || !strings.Contains(err.Error(), "metadata_scan_store_unavailable") {
		t.Fatalf("unwired media inventory = %v", err)
	}
	missingQuarantineID, _ := create(23, "QUARANTINED")
	if err := scanPendingMedia(ctx, connection, media); err == nil || !strings.Contains(err.Error(), "metadata_quarantine_unavailable") {
		t.Fatalf("missing quarantine object = %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT state FROM "+pgx.Identifier{schema, "assets"}.Sanitize()+" WHERE id=$1", missingQuarantineID).Scan(&state); err != nil || state != "VALIDATING" {
		t.Fatalf("missing quarantine state = %q, %v", state, err)
	}
	workerID, workerBody := create(24, "QUARANTINED")
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(quarantine.Bucket), Key: aws.String("quarantine/" + workerID), Body: bytes.NewReader(workerBody), ContentType: aws.String("image/png")}); err != nil {
		t.Fatal(err)
	}
	missingWorker := media
	missingWorker.Worker = raster.ProcessWorker{}
	if err := scanPendingMedia(ctx, connection, missingWorker); !errors.Is(err, raster.ErrUnavailable) {
		t.Fatalf("unavailable raster worker = %v", err)
	}
	approvedID, approvedBody := create(25, "APPROVED")
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(quarantine.Bucket), Key: aws.String("quarantine/" + approvedID), Body: bytes.NewReader(approvedBody), ContentType: aws.String("image/png")}); err != nil {
		t.Fatal(err)
	}
	if err := scanPendingMedia(ctx, connection, media); err != nil {
		t.Fatalf("approved image variant recovery: %v", err)
	}
	var recovered []byte
	if err := pool.QueryRow(ctx, "SELECT variants FROM "+pgx.Identifier{schema, "asset_details_v2"}.Sanitize()+" WHERE asset_id=$1", approvedID).Scan(&recovered); err != nil || !strings.Contains(string(recovered), "approved/") {
		t.Fatalf("recovered approved variants = %s, %v", recovered, err)
	}
	moderationID, moderationBody := create(26, "QUARANTINED")
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(quarantine.Bucket), Key: aws.String("quarantine/" + moderationID), Body: bytes.NewReader(moderationBody), ContentType: aws.String("image/png")}); err != nil {
		t.Fatal(err)
	}
	missingModeration := media
	missingModeration.Moderation = metadata.PostgresVisibilityAuthority{Pool: pool, Schema: "missing_visibility"}
	if err := scanPendingMedia(ctx, connection, missingModeration); err == nil || !strings.Contains(err.Error(), "metadata_scan_moderation_unavailable") {
		t.Fatalf("unavailable visibility authority = %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT state FROM "+pgx.Identifier{schema, "assets"}.Sanitize()+" WHERE id=$1", moderationID).Scan(&state); err != nil || state != "QUARANTINED" {
		t.Fatalf("moderation outage state = %q, %v", state, err)
	}
	lockedID, lockedBody := create(27, "QUARANTINED")
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(quarantine.Bucket), Key: aws.String("quarantine/" + lockedID), Body: bytes.NewReader(lockedBody), ContentType: aws.String("image/png")}); err != nil {
		t.Fatal(err)
	}
	other, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	lockKey := schema + ":" + lockedID
	if _, err := other.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1,0))", lockKey); err != nil {
		t.Fatal(err)
	}
	if err := scanPendingMedia(ctx, connection, media); err != nil {
		t.Fatalf("scan with an active asset worker: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT state FROM "+pgx.Identifier{schema, "assets"}.Sanitize()+" WHERE id=$1", lockedID).Scan(&state); err != nil || state != "QUARANTINED" {
		t.Fatalf("locked asset state = %q, %v", state, err)
	}
	if _, err := other.Exec(ctx, "SELECT pg_advisory_unlock(hashtextextended($1,0))", lockKey); err != nil {
		t.Fatal(err)
	}
	other.Release()
	if err := scanPendingMedia(ctx, connection, media); err != nil {
		t.Fatalf("released asset scan: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT state FROM "+pgx.Identifier{schema, "assets"}.Sanitize()+" WHERE id=$1", lockedID).Scan(&state); err != nil || state != "APPROVED" {
		t.Fatalf("released asset state = %q, %v", state, err)
	}
	uploadingID, uploadingBody := create(29, "UPLOADING")
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(quarantine.Bucket), Key: aws.String("quarantine/" + uploadingID), Body: bytes.NewReader(uploadingBody), ContentType: aws.String("image/png")}); err != nil {
		t.Fatal(err)
	}
	if err := scanPendingMedia(ctx, connection, media); err != nil {
		t.Fatalf("uploaded object recovery: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT state FROM "+pgx.Identifier{schema, "assets"}.Sanitize()+" WHERE id=$1", uploadingID).Scan(&state); err != nil || state != "APPROVED" {
		t.Fatalf("recovered upload state = %q, %v", state, err)
	}
	oversizedID, _ := create(30, "QUARANTINED")
	oversizedBody := bytes.Repeat([]byte("x"), raster.MaxBytes+1)
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(quarantine.Bucket), Key: aws.String("quarantine/" + oversizedID), Body: bytes.NewReader(oversizedBody), ContentType: aws.String("image/png")}); err != nil {
		t.Fatal(err)
	}
	if err := scanPendingMedia(ctx, connection, media); err != nil {
		t.Fatalf("oversized object rejection: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT state FROM "+pgx.Identifier{schema, "assets"}.Sanitize()+" WHERE id=$1", oversizedID).Scan(&state); err != nil || state != "REJECTED" {
		t.Fatalf("oversized object state = %q, %v", state, err)
	}
	if _, err := pool.Exec(ctx, "CREATE SCHEMA scan_visibility; CREATE TABLE scan_visibility.safety_visibility(target_type text,target_id text,status text,revision bigint); CREATE VIEW scan_visibility.safety_visibility_v2 AS SELECT target_type,target_id,status,revision FROM scan_visibility.safety_visibility"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO scan_visibility.safety_visibility VALUES('token',$1,'removed',1)", poolAddress); err != nil {
		t.Fatal(err)
	}
	removedID, removedBody := create(28, "QUARANTINED")
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(quarantine.Bucket), Key: aws.String("quarantine/" + removedID), Body: bytes.NewReader(removedBody), ContentType: aws.String("image/png")}); err != nil {
		t.Fatal(err)
	}
	removed := media
	removed.Moderation = metadata.PostgresVisibilityAuthority{Pool: pool, Schema: "scan_visibility"}
	if err := scanPendingMedia(ctx, connection, removed); err != nil {
		t.Fatalf("removed asset scan: %v", err)
	}
	var blocked bool
	if err := pool.QueryRow(ctx, "SELECT a.state,d.blocked FROM "+pgx.Identifier{schema, "assets"}.Sanitize()+" a JOIN "+pgx.Identifier{schema, "documents"}.Sanitize()+" d ON a.pool=d.pool WHERE a.id=$1", removedID).Scan(&state, &blocked); err != nil || state != "QUARANTINED" || !blocked {
		t.Fatalf("removed asset state = %q, blocked=%v, %v", state, blocked, err)
	}
}
