package tempfile_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
)

// Ptrace observes actual filesystem operations and injects EIO syscall results
// without replacing the writer or its signing caller.
type repair108SyscallInfo struct {
	Operation uint8
	Padding   [3]byte
	Arch      uint32
	IP        uint64
	SP        uint64
	Data      [8]uint64
}

type repair108SignResult struct {
	Failed    bool
	IOError   bool
	Signature []byte
}

func repair108TraceChild(t *testing.T, dir string) {
	t.Helper()
	pv, err := privval.LoadFilePV(filepath.Join(dir, "key.json"), filepath.Join(dir, "state.json"))
	require.NoError(t, err)
	vote := repair108Vote()
	runtime.LockOSThread()
	require.NoError(t, unix.Sendmsg(3, []byte(strconv.Itoa(unix.Gettid())), nil, nil, 0))
	var start [1]byte
	_, _, _, _, err = unix.Recvmsg(3, start[:], nil, 0)
	require.NoError(t, err)
	err = pv.SignVote(t.Context(), repair108ChainID, vote)
	result, marshalErr := json.Marshal(repair108SignResult{
		Failed:    err != nil,
		IOError:   errors.Is(err, unix.EIO),
		Signature: vote.Signature,
	})
	require.NoError(t, marshalErr)
	require.NoError(t, unix.Sendmsg(3, result, nil, nil, 0))
	os.Exit(0)
}

func repair108SetIOError(t *testing.T, tid int) {
	t.Helper()
	// NT_PRSTATUS holds the native general registers. Only the syscall result
	// register changes; the actual sync/close has already executed.
	resultOffsets := map[string]int{"amd64": 80, "arm64": 0, "riscv64": 80}
	offset, supported := resultOffsets[runtime.GOARCH]
	require.True(t, supported, "syscall result register layout required for %s", runtime.GOARCH)
	var registers [512]byte
	iov := unix.Iovec{Base: &registers[0]}
	iov.SetLen(len(registers))
	_, _, errno := unix.Syscall6(unix.SYS_PTRACE, unix.PTRACE_GETREGSET, uintptr(tid), unix.NT_PRSTATUS, uintptr(unsafe.Pointer(&iov)), 0, 0)
	require.Zero(t, errno)
	result := -int64(unix.EIO)
	binary.NativeEndian.PutUint64(registers[offset:], uint64(result))
	_, _, errno = unix.Syscall6(unix.SYS_PTRACE, unix.PTRACE_SETREGSET, uintptr(tid), unix.NT_PRSTATUS, uintptr(unsafe.Pointer(&iov)), 0, 0)
	require.Zero(t, errno)
}

func TestRepair_108_SignerIOFailures(t *testing.T) {
	if dir := os.Getenv("REPAIR_108_TRACE_CHILD"); dir != "" {
		repair108TraceChild(t, dir)
		return
	}
	for _, fault := range []string{"none", "file-sync", "file-close", "directory-sync", "directory-close"} {
		t.Run(fault, func(t *testing.T) {
			keyPath, statePath := repair108Signer(t)
			dir := filepath.Dir(statePath)
			before, err := os.ReadFile(statePath)
			require.NoError(t, err)
			sockets, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_SEQPACKET|unix.SOCK_CLOEXEC, 0)
			require.NoError(t, err)
			parentSocket := os.NewFile(uintptr(sockets[0]), "trace-parent")
			defer parentSocket.Close()
			childSocket := os.NewFile(uintptr(sockets[1]), "trace-child")
			defer childSocket.Close()
			require.NoError(t, unix.SetsockoptTimeval(sockets[0], unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 20}))
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRepair_108_SignerIOFailures$")
			cmd.Env = append(os.Environ(), "REPAIR_108_TRACE_CHILD="+dir)
			cmd.ExtraFiles = []*os.File{childSocket}
			var output bytes.Buffer
			cmd.Stdout, cmd.Stderr = &output, &output
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			require.NoError(t, cmd.Start())
			waited, attached := false, false
			tid := 0
			defer func() {
				if attached {
					_ = unix.PtraceDetach(tid)
				}
				if !waited {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
			}()
			require.NoError(t, childSocket.Close())
			var handshake [32]byte
			n, _, _, _, err := unix.Recvmsg(sockets[0], handshake[:], nil, 0)
			if err != nil || n == 0 {
				_ = cmd.Process.Kill()
				waitErr := cmd.Wait()
				waited = true
				t.Fatalf("receive signer thread: n=%d, error=%v, child=%v: %s", n, err, waitErr, output.String())
			}
			tid, err = strconv.Atoi(string(handshake[:n]))
			require.NoError(t, err)
			require.NoError(t, unix.PtraceAttach(tid))
			attached = true
			var status unix.WaitStatus
			_, err = unix.Wait4(tid, &status, unix.WALL, nil)
			require.NoError(t, err)
			require.True(t, status.Stopped())
			require.NoError(t, unix.PtraceSetOptions(tid, unix.PTRACE_O_TRACESYSGOOD|unix.PTRACE_O_EXITKILL))
			require.NoError(t, unix.Sendmsg(sockets[0], []byte{1}, nil, nil, 0))
			require.NoError(t, unix.PtraceSyscall(tid, 0))
			var events []string
			injected, pending := false, false
			var result repair108SignResult
			for {
				_, err := unix.Wait4(tid, &status, unix.WALL, nil)
				if errors.Is(err, unix.EINTR) {
					continue
				}
				require.NoError(t, err)
				require.True(t, status.Stopped(), "signer must return its result before exit: %v", status)
				signal := 0
				if status.StopSignal() == unix.SIGTRAP|0x80 {
					var info repair108SyscallInfo
					_, _, errno := unix.Syscall6(unix.SYS_PTRACE, unix.PTRACE_GET_SYSCALL_INFO, uintptr(tid), unsafe.Sizeof(info), uintptr(unsafe.Pointer(&info)), 0, 0)
					require.Zero(t, errno)
					if info.Operation == 1 && (info.Data[0] == unix.SYS_FSYNC || info.Data[0] == unix.SYS_CLOSE) {
						path, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%d", cmd.Process.Pid, info.Data[1]))
						require.NoError(t, err)
						kind := ""
						if path == dir {
							kind = "directory"
						} else if filepath.Dir(path) == dir && strings.HasPrefix(filepath.Base(path), "write-file-atomic-") {
							kind = "file"
						}
						operation := "close"
						if info.Data[0] == unix.SYS_FSYNC {
							operation = "sync"
						}
						if kind != "" {
							event := kind + "-" + operation
							events = append(events, event)
							if kind == "directory" {
								data, err := os.ReadFile(statePath)
								require.NoError(t, err)
								var state privval.FilePVLastSignState
								require.NoError(t, json.Unmarshal(data, &state))
								require.Equal(t, repair108Vote().Height, state.Height, "directory sync must follow rename")
							}
							if event == fault && !injected {
								pending, injected = true, true
							}
						}
					} else if info.Operation == 2 && pending {
						require.Zero(t, int64(info.Data[0]), "filesystem operation must execute before injecting its EIO result")
						repair108SetIOError(t, tid)
						pending = false
					}
				} else if status.StopSignal() != unix.SIGSTOP {
					signal = int(status.StopSignal())
				}
				data := make([]byte, 4096)
				n, _, _, _, receiveErr := unix.Recvmsg(sockets[0], data, nil, unix.MSG_DONTWAIT)
				if receiveErr == nil && n > 0 {
					require.NoError(t, json.Unmarshal(data[:n], &result))
					require.NoError(t, unix.PtraceDetach(tid))
					attached = false
					break
				}
				require.ErrorIs(t, receiveErr, unix.EAGAIN)
				require.NoError(t, unix.PtraceSyscall(tid, signal))
			}
			err = cmd.Wait()
			waited = true
			require.NoError(t, err, "%s", output.String())
			require.NoError(t, ctx.Err())
			expected := []string{"file-sync", "file-close", "directory-sync", "directory-close"}
			if strings.HasPrefix(fault, "file-") {
				expected = expected[:2]
				after, err := os.ReadFile(statePath)
				require.NoError(t, err)
				require.Equal(t, before, after, "failed file finalization must preserve the previous watermark")
			}
			require.Equal(t, expected, events)
			require.Equal(t, fault != "none", injected)
			require.Equal(t, fault != "none", result.Failed)
			require.Equal(t, fault != "none", result.IOError, "EIO must reach SignVote")
			if fault == "none" {
				vote := repair108Vote()
				vote.Signature = result.Signature
				repair108AssertRecovered(t, keyPath, statePath, vote)
			} else {
				require.Empty(t, result.Signature, "failed persistence must not return a signature")
			}
			leftovers, err := filepath.Glob(filepath.Join(dir, "write-file-atomic-*"))
			require.NoError(t, err)
			require.Empty(t, leftovers)
		})
	}
}
