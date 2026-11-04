package bringup

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tmed "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	coretypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"golang.org/x/crypto/ssh"
)

func m796Read(t *testing.T, p string) []byte {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func m796Write(t *testing.T, p string, b []byte) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func m796SHA(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func m796CLI(t *testing.T, args []string, input []byte) (string, error) {
	t.Helper()
	var out bytes.Buffer
	e := RunInspector(context.Background(), args, bytes.NewReader(input), &out)
	return out.String(), e
}
func m796Wire(t *testing.T, p string, terms ...string) {
	t.Helper()
	raw := string(m796Read(t, p))
	for _, term := range terms {
		if !strings.Contains(raw, term) {
			t.Fatalf("%s missing production wiring %s", p, term)
		}
	}
}

func TestRepairM_796_FleetTransportDeadlines(t *testing.T) {
	limits := FleetLimits{Connect: 150 * time.Millisecond, Transfer: 400 * time.Millisecond, Recovery: time.Second}
	payload := []byte("source-bound disposable paxd release\n")
	asset := "paxd-v796-linux-amd64"
	for _, fault := range []string{"", "headers", "body", "digest", "duplicate"} {
		t.Run(fault, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if fault == "headers" {
					<-r.Context().Done()
					return
				}
				if r.URL.Path == "/SHA256SUMS" {
					line := m796SHA(payload) + "  " + asset + "\n"
					if fault == "duplicate" {
						line += line
					}
					io.WriteString(w, line)
					return
				}
				if r.URL.Path != "/"+asset {
					t.Error("release asset path drift")
					w.WriteHeader(404)
					return
				}
				if fault == "body" {
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
					<-r.Context().Done()
					return
				}
				if fault == "digest" {
					io.WriteString(w, "corrupt")
					return
				}
				w.Write(payload)
			}))
			defer server.Close()
			directory := filepath.Join(t.TempDir(), "release")
			start := time.Now()
			path, sha, e := FetchFleetRelease(context.Background(), limits, server.URL, "v796", directory)
			if fault == "" {
				if e != nil || sha != m796SHA(payload) || !bytes.Equal(m796Read(t, path), payload) {
					t.Fatalf("release producer -> checksum -> publication: %s %v", sha, e)
				}
			} else {
				if e == nil {
					t.Fatal("invalid/incomplete release published")
				}
				if _, e = os.Stat(filepath.Join(directory, asset)); !errors.Is(e, os.ErrNotExist) {
					t.Fatal("failed transfer left a published asset")
				}
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("release deadline not enforced")
			}
		})
	}
	// A TCP peer which never completes the SSH handshake is bounded by the
	// connection deadline, independently of the transfer budget.
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, e := listener.Accept()
		if e == nil {
			accepted <- c
		}
	}()
	transport := FleetSSH{Limits: limits, Config: func(string) (*ssh.ClientConfig, error) {
		return &ssh.ClientConfig{User: "source", HostKeyCallback: ssh.FixedHostKey(m796SSHKey(t).PublicKey())}, nil
	}}
	start := time.Now()
	if e = transport.Run(context.Background(), listener.Addr().String(), "true", nil, io.Discard); e == nil {
		t.Fatal("stalled handshake accepted")
	}
	if time.Since(start) > time.Second {
		t.Fatal("connection deadline not enforced")
	}
	(<-accepted).Close()
	peer := m796FleetPeer(t, "command", true)
	transport = peer.transport(limits)
	start = time.Now()
	if e = transport.Run(context.Background(), "source@"+peer.address, fleetScript("command", FleetResyncPlan{}, "sleep 100"), nil, io.Discard); e == nil {
		t.Fatal("stalled mutation accepted")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("mutation deadline not enforced")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e = transport.Run(ctx, "source@"+peer.address, "true", nil, io.Discard); e == nil {
		t.Fatal("canceled transfer admitted")
	}
	m796Wire(t, "sync-fleet.sh", "fleet-release", "fleet-resync", "fleet-recover", "fleet_scp", "timeout --signal=TERM")
	m796Wire(t, "cmd/bringup-inspect/main.go", "signal.NotifyContext")
}

func m796SSHKey(t *testing.T) ssh.Signer {
	t.Helper()
	_, private, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	signer, e := ssh.NewSignerFromKey(private)
	if e != nil {
		t.Fatal(e)
	}
	return signer
}

// The loopback peer owns disposable on-disk service generations. The real
// FleetOwner and SSH transport produce every request, stream real tar bytes,
// journal before mutation and recover by reopening their actual durable store.
// Only the remote systemd/SSH host is a peer; the owner is never substituted.
type m796Service struct {
	dir    string
	server *httptest.Server
}

func (s *m796Service) start() error {
	if s.server != nil {
		return nil
	}
	binary, e := os.ReadFile(filepath.Join(s.dir, "binary"))
	if e != nil {
		return e
	}
	data, e := os.ReadFile(filepath.Join(s.dir, "data"))
	if e != nil {
		return e
	}
	generation := m796SHA(binary)
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/running" {
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"executable": generation, "committed": m796SHA(data)})
	}))
	return nil
}
func (s *m796Service) stop() {
	if s.server != nil {
		s.server.Close()
		s.server = nil
	}
}
func (s *m796Service) running() (string, error) {
	if s.server == nil {
		return "-", nil
	}
	response, e := s.server.Client().Get(s.server.URL + "/running")
	if e != nil {
		return "", e
	}
	defer response.Body.Close()
	var wire struct{ Executable string }
	if response.StatusCode != 200 {
		return "", errors.New("service unavailable")
	}
	e = json.NewDecoder(response.Body).Decode(&wire)
	return wire.Executable, e
}

type m796Peer struct {
	t         *testing.T
	address   string
	key       ssh.Signer
	mu        sync.Mutex
	services  map[string]*m796Service
	block     string
	entered   chan struct{}
	once      sync.Once
	denyStart bool
}

func m796FleetPeer(t *testing.T, block string, targetActive bool) *m796Peer {
	t.Helper()
	p := &m796Peer{t: t, key: m796SSHKey(t), services: map[string]*m796Service{}, block: block, entered: make(chan struct{})}
	for _, role := range []string{"source", "target"} {
		dir := filepath.Join(t.TempDir(), role)
		m796Write(t, filepath.Join(dir, "binary"), []byte("old-"+role))
		m796Write(t, filepath.Join(dir, "data"), []byte("committed-"+role))
		active := role == "source" || targetActive
		service := &m796Service{dir: dir}
		p.services[role] = service
		if active {
			if e := service.start(); e != nil {
				t.Fatal(e)
			}
		}
		t.Cleanup(service.stop)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	p.address = listener.Addr().String()
	t.Cleanup(func() { listener.Close() })
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(p.key)
	go func() {
		for {
			conn, e := listener.Accept()
			if e != nil {
				return
			}
			go func() {
				defer conn.Close()
				server, channels, requests, e := ssh.NewServerConn(conn, config)
				if e != nil {
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				closed := make(chan struct{})
				go func() { server.Wait(); close(closed) }()
				for next := range channels {
					if next.ChannelType() != "session" {
						next.Reject(ssh.UnknownChannelType, "session required")
						continue
					}
					channel, reqs, e := next.Accept()
					if e != nil {
						return
					}
					go func() {
						defer channel.Close()
						for request := range reqs {
							if request.Type != "exec" {
								request.Reply(false, nil)
								continue
							}
							var wire struct{ Command string }
							if ssh.Unmarshal(request.Payload, &wire) != nil {
								request.Reply(false, nil)
								return
							}
							request.Reply(true, nil)
							e := p.execute(server.User(), wire.Command, channel, closed)
							status := uint32(0)
							if e != nil {
								status = 1
							}
							channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
							return
						}
					}()
				}
			}()
		}
	}()
	return p
}
func (p *m796Peer) transport(l FleetLimits) FleetSSH {
	return FleetSSH{Limits: l, Config: func(dest string) (*ssh.ClientConfig, error) {
		role, _, ok := strings.Cut(dest, "@")
		if !ok {
			return nil, errors.New("role required")
		}
		return &ssh.ClientConfig{User: role, HostKeyCallback: ssh.FixedHostKey(p.key.PublicKey())}, nil
	}}
}
func (p *m796Peer) execute(role, command string, ch ssh.Channel, closed <-chan struct{}) error {
	prefix, quoted, ok := strings.Cut(command, " sh -c ")
	if !ok || !strings.HasPrefix(prefix, "timeout --signal=TERM --kill-after=5s ") {
		return errors.New("unbounded remote descendants")
	}
	if len(quoted) < 2 || quoted[0] != '\'' || quoted[len(quoted)-1] != '\'' {
		return errors.New("unquoted remote body")
	}
	script := strings.ReplaceAll(quoted[1:len(quoted)-1], "'\"'\"'", "'")
	first, body, ok := strings.Cut(script, "\n")
	if !ok || !strings.HasPrefix(first, "# bringup-fleet ") {
		return errors.New("operation binding absent")
	}
	var request struct {
		Operation string
		Plan      FleetResyncPlan
	}
	if json.Unmarshal([]byte(strings.TrimPrefix(first, "# bringup-fleet ")), &request) != nil {
		return errors.New("operation malformed")
	}
	op := request.Operation
	if op == "command" {
		p.once.Do(func() { close(p.entered) })
		<-closed
		return errors.New("command canceled")
	}
	p.mu.Lock()
	s := p.services[role]
	if s == nil {
		p.mu.Unlock()
		return errors.New("foreign service")
	}
	write := func(name string, b []byte) error { return os.WriteFile(filepath.Join(s.dir, name), b, 0600) }
	read := func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(s.dir, name)) }
	var e error
	switch op {
	case "state":
		if !strings.Contains(body, "/proc/$pid/exe") {
			e = errors.New("running executable not qualified")
			break
		}
		binary, err := read("binary")
		if err != nil {
			e = err
			break
		}
		state, running := "inactive", "-"
		if s.server != nil {
			state = "active"
			running, e = s.running()
			if e != nil {
				break
			}
		}
		_, e = fmt.Fprintf(ch, "%s\n%s\n%s\n", state, m796SHA(binary), running)
	case "stop":
		s.stop()
	case "start":
		if p.denyStart {
			e = errors.New("service start refused")
			break
		}
		e = s.start()
	case "copy-source":
		if s.server != nil {
			e = errors.New("copy of running source")
			break
		}
		if !strings.Contains(body, "--exclude=priv_validator_state.json") {
			e = errors.New("validator state exposed")
			break
		}
		if p.block == op {
			p.once.Do(func() { close(p.entered) })
			p.mu.Unlock()
			<-closed
			return errors.New("copy interrupted")
		}
		data, err := read("data")
		if err != nil {
			e = err
			break
		}
		p.mu.Unlock()
		archive := tar.NewWriter(ch)
		e = archive.WriteHeader(&tar.Header{Name: "committed-state", Mode: 0600, Size: int64(len(data))})
		if e == nil {
			_, e = archive.Write(data)
		}
		if closeError := archive.Close(); e == nil {
			e = closeError
		}
		return e
	case "copy-target":
		p.mu.Unlock()
		archive := tar.NewReader(ch)
		header, err := archive.Next()
		if err != nil {
			return err
		}
		if header.Name != "committed-state" {
			return errors.New("foreign data entry")
		}
		data, err := io.ReadAll(archive)
		if err != nil {
			return err
		}
		if _, err = archive.Next(); err != io.EOF {
			return errors.New("extra tar member")
		}
		p.mu.Lock()
		e = write("incoming", data)
	case "stage":
		p.mu.Unlock()
		data, err := io.ReadAll(ch)
		p.mu.Lock()
		if err != nil {
			e = err
			break
		}
		if m796SHA(data) != request.Plan.SHA256 {
			e = errors.New("wrong staged binary")
			break
		}
		e = write("staged", data)
	case "swap":
		if !strings.Contains(body, "data.stale-") || !strings.Contains(body, "priv_validator_state.json") {
			e = errors.New("swap recovery material absent")
			break
		}
		s.stop()
		binary, err := read("binary")
		if err != nil {
			e = err
			break
		}
		data, err := read("data")
		if err != nil {
			e = err
			break
		}
		if e = write("backup", binary); e != nil {
			break
		}
		if e = write("stale", data); e != nil {
			break
		}
		staged, err := read("staged")
		if err != nil {
			e = err
			break
		}
		incoming, err := read("incoming")
		if err != nil {
			e = err
			break
		}
		if e = write("binary", staged); e != nil {
			break
		}
		if e = write("data", incoming); e != nil {
			break
		}
		if p.block == op {
			p.once.Do(func() { close(p.entered) })
			p.mu.Unlock()
			<-closed
			return errors.New("swap interrupted")
		}
	case "rollback":
		s.stop()
		for name, dest := range map[string]string{"backup": "binary", "stale": "data"} {
			raw, err := read(name)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				e = err
				break
			}
			if e = write(dest, raw); e != nil {
				break
			}
		}
	default:
		e = errors.New("unexpected operation")
	}
	p.mu.Unlock()
	return e
}
func m796Owner(t *testing.T, root string, peer *m796Peer) *FleetOwner {
	t.Helper()
	owner, e := OpenFleetOwner(root, peer.transport(FleetLimits{Connect: time.Second, Transfer: 4 * time.Second, Recovery: 2 * time.Second}))
	if e != nil {
		t.Fatal(e)
	}
	return owner
}
func m796Plan(t *testing.T, peer *m796Peer) FleetResyncPlan {
	t.Helper()
	asset := filepath.Join(t.TempDir(), "paxd")
	raw := []byte("new-qualified-release")
	m796Write(t, asset, raw)
	return FleetResyncPlan{Source: "source@" + peer.address, Target: "target@" + peer.address, Home: "/var/lib/paxd", Binary: "/usr/local/bin/paxd", Asset: asset, SHA256: m796SHA(raw), ID: "repair796"}
}
func m796ServiceState(t *testing.T, peer *m796Peer, role string, active bool, binary, data string) {
	t.Helper()
	peer.mu.Lock()
	defer peer.mu.Unlock()
	s := peer.services[role]
	if (s.server != nil) != active || string(m796Read(t, filepath.Join(s.dir, "binary"))) != binary || string(m796Read(t, filepath.Join(s.dir, "data"))) != data {
		t.Fatalf("%s lost its running state/version/data: %+v", role, s)
	}
	if active {
		running, e := s.running()
		if e != nil || running != m796SHA([]byte(binary)) {
			t.Fatalf("%s running service exposes a different executable: %s %v", role, running, e)
		}
	}
}
func TestRepairM_796_FleetInterruptionRecovery(t *testing.T) {
	for _, phase := range []string{"copy-source", "swap"} {
		for _, active := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/target-active-%t", phase, active), func(t *testing.T) {
				peer := m796FleetPeer(t, phase, active)
				root := filepath.Join(t.TempDir(), "owner")
				owner := m796Owner(t, root, peer)
				plan := m796Plan(t, peer)
				if _, e := OpenFleetOwner(root, peer.transport(owner.ssh.Limits)); e == nil {
					t.Fatal("second live owner admitted")
				}
				result := make(chan error, 1)
				go func() { result <- owner.Resync(context.Background(), plan) }()
				select {
				case <-peer.entered:
				case <-time.After(5 * time.Second):
					t.Fatal("did not reach interruption point")
				}
				if _, e := os.Stat(filepath.Join(root, "pending.json")); e != nil {
					t.Fatal("mutation preceded durable journal", e)
				}
				if e := owner.Close(); e != nil {
					t.Fatal(e)
				}
				select {
				case e := <-result:
					if e == nil {
						t.Fatal("interrupted owner reported success")
					}
				case <-time.After(3 * time.Second):
					t.Fatal("owner cancellation did not reach streamed children")
				}
				if phase == "copy-source" {
					peer.mu.Lock()
					stopped := peer.services["source"].server == nil
					peer.mu.Unlock()
					if !stopped {
						t.Fatal("case did not stop the source")
					}
				}
				reopened := m796Owner(t, root, peer)
				if e := reopened.Recover(context.Background()); e != nil {
					t.Fatal(e)
				}
				m796ServiceState(t, peer, "source", true, "old-source", "committed-source")
				m796ServiceState(t, peer, "target", active, "old-target", "committed-target")
				if _, e := os.Stat(filepath.Join(root, "pending.json")); !errors.Is(e, os.ErrNotExist) {
					t.Fatal("reconciled journal retained", e)
				}
				peer.mu.Lock()
				peer.block = ""
				peer.mu.Unlock()
				plan.ID = "afterreopen"
				if e := reopened.Resync(context.Background(), plan); e != nil {
					t.Fatal("resync after reopen", e)
				}
				if e := reopened.Close(); e != nil {
					t.Fatal(e)
				}
				reopened = m796Owner(t, root, peer)
				defer reopened.Close()
				if e := reopened.Recover(context.Background()); e != nil {
					t.Fatal(e)
				}
				m796ServiceState(t, peer, "source", true, "old-source", "committed-source")
				m796ServiceState(t, peer, "target", active, "new-qualified-release", "committed-source")
			})
		}
	}
	// Refused source restoration leaves the durable operation for the next
	// owner and prevents admitting an unrelated replacement plan.
	peer := m796FleetPeer(t, "", true)
	root := filepath.Join(t.TempDir(), "owner")
	owner := m796Owner(t, root, peer)
	plan := m796Plan(t, peer)
	if _, e := owner.prepare(context.Background(), plan); e != nil {
		t.Fatal(e)
	}
	owner.Close()
	peer.mu.Lock()
	peer.denyStart = true
	peer.mu.Unlock()
	owner = m796Owner(t, root, peer)
	defer owner.Close()
	if e := owner.Recover(context.Background()); e == nil {
		t.Fatal("failed recovery accepted")
	}
	plan.ID = "another"
	if _, e := owner.prepare(context.Background(), plan); e == nil {
		t.Fatal("new mutation admitted before recovery")
	}
	if _, e := os.Stat(filepath.Join(root, "pending.json")); e != nil {
		t.Fatal("failed recovery erased journal")
	}
	// A real running service retains the executable it started with even
	// when another actor replaces the file on disk. State admission detects it.
	versionPeer := m796FleetPeer(t, "", true)
	versionOwner := m796Owner(t, filepath.Join(t.TempDir(), "owner"), versionPeer)
	defer versionOwner.Close()
	versionPlan := m796Plan(t, versionPeer)
	versionPeer.mu.Lock()
	binaryPath := filepath.Join(versionPeer.services["source"].dir, "binary")
	versionPeer.mu.Unlock()
	m796Write(t, binaryPath, []byte("replacement-on-disk"))
	if _, _, e := versionOwner.state(context.Background(), versionPlan.Source, versionPlan); e == nil {
		t.Fatal("old running executable hidden by updated on-disk binary")
	}
	m796Wire(t, "sync-fleet.sh", "fleet-resync", "fleet-recover")
}

func TestRepairM_796_CAApprovedRoster(t *testing.T) {
	approved := m796Read(t, "tests/fixtures/repair_m794/approved-roster.tsv")
	rows, e := ReadServiceRoster(bytes.NewReader(approved))
	if e != nil {
		t.Fatal(e)
	}
	base, e := ReadServiceRoster(bytes.NewReader(m796Read(t, "ca-services.tsv")))
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 30 || len(base) != len(rows) {
		t.Fatal("independently approved identity membership differs")
	}
	names := map[string]string{}
	for _, line := range strings.Split(string(m796Read(t, "railway-names.env")), "\n") {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatal("name declaration malformed")
		}
		names[key] = value
	}
	for i, row := range rows {
		expected := base[i]
		extra, ok := names[strings.ToUpper(strings.ReplaceAll(row.Name, "-", "_"))]
		if !ok {
			t.Fatalf("missing name of %s", row.Name)
		}
		if extra != "" {
			if expected.SANs == "-" {
				expected.SANs = extra
			} else {
				expected.SANs += "," + extra
			}
		}
		if row != expected {
			t.Fatalf("all six approved fields must agree: %+v != %+v", row, expected)
		}
		bundle := IdentityBundle{}
		for _, file := range identityFiles {
			bundle[file.File] = m796Read(t, filepath.Join("tests/fixtures/repair_m794", row.Name, file.File))
		}
		if e = BundleConsistent(bundle); e != nil {
			t.Fatalf("actual CA issuance %s: %v", row.Name, e)
		}
		// Exercise the actual Railway base64 transport and remote report consumer.
		values := map[string]string{}
		for _, file := range identityFiles {
			values["FIXTURE_"+file.Suffix] = base64.StdEncoding.EncodeToString(bundle[file.File])
		}
		wire, _ := json.Marshal(values)
		decoded, e := ReadRailwayBundle(bytes.NewReader(wire), "FIXTURE")
		if e != nil {
			t.Fatal(e)
		}
		var report bytes.Buffer
		if e = BundleReport(decoded, &report); e != nil {
			t.Fatal(e)
		}
		state := InspectBundleReport(&report, bundle["ca.pem"], time.Now(), row.CN, row.EKU, row.SANs)
		if state.State != "present" {
			t.Fatalf("CA -> transport -> certificate consumer %s: %+v", row.Name, state)
		}
		if state = CertificateInventory(bundle["cert.pem"], bundle["ca.pem"], time.Now(), 0, row.CN+"-foreign", row.EKU, row.SANs); state.State != "incompatible" {
			t.Fatal("foreign common name accepted")
		}
		if row.EKU == "serverAuth" {
			if state = CertificateInventory(bundle["cert.pem"], bundle["ca.pem"], time.Now(), 0, row.CN, row.EKU, "DNS:foreign.invalid"); state.State != "incompatible" {
				t.Fatal("wrong SAN accepted")
			}
		}
	}
	for _, bad := range []string{string(approved) + strings.Split(string(approved), "\n")[0] + "\n", "receipt-authority box:KERNEL_HOST volume retired serverAuth -\n", "identity railway:identity IDENTITY_TLS identity serverAuth\n"} {
		if _, e = ReadServiceRoster(strings.NewReader(bad)); e == nil {
			t.Fatal("malformed/retired identity roster accepted")
		}
	}
	m796Wire(t, "tests/ca.test.sh", "approved-roster.tsv", "diff -u", "NF != 6", "openssl")
}

func TestRepairM_796_EdgeAggregateRoutes(t *testing.T) {
	inventory, e := ReadPublicInventory(bytes.NewReader(m796Read(t, "../../docs/site/docs/reference/public-rpc.md")))
	if e != nil {
		t.Fatal(e)
	}
	if len(inventory.Nodes) != 14 {
		t.Fatal("approved node inventory drift")
	}
	pool := filepath.Join(t.TempDir(), "pool")
	var nodeNames []string
	for _, url := range inventory.Nodes {
		nodeNames = append(nodeNames, strings.TrimPrefix(url, "https://"))
	}
	m796Write(t, pool, []byte(strings.Join(nodeNames, "\n")+"\n"))
	down := filepath.Join(t.TempDir(), "down")
	m796Write(t, down, nil)
	var manifest, sites strings.Builder
	for _, item := range [][]string{{"gateway.paxeer.network", "railway", "gateway-beta.up.railway.app:443", "-"}, {"hooks.paxeer.network", "box", "10.20.0.6:8443", "hooks.box.test"}, {"machine.paxeer.network", "stream", "10.20.0.7:9454", "-"}} {
		line, e := m796CLI(t, append([]string{"edge-map"}, item...), nil)
		if e != nil {
			t.Fatal(e)
		}
		manifest.WriteString(line)
		if item[1] != "stream" {
			body, e := m796CLI(t, []string{"edge-http", item[0], item[1], item[2], item[3], "tls", "/var/www/certbot", "/etc/letsencrypt/live", "127.0.0.53", "1"}, nil)
			if e != nil {
				t.Fatal(e)
			}
			sites.WriteString(body)
		}
	}
	stream, e := m796CLI(t, []string{"edge-stream", "9454", "127.0.0.53"}, []byte(manifest.String()))
	if e != nil {
		t.Fatal(e)
	}
	sites.WriteString(stream)
	rpc, e := m796CLI(t, []string{"edge-rpc", "api-mainnet-beta.paxeer.network", "tls", "/var/www/certbot", "/etc/letsencrypt/live", "127.0.0.53", "1", pool, down}, nil)
	if e != nil {
		t.Fatal(e)
	}
	sites.WriteString(rpc)
	manifest.WriteString("api-mainnet-beta.paxeer.network rpc - 443 rpc -\n")
	frame := manifest.String() + "@@sites\n" + sites.String()
	members, e := m796CLI(t, []string{"edge-contract", "api-mainnet-beta.paxeer.network", "443"}, []byte(frame))
	if e != nil || members != strings.Join(nodeNames, "\n")+"\n" {
		t.Fatalf("render -> manifest -> current consumer %s %v", members, e)
	}
	for _, term := range []string{"proxy_ssl_verify on;", "proxy_ssl_server_name on;", "proxy_ssl_name gateway-beta.up.railway.app;", "proxy_set_header Host gateway-beta.up.railway.app;", "proxy_set_header X-Forwarded-Host gateway.paxeer.network;", "proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;", "proxy_set_header X-Forwarded-Proto $scheme;", "proxy_set_header X-Forwarded-Host $host;"} {
		if !strings.Contains(sites.String(), term) {
			t.Fatalf("lost upstream security/header directive %s", term)
		}
	}
	for _, name := range nodeNames {
		if !strings.Contains(rpc, "proxy_ssl_name "+name+";") {
			t.Fatalf("pool member SNI absent: %s", name)
		}
	}
	for _, bad := range []string{strings.Replace(frame, "proxy_ssl_verify on;", "proxy_ssl_verify off;", 1), strings.Replace(frame, "gateway-beta.up.railway.app:443 443 railway -", "gateway-app 443", 1), strings.ReplaceAll(frame, "max_fails=2 fail_timeout=10s;", "max_fails=2 fail_timeout=10s down;")} {
		if _, e = m796CLI(t, []string{"edge-contract"}, []byte(bad)); e == nil {
			t.Fatal("legacy or unsafe rendered route accepted")
		}
	}
	m796Wire(t, "tests/edge.test.sh", "rpc-names", "api-mainnet-beta.paxeer.network", "gateway-beta.up.railway.app", "edge-contract", "edge_aggregate_headers")
}

func TestRepairM_796_FoundationArtifactInventory(t *testing.T) {
	wire, e := m796CLI(t, []string{"runtime-plan"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	plan, e := ReadRuntimePlan(strings.NewReader(wire))
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(plan.Names, []string{"paxd", "attestor", "bringup-inspect"}) {
		t.Fatal("merged artifacts differ")
	}
	if e = ValidateRuntimeSources(plan, "../.."); e != nil {
		t.Fatal("actual transitive source closure", e)
	}
	if _, e = m796CLI(t, []string{"runtime-sources", "../.."}, nil); e != nil {
		t.Fatal(e)
	}
	var graph runtimeSourceGraph
	if e = json.Unmarshal(runtimeDependencyDeclaration, &graph); e != nil {
		t.Fatal(e)
	}
	attestor := false
	for _, pkg := range graph.Packages {
		if pkg.Path == "services/wallet/attestor/cmd/attestor" && pkg.ImportPath == "github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/cmd/attestor" {
			attestor = true
		}
	}
	if !attestor {
		t.Fatal("go list graph lost actual attestor module resolution")
	}
	for _, fault := range []string{"stale-source", "stale-module", "missing-producer", "missing-closure", "missing-transitive"} {
		t.Run(fault, func(t *testing.T) {
			var bad runtimeSourceGraph
			if e := json.Unmarshal(runtimeDependencyDeclaration, &bad); e != nil {
				t.Fatal(e)
			}
			switch fault {
			case "stale-source":
				for name := range bad.Packages[0].Files {
					bad.Packages[0].Files[name] = strings.Repeat("0", 64)
					break
				}
			case "stale-module":
				bad.Manifests["go.mod"] = strings.Repeat("0", 64)
			case "missing-producer":
				bad.Producers[1] = "services/human/wallet/attestor/cmd/attestor"
			case "missing-closure":
				bad.Packages = nil
			case "missing-transitive":
				kept := bad.Packages[:0]
				for _, pkg := range bad.Packages {
					if pkg.Path != "layerxproof/verify" {
						kept = append(kept, pkg)
					}
				}
				bad.Packages = kept
			}
			encoded, e := json.Marshal(bad)
			if e != nil {
				t.Fatal(e)
			}
			if e = validateRuntimeSourceGraph(plan, "../..", encoded); e == nil {
				t.Fatal("invalid recorded import graph accepted")
			}
		})
	}
	m796Wire(t, "tests/runtime-dependencies.py", "'list', '-deps', '-json'", "package['ImportPath']", "'services/wallet/attestor'")
	altered := plan
	altered.GoPaths = nil
	for _, path := range plan.GoPaths {
		if path != "layerxproof" {
			altered.GoPaths = append(altered.GoPaths, path)
		}
	}
	if e = ValidateRuntimeSources(altered, "../.."); e == nil {
		t.Fatal("missing actual linked proof dependency accepted")
	}
	for _, change := range []func(*RuntimePlan){func(p *RuntimePlan) { p.Version = 1 }, func(p *RuntimePlan) { p.Names = []string{"paxd", "layerxd", "attestor"} }, func(p *RuntimePlan) { p.GoPaths = append(p.GoPaths, "custodyproof") }, func(p *RuntimePlan) { p.GoPaths = append(p.GoPaths, "spec/checkpoint-settlement.json") }, func(p *RuntimePlan) { p.Libraries = []string{"legacy.so", "bad.so", "old.so"} }} {
		bad := plan
		change(&bad)
		raw, _ := json.Marshal(bad)
		if _, e = ReadRuntimePlan(bytes.NewReader(raw)); e == nil {
			t.Fatal("legacy artifact/source inventory accepted")
		}
	}
	m796Wire(t, "tests/foundation-artifacts.py", "runtime-bundle.json", "runtime-sources", "source_binding(selected_revision, PATHS[name]) == binding", "record.get(\"sha256\") == sha256(path)")
	m796Wire(t, "tests/paxeer-x-fixture-foundation.py", "ART.validate(", "artifact_manifest_sha256")
}

func m796Genesis(t *testing.T) (NativeGenesis, []byte, []byte, tmed.SecretKey) {
	t.Helper()
	const fixture = "../../services/platform/hosted/node/tests/fixtures/repair_22_31_355/"
	raw := m796Read(t, fixture+"genesis.json")
	var config struct {
		Seed      string `json:"receipt_seed"`
		Validator struct {
			Private struct{ Value string } `json:"priv_key"`
		} `json:"validator_key"`
	}
	if e := json.Unmarshal(m796Read(t, fixture+"native_configuration.json"), &config); e != nil {
		t.Fatal(e)
	}
	genesis, e := ReadNativeGenesis(raw, []byte(config.Seed))
	if e != nil {
		t.Fatal("real merged paxd genesis refused", e)
	}
	private, e := base64.StdEncoding.DecodeString(config.Validator.Private.Value)
	if e != nil {
		t.Fatal(e)
	}
	key, e := tmed.SecretKeyFromSecretBytes(private)
	if e != nil {
		t.Fatal(e)
	}
	return genesis, raw, []byte(config.Seed), key
}
func m796Commit(t *testing.T, g NativeGenesis, key tmed.SecretKey, height int64, app byte) coretypes.ResultCommit {
	t.Helper()
	hash := bytes.Repeat([]byte{app}, 32)
	block := tmtypes.MakeBlock(height, nil, &tmtypes.Commit{}, nil)
	header := block.Header
	header.ChainID = g.Doc.ChainID
	header.Time = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	header.ValidatorsHash = g.Doc.ValidatorHash()
	header.NextValidatorsHash = g.Doc.ValidatorHash()
	header.AppHash = hash
	header.ConsensusHash = g.Doc.ConsensusParams.HashConsensusParams()
	header.ProposerAddress = key.Public().Address()
	id := tmtypes.BlockID{Hash: header.Hash(), PartSetHeader: tmtypes.PartSetHeader{Total: 1, Hash: hash}}
	vote := &tmtypes.Vote{Type: tmproto.PrecommitType, Height: height, Round: 0, BlockID: id, Timestamp: header.Time, ValidatorAddress: key.Public().Address(), ValidatorIndex: 0}
	vote.Signature = utils.Some(key.Sign(tmtypes.VoteSignBytes(g.Doc.ChainID, vote.ToProto())))
	commit := &tmtypes.Commit{Height: height, Round: 0, BlockID: id, Signatures: []tmtypes.CommitSig{vote.CommitSig()}}
	result := coretypes.ResultCommit{SignedHeader: tmtypes.SignedHeader{Header: &header, Commit: commit}, CanonicalCommit: true}
	if e := result.SignedHeader.ValidateBasic(g.Doc.ChainID); e != nil {
		t.Fatal(e)
	}
	if e := g.Doc.ValidatorSet().VerifyCommitLight(g.Doc.ChainID, id, height, commit); e != nil {
		t.Fatal(e)
	}
	return result
}
func TestRepairM_796_NativeGenesisInputs(t *testing.T) {
	genesis, raw, seed, key := m796Genesis(t)
	var report bytes.Buffer
	if e := WriteNativeGenesisReport(&report, genesis); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(report.String(), `"modules":["bank","sequencer","lxphost"]`) || len(genesis.ReceiptPublic) != 32 {
		t.Fatal("merged genesis consumer report differs")
	}
	path := filepath.Join(t.TempDir(), "genesis.json")
	seedPath := filepath.Join(t.TempDir(), "receipt.key")
	m796Write(t, path, raw)
	m796Write(t, seedPath, seed)
	if out, e := m796CLI(t, []string{"native-genesis-check", path, seedPath}, nil); e != nil || out != report.String() {
		t.Fatal("file transport -> native codec", out, e)
	}
	var original map[string]any
	if e := json.Unmarshal(raw, &original); e != nil {
		t.Fatal(e)
	}
	for _, fault := range []string{"guarantor", "asset", "range", "unknown", "kernel", "chain", "malformed", "signer"} {
		t.Run(fault, func(t *testing.T) {
			var altered map[string]any
			json.Unmarshal(raw, &altered)
			app := altered["app_state"].(map[string]any)
			badSeed := seed
			switch fault {
			case "guarantor":
				app["guarantor"] = map[string]any{}
			case "asset":
				app["bank"].(map[string]any)["balances"].([]any)[0].(map[string]any)["coins"].([]any)[0].(map[string]any)["amount"] = "-1"
			case "range":
				app["sequencer"].(map[string]any)["receipt_keys"].([]any)[0].(map[string]any)["first_height"] = 0
			case "unknown":
				app["sequencer"].(map[string]any)["guarantors"] = []any{}
			case "kernel":
				app["lxphost"].(map[string]any)["last_sequence"] = 1
			case "chain":
				altered["chain_id"] = "foreign-1"
			case "signer":
				badSeed = []byte(strings.Repeat("01", 32))
			}
			bad, _ := json.Marshal(altered)
			if fault == "malformed" {
				bad = []byte("{broken")
			}
			if _, e := ReadNativeGenesis(bad, badSeed); e == nil {
				t.Fatal("rejected initialization input accepted")
			}
		})
	}
	result := m796Commit(t, genesis, key, 1, 7)
	if e := result.SignedHeader.ValidateBasic(genesis.Doc.ChainID); e != nil {
		t.Fatal(e)
	}
	if bank := original["app_state"].(map[string]any)["bank"].(map[string]any); len(bank["balances"].([]any)) == 0 {
		t.Fatal("native asset balances lost")
	}
	m796Wire(t, "tests/kernel-genesis.test.sh", "validate-genesis", "native-genesis-check", "foundation-observe", "bad-asset", "bad-range", "retired-module")
}

func TestRepairM_796_FoundationCommittedRestart(t *testing.T) {
	genesis, raw, seed, key := m796Genesis(t)
	result := m796Commit(t, genesis, key, 3, 9)
	fault := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Method string
			Params json.RawMessage
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			w.WriteHeader(400)
			return
		}
		var answer any
		switch q.Method {
		case "status":
			answer = map[string]any{"node_info": map[string]any{"network": genesis.Doc.ChainID}, "sync_info": map[string]any{"latest_block_height": strconv.FormatInt(result.Header.Height, 10), "latest_block_hash": hex.EncodeToString(result.Header.Hash()), "catching_up": false}}
		case "commit":
			var params map[string]string
			if json.Unmarshal(q.Params, &params) != nil || params["height"] != strconv.FormatInt(result.Header.Height, 10) {
				t.Error("commit was not bound to observed height")
			}
			copyResult := result
			if fault == "signature" {
				commit := *result.Commit
				commit.Signatures = append([]tmtypes.CommitSig(nil), result.Commit.Signatures...)
				commit.Signatures[0].Signature = utils.Some(tmed.TestSecretKey([]byte("foreign")).Sign([]byte("foreign vote")))
				copyResult.Commit = &commit
			}
			answer = copyResult
		case "eth_chainId":
			answer = "0x7d"
			if fault == "chain" {
				answer = "0x1"
			}
		case "eth_getBlockByNumber":
			var params []json.RawMessage
			if json.Unmarshal(q.Params, &params) != nil || len(params) != 2 || string(params[0]) != fmt.Sprintf("\"0x%x\"", result.Header.Height) {
				t.Error("unbound EVM block request")
			}
			hash := "0x" + hex.EncodeToString(result.Header.Hash())
			if fault == "evm" {
				hash = "0x" + strings.Repeat("0", 64)
			}
			answer = map[string]any{"number": fmt.Sprintf("0x%x", result.Header.Height), "hash": hash}
		default:
			t.Errorf("retired foundation RPC %s", q.Method)
			w.WriteHeader(400)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": answer})
	}))
	defer server.Close()
	root := filepath.Join(t.TempDir(), "committed")
	state, e := OpenFoundationState(root, genesis)
	if e != nil {
		t.Fatal(e)
	}
	first, e := state.Observe(context.Background(), server.Client(), server.URL, server.URL)
	if e != nil || first.Height != 3 {
		t.Fatal("real signed commit consumer", e)
	}
	saved := m796Read(t, filepath.Join(root, "committed.json"))
	if e = state.Close(); e != nil {
		t.Fatal(e)
	}
	state, e = OpenFoundationState(root, genesis)
	if e != nil {
		t.Fatal(e)
	}
	if state.retained == nil || state.retained.BlockHash != first.BlockHash {
		t.Fatal("reopen lost cryptographic committed floor")
	}
	for _, bad := range []string{"rollback", "divergence", "signature", "chain", "evm"} {
		t.Run(bad, func(t *testing.T) {
			result = m796Commit(t, genesis, key, 3, 9)
			fault = bad
			if bad == "rollback" {
				result = m796Commit(t, genesis, key, 2, 9)
			}
			if bad == "divergence" {
				result = m796Commit(t, genesis, key, 3, 10)
			}
			if _, e = state.Observe(context.Background(), server.Client(), server.URL, server.URL); e == nil {
				t.Fatal("uncommitted/divergent observation accepted")
			}
			if !bytes.Equal(saved, m796Read(t, filepath.Join(root, "committed.json"))) {
				t.Fatal("refused observation advanced durable floor")
			}
		})
	}
	fault = ""
	result = m796Commit(t, genesis, key, 4, 11)
	if _, e = state.Observe(context.Background(), server.Client(), server.URL, server.URL); e != nil {
		t.Fatal(e)
	}
	state.Close()
	genesisPath, seedPath := filepath.Join(t.TempDir(), "genesis.json"), filepath.Join(t.TempDir(), "receipt.key")
	m796Write(t, genesisPath, raw)
	m796Write(t, seedPath, seed)
	if _, e = m796CLI(t, []string{"foundation-observe", root, genesisPath, seedPath, server.URL, server.URL}, nil); e != nil {
		t.Fatal("production CLI did not reopen real checkpoint", e)
	}
	// Drive the native manifest producer output through the same provenance
	// transport used by the maintained Python fixture and its retained consumer.
	directory := t.TempDir()
	inputs := map[string]string{"native_genesis": "chain/config/genesis.json", "receipt_key": "chain/config/receipt.key", "initialization": "genesis/initialization.json", "committed": "run/committed/committed.json"}
	var report bytes.Buffer
	if e = WriteNativeGenesisReport(&report, genesis); e != nil {
		t.Fatal(e)
	}
	material := map[string][]byte{inputs["native_genesis"]: raw, inputs["receipt_key"]: seed, inputs["initialization"]: report.Bytes(), inputs["committed"]: m796Read(t, filepath.Join(root, "committed.json"))}
	artifacts := map[string]FoundationArtifact{}
	for name, value := range material {
		m796Write(t, filepath.Join(directory, name), value)
		artifacts[name] = FoundationArtifact{SHA256: m796SHA(value), Size: int64(len(value))}
	}
	var committed FoundationCheckpoint
	if e = json.Unmarshal(material[inputs["committed"]], &committed); e != nil {
		t.Fatal(e)
	}
	var invocations []FoundationInvocation
	if e = json.Unmarshal(m796Read(t, "tests/fixtures/repair_m797/invocations.json"), &invocations); e != nil {
		t.Fatal(e)
	}
	artifact := filepath.Join(directory, "artifact-manifest.json")
	bundle := m796Read(t, "runtime-bundle.json")
	m796Write(t, artifact, bundle)
	revision, binding := strings.Repeat("a", 40), m796SHA(raw)
	manifest := map[string]any{"version": 2, "stage": "dependency-foundation", "purpose": "disposable-test-only", "source_revision": revision, "source_binding": binding, "source_paths": []string{"tools/bringup"}, "producers": invocations, "inputs": inputs, "generated_artifacts": artifacts, "artifact_manifest": artifact, "artifact_manifest_sha256": m796SHA(bundle), "chain_id": 125, "network_id": 125, "cosmos_chain_id": genesis.Doc.ChainID, "committed": committed}
	encoded, e := json.Marshal(manifest)
	if e != nil {
		t.Fatal(e)
	}
	args := []string{"native-foundation-provenance", directory, revision, binding, "foundation"}
	accepted, e := m796CLI(t, args, encoded)
	if e != nil || accepted != string(encoded) {
		t.Fatal("native foundation provenance consumer", e)
	}
	for _, fault := range []string{"legacy", "retired-input", "wrong-chain", "foreign-commit", "unbound-input", "bad-provenance"} {
		t.Run("manifest/"+fault, func(t *testing.T) {
			var bad map[string]any
			if e := json.Unmarshal(encoded, &bad); e != nil {
				t.Fatal(e)
			}
			switch fault {
			case "legacy":
				bad["version"] = 1
			case "retired-input":
				bad["inputs"].(map[string]any)["native_genesis"] = "genesis/custody.json"
			case "wrong-chain":
				bad["chain_id"] = 126
			case "foreign-commit":
				bad["committed"].(map[string]any)["genesis"] = strings.Repeat("0", 64)
			case "unbound-input":
				bad["inputs"].(map[string]any)["foreign"] = "not-produced.json"
			case "bad-provenance":
				bad["producers"].([]any)[0].(map[string]any)["exit_code"] = 1
			}
			altered, e := json.Marshal(bad)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = m796CLI(t, args, altered); e == nil {
				t.Fatal("invalid native handoff accepted")
			}
		})
	}
	m796Wire(t, "tests/foundation_provenance.py", "native-foundation-provenance")
	badGenesis := genesis
	badGenesis.Digest = strings.Repeat("a", 64)
	if _, e = OpenFoundationState(root, badGenesis); e == nil {
		t.Fatal("checkpoint replayed against foreign initialization")
	}
	m796Wire(t, "tests/paxeer-x-fixture-foundation.py", "'version': 2", "native-genesis-check", "validate-genesis", "foundation-observe", "restart_preserved", "committed", "PAXEER_X_NATIVE_INPUTS")
	source := string(m796Read(t, "tests/paxeer-x-fixture-foundation.py"))
	for _, retired := range []string{"custody-genesis.py", "anchor-genesis.py", "init-chain.sh", "--layerxd", "app_state.layerxcustody"} {
		if strings.Contains(source, retired) {
			t.Fatalf("retired producer %s", retired)
		}
	}
}

func m796MintInputs(t *testing.T) MintInputs {
	t.Helper()
	now := time.Now().UTC()
	issuerKey, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	authority := &x509.Certificate{SerialNumber: big.NewInt(796), Subject: pkix.Name{CommonName: "disposable attestor CA"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(48 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	input := MintInputs{Issuer: "https://supabase.paxeer.app/auth/v1", Tenant: "paxeer-x-mainnet-beta", Now: now, Modules: m796Read(t, "../../services/interop/deploy/gateway/module-registry.example.json")}
	for i := 1; i <= 5; i++ {
		key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		leaf := &x509.Certificate{SerialNumber: big.NewInt(int64(i)), Subject: pkix.Name{CommonName: fmt.Sprintf("paxeer-attestor-%d", i)}, DNSNames: []string{fmt.Sprintf("node-%d", i)}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
		der, e := x509.CreateCertificate(rand.Reader, leaf, authority, &key.PublicKey, issuerKey)
		if e != nil {
			t.Fatal(e)
		}
		input.Certificates = append(input.Certificates, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	}
	return input
}
func TestRepairM_796_MintCurrentConsumers(t *testing.T) {
	input := m796MintInputs(t)
	out := filepath.Join(t.TempDir(), "mint")
	paths, e := MintSecrets(out, input)
	if e != nil {
		t.Fatal(e)
	}
	expected := map[string]int{"wallet-gateway": 4, "gateway": 7, "webhooks-public": 1, "webhooks-ingress": 8, "internal-kms": 2, "internal-journeys": 2, "internal-payments": 2, "internal-approvals": 2, "internal-programs": 2, "identity": 9, "dashboard": 1, "ramp": 1, "paxeer-human-service": 3, "paxeer-program-registry": 5, "paxeer-attestor-1": 3, "paxeer-attestor-2": 3, "paxeer-attestor-3": 3, "paxeer-attestor-4": 3, "paxeer-attestor-5": 3}
	if len(paths) != 31 {
		t.Fatalf("merged delivery file count %d", len(paths))
	}
	consumed := map[string]map[string][]byte{}
	for service, count := range expected {
		suffix := ".railway.env"
		if strings.HasPrefix(service, "paxeer-") {
			suffix = ".box.env"
		}
		raw := m796Read(t, filepath.Join(out, service+suffix))
		values, e := ReadMintEnvironment(service, bytes.NewReader(raw))
		if e != nil || len(values) != count {
			t.Fatalf("%s actual delivery consumer: %v", service, e)
		}
		consumed[service] = values
		if _, e = m796CLI(t, []string{"mint-consume", service}, raw); e != nil {
			t.Fatal("CLI delivery consumer", e)
		}
		if _, e = ReadMintEnvironment(service, bytes.NewReader(append(append([]byte(nil), raw...), []byte("ENDPOINT_AUTHORITY_TOKEN=retired\n")...))); e == nil {
			t.Fatal("foreign secret accepted")
		}
		line := strings.SplitN(string(raw), "\n", 2)[0]
		if _, e = ReadMintEnvironment(service, strings.NewReader(string(raw)+line+"\n")); e == nil {
			t.Fatal("duplicate secret accepted")
		}
	}
	// Every independent token boundary must be distinct; the only repeated
	// values belong to the same declared producer/consumer trust boundary.
	byBoundary := map[string][]byte{}
	tokenOwners := map[string]string{}
	for _, d := range MintDeliveries() {
		value := consumed[d.Service][d.Variable]
		if old := byBoundary[d.Secret]; old != nil && !bytes.Equal(old, value) {
			t.Fatalf("paired ends differ for %s", d.Secret)
		}
		byBoundary[d.Secret] = value
		if fleetSHA.Match(value) {
			if previous, ok := tokenOwners[string(value)]; ok && previous != d.Secret {
				t.Fatalf("independent credentials collide: %s/%s", previous, d.Secret)
			}
			tokenOwners[string(value)] = d.Secret
		}
		for _, retired := range []string{"LAYERX_KERNEL_", "AUTHORIZATION", "AUTHORITY_TOKEN"} {
			if strings.Contains(d.Variable, retired) {
				t.Fatal("retired secret produced", d)
			}
		}
	}
	if len(tokenOwners) != 25 {
		t.Fatalf("independent current token boundary count %d", len(tokenOwners))
	}
	for service, values := range consumed {
		table, role := "", "*"
		files := map[string][]byte{}
		for name, raw := range values {
			if name == "WALLET_IDENTITY_BINDING_TENANT" {
				continue
			}
			files[name] = raw
		}
		switch service {
		case "gateway":
			table = "../../docker/platform-gateway/files.tsv"
		case "webhooks-public":
			table = "../../docker/platform-webhooks/files.tsv"
			role = "public"
		case "webhooks-ingress":
			table = "../../docker/platform-webhooks/files.tsv"
			role = "ingress"
		case "wallet-gateway":
			table = "../../docker/wallet-gateway/files.tsv"
		}
		if table != "" {
			mapped, e := MintFileDestinations(files, bytes.NewReader(m796Read(t, table)), role)
			if e != nil || len(mapped) != len(files) {
				t.Fatalf("%s actual image entrypoint table: %v", service, e)
			}
			for path, value := range mapped {
				if !strings.HasPrefix(path, "/run/") || len(value) == 0 {
					t.Fatal("invalid consumer destination")
				}
			}
		}
	}
	if _, ok := consumed["webhooks-public"]["WEBHOOKS_KMS_TOKEN"]; ok {
		t.Fatal("public webhook received ingress KMS capability")
	}
	if _, ok := consumed["webhooks-public"]["WEBHOOKS_CURSOR_KEY"]; ok {
		t.Fatal("public webhook received ingress cursor capability")
	}
	gateway := consumed["gateway"]
	identity := consumed["identity"]
	if !bytes.Equal(gateway["ENDPOINT_IDENTITY_TOKEN"], identity["IDENTITY_GATEWAY_TOKEN"]) {
		t.Fatal("identity plain env and gateway decoded file do not agree")
	}
	if !bytes.Equal(gateway["ENDPOINT_MODULE_REGISTRY"], input.Modules) {
		t.Fatal("module registry changed in delivery")
	}
	wallet := consumed["wallet-gateway"]
	token, public := wallet["WALLET_CUSTODY_INVENTORY"], wallet["WALLET_CUSTODY_INVENTORY_PUBLIC_KEY"]
	if e = VerifyMintInventory(token, public, input.Certificates, input.Issuer, input.Tenant, input.Now); e != nil {
		t.Fatal("real signed inventory and certificate pins", e)
	}
	for _, bad := range []string{"signature", "tenant", "certificates", "expired", "authority"} {
		t.Run(bad, func(t *testing.T) {
			badToken, badPublic, badCerts, tenant, now := append([]byte(nil), token...), public, input.Certificates, input.Tenant, input.Now
			switch bad {
			case "signature":
				parts := strings.Split(strings.TrimSpace(string(token)), ".")
				parts[1] = base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"forged"}`))
				badToken = []byte(strings.Join(parts, "."))
			case "tenant":
				tenant = "foreign"
			case "certificates":
				badCerts = append([][]byte(nil), input.Certificates...)
				badCerts[0] = badCerts[1]
			case "expired":
				now = now.Add(31 * 24 * time.Hour)
			case "authority":
				badPublic = wallet["WALLET_IDENTITY_BINDING_KEY"]
			}
			if e = VerifyMintInventory(badToken, badPublic, badCerts, input.Issuer, tenant, now); e == nil {
				t.Fatal("malformed inventory accepted")
			}
		})
	}
	e = filepath.WalkDir(out, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		want := os.FileMode(0600)
		if d.IsDir() {
			want = 0700
		}
		if info.Mode().Perm() != want {
			return fmt.Errorf("secret mode %s: %o", path, info.Mode().Perm())
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = MintSecrets(out, input); e == nil {
		t.Fatal("existing mint overwritten")
	}
	for _, fault := range []string{"cert", "duplicate-cert", "modules", "issuer", "tenant", "ordinals"} {
		t.Run(fault, func(t *testing.T) {
			bad := input
			bad.Certificates = append([][]byte(nil), input.Certificates...)
			switch fault {
			case "cert":
				bad.Certificates[0] = []byte("not a certificate")
			case "duplicate-cert":
				bad.Certificates[1] = bad.Certificates[0]
			case "modules":
				bad.Modules = []byte(`{"schema_version":1}`)
			case "issuer":
				bad.Issuer = "http://untrusted/auth/v1"
			case "tenant":
				bad.Tenant = "unsafe\nTENANT"
			case "ordinals":
				bad.Modules = []byte(`{"schema_version":2,"assets":[{"asset":"` + strings.Repeat("a", 64) + `","symbol":"LXT","currency":"LXT","decimals":18}],"modules":[{"module":1,"ordinals":[1,1]}]}`)
			}
			directory := filepath.Join(t.TempDir(), "bad")
			if _, e = MintSecrets(directory, bad); e == nil {
				t.Fatal("malformed mint input accepted")
			}
			if _, e = os.Stat(directory); !errors.Is(e, os.ErrNotExist) {
				t.Fatal("refused mint published output")
			}
		})
	}
	badEnv := []byte("ENDPOINT_IDENTITY_TOKEN=invalid!\n")
	if _, e = ReadMintEnvironment("gateway", bytes.NewReader(badEnv)); e == nil {
		t.Fatal("malformed base64 accepted")
	}
	if _, e = MintFileDestinations(map[string][]byte{"SECRET": []byte("token")}, strings.NewReader("SECRET\t/run/token\t0644\t*\n"), "*"); e == nil {
		t.Fatal("public secret mode accepted")
	}
	// Use the production deployment-input reader and CLI producer as well as
	// API calls, with real generated certificates and maintained issuer files.
	certDir := t.TempDir()
	for i, cert := range input.Certificates {
		m796Write(t, filepath.Join(certDir, fmt.Sprintf("attestor-%d.crt", i+1)), cert)
	}
	cliOut := filepath.Join(t.TempDir(), "mint")
	printed, e := m796CLI(t, []string{"mint", cliOut, certDir, "../../services/interop/deploy/gateway/module-registry.example.json", "../../services/wallet/deploy"}, nil)
	if e != nil || len(strings.Fields(printed)) != 31 {
		t.Fatal("production deployment producer", e)
	}
	if !strings.Contains(string(m796Read(t, filepath.Join(cliOut, "gateway.railway.sh"))), "--service router --environment") {
		t.Fatal("mint ignored the actual gateway rollout consumer")
	}
	for _, value := range byBoundary {
		if len(value) > 16 && strings.Contains(printed, string(value)) {
			t.Fatal("secret reached stdout")
		}
	}
	m796Wire(t, "mint-secrets.sh", "mint", "ATTESTOR_CERT_DIR", "MODULE_REGISTRY_FILE")
	m796Wire(t, "tests/mint-secrets.test.sh", "mint-consume", "gateway.railway.env", "loadWalletInventory", "malformed", "distinct")
	m796Wire(t, "../../services/platform/hosted/identity/env-init.sh", "for service in gateway registry webhooks", `variable="IDENTITY_`, `>"$tokens_dir/$service.new"`)
}
