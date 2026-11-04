package bringup

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type RolloutService struct {
	Target  string `toml:"target"`
	Service string `toml:"railway_service"`
	BoxRole string `toml:"box_role"`
	Unit    string `toml:"box_unit"`
	Health  string `toml:"health"`
}

func ReadRolloutService(input io.Reader, name string) (RolloutService, error) {
	var d struct {
		Service map[string]RolloutService `toml:"service"`
	}
	if _, e := toml.NewDecoder(input).Decode(&d); e != nil {
		return RolloutService{}, errors.New("rollout unreadable")
	}
	r, ok := d.Service[name]
	if !ok || ((r.Target != "box" || !safeIdentity(r.BoxRole) || r.Unit == "") && (r.Target != "railway" || !safeIdentity(r.Service))) {
		return r, errors.New("rollout service absent or invalid")
	}
	return r, nil
}
func (s RolloutService) HostKey() string {
	return strings.ToUpper(strings.ReplaceAll(s.BoxRole, "-", "_")) + "_HOST"
}
func ReadRoleInstances(input io.Reader) ([]string, error) {
	var doc struct {
		LatestDeployment struct {
			ID, Status string
			Instances  []struct{ ID, Status string }
			Meta       struct {
				ServiceManifest struct {
					Deploy struct {
						NumReplicas       int `json:"numReplicas"`
						MultiRegionConfig map[string]struct {
							NumReplicas int `json:"numReplicas"`
						} `json:"multiRegionConfig"`
					} `json:"deploy"`
				} `json:"serviceManifest"`
			} `json:"meta"`
		} `json:"latestDeployment"`
	}
	if json.NewDecoder(input).Decode(&doc) != nil {
		return nil, errors.New("role deployment unreadable")
	}
	d := doc.LatestDeployment
	if !safeIdentity(d.ID) || d.Status != "SUCCESS" {
		return nil, errors.New("role deployment not successful")
	}
	want := d.Meta.ServiceManifest.Deploy.NumReplicas
	if len(d.Meta.ServiceManifest.Deploy.MultiRegionConfig) > 0 {
		want = 0
		for _, r := range d.Meta.ServiceManifest.Deploy.MultiRegionConfig {
			if r.NumReplicas < 1 {
				return nil, errors.New("invalid role replica count")
			}
			want += r.NumReplicas
		}
	}
	if want < 1 || len(d.Instances) != want {
		return nil, errors.New("role replica coverage missing")
	}
	seen := map[string]bool{}
	var ids []string
	for _, i := range d.Instances {
		if !safeIdentity(i.ID) || seen[i.ID] || (i.Status != "RUNNING" && i.Status != "SUCCESS") {
			return nil, errors.New("role instance not running or duplicate")
		}
		seen[i.ID] = true
		ids = append(ids, i.ID)
	}
	return ids, nil
}
func ReadProcessEnvironment(procRoot, executable string) (map[string]string, error) {
	return readProcessEnvironment(procRoot, executable, "")
}
func readProcessEnvironment(procRoot, executable, kind string) (map[string]string, error) {
	entries, e := os.ReadDir(procRoot)
	if e != nil {
		return nil, e
	}
	var found []map[string]string
	for _, entry := range entries {
		if _, e = strconv.ParseUint(entry.Name(), 10, 32); e != nil {
			continue
		}
		path := filepath.Join(procRoot, entry.Name())
		exe, e := os.Readlink(filepath.Join(path, "exe"))
		if e != nil || filepath.Base(exe) != executable {
			continue
		}
		b, e := os.ReadFile(filepath.Join(path, "environ"))
		if e != nil {
			return nil, errors.New("producer environment unreadable")
		}
		env := map[string]string{}
		for _, line := range bytes.Split(b, []byte{0}) {
			key, value, ok := strings.Cut(string(line), "=")
			if ok {
				if _, exists := env[key]; exists {
					return nil, errors.New("duplicate producer environment")
				}
				env[key] = value
			}
		}
		if kind != "" && env["LAYERX_EVENTS_KIND"] != kind {
			continue
		}
		found = append(found, env)
	}
	if len(found) != 1 {
		return nil, errors.New("exactly one running producer is required")
	}
	return found[0], nil
}
func BundleTLSClient(bundle IdentityBundle) (*http.Client, error) {
	if e := BundleConsistent(bundle); e != nil {
		return nil, errors.New("client custody inconsistent")
	}
	cert, e := tls.X509KeyPair(bundle["cert.pem"], bundle["key.pem"])
	if e != nil {
		return nil, errors.New("client identity invalid")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(bundle["ca.pem"]) {
		return nil, errors.New("client trust invalid")
	}
	return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{cert}}}, Timeout: 10 * time.Second}, nil
}
func producerHTTPClient(env map[string]string, prefix string) (*http.Client, string, string, error) {
	endpoint := env[prefix+"_UPSTREAM_URL"]
	if e := protectedURL(endpoint); e != nil {
		return nil, "", "", e
	}
	read := func(suffix string) ([]byte, error) {
		path := env[prefix+suffix]
		if path == "" {
			return nil, errors.New("producer credential path missing")
		}
		return os.ReadFile(path)
	}
	ca, e := read("_UPSTREAM_CA_DER")
	if e != nil {
		return nil, "", "", errors.New("producer trust missing")
	}
	issuer, e := x509.ParseCertificate(ca)
	if e != nil {
		return nil, "", "", errors.New("producer trust malformed")
	}
	pool := x509.NewCertPool()
	pool.AddCert(issuer)
	p12, e := read("_UPSTREAM_CLIENT_IDENTITY_PKCS12")
	if e != nil {
		return nil, "", "", errors.New("producer identity missing")
	}
	password, e := read("_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE")
	if e != nil {
		return nil, "", "", errors.New("producer password missing")
	}
	key, certs, e := decodeIdentityP12(p12, strings.TrimSpace(string(password)))
	if e != nil || len(certs) == 0 {
		return nil, "", "", errors.New("producer identity malformed")
	}
	identity := tls.Certificate{PrivateKey: key, Leaf: certs[0]}
	for _, c := range certs {
		identity.Certificate = append(identity.Certificate, c.Raw)
	}
	token, e := read("_UPSTREAM_TOKEN_FILE")
	if e != nil || len(bytes.TrimSpace(token)) == 0 {
		return nil, "", "", errors.New("producer bearer missing")
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool, Certificates: []tls.Certificate{identity}}}, Timeout: 10 * time.Second}
	return client, endpoint, "Bearer " + strings.TrimSpace(string(token)), nil
}
func ValidateEventAdmission(input []byte, kind, role string) error {
	var a struct {
		Schema, Role, Kind     string
		Ready                  bool
		Generation, Principals uint64
		PrincipalDigest        bool   `json:"principal_digest"`
		Fresh                  uint64 `json:"fresh_for_ms"`
	}
	if json.Unmarshal(input, &a) != nil || a.Schema != "layerx.event-admission.v1" || a.Role != role || a.Kind != kind || !a.Ready || a.Generation == 0 || a.Principals == 0 || a.Fresh != 10000 || (role == "source-producer" && (kind == "payment" || kind == "program") && !a.PrincipalDigest) {
		return errors.New("event handoff admission refused")
	}
	return nil
}

// ProbeEventHandoff reads both authenticated endpoints using the running
// producer's exact PKCS12, root and bearer files; no secret leaves the origin.
func ProbeEventHandoff(ctx context.Context, env map[string]string, kind string, output io.Writer) error {
	if kind != "journey" && kind != "approval" && kind != "payment" && kind != "program" {
		return errors.New("unknown event kind")
	}
	for _, leg := range []struct{ prefix, path, role string }{{"LAYERX_EVENTS_" + strings.ToUpper(kind), "/internal/v1/producer-readiness", "source-producer"}, {"LAYERX_EVENTS_WEBHOOKS", "/internal/v1/readiness/" + kind, "webhook-trigger"}} {
		client, endpoint, token, e := producerHTTPClient(env, leg.prefix)
		if e != nil {
			return e
		}
		status, b, _, e := jsonRequest(ctx, client, "GET", strings.TrimRight(endpoint, "/")+leg.path, token, "", nil)
		client.CloseIdleConnections()
		if e != nil || status != 200 {
			return errors.New("authenticated event handoff unavailable")
		}
		if e = ValidateEventAdmission(b, kind, leg.role); e != nil {
			return e
		}
	}
	_, e := fmt.Fprintf(output, "pass producer-handoff kind=%s source=admitted trigger=admitted\n", kind)
	return e
}
func ProbeInternalReady(ctx context.Context, client *http.Client, endpoint string) error {
	status, b, _, e := jsonRequest(ctx, client, "GET", endpoint, "", "", nil)
	if e != nil || status != 200 {
		return errors.New("authorized internal readiness refused")
	}
	var d struct{ Ready bool }
	if json.Unmarshal(b, &d) != nil || !d.Ready {
		return errors.New("internal readiness not ready")
	}
	return nil
}

type RegistryCursor struct {
	Endpoint, Program string
	Next              uint64
	Match             *types.Log
}

const ProgramsAddress = "0x000000000000000000000000000000000000101d"

var ProgramDeployedTopic = crypto.Keccak256Hash([]byte("ProgramDeployed(bytes32,bytes32)"))

func registryLog(ctx context.Context, client *http.Client, endpoint, program string, log *types.Log) error {
	if log == nil || log.Removed || log.Address != common.HexToAddress(ProgramsAddress) || len(log.Topics) != 2 || log.Topics[0] != ProgramDeployedTopic || log.Topics[1] != common.HexToHash(program) || len(log.Data) != 32 || bytes.Equal(log.Data, make([]byte, 32)) || log.BlockNumber == 0 || log.BlockHash == (common.Hash{}) || log.TxHash == (common.Hash{}) {
		return errors.New("invalid committed Programs event")
	}
	b, e := callRPC(ctx, client, endpoint, "eth_getBlockByNumber", []any{fmt.Sprintf("0x%x", log.BlockNumber), false}, "", false)
	if e != nil {
		return e
	}
	var block struct {
		Hash         common.Hash
		Number       string
		Transactions []common.Hash
	}
	if json.Unmarshal(b, &block) != nil || block.Hash != log.BlockHash || block.Number != fmt.Sprintf("0x%x", log.BlockNumber) {
		return errors.New("Programs event not canonical")
	}
	included := false
	for _, tx := range block.Transactions {
		included = included || tx == log.TxHash
	}
	if !included {
		return errors.New("Programs transaction absent from canonical block")
	}
	b, e = callRPC(ctx, client, endpoint, "eth_getTransactionReceipt", []string{log.TxHash.Hex()}, "", false)
	if e != nil {
		return e
	}
	var receipt struct {
		Status                     string
		TransactionHash, BlockHash common.Hash
		BlockNumber                string
		Logs                       []types.Log
	}
	if json.Unmarshal(b, &receipt) != nil || receipt.Status != "0x1" || receipt.TransactionHash != log.TxHash || receipt.BlockHash != log.BlockHash || receipt.BlockNumber != fmt.Sprintf("0x%x", log.BlockNumber) {
		return errors.New("Programs receipt not committed")
	}
	for _, candidate := range receipt.Logs {
		a, _ := json.Marshal(log)
		b, _ := json.Marshal(candidate)
		if bytes.Equal(a, b) {
			return nil
		}
	}
	return errors.New("Programs log absent from receipt")
}
func ObserveRegistryEvents(ctx context.Context, client *http.Client, endpoint, program, cursorPath string) (*types.Log, error) {
	if e := protectedURL(endpoint); e != nil {
		return nil, e
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(program) {
		return nil, errors.New("registry program identity missing")
	}
	cursor := RegistryCursor{Endpoint: endpoint, Program: program}
	if b, e := os.ReadFile(cursorPath); e == nil {
		if json.Unmarshal(b, &cursor) != nil || cursor.Endpoint != endpoint || cursor.Program != program {
			return nil, errors.New("registry cursor binding invalid")
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if cursor.Match != nil {
		if e := registryLog(ctx, client, endpoint, program, cursor.Match); e != nil {
			return nil, e
		}
	}
	b, e := callRPC(ctx, client, endpoint, "eth_chainId", []string{}, "", false)
	if e != nil || string(b) != `"0x7d"` {
		return nil, errors.New("registry chain is not 125")
	}
	b, e = callRPC(ctx, client, endpoint, "eth_blockNumber", []string{}, "", false)
	if e != nil {
		return nil, e
	}
	var q string
	if json.Unmarshal(b, &q) != nil {
		return nil, errors.New("registry head malformed")
	}
	head, e := quantity(q)
	if e != nil || !head.IsUint64() || head.Sign() == 0 {
		return nil, errors.New("registry head invalid")
	}
	height := head.Uint64()
	if cursor.Next > height+1 {
		return nil, errors.New("registry cursor ahead of chain")
	}
	for page := 0; page < 16 && cursor.Next <= height; page++ {
		end := cursor.Next + 255
		if end < cursor.Next || end > height {
			end = height
		}
		filter := map[string]any{"address": ProgramsAddress, "topics": []string{ProgramDeployedTopic.Hex(), "0x" + program}, "fromBlock": fmt.Sprintf("0x%x", cursor.Next), "toBlock": fmt.Sprintf("0x%x", end)}
		b, e = callRPC(ctx, client, endpoint, "eth_getLogs", []any{filter}, "", false)
		if e != nil {
			return nil, e
		}
		var logs []types.Log
		if json.Unmarshal(b, &logs) != nil || string(b) == "null" {
			return nil, errors.New("Programs event page malformed")
		}
		for i := range logs {
			log := logs[i]
			if log.BlockNumber < cursor.Next || log.BlockNumber > end {
				return nil, errors.New("Programs log outside requested page")
			}
			if e = registryLog(ctx, client, endpoint, program, &log); e != nil {
				return nil, e
			}
			if cursor.Match == nil || cursor.Match.BlockNumber < log.BlockNumber || cursor.Match.BlockNumber == log.BlockNumber && cursor.Match.Index < log.Index {
				cursor.Match = &log
			}
		}
		cursor.Next = end + 1
		b, e = json.Marshal(cursor)
		if e != nil {
			return nil, e
		}
		if e = edgeAtomic(cursorPath, b, 0600); e != nil {
			return nil, e
		}
	}
	if cursor.Next <= height {
		return nil, errors.New("Programs cursor catchup incomplete; durable progress retained")
	}
	if cursor.Match == nil {
		return nil, errors.New("registry program has no committed deployment event")
	}
	return cursor.Match, nil
}
func ProbeRegistryPublication(ctx context.Context, client *http.Client, endpoint, token, program string, log *types.Log, now time.Time) error {
	status, b, _, e := jsonRequest(ctx, client, "GET", strings.TrimRight(endpoint, "/")+"/v1/programs/registry/"+program, token, "", nil)
	if e != nil || status != 200 {
		return errors.New("registry publication unavailable")
	}
	var p struct {
		Program             string `json:"program_id"`
		Lifecycle           string
		Observed, At, Until uint64
		Versions            []struct {
			CodeHash string `json:"code_hash"`
			Source   struct{ Status string }
		}
		Receipt struct {
			Verification string
			Digest       string `json:"deployment_receipt_digest"`
		}
	}
	// Explicit field names are part of the published read contract.
	var raw struct {
		Program   string          `json:"program_id"`
		Lifecycle string          `json:"lifecycle"`
		Observed  uint64          `json:"observed_sequence"`
		At        uint64          `json:"observed_at"`
		Until     uint64          `json:"valid_through"`
		Versions  json.RawMessage `json:"versions"`
		Receipt   json.RawMessage `json:"receipt"`
	}
	if json.Unmarshal(b, &raw) != nil {
		return errors.New("registry publication malformed")
	}
	p.Program, p.Lifecycle, p.Observed, p.At, p.Until = raw.Program, raw.Lifecycle, raw.Observed, raw.At, raw.Until
	if json.Unmarshal(raw.Versions, &p.Versions) != nil || json.Unmarshal(raw.Receipt, &p.Receipt) != nil || p.Program != program || p.Lifecycle != "active" || log == nil || p.Observed < log.BlockNumber || p.At == 0 || p.At > uint64(now.UnixMilli()) || p.Until < uint64(now.UnixMilli()) || p.Receipt.Verification != "receipt-verified" || len(p.Receipt.Digest) != 64 {
		return errors.New("registry publication lacks current verified observation")
	}
	for _, v := range p.Versions {
		if v.CodeHash == hex.EncodeToString(log.Data) && v.Source.Status == "verified" {
			return nil
		}
	}
	return errors.New("committed deployment has no verified publication")
}
func DocumentationNames(inventory PublicInventory, spec io.Reader) (map[string]bool, error) {
	allowed := map[string]bool{}
	for _, endpoint := range inventory.Endpoints() {
		u, e := url.Parse(endpoint)
		if e != nil {
			return nil, e
		}
		allowed[u.Hostname()] = true
	}
	section := ""
	s := bufio.NewScanner(spec)
	host := regexp.MustCompile(`(?:[a-z0-9-]+\.)+paxeer\.network`)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "[") {
			section = strings.Trim(line, "[]")
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if ok && ((section == PublicNamesSection && key != "rpc" && key != "router" && key != "title") || (section == "contract_context.108.decision" && key == "derivation_message")) {
			for _, name := range host.FindAllString(line, -1) {
				allowed[name] = true
			}
		}
	}
	return allowed, s.Err()
}
func ValidateDocumentNames(inventory PublicInventory, allowed map[string]bool, text string) error {
	if strings.Contains(text, "FAUCET_URL") {
		return errors.New("unpublished faucet configuration")
	}
	host := regexp.MustCompile(`(?:[a-z0-9-]+\.)+paxeer\.network`)
	for _, name := range host.FindAllString(strings.ToLower(text), -1) {
		if !allowed[name] {
			return errors.New("unpublished public name")
		}
	}
	nodes := map[string]bool{}
	for _, endpoint := range inventory.Endpoints() {
		u, e := url.Parse(endpoint)
		if e != nil {
			return e
		}
		nodes[strings.ToLower(u.Hostname())] = true
	}
	for _, match := range regexp.MustCompile(`(?i)https?://[^\s`+"`"+`<>"')\],;]+`).FindAllString(text, -1) {
		u, e := url.Parse(match)
		if e != nil {
			return errors.New("malformed documentation URL")
		}
		if nodes[strings.ToLower(u.Hostname())] && (u.Scheme != "https" || u.Port() != "" || (u.EscapedPath() != "" && u.EscapedPath() != "/") || u.RawQuery != "" || u.Fragment != "") {
			return errors.New("node RPC must use its declared HTTPS root")
		}
	}
	return nil
}
func registryLoopbackClient(bundle IdentityBundle, port string) (*http.Client, error) {
	client, e := BundleTLSClient(bundle)
	if e != nil {
		return nil, e
	}
	transport := client.Transport.(*http.Transport)
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "layerx-program-registry:"+port {
			address = "127.0.0.1:" + port
		}
		return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, address)
	}
	return client, nil
}

// SelectInternalOrigin keeps the deployed service's TLS identity while using
// an explicitly configured private route from the credential-owning box.
func SelectInternalOrigin(client *http.Client, endpoint, connect string) error {
	if connect == "" {
		return nil
	}
	host, port, e := net.SplitHostPort(connect)
	if e != nil || host == "" || !edgePort(port) {
		return errors.New("internal connect target invalid")
	}
	u, e := url.Parse(endpoint)
	if e != nil {
		return e
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		return errors.New("internal TLS transport absent")
	}
	expected := u.Host
	if u.Port() == "" {
		expected = net.JoinHostPort(u.Hostname(), "443")
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != expected {
			return nil, errors.New("internal redirect target refused")
		}
		return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, connect)
	}
	return nil
}
func InspectEventRole(env map[string]string, kind string) error {
	plural := map[string]string{"journey": "journeys", "approval": "approvals", "payment": "payments", "program": "programs"}[kind]
	if plural == "" || env["LAYERX_EVENTS_KIND"] != plural {
		return errors.New("event source role mismatch")
	}
	var rows []struct {
		TokenFile string `json:"token_file"`
		Digest    bool   `json:"allow_principal_digest"`
	}
	b, e := os.ReadFile(env["LAYERX_EVENTS_PRODUCERS_FILE"])
	if e != nil || json.Unmarshal(b, &rows) != nil || len(rows) != 1 || rows[0].Digest != (kind == "program" || kind == "payment") {
		return errors.New("event producer roster invalid")
	}
	for _, path := range []string{rows[0].TokenFile, env["LAYERX_EVENTS_TOKEN_FILE"], env["LAYERX_EVENTS_UPSTREAM_CA_DER"], env["LAYERX_EVENTS_TLS_CERT_DER"], env["LAYERX_EVENTS_TLS_KEY_DER"], env["LAYERX_EVENTS_CLIENT_CA_DER"], env["LAYERX_EVENTS_CREDENTIALS_FILE"], env["LAYERX_EVENTS_ENROLLMENT_KEY_FILE"]} {
		if b, e = os.ReadFile(path); e != nil || len(bytes.TrimSpace(b)) == 0 {
			return errors.New("event source credential material absent")
		}
	}
	return protectedURL(env["LAYERX_EVENTS_UPSTREAM_URL"])
}
