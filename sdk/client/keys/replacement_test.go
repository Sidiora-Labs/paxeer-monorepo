package keys

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/multisig"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/testdata"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const replacementKeyName = "signer"

func newReplacementTestKeyring(t *testing.T, home string) keyring.Keyring {
	t.Helper()
	kb, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, home, nil)
	require.NoError(t, err)
	return kb
}

func createReplacementTestIdentity(t *testing.T, kb keyring.Keyring, name string, index uint32) keyring.Info {
	t.Helper()
	info, err := kb.NewAccount(name, testdata.TestMnemonic, "", hd.CreateHDPath(118, 0, index).String(), hd.Secp256k1)
	require.NoError(t, err)
	return info
}

func replacementTestContext(home string, kb keyring.Keyring, input io.Reader) client.Context {
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	return client.Context{}.
		WithCodec(codec.NewProtoCodec(registry)).
		WithKeyring(kb).
		WithKeyringDir(home).
		WithInput(input)
}

func newReplacementTestCommand(home string, args ...string) *cobra.Command {
	cmd := AddKeyCommand()
	cmd.Flags().AddFlagSet(Commands(home).PersistentFlags())
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(append([]string{replacementKeyName, "--" + flags.FlagKeyringBackend + "=" + keyring.BackendTest}, args...))
	return cmd
}

func executeReplacementTestCommand(cmd *cobra.Command, ctx client.Context) error {
	return cmd.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &ctx))
}

func assertReplacementIdentity(t *testing.T, home string, expected keyring.Info) {
	t.Helper()
	// Reopen the encrypted file backend; no live keyring object is reused.
	reopened := newReplacementTestKeyring(t, home)
	actual, err := reopened.Key(replacementKeyName)
	require.NoError(t, err)
	require.Equal(t, expected.GetName(), actual.GetName())
	require.Equal(t, expected.GetType(), actual.GetType())
	require.Equal(t, expected.GetAlgo(), actual.GetAlgo())
	require.Equal(t, expected.GetPubKey().Bytes(), actual.GetPubKey().Bytes())
	byAddress, err := reopened.KeyByAddress(expected.GetAddress())
	require.NoError(t, err)
	require.Equal(t, replacementKeyName, byAddress.GetName())
	message := []byte("key-overwrite-loss restart recovery")
	signature, publicKey, err := reopened.Sign(replacementKeyName, message)
	require.NoError(t, err)
	require.Equal(t, expected.GetPubKey().Bytes(), publicKey.Bytes())
	require.True(t, expected.GetPubKey().VerifySignature(message, signature))
	signature, publicKey, err = reopened.SignByAddress(expected.GetAddress(), message)
	require.NoError(t, err)
	require.True(t, publicKey.VerifySignature(message, signature))
	entries, err := os.ReadDir(home)
	require.NoError(t, err)
	for _, entry := range entries {
		require.False(t, strings.HasPrefix(entry.Name(), ".keyring-replacement-"), "private staging directory was left behind")
	}
}

func TestKeyReplacementInvalidInputPreservesIdentity(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		input     string
		wantError string
	}{
		{"invalid recovery phrase", []string{"--recover"}, "y\ninvalid mnemonic\n", "invalid mnemonic"},
		{"missing recovery phrase", []string{"--recover"}, "y\n", "EOF"},
		{"invalid interactive phrase", []string{"--interactive"}, "y\ninvalid mnemonic\n", "invalid mnemonic"},
		{"passphrase mismatch", []string{"--interactive"}, "y\n\nfirst passphrase\nsecond passphrase\n", "passphrases don't match"},
		{"invalid HD path", []string{"--recover", "--hd-path=invalid"}, "y\n" + testdata.TestMnemonic + "\n", "doesn't contain '/' separators"},
		{"invalid public key", []string{"--pubkey={"}, "y\n", ""},
		{"unknown public key type", []string{`--pubkey={"@type":"/unknown.PubKey","key":"AA=="}`}, "y\n", ""},
		{"zero multisig threshold", []string{"--multisig=member", "--multisig-threshold=0"}, "y\n", "threshold must be a positive integer"},
		{"excessive multisig threshold", []string{"--multisig=member", "--multisig-threshold=2"}, "y\n", "threshold k of n multisignature: 1 < 2"},
		{"missing multisig member", []string{"--multisig=missing"}, "y\n", "key not found"},
		{"Ledger custom path", []string{"--ledger", "--hd-path=m/44'/118'/0'/0/0"}, "y\n", "cannot set custom bip32 path with ledger"},
		{"unavailable Ledger", []string{"--ledger"}, "y\n", "failed to generate ledger key"},
		{"duplicate signing identity", []string{"--recover", "--index=1", "--coin-type=118"}, "y\n" + testdata.TestMnemonic + "\n", "already exists"},
		{"invalid output format", []string{"--recover", "--index=2", "--coin-type=118", "--" + cli.OutputFlag + "=invalid"}, "y\n" + testdata.TestMnemonic + "\n", "invalid output format"},
		{"overwrite declined", nil, "n\n", "aborted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			kb := newReplacementTestKeyring(t, home)
			original := createReplacementTestIdentity(t, kb, replacementKeyName, 0)
			member := createReplacementTestIdentity(t, kb, "member", 1)
			before, err := keyringDirectorySnapshot(filepath.Join(home, "keyring-test"), "")
			require.NoError(t, err)
			cmd := newReplacementTestCommand(home, tc.args...)
			err = executeReplacementTestCommand(cmd, replacementTestContext(home, kb, strings.NewReader(tc.input)))
			require.Error(t, err)
			if tc.wantError != "" {
				require.Contains(t, err.Error(), tc.wantError)
			}
			after, err := keyringDirectorySnapshot(filepath.Join(home, "keyring-test"), "")
			require.NoError(t, err)
			require.Equal(t, before, after, "an unsuccessful replacement changed the durable keyring")
			kb = nil
			assertReplacementIdentity(t, home, original)
			reopenedMember, err := newReplacementTestKeyring(t, home).Key("member")
			require.NoError(t, err)
			require.Equal(t, member.GetAddress(), reopenedMember.GetAddress())
		})
	}
}

func TestKeyReplacementPersists(t *testing.T) {
	for _, kind := range []string{"local", "same identity", "public key", "multisig with original member"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			kb := newReplacementTestKeyring(t, home)
			original := createReplacementTestIdentity(t, kb, replacementKeyName, 0)
			member := createReplacementTestIdentity(t, kb, "member", 1)
			producer := keyring.NewInMemory()
			expected := createReplacementTestIdentity(t, producer, replacementKeyName, 2)
			ctx := replacementTestContext(home, kb, strings.NewReader("y\n"+testdata.TestMnemonic+"\n"))
			args := []string{"--recover", "--index=2", "--coin-type=118"}
			wantType := keyring.TypeLocal
			switch kind {
			case "same identity":
				args = []string{"--recover", "--index=0", "--coin-type=118"}
				expected = original
			case "public key":
				pubJSON, err := ctx.Codec.MarshalInterfaceJSON(expected.GetPubKey())
				require.NoError(t, err)
				args = []string{"--pubkey=" + string(pubJSON)}
				wantType = keyring.TypeOffline
			case "multisig with original member":
				args = []string{"--multisig=" + replacementKeyName + ",member", "--multisig-threshold=2", "--nosort"}
				pk := multisig.NewLegacyAminoPubKey(2, []cryptotypes.PubKey{original.GetPubKey(), member.GetPubKey()})
				var err error
				expected, err = keyring.NewMultiInfo(replacementKeyName, pk)
				require.NoError(t, err)
				wantType = keyring.TypeMulti
			}
			cmd := newReplacementTestCommand(home, args...)
			var out bytes.Buffer
			cmd.SetOut(&out)
			require.NoError(t, executeReplacementTestCommand(cmd, ctx))
			require.Contains(t, out.String(), "name: "+replacementKeyName)
			kb = nil
			reopened := newReplacementTestKeyring(t, home)
			actual, err := reopened.Key(replacementKeyName)
			require.NoError(t, err)
			require.Equal(t, wantType, actual.GetType())
			require.Equal(t, expected.GetPubKey().Bytes(), actual.GetPubKey().Bytes())
			byAddress, err := reopened.KeyByAddress(actual.GetAddress())
			require.NoError(t, err)
			require.Equal(t, replacementKeyName, byAddress.GetName())
			if !bytes.Equal(original.GetAddress(), actual.GetAddress()) {
				_, err = reopened.KeyByAddress(original.GetAddress())
				require.Error(t, err)
			}
			stillMember, err := reopened.Key("member")
			require.NoError(t, err)
			require.Equal(t, member.GetPubKey().Bytes(), stillMember.GetPubKey().Bytes())
			infos, err := reopened.List()
			require.NoError(t, err)
			require.Len(t, infos, 2)
			if wantType == keyring.TypeLocal {
				assertReplacementIdentity(t, home, expected)
			}
		})
	}
}

func TestKeyReplacementStagedStorageFailure(t *testing.T) {
	home := t.TempDir()
	kb := newReplacementTestKeyring(t, home)
	original := createReplacementTestIdentity(t, kb, replacementKeyName, 0)
	cmd := newReplacementTestCommand(home)
	require.NoError(t, cmd.Flags().Set(flags.FlagKeyringBackend, keyring.BackendTest))
	r, err := stageKeyReplacement(replacementTestContext(home, kb, strings.NewReader("")), cmd, kb, replacementKeyName)
	require.NoError(t, err)
	// An actual directory at the record path makes the real encrypted file
	// backend fail record access after key derivation, without a storage double.
	require.NoError(t, os.Mkdir(filepath.Join(r.stageDir, replacementKeyName+".info"), 0700))
	_, err = r.keyring.NewAccount(replacementKeyName, testdata.TestMnemonic, "", hd.CreateHDPath(118, 0, 2).String(), hd.Secp256k1)
	require.Error(t, err)
	r.close(cmd)
	r = nil
	kb = nil
	assertReplacementIdentity(t, home, original)
}

func TestKeyReplacementCommitFailure(t *testing.T) {
	home := t.TempDir()
	kb := newReplacementTestKeyring(t, home)
	original := createReplacementTestIdentity(t, kb, replacementKeyName, 0)
	cmd := newReplacementTestCommand(home)
	require.NoError(t, cmd.Flags().Set(flags.FlagKeyringBackend, keyring.BackendTest))
	r, err := stageKeyReplacement(replacementTestContext(home, kb, strings.NewReader("")), cmd, kb, replacementKeyName)
	require.NoError(t, err)
	createReplacementTestIdentity(t, r.keyring, replacementKeyName, 2)
	// A missing exchange source is a real filesystem commit failure.
	missing := filepath.Join(r.stageRoot, "missing")
	require.Error(t, exchangeKeyringDirectories(r.liveDir, missing))
	// Also exercise the transaction's pre-commit filesystem error path.
	require.NoError(t, os.Rename(r.stageDir, r.stageDir+".moved"))
	require.Error(t, r.commit())
	r.close(cmd)
	r = nil
	kb = nil
	assertReplacementIdentity(t, home, original)
}

func TestKeyReplacementConcurrentChangePreserved(t *testing.T) {
	home := t.TempDir()
	kb := newReplacementTestKeyring(t, home)
	original := createReplacementTestIdentity(t, kb, replacementKeyName, 0)
	cmd := newReplacementTestCommand(home)
	require.NoError(t, cmd.Flags().Set(flags.FlagKeyringBackend, keyring.BackendTest))
	r, err := stageKeyReplacement(replacementTestContext(home, kb, strings.NewReader("")), cmd, kb, replacementKeyName)
	require.NoError(t, err)
	createReplacementTestIdentity(t, r.keyring, replacementKeyName, 2)
	added := createReplacementTestIdentity(t, kb, "another", 3)
	err = r.commit()
	require.ErrorContains(t, err, "keyring contents changed during replacement")
	r.close(cmd)
	r = nil
	kb = nil
	assertReplacementIdentity(t, home, original)
	actual, err := newReplacementTestKeyring(t, home).Key("another")
	require.NoError(t, err)
	require.Equal(t, added.GetAddress(), actual.GetAddress())
}

func TestKeyReplacementDryRunPreservesIdentity(t *testing.T) {
	home := t.TempDir()
	kb := newReplacementTestKeyring(t, home)
	original := createReplacementTestIdentity(t, kb, replacementKeyName, 0)
	cmd := newReplacementTestCommand(home, "--dry-run", "--recover", "--index=2", "--coin-type=118")
	require.NoError(t, executeReplacementTestCommand(cmd, replacementTestContext(home, kb, strings.NewReader(testdata.TestMnemonic+"\n"))))
	kb = nil
	assertReplacementIdentity(t, home, original)
}

func TestKeyReplacementUnsupportedBackendPreservesIdentity(t *testing.T) {
	kb := keyring.NewInMemory()
	original := createReplacementTestIdentity(t, kb, replacementKeyName, 0)
	ctx := replacementTestContext("", kb, strings.NewReader("y\n"))
	cmd := newReplacementTestCommand("")
	require.NoError(t, cmd.Flags().Set(flags.FlagKeyringBackend, keyring.BackendMemory))
	// The real in-memory keyring has no atomic replacement primitive. Refuse
	// before deletion instead of silently using a delete-and-create fallback.
	_, err := stageKeyReplacement(ctx, cmd, kb, replacementKeyName)
	require.ErrorContains(t, err, "atomic key replacement is not supported")
	message := []byte("unsupported backend retains signing identity")
	sig, pub, err := kb.Sign(replacementKeyName, message)
	require.NoError(t, err)
	require.Equal(t, original.GetPubKey().Bytes(), pub.Bytes())
	require.True(t, pub.VerifySignature(message, sig))
}
