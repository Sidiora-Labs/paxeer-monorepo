package voice

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

func recordingStorageFixture(t *testing.T) *storage.Client {
	t.Helper()
	binary, err := exec.LookPath("minio")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(net.IPv4(127, 0, 0, 1).String(), "0"))
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	access := "voice" + hex.EncodeToString(secret[:8])
	password := hex.EncodeToString(secret)
	log, err := os.CreateTemp(t.TempDir(), "storage-*.log")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	command := exec.Command(binary, "server", "--address", address, "--console-address", ":0", filepath.Join(t.TempDir(), "objects"))
	command.Env = append(os.Environ(), "MINIO_ROOT_USER="+access, "MINIO_ROOT_PASSWORD="+password)
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	endpoint := "http://" + address
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response, requestErr := http.Get(endpoint + "/minio/health/live") //nolint:gosec // The endpoint is the local storage fixture.
		if requestErr == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	ctx := context.Background()
	region := "us-east-1"
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(access, password, "")))
	if err != nil {
		t.Fatal(err)
	}
	raw := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
	bucket := "voice-" + uuid.NewString()
	if _, err := raw.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal(err)
	}
	client, err := storage.New(ctx, storage.Config{Endpoint: endpoint, Region: region, AccessKeyID: access, SecretAccessKey: password, Bucket: bucket})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestRecordingObjectRequiresConsentAndExpiresFromStorage(t *testing.T) {
	c, ctx := fixture(t)
	c.RecordingStorage = recordingStorageFixture(t)
	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	if _, err := c.DB.Exec(ctx, `INSERT INTO voice_recordings(id,room_id,state,purpose,policy_version,delete_at) VALUES($1,$2,'stopped','session archive','recording-v2',now()+interval '1 hour')`, id, room.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DB.Exec(ctx, `INSERT INTO voice_participants(room_id,wallet,role) VALUES($1,$2,'host')`, room.ID, host); err != nil {
		t.Fatal(err)
	}
	if _, err := c.OpenRecording(ctx, room.ID); err == nil {
		t.Fatal("recording opened without consent")
	}
	if _, err := c.DB.Exec(ctx, `INSERT INTO voice_consents(recording_id,wallet,agreed,policy_version,purpose,session_id) VALUES($1,$2,true,'recording-v2','session archive','voice-fixture')`, id, host); err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("restricted/voice/%s.ogg", id)
	want := []byte("recorded audio bytes")
	if err := c.RecordingStorage.Put(ctx, key, bytes.NewReader(want), int64(len(want)), "audio/ogg"); err != nil {
		t.Fatal(err)
	}
	object, err := c.OpenRecording(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(object.Body)
	_ = object.Body.Close()
	if err != nil || !bytes.Equal(got, want) || object.ContentType != "audio/ogg" {
		t.Fatalf("consented object = %q, %q: %v", got, object.ContentType, err)
	}
	if _, err := c.DB.Exec(ctx, `UPDATE voice_recordings SET delete_at=now()-interval '1 second' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	count, err := c.DeleteExpiredRecordings(ctx)
	if err != nil || count != 1 {
		t.Fatalf("expired recordings = %d: %v", count, err)
	}
	var deleted bool
	if err := c.DB.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM voice_recordings WHERE id=$1`, id).Scan(&deleted); err != nil || !deleted {
		t.Fatalf("recording tombstone = %t: %v", deleted, err)
	}
	if _, err := c.RecordingStorage.Get(ctx, key); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expired object = %v", err)
	}
	if count, err := c.DeleteExpiredRecordings(ctx); err != nil || count != 0 {
		t.Fatalf("repeated expiration = %d: %v", count, err)
	}
}
