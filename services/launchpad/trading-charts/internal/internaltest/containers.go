// Package internaltest provides real infrastructure (Postgres, Redis) for the
// trading-charts integration tests — never fakes (house rule no_stub) and never
// containers: the tests run against already running servers named by
// environment variables.
//
//   - PostgresEnv is a DSN whose role may CREATE DATABASE. Each call gets a
//     throwaway database with the trading-charts goose migrations applied (the
//     migrations create the fixed candles schema, so a schema per test would
//     collide); it is dropped WITH (FORCE) on cleanup.
//   - RedisEnv is a redis:// URL naming the logical database reserved for these
//     tests. It is flushed before and after each test; a file lock serializes
//     access across package test processes.
//
// It is imported only by *_test.go files, so it adds nothing to the production
// binary.
package internaltest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	chartcleanup "github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/cleanup"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gofrs/flock"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/migrate"
)

const (
	PostgresEnv = "CHARTS_TEST_POSTGRES_URL"
	RedisEnv    = "CHARTS_TEST_REDIS_URL"
)

type redisFixture struct {
	once sync.Once
	uri  string
}

var redisFixtures sync.Map

// NewDatabase creates a throwaway database on the PostgresEnv server and
// returns its DSN. The database is dropped via t.Cleanup.
func NewDatabase(t *testing.T) string {
	t.Helper()
	admin := os.Getenv(PostgresEnv)
	if admin == "" {
		t.Fatalf("%s must name a PostgreSQL server whose role may CREATE DATABASE", PostgresEnv)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	name := "charts_test_" + hex.EncodeToString(suffix)
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("connect %s: %v", PostgresEnv, err)
	}
	if _, err = conn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		_ = conn.Close(ctx)
		t.Fatalf("create throwaway database: %v", err)
	}
	_ = conn.Close(ctx)
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		c, err := pgx.Connect(cleanup, admin)
		if err != nil {
			t.Errorf("drop %s: %v", name, err)
			return
		}
		defer chartcleanup.CloseContext(cleanup, c)
		if _, err = c.Exec(cleanup, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop %s: %v", name, err)
		}
	})
	u, err := url.Parse(admin)
	if err != nil {
		t.Fatalf("parse %s: %v", PostgresEnv, err)
	}
	u.Path = "/" + name
	return u.String()
}

// OpenPool opens a bounded pgx pool on dsn, closed via t.Cleanup.
func OpenPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	cfg.MaxConns = 8
	cfg.MaxConnIdleTime = 30 * time.Second
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// NewPostgres creates a throwaway database, applies the trading-charts goose
// migrations (the candles schema), and returns its DSN plus a ready pool.
func NewPostgres(t *testing.T) (string, *pgxpool.Pool) {
	t.Helper()
	dsn := NewDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := migrate.Up(ctx, dsn); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	return dsn, OpenPool(t, dsn)
}

// NewRedisURL returns the RedisEnv URL after flushing its logical database;
// the database is flushed again on cleanup.
func NewRedisURL(t *testing.T) string {
	t.Helper()
	value, _ := redisFixtures.LoadOrStore(t, &redisFixture{})
	fixture := value.(*redisFixture)
	fixture.once.Do(func() {
		fixture.uri = newRedisURL(t)
		t.Cleanup(func() { redisFixtures.Delete(t) })
	})
	return fixture.uri
}

func newRedisURL(t *testing.T) string {
	t.Helper()
	uri := os.Getenv(RedisEnv)
	if uri == "" {
		t.Fatalf("%s must name the redis:// logical database reserved for these tests", RedisEnv)
	}
	opt, err := goredis.ParseURL(uri)
	if err != nil {
		t.Fatalf("parse %s: %v", RedisEnv, err)
	}
	identity := fmt.Sprintf("%s/%d", opt.Addr, opt.DB)
	lock := flock.New(filepath.Join(os.TempDir(), fmt.Sprintf("trading-charts-redis-%x.lock", sha256.Sum256([]byte(identity)))))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	locked, err := lock.TryLockContext(ctx, 10*time.Millisecond)
	if err != nil || !locked {
		t.Fatalf("lock Redis test database: acquired=%t: %v", locked, err)
	}
	t.Cleanup(func() {
		if err := lock.Unlock(); err != nil {
			t.Errorf("unlock Redis test database: %v", err)
		}
	})
	flush := func() {
		client := goredis.NewClient(opt)
		defer chartcleanup.Close(client)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.FlushDB(ctx).Err(); err != nil {
			t.Errorf("flush %s: %v", RedisEnv, err)
		}
	}
	flush()
	t.Cleanup(flush)
	return uri
}

// NewObjectStore starts an owned S3-compatible server and creates a private bucket.
func NewObjectStore(t *testing.T) *storage.Client {
	client, _ := NewObjectStoreWithConfig(t)
	return client
}

// NewObjectStoreWithConfig returns the owned provider and its connection settings.
func NewObjectStoreWithConfig(t *testing.T) (*storage.Client, storage.Config) {
	t.Helper()
	binary, err := exec.LookPath("minio")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	userBytes := make([]byte, 12)
	passwordBytes := make([]byte, 24)
	if _, err = rand.Read(userBytes); err != nil {
		t.Fatal(err)
	}
	if _, err = rand.Read(passwordBytes); err != nil {
		t.Fatal(err)
	}
	user, password := hex.EncodeToString(userBytes), hex.EncodeToString(passwordBytes)
	address := net.JoinHostPort(net.IPv4(127, 0, 0, 1).String(), strconv.Itoa(port))
	endpoint := "http://" + address
	data := t.TempDir()
	output, err := os.CreateTemp(data, "server-*.log")
	if err != nil {
		t.Fatal(err)
	}
	process, err := os.StartProcess(binary, []string{"minio", "server", "--quiet", "--address", address, filepath.Join(data, "objects")}, &os.ProcAttr{Env: append(os.Environ(), "MINIO_ROOT_USER="+user, "MINIO_ROOT_PASSWORD="+password, "MINIO_BROWSER=off"), Files: []*os.File{os.Stdin, output, output}})
	if err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := process.Signal(syscall.SIGINT); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Error(err)
		}
		state, err := process.Wait()
		if err != nil || !state.Success() {
			t.Error(err)
		}
		_ = output.Close()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for {
		connection, dialErr := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if dialErr == nil {
			_ = connection.Close()
			break
		}
		if ctx.Err() != nil {
			t.Fatalf("object server readiness: %v", ctx.Err())
		}
		time.Sleep(20 * time.Millisecond)
	}
	config, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("us-east-1"), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(user, password, "")))
	if err != nil {
		t.Fatal(err)
	}
	raw := s3.NewFromConfig(config, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
	bucket := "charts-staging-test-" + hex.EncodeToString(userBytes)
	if _, err = raw.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal(err)
	}
	return storage.NewFromS3(raw, bucket), storage.Config{Endpoint: endpoint, Region: "us-east-1", Bucket: bucket, AccessKeyID: user, SecretAccessKey: password}
}
