package db

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/store"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestRepair_86_DuplicateSaveReopenAndPrune(t *testing.T) {
	dir := t.TempDir()
	disk, trusted := repair86Open(t, dir, false)
	blocks := []*types.LightBlock{repair86Produce(t, 10), repair86Produce(t, 20), repair86Produce(t, 30)}
	for _, block := range blocks {
		require.NoError(t, trusted.SaveLightBlock(block))
		require.NoError(t, trusted.SaveLightBlock(block))
	}
	// A second delivery at an existing height still replaces its contents.
	replacement := repair86ProduceWithAppHash(t, 20, []byte("replacement"))
	require.NoError(t, trusted.SaveLightBlock(replacement))
	blocks[1] = replacement
	repair86AssertSet(t, trusted, blocks)
	require.NoError(t, disk.Close())

	disk, trusted = repair86Open(t, dir, false)
	repair86AssertSet(t, trusted, blocks)
	require.NoError(t, trusted.Prune(2))
	repair86AssertSet(t, trusted, blocks[1:])
	_, err := trusted.LightBlock(10)
	require.ErrorIs(t, err, store.ErrLightBlockNotFound)
	require.NoError(t, disk.Close())

	disk, trusted = repair86Open(t, dir, false)
	defer func() { require.NoError(t, disk.Close()) }()
	repair86AssertSet(t, trusted, blocks[1:])
	before, err := trusted.LightBlockBefore(30)
	require.NoError(t, err)
	require.Equal(t, replacement.Hash(), before.Hash())
}

func TestRepair_86_MissingDeleteReopen(t *testing.T) {
	dir := t.TempDir()
	disk, trusted := repair86Open(t, dir, false)
	require.NoError(t, trusted.DeleteLightBlock(99))
	repair86AssertSet(t, trusted, nil)
	require.NoError(t, disk.Close())

	disk, trusted = repair86Open(t, dir, false)
	repair86AssertSet(t, trusted, nil)
	blocks := []*types.LightBlock{repair86Produce(t, 10), repair86Produce(t, 20), repair86Produce(t, 30)}
	for _, block := range blocks {
		require.NoError(t, trusted.SaveLightBlock(block))
	}
	require.NoError(t, trusted.DeleteLightBlock(99))
	require.NoError(t, trusted.DeleteLightBlock(20))
	require.NoError(t, trusted.DeleteLightBlock(20))
	want := []*types.LightBlock{blocks[0], blocks[2]}
	repair86AssertSet(t, trusted, want)
	_, err := trusted.LightBlock(20)
	require.ErrorIs(t, err, store.ErrLightBlockNotFound)
	require.NoError(t, disk.Close())

	disk, trusted = repair86Open(t, dir, false)
	repair86AssertSet(t, trusted, want)
	require.NoError(t, trusted.Prune(1))
	repair86AssertSet(t, trusted, blocks[2:])
	require.NoError(t, trusted.DeleteLightBlock(30))
	require.NoError(t, trusted.DeleteLightBlock(30))
	repair86AssertSet(t, trusted, nil)
	require.NoError(t, disk.Close())

	disk, trusted = repair86Open(t, dir, false)
	defer func() { require.NoError(t, disk.Close()) }()
	repair86AssertSet(t, trusted, nil)
}

func TestRepair_86_FailedWriteReopenAndRetry(t *testing.T) {
	for _, operation := range []string{"insert", "overwrite", "delete", "prune"} {
		t.Run(operation, func(t *testing.T) {
			dir := t.TempDir()
			disk, trusted := repair86Open(t, dir, false)
			blocks := []*types.LightBlock{repair86Produce(t, 10), repair86Produce(t, 20), repair86Produce(t, 30)}
			for _, block := range blocks {
				require.NoError(t, trusted.SaveLightBlock(block))
			}
			require.NoError(t, disk.Close())
			disk, trusted = repair86Open(t, dir, true)
			replacement := repair86ProduceWithAppHash(t, 20, []byte("replacement"))
			inserted := repair86Produce(t, 40)
			apply := func(target store.Store) error {
				switch operation {
				case "insert":
					return target.SaveLightBlock(inserted)
				case "overwrite":
					return target.SaveLightBlock(replacement)
				case "delete":
					return target.DeleteLightBlock(20)
				default:
					return target.Prune(1)
				}
			}
			// Real LevelDB permits reads and batch staging, then rejects WriteSync.
			require.ErrorIs(t, apply(trusted), leveldb.ErrReadOnly)
			repair86AssertSet(t, trusted, blocks)
			_, err := trusted.LightBlock(40)
			require.ErrorIs(t, err, store.ErrLightBlockNotFound)
			require.NoError(t, disk.Close())

			disk, trusted = repair86Open(t, dir, false)
			repair86AssertSet(t, trusted, blocks)
			require.NoError(t, apply(trusted))
			var want []*types.LightBlock
			switch operation {
			case "insert":
				want = append(blocks, inserted)
			case "overwrite":
				want = []*types.LightBlock{blocks[0], replacement, blocks[2]}
			case "delete":
				want = []*types.LightBlock{blocks[0], blocks[2]}
			case "prune":
				want = blocks[2:]
			}
			repair86AssertSet(t, trusted, want)
			require.NoError(t, disk.Close())
			disk, trusted = repair86Open(t, dir, false)
			defer func() { require.NoError(t, disk.Close()) }()
			repair86AssertSet(t, trusted, want)
		})
	}
}

func TestRepair_86_ReadFailurePreservesSize(t *testing.T) {
	dir := t.TempDir()
	disk, trusted := repair86Open(t, dir, false)
	block := repair86Produce(t, 10)
	require.NoError(t, trusted.SaveLightBlock(block))
	require.NoError(t, disk.Close())
	require.ErrorIs(t, trusted.SaveLightBlock(block), leveldb.ErrClosed)
	require.ErrorIs(t, trusted.DeleteLightBlock(10), leveldb.ErrClosed)
	require.ErrorIs(t, trusted.DeleteLightBlock(99), leveldb.ErrClosed)
	require.Equal(t, uint16(1), trusted.Size())
	disk, trusted = repair86Open(t, dir, false)
	defer func() { require.NoError(t, disk.Close()) }()
	repair86AssertSet(t, trusted, []*types.LightBlock{block})
}

func TestRepair_86_CounterOverflow(t *testing.T) {
	backend := dbm.NewMemDB()
	defer func() { require.NoError(t, backend.Close()) }()
	trusted := New(backend)
	for height := int64(1); height <= math.MaxUint16; height++ {
		require.NoError(t, trusted.SaveLightBlock(repair86Produce(t, height)))
	}
	require.Equal(t, uint16(math.MaxUint16), trusted.Size())
	last := repair86ProduceWithAppHash(t, math.MaxUint16, []byte("replacement at capacity"))
	require.NoError(t, trusted.SaveLightBlock(last))
	require.Equal(t, uint16(math.MaxUint16), trusted.Size())
	extra := repair86Produce(t, math.MaxUint16+1)
	require.Error(t, trusted.SaveLightBlock(extra))
	require.Equal(t, uint16(math.MaxUint16), trusted.Size())
	_, err := trusted.LightBlock(extra.Height)
	require.ErrorIs(t, err, store.ErrLightBlockNotFound)
	trusted = New(backend)
	require.Equal(t, uint16(math.MaxUint16), trusted.Size())
	first, err := trusted.FirstLightBlockHeight()
	require.NoError(t, err)
	require.Equal(t, int64(1), first)
	require.NoError(t, trusted.DeleteLightBlock(1))
	require.NoError(t, trusted.SaveLightBlock(extra))
	require.Equal(t, uint16(math.MaxUint16), trusted.Size())
	require.NoError(t, trusted.Prune(2))
	repair86AssertSet(t, trusted, []*types.LightBlock{last, extra})
	trusted = New(backend)
	repair86AssertSet(t, trusted, []*types.LightBlock{last, extra})
}

func repair86Open(t *testing.T, dir string, readOnly bool) (*dbm.GoLevelDB, store.Store) {
	t.Helper()
	disk, err := dbm.NewGoLevelDBWithOpts("repair86", dir, &opt.Options{ReadOnly: readOnly})
	require.NoError(t, err)
	return disk, New(disk)
}

func repair86Produce(t *testing.T, height int64) *types.LightBlock {
	t.Helper()
	return repair86ProduceWithAppHash(t, height, []byte("application state"))
}

func repair86ProduceWithAppHash(t *testing.T, height int64, appHash []byte) *types.LightBlock {
	t.Helper()
	const chainID = "repair86"
	key := ed25519.TestSecretKey([]byte(chainID))
	validators := types.NewValidatorSet([]*types.Validator{types.NewValidator(key.Public(), 10)})
	block := types.MakeBlock(height, nil, &types.Commit{}, nil)
	block.Header.ChainID = chainID
	block.Header.Time = time.Unix(1700000000+height, 0).UTC()
	block.Header.ValidatorsHash = validators.Hash()
	block.Header.NextValidatorsHash = validators.Hash()
	block.Header.ProposerAddress = key.Public().Address()
	block.Header.AppHash = appHash
	parts, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
	vote := &types.Vote{
		Type: tmproto.PrecommitType, Height: height, Round: 0, BlockID: blockID,
		Timestamp: block.Time, ValidatorAddress: key.Public().Address(), ValidatorIndex: 0,
	}
	vote.Signature = utils.Some(key.Sign(types.VoteSignBytes(chainID, vote.ToProto())))
	votes := types.NewVoteSet(chainID, height, 0, tmproto.PrecommitType, validators)
	added, err := votes.AddVote(vote)
	require.NoError(t, err)
	require.True(t, added)
	produced := &types.LightBlock{
		SignedHeader: &types.SignedHeader{Header: &block.Header, Commit: votes.MakeCommit()},
		ValidatorSet: validators,
	}
	require.NoError(t, produced.ValidateBasic(chainID))
	encoded, err := produced.ToProto()
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)
	var incoming tmproto.LightBlock
	require.NoError(t, incoming.Unmarshal(wire))
	decoded, err := types.LightBlockFromProto(&incoming)
	require.NoError(t, err)
	return decoded
}

func repair86AssertSet(t *testing.T, trusted store.Store, want []*types.LightBlock) {
	t.Helper()
	require.Equal(t, uint16(len(want)), trusted.Size())
	first, err := trusted.FirstLightBlockHeight()
	require.NoError(t, err)
	last, err := trusted.LastLightBlockHeight()
	require.NoError(t, err)
	if len(want) == 0 {
		require.Equal(t, int64(-1), first)
		require.Equal(t, int64(-1), last)
		_, err = trusted.LightBlockBefore(math.MaxInt64)
		require.ErrorIs(t, err, store.ErrLightBlockNotFound)
		return
	}
	require.Equal(t, want[0].Height, first)
	require.Equal(t, want[len(want)-1].Height, last)
	for i := len(want) - 1; i >= 0; i-- {
		got, err := trusted.LightBlock(want[i].Height)
		require.NoError(t, err)
		require.NoError(t, got.ValidateBasic("repair86"))
		require.Equal(t, want[i].Hash(), got.Hash())
		require.Equal(t, want[i].Commit, got.Commit)
		require.Equal(t, want[i].ValidatorSet.Hash(), got.ValidatorSet.Hash())
		before, err := trusted.LightBlockBefore(want[i].Height)
		if i == 0 {
			require.ErrorIs(t, err, store.ErrLightBlockNotFound)
		} else {
			require.NoError(t, err)
			require.Equal(t, want[i-1].Height, before.Height)
		}
	}
}
