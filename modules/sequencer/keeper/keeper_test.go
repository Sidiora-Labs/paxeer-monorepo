package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
	"math"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const testChainID = "sequencer-test-1"

type fixture struct {
	keeper   keeper.Keeper
	ctx      sdk.Context
	private  ed25519.PrivateKey
	keyID    [types.KeyIDLength]byte
	storeKey sdk.StoreKey
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(types.StoreKey)
	ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, tmproto.Header{ChainID: testChainID, Height: 1000}, false).WithChainID(testChainID)

	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x07}, ed25519.SeedSize))
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	var keyID [types.KeyIDLength]byte
	copy(keyID[:], bytes.Repeat([]byte{0x44}, types.KeyIDLength))
	var leader [32]byte
	copy(leader[:], bytes.Repeat([]byte{0x55}, 32))

	k := keeper.NewKeeper(key)
	require.NoError(t, k.InitGenesis(ctx, types.GenesisState{
		Leaders:     [][32]byte{leader},
		ReceiptKeys: []types.ReceiptKey{{KeyId: keyID, PublicKey: public, FirstHeight: 1000, LastHeight: 1999}},
	}))
	return fixture{keeper: k, ctx: ctx, private: private, keyID: keyID, storeKey: key}
}

func (f fixture) batch(t *testing.T, height, fromSeq, toSeq uint64, prev, last byte) types.MsgSequenceBatch {
	t.Helper()
	msg := types.MsgSequenceBatch{
		Height:      height,
		FromSeq:     fromSeq,
		ToSeq:       toSeq,
		PrevSeqHash: bytes.Repeat([]byte{prev}, types.HashLength),
		LastSeqHash: bytes.Repeat([]byte{last}, types.HashLength),
		TxRoot:      bytes.Repeat([]byte{0x33}, types.HashLength),
		KeyId:       bytes.Clone(f.keyID[:]),
	}
	if msg.IsEmpty() {
		msg.TxRoot = bytes.Clone(types.EmptyTxRoot[:])
	}
	digest, err := msg.SignBytes(testChainID)
	require.NoError(t, err)
	msg.Sig = ed25519.Sign(f.private, digest[:])
	return msg
}

func (f fixture) at(height int64, txIndex int) sdk.Context {
	return f.ctx.WithBlockHeight(height).WithTxIndex(txIndex)
}

func (f fixture) requireStored(t *testing.T, lastSeq uint64, lastSeqHash byte) {
	t.Helper()
	seq, err := f.keeper.LastSeq(f.ctx)
	require.NoError(t, err)
	require.Equal(t, lastSeq, seq)
	hash, err := f.keeper.LastSeqHash(f.ctx)
	require.NoError(t, err)
	require.Equal(t, bytes.Repeat([]byte{lastSeqHash}, types.HashLength), hash[:])
}

func TestSequenceBatchMustBeFirst(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, f.keeper.SetActivationHeight(f.ctx, 1))
	f.requireStored(t, 0, 0x00)

	first := f.batch(t, 1000, 1, 30, 0x00, 0x11)
	for _, index := range []int{1, 2, 7} {
		require.ErrorIs(t, f.keeper.DeliverSequenceBatch(f.at(1000, index), first), keeper.ErrBatchPosition)
		f.requireStored(t, 0, 0x00)
	}
	require.NoError(t, f.keeper.DeliverSequenceBatch(f.at(1000, 0), first))
	f.requireStored(t, 30, 0x11)

	again := f.batch(t, 1000, 31, 30, 0x11, 0x11)
	require.ErrorIs(t, f.keeper.DeliverSequenceBatch(f.at(1000, 0), again), keeper.ErrBatchRepeated)
	require.ErrorIs(t, f.keeper.DeliverSequenceBatch(f.at(1000, 0), first), keeper.ErrBatchRepeated)
	f.requireStored(t, 30, 0x11)

	wrongHeight := f.batch(t, 1002, 31, 30, 0x11, 0x11)
	require.ErrorIs(t, f.keeper.DeliverSequenceBatch(f.at(1001, 0), wrongHeight), types.ErrInvalidSequenceBatch)
	f.requireStored(t, 30, 0x11)

	next := f.batch(t, 1001, 31, 30, 0x11, 0x11)
	require.NoError(t, f.keeper.DeliverSequenceBatch(f.at(1001, 0), next))
	f.requireStored(t, 30, 0x11)
	require.ErrorIs(t, f.keeper.DeliverSequenceBatch(f.at(999, 0), f.batch(t, 999, 31, 30, 0x11, 0x11)), keeper.ErrBatchRepeated)
}

func TestSequenceBatchChainsFromStore(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, f.keeper.SetActivationHeight(f.ctx, 1))
	require.NoError(t, f.keeper.DeliverSequenceBatch(f.at(1000, 0), f.batch(t, 1000, 1, 30, 0x00, 0x11)))
	f.requireStored(t, 30, 0x11)

	ctx := f.at(1001, 0)
	require.ErrorIs(t, f.keeper.DeliverSequenceBatch(ctx, f.batch(t, 1001, 32, 42, 0x11, 0x22)), keeper.ErrSeqDiscontinuity)
	require.ErrorIs(t, f.keeper.DeliverSequenceBatch(ctx, f.batch(t, 1001, 30, 42, 0x11, 0x22)), keeper.ErrSeqDiscontinuity)
	require.ErrorIs(t, f.keeper.DeliverSequenceBatch(ctx, f.batch(t, 1001, 31, 42, 0x12, 0x22)), keeper.ErrSeqDiscontinuity)
	f.requireStored(t, 30, 0x11)

	otherKey := f.batch(t, 1001, 31, 42, 0x11, 0x22)
	otherKey.KeyId = bytes.Repeat([]byte{0x45}, types.KeyIDLength)
	require.ErrorIs(t, f.keeper.DeliverSequenceBatch(ctx, otherKey), keeper.ErrUnknownReceiptKey)

	badSig := f.batch(t, 1001, 31, 42, 0x11, 0x22)
	badSig.TxRoot = bytes.Repeat([]byte{0x34}, types.HashLength)
	require.ErrorIs(t, f.keeper.DeliverSequenceBatch(ctx, badSig), types.ErrBadSignature)

	malformed := f.batch(t, 1001, 31, 42, 0x11, 0x22)
	malformed.Sig = malformed.Sig[:63]
	require.ErrorIs(t, f.keeper.DeliverSequenceBatch(ctx, malformed), types.ErrInvalidSequenceBatch)
	f.requireStored(t, 30, 0x11)

	require.NoError(t, f.keeper.DeliverSequenceBatch(ctx, f.batch(t, 1001, 31, 42, 0x11, 0x22)))
	f.requireStored(t, 42, 0x22)

	require.NoError(t, f.keeper.DeliverSequenceBatch(f.at(1002, 0), f.batch(t, 1002, 43, 42, 0x22, 0x22)))
	f.requireStored(t, 42, 0x22)

	outOfRange := f.batch(t, 2000, 43, 43, 0x22, 0x23)
	require.ErrorIs(t, f.keeper.DeliverSequenceBatch(f.at(2000, 0), outOfRange), keeper.ErrUnknownReceiptKey)
	f.requireStored(t, 42, 0x22)

	exported, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(42), exported.LastSeq)
	require.Equal(t, [][32]byte{[32]byte(bytes.Repeat([]byte{0x55}, 32))}, exported.Leaders)
	require.Len(t, exported.ReceiptKeys, 1)
	require.Equal(t, f.keyID, exported.ReceiptKeys[0].KeyId)
	require.Equal(t, uint64(1000), exported.ReceiptKeys[0].FirstHeight)
	require.Equal(t, uint64(1999), exported.ReceiptKeys[0].LastHeight)
}

func TestSequenceBatchRefusedBelowActivation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		activation int64
		heights    []int64
	}{
		{name: "below activation", activation: 1001, heights: []int64{1, 999, 1000}},
		{name: "unset activation", heights: []int64{1, 1000, 1001, 1999, math.MaxInt64}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			if tc.activation != 0 {
				require.NoError(t, f.keeper.SetActivationHeight(f.ctx, tc.activation))
			}
			snapshot := func() map[string][]byte {
				entries := make(map[string][]byte)
				iterator := f.ctx.KVStore(f.storeKey).Iterator(nil, nil)
				defer iterator.Close()
				for ; iterator.Valid(); iterator.Next() {
					entries[string(iterator.Key())] = bytes.Clone(iterator.Value())
				}
				return entries
			}
			before := snapshot()
			for _, height := range tc.heights {
				msg := f.batch(t, uint64(height), 1, 3, 0x00, 0x11)
				err := f.keeper.DeliverSequenceBatch(f.at(height, 0), msg)
				require.ErrorIs(t, err, keeper.ErrNotActivated, "height %d", height)
				codespace, code, _ := sdkerrors.ABCIInfo(err, false)
				require.Equal(t, "sequencer", codespace)
				require.Equal(t, uint32(11), code)
				require.Contains(t, err.Error(), fmt.Sprintf("block height %d, activation height %d", height, tc.activation))
				f.requireStored(t, 0, 0x00)
				require.Equal(t, before, snapshot(), "store changed at height %d", height)
			}
		})
	}
}

func TestSequenceBatchAcceptedAtActivation(t *testing.T) {
	f := newFixture(t)
	const activation int64 = 1001
	require.NoError(t, f.keeper.SetActivationHeight(f.ctx, activation))
	f.requireStored(t, 0, 0x00)
	msg := f.batch(t, uint64(activation), 1, 3, 0x00, 0x11)
	require.NoError(t, f.keeper.DeliverSequenceBatch(f.at(activation, 0), msg))
	f.requireStored(t, 3, 0x11)
}

func TestSequenceBatchFeeFreeAndBlockOnly(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, f.keeper.SetActivationHeight(f.ctx, 1))
	msg := f.batch(t, 1000, 1, 30, 0x00, 0x11)

	for name, ctx := range map[string]sdk.Context{
		"check":   f.at(1000, 0).WithIsCheckTx(true),
		"recheck": f.at(1000, 0).WithIsReCheckTx(true),
	} {
		require.ErrorIs(t, keeper.RefuseCheckTx(ctx), keeper.ErrBatchBlockOnly, name)
		require.ErrorIs(t, f.keeper.DeliverSequenceBatch(ctx, msg), keeper.ErrBatchBlockOnly, name)
		f.requireStored(t, 0, 0x00)
	}
	require.NoError(t, keeper.RefuseCheckTx(f.at(1000, 0)))

	meter := sdk.NewGasMeter(1, 1, 1)
	ctx := f.at(1000, 0).WithGasMeter(meter)
	require.NoError(t, f.keeper.DeliverSequenceBatch(ctx, msg))
	require.Equal(t, uint64(0), meter.GasConsumed())
	require.False(t, meter.IsOutOfGas())
	f.requireStored(t, 30, 0x11)

	require.ErrorIs(t, types.GenesisState{}.Validate(), types.ErrInvalidGenesis)
	var public [32]byte
	copy(public[:], f.private.Public().(ed25519.PublicKey))
	leader := [32]byte(bytes.Repeat([]byte{0x55}, 32))
	valid := types.GenesisState{
		Leaders:     [][32]byte{leader},
		ReceiptKeys: []types.ReceiptKey{{KeyId: f.keyID, PublicKey: public, FirstHeight: 1000, LastHeight: 1999}},
	}
	require.NoError(t, valid.Validate())
	for name, mutate := range map[string]func(*types.GenesisState){
		"no leaders":        func(g *types.GenesisState) { g.Leaders = nil },
		"zero leader":       func(g *types.GenesisState) { g.Leaders = [][32]byte{{}} },
		"repeated leader":   func(g *types.GenesisState) { g.Leaders = [][32]byte{leader, leader} },
		"no receipt keys":   func(g *types.GenesisState) { g.ReceiptKeys = nil },
		"zero keyId":        func(g *types.GenesisState) { g.ReceiptKeys[0].KeyId = [types.KeyIDLength]byte{} },
		"repeated keyId":    func(g *types.GenesisState) { g.ReceiptKeys = append(g.ReceiptKeys, g.ReceiptKeys[0]) },
		"identity key":      func(g *types.GenesisState) { g.ReceiptKeys[0].PublicKey = [32]byte{0x01} },
		"zero first height": func(g *types.GenesisState) { g.ReceiptKeys[0].FirstHeight = 0 },
		"inverted range":    func(g *types.GenesisState) { g.ReceiptKeys[0].LastHeight = 999 },
		"lastSeq not zero":  func(g *types.GenesisState) { g.LastSeq = 1 },
	} {
		mutated := valid
		mutated.Leaders = append([][32]byte(nil), valid.Leaders...)
		mutated.ReceiptKeys = append([]types.ReceiptKey(nil), valid.ReceiptKeys...)
		mutate(&mutated)
		require.ErrorIs(t, mutated.Validate(), types.ErrInvalidGenesis, name)
		require.ErrorIs(t, f.keeper.InitGenesis(f.ctx, mutated), types.ErrInvalidGenesis, name)
	}
	f.requireStored(t, 30, 0x11)
}
