package sequencer_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	gigalib "github.com/Sidiora-Labs/Paxeer-X-Network/engine/executor/lib"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdked25519 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type applyCheckHome struct {
	home   string
	txs    [][]byte
	signed []*ethtypes.Transaction
	keys   seqkeeper.ReceiptKeys
	commit sdk.CommitID
}

func newApplyCheckHome(t *testing.T, rotated bool) applyCheckHome {
	t.Helper()
	previous := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previous })
	at := time.Unix(1_800_000_000, 0).UTC()
	validator := sdked25519.GenPrivKey().PubKey()
	w := app.NewTestWrapper(t, at, validator, true, func(ba *baseapp.BaseApp) { ba.SetOccEnabled(false); ba.SetConcurrencyWorkers(1) })
	a := w.App
	validatorAddress := sdk.ValAddress(validator.Address())
	delegation := sdk.NewCoin(a.StakingKeeper.BondDenom(w.Ctx), a.StakingKeeper.PowerReduction(w.Ctx).MulRaw(100))
	w.FundAcc(sdk.AccAddress(validatorAddress), sdk.NewCoins(delegation))
	_, err := staking.NewHandler(a.StakingKeeper)(w.Ctx, stakingtypes.NewMsgDelegate(sdk.AccAddress(validatorAddress), validatorAddress, delegation))
	require.NoError(t, err)
	payer, recipient := utils.NewSigner(), utils.NewSigner()
	a.EvmKeeper.SetAddressMapping(w.Ctx, payer.AccountAddress, payer.EvmAddress)
	w.FundAcc(payer.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000_000_000)))
	params := a.GetConsensusParams(w.Ctx)
	params.Block.MaxGasWanted = params.Block.MaxGas
	a.StoreConsensusParams(w.Ctx, params)
	entry := func(seed byte, first uint64) seqkeeper.ReceiptKeyEntry {
		private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
		public := private.Public().(ed25519.PublicKey)
		id := sha256.Sum256(public)
		return seqkeeper.ReceiptKeyEntry{KeyId: id[:], PublicKey: public, FirstHeight: first, LastHeight: math.MaxUint64}
	}
	keys := seqkeeper.ReceiptKeys{entry(0x61, 1)}
	require.NoError(t, a.SequencerKeeper.InitReceiptKeyState(w.Ctx, a.GetSubspace(seqtypes.ModuleName), keys))
	require.NoError(t, a.SequencerKeeper.SetActivationHeight(w.Ctx, 3))
	a.UpgradeKeeper.SetDone(w.Ctx, app.MergeUpgrade)
	for height := int64(1); height <= 2; height++ {
		if height == 2 && rotated {
			keys[0].LastHeight = 1
			keys = append(keys, entry(0x62, 2))
			require.NoError(t, a.SequencerKeeper.InitReceiptKeyState(a.NewUncachedContext(false, tmproto.Header{Height: height}), a.GetSubspace(seqtypes.ModuleName), keys))
		}
		_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Header: &tmproto.Header{ChainID: "pax-test", Height: height, Time: at.Add(time.Duration(height-1) * time.Second), ProposerAddress: validator.Address()}})
		require.NoError(t, err)
		_, err = a.Commit(context.Background())
		require.NoError(t, err)
	}
	require.True(t, a.Sequenced(a.GetCheckCtx(), 3))
	path, err := a.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	f := applyCheckHome{home: filepath.Dir(filepath.Dir(path)), keys: keys, commit: a.LastCommitID()}
	f.commit.Hash = bytes.Clone(f.commit.Hash)
	require.NoError(t, os.MkdirAll(filepath.Join(f.home, "config"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(f.home, "config", "genesis.json"), []byte(`{"chain_id":"pax-test"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(f.home, app.ApplyCheckMarker), nil, 0o600))
	constructor, err := hex.DecodeString("6001600c60003960016000f300")
	require.NoError(t, err)
	for nonce, tx := range []*ethtypes.LegacyTx{
		{To: &recipient.EvmAddress, Value: big.NewInt(1)},
		{Data: constructor, Value: new(big.Int)},
	} {
		tx.Nonce, tx.Gas, tx.GasPrice = uint64(nonce), 1_000_000, big.NewInt(500_000_000_000)
		signed, err := ethtypes.SignTx(ethtypes.NewTx(tx), ethtypes.LatestSignerForChainID(a.EvmKeeper.ChainID(a.GetCheckCtx())), payer.EvmPrivateKey)
		require.NoError(t, err)
		raw, err := signed.MarshalBinary()
		require.NoError(t, err)
		encoded, err := app.DecodeApplyCheckTransaction(hex.EncodeToString(raw))
		require.NoError(t, err)
		f.txs = append(f.txs, encoded)
		f.signed = append(f.signed, signed)
	}
	require.NoError(t, a.Close())
	reopened := openApplyCheckHome(t, f)
	stored, err := reopened.SequencerKeeper.ReceiptKeys(reopened.GetCheckCtx())
	require.NoError(t, err)
	require.Len(t, stored, len(keys))
	for i, key := range stored {
		require.Equal(t, keys[i].KeyId, key.KeyId[:])
		require.Equal(t, keys[i].FirstHeight, key.FirstHeight)
		require.Equal(t, keys[i].LastHeight, key.LastHeight)
	}
	require.Equal(t, keys, seqkeeper.GetReceiptKeys(reopened.GetCheckCtx(), reopened.GetSubspace(seqtypes.ModuleName)))
	require.True(t, reopened.Sequenced(reopened.GetCheckCtx(), 3))
	require.NoError(t, reopened.Close())
	return f
}

func openApplyCheckHome(t *testing.T, f applyCheckHome) *app.App {
	t.Helper()
	a := app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, f.home, 0, true, tmconfig.TestConfig(), app.MakeEncodingConfig(), nil, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
	require.Equal(t, f.commit, a.LastCommitID())
	return a
}

func applyCheckFiles(t *testing.T, home string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	require.NoError(t, filepath.WalkDir(home, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			files[rel] = link
			return err
		}
		data, err := os.ReadFile(path)
		files[rel] = hex.EncodeToString(data)
		return err
	}))
	return files
}

func checkStandInForms(t *testing.T, f applyCheckHome, library string) {
	t.Helper()
	before := applyCheckFiles(t, f.home)
	key, err := sequencer.NewApplyCheckReceiptKey()
	require.NoError(t, err)
	report, err := app.ApplyCheckHomeWithOptions(context.Background(), f.home, f.txs, time.Time{}, sequencer.ApplyCheck{ReceiptKey: key}, app.ApplyCheckOptions{StandInReceiptKey: true})
	require.NoError(t, err)
	encoded, err := json.Marshal(report)
	require.NoError(t, err)
	require.True(t, report.Comparison.Equal, "%s", encoded)
	require.Equal(t, "stand-in", report.ReceiptKeyMode)
	require.Equal(t, int64(3), report.Height)
	require.Equal(t, f.commit.Version, report.CommittedHeight)
	require.Equal(t, hex.EncodeToString(f.commit.Hash), report.CommittedAppHash)
	require.Len(t, report.Forms, 3)
	for i, form := range report.Forms {
		require.Equal(t, []string{"sequencer", "follower_fast", "follower_ordinary"}[i], form.Name)
		require.Empty(t, form.Error)
		require.Empty(t, form.Panic)
		require.NotEmpty(t, form.AppHash)
		if form.Name == "follower_fast" {
			require.Equal(t, library, form.EngineLibrary)
		} else {
			require.Empty(t, form.EngineLibrary)
		}
		require.Len(t, form.Results, 2)
		for j, result := range form.Results {
			require.Zero(t, result.Code)
			require.Empty(t, result.ReceiptError)
			require.NotNil(t, result.Status)
			require.Equal(t, uint32(1), *result.Status)
			require.NotNil(t, result.Receipt)
			require.Equal(t, f.signed[j].Hash().Hex(), result.Receipt.TxHashHex)
		}
		sender, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(f.signed[1].ChainId()), f.signed[1])
		require.NoError(t, err)
		require.Equal(t, crypto.CreateAddress(sender, 1).Hex(), form.Results[1].ContractAddress)
	}
	public := key.PublicKey()
	require.NotContains(t, string(encoded), hex.EncodeToString(public[:]))
	require.Equal(t, before, applyCheckFiles(t, f.home))
	reopened := openApplyCheckHome(t, f)
	require.Equal(t, int64(2), reopened.LastBlockHeight())
	require.Equal(t, f.keys, seqkeeper.GetReceiptKeys(reopened.GetCheckCtx(), reopened.GetSubspace(seqtypes.ModuleName)))
	require.NoError(t, reopened.Close())
}

func TestApplyCheckStandInReceiptKey(t *testing.T) {
	vm, err := gigalib.InitEvmoneVM()
	require.NoError(t, err)
	vm.Destroy()
	for _, rotated := range []bool{true, false} {
		t.Run(fmt.Sprintf("rotated_%t", rotated), func(t *testing.T) {
			f := newApplyCheckHome(t, rotated)
			before := applyCheckFiles(t, f.home)
			_, err := app.ApplyCheckHome(context.Background(), f.home, f.txs, time.Time{}, sequencer.ApplyCheck{})
			require.ErrorContains(t, err, "sequenced home requires sequencer.receipt-key-file")
			require.Equal(t, before, applyCheckFiles(t, f.home))
			checkStandInForms(t, f, "loaded")
		})
	}
}

func TestApplyCheckReceiptKeyState(t *testing.T) {
	f := newApplyCheckHome(t, true)
	a := openApplyCheckHome(t, f)
	defer a.Close()
	// Read the old check view before registering the temporary identity.
	_, err := a.SequencerKeeper.ReceiptKeys(a.GetCheckCtx())
	require.NoError(t, err)
	key, err := sequencer.NewApplyCheckReceiptKey()
	require.NoError(t, err)
	require.NoError(t, (sequencer.ApplyCheck{ReceiptKey: key}).PrepareApplyCheckState(a, 3))
	id, err := key.KeyID(3)
	require.NoError(t, err)
	public := key.PublicKey()
	require.Equal(t, sha256.Sum256(public[:]), id)
	for _, ctx := range []sdk.Context{a.GetCheckCtx(), a.NewUncachedContext(false, tmproto.Header{Height: 3}), sdk.NewContext(a.CommitMultiStore().CacheMultiStore(), tmproto.Header{Height: 3}, false)} {
		stored, err := a.SequencerKeeper.ReceiptKeys(ctx)
		require.NoError(t, err)
		require.Len(t, stored, 1)
		require.Equal(t, id, stored[0].KeyId)
		require.Equal(t, public, stored[0].PublicKey)
		require.True(t, stored[0].Covers(3))
		params := seqkeeper.GetReceiptKeys(ctx, a.GetSubspace(seqtypes.ModuleName))
		require.Len(t, params, 1)
		require.Equal(t, id[:], params[0].KeyId)
		require.Equal(t, public[:], params[0].PublicKey)
		pins, err := a.SequencerKeeper.ReceiptKeyPins(ctx)
		require.NoError(t, err)
		require.Len(t, pins, 1)
		require.Equal(t, id, pins[0].SequencerID)
		require.Equal(t, uint64(3), pins[0].FirstBatchNumber)
		require.Equal(t, uint64(math.MaxUint64), pins[0].LastBatchNumber)
	}
	_, err = key.KeyID(2)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyNotPinned)
}

func TestApplyCheckEngineLibrary(t *testing.T) {
	if mode := os.Getenv("PAXEER_APPLYCHECK_ENGINE_LIBRARY"); mode != "" {
		vm, err := gigalib.InitEvmoneVM()
		if mode == "absent" {
			require.Error(t, err)
			require.Nil(t, vm)
		} else {
			require.NoError(t, err)
			require.NotNil(t, vm)
			vm.Destroy()
		}
		checkStandInForms(t, newApplyCheckHome(t, true), mode)
		return
	}
	library, err := gigalib.EvmoneLibraryPath()
	require.NoError(t, err)
	for _, mode := range []string{"absent", "loaded"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "absent" {
				hidden := library + ".applycheck-hidden"
				require.NoError(t, os.Rename(library, hidden))
				t.Cleanup(func() { require.NoError(t, os.Rename(hidden, library)) })
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestApplyCheckEngineLibrary$")
			child.Env = append(os.Environ(), "PAXEER_APPLYCHECK_ENGINE_LIBRARY="+mode)
			output, err := child.CombinedOutput()
			require.NoError(t, err, "%s", output)
		})
	}
}
