package util_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
)

// These integration tests use the supported local OpenSSH/rsync receiver shared
// with the durable-backup tests. The focused repair gate uses loopback protocol
// fixtures in repair_m1164_test.go and requires no installed executables.
func TestSSHSession_NewSSHSession(t *testing.T) {
	receiver := repair114NewReceiver(t)
	session := receiver.connect(t)
	require.NoError(t, session.Close())
	_, err := util.NewSSHSession(slog.Default(), receiver.username, "127.0.0.1", receiver.port, "/nonexistent/key", receiver.knownHosts, false)
	require.ErrorContains(t, err, "read private key")
	_, err = util.NewSSHSession(slog.Default(), "wronguser", "127.0.0.1", receiver.port, receiver.key, receiver.knownHosts, false)
	require.Error(t, err)
	_, err = util.NewSSHSession(slog.Default(), receiver.username, "127.0.0.1", receiver.port, receiver.key, "", false)
	require.ErrorContains(t, err, "known-hosts trust source")
}

func TestSSHSession_Mkdirs(t *testing.T) {
	receiver := repair114NewReceiver(t)
	session := receiver.connect(t)
	defer session.Close()
	directory := filepath.Join(receiver.root, "space 'quote\" $(literal)\nroot", "nested")
	require.NoError(t, session.Mkdirs(directory))
	require.DirExists(t, directory)
	require.NoError(t, session.Mkdirs(directory))
	file := filepath.Join(receiver.root, "ordinary-file")
	require.NoError(t, os.WriteFile(file, []byte("file"), 0600))
	require.Error(t, session.Mkdirs(file))
	require.FileExists(t, file)
}

func TestSSHSession_FindFiles(t *testing.T) {
	receiver := repair114NewReceiver(t)
	session := receiver.connect(t)
	defer session.Close()
	directory := filepath.Join(receiver.root, "search 'quote\" $(literal)\nroot")
	require.NoError(t, session.Mkdirs(directory))
	names := []string{"space name.txt", "single'quote.log", "line\nbreak.txt", "ignored.dat"}
	for _, name := range names {
		require.NoError(t, os.WriteFile(filepath.Join(directory, name), []byte(name), 0600))
	}
	found, err := session.FindFiles(directory, []string{".txt", ".log"})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{filepath.Join(directory, names[0]), filepath.Join(directory, names[1]), filepath.Join(directory, names[2])}, found)
	found, err = session.FindFiles(filepath.Join(receiver.root, "missing"), []string{".txt"})
	require.NoError(t, err)
	require.Empty(t, found)
}

func TestSSHSession_Rsync(t *testing.T) {
	receiver := repair114NewReceiver(t)
	session := receiver.connect(t)
	defer session.Close()
	directory := filepath.Join(receiver.root, "transfer 'quote\" $(literal)\nroot")
	require.NoError(t, session.Mkdirs(directory))
	source := filepath.Join(t.TempDir(), "source 'quote\" $(literal)\nfile")
	content := bytes.Repeat([]byte("transfer contents\n"), 4096)
	require.NoError(t, os.WriteFile(source, content, 0600))
	link := filepath.Join(filepath.Dir(source), "relative-link")
	require.NoError(t, os.Symlink(filepath.Base(source), link))
	destination := filepath.Join(directory, "-destination 'quote\" $(literal)\nfile")
	require.NoError(t, session.Rsync(link, destination, 0))
	received, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, content, received)
	throttled := filepath.Join(directory, "throttled")
	require.NoError(t, session.Rsync(source, throttled, 1))
	received, err = os.ReadFile(throttled)
	require.NoError(t, err)
	require.Equal(t, content, received)
	require.Error(t, session.Rsync(source, filepath.Join(receiver.root, "missing-parent", "file"), 0))
	require.FileExists(t, source)
	// Lose the receiver after rsync's success but before the durability barrier.
	marker := filepath.Join(receiver.root, "crash-before-ack")
	require.NoError(t, os.WriteFile(marker, []byte("interrupt"), 0600))
	require.ErrorContains(t, session.Rsync(source, destination, 0), "remote durable commit failed")
	require.FileExists(t, source)
	require.NoError(t, os.Remove(marker))
	receiver.stop()
	receiver.start(t)
	recovered := receiver.connect(t)
	defer recovered.Close()
	require.NoError(t, recovered.Rsync(source, destination, 0))
	received, err = os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, content, received)
}

func TestSSHSession_ChangedHostRejected(t *testing.T) {
	receiver := repair114NewReceiver(t)
	session := receiver.connect(t)
	defer session.Close()
	source := filepath.Join(t.TempDir(), "retained")
	require.NoError(t, os.WriteFile(source, []byte("private database contents"), 0600))
	receiver.stop()
	repair114WriteSSHKey(t, filepath.Join(receiver.root, "host-key"))
	receiver.start(t)
	_, err := util.NewSSHSession(slog.Default(), receiver.username, "127.0.0.1", receiver.port, receiver.key, receiver.knownHosts, false)
	require.ErrorContains(t, err, "key mismatch")
	require.ErrorContains(t, session.Rsync(source, filepath.Join(receiver.root, "must-not-exist"), 0), "failed to rsync data")
	require.NoFileExists(t, filepath.Join(receiver.root, "must-not-exist"))
	require.FileExists(t, source)
}

func TestSSHSession_CancelTransfer(t *testing.T) {
	receiver := repair114NewReceiver(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	session, err := util.NewSSHSessionContext(ctx, slog.Default(), receiver.username, "127.0.0.1", receiver.port, receiver.key, receiver.knownHosts, false)
	require.NoError(t, err)
	defer session.Close()
	source := filepath.Join(t.TempDir(), "large-source")
	require.NoError(t, os.WriteFile(source, bytes.Repeat([]byte("payload"), 1024*1024), 0600))
	destination := filepath.Join(receiver.root, "slow-transfer")
	done := make(chan error, 1)
	go func() { done <- session.RsyncContext(ctx, source, destination, 0.001) }()
	require.Eventually(t, func() bool {
		files, _ := filepath.Glob(filepath.Join(receiver.root, ".slow-transfer.*"))
		return len(files) > 0
	}, 5*time.Second, 10*time.Millisecond)
	cancel()
	require.NoError(t, session.Close())
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(3 * time.Second):
		t.Fatal("transfer did not join cancellation")
	}
	require.FileExists(t, source)
}

func TestSSHSession_TransferProgressTimeout(t *testing.T) {
	const inactivity = 3 * time.Second
	content := make([]byte, 8*1024*1024)
	_, err := rand.Read(content)
	require.NoError(t, err)
	source := filepath.Join(t.TempDir(), "incompressible-source")
	require.NoError(t, os.WriteFile(source, content, 0600))

	newReceiver := func(t *testing.T) *repair114Receiver {
		t.Helper()
		r := repair114NewReceiver(t)
		// Observe the real rsync and sync commands, and inject faults in their
		// processes without replacing either executable or the SSH transport.
		script := fmt.Sprintf(`#!/bin/sh
cd %s || exit 1
case "$SSH_ORIGINAL_COMMAND" in
  'rsync --server '*)
    printf '%%s\n' "$$" > transfer-pid
    exec /bin/sh -c "exec $SSH_ORIGINAL_COMMAND"
    ;;
  'sync -f -- '*)
    printf '%%s\n' "$$" > ack-started
    if [ -f crash-before-ack ]; then kill -KILL $$; fi
    if [ -f delay-before-ack ]; then kill -STOP $$; fi
    /bin/sh -c "$SSH_ORIGINAL_COMMAND" || exit 1
    printf complete > ack-complete
    exit 0
    ;;
esac
exec /bin/sh -c "$SSH_ORIGINAL_COMMAND"
`, util.QuoteRemoteArgument(r.root))
		require.NoError(t, os.WriteFile(filepath.Join(r.root, "receiver.sh"), []byte(script), 0700))
		return r
	}
	connect := func(t *testing.T, r *repair114Receiver, parent context.Context, legacy bool) *util.SSHSession {
		t.Helper()
		options := util.SSHTimeouts{Dial: 2 * time.Second, Command: 500 * time.Millisecond, Transfer: inactivity}
		var session *util.SSHSession
		var err error
		if legacy {
			session, err = util.NewSSHSession(slog.Default(), r.username, "127.0.0.1", r.port, r.key, r.knownHosts, false, options)
		} else {
			session, err = util.NewSSHSessionContext(parent, slog.Default(), r.username, "127.0.0.1", r.port, r.key, r.knownHosts, false, options)
		}
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, session.Close()) })
		return session
	}
	readPID := func(t *testing.T, path string) int {
		t.Helper()
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		require.NoError(t, err)
		require.Positive(t, pid)
		return pid
	}
	waitProgress := func(t *testing.T, r *repair114Receiver) {
		t.Helper()
		require.Eventually(t, func() bool {
			files, _ := filepath.Glob(filepath.Join(r.root, ".destination.*"))
			for _, file := range files {
				info, err := os.Stat(file)
				if err == nil && info.Size() > 0 {
					return true
				}
			}
			return false
		}, 5*time.Second, 10*time.Millisecond, "receiver must be writing actual file data")
	}
	waitResult := func(t *testing.T, done <-chan error, bound time.Duration) error {
		t.Helper()
		select {
		case err := <-done:
			return err
		case <-time.After(bound):
			t.Fatal("transfer did not terminate and join within its bound")
			return nil
		}
	}
	// Capture the local rsync process and its SSH child before cancellation so
	// returning an error alone cannot hide a still-running transfer subprocess.
	localProcesses := func(t *testing.T) (int, []string) {
		t.Helper()
		paths, err := filepath.Glob("/proc/[0-9]*/cmdline")
		require.NoError(t, err)
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			args := strings.Split(string(data), "\x00")
			if filepath.Base(args[0]) != "rsync" {
				continue
			}
			for _, arg := range args {
				if arg != source {
					continue
				}
				pid, err := strconv.Atoi(filepath.Base(filepath.Dir(path)))
				require.NoError(t, err)
				group, err := syscall.Getpgid(pid)
				require.NoError(t, err)
				require.Equal(t, pid, group, "rsync must own its process group")
				children, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/children", pid, pid))
				require.NoError(t, err)
				require.NotEmpty(t, strings.Fields(string(children)), "real SSH child must be active")
				return pid, strings.Fields(string(children))
			}
		}
		t.Fatal("active local rsync was not found")
		return 0, nil
	}
	assertJoined := func(t *testing.T, pid int, children []string) {
		t.Helper()
		_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
		require.True(t, os.IsNotExist(err), "rsync must be reaped before returning")
		require.Eventually(t, func() bool {
			for _, child := range children {
				data, err := os.ReadFile(filepath.Join("/proc", child, "stat"))
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					return false
				}
				state := strings.Fields(string(data)[strings.LastIndex(string(data), ")")+1:])[0]
				if state != "Z" && state != "X" {
					return false
				}
			}
			return true
		}, time.Second, 10*time.Millisecond, "SSH child must also terminate")
	}
	assertRetained := func(t *testing.T) {
		t.Helper()
		data, err := os.ReadFile(source)
		require.NoError(t, err)
		require.Equal(t, content, data, "failed transfers must retain the source bytes")
	}

	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("progress/legacy=%t", legacy), func(t *testing.T) {
			r := newReceiver(t)
			session := connect(t, r, t.Context(), legacy)
			// Random bytes prevent -z from bypassing the bandwidth limit.
			smallSource := filepath.Join(t.TempDir(), "source")
			payload := content[:1024*1024]
			require.NoError(t, os.WriteFile(smallSource, payload, 0600))
			destination := filepath.Join(r.root, "destination")
			start := time.Now()
			if legacy {
				err = session.Rsync(smallSource, destination, 0.2)
			} else {
				err = session.RsyncContext(t.Context(), smallSource, destination, 0.2)
			}
			require.NoError(t, err)
			require.Greater(t, time.Since(start), inactivity)
			data, err := os.ReadFile(destination)
			require.NoError(t, err)
			require.Equal(t, payload, data)
			require.FileExists(t, filepath.Join(r.root, "ack-complete"), "durability barrier must precede success")
		})
	}

	t.Run("stalled receiver", func(t *testing.T) {
		r := newReceiver(t)
		session := connect(t, r, t.Context(), true)
		done := make(chan error, 1)
		go func() { done <- session.Rsync(source, filepath.Join(r.root, "destination"), 0.25) }()
		waitProgress(t, r)
		pid, children := localProcesses(t)
		remotePID := readPID(t, filepath.Join(r.root, "transfer-pid"))
		group, err := syscall.Getpgid(remotePID)
		require.NoError(t, err)
		require.Equal(t, remotePID, group)
		t.Cleanup(func() { _ = syscall.Kill(-group, syscall.SIGKILL) })
		require.NoError(t, syscall.Kill(-group, syscall.SIGSTOP))
		err = waitResult(t, done, 15*time.Second)
		require.ErrorContains(t, err, "failed to rsync data")
		require.NoError(t, t.Context().Err(), "inactivity alone must stop the transfer")
		assertJoined(t, pid, children)
		require.NoFileExists(t, filepath.Join(r.root, "ack-started"))
		assertRetained(t)
	})

	for _, mode := range []string{"caller deadline", "caller cancellation", "session cancellation", "Close"} {
		t.Run(mode, func(t *testing.T) {
			r := newReceiver(t)
			parent, cancelParent := context.WithCancel(t.Context())
			defer cancelParent()
			session := connect(t, r, parent, false)
			caller, cancel := context.WithCancel(t.Context())
			if mode == "caller deadline" {
				cancel()
				caller, cancel = context.WithTimeout(t.Context(), 2*time.Second)
			}
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- session.RsyncContext(caller, source, filepath.Join(r.root, "destination"), 0.25) }()
			waitProgress(t, r)
			pid, children := localProcesses(t)
			switch mode {
			case "caller cancellation":
				cancel()
			case "session cancellation":
				cancelParent()
			case "Close":
				closed := make(chan error, 1)
				go func() { closed <- session.Close() }()
				require.NoError(t, waitResult(t, closed, 3*time.Second))
			}
			err := waitResult(t, done, 3*time.Second)
			if mode == "caller deadline" {
				require.ErrorIs(t, err, context.DeadlineExceeded)
			} else {
				require.ErrorIs(t, err, context.Canceled)
			}
			assertJoined(t, pid, children)
			require.NoFileExists(t, filepath.Join(r.root, "ack-started"))
			assertRetained(t)
		})
	}

	for _, fault := range []string{"delay-before-ack", "crash-before-ack"} {
		t.Run(fault, func(t *testing.T) {
			r := newReceiver(t)
			session := connect(t, r, t.Context(), true)
			require.NoError(t, os.WriteFile(filepath.Join(r.root, fault), []byte("fault"), 0600))
			destination := filepath.Join(r.root, "destination")
			done := make(chan error, 1)
			go func() { done <- session.Rsync(source, destination, 0) }()
			if fault == "delay-before-ack" {
				require.Eventually(t, func() bool {
					data, _ := os.ReadFile(filepath.Join(r.root, "ack-started"))
					_, err := strconv.Atoi(strings.TrimSpace(string(data)))
					return err == nil
				}, 5*time.Second, 10*time.Millisecond)
				pid := readPID(t, filepath.Join(r.root, "ack-started"))
				t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
			}
			err := waitResult(t, done, 3*time.Second)
			require.ErrorContains(t, err, "remote durable commit failed")
			if fault == "delay-before-ack" {
				require.ErrorIs(t, err, context.DeadlineExceeded, "acknowledgment has its own Command bound")
			}
			require.FileExists(t, filepath.Join(r.root, "ack-started"))
			require.NoFileExists(t, filepath.Join(r.root, "ack-complete"))
			data, err := os.ReadFile(destination)
			require.NoError(t, err)
			require.Equal(t, content, data, "file transfer must finish before the acknowledgment fault")
			assertRetained(t)
		})
	}
}
