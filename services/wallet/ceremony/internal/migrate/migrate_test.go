package migrate

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPlanMigrationExcludesFundedAndMigrated(t *testing.T)       { m160TestPlan(t) }
func TestPlanMigrationRequiresMigratedColumn(t *testing.T)          { m160TestSchema(t) }
func TestDeliverMigratesEveryEligibleWallet(t *testing.T)           { m160TestCompleteDelivery(t) }
func TestDeliverStopsTheRunOnSignatureMismatch(t *testing.T)        { m160TestSignatureMismatch(t) }
func TestDeliverRefusesEnvelopeAddressMismatch(t *testing.T)        { m160TestEnvelopeMismatch(t) }
func TestDeliverRefusesReplacementForExternalIdentity(t *testing.T) { m160TestExternalIdentity(t) }

func TestLoadOptionsRequiresProtectedSealingKey(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(root, "journal-key")
	key := bytes.Repeat([]byte{0x36}, 32)
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"CEREMONY_ID": "retained-original-identity", "CEREMONY_JOURNAL_DIR": root, "CEREMONY_JOURNAL_KEY_FILE": keyPath, "CEREMONY_REHEARSAL_RECEIPT": filepath.Join(root, "counts.sealed"), "CEREMONY_RPC_URL": "http://127.0.0.1:8545"}
	get := func(k string) string { return values[k] }
	options, err := LoadOptions(get)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(options.JournalKey, key) || options.CeremonyID != values["CEREMONY_ID"] || options.Rehearsal {
		t.Fatal("loaded ceremony authority differs")
	}
	for i := range options.JournalKey {
		options.JournalKey[i] = 0
	}
	if err = os.Chmod(keyPath, 0640); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadOptions(get); !errors.Is(err, ErrJournal) {
		t.Fatalf("readable key accepted: %v", err)
	}
	if err = os.Chmod(keyPath, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "key-link")
	if err = os.Symlink(keyPath, link); err != nil {
		t.Fatal(err)
	}
	values["CEREMONY_JOURNAL_KEY_FILE"] = link
	if _, err = LoadOptions(get); !errors.Is(err, ErrJournal) {
		t.Fatalf("symlink key accepted: %v", err)
	}
	values["CEREMONY_JOURNAL_KEY_FILE"] = keyPath
	values["CEREMONY_ID"] = "../other"
	if _, err = LoadOptions(get); !errors.Is(err, ErrJournal) {
		t.Fatalf("path-changing ceremony accepted: %v", err)
	}
	values["CEREMONY_ID"] = "retained-original-identity"
	values["CEREMONY_REHEARSAL_RECEIPT"] = "relative"
	if _, err = LoadOptions(get); !errors.Is(err, ErrJournal) {
		t.Fatalf("relative receipt accepted: %v", err)
	}
}

func TestLoadOptionsRejectsMissingOrTruncatedJournalKey(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(root, "key")
	values := map[string]string{"CEREMONY_ID": "retained", "CEREMONY_JOURNAL_DIR": root, "CEREMONY_JOURNAL_KEY_FILE": keyPath, "CEREMONY_REHEARSAL_RECEIPT": filepath.Join(root, "receipt")}
	get := func(k string) string { return values[k] }
	if _, err := LoadOptions(get); !errors.Is(err, ErrJournal) {
		t.Fatalf("missing key accepted: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte{1, 2, 3}, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOptions(get); !errors.Is(err, ErrJournal) {
		t.Fatalf("truncated key accepted: %v", err)
	}
}
