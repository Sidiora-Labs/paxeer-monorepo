package sequencer_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"math"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	sdkcodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type moduleGenesisEnv struct {
	ctx        sdk.Context
	keeper     keeper.Keeper
	paramSpace paramtypes.Subspace
	module     sequencer.AppModule
	storeKey   *sdk.KVStoreKey
}

func newModuleGenesisEnv(t *testing.T, height int64) moduleGenesisEnv {
	t.Helper()
	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(types.ModuleName)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	tkey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(paramsKey, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkey, sdk.StoreTypeTransient, db)
	require.NoError(t, ms.LoadLatestVersion())
	paramsKeeper := paramskeeper.NewKeeper(sdkcodec.NewProtoCodec(codectypes.NewInterfaceRegistry()), sdkcodec.NewLegacyAmino(), paramsKey, tkey)
	paramSpace := paramsKeeper.Subspace(types.ModuleName)
	k := keeper.NewKeeper(key)
	return moduleGenesisEnv{
		ctx:        sdk.NewContext(ms, tmproto.Header{Height: height}, false),
		keeper:     k,
		paramSpace: paramSpace,
		module:     sequencer.NewAppModule(k, paramSpace),
		storeKey:   key,
	}
}

type moduleGenesisJSON struct {
	Leaders          [][]byte           `json:"leaders,omitempty"`
	ReceiptKeys      keeper.ReceiptKeys `json:"receipt_keys"`
	ActivationHeight uint64             `json:"activation_height,omitempty"`
	LastSeq          uint64             `json:"last_seq,omitempty"`
	LastSeqHash      []byte             `json:"last_seq_hash,omitempty"`
}

func moduleGenesisKeys() keeper.ReceiptKeys {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	return keeper.ReceiptKeys{{
		KeyId:       bytes.Repeat([]byte{9}, types.KeyIDLength),
		PublicKey:   private.Public().(ed25519.PublicKey),
		FirstHeight: 1,
		LastHeight:  math.MaxUint64,
	}}
}

func TestModuleGenesisActivationHeightRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name       string
		activation uint64
		want       uint64
	}{
		{name: "explicit activation", activation: 42, want: 42},
		{name: "legacy genesis without activation height", activation: 0, want: 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newModuleGenesisEnv(t, 10)
			genesis := moduleGenesisJSON{
				Leaders:          [][]byte{bytes.Repeat([]byte{1}, 32)},
				ReceiptKeys:      moduleGenesisKeys(),
				ActivationHeight: tc.activation,
				LastSeq:          7,
				LastSeqHash:      bytes.Repeat([]byte{2}, types.HashLength),
			}
			encoded, err := json.Marshal(genesis)
			require.NoError(t, err)
			require.NoError(t, env.module.ValidateGenesis(nil, nil, encoded))
			env.module.InitGenesis(env.ctx, nil, encoded)
			activation, err := env.keeper.ActivationHeight(env.ctx)
			require.NoError(t, err)
			require.Equal(t, int64(tc.want), activation)

			exported := env.module.ExportGenesis(env.ctx, nil)
			var decoded moduleGenesisJSON
			require.NoError(t, json.Unmarshal(exported, &decoded))
			genesis.ActivationHeight = tc.want
			require.Equal(t, genesis, decoded)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(exported, &fields))
			require.Contains(t, fields, "activation_height")

			restored := newModuleGenesisEnv(t, 8000)
			require.NoError(t, restored.module.ValidateGenesis(nil, nil, exported))
			restored.module.InitGenesis(restored.ctx, nil, exported)
			activation, err = restored.keeper.ActivationHeight(restored.ctx)
			require.NoError(t, err)
			require.Equal(t, int64(tc.want), activation)
			originalState, err := env.keeper.ExportGenesis(env.ctx)
			require.NoError(t, err)
			restoredState, err := restored.keeper.ExportGenesis(restored.ctx)
			require.NoError(t, err)
			require.Equal(t, originalState, restoredState)
			require.JSONEq(t, string(exported), string(restored.module.ExportGenesis(restored.ctx, nil)))
		})
	}

	t.Run("activation without leaders is refused", func(t *testing.T) {
		env := newModuleGenesisEnv(t, 10)
		encoded, err := json.Marshal(moduleGenesisJSON{ReceiptKeys: moduleGenesisKeys(), ActivationHeight: 42})
		require.NoError(t, err)
		require.ErrorIs(t, env.module.ValidateGenesis(nil, nil, encoded), types.ErrInvalidGenesis)
		require.Panics(t, func() { env.module.InitGenesis(env.ctx, nil, encoded) })
		activation, err := env.keeper.ActivationHeight(env.ctx)
		require.NoError(t, err)
		require.Zero(t, activation)
		require.False(t, env.paramSpace.Has(env.ctx, keeper.KeyLeaders))
	})

	t.Run("unactivated genesis omits activation height", func(t *testing.T) {
		env := newModuleGenesisEnv(t, 10)
		genesis := env.module.DefaultGenesis(nil)
		require.NoError(t, env.module.ValidateGenesis(nil, nil, genesis))
		env.module.InitGenesis(env.ctx, nil, genesis)
		exported := env.module.ExportGenesis(env.ctx, nil)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(exported, &fields))
		require.NotContains(t, fields, "activation_height")
		require.JSONEq(t, string(genesis), string(exported))
		require.False(t, env.paramSpace.Has(env.ctx, keeper.KeyLeaders))
	})
}

func TestModuleInitGenesisSetsLeadersParam(t *testing.T) {
	env := newModuleGenesisEnv(t, 10)
	leaders := [][32]byte{{3}, {1}, {2}}
	genesis := moduleGenesisJSON{ReceiptKeys: moduleGenesisKeys(), ActivationHeight: 42}
	var storedLeaders []byte
	for _, leader := range leaders {
		genesis.Leaders = append(genesis.Leaders, bytes.Clone(leader[:]))
		storedLeaders = append(storedLeaders, leader[:]...)
	}
	encoded, err := json.Marshal(genesis)
	require.NoError(t, err)
	require.NoError(t, env.module.ValidateGenesis(nil, nil, encoded))
	env.module.InitGenesis(env.ctx, nil, encoded)
	require.True(t, env.paramSpace.Has(env.ctx, keeper.KeyLeaders))
	require.Equal(t, leaders, keeper.GetLeadersParam(env.ctx, env.paramSpace))
	pinned, err := env.keeper.Leaders(env.ctx)
	require.NoError(t, err)
	require.Equal(t, leaders, pinned)
	require.Equal(t, storedLeaders, env.ctx.KVStore(env.storeKey).Get(types.LeaderListKey))
	require.Equal(t, genesis.ReceiptKeys, keeper.GetReceiptKeys(env.ctx, env.paramSpace))
}
