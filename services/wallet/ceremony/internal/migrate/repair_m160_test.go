package migrate

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/attestor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/envelope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/testsupport"
)

// These fixtures are peers of the production migration, not replacements for
// its planner, client, binding verification, journal or recovery implementation.
// They keep wallet rows in memory and serve the attestor HTTP contract over
// pinned mutual TLS. No database daemon, toolchain subprocess or live RPC is used.
type m160Wallet struct {
	Wallet
	original      originalIdentity
	secp          *ecdsa.PrivateKey
	identity      ed25519.PrivateKey
	fundedAccount bool
	attestorID    string
}

type m160Rows struct {
	columns []string
	values  [][]driver.Value
	offset  int
}

func (r *m160Rows) Columns() []string { return r.columns }
func (r *m160Rows) Close() error      { return nil }
func (r *m160Rows) Next(out []driver.Value) error {
	if r.offset == len(r.values) {
		return io.EOF
	}
	copy(out, r.values[r.offset])
	r.offset++
	return nil
}
func m160Result(values ...[]driver.Value) driver.Rows {
	columns := make([]string, 0)
	if len(values) != 0 {
		for i := range values[0] {
			columns = append(columns, fmt.Sprintf("column%d", i))
		}
	}
	return &m160Rows{columns: columns, values: values}
}

type m160Database struct {
	mu      sync.Mutex
	wallets []*m160Wallet
	schema  bool
	locks   map[string]bool
}
type m160Connector struct{ data *m160Database }

func (c m160Connector) Connect(context.Context) (driver.Conn, error) {
	return &m160Conn{data: c.data}, nil
}
func (c m160Connector) Driver() driver.Driver { return m160Driver{} }

type m160Driver struct{}

func (m160Driver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use the in-memory connector")
}

type m160Conn struct {
	data    *m160Database
	updates map[string]string
}

func (c *m160Conn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fixture supports direct context queries only")
}
func (c *m160Conn) Close() error { return nil }
func (c *m160Conn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *m160Conn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	if c.updates != nil {
		return nil, errors.New("nested transaction")
	}
	c.updates = map[string]string{}
	return c, nil
}
func (c *m160Conn) Commit() error {
	c.data.mu.Lock()
	defer c.data.mu.Unlock()
	for _, w := range c.data.wallets {
		if key, ok := c.updates[w.ID]; ok {
			w.Migrated = true
			w.attestorID = key
		}
	}
	c.updates = nil
	return nil
}
func (c *m160Conn) Rollback() error { c.updates = nil; return nil }
func (c *m160Conn) row(w *m160Wallet) []driver.Value {
	migrated := w.Migrated
	if _, ok := c.updates[w.ID]; ok {
		migrated = true
	}
	return []driver.Value{w.ID, w.UserID, w.Address, w.EncryptedPrivateKey, int64(w.KeyVersion), w.ChainID, w.Kind, w.Kind == "funded" || w.fundedAccount, migrated, w.DID, w.MainAccountID, w.OriginalIdentityKeyID}
}
func (c *m160Conn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.data.mu.Lock()
	defer c.data.mu.Unlock()
	switch {
	case strings.Contains(query, "from information_schema.columns"):
		n := int64(0)
		if c.data.schema {
			n = 1
		}
		return m160Result([]driver.Value{n}), nil
	case strings.Contains(query, "pg_try_advisory_lock"):
		key := args[0].Value.(string)
		free := !c.data.locks[key]
		if free {
			c.data.locks[key] = true
		}
		return m160Result([]driver.Value{free}), nil
	case strings.Contains(query, "from wallets w order by"):
		var rows [][]driver.Value
		for _, w := range c.data.wallets {
			rows = append(rows, c.row(w))
		}
		return m160Result(rows...), nil
	case strings.Contains(query, "from account_provisioning"):
		for _, w := range c.data.wallets {
			if w.ID == args[0].Value && w.UserID == args[1].Value && w.Kind == args[2].Value {
				return m160Result([]driver.Value{w.original.PublicKey, w.original.KeyID, w.DID, w.MainAccountID, fmt.Sprint(w.original.BindNonce), w.original.BindSignature, "active"}), nil
			}
		}
		return m160Result(), nil
	case strings.Contains(query, "from agent_principals"):
		for _, w := range c.data.wallets {
			if w.ID == args[0].Value {
				return m160Result([]driver.Value{w.original.PublicKey, w.UserID}), nil
			}
		}
		return m160Result(), nil
	case strings.Contains(query, "from wallets w where id="):
		for _, w := range c.data.wallets {
			if w.ID == args[0].Value {
				return m160Result(c.row(w)), nil
			}
		}
		return m160Result(), nil
	case strings.Contains(query, "select coalesce(attestor_key_id"):
		for _, w := range c.data.wallets {
			if w.ID == args[0].Value {
				return m160Result([]driver.Value{w.attestorID, w.OriginalIdentityKeyID}), nil
			}
		}
		return m160Result(), nil
	default:
		return nil, fmt.Errorf("unsupported fixture query: %s", query)
	}
}
func (c *m160Conn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.data.mu.Lock()
	defer c.data.mu.Unlock()
	switch {
	case strings.Contains(query, "pg_advisory_unlock"):
		delete(c.data.locks, args[0].Value.(string))
		return driver.RowsAffected(1), nil
	case strings.HasPrefix(query, "update wallets set migrated_at"):
		if c.updates == nil {
			return nil, errors.New("migration mark outside transaction")
		}
		for _, w := range c.data.wallets {
			if w.ID == args[0].Value && !w.Migrated {
				c.updates[w.ID] = args[1].Value.(string)
				return driver.RowsAffected(1), nil
			}
		}
		return driver.RowsAffected(0), nil
	case query == "alter table wallets drop column migrated_at":
		c.data.schema = false
		return driver.RowsAffected(0), nil
	default:
		return nil, fmt.Errorf("unsupported fixture mutation: %s", query)
	}
}

// Fixture peers persist descriptions between closes. The production consumer
// must reopen its sealed journal and authenticate each member's base/base+1
// transition before it can issue a refresh replay.
type m160PeerKey struct {
	Description   attestor.DescribeResponse
	Imports       int
	Refreshes     []string
	Verifications int
	OwnerSigns    int
}
type m160Peers struct {
	mu             sync.Mutex
	root           string
	tls            *testsupport.TLSFiles
	gateway        *testsupport.TLSFiles
	certificates   []tls.Certificate
	config         attestor.Config
	servers        []*httptest.Server
	states         []map[string]*m160PeerKey
	keys           map[string]*m160Wallet
	interrupt      string
	incomplete     string
	corrupt        string
	protocolErrors []string
}

func m160JSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
func m160APIError(w http.ResponseWriter, status int, code string) {
	w.WriteHeader(status)
	m160JSON(w, attestor.ErrorBody{Error: attestor.ErrorDetail{Category: "key", Code: code, Message: code}})
}
func (p *m160Peers) ids() []string { return []string{"n1", "n2", "n3", "n4", "n5"} }
func (p *m160Peers) checkpoint(i int) error {
	raw, err := json.Marshal(p.states[i])
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(p.root, fmt.Sprintf("member%d.json", i)), raw, 0600)
}
func (p *m160Peers) fail(w http.ResponseWriter, format string, args ...any) {
	p.protocolErrors = append(p.protocolErrors, fmt.Sprintf(format, args...))
	m160APIError(w, 409, "fixture_contract_mismatch")
}
func (p *m160Peers) serve(i int, w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := p.ids()[i]
	if r.Method != http.MethodPost || r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		p.fail(w, "unauthenticated request")
		return
	}
	var body struct {
		SessionID string `json:"session_id"`
		KeyID     string `json:"key_id"`
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil || json.Unmarshal(raw, &body) != nil || body.SessionID == "" || body.KeyID == "" {
		p.fail(w, "invalid request")
		return
	}
	held := p.states[i][body.KeyID]
	if r.URL.Path == attestor.PathImport {
		var req attestor.ImportRequest
		if strictJSON(raw, &req) != nil {
			p.fail(w, "import codec")
			return
		}
		bundle, e := attestor.DecodeBundle(req.Share)
		if e != nil || bundle.Validate() != nil || req.Share.ParticipantID != id || req.Epoch != 0 || req.CeremonyID == "" || req.PublicKey == "" || req.Threshold != attestor.SignQuorum {
			p.fail(w, "import authority")
			return
		}
		public, e := attestor.PublicKeyHex(bundle.PublicKey)
		if e != nil || public != req.PublicKey || !reflect.DeepEqual(req.Participants, p.ids()) {
			p.fail(w, "import identity")
			return
		}
		if held != nil && held.Description.ImportSessionID != req.SessionID {
			p.fail(w, "replacement import")
			return
		}
		if held == nil {
			digest := sha256.Sum256(raw)
			held = &m160PeerKey{Description: attestor.DescribeResponse{NodeID: id, KeyID: req.KeyID, Curve: req.Share.Curve, PublicKey: req.PublicKey, Owner: req.Owner, Account: req.Account, Participants: req.Participants, AuditSequence: 1, CeremonyID: req.CeremonyID, ImportSessionID: req.SessionID, ImportDigest: hex.EncodeToString(digest[:])}, Imports: 1}
			p.states[i][body.KeyID] = held
		}
	} else if held == nil {
		m160APIError(w, 404, "key_not_found")
		return
	}
	d := &held.Description
	response := func() attestor.KeyResponse {
		return attestor.KeyResponse{NodeID: id, KeyID: d.KeyID, Curve: d.Curve, PublicKey: d.PublicKey, Epoch: d.Epoch, Participants: d.Participants, Refreshed: d.Epoch > 0, AuditSequence: d.AuditSequence}
	}
	switch r.URL.Path {
	case attestor.PathImport:
		if err := p.checkpoint(i); err != nil {
			p.fail(w, "checkpoint: %v", err)
			return
		}
		m160JSON(w, response())
	case attestor.PathDescribe:
		m160JSON(w, *d)
	case attestor.PathRefresh:
		var req attestor.RefreshRequest
		if strictJSON(raw, &req) != nil || req.ExpectedEpoch == nil || req.CeremonyID == "" || req.ImportSessionID != d.ImportSessionID || req.ExistingKey != (d.ImportSessionID == "") {
			p.fail(w, "refresh identity")
			return
		}
		if d.Refresh != nil && d.RefreshSessionID != req.SessionID && !(d.RefreshState == "aborted" && req.RecoverySessionID == d.RefreshSessionID) {
			p.fail(w, "rotated live refresh session")
			return
		}
		base := *req.ExpectedEpoch
		if d.Epoch != base && d.Epoch != base+1 {
			p.fail(w, "unexpected refresh epoch")
			return
		}
		held.Refreshes = append(held.Refreshes, req.SessionID)
		d.RefreshSessionID = req.SessionID
		d.RefreshState = "complete"
		d.Epoch = base + 1
		d.AuditSequence++
		d.Refresh = &attestor.RefreshDescription{ExistingKey: req.ExistingKey, CeremonyID: req.CeremonyID, ImportSessionID: req.ImportSessionID, SessionID: req.SessionID, KeyID: req.KeyID, BaseEpoch: base, Curve: d.Curve, PublicKey: d.PublicKey, Participants: d.Participants, Decision: "commit", AuditSequence: d.AuditSequence}
		if i == 4 && p.interrupt == req.KeyID {
			d.Epoch = base
			d.RefreshState = "staged"
			d.Refresh.AuditSequence = 0
		}
		if i == 4 && p.incomplete == req.KeyID {
			d.RefreshState = "staged"
			d.Refresh.AuditSequence = 0
		}
		if err := p.checkpoint(i); err != nil {
			p.fail(w, "checkpoint: %v", err)
			return
		}
		if i == 4 && p.interrupt == req.KeyID {
			m160APIError(w, 503, "refresh_interrupted")
			return
		}
		m160JSON(w, response())
	case attestor.PathSign:
		var req attestor.SignRequest
		if strictJSON(raw, &req) != nil || !reflect.DeepEqual(req.Signers, p.ids()[:attestor.SignQuorum]) || i >= attestor.SignQuorum {
			p.fail(w, "sign identity")
			return
		}
		material := p.keys[req.KeyID]
		if material == nil {
			p.fail(w, "missing committed signing fixture")
			return
		}
		out := attestor.SignResponse{NodeID: id, KeyID: req.KeyID, Kind: req.Kind, AuditSequence: d.AuditSequence + 1}
		if req.Kind == attestor.KindVerify {
			if req.ImportSessionID != d.ImportSessionID || req.CeremonyID != d.CeremonyID || req.ExpectedEpoch == nil || *req.ExpectedEpoch != d.Epoch || d.RefreshState != "complete" || req.Message != "" || r.Header.Get("Authorization") != "" {
				p.fail(w, "operator verification authority")
				return
			}
			message := attestor.VerificationMessage(req.KeyID, mustHex(d.PublicKey), req.ImportSessionID)
			secret := material.secp
			if p.corrupt == req.KeyID {
				secret, _ = crypto.HexToECDSA(fmt.Sprintf("%064x", 99))
			}
			signature, e := crypto.Sign(crypto.Keccak256(message), secret)
			if e != nil {
				p.fail(w, "fixture signature: %v", e)
				return
			}
			out.Message = hex.EncodeToString(message)
			out.SignedBytes = hex.EncodeToString(crypto.Keccak256(message))
			out.Signature = hex.EncodeToString(signature)
			recovery := signature[64]
			out.RecoveryID = &recovery
			held.Verifications++
			d.VerificationSessionID = req.SessionID
			d.VerificationState = "complete"
		} else if req.Kind == "lx_bind" {
			if r.Header.Get("Authorization") != "Bearer owner-"+material.UserID || attestor.SPKIHash(r.TLS.PeerCertificates[0]) != p.gateway.OperatorSPKI {
				p.fail(w, "original owner authority")
				return
			}
			message, e := hex.DecodeString(req.Message)
			if e != nil || !bytes.Equal(message, material.original.message(material.Wallet)) {
				p.fail(w, "original binding message")
				return
			}
			out.SignedBytes = req.Message
			out.Signature = hex.EncodeToString(ed25519.Sign(material.identity, message))
			held.OwnerSigns++
		} else {
			p.fail(w, "unexpected signing kind %s", req.Kind)
			return
		}
		d.AuditSequence++
		if err := p.checkpoint(i); err != nil {
			p.fail(w, "checkpoint: %v", err)
			return
		}
		m160JSON(w, out)
	default:
		p.fail(w, "unexpected endpoint %s", r.URL.Path)
	}
}
func (p *m160Peers) start(t *testing.T) {
	t.Helper()
	p.config.Nodes = nil
	for i, id := range p.ids() {
		index := i
		srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { p.serve(index, w, r) }))
		clientCAs := p.tls.Pool()
		gatewayCA, err := os.ReadFile(p.gateway.CAFile)
		if err != nil || !clientCAs.AppendCertsFromPEM(gatewayCA) {
			t.Fatal("gateway CA", err)
		}
		srv.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{p.certificates[i]}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}
		srv.StartTLS()
		p.servers = append(p.servers, srv)
		p.config.Nodes = append(p.config.Nodes, attestor.NodeConfig{ID: id, URL: srv.URL, Pin: attestor.SPKIHash(p.certificates[i].Leaf)})
	}
}
func (p *m160Peers) stop() {
	for _, srv := range p.servers {
		srv.Close()
	}
	p.servers = nil
}
func (p *m160Peers) reopen(t *testing.T) {
	t.Helper()
	p.stop()
	p.states = nil
	for i := range p.ids() {
		raw, err := os.ReadFile(filepath.Join(p.root, fmt.Sprintf("member%d.json", i)))
		if err != nil {
			t.Fatal(err)
		}
		var state map[string]*m160PeerKey
		if err := json.Unmarshal(raw, &state); err != nil {
			t.Fatal(err)
		}
		p.states = append(p.states, state)
	}
	p.interrupt = ""
	p.start(t)
}
func (p *m160Peers) client(t *testing.T) *attestor.Client {
	t.Helper()
	client, err := attestor.New(p.config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}

type m160Fixture struct {
	db      *sql.DB
	data    *m160Database
	peers   *m160Peers
	master  []byte
	options Options
}

func m160New(t *testing.T, kinds ...string) *m160Fixture {
	t.Helper()
	f := &m160Fixture{data: &m160Database{schema: true, locks: map[string]bool{}}, master: bytes.Repeat([]byte{0x46}, 32)}
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	f.options = Options{CeremonyID: "repair-m160", JournalDir: root, JournalKey: bytes.Repeat([]byte{0x5a}, 32), RehearsalReceipt: filepath.Join(root, "receipt.sealed")}
	p := &m160Peers{root: t.TempDir(), tls: testsupport.NewTLSFiles(t), keys: map[string]*m160Wallet{}}
	p.gateway = testsupport.NewTLSFiles(t)
	p.config = attestor.Config{CertFile: p.tls.CertFile, KeyFile: p.tls.KeyFile, CAFile: p.tls.CAFile, GatewayCertFile: p.gateway.CertFile, GatewayKeyFile: p.gateway.KeyFile, OwnerTokensFile: filepath.Join(root, "owners.json")}
	for _, id := range p.ids() {
		cert, _ := p.tls.ServerCertificate(t, id)
		p.certificates = append(p.certificates, cert)
		p.states = append(p.states, map[string]*m160PeerKey{})
	}
	tokens := map[string]string{}
	for i, kind := range kinds {
		secp, err := crypto.HexToECDSA(fmt.Sprintf("%064x", i+1))
		if err != nil {
			t.Fatal(err)
		}
		identity := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(i + 1)}, 32))
		pub := hex.EncodeToString(identity.Public().(ed25519.PublicKey))
		w := &m160Wallet{Wallet: Wallet{ID: fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1), UserID: fmt.Sprintf("10000000-0000-0000-0000-%012d", i+1), Address: crypto.PubkeyToAddress(secp.PublicKey).Hex(), KeyVersion: 1, ChainID: 125, Kind: kind, DID: "did:layerx:" + pub}, secp: secp, identity: identity}
		name := []byte("agent:" + w.DID + ":main")
		hash := sha256.Sum256(append(append([]byte("LX:ACCOUNT:v1"), binary.BigEndian.AppendUint32(nil, uint32(len(name)))...), name...))
		w.MainAccountID = hex.EncodeToString(hash[:])
		w.OriginalIdentityKeyID = "original:" + w.ID
		if kind == "agent" {
			w.OriginalIdentityKeyID = ""
		}
		w.original = originalIdentity{KeyID: w.OriginalIdentityKeyID, PublicKey: pub, Owner: w.UserID, External: kind == "agent"}
		w.original.BindSignature = hex.EncodeToString(ed25519.Sign(identity, w.original.message(w.Wallet)))
		block, err := aes.NewCipher(f.master)
		if err != nil {
			t.Fatal(err)
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			t.Fatal(err)
		}
		nonce := bytes.Repeat([]byte{byte(i + 1)}, gcm.NonceSize())
		sealed := gcm.Seal(nil, nonce, crypto.FromECDSA(secp), nil)
		encoded := append([]byte{envelope.Version1}, nonce...)
		encoded = append(encoded, sealed[len(sealed)-gcm.Overhead():]...)
		encoded = append(encoded, sealed[:len(sealed)-gcm.Overhead()]...)
		w.EncryptedPrivateKey = base64.StdEncoding.EncodeToString(encoded)
		f.data.wallets = append(f.data.wallets, w)
		p.keys[SecpKeyID(w.ID)] = w
		p.keys[w.original.KeyID] = w
		tokens[w.UserID] = "owner-" + w.UserID
		if kind != "agent" {
			for j, id := range p.ids() {
				p.states[j][w.original.KeyID] = &m160PeerKey{Description: attestor.DescribeResponse{NodeID: id, KeyID: w.original.KeyID, Curve: attestor.CurveEd25519, PublicKey: pub, DID: w.DID, Owner: w.UserID, Account: w.Address, Participants: p.ids(), AuditSequence: 1}}
			}
		}
	}
	raw, err := json.Marshal(tokens)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.config.OwnerTokensFile, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for i := range p.ids() {
		if err := p.checkpoint(i); err != nil {
			t.Fatal(err)
		}
	}
	// Both client authorities authenticate to the same loopback peers.
	p.start(t)
	t.Cleanup(p.stop)
	f.peers = p
	f.db = sql.OpenDB(m160Connector{data: f.data})
	t.Cleanup(func() { _ = f.db.Close() })
	f.options.RPCURL = m160RPC(t, f.data.wallets)
	return f
}
func m160RPC(t *testing.T, wallets []*m160Wallet) string {
	t.Helper()
	contract, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"getUnifiedAccount","stateMutability":"view","inputs":[{"name":"evm","type":"address"}],"outputs":[{"name":"evm","type":"address"},{"name":"paxAddr","type":"string"},{"name":"didPublicKey","type":"bytes32"},{"name":"layerxMainAccountId","type":"bytes32"}]},{"type":"function","name":"layerXBindNonce","stateMutability":"view","inputs":[{"name":"evm","type":"address"}],"outputs":[{"name":"nonce","type":"uint64"}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "request", 400)
			return
		}
		var result any
		if req.Method == "eth_chainId" {
			result = "0x7d"
		} else if req.Method == "eth_call" && len(req.Params) == 2 {
			var call struct {
				To   string `json:"to"`
				Data string `json:"data"`
			}
			if json.Unmarshal(req.Params[0], &call) != nil || call.To != "0x0000000000000000000000000000000000001004" {
				http.Error(w, "contract", 400)
				return
			}
			raw := mustHex(call.Data)
			if len(raw) != 36 {
				http.Error(w, "calldata", 400)
				return
			}
			method, e := contract.MethodById(raw[:4])
			if e != nil {
				http.Error(w, "selector", 400)
				return
			}
			address := common.BytesToAddress(raw[16:])
			var encoded []byte
			for _, held := range wallets {
				if address == common.HexToAddress(held.Address) {
					if method.Name == "getUnifiedAccount" {
						var public, main [32]byte
						copy(public[:], mustHex(held.original.PublicKey))
						copy(main[:], mustHex(held.MainAccountID))
						encoded, e = method.Outputs.Pack(address, "pax-test", public, main)
					} else {
						encoded, e = method.Outputs.Pack(held.original.BindNonce)
					}
					break
				}
			}
			if e != nil || len(encoded) == 0 {
				http.Error(w, "account", 400)
				return
			}
			result = "0x" + hex.EncodeToString(encoded)
		} else {
			http.Error(w, "method", 400)
			return
		}
		m160JSON(w, map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}
func (f *m160Fixture) plan(t *testing.T) *Plan {
	t.Helper()
	plan, err := PlanMigration(context.Background(), f.db)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}
func (f *m160Fixture) authorize(t *testing.T, client *attestor.Client, plan *Plan) {
	t.Helper()
	// Rehearse against an independent copy using the production delivery path.
	// The receipt is sealed from the returned counts, never invented by the test.
	data := &m160Database{schema: true, locks: map[string]bool{}}
	for _, wallet := range f.data.wallets {
		copied := *wallet
		data.wallets = append(data.wallets, &copied)
	}
	db := sql.OpenDB(m160Connector{data: data})
	defer db.Close()
	peers := &m160Peers{root: t.TempDir(), tls: f.peers.tls, gateway: f.peers.gateway, certificates: f.peers.certificates, config: f.peers.config, keys: f.peers.keys}
	raw, err := json.Marshal(f.peers.states)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &peers.states); err != nil {
		t.Fatal(err)
	}
	peers.start(t)
	defer peers.stop()
	rehearsalClient, err := attestor.New(peers.config)
	if err != nil {
		t.Fatal(err)
	}
	defer rehearsalClient.Close()
	if !reflect.DeepEqual(client.Membership(), rehearsalClient.Membership()) {
		t.Fatal("rehearsal membership changed")
	}
	options := f.options
	options.Rehearsal = true
	options.JournalDir = t.TempDir()
	if err := os.Chmod(options.JournalDir, 0700); err != nil {
		t.Fatal(err)
	}
	report, err := Deliver(context.Background(), db, rehearsalClient, f.master, plan, options)
	if err != nil {
		t.Fatalf("rehearsal delivery: %v", err)
	}
	if err := WriteRehearsalReceipt(options, plan, rehearsalClient, report); err != nil {
		t.Fatal(err)
	}
	if len(peers.protocolErrors) != 0 {
		t.Fatalf("rehearsal peer contract: %v", peers.protocolErrors)
	}
}
func (f *m160Fixture) journal(t *testing.T, w Wallet) walletJournal {
	t.Helper()
	raw, err := protectedRead(journalPath(f.options, w.ID), 16<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer zero(raw)
	var journal walletJournal
	if err := openValue(f.options.JournalKey, journalDomain(f.options, w.ID), raw, &journal); err != nil {
		t.Fatal(err)
	}
	return journal
}
func (f *m160Fixture) assertProtocol(t *testing.T) {
	t.Helper()
	if len(f.peers.protocolErrors) != 0 {
		t.Fatalf("peer contract: %v", f.peers.protocolErrors)
	}
}

func TestRepairM_160_RefreshResumeOrder(t *testing.T) {
	m160Recovery(t, false)
}
func TestRepairM_160_OriginalIdentityRefreshResumeOrder(t *testing.T) {
	m160Recovery(t, true)
}

func TestRepairM_160_RecoveryPublicKeyEncodings(t *testing.T) {
	for _, encoding := range []string{"uncompressed", "compressed", "mixed"} {
		t.Run(encoding, func(t *testing.T) {
			f := m160New(t, "standard")
			client := f.peers.client(t)
			plan := f.plan(t)
			f.authorize(t, client, plan)
			wallet := plan.Eligible[0]
			keyID := SecpKeyID(wallet.ID)
			f.peers.interrupt = keyID
			if _, err := Deliver(context.Background(), f.db, client, f.master, plan, f.options); err == nil {
				t.Fatal("missing interruption")
			}
			journal := f.journal(t, wallet)
			bundle, err := attestor.DecodeBundle(journal.Secp.Shares[0])
			if err != nil {
				t.Fatal(err)
			}
			uncompressed, err := attestor.PublicKeyHex(bundle.PublicKey)
			if err != nil || uncompressed != journal.Secp.PublicKey || len(mustHex(uncompressed)) != 65 {
				t.Fatalf("production public key: %q %v", uncompressed, err)
			}
			compressed, err := attestor.EncodePoint(bundle.PublicKey)
			if err != nil || len(mustHex(compressed)) != 33 {
				t.Fatalf("production compressed point: %q %v", compressed, err)
			}
			client.Close()
			f.peers.stop()
			for i := range f.peers.ids() {
				d := &f.peers.states[i][keyID].Description
				if encoding == "compressed" || encoding == "mixed" && i%2 == 0 {
					d.PublicKey = compressed
				}
				if encoding == "compressed" || encoding == "mixed" && i%2 != 0 {
					d.Refresh.PublicKey = compressed
				}
				if err := f.peers.checkpoint(i); err != nil {
					t.Fatal(err)
				}
			}
			f.peers.reopen(t)
			client = f.peers.client(t)
			inspect := func(publicKey string) ([]attestor.DescribeResponse, error) {
				return client.InspectRefreshRecovery(context.Background(), f.options.CeremonyID, journal.Secp.ImportSession, journal.Secp.RefreshSession, keyID, publicKey, journal.Secp.Curve, journal.Secp.BaseEpoch)
			}
			for _, expected := range []string{uncompressed, compressed} {
				rows, err := inspect(expected)
				if err != nil || len(rows) != attestor.NodeCount {
					t.Fatalf("inspect %d-byte key: %+v %v", len(mustHex(expected)), rows, err)
				}
				epochs := map[uint64]int{}
				for i, row := range rows {
					if !reflect.DeepEqual(row, f.peers.states[i][keyID].Description) || row.Refresh == nil || row.Refresh.SessionID != journal.Secp.RefreshSession || row.Refresh.Decision != "commit" {
						t.Fatalf("inspection changed or lost the durable description: %+v", row)
					}
					epochs[row.Epoch]++
				}
				if epochs[0] != 1 || epochs[1] != 4 {
					t.Fatalf("missing partial commit: %v", epochs)
				}
			}
			other, err := crypto.HexToECDSA(fmt.Sprintf("%064x", 99))
			if err != nil {
				t.Fatal(err)
			}
			for _, publicKey := range []string{
				"", "not-hex", "05" + uncompressed[2:], uncompressed[:128], "04" + strings.Repeat("00", 64),
				hex.EncodeToString(crypto.FromECDSAPub(&other.PublicKey)), hex.EncodeToString(crypto.CompressPubkey(&other.PublicKey)),
			} {
				if _, err := inspect(publicKey); err == nil {
					t.Fatalf("accepted invalid or unrelated expected public key %q", publicKey)
				}
			}
			for _, wrongKey := range []string{hex.EncodeToString(crypto.FromECDSAPub(&other.PublicKey)), hex.EncodeToString(crypto.CompressPubkey(&other.PublicKey)), "04" + strings.Repeat("00", 64)} {
				d := &f.peers.states[4][keyID].Description
				original := d.PublicKey
				d.PublicKey = wrongKey
				if _, err := inspect(uncompressed); !errors.Is(err, attestor.ErrDisagree) {
					t.Fatalf("accepted unrelated member public key: %v", err)
				}
				d.PublicKey = original
				original = d.Refresh.PublicKey
				d.Refresh.PublicKey = wrongKey
				if _, err := inspect(compressed); !errors.Is(err, attestor.ErrDisagree) {
					t.Fatalf("accepted unrelated refresh public key: %v", err)
				}
				d.Refresh.PublicKey = original
			}
			if _, err := client.DescribeCeremony(context.Background(), keyID); !errors.Is(err, attestor.ErrDisagree) {
				t.Fatalf("strict description accepted partial commit: %v", err)
			}
			if err := client.ReconcileCeremony(context.Background(), f.options.CeremonyID, journal.Secp.ImportSession, journal.Secp.RefreshSession, keyID, uncompressed, journal.Secp.Curve, 1); !errors.Is(err, attestor.ErrDisagree) {
				t.Fatalf("reconciled partial commit: %v", err)
			}
			retained := f.journal(t, wallet)
			if !reflect.DeepEqual(journal, retained) || retained.Matched || retained.Secp.Refreshed || f.data.wallets[0].Migrated {
				t.Fatal("inspection changed recovery state or marked completion")
			}
			f.assertProtocol(t)
		})
	}
}

func m160Recovery(t *testing.T, identity bool) {
	t.Helper()
	f := m160New(t, "standard")
	client := f.peers.client(t)
	plan := f.plan(t)
	f.authorize(t, client, plan)
	wallet := plan.Eligible[0]
	keyID := SecpKeyID(wallet.ID)
	if identity {
		keyID = wallet.OriginalIdentityKeyID
	}
	f.peers.interrupt = keyID
	report, err := Deliver(context.Background(), f.db, client, f.master, plan, f.options)
	if err == nil || report.Matched != 0 || f.data.wallets[0].Migrated {
		t.Fatalf("interrupted delivery: %+v %v", report, err)
	}
	before := f.journal(t, wallet)
	curve := before.Secp
	if identity {
		curve = before.Identity
	}
	if !before.Secp.Imported || curve.Refreshed || len(before.Secp.Shares) != 5 {
		t.Fatalf("lost recovery material: %+v", curve)
	}
	if _, err := client.DescribeCeremony(context.Background(), keyID); !errors.Is(err, attestor.ErrDisagree) {
		t.Fatalf("fixture lacks mixed epochs: %v", err)
	}
	for i := range f.peers.ids() {
		want := uint64(1)
		if i == 4 {
			want = 0
		}
		if f.peers.states[i][keyID].Description.Epoch != want {
			t.Fatalf("member %d epoch not %d", i, want)
		}
	}
	client.Close()
	f.peers.reopen(t)
	client = f.peers.client(t)
	// Nothing from the old journal struct or client is passed to the resumed run.
	report, err = Deliver(context.Background(), f.db, client, f.master, f.plan(t), f.options)
	if err != nil || report != (Report{Read: 1, Verified: 1, Imported: 1, Refreshed: 1, TestSigned: 1, HeldIdentity: 1, Matched: 1}) {
		t.Fatalf("reopened delivery: %+v %v", report, err)
	}
	after := f.journal(t, wallet)
	resumed := after.Secp
	if identity {
		resumed = after.Identity
	}
	if resumed.ImportSession != curve.ImportSession || resumed.RefreshSession != curve.RefreshSession || resumed.VerifySession != curve.VerifySession || resumed.RecoverySession != curve.RecoverySession || resumed.RefreshRecoverySession != curve.RefreshRecoverySession {
		t.Fatal("recovery rotated the recorded sessions")
	}
	if !after.Matched || !after.Secp.Refreshed || !after.Identity.Refreshed || !f.data.wallets[0].Migrated || after.Original != before.Original || after.Identity.KeyID != wallet.OriginalIdentityKeyID {
		t.Fatal("completion changed or lost original identity")
	}
	for i := range f.peers.ids() {
		state := f.peers.states[i][keyID]
		if state.Description.Epoch != curve.BaseEpoch+1 || state.Description.RefreshState != "complete" || len(state.Refreshes) != 2 {
			t.Fatalf("member %d did not replay and reconcile: %+v", i, state)
		}
		for _, session := range state.Refreshes {
			if session != curve.RefreshSession {
				t.Fatal("refresh replay used replacement session")
			}
		}
		if f.peers.states[i][SecpKeyID(wallet.ID)].Imports != 1 {
			t.Fatal("recovery re-imported original shares")
		}
		if f.peers.states[i][IdentityKeyID(wallet.ID)] != nil {
			t.Fatal("created a replacement identity")
		}
	}
	f.assertProtocol(t)
}

func TestRepairM_160_RequireEveryMemberComplete(t *testing.T) {
	for _, identity := range []bool{false, true} {
		t.Run(fmt.Sprint(identity), func(t *testing.T) {
			f := m160New(t, "standard")
			client := f.peers.client(t)
			plan := f.plan(t)
			f.authorize(t, client, plan)
			keyID := SecpKeyID(plan.Eligible[0].ID)
			if identity {
				keyID = plan.Eligible[0].OriginalIdentityKeyID
			}
			f.peers.incomplete = keyID
			report, err := Deliver(context.Background(), f.db, client, f.master, plan, f.options)
			if !errors.Is(err, attestor.ErrDisagree) || report.Matched != 0 || f.data.wallets[0].Migrated {
				t.Fatalf("accepted staged fifth member: %+v %v", report, err)
			}
			journal := f.journal(t, plan.Eligible[0])
			curve := journal.Secp
			if identity {
				curve = journal.Identity
			}
			if curve.Refreshed || journal.Matched || len(journal.Secp.Shares) != 5 {
				t.Fatal("recorded completion or discarded shares before reconciliation")
			}
			client.Close()
			f.peers.reopen(t)
			f.peers.incomplete = ""
			client = f.peers.client(t)
			report, err = Deliver(context.Background(), f.db, client, f.master, f.plan(t), f.options)
			if err != nil || report.Matched != 1 {
				t.Fatalf("reconcile recovery: %+v %v", report, err)
			}
			f.assertProtocol(t)
		})
	}
}

func TestRepairM_160_RejectUnrelatedRecovery(t *testing.T) {
	for _, identity := range []bool{false, true} {
		for _, mutation := range []string{"session", "ceremony", "import", "epoch", "public", "decision", "members"} {
			t.Run(fmt.Sprintf("%t/%s", identity, mutation), func(t *testing.T) {
				f := m160New(t, "standard")
				client := f.peers.client(t)
				plan := f.plan(t)
				f.authorize(t, client, plan)
				wallet := plan.Eligible[0]
				keyID := SecpKeyID(wallet.ID)
				if identity {
					keyID = wallet.OriginalIdentityKeyID
				}
				f.peers.interrupt = keyID
				if _, err := Deliver(context.Background(), f.db, client, f.master, plan, f.options); err == nil {
					t.Fatal("missing interruption")
				}
				saved, err := os.ReadFile(journalPath(f.options, wallet.ID))
				if err != nil {
					t.Fatal(err)
				}
				client.Close()
				f.peers.reopen(t)
				d := &f.peers.states[4][keyID].Description
				switch mutation {
				case "session":
					d.Refresh.SessionID = "another-session"
				case "ceremony":
					d.Refresh.CeremonyID = "another-ceremony"
				case "import":
					d.Refresh.ImportSessionID = "another-import"
				case "epoch":
					d.Epoch = 2
				case "public":
					d.Refresh.PublicKey = "00"
				case "decision":
					d.Refresh.Decision = "abort"
				case "members":
					d.Refresh.Participants = []string{"n1"}
				}
				if err := f.peers.checkpoint(4); err != nil {
					t.Fatal(err)
				}
				client = f.peers.client(t)
				report, err := Deliver(context.Background(), f.db, client, f.master, f.plan(t), f.options)
				if !errors.Is(err, attestor.ErrDisagree) || report.Matched != 0 || f.data.wallets[0].Migrated {
					t.Fatalf("accepted unrelated recovery: %+v %v", report, err)
				}
				retained, err := os.ReadFile(journalPath(f.options, wallet.ID))
				if err != nil || !bytes.Equal(saved, retained) {
					t.Fatal("refusal changed the sealed recovery journal")
				}
				for i := range f.peers.ids() {
					if len(f.peers.states[i][keyID].Refreshes) != 1 {
						t.Fatal("replayed an invalid transition")
					}
				}
				f.assertProtocol(t)
			})
		}
	}
}

func TestRepairM_160_MigrationStaleTests(t *testing.T) {
	t.Run("funded-and-migrated", m160TestPlan)
	t.Run("schema", m160TestSchema)
	t.Run("completion-and-original-identity", m160TestCompleteDelivery)
	t.Run("signature-mismatch-stops-run", m160TestSignatureMismatch)
	t.Run("envelope-mismatch", m160TestEnvelopeMismatch)
	t.Run("external-identity-refused", m160TestExternalIdentity)
	t.Run("mandatory-rehearsal-and-journal", m160TestRehearsal)
}

func m160TestPlan(t *testing.T) {
	f := m160New(t, "standard", "agent", "standard", "funded", "standard")
	f.data.wallets[2].fundedAccount = true
	f.data.wallets[4].Migrated = true
	f.data.wallets[4].attestorID = SecpKeyID(f.data.wallets[4].ID)
	wallets, err := ReadWallets(context.Background(), f.db)
	if err != nil || len(wallets) != 5 {
		t.Fatalf("read wallets: %d %v", len(wallets), err)
	}
	for _, w := range wallets {
		if w.EncryptedPrivateKey == "" || !common.IsHexAddress(w.Address) || w.ChainID != 125 || w.KeyVersion != 1 || w.DID == "" {
			t.Fatalf("incomplete wallet %s", w.ID)
		}
	}
	plan := f.plan(t)
	if plan.Total != 5 || plan.AlreadyMigrated != 1 || len(plan.Funded) != 2 || len(plan.Eligible) != 2 {
		t.Fatalf("plan %+v", plan)
	}
	eligible := map[string]bool{}
	for _, w := range plan.Eligible {
		eligible[w.ID] = true
	}
	if !eligible[f.data.wallets[0].ID] || !eligible[f.data.wallets[1].ID] {
		t.Fatal("standard and agent wallets are not the eligible set")
	}
	funded := map[string]bool{}
	for _, w := range plan.Funded {
		funded[w.ID] = true
	}
	if !funded[f.data.wallets[2].ID] || !funded[f.data.wallets[3].ID] {
		t.Fatal("funded account and funded kind are not both excluded")
	}
}
func m160TestSchema(t *testing.T) {
	f := m160New(t, "standard")
	if _, err := f.db.Exec(`alter table wallets drop column migrated_at`); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanMigration(context.Background(), f.db); !errors.Is(err, ErrSchema) {
		t.Fatalf("missing column: %v", err)
	}
}
func m160TestCompleteDelivery(t *testing.T) {
	f := m160New(t, "standard", "standard", "standard", "funded")
	client := f.peers.client(t)
	plan := f.plan(t)
	f.authorize(t, client, plan)
	original := make([]Wallet, len(f.data.wallets))
	for i, w := range f.data.wallets {
		original[i] = w.Wallet
	}
	report, err := Deliver(context.Background(), f.db, client, f.master, plan, f.options)
	want := Report{Read: 3, Verified: 3, Imported: 3, Refreshed: 3, TestSigned: 3, HeldIdentity: 3, Matched: 3}
	if err != nil || report != want {
		t.Fatalf("report %+v want %+v: %v", report, want, err)
	}
	for j, w := range f.data.wallets[:3] {
		if !w.Migrated || w.attestorID != SecpKeyID(w.ID) || w.DID != original[j].DID || w.MainAccountID != original[j].MainAccountID || w.OriginalIdentityKeyID != original[j].OriginalIdentityKeyID {
			t.Fatal("completion replaced the original identity or failed to mark the wallet")
		}
		journal := f.journal(t, original[j])
		if !journal.Matched || journal.Identity.PublicKey != w.original.PublicKey || journal.Identity.KeyID != w.original.KeyID || journal.Identity.Verification == nil || !ed25519.Verify(w.identity.Public().(ed25519.PublicKey), journal.Identity.Verification.Message, journal.Identity.Verification.Signature) {
			t.Fatal("missing original identity evidence")
		}
		for i, id := range f.peers.ids() {
			secp, identity := f.peers.states[i][SecpKeyID(w.ID)], f.peers.states[i][w.OriginalIdentityKeyID]
			if secp == nil || identity == nil || secp.Description.Epoch != 1 || identity.Description.Epoch != 1 || secp.Description.RefreshState != "complete" || identity.Description.RefreshState != "complete" {
				t.Fatalf("node %s lacks both refreshed shares of %s", id, w.ID)
			}
			if f.peers.states[i][IdentityKeyID(w.ID)] != nil {
				t.Fatal("replacement identity generated")
			}
			if i < attestor.SignQuorum && (secp.Verifications != 1 || secp.OwnerSigns != 0 || identity.Verifications != 0 || identity.OwnerSigns != 2) {
				t.Fatalf("node %s did not use operator verification and both original-owner signatures", id)
			}
		}
	}
	funded := f.data.wallets[3]
	if funded.Migrated {
		t.Fatal("funded wallet marked migrated")
	}
	for i := range f.peers.ids() {
		if f.peers.states[i][SecpKeyID(funded.ID)] != nil || f.peers.states[i][funded.OriginalIdentityKeyID].Description.Epoch != 0 {
			t.Fatal("funded wallet was delivered")
		}
	}
	again := f.plan(t)
	if len(again.Eligible) != 0 || again.AlreadyMigrated != 3 || len(again.Funded) != 1 {
		t.Fatalf("second plan %+v", again)
	}
	f.assertProtocol(t)
}
func m160TestSignatureMismatch(t *testing.T) {
	f := m160New(t, "standard", "standard", "standard")
	client := f.peers.client(t)
	plan := f.plan(t)
	f.authorize(t, client, plan)
	first, second, third := plan.Eligible[0], plan.Eligible[1], plan.Eligible[2]
	f.peers.corrupt = SecpKeyID(second.ID)
	report, err := Deliver(context.Background(), f.db, client, f.master, plan, f.options)
	var mismatch *MismatchError
	if !errors.As(err, &mismatch) || !errors.Is(err, ErrAddressMismatch) {
		t.Fatalf("signature mismatch: %v", err)
	}
	if mismatch.WalletID != second.ID || mismatch.Stored != common.HexToAddress(second.Address) || mismatch.Recovered == mismatch.Stored {
		t.Fatalf("mismatch %+v", mismatch)
	}
	if report.Read != 2 || report.TestSigned != 2 || report.Matched != 1 {
		t.Fatalf("report %+v", report)
	}
	if !f.data.wallets[0].Migrated || f.data.wallets[1].Migrated || f.data.wallets[2].Migrated || f.data.wallets[0].ID != first.ID {
		t.Fatal("wrong migrated set")
	}
	for i := range f.peers.ids() {
		if f.peers.states[i][SecpKeyID(third.ID)] != nil {
			t.Fatal("run continued past mismatch")
		}
	}
	if f.journal(t, second).Matched {
		t.Fatal("mismatch recorded as matched")
	}
	f.assertProtocol(t)
}
func m160TestEnvelopeMismatch(t *testing.T) {
	f := m160New(t, "standard", "standard")
	// The original identity producer and RPC bind this wallet to its stored
	// address; only the encrypted EVM envelope belongs to another wallet.
	f.data.wallets[0].EncryptedPrivateKey = f.data.wallets[1].EncryptedPrivateKey
	client := f.peers.client(t)
	plan := f.plan(t)
	options := f.options
	options.Rehearsal = true
	report, err := Deliver(context.Background(), f.db, client, f.master, plan, options)
	var walletError *WalletError
	if !errors.As(err, &walletError) || walletError.WalletID != plan.Eligible[0].ID || walletError.Stage != "open" || !errors.Is(err, envelope.ErrAddressMismatch) {
		t.Fatalf("envelope mismatch: %v", err)
	}
	if report.Verified != 0 || report.Imported != 0 {
		t.Fatalf("report %+v", report)
	}
	for _, w := range f.data.wallets {
		if w.Migrated {
			t.Fatal("wallet marked migrated")
		}
		for i := range f.peers.ids() {
			if f.peers.states[i][SecpKeyID(w.ID)] != nil {
				t.Fatal("key delivered after envelope mismatch")
			}
		}
	}
	f.assertProtocol(t)
}
func m160TestExternalIdentity(t *testing.T) {
	f := m160New(t, "agent")
	client := f.peers.client(t)
	plan := f.plan(t)
	options := f.options
	options.Rehearsal = true
	report, err := Deliver(context.Background(), f.db, client, f.master, plan, options)
	if !errors.Is(err, ErrExternalIdentity) || report.Imported != 0 || report.Matched != 0 || f.data.wallets[0].Migrated {
		t.Fatalf("external custody accepted: %+v %v", report, err)
	}
	for i := range f.peers.ids() {
		if len(f.peers.states[i]) != 0 {
			t.Fatal("created replacement custody for external identity")
		}
	}
	f.assertProtocol(t)
}
func m160TestRehearsal(t *testing.T) {
	f := m160New(t, "standard")
	client := f.peers.client(t)
	plan := f.plan(t)
	bad := f.options
	bad.JournalKey = nil
	if _, err := Deliver(context.Background(), f.db, client, f.master, plan, bad); !errors.Is(err, ErrJournal) {
		t.Fatalf("unprotected journal: %v", err)
	}
	if _, err := Deliver(context.Background(), f.db, client, f.master, plan, f.options); !errors.Is(err, ErrRehearsal) {
		t.Fatalf("missing rehearsal: %v", err)
	}
	incomplete := f.options
	incomplete.Rehearsal = true
	if err := WriteRehearsalReceipt(incomplete, plan, client, Report{Read: 1}); !errors.Is(err, ErrRehearsal) {
		t.Fatalf("incomplete rehearsal accepted: %v", err)
	}
	f.authorize(t, client, plan)
	changed := *plan
	changed.Eligible = append([]Wallet(nil), plan.Eligible...)
	changed.Eligible[0].DID = "did:layerx:" + strings.Repeat("00", 32)
	if _, err := Deliver(context.Background(), f.db, client, f.master, &changed, f.options); !errors.Is(err, ErrRehearsal) {
		t.Fatalf("changed identity rehearsal accepted: %v", err)
	}
	if err := os.Chmod(f.options.RehearsalReceipt, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Deliver(context.Background(), f.db, client, f.master, plan, f.options); !errors.Is(err, ErrRehearsal) {
		t.Fatalf("readable receipt accepted: %v", err)
	}
	for i := range f.peers.ids() {
		if f.peers.states[i][SecpKeyID(plan.Eligible[0].ID)] != nil {
			t.Fatal("import before rehearsal validation")
		}
	}
	f.assertProtocol(t)
}
