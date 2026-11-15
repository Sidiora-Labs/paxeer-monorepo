package ceremonyfixture

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/archive"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/attestor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/envelope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/migrate"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/rehearsal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/testsupport"
)

type IdentityStates struct {
	*Fixture
	Deliverable, Waiting, External, Funded, Migrated string
}

func NewIdentityStates(t *testing.T, held bool) *IdentityStates {
	t.Helper()
	pg := testsupport.StartPostgres(t)
	testsupport.ApplyGatewayMigrations(t, pg.DB)
	master, masterB64 := testsupport.MasterKey(t)
	vectors := testsupport.GatewayVectors(t, masterB64, 5)
	f := &IdentityStates{}
	f.Deliverable = testsupport.InsertWallet(t, pg.DB, vectors[0], "standard")
	f.Waiting = testsupport.InsertWallet(t, pg.DB, vectors[1], "standard")
	f.External = testsupport.InsertWallet(t, pg.DB, vectors[2], "agent")
	f.Funded = testsupport.InsertWallet(t, pg.DB, vectors[3], "funded")
	testsupport.InsertFundedAccount(t, pg.DB, f.Funded)
	f.Migrated = testsupport.InsertWallet(t, pg.DB, vectors[4], "standard")
	testsupport.MarkMigrated(t, pg.DB, f.Migrated, migrate.SecpKeyID(f.Migrated))
	ids := []string{}
	if held {
		ids = append(ids, f.Deliverable)
	}
	f.Fixture = New(t, pg, master, vectors, ids)
	return f
}

func (f *Fixture) WalletState(t *testing.T, id string) string {
	t.Helper()
	var state string
	if err := f.DB.DB.QueryRow(`select row_to_json(w)::text || ':' || coalesce((select json_agg(p order by p.id)::text from account_provisioning p where p.wallet_id=w.id), 'null') from wallets w where w.id=$1::uuid`, id).Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

func (f *Fixture) AssertNoJournal(t *testing.T, ids ...string) {
	t.Helper()
	for _, dir := range []string{f.Options.CeremonyOptions.JournalDir, f.Options.DeliveryJournalDir} {
		for _, id := range ids {
			digest := sha256.Sum256([]byte(id))
			path := filepath.Join(dir, hex.EncodeToString(digest[:])+".sealed")
			for _, name := range []string{path, path + ".lock"} {
				if _, err := os.Lstat(name); !os.IsNotExist(err) {
					t.Fatalf("retained wallet has a ceremony journal or lock: %v", err)
				}
			}
		}
	}
}

func (f *Fixture) DeliveryOptions() migrate.Options {
	options := *f.Options.CeremonyOptions
	options.Rehearsal = false
	options.JournalDir = f.Options.DeliveryJournalDir
	return options
}

func (f *Fixture) CommandEnvironment(t *testing.T) map[string]string {
	t.Helper()
	env := f.Environment(t)
	env[migrate.EnvDatabaseURL] = f.DB.URL
	env[envelope.EnvMasterKey] = base64.StdEncoding.EncodeToString(f.Options.MasterKey)
	env[archive.EnvPath] = f.Options.ArchivePath
	env[archive.EnvPassphrase] = string(f.Options.Passphrase)
	env[rehearsal.EnvSourceURL] = f.Options.SourceURL
	env[rehearsal.EnvAdminURL] = f.Options.AdminURL
	env[rehearsal.EnvGatewayMigrationsDir] = f.Options.MigrationsDir
	config := f.Isolated.Config(t)
	var nodes, pins string
	for i, node := range config.Nodes {
		if i > 0 {
			nodes += ","
			pins += ","
		}
		nodes += node.ID + "=" + node.URL
		pins += node.ID + "=" + hex.EncodeToString(node.Pin[:])
	}
	env["CEREMONY_REHEARSAL_NODES"] = nodes
	env["CEREMONY_REHEARSAL_NODE_PINS"] = pins
	env["CEREMONY_REHEARSAL_TLS_CERT_FILE"] = config.CertFile
	env["CEREMONY_REHEARSAL_TLS_KEY_FILE"] = config.KeyFile
	env["CEREMONY_REHEARSAL_TLS_CA_FILE"] = config.CAFile
	env["CEREMONY_REHEARSAL_GATEWAY_TLS_CERT_FILE"] = config.GatewayCertFile
	env["CEREMONY_REHEARSAL_GATEWAY_TLS_KEY_FILE"] = config.GatewayKeyFile
	env["CEREMONY_REHEARSAL_OWNER_TOKENS_FILE"] = config.OwnerTokensFile
	env["CEREMONY_REHEARSAL_JOURNAL_DIR"] = f.Options.CeremonyOptions.JournalDir
	env["CEREMONY_REHEARSAL_RPC_URL"] = f.RPCURL
	return env
}

func (f *IdentityStates) AssertPlan(t *testing.T, plan *migrate.Plan) {
	t.Helper()
	if plan.Total != 5 || len(plan.Eligible) != 3 || len(plan.Funded) != 1 || plan.AlreadyMigrated != 1 || len(plan.Deliverable) != 1 || len(plan.WaitingIdentity) != 1 || len(plan.ExternalIdentity) != 1 {
		t.Fatalf("identity state counts: %+v", plan)
	}
	if plan.Eligible[0].ID != f.Deliverable || plan.Eligible[1].ID != f.Waiting || plan.Eligible[2].ID != f.External || plan.Deliverable[0].ID != f.Deliverable || plan.WaitingIdentity[0].ID != f.Waiting || plan.ExternalIdentity[0].ID != f.External || plan.Funded[0].ID != f.Funded {
		t.Fatal("identity state sets changed wallet order")
	}
}

func (f *Fixture) Client(t *testing.T) *attestor.Client {
	t.Helper()
	client, err := attestor.New(f.Delivery.Config(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}
