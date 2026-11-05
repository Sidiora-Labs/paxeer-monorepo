package util_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/segment"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestRepair_114_LockProcess(t *testing.T) {
	directory := os.Getenv("LITT_REPAIR_114_LOCK_DIR")
	if directory == "" {
		return
	}
	var signal [1]byte
	if _, err := io.ReadFull(os.Stdin, signal[:]); err != nil {
		os.Exit(2)
	}
	release, err := util.LockDirectories(slog.Default(), []string{directory}, "writer.lock", true)
	if err != nil {
		fmt.Println("busy")
		os.Exit(0)
	}
	fmt.Println("acquired")
	_, _ = io.ReadFull(os.Stdin, signal[:])
	release()
	os.Exit(0)
}

type repair114LockProcess struct {
	cmd    *exec.Cmd
	input  io.WriteCloser
	result <-chan string
}

func repair114StartLockProcess(t *testing.T, directory string) repair114LockProcess {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRepair_114_LockProcess$")
	cmd.Env = append(os.Environ(), "LITT_REPAIR_114_LOCK_DIR="+directory)
	input, err := cmd.StdinPipe()
	require.NoError(t, err)
	output, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = input.Close()
		_ = cmd.Process.Kill()
	})
	result := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(output).ReadString('\n')
		result <- strings.TrimSpace(line)
	}()
	return repair114LockProcess{cmd: cmd, input: input, result: result}
}

func repair114LockResult(t *testing.T, process repair114LockProcess) string {
	t.Helper()
	select {
	case result := <-process.result:
		return result
	case <-time.After(15 * time.Second):
		t.Fatal("lock contender did not report its result")
		return ""
	}
}

func TestRepair_114_StaleLockTakeover(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "writer.lock")
	owner := repair114StartLockProcess(t, directory)
	_, err := owner.input.Write([]byte{1})
	require.NoError(t, err)
	require.Equal(t, "acquired", repair114LockResult(t, owner))
	guardBefore, err := os.Stat(path + ".guard")
	require.NoError(t, err)
	require.NoError(t, owner.cmd.Process.Kill())
	require.Error(t, owner.cmd.Wait())
	_, err = os.Stat(path)
	require.NoError(t, err, "crash must leave metadata for stale recovery")

	processes := make([]repair114LockProcess, 12)
	for i := range processes {
		processes[i] = repair114StartLockProcess(t, directory)
	}
	for _, process := range processes {
		_, err := process.input.Write([]byte{1})
		require.NoError(t, err)
	}
	winner := -1
	for i, process := range processes {
		result := repair114LockResult(t, process)
		switch result {
		case "acquired":
			require.Equal(t, -1, winner, "two writers acquired the stale lock")
			winner = i
		case "busy":
			require.NoError(t, process.cmd.Wait())
		default:
			t.Fatalf("unexpected contender result: %q", result)
		}
	}
	require.NotEqual(t, -1, winner)
	lock, err := util.NewFileLock(slog.Default(), path, false)
	require.Error(t, err)
	require.Nil(t, lock)
	require.NoError(t, processes[winner].cmd.Process.Kill())
	require.Error(t, processes[winner].cmd.Wait())

	restarted, err := util.NewFileLock(slog.Default(), path, true)
	require.NoError(t, err)
	t.Cleanup(restarted.Release)
	guardAfter, err := os.Stat(path + ".guard")
	require.NoError(t, err)
	require.True(t, os.SameFile(guardBefore, guardAfter), "guard inode must survive recovery")
	pidInfo, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(pidInfo), fmt.Sprintf("PID: %d\n", os.Getpid()))
	restarted.Release()
	successor, err := util.NewFileLock(slog.Default(), path, false)
	require.NoError(t, err)
	defer successor.Release()
	restarted.Release()
	contender, err := util.NewFileLock(slog.Default(), path, false)
	require.Error(t, err, "old release must not unlock the successor")
	require.Nil(t, contender)
	_, err = os.Stat(path)
	require.NoError(t, err, "old release must not unlink successor metadata")
}

func TestRepair_114_LockReleaseOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	lock, err := util.NewFileLock(slog.Default(), path, false)
	require.NoError(t, err)
	defer lock.Release()
	require.NoError(t, os.Rename(path, path+".old"))
	require.NoError(t, os.WriteFile(path, []byte("replacement metadata"), 0600))
	lock.Release()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "replacement metadata", string(data))
}

func TestRepair_114_MoveSymlinkContract(t *testing.T) {
	crossRoot, err := os.MkdirTemp("/dev/shm", "litt-repair-114-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(crossRoot)) })
	localRoot := t.TempDir()
	localInfo, err := os.Stat(localRoot)
	require.NoError(t, err)
	crossInfo, err := os.Stat(crossRoot)
	require.NoError(t, err)
	require.NotEqual(t, localInfo.Sys().(*syscall.Stat_t).Dev, crossInfo.Sys().(*syscall.Stat_t).Dev)
	for _, preserve := range []bool{false, true} {
		for _, cross := range []bool{false, true} {
			for _, kind := range []string{"root-file", "root-directory", "nested-file", "nested-directory", "dangling"} {
				t.Run(fmt.Sprintf("%s/preserve=%t/cross=%t", kind, preserve, cross), func(t *testing.T) {
					root, err := os.MkdirTemp(localRoot, "source-")
					require.NoError(t, err)
					outside := filepath.Join(root, "target")
					if strings.HasSuffix(kind, "directory") {
						require.NoError(t, os.Mkdir(outside, 0700))
						require.NoError(t, os.WriteFile(filepath.Join(outside, "payload"), []byte("unchanged"), 0600))
					} else if kind != "dangling" {
						require.NoError(t, os.WriteFile(outside, []byte("unchanged"), 0600))
					}
					source := filepath.Join(root, "source")
					link := source
					if strings.HasPrefix(kind, "nested") {
						require.NoError(t, os.Mkdir(source, 0700))
						require.NoError(t, os.WriteFile(filepath.Join(source, "aaa-first"), []byte("keep"), 0600))
						link = filepath.Join(source, "zzz-link")
					}
					require.NoError(t, os.Symlink(outside, link))
					destParent := root
					if cross {
						destParent, err = os.MkdirTemp(crossRoot, "destination-")
						require.NoError(t, err)
					}
					destination := filepath.Join(destParent, "destination")
					err = util.RecursiveMove(source, destination, preserve, true)
					require.ErrorContains(t, err, "symlink")
					linkInfo, err := os.Lstat(link)
					require.NoError(t, err)
					require.NotZero(t, linkInfo.Mode()&os.ModeSymlink)
					_, err = os.Lstat(destination)
					require.True(t, os.IsNotExist(err), "rejection must precede all destination mutation")
					if strings.HasPrefix(kind, "nested") {
						data, err := os.ReadFile(filepath.Join(source, "aaa-first"))
						require.NoError(t, err)
						require.Equal(t, "keep", string(data))
					}
					if kind != "dangling" {
						targetFile := outside
						if strings.HasSuffix(kind, "directory") {
							targetFile = filepath.Join(outside, "payload")
						}
						data, err := os.ReadFile(targetFile)
						require.NoError(t, err)
						require.Equal(t, "unchanged", string(data))
					}
				})
			}
			// Exercise both successful transfer paths as well as their rejection contract.
			t.Run(fmt.Sprintf("regular/preserve=%t/cross=%t", preserve, cross), func(t *testing.T) {
				sourceDir := t.TempDir()
				source := filepath.Join(sourceDir, "regular")
				require.NoError(t, os.WriteFile(source, []byte("payload"), 0600))
				destination := filepath.Join(sourceDir, "moved")
				if cross {
					destination = filepath.Join(crossRoot, fmt.Sprintf("regular-%t", preserve))
				}
				require.NoError(t, util.RecursiveMove(source, destination, preserve, true))
				data, err := os.ReadFile(destination)
				require.NoError(t, err)
				require.Equal(t, "payload", string(data))
				_, err = os.Stat(source)
				if preserve {
					require.NoError(t, err)
				} else {
					require.True(t, os.IsNotExist(err))
				}
			})
		}
	}
}

func repair114WriteSSHKey(t *testing.T, path string) ssh.PublicKey {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	encoded, err := ssh.MarshalPrivateKey(private, "repair-114")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(encoded), 0600))
	signer, err := ssh.NewSignerFromKey(private)
	require.NoError(t, err)
	return signer.PublicKey()
}

type repair114Receiver struct {
	root       string
	config     string
	key        string
	knownHosts string
	username   string
	port       uint64
	process    *exec.Cmd
	log        *os.File
}

func repair114NewReceiver(t *testing.T) *repair114Receiver {
	t.Helper()
	for _, binary := range []string{"rsync", "ssh", "sync"} {
		_, err := exec.LookPath(binary)
		require.NoError(t, err, "real backup integration requires %s", binary)
	}
	root := t.TempDir()
	hostKey := filepath.Join(root, "host-key")
	hostPublic := repair114WriteSSHKey(t, hostKey)
	key := filepath.Join(root, "client-key")
	clientPublic := repair114WriteSSHKey(t, key)
	authorized := filepath.Join(root, "authorized_keys")
	require.NoError(t, os.WriteFile(authorized, ssh.MarshalAuthorizedKey(clientPublic), 0600))
	currentUser, err := user.Current()
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	knownHosts := filepath.Join(root, "known_hosts")
	knownEntry := fmt.Sprintf("[127.0.0.1]:%d %s", port, ssh.MarshalAuthorizedKey(hostPublic))
	require.NoError(t, os.WriteFile(knownHosts, []byte(knownEntry), 0600))

	// Run the real receiver shell. The fault marker kills it after rsync has
	// finished but before the filesystem flush, modeling loss of the receiver
	// at the precise boundary where process success used to permit reclamation.
	wrapper := filepath.Join(root, "receiver.sh")
	script := fmt.Sprintf("#!/bin/sh\ncase \"$SSH_ORIGINAL_COMMAND\" in\n  'sync -f -- '*)\n    if [ -f '%s/crash-before-ack' ]; then kill -KILL $$; fi\n    ;;\nesac\nexec /bin/sh -c \"$SSH_ORIGINAL_COMMAND\"\n", root)
	require.NoError(t, os.WriteFile(wrapper, []byte(script), 0700))
	config := filepath.Join(root, "sshd_config")
	configText := fmt.Sprintf("ListenAddress 127.0.0.1\nPort %d\nHostKey %s\nPidFile %s/sshd.pid\nAuthorizedKeysFile %s\nStrictModes no\nUsePAM no\nPasswordAuthentication no\nKbdInteractiveAuthentication no\nPermitRootLogin yes\nForceCommand /bin/sh %s\n", port, hostKey, root, authorized, wrapper)
	require.NoError(t, os.WriteFile(config, []byte(configText), 0600))
	r := &repair114Receiver{root: root, config: config, key: key, knownHosts: knownHosts, username: currentUser.Username, port: uint64(port)}
	t.Cleanup(func() { r.stop() })
	r.start(t)
	return r
}

func (r *repair114Receiver) start(t *testing.T) {
	t.Helper()
	sshd, err := exec.LookPath("sshd")
	if err != nil {
		sshd = "/usr/sbin/sshd"
	}
	r.log, err = os.OpenFile(filepath.Join(r.root, "sshd.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	require.NoError(t, err)
	r.process = exec.CommandContext(t.Context(), sshd, "-D", "-e", "-f", r.config)
	r.process.Stdout = r.log
	r.process.Stderr = r.log
	require.NoError(t, r.process.Start())
	deadline := time.Now().Add(5 * time.Second)
	for {
		connection, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.FormatUint(r.port, 10)), 100*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return
		}
		if time.Now().After(deadline) {
			log, _ := os.ReadFile(filepath.Join(r.root, "sshd.log"))
			t.Fatalf("receiver failed to start: %v\n%s", err, log)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (r *repair114Receiver) stop() {
	if r.process != nil {
		_ = r.process.Process.Kill()
		_ = r.process.Wait()
		_ = r.log.Close()
		r.process = nil
	}
}

func (r *repair114Receiver) connect(t *testing.T) *util.SSHSession {
	t.Helper()
	session, err := util.NewSSHSession(slog.Default(), r.username, "127.0.0.1", r.port, r.key, r.knownHosts, false)
	require.NoError(t, err)
	return session
}

func TestRepair_114_BackupDurableAck(t *testing.T) {
	receiver := repair114NewReceiver(t)
	session := receiver.connect(t)
	defer func() { _ = session.Close() }()
	logger := slog.Default()
	monitor := util.NewErrorMonitor(t.Context(), logger, nil)
	sourceRoot := t.TempDir()
	snapshotRoot := t.TempDir()
	paths, err := segment.BuildSegmentPaths([]string{sourceRoot}, snapshotRoot, "table")
	require.NoError(t, err)
	require.NoError(t, paths[0].MakeDirectories(true))
	seg, err := segment.CreateSegment(logger, monitor, 0, paths, true, 1, true)
	require.NoError(t, err)
	key, value := []byte("backup-key"), bytes.Repeat([]byte("recoverable value"), 1024)
	_, _, err = seg.Write(&types.PutRequest{Key: key, Value: value})
	require.NoError(t, err)
	addresses, err := seg.Seal(time.Now())
	require.NoError(t, err)
	require.Len(t, addresses, 1)
	require.NoError(t, seg.Snapshot())
	names := make([]string, 0)
	for _, path := range seg.GetFilePaths() {
		names = append(names, filepath.Base(path))
	}
	seg.Release()
	require.NoError(t, seg.BlockUntilFullyDeleted(), "live GC must leave only snapshot hardlinks")
	remoteRoot := filepath.Join(receiver.root, "backup 'receiver")
	remotePaths, err := segment.BuildSegmentPaths([]string{remoteRoot}, "", "table")
	require.NoError(t, err)
	remoteDir := remotePaths[0].SegmentDirectory()
	// Mkdirs' existing quoting contract is outside this repair; create the
	// receiver directories directly before exercising Rsync's literal paths.
	require.NoError(t, os.MkdirAll(remoteDir, 0700))
	fault := filepath.Join(receiver.root, "crash-before-ack")
	require.NoError(t, os.WriteFile(fault, []byte("crash"), 0600))
	firstSource := filepath.Join(paths[0].SoftlinkPath(), names[0])
	err = session.Rsync(firstSource, filepath.Join(remoteDir, names[0]), 0)
	require.ErrorContains(t, err, "remote durable commit failed")
	for _, name := range names {
		_, err := os.Stat(filepath.Join(paths[0].SoftlinkPath(), name))
		require.NoError(t, err, "failed acknowledgment must retain snapshot data")
	}
	require.NoError(t, session.Close())
	receiver.stop()
	require.NoError(t, os.Remove(fault))
	receiver.start(t)
	session = receiver.connect(t)
	for _, name := range names {
		source := filepath.Join(paths[0].SoftlinkPath(), name)
		require.NoError(t, session.Rsync(source, filepath.Join(remoteDir, name), 1))
	}
	// Match the consumer's reclaim order: release the complete snapshot only
	// after every file has obtained its durable acknowledgment.
	for _, name := range names {
		require.NoError(t, util.DeepDelete(filepath.Join(paths[0].SoftlinkPath(), name)))
	}
	require.NoError(t, session.Close())
	receiver.stop()
	receiver.start(t)
	session = receiver.connect(t)
	stdout, stderr, err := session.Exec("test -d " + "'" + strings.ReplaceAll(remoteDir, "'", "'\"'\"'") + "' && printf recovered")
	require.NoError(t, err, stderr)
	require.Equal(t, "recovered", stdout)
	recovered, err := segment.LoadSegment(logger, monitor, 0, remotePaths, false, time.Now(), true)
	require.NoError(t, err, "durably accepted segment must reopen after receiver restart")
	require.Equal(t, uint32(1), recovered.KeyCount())
	got, err := recovered.Read(key, addresses[0].Address)
	require.NoError(t, err)
	require.Equal(t, value, got)
	recovered.Release()
	require.NoError(t, recovered.BlockUntilFullyDeleted())
}
