package bringup

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func m798Read(t *testing.T, path string) []byte {
	t.Helper()
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func m798JSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func m798CLI(t *testing.T, args []string, raw []byte) ([]byte, error) {
	t.Helper()
	var output bytes.Buffer
	e := RunInspector(context.Background(), args, bytes.NewReader(raw), &output)
	return output.Bytes(), e
}
func m798Wire(t *testing.T, path string, terms ...string) {
	t.Helper()
	raw := m798Read(t, path)
	for _, term := range terms {
		if !bytes.Contains(raw, []byte(term)) {
			t.Fatalf("%s lacks production wiring %q", path, term)
		}
	}
}

type m798Native struct {
	Node, Comet string
	Client      *http.Client
	mu          sync.Mutex
	mode        string
	calls       map[string]int
	exchanges   map[string]json.RawMessage
}

func m798NativePeer(t *testing.T) *m798Native {
	t.Helper()
	var fixture struct {
		Exchanges []struct {
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
		} `json:"rpc_exchanges"`
	}
	if e := json.Unmarshal(m798Read(t, "../../services/platform/hosted/node/tests/fixtures/repair_22_31_355/native_configuration.json"), &fixture); e != nil {
		t.Fatal(e)
	}
	peer := &m798Native{calls: map[string]int{}, exchanges: map[string]json.RawMessage{}}
	for _, exchange := range fixture.Exchanges {
		peer.exchanges[exchange.Method] = exchange.Result
	}
	handler := func(owner string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			peer.mu.Lock()
			defer peer.mu.Unlock()
			var request struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      json.RawMessage `json:"id"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params"`
			}
			if r.Method != "POST" || r.URL.Path != "/" || json.NewDecoder(r.Body).Decode(&request) != nil || request.JSONRPC != "2.0" || string(request.ID) != "1" {
				t.Error("node/root transport contract violated")
				http.Error(w, "bad route", 400)
				return
			}
			expectedOwner := "node"
			if request.Method == "status" || request.Method == "commit" {
				expectedOwner = "comet"
			}
			if owner != expectedOwner || peer.exchanges[request.Method] == nil {
				t.Errorf("wrong owner for %s", request.Method)
				http.Error(w, "wrong owner", 400)
				return
			}
			if request.Method == "commit" && string(request.Params) != `{"height":"23869863"}` {
				t.Errorf("commit height not bound to node progress: %s", request.Params)
			}
			peer.calls[request.Method]++
			result := append(json.RawMessage(nil), peer.exchanges[request.Method]...)
			switch peer.mode {
			case "unreachable":
				http.Error(w, "recovery in progress", 503)
				return
			case "chain":
				if request.Method == "eth_chainId" {
					result = json.RawMessage(`"0x1"`)
				}
			case "catching":
				if request.Method == "status" {
					result = bytes.Replace(result, []byte(`"catching_up": false`), []byte(`"catching_up": true`), 1)
					result = bytes.Replace(result, []byte(`"catching_up":false`), []byte(`"catching_up":true`), 1)
				}
			case "noncanonical":
				if request.Method == "commit" {
					result = bytes.Replace(result, []byte(`"canonical": true`), []byte(`"canonical": false`), 1)
					result = bytes.Replace(result, []byte(`"canonical":true`), []byte(`"canonical":false`), 1)
				}
			case "mismatch":
				if request.Method == "commit" {
					result = bytes.ReplaceAll(result, []byte(`23869863`), []byte(`23869862`))
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
		}
	}
	node := httptest.NewServer(handler("node"))
	comet := httptest.NewServer(handler("comet"))
	t.Cleanup(node.Close)
	t.Cleanup(comet.Close)
	peer.Node = node.URL
	peer.Comet = comet.URL
	peer.Client = node.Client()
	return peer
}
func (p *m798Native) set(mode string) { p.mu.Lock(); defer p.mu.Unlock(); p.mode = mode }
func m798Observe(t *testing.T, peer *m798Native, unit, binary, state string) UpgradeObservation {
	t.Helper()
	transcript := m798Read(t, "tests/fixtures/repair_m798/systemd-"+state+".txt")
	transcript = bytes.ReplaceAll(transcript, []byte("{{PID}}"), []byte(strconv.Itoa(os.Getpid())))
	raw, e := m798CLI(t, []string{"upgrade-observe", unit, binary, peer.Node, peer.Comet, "v6.12.0", "23869862"}, transcript)
	if e != nil {
		t.Fatal("production observation failed", e)
	}
	var o UpgradeObservation
	if e = json.Unmarshal(raw, &o); e != nil {
		t.Fatal(e)
	}
	return o
}
func m798Plan(t *testing.T, peer *m798Native, binary string, count int) UpgradePlan {
	t.Helper()
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	digest, e := upgradeFileSHA(executable)
	if e != nil {
		t.Fatal(e)
	}
	plan := UpgradePlan{Version: "v6.12.0", Name: "v6.12.0", SHA256: digest, Height: 23869862, RecoverySeconds: 30}
	for i := 0; i < count; i++ {
		plan.Units = append(plan.Units, UpgradeUnit{ID: fmt.Sprintf("unit-%d", i), Destination: fmt.Sprintf("fixture-host-%d", i), Unit: fmt.Sprintf("paxd-%d.service", i), Binary: binary, Node: peer.Node, Comet: peer.Comet})
	}
	return plan
}
func m798Begin(t *testing.T, path string, plan UpgradePlan) {
	t.Helper()
	var lines strings.Builder
	for _, u := range plan.Units {
		fmt.Fprintln(&lines, u.ID, u.Destination, u.Unit, u.Binary, u.Node, u.Comet)
	}
	raw, e := m798CLI(t, []string{"upgrade-state", path, "begin", plan.Version, plan.Name, plan.SHA256, strconv.FormatUint(plan.Height, 10), strconv.FormatInt(plan.RecoverySeconds, 10)}, []byte(lines.String()))
	if e != nil || len(raw) != 0 {
		t.Fatalf("plan producer/consumer failed: %s %v", raw, e)
	}
}
func m798Open(t *testing.T, path string) *UpgradeState {
	t.Helper()
	s, e := OpenUpgradeState(path)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func m798Next(t *testing.T, path, id string, o UpgradeObservation, want string) {
	t.Helper()
	raw, e := m798CLI(t, []string{"upgrade-state", path, "next", id}, m798JSON(t, o))
	if e != nil || strings.TrimSpace(string(raw)) != want {
		t.Fatalf("unit %s action=%s want=%s error=%v", id, raw, want, e)
	}
}
func m798Applied(t *testing.T, path, id, action string) {
	t.Helper()
	if raw, e := m798CLI(t, []string{"upgrade-state", path, "applied", id, action}, nil); e != nil {
		t.Fatal(string(raw), e)
	}
}
func m798InstallExecutable(t *testing.T, path string) {
	t.Helper()
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	source, e := os.Open(executable)
	if e != nil {
		t.Fatal(e)
	}
	defer source.Close()
	staged := path + ".new"
	target, e := os.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if e != nil {
		t.Fatal(e)
	}
	_, copyErr := io.Copy(target, source)
	syncErr := target.Sync()
	closeErr := target.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		t.Fatal(copyErr, syncErr, closeErr)
	}
	if e = os.Rename(staged, path); e != nil {
		t.Fatal(e)
	}
}

func TestRepairM_798_UpgradeActiveState(t *testing.T) {
	for _, state := range []string{"inactive", "failed"} {
		t.Run(state, func(t *testing.T) {
			peer := m798NativePeer(t)
			dir := t.TempDir()
			binary := filepath.Join(dir, "paxd")
			m798InstallExecutable(t, binary)
			plan := m798Plan(t, peer, binary, 2)
			path := filepath.Join(dir, "upgrade.json")
			m798Begin(t, path, plan)
			o := m798Observe(t, peer, plan.Units[0].Unit, binary, state)
			if o.Active != state || o.Ready || o.PID != 0 || !o.Halt || o.DiskSHA != plan.SHA256 {
				t.Fatalf("producer lost inactive evidence: %+v", o)
			}
			s := m798Open(t, path)
			now := time.Unix(1700000000, 0)
			action, e := s.Next("unit-0", o, now)
			if e != nil || action != "restart" {
				t.Fatal(action, e)
			}
			if e = s.Applied("unit-0", "restart"); e != nil {
				t.Fatal(e)
			}
			action, e = s.Next("unit-0", o, now.Add(time.Duration(plan.RecoverySeconds+1)*time.Second))
			if e != nil || action != "failed" {
				t.Fatal("inactive unit passed recovery", action, e)
			}
			s.Close()
			s = m798Open(t, path)
			defer s.Close()
			r := s.document.Progress["unit-0"]
			if r.Phase != "failed" || len(r.Failures) != 1 || r.Failures[0].Observation.Active != state || r.Failures[0].Reason != "recovery deadline exceeded" {
				t.Fatalf("failure evidence not retained: %+v", r)
			}
			other := m798Observe(t, peer, plan.Units[1].Unit, binary, "inactive")
			action, e = s.Next("unit-1", other, now.Add(time.Duration(plan.RecoverySeconds+2)*time.Second))
			if e != nil || action != "blocked" || s.document.Progress["unit-1"].Phase != "waiting" {
				t.Fatal("later swap escaped failed recovery", action, e)
			}
			info, e := os.Stat(path)
			if e != nil || info.Mode().Perm() != 0600 {
				t.Fatal("upgrade evidence is not private", e)
			}
		})
	}
	m798Wire(t, "upgrade-fleet.sh", `upgrade_record next "unit-$k"`, `recover_unit "$k" || return 1`, `timeout 20s ssh`, `upgrade_record applied "unit-$k" restart`)
}

func TestRepairM_798_UpgradeBoundedCommittedRecovery(t *testing.T) {
	for _, mode := range []string{"unreachable", "chain", "catching", "noncanonical", "mismatch", "stalled", "old-running"} {
		t.Run(mode, func(t *testing.T) {
			peer := m798NativePeer(t)
			dir := t.TempDir()
			binary := filepath.Join(dir, "paxd")
			m798InstallExecutable(t, binary)
			plan := m798Plan(t, peer, binary, 2)
			path := filepath.Join(dir, "journal.json")
			m798Begin(t, path, plan)
			if mode == "old-running" {
				f, e := os.OpenFile(binary, os.O_APPEND|os.O_WRONLY, 0)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = f.WriteString("target-generation"); e != nil {
					t.Fatal(e)
				}
				f.Close()
				digest, e := upgradeFileSHA(binary)
				if e != nil {
					t.Fatal(e)
				}
				plan.SHA256 = digest
				// A separately named journal binds this target's actual installed bytes.
				path = filepath.Join(dir, "old-running.json")
				m798Begin(t, path, plan)
			} else if mode != "stalled" {
				peer.set(mode)
			}
			o := m798Observe(t, peer, plan.Units[0].Unit, binary, "active")
			if mode != "stalled" && mode != "old-running" && o.Ready {
				t.Fatalf("bad canonical readiness accepted: %+v", o)
			}
			if mode == "old-running" && (o.RunningSHA == o.DiskSHA || o.RunningSHA == plan.SHA256) {
				t.Fatal("producer hashed path instead of live process")
			}
			s := m798Open(t, path)
			now := time.Unix(1700000000, 0)
			action, e := s.Next("unit-0", o, now)
			if e != nil {
				t.Fatal(e)
			}
			if mode == "old-running" {
				if action != "restart" {
					t.Fatal(action)
				}
				if e = s.Applied("unit-0", "restart"); e != nil {
					t.Fatal(e)
				}
			} else if action != "recover" {
				t.Fatal(action)
			}
			action, e = s.Next("unit-0", o, now.Add(time.Duration(plan.RecoverySeconds+1)*time.Second))
			if e != nil || action != "failed" {
				t.Fatal("failed/stalled target marked current", action, e)
			}
			s.Close()
			reopened := m798Open(t, path)
			defer reopened.Close()
			retained := reopened.document.Progress["unit-0"]
			if len(retained.Failures) != 1 || retained.Failures[0].Observation != o || retained.Phase != "failed" {
				t.Fatal("readiness failure evidence changed on reopen")
			}
			other := m798Observe(t, peer, plan.Units[1].Unit, binary, "active")
			action, e = reopened.Next("unit-1", other, now.Add(time.Duration(plan.RecoverySeconds+2)*time.Second))
			if e != nil || action != "blocked" {
				t.Fatal("later unit was not stopped", action, e)
			}
		})
	}
}

func TestRepairM_798_UpgradeHashOnlyRestartRecovery(t *testing.T) {
	peer := m798NativePeer(t)
	dir := t.TempDir()
	binary := filepath.Join(dir, "paxd")
	if e := os.WriteFile(binary, []byte("old generation"), 0700); e != nil {
		t.Fatal(e)
	}
	plan := m798Plan(t, peer, binary, 1)
	path := filepath.Join(dir, "journal.json")
	m798Begin(t, path, plan)
	o := m798Observe(t, peer, plan.Units[0].Unit, binary, "inactive")
	m798Next(t, path, "unit-0", o, "swap")
	// Stop after intent and reopen before mv. The unperformed swap remains due.
	s := m798Open(t, path)
	if s.document.Progress["unit-0"].Phase != "installing" {
		t.Fatal("swap intent not durable")
	}
	s.Close()
	m798Begin(t, path, plan)
	m798Next(t, path, "unit-0", o, "swap")
	// Stop after atomic installation, before either acknowledgement or restart.
	m798InstallExecutable(t, binary)
	s = m798Open(t, path)
	s.Close()
	m798Begin(t, path, plan)
	installed := m798Observe(t, peer, plan.Units[0].Unit, binary, "inactive")
	if installed.DiskSHA != plan.SHA256 || installed.RunningSHA != "" {
		t.Fatal("installation fixture not observed through producer")
	}
	m798Next(t, path, "unit-0", installed, "restart")
	// Stop after actual activation but before acknowledgement. The running test
	// process supplies real /proc executable bytes and start identity.
	peer.set("catching")
	active := m798Observe(t, peer, plan.Units[0].Unit, binary, "active")
	if active.RunningSHA != plan.SHA256 || active.StartTicks == 0 || active.Ready {
		t.Fatalf("activation producer invalid: %+v", active)
	}
	s = m798Open(t, path)
	s.Close()
	m798Begin(t, path, plan)
	m798Next(t, path, "unit-0", active, "recover")
	// Stop and reopen the recovering component. Canonical progress now clears
	// the original halt baseline, without a duplicate restart.
	s = m798Open(t, path)
	s.Close()
	m798Begin(t, path, plan)
	peer.set("")
	recovered := m798Observe(t, peer, plan.Units[0].Unit, binary, "active")
	if !recovered.Ready || recovered.Height != 23869863 {
		t.Fatalf("recorded committed transport not consumed: %+v", recovered)
	}
	m798Next(t, path, "unit-0", recovered, "current")
	s = m798Open(t, path)
	if s.document.Progress["unit-0"].Phase != "current" || s.document.Progress["unit-0"].Last != recovered {
		t.Fatal("recovered target not durable")
	}
	s.Close()
	// A successful prior run cannot substitute for fresh progress on reopen.
	m798Begin(t, path, plan)
	m798Next(t, path, "unit-0", recovered, "recover")
	m798Wire(t, "upgrade-fleet.sh", `upgrade_record begin`, `systemctl restart --no-block`, `upgrade-observe`, `/var/lib/paxeer-bringup/upgrade-$version.json`)
}

func TestRepairM_798_UpgradeSharedBinaryPerUnit(t *testing.T) {
	peer := m798NativePeer(t)
	dir := t.TempDir()
	binary := filepath.Join(dir, "paxd")
	m798InstallExecutable(t, binary)
	plan := m798Plan(t, peer, binary, 2)
	// Both units are on the same host/path, with independent per-unit RPCs.
	second := m798NativePeer(t)
	plan.Units[1].Destination = plan.Units[0].Destination
	plan.Units[1].Node = second.Node
	plan.Units[1].Comet = second.Comet
	path := filepath.Join(dir, "journal.json")
	m798Begin(t, path, plan)
	firstStopped := m798Observe(t, peer, plan.Units[0].Unit, binary, "inactive")
	m798Next(t, path, "unit-0", firstStopped, "restart")
	m798Applied(t, path, "unit-0", "restart")
	beforeFirstRecovered := m798Observe(t, second, plan.Units[1].Unit, binary, "inactive")
	m798Next(t, path, "unit-1", beforeFirstRecovered, "blocked")
	firstActive := m798Observe(t, peer, plan.Units[0].Unit, binary, "active")
	m798Next(t, path, "unit-0", firstActive, "current")
	secondStopped := m798Observe(t, second, plan.Units[1].Unit, binary, "inactive")
	if secondStopped.DiskSHA != firstActive.DiskSHA {
		t.Fatal("not a shared installed generation")
	}
	m798Next(t, path, "unit-1", secondStopped, "restart")
	// A crash with unit 1's pending restart cannot erase unit 0's completion.
	s := m798Open(t, path)
	if s.document.Progress["unit-0"].Phase != "current" || s.document.Progress["unit-1"].Phase != "restarting" {
		t.Fatal("shared path collapsed unit progress")
	}
	s.Close()
	secondActive := m798Observe(t, second, plan.Units[1].Unit, binary, "active")
	m798Next(t, path, "unit-1", secondActive, "current")
	s = m798Open(t, path)
	defer s.Close()
	if s.document.Progress["unit-0"].Last.Unit == s.document.Progress["unit-1"].Last.Unit {
		t.Fatal("unit identities collapsed")
	}
	duplicate := plan
	duplicate.Units = append([]UpgradeUnit(nil), plan.Units...)
	duplicate.Units[1].Node = duplicate.Units[0].Node
	if e := duplicate.validate(); e == nil {
		t.Fatal("shared RPC endpoint substituted for per-unit progress")
	}
}

func TestRepairM_798_UpgradeJournalRefusals(t *testing.T) {
	peer := m798NativePeer(t)
	dir := t.TempDir()
	binary := filepath.Join(dir, "paxd")
	m798InstallExecutable(t, binary)
	plan := m798Plan(t, peer, binary, 1)
	path := filepath.Join(dir, "journal.json")
	m798Begin(t, path, plan)
	s := m798Open(t, path)
	changed := plan
	changed.SHA256 = strings.Repeat("a", 64)
	if s.Begin(changed) == nil {
		t.Fatal("foreign generation adopted journal")
	}
	changed = plan
	changed.Units = append([]UpgradeUnit(nil), plan.Units...)
	changed.Units[0].Binary = "/different/paxd"
	if s.Begin(changed) == nil {
		t.Fatal("foreign inventory adopted journal")
	}
	o := m798Observe(t, peer, plan.Units[0].Unit, binary, "active")
	o.Unit = "wrong.service"
	if _, e := s.Next("unit-0", o, time.Now()); e == nil {
		t.Fatal("cross-unit evidence admitted")
	}
	s.Close()
	if s.Begin(plan) == nil {
		t.Fatal("closed component accepted work")
	}
	raw := m798Read(t, path)
	if e := os.WriteFile(path, bytes.Replace(raw, []byte(`"waiting"`), []byte(`"unrecorded"`), 1), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := OpenUpgradeState(path); e == nil {
		t.Fatal("corrupted recovery phase ignored")
	}
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Chmod(path, 0644); e != nil {
		t.Fatal(e)
	}
	if _, e := OpenUpgradeState(path); e == nil {
		t.Fatal("public upgrade inventory/evidence accepted")
	}
	for _, input := range []string{"active\n", "ActiveState=active\nMainPID=0\n", "ActiveState=failed\nActiveState=active\nMainPID=0\n@@journal\n"} {
		if _, e := ObserveUpgrade(context.Background(), peer.Client, "paxd", binary, peer.Node, peer.Comet, "v6.12.0", 23869862, strings.NewReader(input)); e == nil {
			t.Fatal("unstructured or duplicate unit state accepted")
		}
	}
}

type m798Hosted struct {
	URL        string
	Client     *http.Client
	mu         sync.Mutex
	mode       string
	calls      map[string]int
	principals map[string]string
	assets     []byte
}

func m798HostedPeer(t *testing.T) *m798Hosted {
	t.Helper()
	peer := &m798Hosted{calls: map[string]int{}, principals: map[string]string{}, assets: m798Read(t, "tests/fixtures/repair_m798/xweb-assets.json")}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer.mu.Lock()
		defer peer.mu.Unlock()
		var request struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if r.URL.Path != "/tenant/beta/rpc" || r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" || json.NewDecoder(r.Body).Decode(&request) != nil || request.JSONRPC != "2.0" || string(request.ID) != "1" {
			t.Error("configured hosted path was not preserved")
			http.Error(w, "wrong route", 400)
			return
		}
		peer.calls[request.Method]++
		var result any
		switch request.Method {
		case "px_listAssets":
			if string(request.Params) != "[]" {
				t.Error("asset discovery params changed")
			}
			var rows map[string]any
			if e := json.Unmarshal(peer.assets, &rows); e != nil {
				t.Error(e)
				return
			}
			row := rows["assets"].([]any)[0].(map[string]any)
			switch peer.mode {
			case "paused":
				row["paused"] = true
			case "decimals":
				row["decimals"] = float64(6)
			case "symbol":
				row["symbol"] = "PAX"
			case "unverified":
				row["verification"] = "indexer_projection"
			case "missing":
				rows["assets"] = rows["assets"].([]any)[1:]
			case "duplicate":
				rows["assets"] = append(rows["assets"].([]any), row)
			case "nested":
				rows["assets"].([]any)[0] = map[string]any{"asset_id": row["asset_id"], "layerx": row}
			case "unavailable":
				http.Error(w, "unavailable", 503)
				return
			}
			result = rows
		case "px_register":
			var args []string
			if json.Unmarshal(request.Params, &args) != nil || len(args) != 2 {
				t.Error("registration params changed")
				http.Error(w, "bad params", 400)
				return
			}
			public, e := hex.DecodeString(args[0])
			signature, e2 := hex.DecodeString(args[1])
			if e != nil || e2 != nil || len(public) != 32 || len(signature) != 64 {
				t.Error("invalid registration bytes")
				return
			}
			// Independent gateway peer uses the deployed length-prefixed domain
			// framing, verifies the actual producer's Ed25519 signature, then persists
			// the public principal in its in-memory identity store.
			tagged := func(domain string) []byte {
				var framing bytes.Buffer
				for _, part := range [][]byte{[]byte(domain), []byte("beta"), public} {
					if e := binary.Write(&framing, binary.BigEndian, uint64(len(part))); e != nil {
						t.Error(e)
					}
					framing.Write(part)
				}
				hash := sha256.Sum256(framing.Bytes())
				return hash[:]
			}
			if !ed25519.Verify(public, tagged("layerx-register-binding-v1"), signature) {
				t.Error("producer signed a different registration binding")
				http.Error(w, "signature", 400)
				return
			}
			subject := "beta." + hex.EncodeToString(tagged("layerx-register-subject-v1"))[:32]
			if prior := peer.principals[args[0]]; prior != "" && prior != subject {
				t.Error("principal is not idempotent")
			}
			peer.principals[args[0]] = subject
			if peer.mode == "principal" {
				subject = "beta.foreign"
			}
			result = map[string]any{"tenant": "beta", "sub": subject, "allowed_signer_public_keys": []string{args[0]}}
		default:
			t.Errorf("node method %s reached hosted gateway", request.Method)
			http.Error(w, "wrong owner", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	t.Cleanup(server.Close)
	peer.URL = server.URL + "/tenant/beta/rpc"
	peer.Client = server.Client()
	return peer
}
func (p *m798Hosted) set(mode string) { p.mu.Lock(); defer p.mu.Unlock(); p.mode = mode }
func m798XwebOptions(t *testing.T, hosted *m798Hosted, node *m798Native) XwebOptions {
	t.Helper()
	values := map[string]string{"XWEB_GATEWAY_ENDPOINT": hosted.URL, "XWEB_EVM_ENDPOINT": node.Node, "XWEB_SEEDS": "https://example.com/"}
	var assets struct {
		Assets []struct {
			ID       string `json:"asset_id"`
			Symbol   string `json:"symbol"`
			Decimals int    `json:"decimals"`
		} `json:"assets"`
	}
	if e := json.Unmarshal(hosted.assets, &assets); e != nil {
		t.Fatal(e)
	}
	for _, a := range assets.Assets {
		values["XWEB_ASSET_ID_"+a.Symbol] = a.ID
		values["XWEB_DECIMALS_"+a.Symbol] = strconv.Itoa(a.Decimals)
	}
	options, e := XwebEnvironment(func(key string) string { return values[key] })
	if e != nil {
		t.Fatal(e)
	}
	return options
}
func TestRepairM_798_XwebHostedAndNodeTransports(t *testing.T) {
	hosted := m798HostedPeer(t)
	node := m798NativePeer(t)
	options := m798XwebOptions(t, hosted, node)
	// The trust record and receipt below come from the actual native chain-125
	// conformance producer; never synthesize a signature to match the consumer.
	chain := m793RecordedChain(t)
	node.exchanges["px_getReceiptKeys"] = m798JSON(t, map[string]any{"keys": []any{chain.key}})
	prices, e := XwebPrices(context.Background(), hosted.Client, options)
	if e != nil {
		t.Fatal(e)
	}
	expected := map[string]string{"SID": "231696428571429", "PAX": "74404761904762", "USDC": "1000", "USDL": "1000"}
	for sym, want := range expected {
		if prices[sym].Price != want || prices[sym].ID != options.Assets[sym].ID {
			t.Fatal("approved base-unit price changed", sym, prices[sym])
		}
	}
	directory := t.TempDir()
	names := []string{"api1.mainnet-beta.paxeer.network", "api2.mainnet-beta.paxeer.network"}
	for _, kind := range []string{"serve", "attestor"} {
		outputDir := filepath.Join(directory, kind)
		var out bytes.Buffer
		if e = RenderXweb(context.Background(), hosted.Client, options, kind, outputDir, names, &out); e != nil {
			t.Fatal(e)
		}
		files, e := os.ReadDir(outputDir)
		if e != nil {
			t.Fatal(e)
		}
		wantCount := 2
		if kind == "attestor" {
			wantCount = 4
		}
		if len(files) != wantCount {
			t.Fatal("renderer lost consumers", kind, len(files))
		}
		for index, file := range files {
			raw := m798Read(t, filepath.Join(outputDir, file.Name()))
			var config struct {
				Gateway struct {
					Endpoint string           `json:"endpoint"`
					Keys     []XwebReceiptKey `json:"receipt_keys"`
				} `json:"gateway"`
				EVM struct {
					Endpoint string `json:"endpoint"`
					ChainID  int    `json:"chain_id"`
				} `json:"evm"`
				Assets  map[string]XwebAsset `json:"assets"`
				Network int                  `json:"kernel_network_id"`
				Peers   []string             `json:"peers"`
			}
			if e = json.Unmarshal(raw, &config); e != nil {
				t.Fatal(e)
			}
			if config.Gateway.Endpoint != hosted.URL || config.Network != 125 || config.EVM.ChainID != 125 || len(config.Gateway.Keys) != 1 || len(config.Assets) != 4 {
				t.Fatal("configuration lost real transport/trust", string(raw))
			}
			for sym, want := range expected {
				if config.Assets[sym].Price != want || config.Assets[sym].ID != options.Assets[sym].ID {
					t.Fatal("loader input lost verified price", sym)
				}
			}
			if kind == "serve" {
				if config.EVM.Endpoint != node.Node || len(config.Peers) != 1 {
					t.Fatal("serving node root changed")
				}
				// Drive the endpoint loaded from actual renderer output across HTTP.
				answer, e := callRPC(context.Background(), node.Client, config.EVM.Endpoint, "eth_chainId", []any{}, "", false)
				if e != nil || string(answer) != `"0x7d"` {
					t.Fatal("loaded node transport failed", e)
				}
			} else if config.EVM.Endpoint != "https://"+names[index%len(names)] || len(config.Peers) != 3 {
				t.Fatal("attestor root/peers changed")
			}
			answer, e := callRPC(context.Background(), hosted.Client, config.Gateway.Endpoint, "px_listAssets", []any{}, "", true)
			if e != nil || len(answer) == 0 {
				t.Fatal("loaded gateway transport failed", e)
			}
			key := config.Gateway.Keys[0]
			transported := m798JSON(t, map[string]any{"keys": []any{map[string]any{"keyId": "0x" + key.ID, "publicKey": "0x" + key.Public, "firstHeight": key.First, "lastHeight": key.Last}}})
			if e = VerifyCommittedReceipt(m798JSON(t, chain.native), transported, chain.hash); e != nil {
				t.Fatal("actual receipt consumer refused rendered trust", e)
			}
			if bytes.Contains(raw, []byte(`"kernel"`)) || bytes.Contains(raw, []byte("sequencer_public_key")) {
				t.Fatal("retired sidecar configuration emitted")
			}
		}
	}
	hosted.mu.Lock()
	assetCalls := hosted.calls["px_listAssets"]
	registrationCalls := hosted.calls["px_register"]
	hosted.mu.Unlock()
	node.mu.Lock()
	historyCalls := node.calls["px_getReceiptKeys"]
	chainCalls := node.calls["eth_chainId"]
	node.mu.Unlock()
	if assetCalls != 9 || registrationCalls != 0 || historyCalls != 2 || chainCalls != 2 {
		t.Fatal("producer/config-consumer path missing", assetCalls, registrationCalls, historyCalls, chainCalls)
	}
	// The native integration path still imports and invokes the real Rust
	// loader. No Rust execution is claimed by this Go-only gate.
	m798Wire(t, "tests/xweb-loader/src/main.rs", "services/interop/crates/x-websearch/src/config.rs", "config::load")
	m798Wire(t, "tests/test_xweb_config.py", "load_config(path)", "load_config(altered, accepted=False)", `["gateway"]["endpoint"]`)
	m798Wire(t, "xweb-config.sh", `xweb_gateway="${XWEB_GATEWAY_ENDPOINT:-}"`, `xweb-config render "$kind"`, `xweb-config prices`, `xweb-config register`, "never append /rpc")
}

func TestRepairM_798_XwebRegistration(t *testing.T) {
	hosted := m798HostedPeer(t)
	directory := t.TempDir()
	path := filepath.Join(directory, "ed25519.key")
	seed := "9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60"
	if e := os.WriteFile(path, []byte(seed+"\n"), 0600); e != nil {
		t.Fatal(e)
	}
	var previous string
	for attempt := 0; attempt < 2; attempt++ {
		var output bytes.Buffer
		if e := RegisterXweb(context.Background(), hosted.Client, hosted.URL, path, &output); e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(output.String(), "public_key=d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a") || strings.Contains(output.String(), seed) {
			t.Fatal("registration lost key or disclosed custody", output.String())
		}
		if attempt == 1 && output.String() != previous {
			t.Fatal("repeated key changed public principal")
		}
		previous = output.String()
	}
	hosted.mu.Lock()
	principalCount := len(hosted.principals)
	calls := hosted.calls["px_register"]
	hosted.mu.Unlock()
	if principalCount != 1 || calls != 2 {
		t.Fatal("real identity peer did not retain registration", principalCount, calls)
	}
	hosted.set("principal")
	var out bytes.Buffer
	if e := RegisterXweb(context.Background(), hosted.Client, hosted.URL, path, &out); e == nil || out.Len() != 0 {
		t.Fatal("foreign registration principal accepted")
	}
	if e := os.Chmod(path, 0644); e != nil {
		t.Fatal(e)
	}
	if e := RegisterXweb(context.Background(), hosted.Client, hosted.URL, path, &out); e == nil || out.Len() != 0 {
		t.Fatal("unprotected registration seed used")
	}
	if e := os.Chmod(path, 0600); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(directory, "link")
	if e := os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if e := RegisterXweb(context.Background(), hosted.Client, hosted.URL, link, &out); e == nil {
		t.Fatal("symlink registration key used")
	}
	if e := RegisterXweb(context.Background(), hosted.Client, "", path, &out); e == nil {
		t.Fatal("registration invented public gateway default")
	}
	if e := os.WriteFile(path, []byte("not-an-ed25519-key"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := RegisterXweb(context.Background(), hosted.Client, hosted.URL, path, &out); e == nil {
		t.Fatal("malformed seed used")
	}
	hosted.mu.Lock()
	defer hosted.mu.Unlock()
	if hosted.calls["px_register"] != 3 {
		t.Fatal("custody refusal still sent registration")
	}
}

func TestRepairM_798_XwebRetiredRouteAndMetadataRefusals(t *testing.T) {
	hosted := m798HostedPeer(t)
	node := m798NativePeer(t)
	options := m798XwebOptions(t, hosted, node)
	for _, mode := range []string{"paused", "decimals", "symbol", "unverified", "missing", "duplicate", "nested", "unavailable"} {
		t.Run(mode, func(t *testing.T) {
			hosted.set(mode)
			if _, e := XwebPrices(context.Background(), hosted.Client, options); e == nil {
				t.Fatal("unproven asset inventory admitted")
			}
			dir := filepath.Join(t.TempDir(), "config")
			var out bytes.Buffer
			if e := RenderXweb(context.Background(), hosted.Client, options, "serve", dir, []string{"api1.mainnet-beta.paxeer.network"}, &out); e == nil || out.Len() != 0 {
				t.Fatal("config published despite unproven pricing")
			}
			if _, e := os.Stat(dir); !os.IsNotExist(e) {
				t.Fatal("refused config left published files")
			}
		})
	}
	hosted.set("")
	for _, mode := range []string{"no-gateway", "same-node", "node-rpc-path", "history-rpc-path", "unprotected-gateway", "wrong-price"} {
		t.Run(mode, func(t *testing.T) {
			changed := options
			switch mode {
			case "no-gateway":
				changed.Gateway = ""
			case "same-node":
				changed.Gateway = node.Node
			case "node-rpc-path":
				changed.Node = node.Node + "/rpc"
			case "history-rpc-path":
				changed.ReceiptNode = node.Node + "/rpc"
			case "unprotected-gateway":
				changed.Gateway = "http://gateway.example/rpc"
			case "wrong-price":
				changed.PAXUSD = "13.43"
			}
			if _, e := XwebPrices(context.Background(), hosted.Client, changed); e == nil {
				t.Fatal("retired or unconfigured route/price admitted")
			}
		})
	}
	// A supplied hosted URL is consumed verbatim; no public-router or node
	// fallback can rescue an unavailable operator deployment.
	hosted.set("unavailable")
	if _, e := XwebPrices(context.Background(), hosted.Client, options); e == nil {
		t.Fatal("gateway failure hidden")
	}
	source := string(m798Read(t, "xweb-config.sh"))
	if strings.Contains(source, "https://api-mainnet-beta.paxeer.network/rpc") || strings.Contains(source, "gateway's /rpc") {
		t.Fatal("retired default/help retained")
	}
	m798Wire(t, "xweb-config.sh", `rpc(node, "eth_call"`, `gateway, node, prices, *names = sys.argv[1:]`, `' "$xweb_gateway" "$xweb_evm" "$prices"`)
}
