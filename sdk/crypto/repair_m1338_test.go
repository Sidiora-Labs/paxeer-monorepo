package crypto_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/crypto/bcrypt"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/mod/modfile"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

func repairM1338Cost(t *testing.T, cost int) {
	t.Helper()
	previous := crypto.BcryptSecurityParameter
	crypto.BcryptSecurityParameter = cost
	t.Cleanup(func() { crypto.BcryptSecurityParameter = previous })
}

func TestRepairM_1338_SymmetricEmptyPayload(t *testing.T) {
	secret := bytes.Repeat([]byte{0x37}, 32)
	for _, plaintext := range [][]byte{nil, {}, []byte("authenticated payload")} {
		ciphertext := crypto.EncryptSymmetric(plaintext, secret)
		require.Len(t, ciphertext, 24+secretbox.Overhead+len(plaintext))
		recovered, err := crypto.DecryptSymmetric(ciphertext, secret)
		require.NoError(t, err)
		require.True(t, bytes.Equal(plaintext, recovered))
		for i := range ciphertext {
			tampered := bytes.Clone(ciphertext)
			tampered[i] ^= 1
			got, err := crypto.DecryptSymmetric(tampered, secret)
			require.ErrorIs(t, err, crypto.ErrCiphertextDecryptionFailed)
			require.Nil(t, got)
		}
		wrongKey := bytes.Repeat([]byte{0x38}, 32)
		got, err := crypto.DecryptSymmetric(ciphertext, wrongKey)
		require.ErrorIs(t, err, crypto.ErrCiphertextDecryptionFailed)
		require.Nil(t, got)
	}
	empty := crypto.EncryptSymmetric(nil, secret)
	for length := 0; length < len(empty); length++ {
		got, err := crypto.DecryptSymmetric(empty[:length], secret)
		require.Error(t, err, "truncated nonce/tag length %d", length)
		require.False(t, errors.Is(err, crypto.ErrCiphertextDecryptionFailed))
		require.Nil(t, got)
	}
}

// The children execute this already-linked test executable, never a Go toolchain
// or an external helper. The producer exits before the fresh consumer starts.
func TestRepairM_1338_ArmorRecoveryProcess(t *testing.T) {
	for i, arg := range os.Args {
		if arg != "--repair-m1338-process" {
			continue
		}
		require.Len(t, os.Args[i+1:], 2)
		repairM1338RecoveryChild(t, os.Args[i+1], os.Args[i+2])
		return
	}
	root := t.TempDir()
	executable, err := os.Executable()
	require.NoError(t, err)
	for _, phase := range []string{"export", "restore"} {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestRepairM_1338_ArmorRecoveryProcess$", "-test.v", "--", "--repair-m1338-process", phase, root)
		output, err := cmd.CombinedOutput()
		cancel()
		require.NoError(t, err, "%s process: %s", phase, output)
		require.Contains(t, string(output), "--- PASS: TestRepairM_1338_ArmorRecoveryProcess")
	}
	want, err := os.ReadFile(filepath.Join(root, "public"))
	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(root, "restored-public"))
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func repairM1338RecoveryChild(t *testing.T, phase, root string) {
	t.Helper()
	const passphrase = "process-independent export passphrase"
	armorPath := filepath.Join(root, "export.asc")
	if phase == "export" {
		repairM1338Cost(t, 4)
		producer := keyring.NewInMemory()
		info, _, err := producer.NewMnemonic("account", keyring.English, "m/44'/118'/0'/0/0", "", hd.Secp256k1)
		require.NoError(t, err)
		exported, err := producer.ExportPrivKeyArmor("account", passphrase)
		require.NoError(t, err)
		_, headers, _, err := crypto.DecodeArmor(exported)
		require.NoError(t, err)
		require.Equal(t, "1", headers["version"])
		require.Equal(t, "4", headers["cost"])
		require.NoError(t, os.WriteFile(armorPath, []byte(exported), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(root, "public"), info.GetPubKey().Bytes(), 0600))
		return
	}
	require.Equal(t, "restore", phase)
	repairM1338Cost(t, 5)
	exported, err := os.ReadFile(armorPath)
	require.NoError(t, err)
	consumer, err := keyring.New("repair-m1338", keyring.BackendTest, filepath.Join(root, "restored"), nil)
	require.NoError(t, err)
	require.NoError(t, consumer.ImportPrivKey("recovered", string(exported), passphrase))
	consumer = nil
	consumer, err = keyring.New("repair-m1338", keyring.BackendTest, filepath.Join(root, "restored"), nil)
	require.NoError(t, err)
	info, err := consumer.Key("recovered")
	require.NoError(t, err)
	require.Equal(t, hd.Secp256k1Type, info.GetAlgo())
	message := []byte("sign after process exit and file-store reopen")
	signature, public, err := consumer.Sign("recovered", message)
	require.NoError(t, err)
	require.True(t, info.GetPubKey().Equals(public))
	require.True(t, public.VerifySignature(message, signature))
	require.False(t, public.VerifySignature([]byte("changed"), signature))
	reexported, err := consumer.ExportPrivKeyArmor("recovered", passphrase)
	require.NoError(t, err)
	_, headers, _, err := crypto.DecodeArmor(reexported)
	require.NoError(t, err)
	require.Equal(t, "5", headers["cost"])
	require.NoError(t, os.WriteFile(filepath.Join(root, "restored-public"), public.Bytes(), 0600))
}

func TestRepairM_1338_ArmorLegacyAndBounds(t *testing.T) {
	repairM1338Cost(t, 12)
	key := secp256k1.GenPrivKey()
	const passphrase = "legacy production default"
	exported := crypto.EncryptArmorPrivKey(key.Bytes(), passphrase, "")
	block, headers, ciphertext, err := crypto.DecodeArmor(exported)
	require.NoError(t, err)
	// Version/cost are the only additions to the legacy producer's wire format;
	// the salt, KDF and authenticated ciphertext are production outputs.
	delete(headers, "version")
	delete(headers, "cost")
	legacy := crypto.EncodeArmor(block, headers, ciphertext)
	crypto.BcryptSecurityParameter = 4
	recovered, algo, err := crypto.UnarmorDecryptPrivKey(legacy, passphrase)
	require.NoError(t, err)
	require.Equal(t, key.Bytes(), recovered)
	require.Equal(t, "secp256k1", algo)
	_, _, err = crypto.UnarmorDecryptPrivKey(legacy, "wrong")
	require.ErrorIs(t, err, sdkerrors.ErrWrongPassword)
	for _, configured := range []int{1, 4, 5} {
		crypto.BcryptSecurityParameter = configured
		exported := crypto.EncryptArmorPrivKey(key.Bytes(), passphrase, key.Type())
		block, headers, ciphertext, err := crypto.DecodeArmor(exported)
		require.NoError(t, err)
		effective := configured
		if effective < bcrypt.MinCost {
			effective = bcrypt.DefaultCost
		}
		require.Equal(t, strconv.Itoa(effective), headers["cost"])
		crypto.BcryptSecurityParameter = 6
		recovered, _, err = crypto.UnarmorDecryptPrivKey(exported, passphrase)
		require.NoError(t, err)
		require.Equal(t, key.Bytes(), recovered)
		for _, badCost := range []string{"", "-1", "0", "3", "17", "31", "99999999999999999999999", "four"} {
			t.Run(fmt.Sprintf("cost-%d-%s", configured, badCost), func(t *testing.T) {
				invalid := make(map[string]string, len(headers))
				for k, v := range headers {
					invalid[k] = v
				}
				invalid["cost"] = badCost
				got, _, err := crypto.UnarmorDecryptPrivKey(crypto.EncodeArmor(block, invalid, ciphertext), passphrase)
				require.Error(t, err)
				require.False(t, errors.Is(err, sdkerrors.ErrWrongPassword))
				require.Nil(t, got)
			})
		}
	}
	crypto.BcryptSecurityParameter = 4
	exported = crypto.EncryptArmorPrivKey(key.Bytes(), passphrase, key.Type())
	block, headers, ciphertext, err = crypto.DecodeArmor(exported)
	require.NoError(t, err)
	for _, changes := range []map[string]string{
		{"version": "2"}, {"version": ""}, {"cost": ""},
		{"salt": "00"}, {"salt": "zz"}, {"salt": ""}, {"kdf": "unknown"},
	} {
		invalid := make(map[string]string, len(headers))
		for k, v := range headers {
			invalid[k] = v
		}
		for k, v := range changes {
			invalid[k] = v
		}
		got, _, err := crypto.UnarmorDecryptPrivKey(crypto.EncodeArmor(block, invalid, ciphertext), passphrase)
		require.Error(t, err, "headers %v", changes)
		require.False(t, errors.Is(err, sdkerrors.ErrWrongPassword))
		require.Nil(t, got)
	}
	crypto.BcryptSecurityParameter = 17
	require.Panics(t, func() { crypto.EncryptArmorPrivKey(key.Bytes(), passphrase, key.Type()) })
}

func TestRepairM_1338_ArmorPasswordCode(t *testing.T) {
	repairM1338Cost(t, 4)
	producer := keyring.NewInMemory()
	info, _, err := producer.NewMnemonic("source", keyring.English, "m/44'/118'/0'/0/0", "", hd.Secp256k1)
	require.NoError(t, err)
	const password = "actual encrypted key passphrase"
	exported, err := producer.ExportPrivKeyArmor("source", password)
	require.NoError(t, err)
	consumer := keyring.NewInMemory()
	err = consumer.ImportPrivKey("restored", exported, "wrong password")
	require.ErrorIs(t, err, sdkerrors.ErrWrongPassword)
	require.True(t, sdkerrors.ErrWrongPassword.Is(err))
	wrapped := fmt.Errorf("caller context: %w", err)
	require.ErrorIs(t, wrapped, sdkerrors.ErrWrongPassword)
	entries, err := consumer.List()
	require.NoError(t, err)
	require.Empty(t, entries)
	block, headers, ciphertext, err := crypto.DecodeArmor(exported)
	require.NoError(t, err)
	tampered := bytes.Clone(ciphertext)
	tampered[len(tampered)-1] ^= 1
	err = consumer.ImportPrivKey("restored", crypto.EncodeArmor(block, headers, tampered), password)
	require.ErrorIs(t, err, sdkerrors.ErrWrongPassword)
	err = consumer.ImportPrivKey("restored", crypto.EncodeArmor(block, headers, ciphertext[:39]), password)
	require.Error(t, err)
	require.False(t, errors.Is(err, sdkerrors.ErrWrongPassword))
	err = consumer.ImportPrivKey("restored", "not armor", password)
	require.Error(t, err)
	require.False(t, errors.Is(err, sdkerrors.ErrWrongPassword))
	require.NoError(t, consumer.ImportPrivKey("restored", exported, password))
	restored, err := consumer.Key("restored")
	require.NoError(t, err)
	require.True(t, info.GetPubKey().Equals(restored.GetPubKey()))
	message := []byte("imported key signs")
	signature, public, err := consumer.Sign("restored", message)
	require.NoError(t, err)
	require.True(t, public.VerifySignature(message, signature))
}

func TestRepairM_1338_XCryptoVersionAgreement(t *testing.T) {
	manifest, err := os.ReadFile("../../go.mod")
	require.NoError(t, err)
	module, err := modfile.Parse("go.mod", manifest, nil)
	require.NoError(t, err)
	found := false
	for _, dependency := range module.Require {
		if dependency.Mod.Path == "golang.org/x/crypto" {
			found = true
			require.Equal(t, requiredXCryptoVersion, dependency.Mod.Version)
		}
	}
	require.True(t, found, "x/crypto must be an explicit requirement")
	for _, replacement := range module.Replace {
		require.NotEqual(t, "golang.org/x/crypto", replacement.Old.Path, "x/crypto must not have a replacement")
	}
	// Go 1.25 test binaries omit dependency build-info records. The linked
	// function's source origin still identifies the actual selected module.
	linked := runtime.FuncForPC(reflect.ValueOf(secretbox.Open).Pointer())
	require.NotNil(t, linked)
	file, _ := linked.FileLine(linked.Entry())
	require.Contains(t, filepath.ToSlash(file), "golang.org/x/crypto@"+requiredXCryptoVersion+"/nacl/secretbox/")
	TestArmorPrivKeyRoundTripOnRequiredXCrypto(t)
}

func TestRepairM_1338_BcryptBackupRisk(t *testing.T) {
	repairM1338Cost(t, 12)
	root := t.TempDir()
	const password = "long distinct directory backup passphrase"
	producer, err := keyring.New("repair-m1338-backup", keyring.BackendFile, root, strings.NewReader(password+"\n"+password+"\n"))
	require.NoError(t, err)
	info, _, err := producer.NewMnemonic("source", keyring.English, "m/44'/118'/0'/0/0", "", hd.Secp256k1)
	require.NoError(t, err)
	exported, err := producer.ExportPrivKeyArmor("source", password)
	require.NoError(t, err)
	_, headers, _, err := crypto.DecodeArmor(exported)
	require.NoError(t, err)
	require.Equal(t, "12", headers["cost"])
	backup := t.TempDir()
	require.NoError(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(backup, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0700)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0600)
	}))
	verifier, err := os.ReadFile(filepath.Join(backup, "keyring-file", "keyhash"))
	require.NoError(t, err)
	cost, err := bcrypt.Cost(verifier)
	require.NoError(t, err)
	require.Equal(t, 10, cost, "legacy keyhash argument 2 normalizes to cost 10")
	require.NoError(t, bcrypt.CompareHashAndPassword(verifier, []byte(password)))
	require.Error(t, bcrypt.CompareHashAndPassword(verifier, []byte("wrong guess")))
	// Restore solely from the copied directory through the real prompt/backend.
	producer = nil
	restored, err := keyring.New("repair-m1338-backup", keyring.BackendFile, backup, strings.NewReader(password+"\n"))
	require.NoError(t, err)
	recovered, err := restored.Key("source")
	require.NoError(t, err)
	require.True(t, info.GetPubKey().Equals(recovered.GetPubKey()))
	message := []byte("backup independently restores the account")
	signature, public, err := restored.Sign("source", message)
	require.NoError(t, err)
	require.True(t, public.VerifySignature(message, signature))
	document, err := os.ReadFile("bcrypt_readme.md")
	require.NoError(t, err)
	for _, boundary := range []string{"cost 10", "offline", "long, difficult-to-guess passphrases", "older stolen backup"} {
		require.Contains(t, string(document), boundary)
	}
}
