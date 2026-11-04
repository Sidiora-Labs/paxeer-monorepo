package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/archive"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/attestor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/envelope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/migrate"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/rehearsal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/testsupport"
)

func protectedCommandEnvironment(t *testing.T) map[string]string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	journal := func(name string) string {
		path := filepath.Join(dir, name)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		return path
	}
	key := filepath.Join(dir, "journal.key")
	if err := os.WriteFile(key, bytes.Repeat([]byte{0xa3}, 32), 0600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		rehearsal.EnvSourceURL:            "postgres://ceremony_reader@127.0.0.1:1/wallets?sslmode=disable",
		rehearsal.EnvAdminURL:             "postgres://ceremony_admin@127.0.0.1:1/postgres?sslmode=disable",
		rehearsal.EnvGatewayMigrationsDir: dir,
		archive.EnvPath:                   filepath.Join(dir, "funded.archive"),
		archive.EnvPassphrase:             "operator held archive passphrase",
		envelope.EnvMasterKey:             "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"CEREMONY_ID":                     "repair-m173",
		"CEREMONY_JOURNAL_DIR":            journal("delivery"),
		"CEREMONY_JOURNAL_KEY_FILE":       key,
		"CEREMONY_REHEARSAL_RECEIPT":      filepath.Join(dir, "rehearsal.receipt"),
		"CEREMONY_REHEARSAL_JOURNAL_DIR":  journal("rehearsal"),
		"CEREMONY_RPC_URL":                "http://127.0.0.1:1/delivery",
		"CEREMONY_REHEARSAL_RPC_URL":      "http://127.0.0.1:1/rehearsal",
	}
	operator := testsupport.NewTLSFiles(t)
	gateway := testsupport.NewTLSFiles(t)
	var pins, delivery, isolated []string
	for i := 1; i <= attestor.NodeCount; i++ {
		id := fmt.Sprintf("original-node-%d", i)
		_, pin := operator.ServerCertificate(t, id)
		pins = append(pins, id+"="+hex.EncodeToString(pin[:]))
		delivery = append(delivery, fmt.Sprintf("%s=https://127.0.0.1:%d", id, 10000+i))
		isolated = append(isolated, fmt.Sprintf("%s=https://127.0.0.1:%d", id, 20000+i))
	}
	for _, prefix := range []string{"CEREMONY_", "CEREMONY_REHEARSAL_"} {
		env[prefix+"TLS_CERT_FILE"] = operator.CertFile
		env[prefix+"TLS_KEY_FILE"] = operator.KeyFile
		env[prefix+"TLS_CA_FILE"] = operator.CAFile
		env[prefix+"GATEWAY_TLS_CERT_FILE"] = gateway.CertFile
		env[prefix+"GATEWAY_TLS_KEY_FILE"] = gateway.KeyFile
		tokens := filepath.Join(dir, prefix+"owner-tokens.json")
		if err := os.WriteFile(tokens, []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}
		env[prefix+"OWNER_TOKENS_FILE"] = tokens
		env[prefix+"NODE_PINS"] = strings.Join(pins, ",")
	}
	env[attestor.EnvNodes] = strings.Join(delivery, ",")
	env["CEREMONY_REHEARSAL_NODES"] = strings.Join(isolated, ",")
	return env
}

func cloneCommandEnvironment(env map[string]string) map[string]string {
	copy := make(map[string]string, len(env))
	for name, value := range env {
		copy[name] = value
	}
	return copy
}

func assertCommandSecretsRedacted(t *testing.T, env map[string]string, output string) {
	t.Helper()
	for _, name := range []string{archive.EnvPassphrase, envelope.EnvMasterKey, rehearsal.EnvSourceURL, rehearsal.EnvAdminURL, "CEREMONY_JOURNAL_KEY_FILE"} {
		if env[name] != "" && strings.Contains(output, env[name]) {
			t.Fatalf("command output exposes %s", name)
		}
	}
}

func TestRepairM_173_MissingProtectedInputs(t *testing.T) {
	TestRehearseRejectsMissingProtectedInputs(t)
	TestUsageAndMissingEnvironment(t)
}

func TestRepairM_173_ProtectedJournalInputs(t *testing.T) {
	env := protectedCommandEnvironment(t)
	get := func(name string) string { return env[name] }
	opts, err := migrate.LoadOptions(get)
	if err != nil {
		t.Fatal(err)
	}
	if opts.CeremonyID != env["CEREMONY_ID"] || opts.JournalDir != env["CEREMONY_JOURNAL_DIR"] || opts.RehearsalReceipt != env["CEREMONY_REHEARSAL_RECEIPT"] || opts.RPCURL != env["CEREMONY_RPC_URL"] || opts.Rehearsal || !bytes.Equal(opts.JournalKey, bytes.Repeat([]byte{0xa3}, 32)) {
		t.Fatal("delivery options lost protected input binding")
	}
	zero(opts.JournalKey)
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, map[string]string)
	}{
		{"short key", func(t *testing.T, e map[string]string) { writeCommandKey(t, e, 31, 0600) }},
		{"long key", func(t *testing.T, e map[string]string) { writeCommandKey(t, e, 33, 0600) }},
		{"readable key", func(t *testing.T, e map[string]string) { writeCommandKey(t, e, 32, 0644) }},
		{"relative key", func(t *testing.T, e map[string]string) { e["CEREMONY_JOURNAL_KEY_FILE"] = "journal.key" }},
		{"symlink key", func(t *testing.T, e map[string]string) {
			path := filepath.Join(t.TempDir(), "linked.key")
			if err := os.Symlink(e["CEREMONY_JOURNAL_KEY_FILE"], path); err != nil {
				t.Fatal(err)
			}
			e["CEREMONY_JOURNAL_KEY_FILE"] = path
		}},
		{"shared journal", func(t *testing.T, e map[string]string) {
			if err := os.Chmod(e["CEREMONY_JOURNAL_DIR"], 0755); err != nil {
				t.Fatal(err)
			}
		}},
		{"relative receipt", func(t *testing.T, e map[string]string) { e["CEREMONY_REHEARSAL_RECEIPT"] = "receipt" }},
		{"unsafe ceremony", func(t *testing.T, e map[string]string) { e["CEREMONY_ID"] = "../other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := protectedCommandEnvironment(t)
			tc.mutate(t, e)
			got, err := migrate.LoadOptions(func(name string) string { return e[name] })
			if !errors.Is(err, migrate.ErrJournal) || len(got.JournalKey) != 0 {
				t.Fatalf("unsafe protected input accepted: err=%v key length=%d", err, len(got.JournalKey))
			}
		})
	}
}

func writeCommandKey(t *testing.T, env map[string]string, size int, mode os.FileMode) {
	t.Helper()
	path := env["CEREMONY_JOURNAL_KEY_FILE"]
	if err := os.WriteFile(path, bytes.Repeat([]byte{0xa3}, size), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func TestRepairM_173_IsolatedConfiguration(t *testing.T) {
	env := protectedCommandEnvironment(t)
	opts, err := rehearsal.LoadOptions(func(name string) string { return env[name] })
	if err != nil {
		t.Fatal(err)
	}
	defer opts.Wipe()
	if !opts.CeremonyOptions.Rehearsal || opts.CeremonyOptions.JournalDir != env["CEREMONY_REHEARSAL_JOURNAL_DIR"] || opts.CeremonyOptions.RPCURL != env["CEREMONY_REHEARSAL_RPC_URL"] || opts.DeliveryJournalDir != env["CEREMONY_JOURNAL_DIR"] {
		t.Fatal("rehearsal configuration lost isolation")
	}
	live, err := attestor.New(*opts.DeliveryAttestorConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	isolated, err := attestor.New(*opts.AttestorConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer isolated.Close()
	for i, member := range live.Membership() {
		if member != isolated.Membership()[i] || opts.DeliveryAttestorConfig.Nodes[i].URL == opts.AttestorConfig.Nodes[i].URL {
			t.Fatal("isolated group changed original member identity or reused delivery endpoint")
		}
	}
	opts.Wipe()
	for _, secret := range [][]byte{opts.MasterKey, opts.Passphrase, opts.CeremonyOptions.JournalKey} {
		if !bytes.Equal(secret, make([]byte, len(secret))) {
			t.Fatal("rehearsal options retained secret bytes")
		}
	}
}

func TestRepairM_173_RejectsIsolationConflicts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, map[string]string)
	}{
		{"delivery endpoints", func(t *testing.T, e map[string]string) { e["CEREMONY_REHEARSAL_NODES"] = e[attestor.EnvNodes] }},
		{"different identities", func(t *testing.T, e map[string]string) {
			pairs := strings.Split(e[attestor.EnvNodePins], ",")
			id, _, _ := strings.Cut(pairs[0], "=")
			pairs[0] = id + "=" + strings.Repeat("00", 32)
			e["CEREMONY_REHEARSAL_NODE_PINS"] = strings.Join(pairs, ",")
		}},
		{"same journal", func(t *testing.T, e map[string]string) {
			e["CEREMONY_REHEARSAL_JOURNAL_DIR"] = e["CEREMONY_JOURNAL_DIR"]
		}},
		{"journal alias", func(t *testing.T, e map[string]string) {
			path := filepath.Join(t.TempDir(), "journal-link")
			if err := os.Symlink(e["CEREMONY_JOURNAL_DIR"], path); err != nil {
				t.Fatal(err)
			}
			e["CEREMONY_REHEARSAL_JOURNAL_DIR"] = path
		}},
		{"operator as gateway", func(t *testing.T, e map[string]string) {
			e["CEREMONY_REHEARSAL_GATEWAY_TLS_CERT_FILE"] = e["CEREMONY_REHEARSAL_TLS_CERT_FILE"]
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := protectedCommandEnvironment(t)
			tc.mutate(t, env)
			for _, args := range [][]string{{"rehearse"}, {"rehearse", reportOnlyCounts}} {
				code, out, errOut := invoke(env, args...)
				if code != 1 || out != "" || !strings.HasPrefix(errOut, "ceremony rehearse: ") {
					t.Fatalf("unsafe rehearsal admitted: exit=%d out=%q error=%q", code, out, errOut)
				}
				assertCommandSecretsRedacted(t, env, out+errOut)
			}
		})
	}
}
