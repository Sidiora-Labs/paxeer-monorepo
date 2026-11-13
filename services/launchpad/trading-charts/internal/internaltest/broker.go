package internaltest

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NewBroker starts an owned file-backed JetStream with the requested subjects.
func NewBroker(t *testing.T, name string, subjects []string) (*nats.Conn, jetstream.Stream) {
	t.Helper()
	binary, err := exec.LookPath("nats-server")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	log, err := os.CreateTemp(directory, "broker-*.log")
	if err != nil {
		t.Fatal(err)
	}
	process, err := os.StartProcess(binary, []string{"nats-server", "-js", "-sd", filepath.Join(directory, "data"), "-a", net.IPv4(127, 0, 0, 1).String(), "-p", strconv.Itoa(port)}, &os.ProcAttr{Env: os.Environ(), Files: []*os.File{os.Stdin, log, log}})
	if err != nil {
		_ = log.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := process.Signal(syscall.SIGINT); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Error(err)
		}
		state, err := process.Wait()
		if err != nil || !state.Success() {
			t.Error(err)
		}
		_ = log.Close()
	})
	uri := "nats://127.0.0.1:" + strconv.Itoa(port)
	var connection *nats.Conn
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		connection, err = nats.Connect(uri, nats.NoReconnect(), nats.Timeout(100*time.Millisecond))
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(connection.Close)
	js, err := jetstream.New(connection)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := js.CreateStream(context.Background(), jetstream.StreamConfig{Name: name, Subjects: subjects, Storage: jetstream.FileStorage})
	if err != nil {
		t.Fatal(err)
	}
	return connection, stream
}
