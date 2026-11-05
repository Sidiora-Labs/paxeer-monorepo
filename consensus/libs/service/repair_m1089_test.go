package service

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type repairM1089Service struct {
	*BaseService
	startErr     error
	startContext context.Context
	starts       atomic.Int32
	stops        atomic.Int32
	reader       *io.PipeReader
	writer       *io.PipeWriter
	allowReturn  chan struct{}
	releaseTasks chan struct{}
	started      [2]chan struct{}
	canceled     [2]chan struct{}
	finished     [2]chan struct{}
	stopped      chan struct{}
	workerErr    error
}

func (s *repairM1089Service) OnStart(ctx context.Context) error {
	s.starts.Add(1)
	s.startContext = ctx
	s.Spawn("pipe reader", func(ctx context.Context) error {
		close(s.started[0])
		<-ctx.Done()
		close(s.canceled[0])
		_, s.workerErr = io.ReadAll(s.reader)
		<-s.releaseTasks
		close(s.finished[0])
		return ctx.Err()
	})
	s.SpawnCritical("critical worker", func(ctx context.Context) error {
		close(s.started[1])
		<-ctx.Done()
		close(s.canceled[1])
		<-s.releaseTasks
		close(s.finished[1])
		return ctx.Err()
	})
	<-s.allowReturn
	return s.startErr
}

func (s *repairM1089Service) OnStop() {
	s.stops.Add(1)
	s.writer.Close()
	close(s.stopped)
}

func repairM1089Await(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("service lifecycle did not complete")
	}
}

func repairM1089Fixture(t *testing.T, startErr error) (*repairM1089Service, context.Context, context.CancelFunc, func(), func()) {
	t.Helper()
	reader, writer := io.Pipe()
	s := &repairM1089Service{
		startErr: startErr, reader: reader, writer: writer,
		allowReturn: make(chan struct{}), releaseTasks: make(chan struct{}),
		stopped: make(chan struct{}),
	}
	for i := range s.started {
		s.started[i] = make(chan struct{})
		s.canceled[i] = make(chan struct{})
		s.finished[i] = make(chan struct{})
	}
	s.BaseService = NewBaseService(t.Name(), s)
	ctx, cancel := context.WithCancel(t.Context())
	allowReturn := sync.OnceFunc(func() { close(s.allowReturn) })
	releaseTasks := sync.OnceFunc(func() { close(s.releaseTasks) })
	t.Cleanup(func() {
		cancel()
		allowReturn()
		releaseTasks()
		writer.Close()
		reader.Close()
	})
	return s, ctx, cancel, allowReturn, releaseTasks
}

func TestRepairM_1089_FailedStartState(t *testing.T) {
	for _, trigger := range []string{"startup_error", "stop_during_start", "parent_cancel_during_start"} {
		t.Run(trigger, func(t *testing.T) {
			startErr := errors.New("startup resource initialization failed")
			s, ctx, cancel, allowReturn, releaseTasks := repairM1089Fixture(t, startErr)
			startResult := make(chan error, 1)
			startReturned := make(chan struct{})
			go func() {
				startResult <- s.Start(ctx)
				close(startReturned)
			}()
			for _, started := range s.started {
				repairM1089Await(t, started)
			}
			if s.startContext != ctx {
				t.Fatal("OnStart did not receive the caller's context")
			}
			retryResult := make(chan error, 1)
			retryReturned := make(chan struct{})
			go func() {
				retryResult <- s.Start(ctx)
				close(retryReturned)
			}()
			waitReturned := make(chan struct{})
			go func() { s.Wait(); close(waitReturned) }()
			stopReturned := make(chan struct{})
			if trigger == "stop_during_start" {
				go func() { s.Stop(); close(stopReturned) }()
			} else if trigger == "parent_cancel_during_start" {
				cancel()
			}
			if trigger != "startup_error" {
				for _, canceled := range s.canceled {
					repairM1089Await(t, canceled)
				}
				if s.stops.Load() != 0 {
					t.Fatal("OnStop ran before OnStart finished")
				}
			}
			allowReturn()
			for _, canceled := range s.canceled {
				repairM1089Await(t, canceled)
			}
			repairM1089Await(t, s.stopped)
			if s.stops.Load() != 1 {
				t.Fatal("failed startup did not clean resources exactly once")
			}
			if trigger != "stop_during_start" {
				go func() { s.Stop(); close(stopReturned) }()
			}
			for _, returned := range []<-chan struct{}{startReturned, retryReturned, stopReturned, waitReturned} {
				select {
				case <-returned:
					t.Fatal("lifecycle returned before owned work finished")
				default:
				}
			}
			releaseTasks()
			for _, returned := range []<-chan struct{}{startReturned, retryReturned, stopReturned, waitReturned} {
				repairM1089Await(t, returned)
			}
			if err := <-startResult; !errors.Is(err, startErr) {
				t.Fatalf("Start returned %v, want the original startup error", err)
			}
			if err := <-retryResult; !errors.Is(err, startErr) {
				t.Fatalf("concurrent Start returned %v, want the original startup error", err)
			}
			for _, finished := range s.finished {
				repairM1089Await(t, finished)
			}
			if s.workerErr != nil {
				t.Fatalf("owned reader failed after resource cleanup: %v", s.workerErr)
			}
			if s.IsRunning() {
				t.Fatal("failed service still reports running")
			}
			if trigger != "parent_cancel_during_start" && ctx.Err() != nil {
				t.Fatal("service cleanup canceled the caller's context")
			}
			repeated := make(chan struct{})
			go func() {
				s.Stop()
				s.Wait()
				close(repeated)
			}()
			repairM1089Await(t, repeated)
			for range 3 {
				if err := s.Start(t.Context()); !errors.Is(err, startErr) {
					t.Fatalf("subsequent Start returned %v, want the original startup error", err)
				}
			}
			if s.starts.Load() != 1 || s.stops.Load() != 1 || s.IsRunning() {
				t.Fatal("failed service restarted or repeated cleanup")
			}
		})
	}
}

func TestRepairM_1089_SuccessfulStartupStillStops(t *testing.T) {
	s, ctx, _, allowReturn, releaseTasks := repairM1089Fixture(t, nil)
	allowReturn()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if !s.IsRunning() {
		t.Fatal("successfully started service is not running")
	}
	if err := s.Start(ctx); err != nil || s.starts.Load() != 1 {
		t.Fatalf("running service did not retain its startup result: %v", err)
	}
	releaseTasks()
	stopped := make(chan struct{})
	go func() { s.Stop(); s.Wait(); close(stopped) }()
	repairM1089Await(t, stopped)
	for _, finished := range s.finished {
		repairM1089Await(t, finished)
	}
	if s.IsRunning() || s.stops.Load() != 1 || s.workerErr != nil {
		t.Fatal("successful service did not shut down its resources and owned work")
	}
}
