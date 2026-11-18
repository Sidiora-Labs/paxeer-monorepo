package brokerprovision

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func realBroker(t *testing.T) (*nats.Conn, jetstream.JetStream, string) {
	t.Helper()
	binary := os.Getenv("NATS_SERVER_BINARY")
	if binary == "" {
		var err error
		binary, err = exec.LookPath("nats-server")
		if err != nil {
			t.Fatal("nats-server binary required", err)
		}
	}
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		t.Fatal(err)
	}
	secret := hex.EncodeToString(secretBytes)
	listeners := []net.Listener{}
	ports := []int{}
	for i := 0; i < 6; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		listeners = append(listeners, listener)
		ports = append(ports, listener.Addr().(*net.TCPAddr).Port)
	}
	document := example(t)
	fileStoreBytes := document.Streams[0].Limits.MaxBytes * int64(document.Streams[0].Limits.Replicas)
	for _, stream := range document.Streams {
		fileStoreBytes += stream.Limits.MaxBytes * int64(stream.Limits.Replicas)
	}
	t.Cleanup(func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	})
	for i := 0; i < 3; i++ {
		directory := t.TempDir()
		routes := []string{}
		for j := 0; j < 3; j++ {
			if i != j {
				routes = append(routes, fmt.Sprintf("nats-route://127.0.0.1:%d", ports[j*2+1]))
			}
		}
		config := fmt.Sprintf("server_name: provision_%d\nhost: 127.0.0.1\nport: %d\nmax_payload: 2097152\njetstream { store_dir: %q\nmax_file_store: %d }\nauthorization { token: %q }\ncluster { name: provision\nlisten: 127.0.0.1:%d\nroutes: [%s] }\n", i, ports[i*2], filepath.Join(directory, "store"), fileStoreBytes, secret, ports[i*2+1], strings.Join(routes, ","))
		path := filepath.Join(directory, "server.conf")
		if err := os.WriteFile(path, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
		log, err := os.Create(filepath.Join(directory, "server.log"))
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command(binary, "-c", path)
		command.Stdout = log
		command.Stderr = log
		_ = listeners[i*2].Close()
		_ = listeners[i*2+1].Close()
		if err = command.Start(); err != nil {
			_ = log.Close()
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- command.Wait() }()
		t.Cleanup(func() {
			_ = command.Process.Signal(os.Interrupt)
			select {
			case err := <-done:
				if err != nil {
					t.Error("broker shutdown", err)
				}
			case <-time.After(3 * time.Second):
				_ = command.Process.Kill()
				<-done
				t.Error("broker shutdown timed out")
			}
			_ = log.Close()
			if t.Failed() {
				raw, readErr := os.ReadFile(log.Name())
				if readErr == nil {
					t.Log(strings.ReplaceAll(string(raw), secret, "[redacted]"))
				}
			}
		})
	}
	address := fmt.Sprintf("nats://127.0.0.1:%d", ports[0])
	var nc *nats.Conn
	var err error
	until := time.Now().Add(20 * time.Second)
	for time.Now().Before(until) {
		nc, err = nats.Connect(address, nats.Token(secret), nats.NoReconnect(), nats.Timeout(100*time.Millisecond))
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal("broker connection unavailable", err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), until)
	defer cancel()
	// An R3 stream admits requests only after the broker cluster elects its leader.
	for {
		request, stop := context.WithTimeout(ctx, 500*time.Millisecond)
		_, err = js.CreateStream(request, jetstream.StreamConfig{Name: "PROVISION_READY", Subjects: []string{"provision.ready"}, Storage: jetstream.FileStorage, Replicas: 3})
		stop()
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("broker cluster unavailable", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err = js.DeleteStream(ctx, "PROVISION_READY"); err != nil {
		t.Fatal(err)
	}
	waitStreamAbsent(t, ctx, js, "PROVISION_READY")
	return nc, js, secret
}

func waitStreamAbsent(t *testing.T, ctx context.Context, js jetstream.JetStream, name string) {
	t.Helper()
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for {
		_, err := js.Stream(bounded, name)
		if errors.Is(err, jetstream.ErrStreamNotFound) {
			return
		}
		if err != nil {
			t.Fatal("inspect removed stream", name, err)
		}
		select {
		case <-bounded.Done():
			t.Fatal("stream remains after removal", name, bounded.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func writeDocument(t *testing.T, d Document) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "broker.json")
	if err := os.WriteFile(path, encoded(t, d), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestProvisionExampleDurablesPreservesDeliveryAndUnlistedResources(t *testing.T) {
	nc, js, _ := realBroker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	d := example(t)
	changes, err := Provision(ctx, js, d, Updates{})
	if err != nil || len(changes) != len(d.Streams)+len(d.Durables) {
		t.Fatalf("provision changes=%v err=%v", changes, err)
	}
	outside := d.Durables[0]
	outside.Durable = "outside_document"
	outsideConfig, err := outside.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	outsideConsumer, err := js.CreateConsumer(ctx, outside.Stream, outsideConfig)
	if err != nil {
		t.Fatal(err)
	}
	outsideInfo, err := outsideConsumer.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	extra := d.Streams[0]
	extra.Name = "OUTSIDE_DOCUMENT"
	extra.Subjects = []string{"outside.document"}
	extraStream, err := js.CreateStream(ctx, extra.BrokerConfig())
	if err != nil {
		t.Fatal(err)
	}
	extraInfo, err := extraStream.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	published := map[string]bool{}
	payload := []byte("launchpad broker publication")
	for i, c := range d.Durables {
		if published[c.Subject] {
			continue
		}
		publisher, err := natsdriver.NewPublisher(nc, c.Stream)
		if err != nil {
			t.Fatal(err)
		}
		ack, err := publisher.Publish(ctx, c.Subject, fmt.Sprintf("provision_%d", i), payload)
		if err != nil || ack.Stream != c.Stream || ack.Sequence == 0 {
			t.Fatalf("publication %s: %+v %v", c.Subject, ack, err)
		}
		published[c.Subject] = true
	}
	before := map[string][]byte{}
	for _, c := range d.Durables {
		consumer, err := natsdriver.NewConsumer(ctx, nc, c)
		if err != nil {
			t.Fatalf("NewConsumer %s/%s: %v", c.Stream, c.Durable, err)
		}
		messages, err := consumer.Fetch(ctx)
		if err != nil || len(messages) != 1 {
			t.Fatalf("fetch %s: %d %v", c.Durable, len(messages), err)
		}
		if messages[0].Subject != c.Subject || !bytes.Equal(messages[0].Wire, payload) {
			t.Fatal("published message changed")
		}
		if err = messages[0].Ack(ctx); err != nil {
			t.Fatal(err)
		}
		consumer.Close()
		durable, err := js.Consumer(ctx, c.Stream, c.Durable)
		if err != nil {
			t.Fatal(err)
		}
		info, err := durable.Info(ctx)
		if err != nil {
			t.Fatal(err)
		}
		before[c.Stream+"/"+c.Durable] = encoded(t, struct {
			Created             time.Time
			Config              jetstream.ConsumerConfig
			Delivered, AckFloor jetstream.SequenceInfo
		}{info.Created, info.Config, info.Delivered, info.AckFloor})
	}
	changes, err = Provision(ctx, js, d, Updates{})
	if err != nil || len(changes) != 0 {
		t.Fatalf("second provision wrote changes: %v %v", changes, err)
	}
	differences, err := Check(ctx, js, d)
	if err != nil || len(differences) != 0 {
		t.Fatalf("equal check: %v %v", differences, err)
	}
	for _, c := range d.Durables {
		durable, err := js.Consumer(ctx, c.Stream, c.Durable)
		if err != nil {
			t.Fatal(err)
		}
		info, err := durable.Info(ctx)
		if err != nil {
			t.Fatal(err)
		}
		after := encoded(t, struct {
			Created             time.Time
			Config              jetstream.ConsumerConfig
			Delivered, AckFloor jetstream.SequenceInfo
		}{info.Created, info.Config, info.Delivered, info.AckFloor})
		if !bytes.Equal(before[c.Stream+"/"+c.Durable], after) {
			t.Fatalf("durable state changed for %s", c.Durable)
		}
		reopened, err := natsdriver.NewConsumer(ctx, nc, c)
		if err != nil {
			t.Fatal(err)
		}
		messages, err := reopened.Fetch(ctx)
		reopened.Close()
		if err != nil || len(messages) != 0 {
			t.Fatalf("acknowledged delivery replayed on %s: %d %v", c.Durable, len(messages), err)
		}
	}
	afterOutside, err := outsideConsumer.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(outsideInfo.Config, afterOutside.Config) || outsideInfo.Created != afterOutside.Created {
		t.Fatal("unlisted durable changed")
	}
	afterExtra, err := extraStream.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(extraInfo.Config, afterExtra.Config) || extraInfo.Created != afterExtra.Created {
		t.Fatal("unlisted stream changed")
	}
}

func TestCommandCheckReportsDriftAndProvisionRequiresNamedUpdates(t *testing.T) {
	nc, js, secret := realBroker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	d := example(t)
	path := writeDocument(t, d)
	t.Setenv("PROVISION_BROKER", nc.ConnectedUrl())
	t.Setenv("PROVISION_TOKEN", secret)
	args := []string{"--document", path, "--broker-env", "PROVISION_BROKER", "--token-env", "PROVISION_TOKEN"}
	var out, errOut bytes.Buffer
	run := func(action string, extra ...string) int {
		out.Reset()
		errOut.Reset()
		return Run(ctx, append(append([]string{action}, args...), extra...), &out, &errOut)
	}
	if status := run("provision"); status != 0 {
		t.Fatalf("provision status=%d: %s", status, &errOut)
	}
	if status := run("provision"); status != 0 || out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("idempotent command status=%d out=%s err=%s", status, &out, &errOut)
	}
	c := d.Durables[0]
	config, err := c.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	config.AckWait += time.Second
	changed, err := js.UpdateConsumer(ctx, c.Stream, config)
	if err != nil {
		t.Fatal(err)
	}
	changedInfo, err := changed.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = js.DeleteStream(ctx, "CHART_OUTBOX"); err != nil {
		t.Fatal(err)
	}
	waitStreamAbsent(t, ctx, js, "CHART_OUTBOX")
	if status := run("check"); status != 1 {
		t.Fatalf("check status=%d out=%s err=%s", status, &out, &errOut)
	}
	expected := "stream CHART_OUTBOX absent\ndurable " + c.Stream + "/" + c.Durable + " differs\n"
	if out.String() != expected || errOut.Len() != 0 {
		t.Fatalf("check report=%q err=%q", out.String(), errOut.String())
	}
	afterCheck, err := changed.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if afterCheck.Created != changedInfo.Created || !reflect.DeepEqual(afterCheck.Config, changedInfo.Config) {
		t.Fatal("check changed durable")
	}
	if _, err = js.Stream(ctx, "CHART_OUTBOX"); !errors.Is(err, jetstream.ErrStreamNotFound) {
		t.Fatal("check created absent stream", err)
	}
	if status := run("provision"); status != 2 || !strings.Contains(errOut.String(), c.Durable) {
		t.Fatalf("unnamed drift admitted status=%d err=%s", status, &errOut)
	}
	if _, err = js.Stream(ctx, "CHART_OUTBOX"); !errors.Is(err, jetstream.ErrStreamNotFound) {
		t.Fatal("preflight wrote before refusing drift", err)
	}
	var s Stream
	for _, candidate := range d.Streams {
		if candidate.Name != "CHART_OUTBOX" {
			s = candidate
			break
		}
	}
	configStream := s.BrokerConfig()
	configStream.MaxMsgs++
	if _, err = js.UpdateStream(ctx, configStream); err != nil {
		t.Fatal(err)
	}
	if status := run("provision", "--update-durable", c.Stream+"/"+c.Durable); status != 2 {
		t.Fatal("durable update authorized stream update")
	}
	if status := run("provision", "--update-durable", c.Stream+"/"+c.Durable, "--update-stream", s.Name); status != 0 {
		t.Fatalf("named updates status=%d err=%s", status, &errOut)
	}
	expected = "stream CHART_OUTBOX absent\nstream " + s.Name + " differs\ndurable " + c.Stream + "/" + c.Durable + " differs\n"
	if out.String() != expected || errOut.Len() != 0 {
		t.Fatalf("provision report=%q err=%q", out.String(), errOut.String())
	}
	if status := run("check"); status != 0 || out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("reconciled check status=%d out=%s err=%s", status, &out, &errOut)
	}
	addressPath := filepath.Join(t.TempDir(), "broker")
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err = os.WriteFile(addressPath, []byte(nc.ConnectedUrl()+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(tokenPath, []byte(secret+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	if status := Run(ctx, []string{"check", "--document", path, "--broker-file", addressPath, "--token-file", tokenPath}, &out, &errOut); status != 0 || out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("file sources status=%d out=%s err=%s", status, &out, &errOut)
	}
	t.Setenv("PROVISION_TOKEN", "wrong-"+secret)
	if status := run("check"); status != 2 {
		t.Fatal("invalid credentials admitted")
	}
	if strings.Contains(out.String()+errOut.String(), secret) || strings.Contains(errOut.String(), nc.ConnectedUrl()) {
		t.Fatal("connection source disclosed")
	}
}

func TestCommandRefusesInvalidArgumentsWithoutDisclosingSources(t *testing.T) {
	path := writeDocument(t, smallDocument(t))
	t.Setenv("PROVISION_ADDRESS", "nats://user:private-credential@invalid:4222")
	t.Setenv("PROVISION_SECRET", "private-credential")
	base := []string{"check", "--document", path, "--broker-env", "PROVISION_ADDRESS", "--token-env", "PROVISION_SECRET"}
	cases := [][]string{nil, {"unknown"}, {"check", "--bad=private-credential"}, base,
		append(append([]string{}, base...), "--update-stream", "KINDLE_INDEXER_V1"),
		append(append([]string{}, base...), "--timeout", "0s"),
		append(append([]string{}, base...), "--broker-file", path),
		{"provision", "--document", path, "--update-stream", "UNLISTED"},
		{"provision", "--document", path, "--update-durable", "KINDLE_INDEXER_V1/bounded", "--update-durable", "KINDLE_INDEXER_V1/bounded"},
	}
	for i, args := range cases {
		var out, errOut bytes.Buffer
		status := Run(context.Background(), args, &out, &errOut)
		if status != 2 || out.Len() != 0 || strings.Contains(errOut.String(), "private-credential") {
			t.Fatalf("invalid arguments %d status=%d out=%s err=%s", i, status, &out, &errOut)
		}
	}
}
