package policy

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func localSecurityPostgres(t *testing.T) string {
	t.Helper()
	initdb, err := exec.LookPath("initdb")
	if err != nil {
		matches, globErr := filepath.Glob("/usr/lib/postgresql/*/bin/initdb")
		if globErr != nil || len(matches) == 0 {
			t.Fatalf("PostgreSQL initdb is required: %v", err)
		}
		initdb = matches[len(matches)-1]
	}
	directory, err := os.MkdirTemp("/tmp", "security-postgres-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	var credential *syscall.Credential
	if os.Geteuid() == 0 {
		account, err := user.Lookup("postgres")
		if err != nil {
			t.Fatal(err)
		}
		uid, err := strconv.ParseUint(account.Uid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		gid, err := strconv.ParseUint(account.Gid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		if err := os.Chown(directory, int(uid), int(gid)); err != nil {
			t.Fatal(err)
		}
	}
	run := func(binary string, args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, binary, args...)
		command.Dir = directory
		command.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
		if output, err := command.CombinedOutput(); err != nil {
			log, _ := os.ReadFile(filepath.Join(directory, "postgres.log"))
			t.Fatalf("PostgreSQL %s: %v\n%s\n%s", filepath.Base(binary), err, output, log)
		}
	}
	data := filepath.Join(directory, "data")
	run(initdb, "-D", data, "-U", "postgres", "--auth=trust", "--no-locale", "--encoding=UTF8")
	pgctl := filepath.Join(filepath.Dir(initdb), "pg_ctl")
	run(pgctl, "-D", data, "-l", filepath.Join(directory, "postgres.log"), "-o", "-h '' -p 5432 -k "+directory, "-w", "-t", "20", "start")
	t.Cleanup(func() { run(pgctl, "-D", data, "-m", "fast", "-w", "-t", "20", "stop") })
	t.Log("started isolated PostgreSQL fixture")
	return "host=" + directory + " port=5432 user=postgres dbname=postgres sslmode=disable"
}

func localSecurityRedis(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	socket := filepath.Join(directory, "redis.sock")
	log, err := os.Create(filepath.Join(directory, "redis.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := log.Close(); err != nil {
			t.Error(err)
		}
	})
	command := exec.Command("redis-server", "--port", "0", "--unixsocket", socket, "--unixsocketperm", "700", "--dir", directory, "--save", "", "--appendonly", "no")
	command.Stdout = log
	command.Stderr = log
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Error(err)
		}
		if err := command.Wait(); err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ProcessState.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
				t.Error(err)
			}
		}
	})
	client := redis.NewClient(&redis.Options{Network: "unix", Addr: socket, DialTimeout: 100 * time.Millisecond, MaxRetries: -1})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for client.Ping(ctx).Err() != nil {
		select {
		case <-ctx.Done():
			output, _ := os.ReadFile(log.Name())
			t.Fatalf("Redis startup: %v\n%s", ctx.Err(), output)
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Log("started isolated Redis fixture")
	return socket
}
