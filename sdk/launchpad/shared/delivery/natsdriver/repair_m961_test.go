package natsdriver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// This committed loopback fixture speaks the NATS/JetStream wire protocol.
// The component being qualified is the production controller and its durable
// journal, codec and NATS client, with no replacement controller or store.
type repair961Broker struct {
	mu       sync.Mutex
	listener net.Listener
	member   EpochMemberAuthority
	clients  map[*repair961Client]bool
	revoked  bool
	sequence uint64
	messages [][]byte
	acked    int
	reloads  int
	denied   int
	stopped  chan struct{}
	workers  sync.WaitGroup
}

type repair961Client struct {
	net.Conn
	writeMu       sync.Mutex
	subscriptions map[string]string
	username      string
}

func (c *repair961Client) send(wire string) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := io.WriteString(c.Conn, wire)
	return err
}

func repair961Fixture(t *testing.T, name string) (EpochMemberAuthority, *repair961Broker) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err = os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	config := []byte(`{"users":["system-operator","epoch-three","epoch-four"]}`)
	replacement := []byte(`{"users":["system-operator","epoch-four"]}`)
	path := filepath.Join(dir, "server.conf")
	if err = os.WriteFile(path, config, 0600); err != nil {
		t.Fatal(err)
	}
	cfg := ConsumerConfig{Stream: "RECOVERY_PROBE", Durable: "epoch_probe", Subject: "recovery.epoch.probe", Batch: 1, MaxAckPending: 16, MaxWaiting: 1, MaxDeliver: 10, FetchWait: 250 * time.Millisecond, AckWait: time.Second}
	id := "N" + strings.Repeat("A", 55)
	if name == "member-two" {
		id = "N" + strings.Repeat("B", 55)
	}
	member := EpochMemberAuthority{Name: name, URL: "nats://" + listener.Addr().String(), ServerID: id, ConfigPath: path, ExistingSHA256: epochSHA(config), Replacement: replacement, ReplacementSHA256: epochSHA(replacement), Credentials: []EpochCredential{{ID: "epoch-three", Username: "epoch-three", Password: "test-epoch-three-secret"}}, Probe: cfg}
	broker := &repair961Broker{listener: listener, member: member, clients: map[*repair961Client]bool{}, stopped: make(chan struct{})}
	broker.workers.Add(1)
	go func() {
		defer broker.workers.Done()
		defer close(broker.stopped)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			client := &repair961Client{Conn: conn, subscriptions: map[string]string{}}
			broker.mu.Lock()
			broker.clients[client] = true
			broker.mu.Unlock()
			broker.workers.Add(1)
			go func() { defer broker.workers.Done(); broker.serve(client) }()
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		<-broker.stopped
		broker.mu.Lock()
		for c := range broker.clients {
			c.Close()
		}
		broker.mu.Unlock()
		broker.workers.Wait()
	})
	control, err := nats.Connect(member.URL, nats.UserInfo("system-operator", "test-system-secret"), nats.NoReconnect(), nats.Timeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(control.Close)
	member.Control = control
	broker.member.Control = control
	return member, broker
}

func (b *repair961Broker) serve(c *repair961Client) {
	defer c.Close()
	defer func() { b.mu.Lock(); delete(b.clients, c); b.mu.Unlock() }()
	info, _ := json.Marshal(map[string]any{"server_id": b.member.ServerID, "server_name": b.member.Name, "version": "2.12.6", "proto": 1, "host": "127.0.0.1", "port": b.listener.Addr().(*net.TCPAddr).Port, "headers": true, "auth_required": true, "max_payload": 1048576})
	if c.send("INFO "+string(info)+"\r\n") != nil {
		return
	}
	reader := bufio.NewReader(c)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			return
		}
		switch parts[0] {
		case "CONNECT":
			var auth struct {
				User string `json:"user"`
				Pass string `json:"pass"`
			}
			if json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "CONNECT "))), &auth) != nil {
				return
			}
			b.mu.Lock()
			denied := b.revoked && auth.User == "epoch-three"
			b.mu.Unlock()
			password := map[string]string{"system-operator": "test-system-secret", "epoch-three": "test-epoch-three-secret", "epoch-four": "test-epoch-four-secret"}[auth.User]
			if denied || password == "" || auth.Pass != password {
				b.mu.Lock()
				b.denied++
				b.mu.Unlock()
				_ = c.send("-ERR 'Authorization Violation'\r\n")
				return
			}
			b.mu.Lock()
			c.username = auth.User
			b.mu.Unlock()
		case "PING":
			if c.send("PONG\r\n") != nil {
				return
			}
		case "PONG":
		case "SUB":
			if len(parts) != 3 && len(parts) != 4 {
				return
			}
			c.subscriptions[parts[1]] = parts[len(parts)-1]
		case "UNSUB":
			// Request multiplexers use an auto-unsubscribe count. The fixture retains
			// the subscription until its connection closes, as the client owns limits.
		case "PUB", "HPUB":
			if len(parts) < 3 || c.username == "" {
				return
			}
			total, err := strconv.Atoi(parts[len(parts)-1])
			if err != nil || total < 0 || total > 1<<20 {
				return
			}
			wire := make([]byte, total+2)
			if _, err = io.ReadFull(reader, wire); err != nil {
				return
			}
			if !bytes.Equal(wire[total:], []byte("\r\n")) {
				return
			}
			body := wire[:total]
			headerSize := 0
			reply := ""
			if parts[0] == "HPUB" {
				headerSize, err = strconv.Atoi(parts[len(parts)-2])
				if err != nil || headerSize > total {
					return
				}
				if len(parts) == 5 {
					reply = parts[2]
				}
			} else if len(parts) == 4 {
				reply = parts[2]
			}
			if !b.publish(c, parts[1], reply, body[headerSize:]) {
				return
			}
		default:
			return
		}
	}
}

func (c *repair961Client) message(destination, subject, reply string, body []byte) bool {
	sid := ""
	for filter, id := range c.subscriptions {
		if filter == destination || (strings.HasSuffix(filter, ">") && strings.HasPrefix(destination, strings.TrimSuffix(filter, ">"))) || (strings.HasSuffix(filter, ".*") && strings.HasPrefix(destination, strings.TrimSuffix(filter, "*"))) {
			sid = id
			break
		}
	}
	if sid == "" {
		return false
	}
	response := "MSG " + subject + " " + sid + " "
	if reply != "" {
		response += reply + " "
	}
	return c.send(response+strconv.Itoa(len(body))+"\r\n"+string(body)+"\r\n") == nil
}

func (b *repair961Broker) publish(c *repair961Client, subject, reply string, body []byte) bool {
	cfg := b.member.Probe
	switch {
	case subject == "$SYS.REQ.SERVER."+b.member.ServerID+".RELOAD":
		if c.username != "system-operator" {
			return false
		}
		config, err := os.ReadFile(b.member.ConfigPath)
		if err != nil {
			return false
		}
		var parsed struct {
			Users []string `json:"users"`
		}
		if json.Unmarshal(config, &parsed) != nil {
			return false
		}
		oldAllowed := false
		for _, user := range parsed.Users {
			if user == "epoch-three" {
				oldAllowed = true
			}
		}
		b.mu.Lock()
		b.revoked = !oldAllowed
		b.reloads++
		if b.revoked {
			for old := range b.clients {
				if old.username == "epoch-three" {
					old.send("-ERR 'Authorization Violation'\r\n")
					old.Close()
				}
			}
		}
		b.mu.Unlock()
		response, _ := json.Marshal(map[string]any{"server": map[string]any{"id": b.member.ServerID}, "data": map[string]any{"success": true}})
		return c.message(reply, reply, "", response)
	case subject == cfg.Subject:
		b.mu.Lock()
		b.sequence++
		sequence := b.sequence
		b.messages = append(b.messages, append([]byte(nil), body...))
		b.mu.Unlock()
		response, _ := json.Marshal(map[string]any{"stream": cfg.Stream, "seq": sequence})
		return c.message(reply, reply, "", response)
	case subject == "$JS.API.CONSUMER.INFO."+cfg.Stream+"."+cfg.Durable:
		config, _ := cfg.BrokerConsumerConfig()
		b.mu.Lock()
		pending := len(b.messages) - b.acked
		seq := b.sequence
		b.mu.Unlock()
		response, _ := json.Marshal(jetstream.ConsumerInfo{Stream: cfg.Stream, Name: cfg.Durable, Created: time.Now().UTC(), Config: config, NumAckPending: pending, Delivered: jetstream.SequenceInfo{Stream: seq, Consumer: seq}})
		return c.message(reply, reply, "", response)
	case subject == "$JS.API.CONSUMER.MSG.NEXT."+cfg.Stream+"."+cfg.Durable:
		b.mu.Lock()
		if len(b.messages) == 0 {
			b.mu.Unlock()
			return false
		}
		message := append([]byte(nil), b.messages[len(b.messages)-1]...)
		seq := b.sequence
		b.mu.Unlock()
		ack := fmt.Sprintf("$JS.ACK.%s.%s.1.%d.%d.%d.0", cfg.Stream, cfg.Durable, seq, seq, time.Now().UnixNano())
		return c.message(reply, cfg.Subject, ack, message)
	case strings.HasPrefix(subject, "$JS.ACK."):
		b.mu.Lock()
		b.acked++
		b.mu.Unlock()
		return c.message(reply, reply, "", []byte(""))
	default:
		return false
	}
}

func repair961Plan() EpochRecoveryPlan {
	return EpochRecoveryPlan{Proposal: "actual-cutover", Actor: "fixture-operator", Epoch: 3, Region: "region-primary", Revision: 2}
}

func repair961Controller(t *testing.T, path string, members []EpochMemberAuthority) *BrokerEpochController {
	t.Helper()
	controller, err := NewJournalBrokerEpochController(path, repair961Plan(), members)
	if err != nil {
		t.Fatal(err)
	}
	return controller
}

func repair961Read(t *testing.T, path string) *epochFenceRecord {
	t.Helper()
	wire, err := readEpochJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	var record epochFenceRecord
	if err = json.Unmarshal(wire, &record); err != nil {
		t.Fatal(err)
	}
	if record.Result != nil {
		record.Result.ProofSHA256 = record.ProofSHA256
	}
	return &record
}

func repair961AssertProof(t *testing.T, path string, result EpochFenceResult, members []EpochMemberAuthority, brokers []*repair961Broker) {
	t.Helper()
	record := repair961Read(t, path)
	wire, err := json.Marshal(result)
	if err != nil || record.State != "verified" || record.BoundProof != result.ProofSHA256 || epochSHA(wire) != result.ProofSHA256 || record.ProofSHA256 != result.ProofSHA256 || len(result.Evidence) != len(members) || len(record.Members) != len(members) {
		t.Fatalf("unconsumed or incomplete durable proof: %+v %v", record, err)
	}
	for i, member := range members {
		progress := record.Members[member.Name]
		if progress == nil || progress.Stage != "verified" || len(progress.Acks) != len(member.Credentials) {
			t.Fatalf("missing member transition: %+v", progress)
		}
		for _, e := range progress.Evidence {
			if !e.AuthenticationDenied || !e.PublishDenied || !e.AckDenied || e.ConfigurationSHA256 != member.ReplacementSHA256 || e.ServerID != member.ServerID {
				t.Fatalf("incomplete denial: %+v", e)
			}
		}
		config, err := os.ReadFile(member.ConfigPath)
		if err != nil || epochSHA(config) != member.ReplacementSHA256 {
			t.Fatal("revoked configuration restored", err)
		}
		if conn, err := connectEpochCredential(member, member.Credentials[0]); !errors.Is(err, nats.ErrAuthorization) {
			if conn != nil {
				conn.Close()
			}
			t.Fatal("old authentication survived", err)
		}
		brokers[i].mu.Lock()
		acked := brokers[i].acked
		messages := len(brokers[i].messages)
		firstMessage := append([]byte(nil), brokers[i].messages[0]...)
		denied := brokers[i].denied
		brokers[i].mu.Unlock()
		expectedProbe := "epoch-" + epochSHA([]byte("actual-cutover:"+member.Name+":"+member.Credentials[0].ID))
		if string(firstMessage) != expectedProbe {
			t.Fatalf("consumer probe was not emitted by production producer: %q", firstMessage)
		}
		if acked != 0 || messages != 1 || denied < 4 {
			t.Fatalf("old publish/ack reached broker: acked=%d messages=%d denied=%d", acked, messages, denied)
		}
		current, err := nats.Connect(member.URL, nats.UserInfo("epoch-four", "test-epoch-four-secret"), nats.NoReconnect())
		if err != nil {
			t.Fatal(err)
		}
		publisher, err := NewPublisher(current, member.Probe.Stream)
		if err != nil {
			current.Close()
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		ack, err := publisher.Publish(ctx, member.Probe.Subject, "current-epoch", []byte("current-epoch-message"))
		cancel()
		current.Close()
		if err != nil || ack.Sequence != 2 {
			t.Fatalf("current producer failed: %+v %v", ack, err)
		}
	}
}

func TestRepairM_961_RuntimeWiring(t *testing.T) {
	member, broker := repair961Fixture(t, "member-one")
	path := filepath.Join(t.TempDir(), "recovery.json")
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	controller := repair961Controller(t, path, []EpochMemberAuthority{member})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := controller.FenceAndBind(ctx, "actual-cutover", "unauthorized-operator"); err == nil {
		t.Fatal("unauthorized operation admitted")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unauthorized operation wrote journal")
	}
	config, _ := os.ReadFile(member.ConfigPath)
	if epochSHA(config) != member.ExistingSHA256 {
		t.Fatal("unauthorized operation revoked broker")
	}
	result, err := controller.FenceAndBind(ctx, "actual-cutover", "fixture-operator")
	if err != nil {
		t.Fatal(err)
	}
	repair961AssertProof(t, path, result, []EpochMemberAuthority{member}, []*repair961Broker{broker})
	reopened := repair961Controller(t, path, []EpochMemberAuthority{member})
	if _, err = reopened.FenceAndBind(ctx, "other-proposal", "fixture-operator"); err == nil {
		t.Fatal("different proposal consumed proof")
	}
	broker.mu.Lock()
	beforeDenials := broker.denied
	beforeReloads := broker.reloads
	broker.mu.Unlock()
	refreshed, err := reopened.FenceAndBind(ctx, "actual-cutover", "fixture-operator")
	if err != nil {
		t.Fatal("verified journal could not reopen", err)
	}
	durable := repair961Read(t, path)
	if durable.BoundProof != refreshed.ProofSHA256 || durable.State != "verified" {
		t.Fatal("reopened proof not consumed")
	}
	broker.mu.Lock()
	afterDenials := broker.denied
	afterReloads := broker.reloads
	messages := len(broker.messages)
	broker.mu.Unlock()
	if afterDenials-beforeDenials != 3 || afterReloads-beforeReloads != 1 || messages != 2 {
		t.Fatal("verified member was trusted without repeating live denial verification")
	}
}

func TestRepairM_961_RestartRecovery(t *testing.T) {
	for _, stage := range []string{"published", "reloaded", "verified"} {
		t.Run(stage, func(t *testing.T) {
			first, b1 := repair961Fixture(t, "member-one")
			second, b2 := repair961Fixture(t, "member-two")
			members := []EpochMemberAuthority{first, second}
			path := filepath.Join(t.TempDir(), "recovery.json")
			if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			controller := repair961Controller(t, path, members)
			interruption := &struct{}{}
			controller.checkpoint = func(member, point string) error {
				if member == "member-one" && point == stage {
					panic(interruption)
				}
				return nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			func() {
				defer func() {
					if recovered := recover(); recovered != interruption {
						t.Fatalf("wrong interruption: %v", recovered)
					}
				}()
				_, _ = controller.FenceAndBind(ctx, "actual-cutover", "fixture-operator")
				t.Fatal("interruption not reached")
			}()
			before := repair961Read(t, path)
			if before.State != "applying" || before.Result != nil || before.BoundProof != "" || before.Members[first.Name] == nil || before.Members[second.Name] != nil {
				t.Fatalf("partial state certified: %+v", before)
			}
			expected := "prepared"
			if stage == "verified" {
				expected = "verified"
			}
			if before.Members[first.Name].Stage != expected {
				t.Fatalf("progress=%s wanted=%s", before.Members[first.Name].Stage, expected)
			}
			controller = nil
			// Drop every component-owned connection and reconstruct all authorities,
			// preserving only the durable journal and brokers' published configuration.
			first.Control.Close()
			second.Control.Close()
			for i := range members {
				conn, err := nats.Connect(members[i].URL, nats.UserInfo("system-operator", "test-system-secret"), nats.NoReconnect())
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				members[i].Control = conn
			}
			reopened := repair961Controller(t, path, []EpochMemberAuthority{members[1], members[0]})
			result, err := reopened.FenceAndBind(ctx, "actual-cutover", "fixture-operator")
			if err != nil {
				t.Fatal(err)
			}
			repair961AssertProof(t, path, result, members, []*repair961Broker{b1, b2})
		})
	}
}

func TestRepairM_961_RejectChangedAuthorityAndConfig(t *testing.T) {
	member, broker := repair961Fixture(t, "member-one")
	path := filepath.Join(t.TempDir(), "recovery.json")
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	controller := repair961Controller(t, path, []EpochMemberAuthority{member})
	interrupted := errors.New("operator interrupted")
	controller.checkpoint = func(_, stage string) error {
		if stage == "published" {
			return interrupted
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := controller.FenceAndBind(ctx, "actual-cutover", "fixture-operator"); !errors.Is(err, interrupted) {
		t.Fatal("expected publication interruption", err)
	}
	record := repair961Read(t, path)
	if record.State != "failed" || record.Result != nil || record.BoundProof != "" {
		t.Fatal("failed operation certified")
	}
	changed := member
	changed.Credentials = append([]EpochCredential(nil), member.Credentials...)
	changed.Credentials[0].Password = "changed-reviewed-authority"
	other := repair961Controller(t, path, []EpochMemberAuthority{changed})
	if _, err := other.FenceAndBind(ctx, "actual-cutover", "fixture-operator"); err == nil {
		t.Fatal("changed credential inventory resumed")
	}
	if err := os.WriteFile(member.ConfigPath, []byte("unreviewed configuration"), 0600); err != nil {
		t.Fatal(err)
	}
	reopened := repair961Controller(t, path, []EpochMemberAuthority{member})
	if _, err := reopened.FenceAndBind(ctx, "actual-cutover", "fixture-operator"); err == nil {
		t.Fatal("unreviewed replacement resumed")
	}
	broker.mu.Lock()
	reloads := broker.reloads
	broker.mu.Unlock()
	if reloads != 0 {
		t.Fatal("rejected authority reloaded broker")
	}
	record = repair961Read(t, path)
	if record.State != "failed" || record.BoundProof != "" {
		t.Fatal("rejected configuration consumed proof")
	}
}

func TestRepairM_961_UnavailableBrokerCannotCertify(t *testing.T) {
	member, broker := repair961Fixture(t, "member-one")
	path := filepath.Join(t.TempDir(), "recovery.json")
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	controller := repair961Controller(t, path, []EpochMemberAuthority{member})
	controller.checkpoint = func(_, stage string) error {
		if stage == "reloaded" {
			return broker.listener.Close()
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := controller.FenceAndBind(ctx, "actual-cutover", "fixture-operator"); err == nil {
		t.Fatal("network unavailability certified as credential denial")
	}
	record := repair961Read(t, path)
	if record.State != "failed" || record.Result != nil || record.BoundProof != "" || record.Members[member.Name].Stage != "prepared" {
		t.Fatal("unavailable broker produced aggregate success")
	}
}

func TestRepairM_961_ConcurrentAndPrivateJournal(t *testing.T) {
	member, _ := repair961Fixture(t, "member-one")
	path := filepath.Join(t.TempDir(), "recovery.json")
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	controller := repair961Controller(t, path, []EpochMemberAuthority{member})
	other := repair961Controller(t, path, []EpochMemberAuthority{member})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	controller.checkpoint = func(_, stage string) error {
		if stage == "published" {
			if _, err := other.FenceAndBind(ctx, "actual-cutover", "fixture-operator"); err == nil {
				t.Fatal("concurrent journal writer admitted")
			}
		}
		return nil
	}
	if _, err := controller.FenceAndBind(ctx, "actual-cutover", "fixture-operator"); err != nil {
		t.Fatal(err)
	}
	wire, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(wire, []byte(member.Credentials[0].Password)) || bytes.Contains(wire, member.Replacement) {
		t.Fatal("private credentials copied into journal")
	}
	target := filepath.Join(filepath.Dir(path), "unrelated.json")
	if err = os.WriteFile(target, []byte("unrelated private evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(filepath.Dir(path), "alias.json")
	if err = os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
	linked := repair961Controller(t, alias, []EpochMemberAuthority{member})
	if _, err = linked.FenceAndBind(ctx, "actual-cutover", "fixture-operator"); err == nil {
		t.Fatal("symlink journal followed")
	}
	actual, err := os.ReadFile(target)
	if err != nil || string(actual) != "unrelated private evidence" {
		t.Fatal("symlink target overwritten")
	}
}
