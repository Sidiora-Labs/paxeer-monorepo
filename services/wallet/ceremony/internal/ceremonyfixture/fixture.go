package ceremonyfixture

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/attestor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/migrate"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/rehearsal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/testsupport"
)

// Fixture uses the gateway's real DKG and binding clients, genuine owner JWTs,
// and independent production attestors. The existing gateway TestChain supplies
// the local chain boundary; no attestor response or signature is simulated.
type Fixture struct {
	Delivery, Isolated *testsupport.CeremonyNodes
	RPCURL             string
	DB                 *testsupport.Postgres
	Options            rehearsal.Options
}

func New(t *testing.T, pg *testsupport.Postgres, master []byte, vectors []testsupport.Vector) *Fixture {
	t.Helper()
	advanceLedger(t, pg.DB)
	wallet := testsupport.WalletDir(t)
	script := filepath.Join(wallet, "ceremony", "testdata", "original-identities.mts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)
	command := exec.CommandContext(ctx, "pnpm", "exec", "tsx", script)
	command.Dir = filepath.Join(wallet, "gateway")
	command.Env = append(os.Environ(), "NODE_ENV=test", "CORS_ORIGINS=http://127.0.0.1", "SUPABASE_URL=http://127.0.0.1:1", "DATABASE_URL="+pg.URL, "WALLET_MASTER_KEY=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.CreateTemp(t.TempDir(), "identity-producer-*.stderr")
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = stderr
	diagnostic := func() string { raw, _ := os.ReadFile(stderr.Name()); return string(raw) }
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		input.Close()
		if err := command.Wait(); err != nil {
			t.Errorf("identity producer: %v\n%s", err, diagnostic())
		}
		stderr.Close()
	})
	decoder := json.NewDecoder(output)
	receive := func(out any) {
		t.Helper()
		if err := decoder.Decode(out); err != nil {
			t.Fatalf("identity producer: %v\n%s", err, diagnostic())
		}
	}
	send := func(value any, out any) {
		t.Helper()
		if err := json.NewEncoder(input).Encode(value); err != nil {
			t.Fatal(err)
		}
		receive(out)
	}
	var ready struct {
		URL string `json:"url"`
	}
	receive(&ready)
	f := &Fixture{RPCURL: ready.URL, DB: pg}
	f.Delivery = testsupport.StartWalletNodes(t, f.RPCURL)
	cfg := f.Delivery.Config(t)
	wallets, err := migrate.ReadWallets(context.Background(), pg.DB)
	if err != nil {
		t.Fatal(err)
	}
	var approved []testsupport.CeremonyInventoryKey
	for _, w := range wallets {
		if w.Kind != "standard" || w.Funded || w.Migrated {
			continue
		}
		identity := migrate.IdentityKeyID(w.ID)
		approved = append(approved, testsupport.CeremonyInventoryKey{KeyID: identity, Curve: attestor.CurveEd25519, Owner: w.UserID, Operations: []string{"generate"}})
	}
	f.Delivery.Approve(t, approved)
	for _, w := range wallets {
		if w.Kind != "standard" || w.Funded || w.Migrated {
			continue
		}
		identity := migrate.IdentityKeyID(w.ID)
		var generated struct {
			PublicKey string `json:"publicKey"`
		}
		send(map[string]any{"op": "generate", "authority": f.Delivery.GatewayAuthority(), "config": cfg, "keyId": identity, "owner": w.UserID, "address": w.Address}, &generated)
		for i := range approved {
			if approved[i].KeyID == identity {
				approved[i].PublicKey = generated.PublicKey
				approved[i].Operations = []string{"sign", "refresh"}
			}
		}
		f.Delivery.Approve(t, approved)
		var vector *testsupport.Vector
		for i := range vectors {
			if strings.EqualFold(vectors[i].Address, w.Address) {
				vector = &vectors[i]
				break
			}
		}
		if vector == nil {
			t.Fatalf("no gateway-produced key for %s", w.Address)
		}
		token := f.Delivery.OwnerToken(t, w.UserID)
		var binding struct{ Signature, PublicKey, DID, Main string }
		send(map[string]any{"op": "bind", "authority": f.Delivery.GatewayAuthority(), "config": cfg, "keyId": identity, "owner": w.UserID, "address": w.Address, "privateKey": vector.Key, "publicKey": generated.PublicKey, "token": token}, &binding)
		if binding.PublicKey != generated.PublicKey {
			t.Fatal("gateway changed the generated identity")
		}
		if _, err = pg.DB.Exec(`insert into account_provisioning(user_id,kind,wallet_id,state,ed_key_id,ed_public_key,did,main_account_id,bind_nonce,bind_signature)
   values($1::uuid,'standard',$2::uuid,'active',$3,$4,$5,$6,0,$7)`, w.UserID, w.ID, identity, binding.PublicKey, binding.DID, binding.Main, binding.Signature); err != nil {
			t.Fatal(err)
		}
		if _, err = pg.DB.Exec(`update wallets set did=$2,main_account_id=$3,layerx_key_id=$4,binding_state='bound' where id=$1::uuid`, w.ID, binding.DID, binding.Main, identity); err != nil {
			t.Fatal(err)
		}
		key, err := crypto.HexToECDSA(vector.Key)
		if err != nil {
			t.Fatal(err)
		}
		public := hex.EncodeToString(crypto.FromECDSAPub(&key.PublicKey))
		approved = append(approved, testsupport.CeremonyInventoryKey{KeyID: migrate.SecpKeyID(w.ID), Curve: attestor.CurveSecp256k1, PublicKey: public, Owner: w.UserID, Operations: []string{"import", "refresh", "sign"}})
		f.Delivery.Approve(t, approved)
	}
	f.Isolated = f.Delivery.Clone(t)
	delivery, isolated := f.Delivery.Config(t), f.Isolated.Config(t)
	root := protectedDir(t)
	journalKey := bytes.Repeat([]byte{0x51}, 32)
	ceremony := migrate.Options{CeremonyID: "wallet-go-22-31-513", JournalDir: protectedDir(t), JournalKey: journalKey, RehearsalReceipt: filepath.Join(root, "rehearsal.sealed"), Rehearsal: true, RPCURL: f.RPCURL}
	f.Options = rehearsal.Options{SourceURL: pg.ReadOnlyRole(t, "identity_ceremony_reader"), AdminURL: pg.URL, MigrationsDir: testsupport.GatewayMigrationsDir(t), MasterKey: master, ArchivePath: filepath.Join(root, "funded.archive"), Passphrase: []byte("operator held archive passphrase"), CeremonyOptions: &ceremony, AttestorConfig: &isolated, DeliveryAttestorConfig: &delivery, DeliveryJournalDir: protectedDir(t)}
	return f
}

func protectedDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func advanceLedger(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`create table if not exists _migrations(filename text primary key,applied_at timestamptz not null default now())`); err != nil {
		t.Fatal(err)
	}
	for _, name := range testsupport.GatewayMigrationFiles(t) {
		var exists bool
		if err := db.QueryRow(`select exists(select 1 from _migrations where filename=$1)`, name).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(testsupport.GatewayMigrationsDir(t), name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(raw)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
		if _, err = db.Exec(`insert into _migrations(filename) values($1)`, name); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *Fixture) Rehearse(t *testing.T) {
	t.Helper()
	opts := f.Options
	opts.CountsOnly = true
	report, err := rehearsal.Rehearse(context.Background(), opts)
	if err != nil {
		t.Fatalf("real rehearsal: %v (%s)", err, report)
	}
	if report.Matched != report.Eligible || report.HeldIdentity != report.Eligible {
		t.Fatalf("incomplete rehearsal: %s", report)
	}
}

func (f *Fixture) Environment(t *testing.T) map[string]string {
	t.Helper()
	cfg := f.Delivery.Config(t)
	var urls, pins []string
	for _, node := range cfg.Nodes {
		urls = append(urls, node.ID+"="+node.URL)
		pins = append(pins, node.ID+"="+hex.EncodeToString(node.Pin[:]))
	}
	keyFile := filepath.Join(protectedDir(t), "journal.key")
	if err := os.WriteFile(keyFile, f.Options.CeremonyOptions.JournalKey, 0600); err != nil {
		t.Fatal(err)
	}
	return map[string]string{attestor.EnvNodes: strings.Join(urls, ","), attestor.EnvNodePins: strings.Join(pins, ","), attestor.EnvTLSCertFile: cfg.CertFile, attestor.EnvTLSKeyFile: cfg.KeyFile, attestor.EnvTLSCAFile: cfg.CAFile,
		"CEREMONY_GATEWAY_TLS_CERT_FILE": cfg.GatewayCertFile, "CEREMONY_GATEWAY_TLS_KEY_FILE": cfg.GatewayKeyFile, "CEREMONY_OWNER_TOKENS_FILE": cfg.OwnerTokensFile,
		"CEREMONY_ID": f.Options.CeremonyOptions.CeremonyID, "CEREMONY_JOURNAL_DIR": f.Options.DeliveryJournalDir, "CEREMONY_JOURNAL_KEY_FILE": keyFile, "CEREMONY_REHEARSAL_RECEIPT": f.Options.CeremonyOptions.RehearsalReceipt, "CEREMONY_RPC_URL": f.RPCURL}
}

func (f *Fixture) AssertMigrated(t *testing.T, ids []string) {
	t.Helper()
	client, err := attestor.New(f.Delivery.Config(t))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for _, id := range ids {
		var did, main, identity string
		if err = f.DB.DB.QueryRow(`select did,main_account_id,layerx_key_id from wallets where id=$1::uuid and migrated_at is not null`, id).Scan(&did, &main, &identity); err != nil {
			t.Fatal(err)
		}
		for _, keyID := range []string{migrate.SecpKeyID(id), identity} {
			rows, err := client.DescribeCeremony(context.Background(), keyID)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				if row.Epoch != 1 || row.RefreshState != "complete" {
					t.Fatalf("incomplete custody: %+v", row)
				}
				if keyID == identity && ("did:layerx:"+row.PublicKey != did || row.DID != did) {
					t.Fatalf("original identity replaced: %+v", row)
				}
			}
		}
		if main == "" {
			t.Fatal("main account lost")
		}
	}
}
