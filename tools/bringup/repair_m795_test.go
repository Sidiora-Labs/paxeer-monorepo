package bringup

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func m795Read(t *testing.T, path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func m795Write(t *testing.T, path string, b []byte) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func m795Private(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	return dir
}
func m795JSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func m795Inventory(t *testing.T) PublicInventory {
	t.Helper()
	i, e := ReadPublicInventory(bytes.NewReader(m795Read(t, "../../docs/site/docs/reference/public-rpc.md")))
	if e != nil {
		t.Fatal(e)
	}
	return i
}
func m795CLI(args []string, input []byte) (string, error) {
	var out bytes.Buffer
	e := RunInspector(context.Background(), args, bytes.NewReader(input), &out)
	return out.String(), e
}
func m795Wire(t *testing.T, path string, terms ...string) {
	t.Helper()
	source := string(m795Read(t, path))
	for _, term := range terms {
		if !strings.Contains(source, term) {
			t.Fatalf("%s does not wire %q", path, term)
		}
	}
}

// Execute the production transaction primitives used by the shell writers.
// The renderer emits both the durable site and its manifest row; the consumer
// validates their binding before accepting a reload acknowledgement.
func m795EdgeRender(t *testing.T, s *EdgeState, rows []EdgeRoute, pool, down []string) error {
	t.Helper()
	var manifest strings.Builder
	for _, row := range rows {
		manifest.WriteString(row.String() + "\n")
		var site bytes.Buffer
		var e error
		if row.Mode == "rpc" {
			e = RenderEdgeRPC(&site, row.Name, pool, down, "/var/www/certbot", "/etc/letsencrypt/live", "127.0.0.53", true, false)
		} else {
			e = RenderEdgeHTTP(&site, row, "/var/www/certbot", "/etc/letsencrypt/live", "127.0.0.53", true, false)
		}
		if e != nil {
			return e
		}
		if _, e = EdgeRouteContract(row, site.String()); e != nil {
			return e
		}
		path := filepath.Join(s.Root, "sites-available", row.Name+".conf")
		if e = edgeAtomic(path, site.Bytes(), 0644); e != nil {
			return e
		}
		if e = os.MkdirAll(filepath.Join(s.Root, "sites-enabled"), 0755); e != nil {
			return e
		}
		link := filepath.Join(s.Root, "sites-enabled", row.Name+".conf")
		if e = os.Remove(link); e != nil && !os.IsNotExist(e) {
			return e
		}
		if e = os.Symlink(path, link); e != nil {
			return e
		}
	}
	for path, value := range map[string]string{"manifest": manifest.String(), "rpc-pool": strings.Join(pool, "\n") + "\n", "rpc-down": strings.Join(down, "\n")} {
		if e := edgeAtomic(filepath.Join(s.Root, "edge", path), []byte(value), 0600); e != nil {
			return e
		}
	}
	return nil
}

func TestRepairM_795_EdgeConcurrentStateMutation(t *testing.T) {
	root := m795Private(t)
	pool := []string{"api1.mainnet-beta.paxeer.network", "api2.mainnet-beta.paxeer.network"}
	rpc := EdgeRoute{Name: "api-mainnet-beta.paxeer.network", Mode: "rpc", Upstream: "-", Port: "443", Kind: "rpc", SNI: "-"}
	route, e := EdgeMapLine("search.paxeer.network", "railway", "search-front.up.railway.app:443", "-")
	if e != nil {
		t.Fatal(e)
	}
	s, e := OpenEdgeState(root)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.WithMutation(func() error {
		if e := s.Begin(); e != nil {
			return e
		}
		if e := m795EdgeRender(t, s, []EdgeRoute{rpc}, pool, nil); e != nil {
			return e
		}
		return s.ReloadResult(0, 0)
	}); e != nil {
		t.Fatal(e)
	}
	inode, e := os.Stat(filepath.Join(root, ".edge-operation.lock"))
	if e != nil {
		t.Fatal(e)
	}
	acquired, release := make(chan struct{}), make(chan struct{})
	failures := make(chan error, 2)
	go func() {
		health, e := OpenEdgeState(root)
		if e != nil {
			failures <- e
			return
		}
		defer health.Close()
		failures <- health.WithMutation(func() error {
			if e := health.Begin(); e != nil {
				return e
			}
			close(acquired)
			<-release
			b, e := os.ReadFile(filepath.Join(root, "edge/manifest"))
			if e != nil {
				return e
			}
			rows, e := ReadEdgeManifest(bytes.NewReader(b), false)
			if e != nil {
				return e
			}
			if e = m795EdgeRender(t, health, rows, pool, []string{pool[1]}); e != nil {
				return e
			}
			return health.ReloadResult(0, 0)
		})
	}()
	<-acquired
	started, entered := make(chan struct{}), make(chan struct{})
	go func() {
		writer, e := OpenEdgeState(root)
		if e != nil {
			failures <- e
			return
		}
		defer writer.Close()
		close(started)
		failures <- writer.WithMutation(func() error {
			close(entered)
			b, e := os.ReadFile(filepath.Join(root, "edge/manifest"))
			if e != nil {
				return e
			}
			rows, e := ReadEdgeManifest(bytes.NewReader(b), false)
			if e != nil {
				return e
			}
			down := strings.Fields(string(m795Read(t, filepath.Join(root, "edge/rpc-down"))))
			if e = writer.Begin(); e != nil {
				return e
			}
			rows = append(rows, route)
			if e = m795EdgeRender(t, writer, rows, pool, down); e != nil {
				return e
			}
			return writer.ReloadResult(0, 0)
		})
	}()
	<-started
	select {
	case <-entered:
		t.Fatal("route writer entered while pool-health transaction held mutation lock")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	for n := 0; n < 2; n++ {
		if e = <-failures; e != nil {
			t.Fatal(e)
		}
	}
	// A subsequent rejected update cannot roll back either accepted writer.
	if e = s.WithMutation(func() error {
		if e := s.Begin(); e != nil {
			return e
		}
		if e := m795EdgeRender(t, s, []EdgeRoute{rpc}, pool, nil); e != nil {
			return e
		}
		if e := s.ReloadResult(1, -1); e == nil {
			return fmt.Errorf("validation refusal was accepted")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e = OpenEdgeState(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.Recover(); e != nil {
		t.Fatal(e)
	}
	rows, e := ReadEdgeManifest(bytes.NewReader(m795Read(t, filepath.Join(root, "edge/manifest"))), false)
	if e != nil || len(rows) != 2 {
		t.Fatalf("accepted route lost: %+v %v", rows, e)
	}
	if string(m795Read(t, filepath.Join(root, "edge/rpc-down"))) != pool[1] {
		t.Fatal("accepted pool health lost")
	}
	if _, e = EdgeRouteContract(route, string(m795Read(t, filepath.Join(root, "sites-available/search.paxeer.network.conf")))); e != nil {
		t.Fatal(e)
	}
	after, e := os.Stat(filepath.Join(root, ".edge-operation.lock"))
	if e != nil || !os.SameFile(inode, after) {
		t.Fatal("mutation lock inode replaced by state rollback")
	}
	m795Wire(t, "edge.sh", `exec 8>"$nginx_dir/.edge-operation.lock"`, `flock -x 8`, `cmd_set() {`+"\n\tedge_recover", `cmd_rpc_health() {`+"\n\tedge_recover", `cmd_rerender()`, "inherited mutation lock")
	if strings.Contains(string(m795Read(t, "edge.sh")), "rpc-health.lock") {
		t.Fatal("cron still has a separate mutation lock")
	}
	m795Wire(t, "edge-sync.sh", `flock -x 8`, `8>%q`)
}

func TestRepairM_795_ArchiveUnpublishedNames(t *testing.T) {
	i := m795Inventory(t)
	names, e := m795CLI([]string{"rpc-names", "../../docs/site/docs/reference/public-rpc.md"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(i.Nodes) != 14 || len(strings.Fields(names)) != 14 {
		t.Fatal("unexpected maintained node inventory")
	}
	for _, name := range strings.Fields(names) {
		if _, e = m795CLI([]string{"archive-name", "../../docs/site/docs/reference/public-rpc.md", name}, nil); e != nil {
			t.Fatalf("published node %s refused: %v", name, e)
		}
	}
	for _, name := range []string{"api15.mainnet-beta.paxeer.network", "api16.mainnet-beta.paxeer.network", "api1.paxeer.network", strings.TrimPrefix(i.Canonical, "https://"), "api1.mainnet-beta.paxeer.network/rpc", "api1.mainnet-beta.paxeer.network;exit"} {
		if e = ArchiveName(i, name); e == nil {
			t.Fatalf("unpublished archive identity accepted: %q", name)
		}
	}
	m795Wire(t, "install-archive.sh", `archive-name "$inventory" "$name"`, "BRINGUP_RPC_INVENTORY")
	m795Wire(t, "docs-names.sh", "docs/site/docs/reference/public-rpc.md")
}

func m795TLSIdentity(t *testing.T, name string) (tls.Certificate, []byte, []byte) {
	t.Helper()
	pub, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(795), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, e := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, key)
	if e != nil {
		t.Fatal(e)
	}
	keyDER, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		t.Fatal(e)
	}
	certPEM, keyPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	pair, e := tls.X509KeyPair(certPEM, keyPEM)
	if e != nil {
		t.Fatal(e)
	}
	return pair, certPEM, keyPEM
}
func m795NamedTLSClient(t *testing.T, server *httptest.Server, pemBytes []byte) *http.Client {
	t.Helper()
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pemBytes) {
		t.Fatal("certificate")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	}}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: time.Second}
}
func TestRepairM_795_ArchiveTLSResume(t *testing.T) {
	root := m795Private(t)
	name := strings.TrimPrefix(m795Inventory(t).Nodes[0], "https://")
	args := []string{"archive-reconcile", "../../docs/site/docs/reference/public-rpc.md", filepath.Join(root, "nginx"), filepath.Join(root, "certs"), filepath.Join(root, "webroot"), name, "v1"}
	out, e := m795CLI(args, nil)
	if e != nil || out != "certificate changed\n" {
		t.Fatalf("initial HTTP reconciliation: %q %v", out, e)
	}
	available := filepath.Join(root, "nginx/sites-available", name+".conf")
	enabled := filepath.Join(root, "nginx/sites-enabled", name+".conf")
	if _, e = os.Readlink(enabled); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(m795Read(t, available)), "listen 443") {
		t.Fatal("TLS before a certificate exists")
	}
	// A failed ACME invocation has no certificate to publish. A new CLI instance
	// must revisit issuance even though the enabled symlink survived shutdown.
	out, e = m795CLI(args, nil)
	if e != nil || out != "certificate unchanged\n" {
		t.Fatalf("reopen after failed ACME skipped issuance: %q %v", out, e)
	}
	pair, cert, key := m795TLSIdentity(t, name)
	m795Write(t, filepath.Join(root, "certs", name, "fullchain.pem"), cert)
	m795Write(t, filepath.Join(root, "certs", name, "privkey.pem"), []byte("partial issuance"))
	out, e = m795CLI(args, nil)
	if e != nil || !strings.HasPrefix(out, "certificate ") {
		t.Fatalf("partial certificate admitted: %q %v", out, e)
	}
	m795Write(t, filepath.Join(root, "certs", name, "privkey.pem"), key)
	out, e = m795CLI(args, nil)
	if e != nil || out != "tls changed\n" {
		t.Fatalf("resumed TLS reconciliation: %q %v", out, e)
	}
	if !strings.Contains(string(m795Read(t, available)), "listen 443 ssl") {
		t.Fatal("resumed site still HTTP only")
	}
	ready := true
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" || r.Host != name {
			t.Error("archive HTTPS host/path mismatch")
		}
		if !ready {
			http.Error(w, "unavailable", 503)
			return
		}
		io.WriteString(w, "ok")
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	client := m795NamedTLSClient(t, server, cert)
	if e = ProbeArchiveHTTPS(context.Background(), client, "https://"+name); e != nil {
		t.Fatal(e)
	}
	ready = false
	if e = ProbeArchiveHTTPS(context.Background(), client, "https://"+name); e == nil {
		t.Fatal("unhealthy HTTPS accepted")
	}
	ready = true
	// Existing operator TLS configuration is kept byte-for-byte.
	custom := append(m795Read(t, available), []byte("\n# operator connection limits\n")...)
	m795Write(t, available, custom)
	out, e = m795CLI(args, nil)
	if e != nil || out != "tls unchanged\n" || !bytes.Equal(custom, m795Read(t, available)) {
		t.Fatalf("operator TLS configuration overwritten: %q %v", out, e)
	}
	m795Write(t, available, []byte("# operator HTTP server\nserver { listen 80; }\n"))
	before := m795Read(t, available)
	if _, e = m795CLI(args, nil); e == nil || !bytes.Equal(before, m795Read(t, available)) {
		t.Fatal("incomplete operator site overwritten or admitted")
	}
	m795Wire(t, "install-archive.sh", `state="$(archive_reconcile)"`, `[[ "$state" = certificate* ]]`, `archive-health "https://$name"`)
}

func TestRepairM_795_GenesisRetiredNetworkBoundary(t *testing.T) {
	home := m795Private(t)
	m795Write(t, filepath.Join(home, "config/config.toml"), []byte("node-key-file = \"config/node_key.json\"\ngenesis-file = \"config/genesis.json\"\n[priv-validator]\nkey-file = \"config/priv_validator_key.json\"\nstate-file = \"data/priv_validator_state.json\"\n"))
	m795Write(t, filepath.Join(home, "config/app.toml"), []byte("[sequencer]\nenable = true\nreceipt-key-file = \"config/receipt.hex\"\n"))
	m795Write(t, filepath.Join(home, "config/receipt.hex"), []byte(strings.Repeat("ab", 32)+"\n"))
	key := tmtypes.GenNodeKey()
	if e := key.SaveAs(filepath.Join(home, "config/node_key.json")); e != nil {
		t.Fatal(e)
	}
	if e := os.MkdirAll(filepath.Join(home, "data"), 0700); e != nil {
		t.Fatal(e)
	}
	pv, e := privval.GenFilePV(filepath.Join(home, "config/priv_validator_key.json"), filepath.Join(home, "data/priv_validator_state.json"), "")
	if e != nil {
		t.Fatal(e)
	}
	if e = pv.Save(); e != nil {
		t.Fatal(e)
	}
	// This committed fixture is a native paxd-produced genesis, decoded and
	// emitted again through its actual consensus codec rather than an LXGB blob.
	doc, e := tmtypes.GenesisDocFromJSON(m795Read(t, "../../services/platform/hosted/node/tests/fixtures/repair_22_31_355/genesis.json"))
	if e != nil {
		t.Fatal(e)
	}
	genesisPath := filepath.Join(home, "config/genesis.json")
	if e = doc.SaveAs(genesisPath); e != nil {
		t.Fatal(e)
	}
	genesis := m795Read(t, genesisPath)
	sum := sha256.Sum256(genesis)
	if e = CheckPaxdGenesis(genesisPath, doc.ChainID, hex.EncodeToString(sum[:])); e != nil {
		t.Fatal(e)
	}
	if e = CheckPaxdGenesis(genesisPath, "foreign-chain", hex.EncodeToString(sum[:])); e == nil {
		t.Fatal("foreign genesis chain admitted")
	}
	identity, e := RetainPaxdIdentity(home, doc.ChainID, true)
	if e != nil || identity.NodeID != string(key.ID()) {
		t.Fatalf("native identity adoption: %+v %v", identity, e)
	}
	if len(identity.Files) != 4 || identity.Files[filepath.Join(home, "config/receipt.hex")] == "" {
		t.Fatal("native validator or sequencer receipt identity not retained")
	}
	record := m795Read(t, filepath.Join(home, "identity/paxd.json"))
	pv.LastSignState.Height = 7
	if e = pv.Save(); e != nil {
		t.Fatal(e)
	}
	out, e := m795CLI([]string{"paxd-identity", "check", home, doc.ChainID}, nil)
	if e != nil {
		t.Fatal(e)
	}
	var reopened PaxdIdentity
	if json.Unmarshal([]byte(out), &reopened) != nil || !reflect.DeepEqual(identity, reopened) {
		t.Fatal("native retained identity changed on reopen")
	}
	retainedPV, e := privval.LoadFilePV(filepath.Join(home, "config/priv_validator_key.json"), filepath.Join(home, "data/priv_validator_state.json"))
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(record, m795Read(t, filepath.Join(home, "identity/paxd.json"))) || retainedPV.LastSignState.Height != 7 {
		t.Fatal("reopen reset signing state or rewrote retention record")
	}
	m795Write(t, filepath.Join(home, "config/receipt.hex"), []byte(strings.Repeat("cd", 32)+"\n"))
	if _, e = RetainPaxdIdentity(home, doc.ChainID, false); e == nil {
		t.Fatal("receipt key rotation silently adopted")
	}
	m795Write(t, filepath.Join(home, "config/receipt.hex"), []byte(strings.Repeat("ab", 32)+"\n"))
	replacement := tmtypes.GenNodeKey()
	if e = replacement.SaveAs(filepath.Join(home, "config/node_key.json")); e != nil {
		t.Fatal(e)
	}
	if _, e = RetainPaxdIdentity(home, doc.ChainID, true); e == nil {
		t.Fatal("adopt silently replaced a retained identity")
	}
	if !bytes.Equal(record, m795Read(t, filepath.Join(home, "identity/paxd.json"))) {
		t.Fatal("refused rotation changed retention record")
	}
	if e = ValidatePaxdInit(home, "node", doc.ChainID, "full"); e == nil {
		t.Fatal("native init accepted a populated home")
	}
	if e = ValidatePaxdInit(filepath.Join(t.TempDir(), "new"), "node", doc.ChainID, "archive"); e != nil {
		t.Fatal(e)
	}
	source := string(m795Read(t, "kernel-genesis.sh"))
	for _, retired := range []string{"18545", "0x0000000000000000000000000000000000001013", "kernel-init", "replica-id", "checkpoint-authority", "layerxcustody governance"} {
		if strings.Contains(source, retired) {
			t.Fatalf("live workflow retains %s", retired)
		}
	}
	m795Wire(t, "kernel-genesis.sh", `"${PAXD:-paxd}" init`, "paxd-genesis-check", "paxd-identity", "offline-render)", "retired live workflow")
}

func m795TokenPlan(t *testing.T, retained string) SecretPlan {
	t.Helper()
	plan, e := NewSecretPlan(bytes.NewReader(m795Read(t, "../../deploy/rollout.kvx")), bytes.NewReader(m795Read(t, "../../services/wallet/deploy/human.toml")), m795Read(t, "../../services/interop/deploy/gateway/module-registry.example.json"), retained)
	if e != nil {
		t.Fatal(e)
	}
	return plan
}
func m795CopyTree(t *testing.T, source, dest string) {
	t.Helper()
	e := filepath.WalkDir(source, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(source, path)
		if e != nil {
			return e
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(dest, rel), 0700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("nonregular fixture")
		}
		m795Write(t, filepath.Join(dest, rel), m795Read(t, path))
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
}
func TestRepairM_795_MintRetiredSecretTargets(t *testing.T) {
	out := m795Private(t)
	plan := m795TokenPlan(t, "")
	var printed bytes.Buffer
	if e := plan.Write(out, &printed); e != nil {
		t.Fatal(e)
	}
	roles := []string{}
	for _, target := range plan.Targets {
		roles = append(roles, target.Role)
		for variable, value := range target.Variables {
			if strings.Contains(variable, "AUTHORITY") || strings.Contains(variable, "REPLICA") || strings.Contains(variable, "COMPONENT") {
				t.Fatalf("retired capability minted: %s", variable)
			}
			if strings.Contains(printed.String(), value) {
				t.Fatal("secret leaked in output")
			}
			if target.Role == "gateway" && target.Consumer != "router" {
				t.Fatal("gateway delivery did not consume actual rollout service")
			}
		}
	}
	sort.Strings(roles)
	expected := []string{"dashboard", "gateway", "identity", "internal-approvals", "internal-journeys", "internal-kms", "internal-payments", "internal-programs", "paxeer-human-service", "paxeer-program-registry", "ramp", "webhooks-ingress", "webhooks-public"}
	if !reflect.DeepEqual(roles, expected) {
		t.Fatalf("secret target inventory differs: %v", roles)
	}
	read := func(file, key string, encoded bool) string {
		t.Helper()
		v, e := ReadMintSecret(filepath.Join(out, file), key, encoded)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	pairs := [][6]string{
		{"paxeer-human-service.box.env", "HUMAN_EVENTS_JOURNEY_TOKEN", "1", "internal-journeys.railway.env", "INTERNAL_JOURNEYS_PRODUCER_TOKEN", "1"},
		{"paxeer-human-service.box.env", "HUMAN_EVENTS_APPROVAL_TOKEN", "1", "internal-approvals.railway.env", "INTERNAL_APPROVALS_PRODUCER_TOKEN", "1"},
		{"gateway.railway.env", "ENDPOINT_EVENTS_PAYMENT_TOKEN", "1", "internal-payments.railway.env", "INTERNAL_PAYMENTS_PRODUCER_TOKEN", "1"},
		{"paxeer-program-registry.box.env", "REGISTRY_PROGRAM_EVENTS_TOKEN", "0", "internal-programs.railway.env", "INTERNAL_PROGRAMS_PRODUCER_TOKEN", "1"},
		{"paxeer-program-registry.box.env", "REGISTRY_REQUEST_TOKEN", "0", "gateway.railway.env", "ENDPOINT_PROGRAM_REGISTRY_TOKEN", "1"},
		{"webhooks-ingress.railway.env", "WEBHOOKS_KMS_TOKEN", "1", "internal-kms.railway.env", "INTERNAL_KMS_TOKEN", "1"},
		{"webhooks-ingress.railway.env", "WEBHOOKS_JOURNEY_SOURCE_TOKEN", "1", "internal-journeys.railway.env", "INTERNAL_JOURNEYS_TOKEN", "1"},
		{"webhooks-ingress.railway.env", "WEBHOOKS_PAYMENT_SOURCE_TOKEN", "1", "internal-payments.railway.env", "INTERNAL_PAYMENTS_TOKEN", "1"},
		{"webhooks-ingress.railway.env", "WEBHOOKS_APPROVAL_SOURCE_TOKEN", "1", "internal-approvals.railway.env", "INTERNAL_APPROVALS_TOKEN", "1"},
		{"webhooks-ingress.railway.env", "WEBHOOKS_PROGRAM_SOURCE_TOKEN", "1", "internal-programs.railway.env", "INTERNAL_PROGRAMS_TOKEN", "1"},
	}
	seen := map[string]bool{}
	for _, pair := range pairs {
		a, b := read(pair[0], pair[1], pair[2] == "1"), read(pair[3], pair[4], pair[5] == "1")
		if a != b {
			t.Fatalf("producer/consumer token differs: %s -> %s", pair[1], pair[4])
		}
		if seen[a] {
			t.Fatal("independent roles share a token")
		}
		seen[a] = true
	}
	if read("paxeer-program-registry.box.env", "REGISTRY_REQUEST_TOKEN", false) == read("paxeer-program-registry.box.env", "REGISTRY_PUBLICATION_TOKEN", false) {
		t.Fatal("registry request grants publication authority")
	}
	if len(plan.Targets[3].Variables) != 1 || plan.Targets[3].Variables["WEBHOOKS_IDENTITY_TOKEN"] == "" {
		t.Fatal("public webhook role holds ingress credentials")
	}
	for _, name := range []string{"router.railway.env", "interop.railway.env"} {
		if _, e := os.Lstat(filepath.Join(out, name)); !os.IsNotExist(e) {
			t.Fatal("obsolete target emitted")
		}
	}
	m795Wire(t, "../../services/platform/hosted/internal/env-init.sh", "INTERNAL_KMS_TOKEN", "INTERNAL_KMS_SEAL_SECRET", `"${variable}_PRODUCER_TOKEN"`)
	m795Wire(t, "../../services/platform/hosted/webhooks/env-init.sh", "LAYERX_WEBHOOKS_IDENTITY_TOKEN_FILE", "LAYERX_WEBHOOKS_SOURCE_TRIGGER_TOKEN_FILE")
	// File encoding/modes round-trip through the real protected secret reader.
	for _, target := range plan.Targets {
		info, e := os.Stat(filepath.Join(out, target.Role+"."+target.Target+".env"))
		if e != nil || info.Mode().Perm() != 0600 {
			t.Fatal("unprotected mint output")
		}
	}
	if e := plan.Write(out, io.Discard); e == nil {
		t.Fatal("filled mint overwritten")
	}
	m795Write(t, filepath.Join(out, "bad.env"), []byte("TOKEN=not-base64\n"))
	if _, e := ReadMintSecret(filepath.Join(out, "bad.env"), "TOKEN", true); e == nil {
		t.Fatal("malformed secret accepted")
	}
	// Reopen the old producer's committed public recording and perform an
	// explicit migration. Surviving bytes are retained; obsolete outputs stay
	// only in the untouched revocation inventory.
	legacy := m795Private(t)
	m795CopyTree(t, "tests/fixtures/repair_m795/legacy-mint", legacy)
	oldGateway := m795Read(t, filepath.Join(legacy, "router.railway.env"))
	migrated := filepath.Join(t.TempDir(), "migrated")
	if e := MigrateMintSecrets("../../deploy/rollout.kvx", "../../services/wallet/deploy/human.toml", legacy, migrated, io.Discard); e != nil {
		t.Fatal(e)
	}
	before, e := ReadMintSecret(filepath.Join(legacy, "router.railway.env"), "ENDPOINT_IDENTITY_TOKEN", true)
	if e != nil {
		t.Fatal(e)
	}
	after, e := ReadMintSecret(filepath.Join(migrated, "gateway.railway.env"), "ENDPOINT_IDENTITY_TOKEN", true)
	if e != nil || before != after {
		t.Fatal("migration rotated surviving identity token")
	}
	if !bytes.Equal(oldGateway, m795Read(t, filepath.Join(legacy, "router.railway.env"))) {
		t.Fatal("migration changed old revocation inventory")
	}
	if _, e = os.Stat(filepath.Join(migrated, "router.railway.env")); !os.IsNotExist(e) {
		t.Fatal("migration copied retired target")
	}
	if !bytes.Equal(m795Read(t, filepath.Join(legacy, "keys/wallet_identity_binding_key.pem")), m795Read(t, filepath.Join(migrated, "keys/wallet_identity_binding_key.pem"))) {
		t.Fatal("migration rotated wallet identity")
	}
	if !strings.Contains(string(m795Read(t, filepath.Join(migrated, "gateway.railway.sh"))), "--stdin --service router --environment") {
		t.Fatal("migration ignores rollout delivery target")
	}
	m795Wire(t, "mint-secrets.sh", "mint-token-plan", "migrate-retired", "mint-migrate")
}

// A loopback Docker API peer keeps its actual created objects independently
// from the lifecycle journal. It records engine requests and checks that the
// recovered ID was durably published before accepting deletion.
type m795DockerAPI struct {
	t                  *testing.T
	mu                 sync.Mutex
	directory          string
	objects            map[string]dockerContainer
	loseCreateResponse bool
	keepDeleted        bool
	inspectStatus      int
	deletes            int
	server             *httptest.Server
}

func m795DockerPeer(t *testing.T, directory string) *m795DockerAPI {
	t.Helper()
	peer := &m795DockerAPI{t: t, directory: directory, objects: map[string]dockerContainer{}}
	peer.server = httptest.NewServer(http.HandlerFunc(peer.serve))
	t.Cleanup(peer.server.Close)
	return peer
}
func (p *m795DockerAPI) serve(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/v1.41")
	if r.Method == "POST" && path == "/containers/create" {
		var config struct {
			Image      string
			Labels     map[string]string
			HostConfig struct {
				NetworkMode, CgroupnsMode   string
				CapDrop, CapAdd             []string
				Binds                       []string
				Memory, NanoCpus, PidsLimit int64
			}
		}
		if json.NewDecoder(r.Body).Decode(&config) != nil || config.Image == "" || config.HostConfig.NetworkMode != "none" || config.HostConfig.CgroupnsMode != "private" || config.HostConfig.Memory != 268435456 || config.HostConfig.NanoCpus != 1000000000 || config.HostConfig.PidsLimit != 128 || len(config.HostConfig.Binds) != 4 || !reflect.DeepEqual(config.HostConfig.CapDrop, []string{"ALL"}) {
			p.t.Error("Docker create lost production isolation configuration")
			http.Error(w, "configuration", 400)
			return
		}
		var state map[string]any
		if json.Unmarshal(m795Read(p.t, filepath.Join(p.directory, "lifecycle.json")), &state) != nil {
			p.t.Error("journal")
		}
		name := r.URL.Query().Get("name")
		if config.Labels[RegistryLifecycleLabel] != state["id"] || config.Labels[RegistryUIDLabel] != fmt.Sprint(os.Getuid()) {
			p.t.Error("create request has no persisted ownership labels")
			http.Error(w, "ownership", 400)
			return
		}
		object := dockerContainer{ID: strings.Repeat("a", 64), Name: "/" + name}
		object.Config.Labels = config.Labels
		p.objects[object.ID] = object
		w.WriteHeader(201)
		if p.loseCreateResponse {
			io.WriteString(w, `{"Id":`)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"Id": object.ID})
		return
	}
	reference := strings.TrimPrefix(path, "/containers/")
	if r.Method == "GET" && strings.HasSuffix(reference, "/json") {
		if p.inspectStatus != 0 {
			http.Error(w, "engine unavailable", p.inspectStatus)
			return
		}
		reference = strings.TrimSuffix(reference, "/json")
		for _, object := range p.objects {
			if object.ID == reference || object.Name == "/"+reference {
				json.NewEncoder(w).Encode(object)
				return
			}
		}
		http.NotFound(w, r)
		return
	}
	if r.Method == "DELETE" && strings.HasPrefix(path, "/containers/") {
		object, ok := p.objects[reference]
		if !ok {
			http.NotFound(w, r)
			return
		}
		var state map[string]any
		if json.Unmarshal(m795Read(p.t, filepath.Join(p.directory, "lifecycle.json")), &state) != nil {
			p.t.Error("journal")
		}
		durable := false
		for _, entry := range state["containers"].([]any) {
			record := entry.(map[string]any)
			if record["id"] == object.ID && record["phase"] == "removing" {
				durable = true
			}
		}
		if !durable || r.URL.Query().Get("force") != "1" {
			p.t.Error("container deletion preceded durable ownership publication")
			http.Error(w, "journal", 409)
			return
		}
		p.deletes++
		if !p.keepDeleted {
			delete(p.objects, reference)
		}
		w.WriteHeader(204)
		return
	}
	p.t.Errorf("unexpected Docker API request %s %s", r.Method, r.URL)
	http.NotFound(w, r)
}
func m795RegistryState(t *testing.T) (string, *m795DockerAPI, *RegistryLifecycle, string) {
	t.Helper()
	dir := m795Private(t)
	// These are the retained ownership fields emitted by prepare(); the quota
	// peer is outside this container-recovery test. Plan/Create/Cleanup below
	// are the same production implementation used by run_job and cleanup.
	m795Write(t, filepath.Join(dir, "lifecycle.json"), m795JSON(t, map[string]any{"schema_version": 1, "id": strings.Repeat("7", 32), "owner_uid": os.Getuid(), "directory": dir, "phase": "prepared", "containers": []any{}, "quota_root": filepath.Join(dir, "quota"), "jobs": []any{}, "mount": nil}))
	peer := m795DockerPeer(t, dir)
	store, e := OpenRegistryLifecycle(dir, DockerPeer{peer.server.Client(), peer.server.URL + "/v1.41"})
	if e != nil {
		t.Fatal(e)
	}
	name, e := store.Plan()
	if e != nil {
		t.Fatal(e)
	}
	return dir, peer, store, name
}
func TestRepairM_795_RegistryOrphanContainerRecovery(t *testing.T) {
	dir, peer, store, name := m795RegistryState(t)
	peer.loseCreateResponse = true
	configuration := m795JSON(t, RegistryCreateConfiguration("sha256:"+strings.Repeat("1", 64), filepath.Join(dir, name), filepath.Join(dir, "quota"), filepath.Join(dir, "builder")))
	if _, e := store.Create(context.Background(), name, configuration); e == nil {
		t.Fatal("lost Docker create response accepted")
	}
	if !bytes.Contains(m795Read(t, filepath.Join(dir, "lifecycle.json")), []byte(`"id": null`)) {
		t.Fatal("test did not stop in planned-to-ID publication window")
	}
	store.Close()
	reopened, e := OpenRegistryLifecycle(dir, DockerPeer{peer.server.Client(), peer.server.URL + "/v1.41"})
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if e = reopened.Cleanup(context.Background(), ""); e != nil {
		t.Fatal(e)
	}
	peer.mu.Lock()
	remaining, deletes := len(peer.objects), peer.deletes
	peer.mu.Unlock()
	if remaining != 0 || deletes != 1 {
		t.Fatalf("owned orphan survived recovery: objects=%d deletes=%d", remaining, deletes)
	}
	var state map[string]any
	if json.Unmarshal(m795Read(t, filepath.Join(dir, "lifecycle.json")), &state) != nil {
		t.Fatal("state")
	}
	record := state["containers"].([]any)[0].(map[string]any)
	if record["id"] != strings.Repeat("a", 64) || record["container_absent"] != true {
		t.Fatal("recovered identifier/absence not durable")
	}
	if e = reopened.Cleanup(context.Background(), ""); e != nil {
		t.Fatal("idempotent reopen cleanup", e)
	}
	t.Run("foreign-labels", func(t *testing.T) {
		_, peer, s, name := m795RegistryState(t)
		defer s.Close()
		if _, e := s.Create(context.Background(), name, configuration); e != nil {
			t.Fatal(e)
		}
		peer.mu.Lock()
		object := peer.objects[strings.Repeat("a", 64)]
		object.Config.Labels[RegistryLifecycleLabel] = strings.Repeat("f", 32)
		peer.objects[object.ID] = object
		peer.mu.Unlock()
		if e := s.Cleanup(context.Background(), name); e == nil {
			t.Fatal("foreign container ownership admitted")
		}
		if peer.deletes != 0 {
			t.Fatal("foreign container deleted")
		}
	})
	t.Run("absence-must-be-observed", func(t *testing.T) {
		dir, peer, s, name := m795RegistryState(t)
		defer s.Close()
		if _, e := s.Create(context.Background(), name, configuration); e != nil {
			t.Fatal(e)
		}
		peer.keepDeleted = true
		if e := s.Cleanup(context.Background(), name); e == nil {
			t.Fatal("engine reported removal while object remained")
		}
		if bytes.Contains(m795Read(t, filepath.Join(dir, "lifecycle.json")), []byte(`"container_absent": true`)) {
			t.Fatal("false absence recorded")
		}
	})
	t.Run("inspection-errors-are-not-absence", func(t *testing.T) {
		dir, peer, s, _ := m795RegistryState(t)
		defer s.Close()
		peer.inspectStatus = 500
		if e := s.Cleanup(context.Background(), ""); e == nil {
			t.Fatal("failed engine inspection accepted as resource absence")
		}
		if bytes.Contains(m795Read(t, filepath.Join(dir, "lifecycle.json")), []byte(`"container_absent": true`)) {
			t.Fatal("unproven absence recorded")
		}
	})
	m795Wire(t, "registry-artifacts.py", "registry-container-plan", "registry-container-create", "registry-container-clean", "container_absent")
}

func m795Readiness(now time.Time, ready bool) searchReadiness {
	checked := uint64(now.UnixMilli())
	state := "ready"
	if !ready {
		state = "unavailable"
	}
	deps := []searchDependency{}
	for _, name := range []string{"index", "content_storage", "payment_journal", "settlement_authority"} {
		last := checked
		deps = append(deps, searchDependency{name, state, true, &last})
	}
	return searchReadiness{1, ready, []searchRole{{"paid_delivery", state, deps, 30000, checked + 30000}}, checked, 125, 3}
}
func TestRepairM_795_SearchDefaultNodeDrift(t *testing.T) {
	inventory := m795Inventory(t)
	names := []string{}
	for _, endpoint := range inventory.Nodes {
		names = append(names, strings.TrimPrefix(endpoint, "https://"))
	}
	listing := "validator " + names[0] + "\nserve " + names[2] + "\nserve " + names[1] + "\n"
	selection, e := SelectSearchServing(inventory, strings.NewReader(listing), "", "")
	if e != nil || selection.Primary != names[1] || selection.Backup != names[2] {
		t.Fatalf("defaults did not filter/order actual serving inventory: %+v %v", selection, e)
	}
	cli, e := m795CLI([]string{"search-selection", "../../docs/site/docs/reference/public-rpc.md", "", ""}, []byte(listing))
	if e != nil || cli != names[1]+" "+names[2]+"\n" {
		t.Fatalf("CLI selector differs: %q %v", cli, e)
	}
	for _, pair := range [][2]string{{names[0], names[1]}, {names[1], names[0]}, {names[1], names[1]}, {"api15.mainnet-beta.paxeer.network", names[1]}, {names[1], "api16.mainnet-beta.paxeer.network"}, {"api1.paxeer.network", names[2]}} {
		if _, e = SelectSearchServing(inventory, strings.NewReader(listing), pair[0], pair[1]); e == nil {
			t.Fatalf("invalid serving pair accepted: %v", pair)
		}
	}
	if _, e = SelectSearchServing(inventory, strings.NewReader("serve "+names[1]+"\n"), "", ""); e == nil {
		t.Fatal("single serving node silently reused as backup")
	}
	if _, e = SelectSearchServing(inventory, strings.NewReader(listing+"validator "+names[1]+"\n"), "", ""); e == nil {
		t.Fatal("ambiguous validator/serving discovery accepted")
	}
	// Both readiness peers use the actual typed codec; TLS verifies the selected
	// inventory names. DNS alone is redirected to the disposable loopback peer.
	pair, cert, _ := m795TLSIdentity(t, selection.Backup)
	var mu sync.Mutex
	backupReady := true
	primarySeen, backupSeen := 0, 0
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/readyz" {
			t.Error("readiness transport path")
		}
		if r.Host == selection.Primary {
			primarySeen++
			w.WriteHeader(503)
			json.NewEncoder(w).Encode(m795Readiness(time.Now(), false))
			return
		}
		if r.Host != selection.Backup {
			t.Error("unregistered readiness host")
		}
		backupSeen++
		if !backupReady {
			w.WriteHeader(503)
		}
		json.NewEncoder(w).Encode(m795Readiness(time.Now(), backupReady))
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	client := m795NamedTLSClient(t, server, cert)
	// The primary deliberately has no valid leaf; a correct verified TLS client
	// must refuse it and consume the current backup's typed readiness instead.
	var rendered bytes.Buffer
	if e = RenderSearchFront(context.Background(), client, selection, &rendered); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(rendered.String(), "* "+selection.Backup+";") || !strings.Contains(rendered.String(), selection.Backup+" 1;") {
		t.Fatalf("readiness-gated backup not admitted: %s", rendered.String())
	}
	mu.Lock()
	backupReady = false
	mu.Unlock()
	rendered.Reset()
	if e = RenderSearchFront(context.Background(), client, selection, &rendered); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(rendered.String(), "* "+selection.Primary+";") || !strings.Contains(rendered.String(), selection.Primary+" 0;") {
		t.Fatal("unready inventory admitted traffic")
	}
	mu.Lock()
	observedBackup := backupSeen
	mu.Unlock()
	if observedBackup < 2 {
		t.Fatal("backup readiness consumer was not driven")
	}
	// Reject malformed/old readiness with valid HTTPS, not just failed TLS.
	invalid := m795Readiness(time.Now(), true)
	invalid.Checked = uint64(time.Now().Add(-time.Minute).UnixMilli())
	for i := range invalid.Roles[0].Dependencies {
		last := invalid.Checked
		invalid.Roles[0].Dependencies[i].LastSuccess = &last
	}
	bad := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(invalid) }))
	bad.TLS = &tls.Config{Certificates: []tls.Certificate{pair}}
	bad.StartTLS()
	defer bad.Close()
	if e = ProbeSearchReady(context.Background(), m795NamedTLSClient(t, bad, cert), "https://"+selection.Backup, time.Now()); e == nil {
		t.Fatal("stale typed readiness accepted")
	}
	m795Wire(t, "search-front.sh", "search_front_names", "search_front_select", "search-selection", "search-render", "search-ready")
}

func TestRepairM_795_ResyncSourceInterruption(t *testing.T) {
	journal := filepath.Join(m795Private(t), "resync.json")
	source, destination := "source@node.invalid", "target@node.invalid"
	s, e := OpenResyncState(journal)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Prepare(source, destination, "active"); e != nil {
		t.Fatal(e)
	}
	// The stop side effect happens only after durable Prepare. A source stopped
	// between preparation and copying must be recovered as well as mid-copy.
	unitFile := filepath.Join(filepath.Dir(journal), "unit-state")
	m795Write(t, unitFile, []byte("inactive"))
	s.Close()
	s, e = OpenResyncState(journal)
	if e != nil {
		t.Fatal(e)
	}
	record, e := s.Pending()
	if e != nil || record == nil || record.Source != source || record.Original != "active" || record.Phase != "prepared" {
		t.Fatalf("lost original source obligation: %+v %v", record, e)
	}
	if e = s.Prepare("another@node.invalid", "next@node.invalid", "active"); e == nil {
		t.Fatal("another destination admitted before recovery")
	}
	if e = s.Phase("restoring"); e != nil {
		t.Fatal(e)
	}
	if action, e := s.RecoveryAction(string(m795Read(t, unitFile))); e != nil || action != "start" {
		t.Fatalf("running source recovery action: %q %v", action, e)
	}
	if e = s.Restored(string(m795Read(t, unitFile))); e == nil {
		t.Fatal("stopped source marked recovered")
	}
	// The service-state peer stores the actual observable transition separately
	// from the journal. The acknowledgement consumer requires the original state.
	m795Write(t, unitFile, []byte("active"))
	if e = s.Restored(string(m795Read(t, unitFile))); e != nil {
		t.Fatal(e)
	}
	if e = s.Prepare(source, "second@node.invalid", "active"); e != nil {
		t.Fatal(e)
	}
	if e = s.Phase("copying"); e != nil {
		t.Fatal(e)
	}
	m795Write(t, unitFile, []byte("inactive"))
	s.Close()
	s, e = OpenResyncState(journal)
	if e != nil {
		t.Fatal(e)
	}
	record, e = s.Pending()
	if e != nil || record.Phase != "copying" {
		t.Fatal("copy interruption phase lost")
	}
	if action, e := s.RecoveryAction(string(m795Read(t, unitFile))); e != nil || action != "start" {
		t.Fatal("mid-copy source recovery not required")
	}
	// Repeated death during cleanup retains the same original source obligation.
	if e = s.Phase("restoring"); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = OpenResyncState(journal)
	if e != nil {
		t.Fatal(e)
	}
	m795Write(t, unitFile, []byte("active"))
	if e = s.Restored(string(m795Read(t, unitFile))); e != nil {
		t.Fatal(e)
	}
	for _, original := range []string{"inactive", "failed"} {
		if e = s.Prepare(source, destination, original); e != nil {
			t.Fatal(e)
		}
		if e = s.Phase("copying"); e != nil {
			t.Fatal(e)
		}
		s.Close()
		s, e = OpenResyncState(journal)
		if e != nil {
			t.Fatal(e)
		}
		r, e := s.Pending()
		if e != nil || r.Original != original {
			t.Fatal("intentionally stopped original state changed")
		}
		if e = s.Phase("restoring"); e != nil {
			t.Fatal(e)
		}
		if e = s.Restored("active"); e == nil {
			t.Fatal("intentionally stopped source was allowed to restart")
		}
		if action, e := s.RecoveryAction(original); e != nil || action != "none" {
			t.Fatal("stopped source restart action", action, e)
		}
		if action, e := s.RecoveryAction("active"); e == nil || action == "start" {
			t.Fatal("operator stopped source recovery not refused")
		}
		if e = s.Restored(original); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.Prepare(source, destination, "activating"); e == nil {
		t.Fatal("unstable source state admitted")
	}
	s.Close()
	m795Wire(t, "sync-fleet.sh", `resync_record prepare "$src" "$dest" "$original"`, `resync_record phase copying`, `trap resync_exit EXIT`, `trap 'exit 143' TERM`, `timeout 45s ssh`, `if [ "$action" = start ]; then`, `resync retained source recovery failed`, `resync_target "$k" "$src" || return 1`)
	sourceCode := string(m795Read(t, "sync-fleet.sh"))
	prepare := strings.Index(sourceCode, `resync_record prepare "$src"`)
	stop := strings.Index(sourceCode, `if [ "$original" = active ] && ! remote`)
	if prepare < 0 || stop < prepare {
		t.Fatal("source can stop before recovery journal is durable")
	}
	cli, e := m795CLI([]string{"resync-journal", journal, "pending"}, nil)
	if e != nil || cli != "" {
		t.Fatalf("cleaned source obligation returned after reopen: %q %v", cli, e)
	}
}
