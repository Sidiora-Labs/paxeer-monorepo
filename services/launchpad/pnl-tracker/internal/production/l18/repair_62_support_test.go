package l18

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const repair62Stream = "repair_62"

type repair62Object struct {
	Kind    string   `json:"kind"`
	Columns []string `json:"columns"`
	Source  string   `json:"source"`
}
type repair62Catalog struct {
	RegistrySHA256 string                    `json:"registry_sha256"`
	Applied        map[int64]string          `json:"applied"`
	Sources        map[string]string         `json:"sources"`
	Objects        map[string]repair62Object `json:"objects"`
}
type repair62Transfer struct {
	NFTID     string `json:"nft_id"`
	Height    uint64 `json:"height"`
	TxIndex   int64  `json:"tx_index"`
	LogIndex  int64  `json:"log_index"`
	Owner     string `json:"owner"`
	BlockHash string `json:"block_hash"`
}
type repair62Claim struct {
	Pool      string `json:"pool"`
	Wallet    string `json:"wallet"`
	Height    uint64 `json:"height"`
	TxIndex   int64  `json:"tx_index"`
	LogIndex  int64  `json:"log_index"`
	BlockHash string `json:"block_hash"`
	Data      string `json:"data"`
}
type repair62Market struct {
	ID        string `json:"id"`
	Pool      string `json:"pool"`
	Creator   string `json:"creator"`
	Height    uint64 `json:"height"`
	BlockHash string `json:"block_hash"`
	TxHash    string `json:"tx_hash"`
	TxIndex   string `json:"tx_index"`
	LogIndex  string `json:"log_index"`
}
type repair62Peers struct {
	Admin, Alice, Bob       string
	Initial, Aged, Advanced uint64
	Markets                 []repair62Market
	Transfers               []repair62Transfer
	Claims                  []repair62Claim
	Claimable               map[string]map[string]string
}

func repair62Root() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../../.."))
}
func repair62Load(t *testing.T, name string, dst any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repair62Root(), "services/launchpad/pnl-tracker/internal/production/l18/testdata/repair_62", name))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, dst); err != nil {
		t.Fatal(err)
	}
}
func repair62Hash(n uint64) string { return fmt.Sprintf("0x%064x", n) }
func repair62Uint(s string) uint64 {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		panic("invalid integer in peer transcript: " + s)
	}
	return n
}
func repair62Contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// The peer replays the reviewed bootstrap catalog and applies the *production*
// RoleManifest SQL to it. Missing objects/columns are errors, never suppressed.
// PostgreSQL itself is not emulated: only the committed read-view transcript
// and the service's durable write responses are served over its real protocol.
type repair62Database struct {
	mu         sync.Mutex
	catalog    repair62Catalog
	manifest   production.RoleManifest
	grants     map[string]map[string]bool
	peers      repair62Peers
	height     uint64
	requests   map[string][]string
	inbox      map[string]string
	community  map[string][]string
	scans      map[string]bool
	outbox     map[string]string
	audit      map[string]string
	queries    []string
	claimPages []int
}

func repair62Bootstrap(t *testing.T) *repair62Database {
	t.Helper()
	d := &repair62Database{grants: map[string]map[string]bool{}, requests: map[string][]string{}, inbox: map[string]string{}, community: map[string][]string{}, scans: map[string]bool{}, outbox: map[string]string{}, audit: map[string]string{}}
	repair62Load(t, "catalog.json", &d.catalog)
	repair62Load(t, "peers.json", &d.peers)
	d.height = d.peers.Initial
	var registry production.MigrationRegistry
	dir := filepath.Join(repair62Root(), "launchpad/spec/production-system/integration/migrations")
	raw, err := os.ReadFile(filepath.Join(dir, "registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	if production.Checksum(raw) != d.catalog.RegistrySHA256 {
		t.Fatal("bootstrap registry changed; refresh its catalog transcript")
	}
	if err = json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	// The full dependency plan, including legacy sources, is bound to its actual
	// reviewed bytes. A stale response fixture cannot qualify a changed schema.
	for path, sum := range d.catalog.Sources {
		raw, err := os.ReadFile(filepath.Join(repair62Root(), path))
		if err != nil {
			t.Fatal(err)
		}
		if production.Checksum(raw) != sum {
			t.Fatalf("bootstrap source changed: %s", path)
		}
	}
	applied := map[int64]bool{}
	for _, id := range registry.ApplicationOrder {
		for _, m := range registry.Registrations {
			if m.MigrationID != id || d.catalog.Applied[id] == "" {
				continue
			}
			if m.SHA256 != d.catalog.Applied[id] {
				t.Fatalf("migration %d checksum differs", id)
			}
			for _, predecessor := range m.PredecessorIDs {
				if !applied[predecessor] {
					t.Fatalf("migration %d missing predecessor %d", id, predecessor)
				}
			}
			applied[id] = true
		}
	}
	if len(applied) != len(d.catalog.Applied) {
		t.Fatal("incomplete bootstrap transcript")
	}
	var roles struct {
		Roles []production.RoleManifest `json:"roles"`
	}
	raw, err = os.ReadFile(filepath.Join(dir, "grants.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &roles); err != nil {
		t.Fatal(err)
	}
	for _, m := range roles.Roles {
		if m.RuntimeRole == "pnl_runtime" {
			d.manifest = m
		}
	}
	provisioning, err := d.manifest.ProvisionSQL()
	if err != nil {
		t.Fatal(err)
	}
	if len(provisioning) == 0 || d.manifest.Superuser || d.manifest.CreateDB || d.manifest.CreateRole || d.manifest.BypassRLS || d.manifest.RuntimeOwnsObjects {
		t.Fatal("runtime gained administrative authority")
	}
	for _, s := range provisioning {
		if strings.HasPrefix(s, `CREATE ROLE "pnl_runtime"`) && !strings.Contains(s, "NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS") {
			t.Fatal("unsafe runtime provisioning", s)
		}
	}
	statements, err := d.manifest.GrantSQL()
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range statements {
		if err = d.applyGrant(sql); err != nil {
			t.Fatalf("unmodified runtime GrantSQL failed: %s: %v", sql, err)
		}
	}
	return d
}

var repair62Grant = regexp.MustCompile(`^GRANT (.+) ON (TABLE|SEQUENCE|FUNCTION) "([a-z0-9_]+)"\."([a-z0-9_]+)"(?:\([^)]*\))? TO "([a-z0-9_]+)"$`)
var repair62Quoted = regexp.MustCompile(`"([a-z0-9_]+)"`)

func (d *repair62Database) applyGrant(sql string) error {
	if strings.HasPrefix(sql, "GRANT USAGE ON SCHEMA ") {
		return nil
	}
	m := repair62Grant.FindStringSubmatch(sql)
	if m == nil || m[5] != "pnl_runtime" {
		return fmt.Errorf("unrecognized grant: %s", sql)
	}
	object := m[3] + "." + m[4]
	obj, ok := d.catalog.Objects[object]
	if !ok {
		return &pgconn.PgError{Code: "42P01", Message: "missing bootstrap object " + object}
	}
	expected := strings.ToLower(m[2])
	if expected == "table" && obj.Kind == "view" {
		expected = "view"
	}
	if expected != obj.Kind {
		return fmt.Errorf("object kind mismatch for %s", object)
	}
	if d.grants[object] == nil {
		d.grants[object] = map[string]bool{}
	}
	privilege := m[1]
	if i := strings.IndexByte(privilege, '('); i >= 0 {
		p := strings.TrimSpace(privilege[:i])
		for _, col := range repair62Quoted.FindAllStringSubmatch(privilege[i:], -1) {
			if !repair62Contains(obj.Columns, col[1]) {
				return &pgconn.PgError{Code: "42703", Message: "missing grant column " + object + "." + col[1]}
			}
			d.grants[object][p+":"+col[1]] = true
		}
	} else {
		for _, p := range strings.Split(privilege, ",") {
			d.grants[object][strings.TrimSpace(p)+":*"] = true
		}
	}
	return nil
}
func (d *repair62Database) permit(object, privilege string, columns ...string) error {
	rights := d.grants[object]
	if rights[privilege+":*"] {
		return nil
	}
	for _, col := range columns {
		if !rights[privilege+":"+col] {
			return &pgconn.PgError{Code: "42501", Message: "permission denied: " + object + "." + col}
		}
	}
	if len(columns) == 0 {
		return &pgconn.PgError{Code: "42501", Message: "permission denied: " + object}
	}
	return nil
}

type repair62Result struct {
	types []uint32
	rows  [][]string
	tag   string
}

func repair62Rows(types []uint32, rows ...[]string) repair62Result {
	return repair62Result{types: types, rows: rows, tag: fmt.Sprintf("SELECT %d", len(rows))}
}
func repair62Text(rows ...[]string) repair62Result {
	var types []uint32
	if len(rows) > 0 {
		types = make([]uint32, len(rows[0]))
		for i := range types {
			types[i] = 25
		}
	}
	return repair62Rows(types, rows...)
}
func repair62Columns(columns string) []string {
	return strings.Fields(strings.ReplaceAll(columns, ",", " "))
}
func repair62Require(sql string, fragments ...string) error {
	for _, f := range fragments {
		if !strings.Contains(sql, f) {
			return fmt.Errorf("transcript predicate missing %q in %s", f, sql)
		}
	}
	return nil
}
func (d *repair62Database) owner(id string, height uint64) string {
	var latest *repair62Transfer
	for i := range d.peers.Transfers {
		x := &d.peers.Transfers[i]
		if x.NFTID != id || x.Height > height || x.BlockHash != repair62Hash(x.Height) {
			continue
		}
		if latest == nil || x.Height > latest.Height || x.Height == latest.Height && (x.TxIndex > latest.TxIndex || x.TxIndex == latest.TxIndex && x.LogIndex > latest.LogIndex) {
			latest = x
		}
	}
	if latest == nil {
		return ""
	}
	return latest.Owner
}
func repair62MarketRow(m repair62Market) []string {
	return []string{m.ID, m.Pool, m.Creator, strconv.FormatUint(m.Height, 10), m.BlockHash, m.TxHash, m.TxIndex, m.LogIndex}
}
func (d *repair62Database) query(sql string, args []string) (repair62Result, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	sql = strings.Join(strings.Fields(sql), " ")
	d.queries = append(d.queries, sql)
	ok := repair62Result{tag: "SELECT 1"}
	switch {
	case sql == "begin" || sql == "commit" || sql == "rollback":
		return repair62Result{tag: strings.ToUpper(sql)}, nil
	case strings.HasPrefix(sql, "SET LOCAL search_path TO "):
		return repair62Result{tag: "SET"}, nil
	case strings.Contains(sql, `"recovery".assert_epoch`):
		if err := d.permit("recovery.assert_epoch", "EXECUTE"); err != nil {
			return ok, err
		}
		if len(args) != 2 || args[0] != "1" || args[1] != "fixture-loopback" {
			return ok, fmt.Errorf("recovery fence mismatch")
		}
		return ok, nil
	case strings.HasPrefix(sql, "SELECT pg_advisory_xact_lock("):
		return ok, nil
	}
	// All cross-service reads must remain on the admitted views. This catches
	// accidental fallback to a base table even if a transcript could answer it.
	for _, name := range []string{"canonical_creations", "raw_logs", "block_completeness", "ingest_watermarks"} {
		object := "indexer.indexer_" + name
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(object) + `\b`)
		if re.MatchString(sql) {
			return ok, &pgconn.PgError{Code: "42501", Message: "private indexer table " + object}
		}
	}
	viewChecks := map[string][]string{
		"indexer.indexer_canonical_creations_v1": repair62Columns("nft_id,pool,creator,height,block_hash,transaction_hash,transaction_index,log_index"),
		"indexer.indexer_raw_logs_v1":            repair62Columns("chain_id,emitter,topics,height,transaction_index,log_index,block_hash,data"),
		"indexer.indexer_block_completeness_v1":  repair62Columns("chain_id,stream_id,height,block_hash,fetch_complete,required_decode_complete"),
		"indexer.indexer_ingest_watermarks_v1":   repair62Columns("chain_id,stream_id,raw_complete_height,typed_complete_height"),
	}
	for name, cols := range viewChecks {
		if strings.Contains(sql, name) {
			if err := d.permit(name, "SELECT", cols...); err != nil {
				return ok, err
			}
		}
	}
	switch {
	case strings.Contains(sql, "FROM indexer.indexer_ingest_watermarks_v1"):
		if len(args) != 1 || args[0] != repair62Stream {
			return ok, fmt.Errorf("unadmitted stream")
		}
		return repair62Rows([]uint32{20, 20}, []string{strconv.FormatUint(d.height, 10), strconv.FormatUint(d.height, 10)}), nil
	case strings.Contains(sql, "FROM indexer.indexer_block_completeness_v1"):
		if len(args) != 2 || args[0] != repair62Stream || repair62Uint(args[1]) > d.height {
			return ok, fmt.Errorf("unavailable source checkpoint")
		}
		return repair62Rows([]uint32{25, 16, 16}, []string{repair62Hash(repair62Uint(args[1])), "t", "t"}), nil
	case strings.Contains(sql, "FROM indexer.indexer_raw_logs_v1"):
		if err := repair62Require(sql, "b.stream_id=$1", "b.height=l.height", "b.block_hash=l.block_hash", "b.fetch_complete AND b.required_decode_complete", "l.chain_id=125", "l.topics->>0=$3"); err != nil {
			return ok, err
		}
		if args[0] != repair62Stream {
			return ok, fmt.Errorf("unadmitted stream")
		}
		if strings.Contains(sql, "SELECT l.topics->>2") {
			if err := repair62Require(sql, "l.emitter=$2", "l.topics->>3=$4", "jsonb_array_length(l.topics)=4", "l.height<=$5", "ORDER BY l.height DESC,l.transaction_index DESC,l.log_index DESC LIMIT 1"); err != nil {
				return ok, err
			}
			if args[1] != NFT || args[2] != nft.Events["Transfer"].ID.Hex() {
				return ok, fmt.Errorf("wrong NFT interface")
			}
			n := new(big.Int)
			n.SetString(strings.TrimPrefix(args[3], "0x"), 16)
			who := d.owner(n.String(), repair62Uint(args[4]))
			if who == "" {
				return repair62Text(), nil
			}
			return repair62Text([]string{addressTopic(who)}), nil
		}
		if err := repair62Require(sql, "l.emitter=$2", "l.topics->>1=$4", "l.topics->>2=$5", "l.height BETWEEN $6 AND $7", "(l.height,l.transaction_index,l.log_index)>($8,$9,$10)", "ORDER BY l.height,l.transaction_index,l.log_index LIMIT 500"); err != nil {
			return ok, err
		}
		if args[1] != Accumulator || args[2] != accumulator.Events["FeesClaimed"].ID.Hex() {
			return ok, fmt.Errorf("wrong claim interface")
		}
		from, to := repair62Uint(args[5]), repair62Uint(args[6])
		height, _ := strconv.ParseInt(args[7], 10, 64)
		ti, _ := strconv.ParseInt(args[8], 10, 64)
		li, _ := strconv.ParseInt(args[9], 10, 64)
		claims := append([]repair62Claim(nil), d.peers.Claims...)
		sort.Slice(claims, func(i, j int) bool {
			a, b := claims[i], claims[j]
			return a.Height < b.Height || a.Height == b.Height && (a.TxIndex < b.TxIndex || a.TxIndex == b.TxIndex && a.LogIndex < b.LogIndex)
		})
		result := repair62Rows([]uint32{20, 20, 20, 17, 3802})
		for _, x := range claims {
			if x.Height < from || x.Height > to || x.BlockHash != repair62Hash(x.Height) || addressTopic(x.Pool) != args[3] || addressTopic(x.Wallet) != args[4] {
				continue
			}
			if int64(x.Height) < height || int64(x.Height) == height && (x.TxIndex < ti || x.TxIndex == ti && x.LogIndex <= li) {
				continue
			}
			topics, _ := json.Marshal([]string{accumulator.Events["FeesClaimed"].ID.Hex(), addressTopic(x.Pool), addressTopic(x.Wallet)})
			result.rows = append(result.rows, []string{strconv.FormatUint(x.Height, 10), strconv.FormatInt(x.TxIndex, 10), strconv.FormatInt(x.LogIndex, 10), `\x` + strings.TrimPrefix(x.Data, "0x"), string(topics)})
			if len(result.rows) == 500 {
				break
			}
		}
		result.tag = fmt.Sprintf("SELECT %d", len(result.rows))
		d.claimPages = append(d.claimPages, len(result.rows))
		return result, nil
	case strings.Contains(sql, "FROM indexer.indexer_canonical_creations_v1"):
		var rows [][]string
		for _, m := range d.peers.Markets {
			if strings.Contains(sql, "LEFT JOIN l18_community_scan") {
				if m.Height <= d.height {
					rows = append(rows, []string{m.ID, m.Creator, m.Pool})
				}
				continue
			}
			if strings.Contains(sql, "nft_id=$1::numeric") {
				if m.ID != args[0] {
					continue
				}
				if strings.HasPrefix(sql, "SELECT creator,pool") {
					rows = append(rows, []string{m.Creator, m.Pool})
				} else {
					rows = append(rows, repair62MarketRow(m))
				}
			} else {
				if err := repair62Require(sql, "nft_id>$1::numeric AND height<=$2 ORDER BY nft_id LIMIT 100"); err != nil {
					return ok, err
				}
				if repair62Uint(m.ID) > repair62Uint(args[0]) && m.Height <= repair62Uint(args[1]) && len(rows) < 100 {
					rows = append(rows, repair62MarketRow(m))
				}
			}
		}
		types := []uint32{25, 25, 25, 20, 25, 25, 25, 25}
		if strings.HasPrefix(sql, "SELECT creator,pool") {
			types = []uint32{25, 25}
		}
		if strings.Contains(sql, "LEFT JOIN l18_community_scan") {
			types = []uint32{25, 25, 25}
		}
		return repair62Rows(types, rows...), nil
	case strings.HasPrefix(sql, "SELECT digest,response,expires_at,release_digest,interface_digest FROM l18_requests"):
		value, found := d.requests[args[0]+":"+args[1]]
		if !found {
			return repair62Rows([]uint32{25, 17, 1184, 25, 25}), nil
		}
		return repair62Rows([]uint32{25, 17, 1184, 25, 25}, value), nil
	case strings.HasPrefix(sql, "INSERT INTO l18_requests("):
		if err := d.permit("pnl.l18_requests", "INSERT"); err != nil {
			return ok, err
		}
		key := args[0] + ":" + args[1]
		if _, exists := d.requests[key]; exists {
			return ok, &pgconn.PgError{Code: "23505", Message: "duplicate request"}
		}
		d.requests[key] = []string{args[2], args[3], args[4], args[5], args[6]}
		return repair62Result{tag: "INSERT 0 1"}, nil
	case strings.HasPrefix(sql, "INSERT INTO l18_outbox("):
		if err := d.permit("pnl.l18_outbox", "INSERT"); err != nil {
			return ok, err
		}
		if _, exists := d.outbox[args[0]]; exists {
			return ok, &pgconn.PgError{Code: "23505", Message: "duplicate outbox identity"}
		}
		d.outbox[args[0]] = args[1]
		return repair62Result{tag: "INSERT 0 1"}, nil
	case strings.HasPrefix(sql, "INSERT INTO l18_audit("):
		if err := d.permit("pnl.l18_audit", "INSERT"); err != nil {
			return ok, err
		}
		d.audit[args[0]] = args[1]
		return repair62Result{tag: "INSERT 0 1"}, nil
	case strings.HasPrefix(sql, "INSERT INTO l18_inbox("):
		if err := d.permit("pnl.l18_inbox", "INSERT"); err != nil {
			return ok, err
		}
		if _, exists := d.inbox[args[0]]; exists {
			return repair62Result{tag: "INSERT 0 0"}, nil
		}
		d.inbox[args[0]] = args[1]
		return repair62Result{tag: "INSERT 0 1"}, nil
	case strings.HasPrefix(sql, "SELECT digest FROM l18_inbox"):
		v, exists := d.inbox[args[0]]
		if !exists {
			return repair62Text(), nil
		}
		return repair62Text([]string{v}), nil
	case strings.HasPrefix(sql, "SELECT owner FROM l18_community_history"):
		v, exists := d.community[args[0]]
		if !exists {
			return repair62Text(), nil
		}
		return repair62Text([]string{v[3]}), nil
	case strings.HasPrefix(sql, "SELECT EXISTS(SELECT 1 FROM l18_community_history"):
		v := d.community[args[0]]
		return repair62Rows([]uint32{16}, []string{strconv.FormatBool(len(v) > 2 && v[2] == "active")}), nil
	case strings.HasPrefix(sql, "INSERT INTO l18_community_history("):
		if err := d.permit("pnl.l18_community_history", "INSERT"); err != nil {
			return ok, err
		}
		d.community[args[1]] = append([]string(nil), args...)
		return repair62Result{tag: "INSERT 0 1"}, nil
	case strings.HasPrefix(sql, "INSERT INTO l18_community_scan("):
		if err := d.permit("pnl.l18_community_scan", "INSERT"); err != nil {
			return ok, err
		}
		d.scans[args[0]] = true
		return repair62Result{tag: "INSERT 0 1"}, nil
	default:
		return ok, fmt.Errorf("query absent from committed peer interface: %s", sql)
	}
}

// Serve real pgx queries through the PostgreSQL frontend/backend codec. Query
// parameters retain their wire types; neither l18 nor its pool is replaced.
func (d *repair62Database) pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() { defer wg.Done(); defer conn.Close(); d.serve(conn) }()
		}
	}()
	config, err := pgxpool.ParseConfig("postgres://pnl_runtime@" + listener.Addr().String() + "/repair62?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeDescribeExec
	config.MaxConns = 2
	config.ConnConfig.ConnectTimeout = 2 * time.Second
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); listener.Close(); wg.Wait() })
	return pool
}

// Statement metadata belongs to the committed peer interface. pgx uses it to
// encode real typed parameters (notably jsonb versus bytea) just as at runtime.
func repair62Signature(sql string) (params, fields []uint32) {
	sql = strings.Join(strings.Fields(sql), " ")
	placeholders := regexp.MustCompile(`\$([0-9]+)`).FindAllStringSubmatch(sql, -1)
	count := 0
	for _, p := range placeholders {
		n, _ := strconv.Atoi(p[1])
		if n > count {
			count = n
		}
	}
	params = make([]uint32, count)
	for i := range params {
		params[i] = 25
	}
	switch {
	case strings.Contains(sql, `"recovery".assert_epoch`):
		params = []uint32{20, 25}
	case strings.Contains(sql, "FROM indexer.indexer_ingest_watermarks_v1"):
		fields = []uint32{20, 20}
	case strings.Contains(sql, "FROM indexer.indexer_block_completeness_v1"):
		params = []uint32{25, 20}
		fields = []uint32{25, 16, 16}
	case strings.Contains(sql, "FROM indexer.indexer_raw_logs_v1"):
		if strings.Contains(sql, "SELECT l.topics->>2") {
			params = []uint32{25, 25, 25, 25, 20}
			fields = []uint32{25}
		} else {
			params = []uint32{25, 25, 25, 25, 25, 20, 20, 20, 20, 20}
			fields = []uint32{20, 20, 20, 17, 3802}
		}
	case strings.Contains(sql, "FROM indexer.indexer_canonical_creations_v1"):
		fields = []uint32{25, 25, 25, 20, 25, 25, 25, 25}
		if count == 1 {
			params[0] = 1700
		}
		if count == 2 {
			params = []uint32{1700, 20}
		}
		if strings.HasPrefix(sql, "SELECT creator,pool") {
			fields = []uint32{25, 25}
		}
		if strings.Contains(sql, "LEFT JOIN l18_community_scan") {
			fields = []uint32{25, 25, 25}
		}
	case strings.HasPrefix(sql, "SELECT digest,response,expires_at,release_digest,interface_digest FROM l18_requests"):
		params = []uint32{25, 2950}
		fields = []uint32{25, 17, 1184, 25, 25}
	case strings.HasPrefix(sql, "INSERT INTO l18_requests("):
		params = []uint32{25, 2950, 25, 17, 1184, 25, 25}
	case strings.HasPrefix(sql, "INSERT INTO l18_outbox("):
		params = []uint32{25, 25, 3802}
	case strings.HasPrefix(sql, "INSERT INTO l18_audit("):
		params = []uint32{25, 25, 3802}
	case strings.HasPrefix(sql, "SELECT digest FROM l18_inbox"):
		fields = []uint32{25}
	case strings.HasPrefix(sql, "SELECT owner FROM l18_community_history"):
		params = []uint32{1700}
		fields = []uint32{25}
	case strings.HasPrefix(sql, "SELECT EXISTS(SELECT 1 FROM l18_community_history"):
		params = []uint32{1700}
		fields = []uint32{16}
	case strings.HasPrefix(sql, "INSERT INTO l18_community_history("):
		params = []uint32{2950, 1700, 25, 25, 25, 25, 20, 25, 3802}
	case strings.HasPrefix(sql, "INSERT INTO l18_community_scan("):
		params = []uint32{1700}
	}
	return params, fields
}
func (d *repair62Database) serve(conn net.Conn) {
	b := pgproto3.NewBackend(conn, conn)
	startup, err := b.ReceiveStartupMessage()
	if err != nil {
		return
	}
	hello, ok := startup.(*pgproto3.StartupMessage)
	if !ok || hello.Parameters["user"] != "pnl_runtime" {
		return
	}
	b.Send(&pgproto3.AuthenticationOk{})
	b.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
	b.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if b.Flush() != nil {
		return
	}
	var sql string
	var args []string
	var result repair62Result
	var queryErr error
	status := byte('I')
	failed := false
	var params, fields []uint32
	var formats []int16
	typeMap := pgtype.NewMap()
	formatAt := func(codes []int16, index int) int16 {
		if len(codes) == 0 {
			return 0
		}
		if len(codes) == 1 {
			return codes[0]
		}
		return codes[index]
	}
	describe := func(types []uint32) {
		if len(types) == 0 {
			b.Send(&pgproto3.NoData{})
			return
		}
		descriptions := make([]pgproto3.FieldDescription, len(types))
		for i, oid := range types {
			descriptions[i] = pgproto3.FieldDescription{Name: []byte(fmt.Sprintf("column%d", i)), DataTypeOID: oid, DataTypeSize: -1, TypeModifier: -1, Format: 0}
		}
		b.Send(&pgproto3.RowDescription{Fields: descriptions})
	}
	sendError := func(err error) {
		fmt.Printf("repair62 PostgreSQL peer: %v\n", err)
		code := "XX000"
		if e, ok := err.(*pgconn.PgError); ok {
			code = e.Code
		}
		b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: code, Message: err.Error()})
		failed = true
	}
	sendRows := func() {
		for _, row := range result.rows {
			values := make([][]byte, len(row))
			for i, v := range row {
				format := formatAt(formats, i)
				if format == 0 {
					values[i] = []byte(v)
					continue
				}
				var value any
				if err := typeMap.Scan(result.types[i], 0, []byte(v), &value); err != nil {
					sendError(err)
					return
				}
				var err error
				values[i], err = typeMap.Encode(result.types[i], format, value, nil)
				if err != nil {
					sendError(err)
					return
				}
			}
			b.Send(&pgproto3.DataRow{Values: values})
		}
		b.Send(&pgproto3.CommandComplete{CommandTag: []byte(result.tag)})
	}
	for {
		message, err := b.Receive()
		if err != nil {
			return
		}
		switch m := message.(type) {
		case *pgproto3.Parse:
			sql = m.Query
			params, fields = repair62Signature(sql)
			queryErr = nil
			b.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Bind:
			formats = m.ResultFormatCodes
			args = make([]string, len(m.Parameters))
			queryErr = nil
			if len(params) != len(args) {
				queryErr = fmt.Errorf("parameter count differs from peer signature")
			} else {
				for i, raw := range m.Parameters {
					var value any
					if err := typeMap.Scan(params[i], formatAt(m.ParameterFormatCodes, i), raw, &value); err != nil {
						queryErr = err
						break
					}
					text, err := typeMap.Encode(params[i], 0, value, nil)
					if err != nil {
						queryErr = err
						break
					}
					args[i] = string(text)
				}
			}
			b.Send(&pgproto3.BindComplete{})
			if queryErr == nil {
				result, queryErr = d.query(sql, args)
			}
		case *pgproto3.Describe:
			if m.ObjectType == 'S' {
				b.Send(&pgproto3.ParameterDescription{ParameterOIDs: params})
				describe(fields)
			} else if queryErr != nil {
				sendError(queryErr)
			} else {
				describe(result.types)
			}
		case *pgproto3.Execute:
			if !failed {
				if queryErr != nil {
					sendError(queryErr)
				} else {
					sendRows()
				}
			}
		case *pgproto3.Sync:
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			failed = false
		case *pgproto3.Query:
			formats = nil
			result, err = d.query(m.String, nil)
			if err != nil {
				sendError(err)
			} else {
				switch strings.ToLower(m.String) {
				case "begin":
					status = 'T'
				case "commit", "rollback":
					status = 'I'
				}
				if len(result.types) > 0 {
					describe(result.types)
				}
				sendRows()
			}
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			failed = false
		case *pgproto3.Close:
			b.Send(&pgproto3.CloseComplete{})
		case *pgproto3.Terminate:
			return
		default:
			sendError(fmt.Errorf("unexpected PostgreSQL message %T", m))
		}
		if b.Flush() != nil {
			return
		}
	}
}

type repair62Fixture struct {
	db      *repair62Database
	ledger  *Ledger
	creator *Creator
	journey *JourneyService
	chain   struct{ Pool, Admin, Alice, Bob string }
	rpc     *rpc.Client
}

func repair62Chain(t *testing.T) *repair62Fixture {
	t.Helper()
	f := &repair62Fixture{db: repair62Bootstrap(t)}
	f.chain.Pool = f.db.peers.Markets[0].Pool
	f.chain.Admin = f.db.peers.Admin
	f.chain.Alice = f.db.peers.Alice
	f.chain.Bob = f.db.peers.Bob
	server := httptest.NewServer(http.HandlerFunc(f.db.rpcResponse))
	t.Cleanup(server.Close)
	var err error
	f.rpc, err = rpc.DialHTTP(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.rpc.Close)
	latest := NewCreator(f.rpc, time.Now)
	committed := func(ctx context.Context) (Checkpoint, error) { return latest.checkpoint(ctx) }
	runtimeHash := strings.ToLower(crypto.Keccak256Hash([]byte{0x60, 0, 0x60, 0}).Hex())
	hashes := map[string]string{NFT: runtimeHash, Accumulator: runtimeHash, FeesRouter: runtimeHash, USDL: runtimeHash}
	f.creator, err = NewCommittedCreator(context.Background(), f.rpc, time.Now, committed, ManifestDigest, hashes)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := queue.NewEpochGuard("recovery", 1, "fixture-loopback")
	if err != nil {
		t.Fatal(err)
	}
	f.ledger, err = NewRuntimeLedger(f.db.pool(t), "pnl", "repair-62", nil, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	f.journey, err = NewJourneyService(f.ledger, f.creator, []byte(strings.Repeat("r", 32)), time.Now, strings.Repeat("a", 64), strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	f.journey, err = f.journey.WithSourceStream(repair62Stream)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *repair62Fixture) advance(height uint64) Checkpoint {
	f.db.mu.Lock()
	defer f.db.mu.Unlock()
	f.db.height = height
	return Checkpoint{Number: height, Hash: repair62Hash(height)}
}
func repair62Pack(a abi.ABI, method string, value any) (string, error) {
	raw, err := a.Methods[method].Outputs.Pack(value)
	return hexutil.Encode(raw), err
}
func (d *repair62Database) rpcResponse(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var req struct {
		ID     json.RawMessage   `json:"id"`
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	result, err := d.rpcResult(req.Method, req.Params)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32000, "message": err.Error()}})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
}
func (d *repair62Database) rpcResult(method string, args []json.RawMessage) (any, error) {
	switch method {
	case "eth_chainId":
		return "0x7d", nil
	case "eth_getBlockByNumber":
		var number string
		if err := json.Unmarshal(args[0], &number); err != nil {
			return nil, err
		}
		height := d.height
		if number != "latest" {
			var err error
			height, err = hexutil.DecodeUint64(number)
			if err != nil {
				return nil, err
			}
		}
		if height > d.height {
			return nil, fmt.Errorf("unrecorded future block")
		}
		return map[string]string{"number": hexutil.EncodeUint64(height), "hash": repair62Hash(height)}, nil
	case "eth_gasPrice":
		return "0x3b9aca00", nil
	case "eth_call", "eth_getCode", "eth_getStorageAt", "eth_estimateGas":
		selectorIndex := 1
		if method == "eth_getStorageAt" {
			selectorIndex = 2
		}
		var selector struct {
			Hash      string `json:"blockHash"`
			Canonical bool   `json:"requireCanonical"`
		}
		if len(args) <= selectorIndex || json.Unmarshal(args[selectorIndex], &selector) != nil || !selector.Canonical || !hash.MatchString(selector.Hash) {
			return nil, fmt.Errorf("unpinned read")
		}
		height, err := strconv.ParseUint(selector.Hash[2:], 16, 64)
		if err != nil || height > d.height {
			return nil, fmt.Errorf("unrecorded checkpoint")
		}
		if method == "eth_getCode" {
			return "0x60006000", nil
		}
		if method == "eth_getStorageAt" {
			return "0x" + strings.Repeat("0", 64), nil
		}
		if method == "eth_estimateGas" {
			return "0x186a0", nil
		}
		var call struct{ To, Data string }
		if err = json.Unmarshal(args[0], &call); err != nil {
			return nil, err
		}
		data, err := hexutil.Decode(call.Data)
		if err != nil || len(data) < 4 {
			return nil, fmt.Errorf("invalid ABI call")
		}
		var a abi.ABI
		switch strings.ToLower(call.To) {
		case NFT:
			a = nft
		case Accumulator:
			a = accumulator
		default:
			return nil, fmt.Errorf("unrecorded target %s", call.To)
		}
		m, err := a.MethodById(data[:4])
		if err != nil {
			return nil, err
		}
		values, err := m.Inputs.Unpack(data[4:])
		if err != nil {
			return nil, err
		}
		if len(values) != 1 {
			return nil, fmt.Errorf("unrecorded call %s", m.Name)
		}
		switch m.Name {
		case "ownerOf", "getPoolAddress", "getFeeStrategy":
			id := values[0].(*big.Int).String()
			for _, market := range d.peers.Markets {
				if market.ID != id || market.Height > height {
					continue
				}
				switch m.Name {
				case "ownerOf":
					return repair62Pack(a, m.Name, common.HexToAddress(d.owner(id, height)))
				case "getPoolAddress":
					return repair62Pack(a, m.Name, common.HexToAddress(market.Pool))
				case "getFeeStrategy":
					return repair62Pack(a, m.Name, uint8(0))
				}
			}
		case "supportsAirdrop":
			return repair62Pack(a, m.Name, false)
		case "getAccumulatedFees":
			pool := strings.ToLower(values[0].(common.Address).Hex())
			for _, market := range d.peers.Markets {
				if market.Pool != pool {
					continue
				}
				raw, exists := d.peers.Claimable[strconv.FormatUint(height, 10)][market.ID]
				if !exists {
					return nil, fmt.Errorf("unrecorded accumulated fees")
				}
				n, _ := new(big.Int).SetString(raw, 10)
				return repair62Pack(a, m.Name, n)
			}
		}
		return nil, fmt.Errorf("unrecorded ABI response: %s", m.Name)
	default:
		return nil, fmt.Errorf("method absent from committed RPC peer: %s", method)
	}
}
