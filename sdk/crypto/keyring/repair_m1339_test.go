package keyring

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	store "github.com/99designs/keyring"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/crypto/bcrypt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/sr25519"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

const repairM1339Password = "repair-backup-password"
const repairM1339Mnemonic = "alcohol woman abuse must during monitor noble actual mixed trade anger aisle"

func repairM1339File(t *testing.T, dir string) Keyring {
	t.Helper()
	kr, err := New("repair-m1339", BackendFile, dir, strings.NewReader(strings.Repeat(repairM1339Password+"\n", 3)))
	require.NoError(t, err)
	return kr
}

func TestRepairM_1339_DocAPI(t *testing.T) {
	memory := NewInMemory()
	software, hardware := memory.SupportedAlgorithms()
	require.Equal(t, "sr25519,secp256k1", software.String())
	require.Equal(t, "secp256k1", hardware.String())
	_, _, err := memory.NewMnemonic("transient", English, sdk.FullFundraiserPath, "", hd.Secp256k1)
	require.NoError(t, err)
	keys, err := NewInMemory().List()
	require.NoError(t, err)
	require.Empty(t, keys)
	dir := t.TempDir()
	created, _, err := repairM1339File(t, dir).NewMnemonic("durable", English, sdk.FullFundraiserPath, "", hd.Secp256k1)
	require.NoError(t, err)
	recovered, err := repairM1339File(t, dir).Key("durable")
	require.NoError(t, err)
	require.True(t, created.GetPubKey().Equals(recovered.GetPubKey()))
	doc, err := os.ReadFile("doc.go")
	require.NoError(t, err)
	require.Contains(t, string(doc), "Package keyring")
	for _, claim := range []string{"Keyring", "NewInMemory", "secp256k1", "sr25519", "insecurely", "cost 12", "power failure", "one active keyring instance"} {
		require.Contains(t, string(doc), claim)
	}
	for _, obsolete := range []string{"NewLegacy", "NewKeyring", "Keybase"} {
		require.NotContains(t, string(doc), obsolete)
	}
}

func TestRepairM_1339_ImportAlgorithm(t *testing.T) {
	for _, algo := range []SignatureAlgo{hd.Secp256k1, hd.Sr25519} {
		t.Run(string(algo.Name()), func(t *testing.T) {
			sourceDir, targetDir := t.TempDir(), t.TempDir()
			source := repairM1339File(t, sourceDir)
			original, err := source.NewAccount("source", repairM1339Mnemonic, "", sdk.FullFundraiserPath, algo)
			require.NoError(t, err)
			armor, err := source.ExportPrivKeyArmor("source", repairM1339Password)
			require.NoError(t, err)
			_, headers, _, err := crypto.DecodeArmor(armor)
			require.NoError(t, err)
			require.Equal(t, string(algo.Name()), headers["type"])
			require.NoError(t, repairM1339File(t, targetDir).ImportPrivKey("imported", armor, repairM1339Password))
			reopened := repairM1339File(t, targetDir)
			info, err := reopened.Key("imported")
			require.NoError(t, err)
			require.Equal(t, algo.Name(), info.GetAlgo())
			require.Equal(t, string(info.GetAlgo()), info.GetPubKey().Type())
			require.True(t, original.GetPubKey().Equals(info.GetPubKey()))
			msg := []byte("export/import/reopen cryptographic identity")
			sig, pub, err := reopened.SignByAddress(info.GetAddress(), msg)
			require.NoError(t, err)
			require.True(t, pub.Equals(original.GetPubKey()))
			require.True(t, original.GetPubKey().VerifySignature(msg, sig))
			require.False(t, original.GetPubKey().VerifySignature([]byte("changed"), sig))
			reexport, err := reopened.ExportPrivKeyArmor("imported", repairM1339Password)
			require.NoError(t, err)
			raw, label, err := crypto.UnarmorDecryptPrivKey(reexport, repairM1339Password)
			require.NoError(t, err)
			originalRaw, originalLabel, err := crypto.UnarmorDecryptPrivKey(armor, repairM1339Password)
			require.NoError(t, err)
			require.Equal(t, originalLabel, label)
			require.Equal(t, originalRaw, raw)

			disabled := NewInMemory(func(o *Options) { o.SupportedAlgos = nil })
			require.ErrorIs(t, disabled.ImportPrivKey("disabled", armor, repairM1339Password), ErrUnsupportedSigningAlgo)
			keys, err := disabled.List()
			require.NoError(t, err)
			require.Empty(t, keys)
		})
	}
	for _, label := range []string{"unknown-algorithm", string(hd.Ed25519Type)} {
		t.Run(label, func(t *testing.T) {
			// Encrypt through the real armor producer; a 32-byte body previously fell
			// through to secp256k1 irrespective of its label.
			armor := crypto.EncryptArmorPrivKey(secp256k1.GenPrivKey().Bytes(), repairM1339Password, label)
			dir := t.TempDir()
			kr := repairM1339File(t, dir)
			require.ErrorIs(t, kr.ImportPrivKey("rejected", armor, repairM1339Password), ErrUnsupportedSigningAlgo)
			keys, err := repairM1339File(t, dir).List()
			require.NoError(t, err)
			require.Empty(t, keys)
		})
	}
	// A syntactically valid sr25519 JSON null must not panic in PubKey.
	malformed := crypto.EncryptArmorPrivKey([]byte("null"), repairM1339Password, string(hd.Sr25519Type))
	require.ErrorContains(t, NewInMemory().ImportPrivKey("empty", malformed, repairM1339Password), "private key size")
}

func TestRepairM_1339_OfflineAlgorithm(t *testing.T) {
	for _, priv := range []cryptotypes.PrivKey{secp256k1.GenPrivKey(), sr25519.GenPrivKey(), ed25519.GenPrivKey()} {
		t.Run(priv.Type(), func(t *testing.T) {
			pub := priv.PubKey()
			actual := hd.PubKeyType(pub.Type())
			for _, label := range []hd.PubKeyType{"", actual} {
				t.Run("label="+string(label), func(t *testing.T) {
					dir := t.TempDir()
					kr := repairM1339File(t, dir)
					wrong := hd.Secp256k1Type
					if actual == wrong {
						wrong = hd.Sr25519Type
					}
					info, err := kr.SavePubKey("watch", pub, wrong)
					require.ErrorIs(t, err, ErrUnsupportedSigningAlgo)
					require.Nil(t, info)
					keys, err := kr.List()
					require.NoError(t, err)
					require.Empty(t, keys)
					created, err := kr.SavePubKey("watch", pub, label)
					require.NoError(t, err)
					require.Equal(t, actual, created.GetAlgo())
					reopened := repairM1339File(t, dir)
					recovered, err := reopened.KeyByAddress(created.GetAddress())
					require.NoError(t, err)
					require.Equal(t, actual, recovered.GetAlgo())
					require.Equal(t, TypeOffline, recovered.GetType())
					require.True(t, pub.Equals(recovered.GetPubKey()))
					outputs, err := MkAccKeysOutput(reopened.List())
					require.NoError(t, err)
					require.Len(t, outputs, 1)
					require.Equal(t, "watch", outputs[0].Name)
					require.Equal(t, sdk.AccAddress(pub.Address()).String(), outputs[0].Address)
					require.Contains(t, outputs[0].PubKey, pub.Type())
					sig, signingPub, err := reopened.Sign("watch", []byte("offline"))
					require.ErrorContains(t, err, "cannot sign with offline keys")
					require.Nil(t, sig)
					require.True(t, pub.Equals(signingPub))
				})
			}
		})
	}
	for _, pub := range []cryptotypes.PubKey{nil, (*ed25519.PubKey)(nil), &ed25519.PubKey{Key: []byte{1}}} {
		_, err := NewInMemory().SavePubKey("invalid", pub, "")
		require.Error(t, err)
	}
}

func TestRepairM_1339_EmptyAddressIndex(t *testing.T) {
	dir := t.TempDir()
	var created Info
	func() {
		kr := repairM1339File(t, dir)
		var err error
		created, _, err = kr.NewMnemonic("retained", English, sdk.FullFundraiserPath, "", hd.Secp256k1)
		require.NoError(t, err)
		ks := kr.(keystore)
		require.NoError(t, ks.db.Set(store.Item{Key: addrHexKeyAsString(created.GetAddress())}))
	}()
	reopened := repairM1339File(t, dir)
	info, err := reopened.KeyByAddress(created.GetAddress())
	require.ErrorIs(t, err, sdkerrors.ErrKeyNotFound)
	require.Nil(t, info)
	sig, pub, err := reopened.SignByAddress(created.GetAddress(), []byte("must fail safely"))
	require.ErrorIs(t, err, sdkerrors.ErrKeyNotFound)
	require.Nil(t, sig)
	require.Nil(t, pub)
	require.ErrorIs(t, reopened.DeleteByAddress(created.GetAddress()), sdkerrors.ErrKeyNotFound)
	retained, err := reopened.Key("retained")
	require.NoError(t, err)
	require.True(t, created.GetPubKey().Equals(retained.GetPubKey()))
}

func TestRepairM_1339_ListInvalidInfo(t *testing.T) {
	dir := t.TempDir()
	var healthy Info
	func() {
		kr := repairM1339File(t, dir)
		var err error
		healthy, _, err = kr.NewMnemonic("healthy", English, sdk.FullFundraiserPath, "", hd.Secp256k1)
		require.NoError(t, err)
		ks := kr.(keystore)
		// Persist damage via the actual encrypted file backend, then abandon it.
		for name, data := range map[string][]byte{
			"empty":          nil,
			"undecodable":    []byte("damaged amino record"),
			"nil-public-key": marshalInfo(newOfflineInfo("nil-public-key", nil, hd.Ed25519Type)),
		} {
			require.NoError(t, ks.db.Set(store.Item{Key: infoKey(name), Data: data}))
		}
	}()
	reopened := repairM1339File(t, dir)
	records, err := reopened.List()
	require.ErrorIs(t, err, ErrCorruptKey)
	var corrupt *CorruptionError
	require.ErrorAs(t, err, &corrupt)
	require.Len(t, corrupt.Entries, 3)
	for i, expected := range []string{"empty.info", "nil-public-key.info", "undecodable.info"} {
		require.Equal(t, expected, corrupt.Entries[i].Entry)
		require.ErrorIs(t, corrupt.Entries[i], ErrCorruptKey)
	}
	require.Len(t, records, 1)
	require.Equal(t, "healthy", records[0].GetName())
	require.True(t, healthy.GetPubKey().Equals(records[0].GetPubKey()))
	output, outputErr := MkAccKeysOutput(records, err)
	require.Nil(t, output)
	require.Same(t, err, outputErr)
	output, err = MkAccKeysOutput(reopened.List())
	require.Nil(t, output)
	require.ErrorAs(t, err, &corrupt)
	// Recovery may explicitly display just the healthy records.
	output, err = MkAccKeysOutput(records)
	require.NoError(t, err)
	require.Len(t, output, 1)
	require.Equal(t, healthy.GetAddress().String(), output[0].Address)
	for _, makeOutput := range []func(Info) (KeyOutput, error){MkAccKeyOutput, MkValKeyOutput, MkConsKeyOutput} {
		_, err := makeOutput(newOfflineInfo("invalid", nil, hd.Ed25519Type))
		require.ErrorIs(t, err, ErrCorruptKey)
		var entry *EntryError
		require.ErrorAs(t, err, &entry)
		require.Equal(t, "invalid.info", entry.Entry)
	}
	ks := reopened.(keystore)
	for _, name := range []string{"empty", "nil-public-key", "undecodable"} {
		_, err := ks.db.Get(infoKey(name))
		require.NoError(t, err, "corruption must not delete recovery material")
	}
}

func TestRepairM_1339_PassphraseMigration(t *testing.T) {
	dir := t.TempDir()
	created, _, err := repairM1339File(t, dir).NewMnemonic("backup", English, sdk.FullFundraiserPath, "", hd.Secp256k1)
	require.NoError(t, err)
	path := filepath.Join(dir, keyringFileDirName, "keyhash")
	modern, err := os.ReadFile(path)
	require.NoError(t, err)
	hash, cost, legacy, err := decodeKeyhash(modern)
	require.NoError(t, err)
	require.False(t, legacy)
	require.Equal(t, 12, cost)
	require.NoError(t, bcrypt.CompareHashAndPassword(hash, []byte(repairM1339Password)))
	// Reproduce the old writer (argument 2 normalized by bcrypt to cost 10).
	old, err := bcrypt.GenerateFromPassword(bytes.Repeat([]byte{9}, 16), []byte(repairM1339Password), 2)
	require.NoError(t, err)
	oldCost, err := bcrypt.Cost(old)
	require.NoError(t, err)
	require.Equal(t, 10, oldCost)
	require.NoError(t, os.WriteFile(path, old, 0600))
	wrong, err := New("repair-m1339", BackendFile, dir, strings.NewReader("wrong-password\nwrong-password\nwrong-password\n"))
	require.NoError(t, err)
	_, err = wrong.Key("backup")
	require.ErrorContains(t, err, "too many failed passphrase attempts")
	require.NotContains(t, err.Error(), "wrong-password")
	unchanged, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, old, unchanged)
	recovered, err := repairM1339File(t, dir).Key("backup")
	require.NoError(t, err)
	require.True(t, created.GetPubKey().Equals(recovered.GetPubKey()))
	migrated, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotEqual(t, old, migrated)
	hash, cost, legacy, err = decodeKeyhash(migrated)
	require.NoError(t, err)
	require.False(t, legacy)
	require.Equal(t, 12, cost)
	require.NoError(t, bcrypt.CompareHashAndPassword(hash, []byte(repairM1339Password)))
	require.NotContains(t, string(migrated), repairM1339Password)
	stat, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), stat.Mode().Perm())
	sig, pub, err := repairM1339File(t, dir).Sign("backup", []byte("old backup still unlocks"))
	require.NoError(t, err)
	require.True(t, created.GetPubKey().Equals(pub))
	require.True(t, pub.VerifySignature([]byte("old backup still unlocks"), sig))
	stable, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, migrated, stable)
	// Reject excessive work before asking for or verifying a password.
	oversized := bytes.Replace(old, []byte("$10$"), []byte("$31$"), 1)
	require.NoError(t, os.WriteFile(path, oversized, 0600))
	_, err = newRealPrompt(filepath.Dir(path), strings.NewReader(""))("")
	require.ErrorContains(t, err, "invalid keyring verifier cost")
	require.NoError(t, os.WriteFile(path, []byte(`{"version":2,"kdf":"bcrypt"}`), 0600))
	_, err = newRealPrompt(filepath.Dir(path), strings.NewReader(""))("")
	require.ErrorContains(t, err, "unsupported keyring verifier format")
}

func TestRepairM_1339_PartialPersistence(t *testing.T) {
	source := NewInMemory()
	original, err := source.NewAccount("account", repairM1339Mnemonic, "", sdk.FullFundraiserPath, hd.Secp256k1)
	require.NoError(t, err)
	dir := t.TempDir()
	kr := repairM1339File(t, dir)
	indexPath := filepath.Join(dir, keyringFileDirName, addrHexKeyAsString(original.GetAddress()))
	// A dangling filesystem link is absent to Get but fails the address Set
	// after the name and encrypted journal have been durably written.
	require.NoError(t, os.Symlink(filepath.Join(dir, "missing-parent", "index"), indexPath))
	info, err := kr.NewAccount("account", repairM1339Mnemonic, "", sdk.FullFundraiserPath, hd.Secp256k1)
	require.Nil(t, info)
	var pending *PendingOperationError
	require.ErrorAs(t, err, &pending)
	require.Equal(t, "create", pending.Operation)
	require.Equal(t, "account.info", pending.NameKey)
	ks := kr.(keystore)
	nameRecord, err := ks.db.Get(infoKey("account"))
	require.NoError(t, err)
	journal, err := ks.db.Get(operationJournalKey)
	require.NoError(t, err)
	var recorded operationJournal
	require.NoError(t, json.Unmarshal(journal.Data, &recorded))
	require.Equal(t, nameRecord.Data, recorded.Data)
	require.Equal(t, "create", recorded.Operation)
	kr, ks = nil, keystore{} // drop the instance: recovery reads only persisted state
	blocked, err := New("repair-m1339", BackendFile, dir, strings.NewReader(repairM1339Password+"\n"))
	require.Nil(t, blocked)
	require.ErrorAs(t, err, &pending)
	require.NoError(t, os.Remove(indexPath))
	reopened := repairM1339File(t, dir)
	recovered, err := reopened.KeyByAddress(original.GetAddress())
	require.NoError(t, err)
	require.Equal(t, "account", recovered.GetName())
	require.True(t, original.GetPubKey().Equals(recovered.GetPubKey()))
	// An identical request succeeds without replacing even the encoded key record.
	retried, err := reopened.NewAccount("account", repairM1339Mnemonic, "", sdk.FullFundraiserPath, hd.Secp256k1)
	require.NoError(t, err)
	require.True(t, original.GetPubKey().Equals(retried.GetPubKey()))
	persisted, err := reopened.(keystore).db.Get(infoKey("account"))
	require.NoError(t, err)
	require.Equal(t, nameRecord.Data, persisted.Data)
	sig, pub, err := reopened.SignByAddress(original.GetAddress(), []byte("recovery"))
	require.NoError(t, err)
	require.True(t, original.GetPubKey().Equals(pub))
	require.True(t, pub.VerifySignature([]byte("recovery"), sig))
	_, err = reopened.(keystore).db.Get(operationJournalKey)
	require.ErrorIs(t, err, store.ErrKeyNotFound)
	// A different account cannot overwrite the recovered name.
	_, _, err = reopened.NewMnemonic("account", English, sdk.FullFundraiserPath, "", hd.Secp256k1)
	require.Error(t, err)
	retained, err := reopened.Key("account")
	require.NoError(t, err)
	require.True(t, original.GetPubKey().Equals(retained.GetPubKey()))

	// A real Remove failure must retain the delete intent for reopen and retry.
	require.NoError(t, os.Remove(indexPath))
	require.NoError(t, os.Mkdir(indexPath, 0700))
	blocker := filepath.Join(indexPath, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("storage obstruction"), 0600))
	err = reopened.Delete("account")
	require.ErrorAs(t, err, &pending)
	require.Equal(t, "delete", pending.Operation)
	deleteJournal, err := reopened.(keystore).db.Get(operationJournalKey)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(deleteJournal.Data, &recorded))
	require.Equal(t, nameRecord.Data, recorded.Data)
	reopened = nil
	require.NoError(t, os.Remove(blocker))
	require.NoError(t, os.Remove(indexPath))
	deleted := repairM1339File(t, dir)
	_, err = deleted.Key("account")
	require.ErrorIs(t, err, sdkerrors.ErrKeyNotFound)
	_, err = deleted.KeyByAddress(original.GetAddress())
	require.ErrorIs(t, err, sdkerrors.ErrKeyNotFound)
	keys, err := deleted.List()
	require.NoError(t, err)
	require.Empty(t, keys)
	// Reusing the same account after recovered deletion succeeds normally.
	restored, err := deleted.NewAccount("account", repairM1339Mnemonic, "", sdk.FullFundraiserPath, hd.Secp256k1)
	require.NoError(t, err)
	require.True(t, original.GetPubKey().Equals(restored.GetPubKey()))
	require.NoError(t, deleted.DeleteByAddress(restored.GetAddress()))
	require.Empty(t, mustRepairM1339List(t, repairM1339File(t, dir)))
}

func mustRepairM1339List(t *testing.T, kr Keyring) []Info {
	t.Helper()
	records, err := kr.List()
	require.NoError(t, err)
	return records
}

func TestRepairM_1339_LedgerErrors(t *testing.T) {
	dir := t.TempDir()
	kr := repairM1339File(t, dir)
	_, hardware := kr.SupportedAlgorithms()
	require.Equal(t, "secp256k1", hardware.String())
	info, err := kr.SaveLedgerKey("unsupported", hd.Sr25519, "pax", 118, 0, 0)
	require.ErrorIs(t, err, ErrUnsupportedSigningAlgo)
	require.Nil(t, info)
	require.NotContains(t, err.Error(), "failed to generate ledger key")
	for _, algo := range hardware {
		info, err := kr.SaveLedgerKey("device", algo, "pax", 118, 0, 0)
		require.Nil(t, info)
		require.ErrorContains(t, err, "failed to generate ledger key: failed to retrieve device: ledger nano S: support for ledger devices is not available in this executable")
		require.NotNil(t, errors.Unwrap(err), "the unavailable producer error must remain wrapped")
		cause := err
		for errors.Unwrap(cause) != nil {
			cause = errors.Unwrap(cause)
		}
		require.EqualError(t, cause, "support for ledger devices is not available in this executable")
	}
	require.Empty(t, mustRepairM1339List(t, repairM1339File(t, dir)))
	// Persist the production Ledger reference format and prove the reopened sign
	// consumer reports unavailable, rather than treating it as a successful sign.
	pub := secp256k1.GenPrivKey().PubKey()
	saved, err := kr.(keystore).writeLedgerKey("cached-device", pub, *hd.NewFundraiserParams(0, 118, 0), hd.Secp256k1Type)
	require.NoError(t, err)
	reopened := repairM1339File(t, dir)
	recovered, err := reopened.Key("cached-device")
	require.NoError(t, err)
	require.True(t, saved.GetPubKey().Equals(recovered.GetPubKey()))
	require.Equal(t, hd.Secp256k1Type, recovered.GetAlgo())
	sig, signingPub, err := reopened.SignByAddress(recovered.GetAddress(), []byte("requires hardware"))
	require.ErrorContains(t, err, "ledger signing failed: failed to connect to ledger: ledger nano S: support for ledger devices is not available in this executable")
	require.NotNil(t, errors.Unwrap(err))
	require.Nil(t, sig)
	require.Nil(t, signingPub)
}
