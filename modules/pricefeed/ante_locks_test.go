package pricefeed

import (
	"encoding/binary"
	"hash/fnv"
	"sync"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestMarkLocksAreBounded(t *testing.T) {
	var table *[256]sync.Mutex = &markLocks
	seen := make(map[*sync.Mutex]struct{})
	for i := uint64(0); i < 65536; i++ {
		operator := make(sdk.ValAddress, 20)
		binary.BigEndian.PutUint64(operator[12:], i)
		hash := fnv.New32a()
		_, err := hash.Write(operator)
		require.NoError(t, err)
		lock := markLockFor(operator)
		require.Same(t, &table[hash.Sum32()%256], lock)
		require.Same(t, lock, markLockFor(append(sdk.ValAddress(nil), operator...)))
		seen[lock] = struct{}{}
	}
	require.Len(t, seen, 256)
}

func TestMarkSubmissionSerialisesOneOperator(t *testing.T) {
	registry := cdctypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	priceKey := sdk.NewKVStoreKey(types.StoreKey)
	stakingKey := sdk.NewKVStoreKey(stakingtypes.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	stores := store.NewCommitMultiStore(dbm.NewMemDB())
	for _, key := range []*sdk.KVStoreKey{priceKey, stakingKey, authKey, paramsKey} {
		stores.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	}
	stores.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, stores.LoadLatestVersion())
	ctx := sdk.NewContext(stores.CacheMultiStore(), tmproto.Header{Height: 49}, true)
	accounts := authkeeper.NewAccountKeeper(cdc, authKey,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, authtypes.ModuleName),
		authtypes.ProtoBaseAccount, map[string][]string{
			stakingtypes.BondedPoolName:    nil,
			stakingtypes.NotBondedPoolName: nil,
		})
	staking := stakingkeeper.NewKeeper(cdc, stakingKey, accounts, nil,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, stakingtypes.ModuleName))
	k := keeper.NewKeeper(priceKey, staking, accounts, nil)
	operator := sdk.ValAddress(secp256k1.GenPrivKeyFromSecret([]byte("mark operator")).PubKey().Address())
	validator, err := stakingtypes.NewValidator(operator,
		ed25519.GenPrivKeyFromSecret([]byte("mark consensus")).PubKey(), stakingtypes.Description{})
	require.NoError(t, err)
	staking.SetValidator(ctx, validator.UpdateStatus(stakingtypes.Bonded))
	feeder, _, err := k.RegisterFeedKey(ctx, operator,
		secp256k1.GenPrivKeyFromSecret([]byte("mark feeder")).PubKey().Bytes())
	require.NoError(t, err)
	builder := authtx.NewTxConfig(cdc, authtx.DefaultSignModes).NewTxBuilder()
	require.NoError(t, builder.SetMsgs(&types.MsgSubmitPrices{Feeder: feeder.String()}))
	tx := builder.GetTx()
	_, exists := k.GetLastSubmissionHeight(ctx, operator)
	require.False(t, exists)

	started := make(chan struct{}, 2)
	results := make(chan error, 2)
	func() {
		lock := markLockFor(operator)
		lock.Lock()
		defer lock.Unlock()
		for i := 0; i < 2; i++ {
			callCtx := ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
			go func() {
				started <- struct{}{}
				results <- MarkSubmission(callCtx, tx, k)
			}()
		}
		<-started
		<-started
		select {
		case err := <-results:
			t.Fatalf("MarkSubmission returned while its stripe was locked: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		_, exists := k.GetLastSubmissionHeight(ctx, operator)
		require.False(t, exists)
	}()

	accepted, limited := 0, 0
	for i := 0; i < 2; i++ {
		select {
		case err := <-results:
			if err == nil {
				accepted++
			} else {
				require.ErrorIs(t, err, types.ErrRateLimited)
				limited++
			}
		case <-time.After(5 * time.Second):
			t.Fatal("MarkSubmission did not release its stripe")
		}
	}
	require.Equal(t, 1, accepted)
	require.Equal(t, 1, limited)
	last, exists := k.GetLastSubmissionHeight(ctx, operator)
	require.True(t, exists)
	require.Equal(t, int64(50), last)
}
