package genutil

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

const validatorIdentityMnemonic = "side video kiss hotel essence door angle student degree during vague adjust submit trick globe muscle frozen vacuum artwork million shield bind useful wave"
const validatorIdentityChainID = "repair-300"

func TestValidatorIdentityFreshMnemonicRestart(t *testing.T) {
	cfg := validatorIdentityConfig(t)
	nodeID, returned, err := InitializeNodeValidatorFilesFromMnemonic(cfg, validatorIdentityMnemonic)
	require.NoError(t, err)
	require.NotEmpty(t, nodeID)
	require.Equal(t, ed25519.TestSecretKey([]byte(validatorIdentityMnemonic)).Public().Bytes(), returned.Bytes())
	for _, path := range []string{cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile()} {
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}

	reopened, err := privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
	require.NoError(t, err)
	pub, err := reopened.GetPubKey(t.Context())
	require.NoError(t, err)
	require.Equal(t, returned.Bytes(), pub.Bytes())
	require.Zero(t, reopened.LastSignState.Height)
	vote := validatorIdentityVote(reopened, 12, 1)
	require.NoError(t, reopened.SignVote(t.Context(), validatorIdentityChainID, vote))
	validatorIdentityVerify(t, reopened, vote)
	reopened = nil

	restartedCfg := config.TestConfig().SetRoot(cfg.RootDir)
	restartedNodeID, restartedPub, err := InitializeNodeValidatorFiles(restartedCfg)
	require.NoError(t, err)
	require.Equal(t, nodeID, restartedNodeID)
	require.Equal(t, returned.Bytes(), restartedPub.Bytes())
	reopened, err = privval.LoadFilePV(restartedCfg.PrivValidator.KeyFile(), restartedCfg.PrivValidator.StateFile())
	require.NoError(t, err)
	require.EqualValues(t, 12, reopened.LastSignState.Height)
	require.Equal(t, vote.Signature, reopened.LastSignState.Signature)
}

func TestValidatorIdentityExistingKeyRestart(t *testing.T) {
	cfg := validatorIdentityConfig(t)
	pv := validatorIdentityUsedValidator(t, cfg)
	original := validatorIdentityVote(pv, 12, 1)
	require.NoError(t, pv.SignVote(t.Context(), validatorIdentityChainID, original))
	before := validatorIdentityFiles(t, cfg)
	pv = nil

	_, returned, err := InitializeNodeValidatorFilesFromMnemonic(cfg, validatorIdentityMnemonic)
	require.NoError(t, err)
	require.Equal(t, before, validatorIdentityFiles(t, cfg))
	reopened, err := privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
	require.NoError(t, err)
	require.Equal(t, returned.Bytes(), reopened.Key.PubKey.Bytes())
	require.EqualValues(t, 12, reopened.LastSignState.Height)
	require.EqualValues(t, 2, reopened.LastSignState.Round)
	require.EqualValues(t, 2, reopened.LastSignState.Step)
	require.Equal(t, original.Signature, reopened.LastSignState.Signature)
	require.Equal(t, tmtypes.VoteSignBytes(validatorIdentityChainID, original), []byte(reopened.LastSignState.SignBytes))

	retry := validatorIdentityVote(reopened, 12, 1)
	require.NoError(t, reopened.SignVote(t.Context(), validatorIdentityChainID, retry))
	require.Equal(t, original.Signature, retry.Signature)
	validatorIdentityVerify(t, reopened, retry)
	for _, vote := range []*tmproto.Vote{validatorIdentityVote(reopened, 12, 2), validatorIdentityVote(reopened, 11, 1)} {
		require.Error(t, reopened.SignVote(t.Context(), validatorIdentityChainID, vote))
		require.Empty(t, vote.Signature)
	}
	require.Equal(t, before, validatorIdentityFiles(t, cfg))
	next := validatorIdentityVote(reopened, 13, 1)
	require.NoError(t, reopened.SignVote(t.Context(), validatorIdentityChainID, next))
	validatorIdentityVerify(t, reopened, next)
}

func TestValidatorIdentityMismatchedKey(t *testing.T) {
	cfg := validatorIdentityConfig(t)
	pv := validatorIdentityUsedValidator(t, cfg)
	before := validatorIdentityFiles(t, cfg)
	expected := pv.Key.PubKey.Bytes()
	pv = nil

	nodeID, pub, err := InitializeNodeValidatorFilesFromMnemonic(cfg, "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about")
	require.ErrorContains(t, err, "does not match")
	require.Empty(t, nodeID)
	require.Nil(t, pub)
	require.Equal(t, before, validatorIdentityFiles(t, cfg))
	reopened, err := privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
	require.NoError(t, err)
	require.Equal(t, expected, reopened.Key.PubKey.Bytes())
	require.EqualValues(t, 12, reopened.LastSignState.Height)
	conflict := validatorIdentityVote(reopened, 12, 2)
	require.Error(t, reopened.SignVote(t.Context(), validatorIdentityChainID, conflict))
	require.Empty(t, conflict.Signature)
}

func TestValidatorIdentityIncompleteFiles(t *testing.T) {
	for _, missing := range []string{"key", "state"} {
		t.Run(missing, func(t *testing.T) {
			cfg := validatorIdentityConfig(t)
			validatorIdentityUsedValidator(t, cfg)
			removed, retained := cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile()
			if missing == "state" {
				removed, retained = retained, removed
			}
			require.NoError(t, os.Remove(removed))
			before, err := os.ReadFile(retained)
			require.NoError(t, err)
			nodeID, pub, err := InitializeNodeValidatorFilesFromMnemonic(cfg, validatorIdentityMnemonic)
			require.ErrorContains(t, err, "incomplete validator files")
			require.Empty(t, nodeID)
			require.Nil(t, pub)
			_, err = os.Lstat(removed)
			require.True(t, os.IsNotExist(err))
			after, err := os.ReadFile(retained)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}

func TestValidatorIdentityCorruptFiles(t *testing.T) {
	for _, corrupt := range []string{"key", "state"} {
		t.Run(corrupt, func(t *testing.T) {
			cfg := validatorIdentityConfig(t)
			validatorIdentityUsedValidator(t, cfg)
			path := cfg.PrivValidator.KeyFile()
			if corrupt == "state" {
				path = cfg.PrivValidator.StateFile()
			}
			require.NoError(t, os.WriteFile(path, []byte("invalid validator JSON"), 0600))
			before := validatorIdentityFiles(t, cfg)
			nodeID, pub, err := InitializeNodeValidatorFilesFromMnemonic(cfg, validatorIdentityMnemonic)
			require.Error(t, err)
			require.Empty(t, nodeID)
			require.Nil(t, pub)
			require.Equal(t, before, validatorIdentityFiles(t, cfg))
		})
	}
}

func TestValidatorIdentitySaveFailure(t *testing.T) {
	cfg := validatorIdentityConfig(t)
	require.NoError(t, os.Mkdir(filepath.Join(filepath.Dir(cfg.PrivValidator.KeyFile()), "validator_pubkey.txt"), 0700))
	nodeID, pub, err := InitializeNodeValidatorFilesFromMnemonic(cfg, validatorIdentityMnemonic)
	require.Error(t, err)
	require.Empty(t, nodeID)
	require.Nil(t, pub)
	_, err = privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
	require.Error(t, err)
}

func validatorIdentityConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := config.TestConfig().SetRoot(t.TempDir())
	require.NoError(t, os.MkdirAll(filepath.Dir(cfg.NodeKeyFile()), 0700))
	require.NoError(t, os.MkdirAll(filepath.Dir(cfg.PrivValidator.StateFile()), 0700))
	return cfg
}

func validatorIdentityUsedValidator(t *testing.T, cfg *config.Config) *privval.FilePV {
	t.Helper()
	pv := privval.NewFilePV(ed25519.TestSecretKey([]byte(validatorIdentityMnemonic)), cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
	require.NoError(t, pv.Save())
	vote := validatorIdentityVote(pv, 12, 1)
	require.NoError(t, pv.SignVote(t.Context(), validatorIdentityChainID, vote))
	validatorIdentityVerify(t, pv, vote)
	return pv
}

func validatorIdentityVote(pv *privval.FilePV, height int64, block byte) *tmproto.Vote {
	hash := bytes.Repeat([]byte{block}, crypto.HashSize)
	vote := tmtypes.Vote{
		Type: tmproto.PrevoteType, Height: height, Round: 2,
		BlockID:   tmtypes.BlockID{Hash: hash, PartSetHeader: tmtypes.PartSetHeader{Total: 1, Hash: hash}},
		Timestamp: time.Unix(1700000000, 0).UTC(), ValidatorAddress: pv.GetAddress(),
	}
	return vote.ToProto()
}

func validatorIdentityVerify(t *testing.T, pv *privval.FilePV, vote *tmproto.Vote) {
	t.Helper()
	signature, err := crypto.SigFromBytes(vote.Signature)
	require.NoError(t, err)
	require.NoError(t, pv.Key.PubKey.Verify(tmtypes.VoteSignBytes(validatorIdentityChainID, vote), signature))
}

func validatorIdentityFiles(t *testing.T, cfg *config.Config) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	for _, path := range []string{cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile(), filepath.Join(filepath.Dir(cfg.PrivValidator.KeyFile()), "validator_pubkey.txt")} {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		files[path] = data
	}
	return files
}
