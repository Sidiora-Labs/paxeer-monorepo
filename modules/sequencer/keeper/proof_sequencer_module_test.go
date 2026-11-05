// Proves: sequencer.AppModule.InitGenesis and Keeper.DeliverSequenceBatch -> committed IAVL and params stores -> sequencer.AppModule.ExportGenesis, node.App.Sequenced and rpc.SequencerReceiptResult.Verify.
//
// This proof repairs Keeper.InitGenesis in keeper.go, which checked receipt-key
// coverage before resolving a default activation height, and GetLeadersParam in
// leaders_param.go, which accepted duplicate or zero persisted leaders. It also
// exercises the idempotent activation setter after a real database close/reopen.
// node/upgrades_sequencer_test.go's rewindBeforeMerge now removes both governed
// params, so its pre-activation state cannot retain an activated leader list.
// The proof uses the consumers' exported methods and the real governance handler,
// transaction codec, receipt signer and verifier; no store or crossing is doubled.
package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	sequencermodule "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	seqnode "github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	sdkcodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const proofSequencerChainID = "sequencer-module-proof-1"

type proofSequencerStore struct {
	ms      storetypes.CommitMultiStore
	ctx     sdk.Context
	key     *storetypes.KVStoreKey
	keeper  keeper.Keeper
	module  sequencermodule.AppModule
	space   paramtypes.Subspace
	amino   *sdkcodec.LegacyAmino
	handler govtypes.Handler
}

func openProofSequencerStore(t *testing.T, dir string, height int64) *proofSequencerStore {
	t.Helper()
	db, err := dbm.NewGoLevelDB("sequencer-proof", dir)
	require.NoError(t, err)
	ms := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(types.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(paramsKey, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, db)
	require.NoError(t, ms.LoadLatestVersion())
	amino := sdkcodec.NewLegacyAmino()
	pk := paramskeeper.NewKeeper(sdkcodec.NewProtoCodec(codectypes.NewInterfaceRegistry()), amino, paramsKey, transientKey)
	space := pk.Subspace(types.ModuleName).WithKeyTable(keeper.ParamKeyTable())
	k := keeper.NewKeeper(key)
	return &proofSequencerStore{
		ms: ms, ctx: sdk.NewContext(ms, tmproto.Header{ChainID: proofSequencerChainID, Height: height}, false).WithTxIndex(0),
		key: key, keeper: k, module: sequencermodule.NewAppModule(k, space), space: space, amino: amino,
		handler: sequencermodule.NewParamChangeProposalHandler(k, space, params.NewParamChangeProposalHandler(pk)),
	}
}

func (s *proofSequencerStore) close(t *testing.T) {
	t.Helper()
	require.NoError(t, s.ms.Close())
}

func (s *proofSequencerStore) change(t *testing.T, leaders keeper.LeaderList, keys keeper.ReceiptKeys) error {
	t.Helper()
	leaderJSON, err := s.amino.MarshalAsJSON(leaders)
	require.NoError(t, err)
	keysJSON, err := s.amino.MarshalAsJSON(keys)
	require.NoError(t, err)
	return s.handler(s.ctx, proposal.NewParameterChangeProposal("sequencer proof", "rotate the governed pins", []proposal.ParamChange{
		proposal.NewParamChange(types.ModuleName, string(keeper.KeyLeaders), string(leaderJSON)),
		proposal.NewParamChange(types.ModuleName, string(keeper.KeyReceiptKeys), string(keysJSON)),
	}, false))
}

func TestProofSequencerModule(t *testing.T) {
	dir := t.TempDir()
	s := openProofSequencerStore(t, dir, 0)
	// Cleanup follows the currently open instance; each earlier instance is closed
	// before the next one is constructed over the same on-disk database.
	t.Cleanup(func() { s.close(t) })
	seed := bytes.Repeat([]byte{0x21}, ed25519.SeedSize)
	private := ed25519.NewKeyFromSeed(seed)
	public := [32]byte(private.Public().(ed25519.PublicKey))
	keyID := [32]byte{0x44}
	leaders := [][32]byte{{0x51}, {0x52}}
	pins := keeper.ReceiptKeys{{KeyId: keyID[:], PublicKey: public[:], FirstHeight: 1, LastHeight: 100}}
	genesis, err := json.Marshal(map[string]any{
		"leaders": [][]byte{leaders[0][:], leaders[1][:]}, "receipt_keys": pins, "activation_height": 2,
	})
	require.NoError(t, err)
	require.NoError(t, s.module.ValidateGenesis(nil, nil, genesis))
	require.Empty(t, s.module.InitGenesis(s.ctx, nil, genesis))
	require.Equal(t, leaders, keeper.GetLeadersParam(s.ctx, s.space))
	require.Equal(t, pins, keeper.GetReceiptKeys(s.ctx, s.space))

	// Use the same digest, signature and exported tx codec as the block and RPC
	// consumers, deriving the range/hash/root from actual transaction bytes.
	tx := []byte("sequencer module proof activity")
	activity := sha256.Sum256(tx)
	lastHash := types.SeqHash([32]byte{}, 1, activity)
	root := types.TxRoot([][32]byte{activity})
	makeBatch := func(height uint64, signer ed25519.PrivateKey) types.MsgSequenceBatch {
		msg := types.MsgSequenceBatch{Height: height, FromSeq: 1, ToSeq: 1,
			PrevSeqHash: make([]byte, 32), LastSeqHash: lastHash[:], TxRoot: root[:], KeyId: keyID[:]}
		digest, err := msg.SignBytes(proofSequencerChainID)
		require.NoError(t, err)
		msg.Sig = ed25519.Sign(signer, digest[:])
		encoded, err := app.EncodeSequenceBatch(msg)
		require.NoError(t, err)
		decoded, err := app.DecodeSequenceBatch(encoded)
		require.NoError(t, err)
		require.Equal(t, msg, decoded)
		require.NoError(t, types.VerifyBatchTxs(decoded, [][32]byte{activity}))
		return decoded
	}
	before := s.module.ExportGenesis(s.ctx, nil)
	err = s.keeper.DeliverSequenceBatch(s.ctx.WithBlockHeight(1), makeBatch(1, private))
	require.ErrorIs(t, err, keeper.ErrNotActivated)
	codespace, code, _ := sdkerrors.ABCIInfo(err, false)
	require.Equal(t, types.ModuleName, codespace)
	require.Equal(t, uint32(11), code)
	require.JSONEq(t, string(before), string(s.module.ExportGenesis(s.ctx, nil)))

	// A real governance proposal updates both physical stores atomically. A
	// duplicate leader fails validation and cannot publish either changed param.
	s.ctx = s.ctx.WithBlockHeight(1)
	rotated := keeper.LeaderList{leaders[1][:], leaders[0][:]}
	otherPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, ed25519.SeedSize))
	otherPublic := [32]byte(otherPrivate.Public().(ed25519.PublicKey))
	rotatedPins := append(append(keeper.ReceiptKeys(nil), pins...), keeper.ReceiptKeyEntry{
		KeyId: bytes.Repeat([]byte{0x45}, 32), PublicKey: otherPublic[:], FirstHeight: 3, LastHeight: 100,
	})
	require.NoError(t, s.change(t, rotated, rotatedPins))
	governed := s.module.ExportGenesis(s.ctx, nil)
	require.ErrorIs(t, s.change(t, keeper.LeaderList{leaders[0][:], leaders[0][:]}, pins), proposal.ErrSettingParameter)
	require.JSONEq(t, string(governed), string(s.module.ExportGenesis(s.ctx, nil)))
	require.Equal(t, rotatedPins, keeper.GetReceiptKeys(s.ctx, s.space))
	commit := s.ms.Commit(true)
	require.Equal(t, int64(1), commit.Version)
	s.close(t)
	s = openProofSequencerStore(t, dir, 2)
	require.Equal(t, commit, s.ms.LastCommitID())
	require.JSONEq(t, string(governed), string(s.module.ExportGenesis(s.ctx, nil)))
	wantLeaders := [][32]byte{leaders[1], leaders[0]}
	gotLeaders, err := s.keeper.Leaders(s.ctx)
	require.NoError(t, err)
	require.Equal(t, wantLeaders, gotLeaders)
	require.Equal(t, gotLeaders, keeper.GetLeadersParam(s.ctx, s.space))
	activation, err := s.keeper.ActivationHeight(s.ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), activation)
	require.NoError(t, s.keeper.SetActivationHeight(s.ctx, activation))
	require.ErrorIs(t, s.keeper.SetActivationHeight(s.ctx, 3), types.ErrInvalidGenesis)
	consumer := &app.App{SequencerKeeper: s.keeper}
	require.False(t, consumer.Sequenced(s.ctx, 1))
	require.True(t, consumer.Sequenced(s.ctx, 2))

	// A wrong signer crosses the same codec and is refused without advancing
	// the persistent sequence. The genuine signer then advances it fee-free.
	before = s.module.ExportGenesis(s.ctx, nil)
	require.ErrorIs(t, s.keeper.DeliverSequenceBatch(s.ctx, makeBatch(2, otherPrivate)), types.ErrBadSignature)
	require.JSONEq(t, string(before), string(s.module.ExportGenesis(s.ctx, nil)))
	msg := makeBatch(2, private)
	meter := sdk.NewGasMeter(1, 1, 1)
	require.NoError(t, s.keeper.DeliverSequenceBatch(s.ctx.WithGasMeter(meter), msg))
	require.Zero(t, meter.GasConsumed())
	exported, err := s.keeper.ExportGenesis(s.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), exported.LastSeq)
	require.Equal(t, lastHash, exported.LastSeqHash)
	require.Equal(t, uint64(2), exported.ActivationHeight)
	require.Equal(t, wantLeaders, exported.Leaders)
	require.Len(t, exported.ReceiptKeys, 2)
	require.NoError(t, exported.Validate())
	moduleExport := s.module.ExportGenesis(s.ctx, nil)
	commit = s.ms.Commit(true)
	require.Equal(t, int64(2), commit.Version)
	s.close(t)
	s = openProofSequencerStore(t, dir, 2)
	require.Equal(t, commit, s.ms.LastCommitID())
	restored, err := s.keeper.ExportGenesis(s.ctx)
	require.NoError(t, err)
	require.Equal(t, exported, restored)
	require.JSONEq(t, string(moduleExport), string(s.module.ExportGenesis(s.ctx, nil)))
	require.ErrorIs(t, s.keeper.DeliverSequenceBatch(s.ctx, msg), keeper.ErrBatchRepeated)
	require.JSONEq(t, string(moduleExport), string(s.module.ExportGenesis(s.ctx, nil)))
	seq, err := s.keeper.LastSeq(s.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), seq)
	hash, err := s.keeper.LastSeqHash(s.ctx)
	require.NoError(t, err)
	require.Equal(t, lastHash, hash)

	// Exported module JSON is consumed by the real InitGenesis on a fresh
	// database; it preserves activation, sequence and both governed pin sets.
	fresh := openProofSequencerStore(t, t.TempDir(), 20)
	defer fresh.close(t)
	require.NoError(t, fresh.module.ValidateGenesis(nil, nil, moduleExport))
	fresh.module.InitGenesis(fresh.ctx, nil, moduleExport)
	imported, err := fresh.keeper.ExportGenesis(fresh.ctx)
	require.NoError(t, err)
	require.Equal(t, restored, imported)
	require.JSONEq(t, string(moduleExport), string(fresh.module.ExportGenesis(fresh.ctx, nil)))

	// Produce a canonical signed receipt using the host's actual signer and
	// verify its JSON view with the RPC consumer and the reopened on-chain pins.
	keyDir := t.TempDir()
	keyFile := filepath.Join(keyDir, "receipt_key")
	require.NoError(t, os.WriteFile(keyFile, []byte(hex.EncodeToString(seed)+"\n"), 0o600))
	consensusFile := filepath.Join(keyDir, "priv_validator_key.json")
	pv, err := privval.GenFilePV(consensusFile, filepath.Join(keyDir, "priv_validator_state.json"), "")
	require.NoError(t, err)
	require.NoError(t, pv.Key.Save())
	signer, err := seqnode.LoadReceiptKey(keyFile, consensusFile, []verify.SequencerAuthorization{{
		SequencerID: keyID, PublicKey: public, FirstBatchNumber: 1, LastBatchNumber: 100,
	}}, 2)
	require.NoError(t, err)
	input := &seqnode.ReceiptInput{
		Accepted: &seqnode.Accepted{Seq: seq, SeqHash: hash, ActivityID: activity, Height: 2, TxIndex: 1, TimestampMs: 1700000000000},
		EVM:      &evmtypes.Receipt{Status: 1}, PaymentDenom: "upax", FeeDenom: "upax", FeeCharged: big.NewInt(0), Amount: big.NewInt(0),
		FromBalanceBefore: big.NewInt(0), FromBalanceAfter: big.NewInt(0), ToBalanceBefore: big.NewInt(0), ToBalanceAfter: big.NewInt(0),
		ModuleID: app.EVMReceiptModuleID,
	}
	signed, err := signer.SignReceipt(input)
	require.NoError(t, err)
	persistedPins := keeper.GetReceiptKeys(s.ctx, s.space)
	verified, err := persistedPins.VerifyReceipt(signed)
	require.NoError(t, err)
	require.Equal(t, seq, verified.Receipt.GlobalSequence)
	blockNumber := hexutil.Uint64(2)
	blockHash := common.BytesToHash(commit.Hash)
	result := evmrpc.SequencerReceiptResult{Rung: evmrpc.RungCommitted, TxHash: common.Hash(activity),
		BlockNumber: &blockNumber, BlockHash: &blockHash,
		SequencerReceiptView: &evmrpc.SequencerReceiptView{
			Seq: hexutil.Uint64(seq), Height: 2, Index: 1, Timestamp: 1700000000000, Status: 1,
			Logs: []evmrpc.SequencerReceiptLog{}, PaymentDenom: input.PaymentDenom, FeeDenom: input.FeeDenom, FeeAmount: (*hexutil.Big)(big.NewInt(0)),
			KeyID: common.Hash(keyID), SigningKey: public[:], Receipt: signed,
		},
	}
	wire, err := json.Marshal(result)
	require.NoError(t, err)
	var observed evmrpc.SequencerReceiptResult
	require.NoError(t, json.Unmarshal(wire, &observed))
	require.NoError(t, observed.Verify(persistedPins))
	require.Equal(t, input.PaymentDenom, observed.PaymentDenom)
	require.Equal(t, evmrpc.RungCommitted, observed.Rung)
	require.Equal(t, seq, uint64(observed.Seq))
	for _, paymentDenom := range []string{"", "uhpx", "different-payment-asset"} {
		observed.PaymentDenom = paymentDenom
		require.ErrorIs(t, observed.Verify(persistedPins), evmrpc.ErrSequencerReceiptMismatch)
	}
	observed.PaymentDenom = input.PaymentDenom
	observed.Status = 0
	require.ErrorIs(t, observed.Verify(persistedPins), evmrpc.ErrSequencerReceiptMismatch)
}

func TestProofSequencerModuleDefaultActivationCoverage(t *testing.T) {
	s := openProofSequencerStore(t, t.TempDir(), 10)
	defer s.close(t)
	gs := genesisStateForTest()
	for i := range gs.ReceiptKeys {
		gs.ReceiptKeys[i].FirstHeight = 20
	}
	// The default resolves to 11, outside all receipt keys. Refusal must occur
	// before writing any of the five activation/genesis keys.
	require.ErrorIs(t, s.keeper.InitGenesis(s.ctx, gs), types.ErrInvalidGenesis)
	for _, key := range [][]byte{types.ActivationHeightKey, types.LeaderListKey, types.ReceiptKeysKey, types.LastSeqKey, types.LastSeqHashKey} {
		require.Nil(t, s.ctx.KVStore(s.key).Get(key))
	}
}

func TestProofSequencerModuleCorruptLeadersParam(t *testing.T) {
	for _, list := range []keeper.LeaderList{
		{make([]byte, 32)},
		{bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{1}, 32)},
	} {
		s := openProofSequencerStore(t, t.TempDir(), 0)
		func() {
			defer s.close(t)
			// Subspace.Set deliberately assumes validation by its caller. Use that
			// real write API to prove the read boundary refuses invalid stored data.
			s.space.Set(s.ctx, keeper.KeyLeaders, list)
			require.Panics(t, func() { keeper.GetLeadersParam(s.ctx, s.space) })
		}()
	}
}
