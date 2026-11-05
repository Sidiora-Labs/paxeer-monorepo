package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/archive"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/attestor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/ceremonyfixture"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/envelope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/migrate"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/rehearsal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/testsupport"
)

func invoke(env map[string]string, args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := run(context.Background(), args, func(k string) string { return env[k] }, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestPlanArchiveDeliverThroughTheCommand(t *testing.T) {
	pg := testsupport.StartPostgres(t)
	testsupport.ApplyGatewayMigrations(t, pg.DB)
	_, masterB64 := testsupport.MasterKey(t)
	vs := testsupport.GatewayVectors(t, masterB64, 5)
	standard := testsupport.InsertWallet(t, pg.DB, vs[0], "standard")
	second := testsupport.InsertWallet(t, pg.DB, vs[1], "standard")
	funded := testsupport.InsertWallet(t, pg.DB, vs[2], "funded")
	testsupport.InsertFundedAccount(t, pg.DB, funded)
	done := testsupport.InsertWallet(t, pg.DB, vs[3], "standard")
	testsupport.MarkMigrated(t, pg.DB, done, migrate.SecpKeyID(done))

	env := map[string]string{migrate.EnvDatabaseURL: pg.URL}
	code, out, errOut := invoke(env, "plan")
	if code != 0 {
		t.Fatalf("plan exit %d: %s", code, errOut)
	}
	if out != "wallets=4 eligible=2 funded=1 already_migrated=1\n" {
		t.Fatalf("plan printed %q", out)
	}

	master, err := base64.StdEncoding.DecodeString(masterB64)
	if err != nil {
		t.Fatal(err)
	}
	fixture := ceremonyfixture.New(t, pg, master, vs)
	for k, v := range fixture.Environment(t) {
		env[k] = v
	}
	env[envelope.EnvMasterKey] = masterB64
	env[archive.EnvPassphrase] = "operator held archive passphrase"
	env[archive.EnvPath] = filepath.Join(t.TempDir(), "funded.archive")

	code, _, errOut = invoke(env, "deliver")
	if code != 1 || !strings.Contains(errOut, "archive") {
		t.Fatalf("deliver before archive exit %d: %s", code, errOut)
	}

	code, out, errOut = invoke(env, "archive", "--report-only-counts")
	if code != 1 || out != "" || !strings.Contains(errOut, "archive: open") {
		t.Fatalf("archive counts before archive exit %d out %q: %s", code, out, errOut)
	}
	code, out, errOut = invoke(env, "deliver", "--report-only-counts")
	if code != 1 || out != "wallets=4 eligible=3 funded=1 migrated=1 eligible_not_migrated=2\n" || !strings.Contains(errOut, "eligible wallets are not migrated: 2 of 3") {
		t.Fatalf("deliver counts before deliver exit %d out %q: %s", code, out, errOut)
	}
	nodeBefore := fixture.Delivery.Fingerprint(t)
	if _, _, errOut := invoke(env, "deliver", "--report-only-counts"); !strings.Contains(errOut, "eligible wallets are not migrated") {
		t.Fatal(errOut)
	}
	if fixture.Delivery.Fingerprint(t) != nodeBefore {
		t.Fatal("deliver counts changed attestor state")
	}
	if _, err := os.Stat(env[archive.EnvPath]); !os.IsNotExist(err) {
		t.Fatalf("counts runs wrote an archive: %v", err)
	}

	code, out, errOut = invoke(env, "archive")
	if code != 0 || out != "funded_archived=1 verified=true\n" {
		t.Fatalf("archive exit %d out %q: %s", code, out, errOut)
	}
	sealed, err := os.ReadFile(env[archive.EnvPath])
	if err != nil {
		t.Fatal(err)
	}
	archiveCounts := fmt.Sprintf("funded_rows=1 archive_rows=1 verified=true archive_sha256=%x\n", sha256.Sum256(sealed))
	code, out, errOut = invoke(env, "archive", "--report-only-counts")
	if code != 0 || out != archiveCounts {
		t.Fatalf("archive counts exit %d out %q: %s", code, out, errOut)
	}

	fixture.Rehearse(t)
	code, out, errOut = invoke(env, "deliver")
	if code != 0 {
		t.Fatalf("deliver exit %d: %s", code, errOut)
	}
	if out != "eligible=2 funded_archived=1 read=2 verified=2 imported=2 refreshed=2 test_signed=2 matched=2\n" {
		t.Fatalf("deliver printed %q", out)
	}
	if strings.Contains(out+errOut, masterB64) {
		t.Fatal("output carries the master key")
	}
	fixture.AssertMigrated(t, []string{standard, second})
	if info, err := os.Stat(env[archive.EnvPath]); err != nil || info.Size() == 0 {
		t.Fatalf("funded archive missing after deliver: %v", err)
	}

	code, out, _ = invoke(env, "plan")
	if code != 0 || out != "wallets=4 eligible=0 funded=1 already_migrated=3\n" {
		t.Fatalf("plan after deliver exit %d printed %q", code, out)
	}
	code, out, errOut = invoke(env, "deliver", "--report-only-counts")
	if code != 0 || out != "wallets=4 eligible=3 funded=1 migrated=3 eligible_not_migrated=0\n" {
		t.Fatalf("deliver counts after deliver exit %d out %q: %s", code, out, errOut)
	}
	code, out, errOut = invoke(env, "archive", "--report-only-counts")
	if code != 0 || out != archiveCounts {
		t.Fatalf("archive counts after deliver exit %d out %q: %s", code, out, errOut)
	}

	late := testsupport.InsertWallet(t, pg.DB, vs[4], "funded")
	testsupport.InsertFundedAccount(t, pg.DB, late)
	code, out, errOut = invoke(env, "archive", "--report-only-counts")
	if code != 1 || !strings.HasPrefix(out, "funded_rows=2 archive_rows=1 verified=false archive_sha256=") || !strings.Contains(errOut, archive.ErrVerify.Error()) {
		t.Fatalf("archive counts with a new funded row exit %d out %q: %s", code, out, errOut)
	}
}

func TestDeliverExitsNonZeroOnMismatch(t *testing.T) {
	pg := testsupport.StartPostgres(t)
	testsupport.ApplyGatewayMigrations(t, pg.DB)
	master, masterB64 := testsupport.MasterKey(t)
	vs := testsupport.GatewayVectors(t, masterB64, 1)
	id := testsupport.InsertWallet(t, pg.DB, vs[0], "standard")
	fixture := ceremonyfixture.New(t, pg, master, vs)
	fixture.Rehearse(t)
	env := fixture.Environment(t)
	env[migrate.EnvDatabaseURL] = pg.URL
	env[envelope.EnvMasterKey] = masterB64
	env[archive.EnvPassphrase] = "operator held archive passphrase"
	env[archive.EnvPath] = filepath.Join(t.TempDir(), "funded.archive")
	if code, _, errOut := invoke(env, "archive"); code != 0 {
		t.Fatalf("archive exit %d: %s", code, errOut)
	}
	if _, err := pg.DB.Exec(`update wallets set address=$2 where id=$1::uuid`, id, common.HexToAddress("0x1234").Hex()); err != nil {
		t.Fatal(err)
	}
	before := fixture.Delivery.Fingerprint(t)
	code, out, errOut := invoke(env, "deliver")
	if code != 1 || !strings.Contains(errOut, migrate.ErrRehearsal.Error()) {
		t.Fatalf("deliver exit %d: %s", code, errOut)
	}
	if out != "eligible=1 funded_archived=0 read=0 verified=0 imported=0 refreshed=0 test_signed=0 matched=0\n" {
		t.Fatalf("deliver printed %q", out)
	}
	if fixture.Delivery.Fingerprint(t) != before {
		t.Fatal("refused delivery reached an attestor")
	}
	var migrated int
	if err := pg.DB.QueryRow(`select count(*) from wallets where migrated_at is not null`).Scan(&migrated); err != nil {
		t.Fatal(err)
	}
	if migrated != 0 {
		t.Fatalf("%d wallets marked migrated after a refused deliver", migrated)
	}
}

func TestUsageAndMissingEnvironment(t *testing.T) {
	if code, _, _ := invoke(map[string]string{}); code != 2 {
		t.Fatalf("no subcommand exit %d", code)
	}
	if code, _, _ := invoke(map[string]string{}, "export"); code != 2 {
		t.Fatalf("unknown subcommand exit %d", code)
	}
	for _, args := range [][]string{
		{"plan", "--report-only-counts"},
		{"deliver", "--delta"},
		{"archive", "--force"},
		{"rehearse", "--delta"},
		{"move", "--report-only-counts"},
		{"deliver", "--report-only-counts", "--report-only-counts"},
		{"move", "--delta", "extra"},
	} {
		code, out, errOut := invoke(map[string]string{}, args...)
		if code != 2 || out != "" || !strings.Contains(errOut, "usage: ceremony") {
			t.Fatalf("%v exit %d: %s", args, code, errOut)
		}
	}
	for _, args := range [][]string{{"move"}, {"move", "--delta"}} {
		code, _, errOut := invoke(map[string]string{}, args...)
		if code != 1 || !strings.Contains(errOut, rehearsal.EnvSourceURL) {
			t.Fatalf("%v without configuration exit %d: %s", args, code, errOut)
		}
	}
	code, _, errOut := invoke(map[string]string{}, "deliver", "--report-only-counts")
	if code != 1 || !strings.Contains(errOut, migrate.EnvDatabaseURL) {
		t.Fatalf("deliver counts without database exit %d: %s", code, errOut)
	}
	code, _, errOut = invoke(map[string]string{migrate.EnvDatabaseURL: "postgres://postgres@127.0.0.1:1/postgres"}, "archive", "--report-only-counts")
	if code != 1 || !strings.Contains(errOut, archive.EnvPath) {
		t.Fatalf("archive counts without an archive path exit %d: %s", code, errOut)
	}
	code, out, errOut := invoke(map[string]string{}, "rehearse", "--report-only-counts")
	if code != 1 || out != "" || !strings.Contains(errOut, "rehearsal: invalid configuration") {
		t.Fatalf("rehearse counts without configuration exit %d: %s", code, errOut)
	}
	code, _, errOut = invoke(map[string]string{}, "plan")
	if code != 1 || !strings.Contains(errOut, migrate.EnvDatabaseURL) {
		t.Fatalf("plan without database exit %d: %s", code, errOut)
	}
	code, out, errOut = invoke(map[string]string{}, "rehearse")
	if code != 1 || out != "" || !strings.Contains(errOut, "rehearsal: invalid configuration") {
		t.Fatalf("rehearse without configuration exit %d: %s", code, errOut)
	}
}

func TestMoveDeltaAndRehearseThroughTheCommand(t *testing.T) {
	pg := testsupport.StartPostgres(t)
	testsupport.ApplyGatewayLedger(t, pg.DB, "005_agent_actions.sql")
	_, masterB64 := testsupport.MasterKey(t)
	vs := testsupport.GatewayVectors(t, masterB64, 3)
	standard := testsupport.InsertWallet(t, pg.DB, vs[0], "standard")
	testsupport.InsertWallet(t, pg.DB, vs[1], "standard")
	funded := testsupport.InsertWallet(t, pg.DB, vs[2], "funded")
	testsupport.InsertFundedAccount(t, pg.DB, funded)
	testsupport.InsertSignature(t, pg.DB, standard)
	reader := pg.ReadOnlyRole(t, "ceremony_reader")
	target := pg.Database(t, "gateway_platform")
	env := map[string]string{
		rehearsal.EnvSourceURL:            reader,
		migrate.EnvDatabaseURL:            target.URL,
		rehearsal.EnvGatewayMigrationsDir: testsupport.GatewayMigrationsDir(t),
	}

	code, out, errOut := invoke(env, "move")
	if code != 0 {
		t.Fatalf("move exit %d: %s", code, errOut)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 14 || lines[13] != "tables=13 rows=18 ledger_before=5 applied_after=17" {
		t.Fatalf("move printed %q", out)
	}
	for _, l := range lines[:13] {
		f := strings.Fields(l)
		if len(f) != 5 || strings.TrimPrefix(f[1], "source_") != strings.TrimPrefix(f[2], "target_") || strings.TrimPrefix(f[3], "source_") != strings.TrimPrefix(f[4], "target_") {
			t.Fatalf("move line %q", l)
		}
	}
	if !strings.Contains(out, "table=wallets source_rows=3 target_rows=3 source_sha256=") {
		t.Fatalf("move printed %q", out)
	}
	if strings.Contains(out+errOut, vs[0].Address) || strings.Contains(out+errOut, "127.0.0.1") {
		t.Fatal("move output carries an address or a host")
	}
	code, _, errOut = invoke(env, "move")
	if code != 1 || !strings.Contains(errOut, rehearsal.ErrTargetNotEmpty.Error()) {
		t.Fatalf("second move exit %d: %s", code, errOut)
	}
	code, out, errOut = invoke(env, "move", "--delta")
	if code != 0 || !strings.HasSuffix(out, "tables=10 dropped=3 moved=0\n") {
		t.Fatalf("delta exit %d out %q: %s", code, out, errOut)
	}
	code, out, errOut = invoke(env, "plan")
	if code != 0 || out != "wallets=3 eligible=2 funded=1 already_migrated=0\n" {
		t.Fatalf("plan on the moved database exit %d out %q: %s", code, out, errOut)
	}

	master, err := base64.StdEncoding.DecodeString(masterB64)
	if err != nil {
		t.Fatal(err)
	}
	// The archival move source predates unified identity provisioning. Provision
	// the held standard identities before exercising the current ceremony contract.
	fixture := ceremonyfixture.New(t, pg, master, vs)
	env = fixture.Environment(t)
	env[rehearsal.EnvSourceURL] = fixture.Options.SourceURL
	env[rehearsal.EnvAdminURL] = pg.URL
	env[rehearsal.EnvGatewayMigrationsDir] = testsupport.GatewayMigrationsDir(t)
	env[envelope.EnvMasterKey] = masterB64
	env[archive.EnvPassphrase] = "operator held archive passphrase"
	config := fixture.Isolated.Config(t)
	var isolated, pins []string
	for _, node := range config.Nodes {
		isolated = append(isolated, node.ID+"="+node.URL)
		pins = append(pins, node.ID+"="+hex.EncodeToString(node.Pin[:]))
	}
	env["CEREMONY_REHEARSAL_NODES"] = strings.Join(isolated, ",")
	env["CEREMONY_REHEARSAL_NODE_PINS"] = strings.Join(pins, ",")
	env["CEREMONY_REHEARSAL_TLS_CERT_FILE"] = config.CertFile
	env["CEREMONY_REHEARSAL_TLS_KEY_FILE"] = config.KeyFile
	env["CEREMONY_REHEARSAL_TLS_CA_FILE"] = config.CAFile
	env["CEREMONY_REHEARSAL_GATEWAY_TLS_CERT_FILE"] = config.GatewayCertFile
	env["CEREMONY_REHEARSAL_GATEWAY_TLS_KEY_FILE"] = config.GatewayKeyFile
	env["CEREMONY_REHEARSAL_OWNER_TOKENS_FILE"] = config.OwnerTokensFile
	env["CEREMONY_REHEARSAL_JOURNAL_DIR"] = fixture.Options.CeremonyOptions.JournalDir
	env["CEREMONY_REHEARSAL_RPC_URL"] = fixture.RPCURL
	sourceBefore := testsupport.Fingerprint(t, pg.DB)
	var sourceRows int64
	for name, table := range sourceBefore {
		if name == "sequences" {
			continue
		}
		rows, _, _ := strings.Cut(table, ":")
		n, err := strconv.ParseInt(rows, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		sourceRows += n
	}
	copySummary := fmt.Sprintf("tables=%d rows=%d ledger_before=22 applied_after=0\n", len(sourceBefore)-1, sourceRows)
	deliveryBefore := fixture.Delivery.Fingerprint(t)
	summary := "wallets=3 eligible=2 funded_archived=1 already_migrated=0 read=2 verified=2 imported=2 refreshed=2 test_signed=2 matched=2\n"
	env[archive.EnvPath] = filepath.Join(t.TempDir(), "full.archive")
	code, out, errOut = invoke(env, "rehearse")
	if code != 0 || !strings.HasPrefix(out, summary+copySummary) || !strings.Contains(out, "\ntable=wallets source_rows=3 target_rows=3 source_sha256=") {
		t.Fatalf("rehearse exit %d out %q: %s", code, out, errOut)
	}
	env[archive.EnvPath] = filepath.Join(t.TempDir(), "counts.archive")
	code, out, errOut = invoke(env, "rehearse", "--report-only-counts")
	if code != 0 || !strings.HasPrefix(out, summary+copySummary) || !strings.Contains(out, "\ntable=wallets source_rows=3 target_rows=3\n") || strings.Contains(out, "sha256") {
		t.Fatalf("rehearse counts exit %d out %q: %s", code, out, errOut)
	}
	if !reflect.DeepEqual(sourceBefore, testsupport.Fingerprint(t, pg.DB)) || deliveryBefore != fixture.Delivery.Fingerprint(t) {
		t.Fatal("rehearsal changed its source or delivery custody")
	}
	if info, err := os.Stat(env["CEREMONY_REHEARSAL_RECEIPT"]); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("missing protected rehearsal receipt: %v", err)
	}
	var files int
	if err := pg.DB.QueryRow(`select count(*) from _migrations`).Scan(&files); err != nil {
		t.Fatal(err)
	}
	if files != 22 {
		t.Fatalf("source ledger has %d migrations", files)
	}
}

func TestRehearseRejectsMissingProtectedInputs(t *testing.T) {
	env := protectedCommandEnvironment(t)
	for _, name := range []string{
		rehearsal.EnvSourceURL, rehearsal.EnvGatewayMigrationsDir, rehearsal.EnvAdminURL,
		archive.EnvPath, archive.EnvPassphrase, envelope.EnvMasterKey,
		"CEREMONY_ID", "CEREMONY_JOURNAL_DIR", "CEREMONY_JOURNAL_KEY_FILE",
		"CEREMONY_REHEARSAL_RECEIPT", "CEREMONY_REHEARSAL_JOURNAL_DIR", "CEREMONY_REHEARSAL_RPC_URL",
		attestor.EnvNodes, attestor.EnvNodePins, attestor.EnvTLSCertFile,
		attestor.EnvTLSKeyFile, attestor.EnvTLSCAFile,
		"CEREMONY_GATEWAY_TLS_CERT_FILE", "CEREMONY_GATEWAY_TLS_KEY_FILE", "CEREMONY_OWNER_TOKENS_FILE",
		"CEREMONY_REHEARSAL_NODES", "CEREMONY_REHEARSAL_NODE_PINS",
		"CEREMONY_REHEARSAL_TLS_CERT_FILE", "CEREMONY_REHEARSAL_TLS_KEY_FILE", "CEREMONY_REHEARSAL_TLS_CA_FILE",
		"CEREMONY_REHEARSAL_GATEWAY_TLS_CERT_FILE", "CEREMONY_REHEARSAL_GATEWAY_TLS_KEY_FILE",
		"CEREMONY_REHEARSAL_OWNER_TOKENS_FILE",
	} {
		t.Run(name, func(t *testing.T) {
			missing := cloneCommandEnvironment(env)
			delete(missing, name)
			for _, args := range [][]string{{"rehearse"}, {"rehearse", reportOnlyCounts}} {
				code, out, errOut := invoke(missing, args...)
				if code != 1 || out != "" || !strings.HasPrefix(errOut, "ceremony rehearse: ") {
					t.Fatalf("%v without %s: exit=%d out=%q error=%q", args, name, code, out, errOut)
				}
				assertCommandSecretsRedacted(t, env, out+errOut)
			}
		})
	}
}
