package holdings

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func phase4Command(ctx context.Context, processGroup bool, executable string, args ...string) (*exec.Cmd, func() bool) {
	command := &exec.Cmd{
		Path:      executable,
		Args:      append([]string{executable}, args...),
		WaitDelay: time.Second,
	}
	if processGroup {
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	stopProcess := func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		if processGroup {
			return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		}
		return command.Process.Kill()
	}
	stop := context.AfterFunc(ctx, func() {
		if err := stopProcess(); err != nil && !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH) {
			slog.Error("stop measurement command", "error", err)
		}
	})
	return command, stop
}
