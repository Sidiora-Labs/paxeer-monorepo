// Package enrolmenttest supplies a private PostgreSQL server for operator cases.
package enrolmenttest

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// Start returns an administrator URL. A configured coordinator server is reused.
func Start(t testing.TB) string {
	t.Helper()
	if dsn := os.Getenv("COORDINATOR_TEST_POSTGRES_DSN"); dsn != "" {
		return dsn
	}
	dir, err := os.MkdirTemp("", "operator-postgres-")
	if err != nil {
		t.Fatal("create PostgreSQL directory")
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error("remove PostgreSQL directory")
		}
	})
	initdb, err := exec.LookPath("initdb")
	if err != nil {
		paths, globErr := filepath.Glob("/usr/lib/postgresql/*/bin/initdb")
		if globErr != nil || len(paths) == 0 {
			t.Fatal("PostgreSQL server binaries required")
		}
		initdb = paths[len(paths)-1]
	}
	pgctl := filepath.Join(filepath.Dir(initdb), "pg_ctl")
	var credential *syscall.Credential
	if os.Geteuid() == 0 {
		account, err := user.Lookup("postgres")
		if err != nil {
			t.Fatal("unprivileged PostgreSQL account required")
		}
		uid, err := strconv.ParseUint(account.Uid, 10, 32)
		if err != nil {
			t.Fatal("invalid PostgreSQL account")
		}
		gid, err := strconv.ParseUint(account.Gid, 10, 32)
		if err != nil {
			t.Fatal("invalid PostgreSQL group")
		}
		credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		if err := os.Chown(dir, int(uid), int(gid)); err != nil {
			t.Fatal("own PostgreSQL directory")
		}
	}
	run := func(binary string, args ...string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...) //nolint:gosec // PostgreSQL binaries are resolved locally and arguments select the private fixture.
		cmd.Dir = dir
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("PostgreSQL %s failed", filepath.Base(binary))
		}
		return nil
	}
	password := rand.Text()
	passwordFile := filepath.Join(dir, "password")
	if err := os.WriteFile(passwordFile, []byte(password+"\n"), 0600); err != nil {
		t.Fatal("write PostgreSQL password")
	}
	if credential != nil {
		if err := os.Chown(passwordFile, int(credential.Uid), int(credential.Gid)); err != nil {
			t.Fatal("own PostgreSQL password file")
		}
	}
	data := filepath.Join(dir, "data")
	if err := run(initdb, "-D", data, "-U", "operator_fixture", "--pwfile="+passwordFile, "--auth=scram-sha-256", "--no-locale", "--encoding=UTF8"); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal("reserve PostgreSQL port")
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal("release PostgreSQL port")
	}
	options := fmt.Sprintf("-h 127.0.0.1 -p %d -k %s", port, dir)
	started := false
	t.Cleanup(func() {
		if !started {
			if err := run(pgctl, "-D", data, "status"); err != nil {
				return
			}
		}
		if err := run(pgctl, "-D", data, "-m", "immediate", "-w", "-t", "15", "stop"); err != nil {
			t.Error(err)
		}
	})
	if err := run(pgctl, "-D", data, "-l", filepath.Join(dir, "postgres.log"), "-o", options, "-w", "-t", "15", "start"); err != nil {
		t.Fatal(err)
	}
	started = true
	dsn := url.URL{Scheme: "postgres", User: url.UserPassword("operator_fixture", password), Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), Path: "/postgres", RawQuery: "sslmode=disable"}
	return dsn.String()
}
