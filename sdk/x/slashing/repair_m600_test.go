package slashing_test

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence"
	evidencekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/keeper"
	evidencetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// All peers are production keepers over local persisted fixtures. Reopening
// constructs a fresh multistore, codec and keepers over the closed database.
type repairM600Store struct {
	cms sdk.CommitMultiStore
	ctx sdk.Context
	cdc codec.Codec
	k   keeper.Keeper
	sk  stakingkeeper.Keeper
	bk  bankkeeper.BaseKeeper
	ek  *evidencekeeper.Keeper
}

func openRepairM600Store(t *testing.T, dir string) *repairM600Store {
	t.Helper()
	db, err := dbm.NewGoLevelDB("slashing", dir)
	require.NoError(t, err)
	cms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(types.StoreKey, stakingtypes.StoreKey, authtypes.StoreKey, banktypes.StoreKey, paramtypes.StoreKey, evidencetypes.StoreKey)
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	}
	tkey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	cms.MountStoreWithDB(tkey, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1, Time: time.Unix(1000, 0).UTC()}, false)
	enc := paxapp.MakeEncodingConfig()
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(enc.Marshaler, enc.Amino, keys[paramtypes.StoreKey], tkey, name)
	}
	ak := authkeeper.NewAccountKeeper(enc.Marshaler, keys[authtypes.StoreKey], subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{stakingtypes.BondedPoolName: {authtypes.Minter, authtypes.Burner}, stakingtypes.NotBondedPoolName: {authtypes.Burner}})
	bk := bankkeeper.NewBaseKeeper(enc.Marshaler, keys[banktypes.StoreKey], ak, subspace(banktypes.ModuleName), map[string]bool{})
	sk := stakingkeeper.NewKeeper(enc.Marshaler, keys[stakingtypes.StoreKey], ak, bk, subspace(stakingtypes.ModuleName))
	k := keeper.NewKeeper(enc.Marshaler, keys[types.StoreKey], sk, subspace(types.ModuleName))
	ek := evidencekeeper.NewKeeper(enc.Marshaler, keys[evidencetypes.StoreKey], sk, k)
	if cms.LastCommitID().Version == 0 {
		ak.SetParams(ctx, authtypes.DefaultParams())
		bk.SetParams(ctx, banktypes.DefaultParams())
		sk.SetParams(ctx, stakingtypes.DefaultParams())
		k.SetParams(ctx, types.DefaultParams())
	}
	return &repairM600Store{cms, ctx, enc.Marshaler, k, sk, bk, ek}
}

func (s *repairM600Store) validator(t *testing.T) cryptotypes.PubKey {
	t.Helper()
	pk := ed25519.GenPrivKey().PubKey()
	v, err := stakingtypes.NewValidator(sdk.ValAddress(pk.Address()), pk, stakingtypes.Description{})
	require.NoError(t, err)
	v.Status = stakingtypes.Bonded
	v.Tokens = s.sk.TokensFromConsensusPower(s.ctx, 100)
	v.DelegatorShares = v.Tokens.ToDec()
	s.sk.SetValidator(s.ctx, v)
	require.NoError(t, s.sk.SetValidatorByConsAddr(s.ctx, v))
	s.sk.SetValidatorByPowerIndex(s.ctx, v)
	s.sk.SetDelegation(s.ctx, stakingtypes.NewDelegation(sdk.AccAddress(v.GetOperator()), v.GetOperator(), v.DelegatorShares))
	require.NoError(t, s.bk.MintCoins(s.ctx, stakingtypes.BondedPoolName, sdk.NewCoins(sdk.NewCoin(s.sk.BondDenom(s.ctx), v.Tokens))))
	require.NoError(t, s.k.AfterValidatorCreated(s.ctx, v.GetOperator()))
	s.k.AfterValidatorBonded(s.ctx, sdk.ConsAddress(pk.Address()), v.GetOperator())
	return pk
}

func (s *repairM600Store) window(window int64, fraction sdk.Dec) {
	params := s.k.GetParams(s.ctx)
	params.SignedBlocksWindow = window
	params.MinSignedPerWindow = fraction
	s.k.SetParams(s.ctx, params)
}

func repairM600Vote(pk cryptotypes.PubKey, signed bool) abci.VoteInfo {
	return abci.VoteInfo{Validator: abci.Validator{Address: pk.Address(), Power: 100}, SignedLastBlock: signed}
}

func (s *repairM600Store) vote(pk cryptotypes.PubKey, signed bool) {
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1).WithEventManager(sdk.NewEventManager())
	slashing.BeginBlocker(s.ctx, []abci.VoteInfo{repairM600Vote(pk, signed)}, s.k)
}

func assertRepairM600Window(t *testing.T, s *repairM600Store, pk cryptotypes.PubKey, want []bool, index int64) {
	t.Helper()
	addr := sdk.ConsAddress(pk.Address())
	array, found := s.k.GetValidatorMissedBlocks(s.ctx, addr)
	require.True(t, found)
	info, found := s.k.GetValidatorSigningInfo(s.ctx, addr)
	require.True(t, found)
	require.Equal(t, addr.String(), array.Address)
	require.Equal(t, int64(len(want)), array.WindowSize)
	require.Len(t, array.MissedBlocks, (len(want)+63)/64)
	var count int64
	for i, missed := range want {
		require.Equal(t, missed, array.MissedBlocks[i/64]&(uint64(1)<<uint(i%64)) != 0, "slot %d", i)
		if missed {
			count++
		}
	}
	var population int64
	for _, word := range array.MissedBlocks {
		population += int64(bits.OnesCount64(word))
	}
	require.Equal(t, count, population, "padding bits must remain clear")
	require.Equal(t, count, info.MissedBlocksCounter)
	require.Equal(t, index, info.IndexOffset)
	// Inspect the actual persisted key and protobuf independently of the getter.
	key := append([]byte{0x02, byte(len(addr))}, addr...)
	require.Equal(t, key, types.ValidatorMissedBlockBitArrayKey(addr))
	kv := s.ctx.KVStore(s.k.GetStoreKey())
	var decoded types.ValidatorMissedBlockArray
	require.NoError(t, s.cdc.Unmarshal(kv.Get(key), &decoded))
	require.Equal(t, array, decoded)
	iter := sdk.KVStorePrefixIterator(kv, key)
	defer iter.Close()
	require.True(t, iter.Valid())
	require.Equal(t, key, iter.Key())
	iter.Next()
	require.False(t, iter.Valid(), "a validator has only one packed bitmap key")
}

func TestRepairM_600_ReadmeBitmap(t *testing.T) {
	for _, window := range []int64{1, 64, 65, 129} {
		t.Run(fmt.Sprintf("window_%d", window), func(t *testing.T) {
			s := openRepairM600Store(t, t.TempDir())
			defer func() { require.NoError(t, s.cms.Close()) }()
			s.window(window, sdk.ZeroDec())
			pk := s.validator(t)
			_, found := s.k.GetValidatorMissedBlocks(s.ctx, sdk.ConsAddress(pk.Address()))
			require.False(t, found, "allocation is deferred until the first vote")
			want := make([]bool, window)
			for n := int64(0); n < 2*window+1; n++ {
				missed := n%3 != 1
				s.vote(pk, !missed)
				want[n%window] = missed
				assertRepairM600Window(t, s, pk, want, (n+1)%window)
			}
		})
	}
}

func TestRepairM_600_EventOrder(t *testing.T) {
	s := openRepairM600Store(t, t.TempDir())
	defer func() { require.NoError(t, s.cms.Close()) }()
	s.window(2, sdk.NewDecWithPrec(5, 1))
	votes := make([]abci.VoteInfo, 7)
	for i := range votes {
		pk := s.validator(t)
		s.vote(pk, false)
		votes[i] = repairM600Vote(pk, i%3 == 1)
	}
	// Each execution starts from identical real persisted keeper state. No
	// worker ordering or EventManager is replaced by a test double.
	var first []abci.Event
	for repetition := 0; repetition < 32; repetition++ {
		ctx, _ := s.ctx.CacheContext()
		ctx = ctx.WithBlockHeight(100)
		var want sdk.Events
		for _, vote := range votes {
			if vote.SignedLastBlock {
				continue
			}
			addr := sdk.ConsAddress(vote.Validator.Address)
			want = append(want,
				sdk.NewEvent(types.EventTypeLiveness,
					sdk.NewAttribute(types.AttributeKeyAddress, addr.String()),
					sdk.NewAttribute(types.AttributeKeyMissedBlocks, "2"),
					sdk.NewAttribute(types.AttributeKeyHeight, "100")),
				sdk.NewEvent(types.EventTypeSlash,
					sdk.NewAttribute(types.AttributeKeyAddress, addr.String()),
					sdk.NewAttribute(types.AttributeKeyPower, "100"),
					sdk.NewAttribute(types.AttributeKeyReason, types.AttributeValueMissingSignature),
					sdk.NewAttribute(types.AttributeKeyJailed, addr.String())))
		}
		slashing.BeginBlocker(ctx, votes, s.k)
		actual := ctx.EventManager().ABCIEvents()
		require.Equal(t, want.ToABCIEvents(), actual, "complete stream, repetition %d", repetition)
		if repetition == 0 {
			first = actual
		} else {
			require.Equal(t, first, actual)
		}
		for _, vote := range votes {
			addr := sdk.ConsAddress(vote.Validator.Address)
			info, found := s.k.GetValidatorSigningInfo(ctx, addr)
			require.True(t, found)
			if vote.SignedLastBlock {
				require.Equal(t, int64(1), info.MissedBlocksCounter)
			} else {
				require.Zero(t, info.MissedBlocksCounter)
			}
			v, found := s.sk.GetValidatorByConsAddr(ctx, addr)
			require.True(t, found)
			require.Equal(t, !vote.SignedLastBlock, v.Jailed)
			if !vote.SignedLastBlock {
				require.Zero(t, info.IndexOffset)
				_, found := s.k.GetValidatorMissedBlocks(ctx, addr)
				require.False(t, found)
			}
		}
	}
}

func TestRepairM_600_StateLayout(t *testing.T) {
	dir := t.TempDir()
	s := openRepairM600Store(t, dir)
	defer func() { require.NoError(t, s.cms.Close()) }()
	s.window(65, sdk.ZeroDec())
	pk := s.validator(t)
	addr := sdk.ConsAddress(pk.Address())
	kv := s.ctx.KVStore(s.k.GetStoreKey())
	// Version 2's on-disk input: boolean wrappers under address+LE64 slot
	// keys. Feed both maintained migrations, then the current signature path.
	info, found := s.k.GetValidatorSigningInfo(s.ctx, addr)
	require.True(t, found)
	info.MissedBlocksCounter = 2
	s.k.SetValidatorSigningInfo(s.ctx, addr, info)
	var legacyKeys [][]byte
	for _, slot := range []uint64{0, 64} {
		key := types.ValidatorMissedBlockBitArrayKey(addr)
		var suffix [8]byte
		binary.LittleEndian.PutUint64(suffix[:], slot)
		key = append(key, suffix[:]...)
		kv.Set(key, s.cdc.MustMarshal(&gogotypes.BoolValue{Value: true}))
		legacyKeys = append(legacyKeys, key)
	}
	s.ctx = s.ctx.WithBlockHeight(66)
	m := keeper.NewMigrator(s.k)
	require.NoError(t, m.Migrate2to3(s.ctx))
	for _, key := range legacyKeys {
		require.False(t, kv.Has(key))
	}
	var heights types.ValidatorMissedBlockArrayLegacyMissedHeights
	require.NoError(t, s.cdc.Unmarshal(kv.Get(types.ValidatorMissedBlockBitArrayKey(addr)), &heights))
	require.Equal(t, []int64{1, 65}, heights.MissedHeights)
	require.NoError(t, m.Migrate3to4(s.ctx))
	want := make([]bool, 65)
	want[0], want[64] = true, true
	assertRepairM600Window(t, s, pk, want, 64)
	s.vote(pk, true)
	want[64] = false
	assertRepairM600Window(t, s, pk, want, 0)
	commit := s.cms.Commit(true)
	height := s.ctx.BlockHeight()
	require.NoError(t, s.cms.Close())
	s = openRepairM600Store(t, dir)
	require.Equal(t, commit, s.cms.LastCommitID())
	s.ctx = s.ctx.WithBlockHeight(height)
	assertRepairM600Window(t, s, pk, want, 0)
	pubkey, err := s.k.GetPubkey(s.ctx, pk.Address())
	require.NoError(t, err)
	require.True(t, pk.Equals(pubkey))
	s.vote(pk, true)
	want[0] = false
	assertRepairM600Window(t, s, pk, want, 1)
}

func TestRepairM_600_BeginBlockAlgorithm(t *testing.T) {
	s := openRepairM600Store(t, t.TempDir())
	defer func() { require.NoError(t, s.cms.Close()) }()
	s.window(65, sdk.ZeroDec())
	pk := ed25519.GenPrivKey().PubKey()
	addr := sdk.ConsAddress(pk.Address())
	require.PanicsWithValue(t, fmt.Sprintf("Validator consensus-address %s not found", addr), func() {
		s.k.HandleValidatorSignatureConcurrent(s.ctx, pk.Address(), 100, false)
	})
	require.NoError(t, s.k.AddPubkey(s.ctx, pk))
	require.PanicsWithValue(t, fmt.Sprintf("Expected signing info for validator %s but not found", addr), func() {
		s.k.HandleValidatorSignatureConcurrent(s.ctx, pk.Address(), 100, false)
	})
	s.k.AfterValidatorBonded(s.ctx, addr, sdk.ValAddress(pk.Address()))
	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	_, _, pending, _, _, events := s.k.HandleValidatorSignatureConcurrent(s.ctx, pk.Address(), 100, false)
	require.Len(t, events, 1)
	require.Equal(t, int64(1), pending.MissedBlocksCounter)
	require.Empty(t, s.ctx.EventManager().Events(), "calculation must not emit")
	stored, found := s.k.GetValidatorSigningInfo(s.ctx, addr)
	require.True(t, found)
	require.Zero(t, stored.MissedBlocksCounter)
	_, found = s.k.GetValidatorMissedBlocks(s.ctx, addr)
	require.False(t, found, "calculation must not persist")
	want := make([]bool, 65)
	for n := int64(0); n < 129; n++ {
		missed := n%65%2 == 0
		s.vote(pk, !missed)
		want[n%65] = missed
	}
	assertRepairM600Window(t, s, pk, want, 64)
	// 65 -> 66 changes the window within two packed words. The final old
	// slot is retained after the newly inserted slot at the write offset.
	s.window(66, sdk.ZeroDec())
	expanded := make([]bool, 66)
	copy(expanded, want[:64])
	expanded[65] = want[64]
	s.vote(pk, false)
	expanded[64] = true
	assertRepairM600Window(t, s, pk, expanded, 65)
	s.window(64, sdk.ZeroDec())
	s.vote(pk, true)
	assertRepairM600Window(t, s, pk, make([]bool, 64), 1)
}

func TestRepairM_600_TombstoneDoc(t *testing.T) {
	s := openRepairM600Store(t, t.TempDir())
	defer func() { require.NoError(t, s.cms.Close()) }()
	pk := s.validator(t)
	addr := sdk.ConsAddress(pk.Address())
	valAddr := sdk.ValAddress(pk.Address())
	s.ctx = s.ctx.WithBlockHeight(10).WithEventManager(sdk.NewEventManager())
	misbehavior := abci.Misbehavior{Type: abci.MisbehaviorType_DUPLICATE_VOTE,
		Validator: abci.Validator{Address: pk.Address(), Power: 100}, Height: 2, Time: s.ctx.BlockTime()}
	evidence.BeginBlocker(s.ctx, []abci.Misbehavior{misbehavior}, *s.ek)
	info, found := s.k.GetValidatorSigningInfo(s.ctx, addr)
	require.True(t, found)
	require.True(t, info.Tombstoned)
	require.True(t, s.k.IsTombstoned(s.ctx, addr))
	require.True(t, evidencetypes.DoubleSignJailEndTime.Equal(info.JailedUntil))
	v, found := s.sk.GetValidatorByConsAddr(s.ctx, addr)
	require.True(t, found)
	require.True(t, v.Jailed)
	storedEvidence, found := s.ek.GetEvidence(s.ctx, evidencetypes.FromABCIEvidence(abci.Evidence(misbehavior)).Hash())
	require.True(t, found)
	require.Equal(t, int64(2), storedEvidence.GetHeight())
	eventCount := len(s.ctx.EventManager().Events())
	evidence.BeginBlocker(s.ctx, []abci.Misbehavior{misbehavior}, *s.ek)
	require.Len(t, s.ctx.EventManager().Events(), eventCount, "already tombstoned evidence is ignored")
	require.PanicsWithValue(t, "cannot tombstone validator that is already tombstoned", func() { s.k.Tombstone(s.ctx, addr) })
	// Prove that the permanent flag, independently of the jail deadline,
	// rejects the real MsgUnjail route after every time restriction expires.
	s.ctx = s.ctx.WithBlockTime(evidencetypes.DoubleSignJailEndTime.Add(time.Second)).WithEventManager(sdk.NewEventManager())
	response, err := keeper.NewMsgServerImpl(s.k).Unjail(sdk.WrapSDKContext(s.ctx), types.NewMsgUnjail(valAddr))
	require.ErrorIs(t, err, types.ErrValidatorJailed)
	require.Nil(t, response)
	require.Empty(t, s.ctx.EventManager().Events())
	v, found = s.sk.GetValidatorByConsAddr(s.ctx, addr)
	require.True(t, found)
	require.True(t, v.Jailed)

	// The recoverable downtime path over the same real keepers has no flag.
	s.ctx = s.ctx.WithBlockTime(time.Unix(2000, 0).UTC())
	downtimePK := s.validator(t)
	downtimeAddr := sdk.ConsAddress(downtimePK.Address())
	s.window(1, sdk.OneDec())
	s.ctx = s.ctx.WithBlockHeight(100)
	s.vote(downtimePK, false)
	info, found = s.k.GetValidatorSigningInfo(s.ctx, downtimeAddr)
	require.True(t, found)
	require.False(t, info.Tombstoned)
	msg := types.NewMsgUnjail(sdk.ValAddress(downtimePK.Address()))
	_, err = keeper.NewMsgServerImpl(s.k).Unjail(sdk.WrapSDKContext(s.ctx), msg)
	require.ErrorIs(t, err, types.ErrValidatorJailed)
	s.ctx = s.ctx.WithBlockTime(info.JailedUntil).WithEventManager(sdk.NewEventManager())
	response, err = keeper.NewMsgServerImpl(s.k).Unjail(sdk.WrapSDKContext(s.ctx), msg)
	require.NoError(t, err)
	require.NotNil(t, response)
	v, found = s.sk.GetValidatorByConsAddr(s.ctx, downtimeAddr)
	require.True(t, found)
	require.False(t, v.Jailed)
	require.Equal(t, sdk.Events{sdk.NewEvent(sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
		sdk.NewAttribute(sdk.AttributeKeySender, msg.ValidatorAddr))}, s.ctx.EventManager().Events())
}
