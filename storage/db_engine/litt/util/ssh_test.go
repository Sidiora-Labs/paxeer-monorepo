package util_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
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
