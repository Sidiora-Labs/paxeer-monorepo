package util

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHTimeouts bounds connections and control commands. Transfer is a finite
// rsync I/O inactivity bound, rounded up to whole seconds, not a total duration.
// Zero fields use the production defaults; negative durations are rejected.
type SSHTimeouts struct{ Dial, Command, Transfer time.Duration }

func (t SSHTimeouts) defaults() (SSHTimeouts, error) {
	if t.Dial < 0 || t.Command < 0 || t.Transfer < 0 {
		return t, fmt.Errorf("SSH timeouts must be positive")
	}
	if t.Dial == 0 {
		t.Dial = 10 * time.Second
	}
	if t.Command == 0 {
		t.Command = time.Minute
	}
	if t.Transfer == 0 {
		t.Transfer = 5 * time.Minute
	}
	return t, nil
}

// SSHSession authenticates control and rsync connections against the same
// immutable known-hosts snapshot. Close cancels and joins all active operations.
type SSHSession struct {
	logger                  *slog.Logger
	client                  *ssh.Client
	user, host              string
	port                    uint64
	keyPath, knownHostsPath string
	verbose                 bool
	ctx                     context.Context
	cancel                  context.CancelFunc
	timeouts                SSHTimeouts
	mu                      sync.Mutex
	closed                  bool
	operations              sync.WaitGroup
	done                    chan struct{}
	cleanupErr              error
}

func NewSSHSession(logger *slog.Logger, user, host string, port uint64, keyPath, knownHosts string, verbose bool, options ...SSHTimeouts) (*SSHSession, error) {
	return NewSSHSessionContext(context.Background(), logger, user, host, port, keyPath, knownHosts, verbose, options...)
}

// NewSSHSessionContext requires a known-hosts trust source. Parent cancellation
// closes the transport, kills in-flight rsync and removes private trust material.
func NewSSHSessionContext(ctx context.Context, logger *slog.Logger, user, host string, port uint64, keyPath, knownHosts string, verbose bool, options ...SSHTimeouts) (*SSHSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if knownHosts == "" {
		return nil, fmt.Errorf("SSH requires a known-hosts trust source")
	}
	if port == 0 || port > 65535 || user == "" || host == "" || strings.HasPrefix(user, "-") || strings.HasPrefix(host, "-") {
		return nil, fmt.Errorf("invalid SSH destination")
	}
	for _, c := range user {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_.-", c) {
			return nil, fmt.Errorf("invalid SSH user")
		}
	}
	if net.ParseIP(host) == nil {
		for _, c := range host {
			if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_.-", c) {
				return nil, fmt.Errorf("invalid SSH host")
			}
		}
	}
	if len(options) > 1 {
		return nil, fmt.Errorf("only one SSH timeout configuration is allowed")
	}
	var timeouts SSHTimeouts
	if len(options) == 1 {
		timeouts = options[0]
	}
	timeouts, err := timeouts.defaults()
	if err != nil {
		return nil, err
	}
	keyPath, err = SanitizePath(keyPath)
	if err != nil {
		return nil, err
	}
	knownHosts, err = SanitizePath(knownHosts)
	if err != nil {
		return nil, err
	}
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}
	key, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	trustData, err := os.ReadFile(knownHosts)
	if err != nil {
		return nil, fmt.Errorf("failed to read known hosts: %w", err)
	}
	// OpenSSH parses -o paths a second time. Private fixed filenames avoid that
	// parser changing operator-supplied spaces, quotes, newlines or expansions.
	trustDir, err := os.MkdirTemp("", "litt-ssh-")
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(trustDir)
		}
	}()
	keyPath, knownHosts = filepath.Join(trustDir, "key"), filepath.Join(trustDir, "known_hosts")
	if err := os.WriteFile(keyPath, keyData, 0600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(knownHosts, trustData, 0600); err != nil {
		return nil, err
	}
	hostKeyCallback, err := knownhosts.New(knownHosts)
	if err != nil {
		return nil, fmt.Errorf("failed to parse known hosts: %w", err)
	}
	config := &ssh.ClientConfig{User: user, HostKeyCallback: hostKeyCallback, Auth: []ssh.AuthMethod{ssh.PublicKeys(key)}, Timeout: timeouts.Dial}
	dialCtx, dialCancel := context.WithTimeout(ctx, timeouts.Dial)
	defer dialCancel()
	address := net.JoinHostPort(host, strconv.FormatUint(port, 10))
	connection, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	deadline, _ := dialCtx.Deadline()
	if err := connection.SetDeadline(deadline); err != nil {
		_ = connection.Close()
		return nil, err
	}
	stopped := make(chan struct{})
	stopCancel := context.AfterFunc(dialCtx, func() { _ = connection.Close(); close(stopped) })
	conn, chans, reqs, err := ssh.NewClientConn(connection, address, config)
	if !stopCancel() {
		<-stopped
	}
	if err != nil {
		_ = connection.Close()
		if dialCtx.Err() != nil {
			return nil, fmt.Errorf("failed SSH handshake: %w", dialCtx.Err())
		}
		return nil, fmt.Errorf("failed SSH handshake: %w", err)
	}
	if err := dialCtx.Err(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := connection.SetDeadline(time.Time{}); err != nil {
		_ = conn.Close()
		return nil, err
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	s := &SSHSession{logger: logger, client: ssh.NewClient(conn, chans, reqs), user: user, host: host, port: port,
		keyPath: keyPath, knownHostsPath: knownHosts, verbose: verbose, ctx: sessionCtx, cancel: cancel, timeouts: timeouts, done: make(chan struct{})}
	go func() {
		<-sessionCtx.Done()
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		_ = s.client.Close()
		s.operations.Wait()
		_ = s.client.Wait()
		s.cleanupErr = os.RemoveAll(trustDir)
		close(s.done)
	}()
	success = true
	return s, nil
}

func (s *SSHSession) Close() error {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	<-s.done
	return s.cleanupErr
}

func (s *SSHSession) operation(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.ctx.Err() != nil {
		return nil, nil, fmt.Errorf("SSH session is closed")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	s.operations.Add(1)
	var operationCtx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		operationCtx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		// Transfers delegate inactivity detection to rsync while retaining the
		// caller's deadline and both caller and session cancellation.
		operationCtx, cancel = context.WithCancel(ctx)
	}
	stop := context.AfterFunc(s.ctx, cancel)
	return operationCtx, func() { stop(); cancel(); s.operations.Done() }, nil
}

func (s *SSHSession) FindFiles(root string, extensions []string) ([]string, error) {
	return s.FindFilesContext(s.ctx, root, extensions)
}

func (s *SSHSession) FindFilesContext(ctx context.Context, root string, extensions []string) ([]string, error) {
	if strings.ContainsRune(root, 0) {
		return nil, fmt.Errorf("path contains NUL")
	}
	// Prefix relative roots so find cannot interpret them as expressions/options.
	if !strings.HasPrefix(root, "/") {
		root = "./" + root
	}
	command := "find " + QuoteRemoteArgument(root) + " -type f -print0"
	stdout, stderr, err := s.ExecContext(ctx, command)
	if err != nil {
		var remoteExit *ssh.ExitError
		if errors.As(err, &remoteExit) && remoteExit.ExitStatus() == 1 && stdout == "" && strings.HasPrefix(stderr, "find: ") && strings.HasSuffix(stderr, ": No such file or directory\n") {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to discover remote files: %w, stderr: %s", err, stderr)
	}
	if stdout != "" && !strings.HasSuffix(stdout, "\x00") {
		return nil, fmt.Errorf("remote discovery is not NUL terminated")
	}
	files := strings.Split(stdout, "\x00")
	filtered := make([]string, 0, len(files))
	for _, file := range files {
		if file == "" {
			continue
		}
		for _, ext := range extensions {
			if strings.HasSuffix(file, ext) {
				filtered = append(filtered, file)
				break
			}
		}
	}
	return filtered, nil
}

func (s *SSHSession) Mkdirs(path string) error { return s.MkdirsContext(s.ctx, path) }

func (s *SSHSession) MkdirsContext(ctx context.Context, path string) error {
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("path contains NUL")
	}
	_, stderr, err := s.ExecContext(ctx, "mkdir -p -- "+QuoteRemoteArgument(path))
	if err != nil {
		return fmt.Errorf("failed to create directory: %w, stderr: %s", err, stderr)
	}
	return nil
}

func (s *SSHSession) Rsync(sourceFile, destFile string, throttleMB float64) error {
	return s.RsyncContext(s.ctx, sourceFile, destFile, throttleMB)
}

func (s *SSHSession) RsyncContext(ctx context.Context, sourceFile, destFile string, throttleMB float64) error {
	ctx, finish, err := s.operation(ctx, 0)
	if err != nil {
		return err
	}
	defer finish()
	cmd, err := s.rsyncCommand(ctx, sourceFile, destFile, throttleMB)
	if err != nil {
		return err
	}
	if s.verbose {
		s.logger.Info("Executing", "command", cmd.Args)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("failed to rsync data: %w", ctx.Err())
		}
		return fmt.Errorf("failed to rsync data: %w", err)
	}
	return s.acknowledgeDurable(ctx, destFile)
}

// rsync parses its -e string itself, with doubled quotes inside a quoted word.
func quoteRsyncArgument(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }

func (s *SSHSession) rsyncCommand(ctx context.Context, sourceFile, destFile string, throttleMB float64) (*exec.Cmd, error) {
	if sourceFile == "" || destFile == "" || strings.ContainsRune(sourceFile+destFile, 0) {
		return nil, fmt.Errorf("invalid transfer path")
	}
	if math.IsNaN(throttleMB) || math.IsInf(throttleMB, 0) || throttleMB > float64(math.MaxInt)/1024 {
		return nil, fmt.Errorf("invalid transfer throttle")
	}
	// EvalSymlinks resolves relative targets against their containing directories.
	sourceFile, err := filepath.EvalSymlinks(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source: %w", err)
	}
	sourceFile, err = filepath.Abs(sourceFile)
	if err != nil {
		return nil, err
	}
	// OpenSSH expands percent/environment tokens in configured filenames. Use
	// escaped values even for the private trust directory (TMPDIR is configurable).
	sshPath := func(p string) string {
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `%`, `%%`, `$`, `\$`).Replace(p) + `"`
	}
	sshArgs := []string{"ssh", "-i", s.keyPath, "-p", strconv.FormatUint(s.port, 10),
		"-o", "StrictHostKeyChecking=yes", "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes",
		"-o", "GlobalKnownHostsFile=/dev/null", "-o", "UserKnownHostsFile=" + sshPath(s.knownHostsPath),
		"-o", "ConnectTimeout=" + strconv.Itoa(int(math.Ceil(s.timeouts.Dial.Seconds()))),
		"-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=2"}
	for i := range sshArgs {
		sshArgs[i] = quoteRsyncArgument(sshArgs[i])
	}
	host := s.host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	target := fmt.Sprintf("%s@%s:%s", s.user, host, destFile)
	args := []string{"-z", "--fsync", "--secluded-args", "--timeout=" + strconv.Itoa(int(math.Ceil(s.timeouts.Transfer.Seconds())))}
	if throttleMB > 0 {
		args = append(args, fmt.Sprintf("--bwlimit=%d", int(throttleMB*1024)))
	}
	args = append(args, "-e", strings.Join(sshArgs, " "), "--", sourceFile, target)
	cmd := exec.CommandContext(ctx, "rsync", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 5 * time.Second
	return cmd, nil
}

func (s *SSHSession) acknowledgeDurable(ctx context.Context, destFile string) error {
	const acknowledgment = "litt-durable-v1\n"
	command := "sync -f -- " + QuoteRemoteArgument(destFile) + " && printf 'litt-durable-v1\\n'"
	stdout, stderr, err := s.ExecContext(ctx, command)
	if err != nil {
		return fmt.Errorf("remote durable commit failed for %s: %w, stderr: %s", destFile, err, stderr)
	}
	if stdout != acknowledgment {
		return fmt.Errorf("missing remote durable acknowledgment for %s: %q", destFile, stdout)
	}
	return nil
}

// QuoteRemoteArgument encodes one literal POSIX shell word, including empty words.
// Remote paths and executable names must use this function, never interpolation.
func QuoteRemoteArgument(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func shellQuote(value string) string { return QuoteRemoteArgument(value) }

func (s *SSHSession) Exec(command string) (string, string, error) {
	return s.ExecContext(s.ctx, command)
}

func (s *SSHSession) ExecContext(ctx context.Context, command string) (stdout string, stderr string, err error) {
	if strings.ContainsRune(command, 0) {
		return "", "", fmt.Errorf("command contains NUL")
	}
	ctx, finish, err := s.operation(ctx, s.timeouts.Command)
	if err != nil {
		return "", "", err
	}
	defer finish()
	type result struct {
		stdout, stderr string
		err            error
	}
	done := make(chan result, 1)
	if s.verbose {
		s.logger.Info("Executing remotely", "command", command)
	}
	go func() {
		r := func() result {
			session, err := s.client.NewSession()
			if err != nil {
				return result{err: err}
			}
			defer session.Close()
			var out, diagnostic bytes.Buffer
			session.Stdout, session.Stderr = &out, &diagnostic
			err = session.Run(command)
			return result{out.String(), diagnostic.String(), err}
		}()
		done <- r
	}()
	var r result
	select {
	case r = <-done:
	case <-ctx.Done():
		// NewSession and the exec request can stall before a channel is returned;
		// closing the entire transport unblocks and joins either phase.
		s.cancel()
		r = <-done
	}
	if ctx.Err() != nil {
		return r.stdout, r.stderr, ctx.Err()
	}
	if r.err != nil {
		return r.stdout, r.stderr, fmt.Errorf("failed remote command: %w", r.err)
	}
	return r.stdout, r.stderr, nil
}
