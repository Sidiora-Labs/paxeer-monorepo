package contract_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-chi/chi/v5"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/user"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
)

func randomHex(t *testing.T) string {
	t.Helper()
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(value)
}

func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func startProcess(t *testing.T, command *exec.Cmd, ready func() bool) {
	t.Helper()
	command.Stdout, command.Stderr = os.Stderr, os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("dependency did not become ready")
}

func startObjectStore(t *testing.T) {
	t.Helper()
	address := freeAddress(t)
	access, secret := randomHex(t), randomHex(t)
	command := exec.Command("minio", "server", t.TempDir(), "--address", address, "--console-address", freeAddress(t))
	command.Env = append(os.Environ(), "MINIO_ROOT_USER="+access, "MINIO_ROOT_PASSWORD="+secret)
	startProcess(t, command, func() bool {
		client := http.Client{Timeout: time.Second}
		response, err := client.Get("http://" + address + "/minio/health/ready")
		if err != nil {
			return false
		}
		_ = response.Body.Close()
		return response.StatusCode == http.StatusOK
	})
	t.Setenv("USER_TEST_MINIO_ENDPOINT", "http://"+address)
	t.Setenv("USER_TEST_MINIO_ACCESS_KEY", access)
	t.Setenv("USER_TEST_MINIO_SECRET_KEY", secret)
}

func startScanner(t *testing.T) (string, int) {
	t.Helper()
	directory := t.TempDir()
	// ClamAV loads the standard antivirus test signature for the isolated scan database.
	signature := "UsersSDKTest:0:*:" + hex.EncodeToString([]byte(`X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`)) + "\n"
	if err := os.WriteFile(filepath.Join(directory, "test.ndb"), []byte(signature), 0600); err != nil {
		t.Fatal(err)
	}
	address := freeAddress(t)
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf("Foreground yes\nTCPAddr %s\nTCPSocket %s\nDatabaseDirectory %s\nLogFile %s\nPidFile %s\n", host, port, directory, filepath.Join(directory, "clamd.log"), filepath.Join(directory, "clamd.pid"))
	path := filepath.Join(directory, "clamd.conf")
	if err := os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	startProcess(t, exec.Command("clamd", "--config-file="+path), func() bool {
		connection, err := net.DialTimeout("tcp", address, time.Second)
		if err != nil {
			return false
		}
		_ = connection.Close()
		return true
	})
	var number int
	if _, err := fmt.Sscan(port, &number); err != nil {
		t.Fatal(err)
	}
	return host, number
}

func TestUsersSDKService(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	options, err := goredis.ParseURL(os.Getenv("USER_TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	redis := goredis.NewClient(options)
	t.Cleanup(func() { _ = redis.Close() })
	if err := redis.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	startObjectStore(t)
	store := internaltest.NewStore(t, "users-sdk-images")
	host, port := startScanner(t)
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	poolAddress, tokenAddress := "0x"+strings.Repeat("b", 40), "0x"+strings.Repeat("c", 40)
	internaltest.SeedPool(t, pool, poolAddress, tokenAddress, wallet, 1700000000)
	deps := user.Deps{
		Queries: sqlcdb.New(pool), Redis: redis, Store: store,
		Scan: func(ctx context.Context, data []byte) sharedhttp.ScanResult {
			return sharedhttp.ScanBuffer(ctx, data, sharedhttp.ScanOptions{Host: host, Port: port})
		},
	}
	for _, stage := range []string{"write", "read"} {
		router := chi.NewRouter()
		handler := chi.NewRouter()
		user.New(deps).RegisterRoutes(handler)
		router.Mount("/", handler)
		router.Mount("/media&user", handler)
		server := httptest.NewServer(router)
		command := exec.Command("node", os.Getenv("USERS_SDK_CONSUMER"))
		command.Env = append(os.Environ(), "USERS_SDK_BASE_URL="+server.URL, "USERS_SDK_STAGE="+stage, "USERS_SDK_PRIVATE_KEY=0x"+hex.EncodeToString(crypto.FromECDSA(key)))
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		err := command.Run()
		server.Close()
		if err != nil {
			t.Fatalf("%s consumer: %v", stage, err)
		}
	}
	rows, err := pool.Query(context.Background(), "SELECT storage_key FROM users.user_images WHERE wallet_address=$1", wallet)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Fatal(err)
		}
		object, err := store.Get(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(object.Body)
		_ = object.Body.Close()
		if err != nil || len(data) == 0 {
			t.Fatalf("stored image: %v, %d bytes", err, len(data))
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("stored images: got %d, want 2", count)
	}
}
