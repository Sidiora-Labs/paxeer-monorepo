package holdings

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

func phase4Command(ctx context.Context, executable string, args ...string) (*exec.Cmd, func() bool) {
	command := &exec.Cmd{
		Path:      executable,
		Args:      append([]string{executable}, args...),
		WaitDelay: time.Second,
	}
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		return command.Process.Kill()
	}
	stop := context.AfterFunc(ctx, func() {
		if err := command.Cancel(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			slog.Error("stop measurement command", "error", err)
		}
	})
	return command, stop
}
