package control

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
)

func TestMain(m *testing.M) {
	if os.Getenv("L47_REPAIR65_WORKER") == "1" {
		fmt.Fprintln(os.Stderr, repair65RunWorker())
		os.Exit(1)
	}
	os.Exit(runControlTests(m))
}

var controlFixturesOnce sync.Once
var controlFixtureError error
var controlFixtureStops []func() error

func runControlTests(m *testing.M) (code int) {
	code = m.Run()
	for n := len(controlFixtureStops) - 1; n >= 0; n-- {
		if err := controlFixtureStops[n](); err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
		}
	}
	return code
}

// Start native fixtures only when a test actually consumes them. Every legacy
// integration test retains its real database/chain assertions and fails if its
// fixture cannot start; focused local-store tests require neither binary.
func ensureControlFixtures(t *testing.T) {
	t.Helper()
	controlFixturesOnce.Do(func() {
		dir, err := os.MkdirTemp("", "control-fixtures-")
		if err != nil {
			controlFixtureError = err
			return
		}
		controlFixtureStops = append(controlFixtureStops, func() error { return os.RemoveAll(dir) })
		stop, err := startControlPostgres(dir)
		if err != nil {
			controlFixtureError = err
			return
		}
		controlFixtureStops = append(controlFixtureStops, stop)
		if os.Getenv("L47_ANVIL_URL") == "" {
			stop, err = startControlAnvil(dir)
			if err != nil {
				controlFixtureError = err
				return
			}
			controlFixtureStops = append(controlFixtureStops, stop)
		}
	})
	if controlFixtureError != nil {
		t.Fatal("real control fixtures:", controlFixtureError)
	}
}

func controlPostgresBinary(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	paths, err := filepath.Glob(filepath.Join("/usr/lib/postgresql", "*", "bin", name))
	if err != nil || len(paths) == 0 {
		return "", fmt.Errorf("%s is required; install PostgreSQL server binaries", name)
	}
	return paths[len(paths)-1], nil
}

func startControlPostgres(dir string) (func() error, error) {
	initdb, err := controlPostgresBinary("initdb")
	if err != nil {
		return nil, err
	}
	pgctl := filepath.Join(filepath.Dir(initdb), "pg_ctl")
	var credential *syscall.Credential
	if os.Geteuid() == 0 {
		account, err := user.Lookup("postgres")
		if err != nil {
			return nil, fmt.Errorf("PostgreSQL requires an unprivileged postgres account: %w", err)
		}
		uid, err := strconv.ParseUint(account.Uid, 10, 32)
		if err != nil {
			return nil, err
		}
		gid, err := strconv.ParseUint(account.Gid, 10, 32)
		if err != nil {
			return nil, err
		}
		credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		if err := os.Chown(dir, int(uid), int(gid)); err != nil {
			return nil, err
		}
	}
	run := func(binary string, args ...string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = dir
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w\n%s", filepath.Base(binary), err, output)
		}
		return nil
	}
	password := filepath.Join(dir, "password")
	if err := os.WriteFile(password, []byte("kindle_fixture_password\n"), 0600); err != nil {
		return nil, err
	}
	if credential != nil {
		if err := os.Chown(password, int(credential.Uid), int(credential.Gid)); err != nil {
			return nil, err
		}
	}
	data := filepath.Join(dir, "data")
	if err := run(initdb, "-D", data, "-U", "kindle_fixture", "--pwfile="+password, "--auth=scram-sha-256", "--no-locale", "--encoding=UTF8"); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		return nil, err
	}
	logPath := filepath.Join(dir, "postgres.log")
	options := fmt.Sprintf("-h 127.0.0.1 -p %d -k %s", port, dir)
	if err := run(pgctl, "-D", data, "-l", logPath, "-o", options, "-w", "-t", "15", "start"); err != nil {
		log, _ := os.ReadFile(logPath)
		return nil, fmt.Errorf("%w\n%s", err, log)
	}
	fixtureDSN = fmt.Sprintf("postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:%d/postgres?sslmode=disable", port)
	return func() error {
		return run(pgctl, "-D", data, "-m", "immediate", "-w", "-t", "15", "stop")
	}, nil
}

func startControlAnvil(dir string) (func() error, error) {
	logPath := filepath.Join(dir, "anvil.log")
	log, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	defer log.Close()
	ipc := filepath.Join(dir, "anvil.ipc")
	cmd := exec.Command("anvil", "--host", "127.0.0.1", "--port", "0", "--chain-id", "125", "--ipc", ipc, "--silent")
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stop := func() error {
		if err := cmd.Process.Kill(); err != nil {
			return err
		}
		err := <-done
		if exit, ok := err.(*exec.ExitError); ok && exit.Sys().(syscall.WaitStatus).Signal() == syscall.SIGKILL {
			return nil
		}
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			output, _ := os.ReadFile(logPath)
			return nil, fmt.Errorf("Anvil exited before readiness: %v\n%s", err, output)
		case <-ctx.Done():
			_ = stop()
			output, _ := os.ReadFile(logPath)
			return nil, fmt.Errorf("Anvil readiness: %w\n%s", ctx.Err(), output)
		case <-ticker.C:
			client, err := ethclient.DialContext(ctx, ipc)
			if err != nil {
				continue
			}
			chainID, err := client.ChainID(ctx)
			client.Close()
			if err != nil {
				continue
			}
			if chainID.Cmp(big.NewInt(125)) != 0 {
				_ = stop()
				return nil, fmt.Errorf("Anvil chain ID = %s, want 125", chainID)
			}
			if err := os.Setenv("L47_ANVIL_URL", ipc); err != nil {
				_ = stop()
				return nil, err
			}
			return stop, nil
		}
	}
}
