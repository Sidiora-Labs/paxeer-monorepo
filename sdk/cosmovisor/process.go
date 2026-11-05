package cosmovisor

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

// LaunchProcess runs a subprocess and returns when the subprocess exits,
// either when it dies, or *after* a successful upgrade.
func LaunchProcess(cfg *Config, args []string, stdout, stderr io.Writer) (bool, error) {
	bin, err := cfg.CurrentBin()
	if err != nil {
		return false, fmt.Errorf("error creating symlink to genesis: %w", err)
	}

	if err := EnsureBinary(bin); err != nil {
		return false, fmt.Errorf("current binary invalid: %w", err)
	}

	cmd := exec.Command(bin, args...)
	outpipe, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}

	defer outpipe.Close()

	errpipe, err := cmd.StderrPipe()
	if err != nil {
		return false, err
	}

	defer errpipe.Close()

	scanOut := bufio.NewScanner(io.TeeReader(outpipe, stdout))
	scanErr := bufio.NewScanner(io.TeeReader(errpipe, stderr))
	// set scanner's buffer size to cfg.LogBufferSize, and ensure larger than bufio.MaxScanTokenSize otherwise fallback to bufio.MaxScanTokenSize
	var maxCapacity int
	if cfg.LogBufferSize < bufio.MaxScanTokenSize {
		maxCapacity = bufio.MaxScanTokenSize
	} else {
		maxCapacity = cfg.LogBufferSize
	}
	bufOut := make([]byte, maxCapacity)
	bufErr := make([]byte, maxCapacity)
	scanOut.Buffer(bufOut, maxCapacity)
	scanErr.Buffer(bufErr, maxCapacity)

	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("launching process %s %s: %w", bin, strings.Join(args, " "), err)
	}

	stopForwarding := forwardProcessSignals(cmd.Process)

	upgradeInfo, err := WaitForUpgradeOrExit(cmd, scanOut, scanErr)
	forwardErr := stopForwarding()
	if forwardErr != nil {
		return false, forwardErr
	}
	if err != nil {
		return false, err
	}

	if upgradeInfo != nil {
		return true, DoUpgrade(cfg, upgradeInfo)
	}

	return false, nil
}

func forwardProcessSignals(process *os.Process) func() error {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGQUIT, syscall.SIGTERM)
	done := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- forwardSignals(process, sigs, done)
	}()
	return func() error {
		signal.Stop(sigs)
		close(done)
		return <-result
	}
}

func forwardSignals(process *os.Process, sigs <-chan os.Signal, done <-chan struct{}) error {
	for {
		select {
		case <-done:
			return nil
		case sig := <-sigs:
			select {
			case <-done:
				return nil
			default:
			}
			if err := process.Signal(sig); err != nil {
				if errors.Is(err, os.ErrProcessDone) {
					return nil
				}
				// Unblock the supervisor so it can report the forwarding failure.
				_ = process.Kill()
				return fmt.Errorf("forwarding signal %s: %w", sig, err)
			}
		}
	}
}

// WaitResult is used to wrap feedback on cmd state with some mutex logic.
// This is needed as multiple go-routines can affect this - two read pipes that can trigger upgrade
// As well as the command, which can fail
type WaitResult struct {
	// both err and info may be updated from several go-routines
	// access is wrapped by mutex and should only be done through methods
	err   error
	info  *UpgradeInfo
	mutex sync.Mutex
}

// AsResult reads the data protected by mutex to avoid race conditions
func (u *WaitResult) AsResult() (*UpgradeInfo, error) {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	return u.info, u.err
}

// SetError will set with the first error using a mutex
// don't set it once info is set, that means we chose to kill the process
func (u *WaitResult) SetError(myErr error) {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	if u.info == nil && u.err == nil && myErr != nil {
		u.err = myErr
	}
}

// SetUpgrade sets first non-nil upgrade info, ensure error is then nil
// pass in a command to shutdown on successful upgrade
func (u *WaitResult) SetUpgrade(up *UpgradeInfo) {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	if u.info == nil && up != nil {
		u.info = up
		u.err = nil
	}
}

// WaitForUpgradeOrExit listens to both output streams of the process, as well as the process state itself
// When it returns, the process is finished and all streams have closed.
//
// It returns (info, nil) if an upgrade should be initiated (and we killed the process)
// It returns (nil, err) if the process died by itself, or there was an issue reading the pipes
// It returns (nil, nil) if the process exited normally without triggering an upgrade. This is very unlikely
// to happened with "start" but may happened with short-lived commands like `gaiad export ...`
func WaitForUpgradeOrExit(cmd *exec.Cmd, scanOut, scanErr *bufio.Scanner) (*UpgradeInfo, error) {
	var res WaitResult
	var readers sync.WaitGroup
	scanErrors := make(chan error, 2)

	waitScan := func(scan *bufio.Scanner) {
		defer readers.Done()
		upgrade, err := WaitForUpdate(scan)
		if upgrade != nil {
			res.SetUpgrade(upgrade)
			_ = cmd.Process.Kill()
			// Drain remaining output before releasing the caller's writers.
			for scan.Scan() {
			}
			err = scan.Err()
		}
		if err != nil {
			scanErrors <- err
			_ = cmd.Process.Kill()
		}
	}

	readers.Add(2)
	go waitScan(scanOut)
	go waitScan(scanErr)
	// Wait must not close StdoutPipe/StderrPipe while scanners still read them.
	readers.Wait()
	err := cmd.Wait()
	close(scanErrors)
	for scanErr := range scanErrors {
		return nil, scanErr
	}
	res.SetError(err)
	return res.AsResult()
}
