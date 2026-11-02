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
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type moduleGenesisEnv struct {
	ctx          sdk.Context
	keeper       keeper.Keeper
	paramSpace   paramtypes.Subspace
	module       sequencer.AppModule
	storeKey     *sdk.KVStoreKey
	paramsKeeper paramskeeper.Keeper
	handler      govtypes.Handler
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
		ctx:          sdk.NewContext(ms, tmproto.Header{Height: height}, false),
		keeper:       k,
		paramSpace:   paramSpace,
		module:       sequencer.NewAppModule(k, paramSpace),
		storeKey:     key,
		paramsKeeper: paramsKeeper,
		handler:      sequencer.NewParamChangeProposalHandler(k, paramSpace, params.NewParamChangeProposalHandler(paramsKeeper)),
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

func newModuleParamChangeEnv(t *testing.T) moduleGenesisEnv {
	t.Helper()
	env := newModuleGenesisEnv(t, 10)
	genesis := moduleGenesisJSON{
		Leaders:          [][]byte{bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)},
		ReceiptKeys:      moduleGenesisKeys(),
		ActivationHeight: 1,
	}
	encoded, err := json.Marshal(genesis)
	require.NoError(t, err)
	env.module.InitGenesis(env.ctx, nil, encoded)
	return env
}

func moduleParamChange(t *testing.T, key []byte, value interface{}) proposal.ParamChange {
	t.Helper()
	encoded, err := sdkcodec.NewLegacyAmino().MarshalAsJSON(value)
	require.NoError(t, err)
	return proposal.NewParamChange(types.ModuleName, string(key), string(encoded))
}

func TestParamChangeSyncsLeaders(t *testing.T) {
	for _, order := range []string{"leaders only", "leaders then receipt keys", "receipt keys then leaders", "receipt keys only"} {
		t.Run(order, func(t *testing.T) {
			env := newModuleParamChangeEnv(t)
			leaders := keeper.LeaderList{bytes.Repeat([]byte{4}, 32), bytes.Repeat([]byte{3}, 32)}
			wantLeaders := [][32]byte{[32]byte(leaders[0]), [32]byte(leaders[1])}
			keys := moduleGenesisKeys()
			keys[0].LastHeight = 10
			private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{8}, ed25519.SeedSize))
			keys = append(keys, keeper.ReceiptKeyEntry{
				KeyId:       bytes.Repeat([]byte{8}, types.KeyIDLength),
				PublicKey:   private.Public().(ed25519.PublicKey),
				FirstHeight: 11,
				LastHeight:  math.MaxUint64,
			})
			leaderChange := moduleParamChange(t, keeper.KeyLeaders, leaders)
			receiptChange := moduleParamChange(t, keeper.KeyReceiptKeys, keys)
			changes := []proposal.ParamChange{leaderChange}
			switch order {
			case "leaders then receipt keys":
				changes = append(changes, receiptChange)
			case "receipt keys then leaders":
				changes = []proposal.ParamChange{receiptChange, leaderChange}
			case "receipt keys only":
				changes = []proposal.ParamChange{receiptChange}
				wantLeaders = keeper.GetLeadersParam(env.ctx, env.paramSpace)
			}
			if order == "leaders only" {
				keys = moduleGenesisKeys()
			}
			p := proposal.NewParameterChangeProposal("sequencer", "update governed lists", changes, false)
			require.NoError(t, env.handler(env.ctx, p))
			require.Equal(t, wantLeaders, keeper.GetLeadersParam(env.ctx, env.paramSpace))
			pinned, err := env.keeper.Leaders(env.ctx)
			require.NoError(t, err)
			require.Equal(t, wantLeaders, pinned)
			var packed []byte
			for _, leader := range wantLeaders {
				packed = append(packed, leader[:]...)
			}
			require.Equal(t, packed, env.ctx.KVStore(env.storeKey).Get(types.LeaderListKey))
			activation, scheduled, err := env.keeper.LeaderSchedule(env.ctx.WithBlockHeight(11))
			require.NoError(t, err)
			require.Equal(t, int64(1), activation)
			require.Equal(t, wantLeaders, scheduled)
			require.Equal(t, keys, keeper.GetReceiptKeys(env.ctx, env.paramSpace))
			storedKeys, err := env.keeper.ReceiptKeys(env.ctx)
			require.NoError(t, err)
			require.Len(t, storedKeys, len(keys))
			for i, key := range keys {
				require.Equal(t, [32]byte(key.KeyId), storedKeys[i].KeyId)
				require.Equal(t, [32]byte(key.PublicKey), storedKeys[i].PublicKey)
				require.Equal(t, key.FirstHeight, storedKeys[i].FirstHeight)
				require.Equal(t, key.LastHeight, storedKeys[i].LastHeight)
			}
		})
	}

	t.Run("another subspace passes through", func(t *testing.T) {
		env := newModuleParamChangeEnv(t)
		other := env.paramsKeeper.Subspace("other").WithKeyTable(keeper.ParamKeyTable())
		beforeParam := bytes.Clone(env.paramSpace.GetRaw(env.ctx, keeper.KeyLeaders))
		beforeStore := bytes.Clone(env.ctx.KVStore(env.storeKey).Get(types.LeaderListKey))
		leaders := keeper.LeaderList{bytes.Repeat([]byte{5}, 32)}
		change := moduleParamChange(t, keeper.KeyLeaders, leaders)
		change.Subspace = "other"
		p := proposal.NewParameterChangeProposal("other", "update another subspace", []proposal.ParamChange{change}, false)
		require.NoError(t, env.handler(env.ctx, p))
		require.Equal(t, [][32]byte{[32]byte(leaders[0])}, keeper.GetLeadersParam(env.ctx, other))
		require.Equal(t, beforeParam, env.paramSpace.GetRaw(env.ctx, keeper.KeyLeaders))
		require.Equal(t, beforeStore, env.ctx.KVStore(env.storeKey).Get(types.LeaderListKey))
	})
}

func TestParamChangeRefusesInvalidLeaders(t *testing.T) {
	for name, invalid := range map[string]keeper.LeaderList{
		"nil":       nil,
		"empty":     {},
		"zero key":  {make([]byte, 32)},
		"short key": {bytes.Repeat([]byte{3}, 31)},
		"long key":  {bytes.Repeat([]byte{3}, 33)},
		"duplicate": {bytes.Repeat([]byte{3}, 32), bytes.Repeat([]byte{3}, 32)},
	} {
		for _, combined := range []bool{false, true} {
			label := "leaders only"
			if combined {
				label = "with receipt keys"
			}
			t.Run(name+"/"+label, func(t *testing.T) {
				env := newModuleParamChangeEnv(t)
				beforeParam := bytes.Clone(env.paramSpace.GetRaw(env.ctx, keeper.KeyLeaders))
				beforeLeaders := bytes.Clone(env.ctx.KVStore(env.storeKey).Get(types.LeaderListKey))
				beforeReceiptParam := bytes.Clone(env.paramSpace.GetRaw(env.ctx, keeper.KeyReceiptKeys))
				beforeReceipts := bytes.Clone(env.ctx.KVStore(env.storeKey).Get(types.ReceiptKeysKey))
				changes := []proposal.ParamChange{
					moduleParamChange(t, keeper.KeyLeaders, keeper.LeaderList{bytes.Repeat([]byte{4}, 32)}),
				}
				if combined {
					keys := moduleGenesisKeys()
					keys[0].LastHeight = 10
					changes = append(changes, moduleParamChange(t, keeper.KeyReceiptKeys, keys))
				}
				changes = append(changes, moduleParamChange(t, keeper.KeyLeaders, invalid))
				p := proposal.NewParameterChangeProposal("sequencer", "refuse invalid leaders", changes, false)
				err := env.handler(env.ctx, p)
				require.ErrorIs(t, err, proposal.ErrSettingParameter)
				require.ErrorContains(t, err, types.ErrInvalidGenesis.Error())
				require.Equal(t, beforeParam, env.paramSpace.GetRaw(env.ctx, keeper.KeyLeaders))
				require.Equal(t, beforeLeaders, env.ctx.KVStore(env.storeKey).Get(types.LeaderListKey))
				require.Equal(t, beforeReceiptParam, env.paramSpace.GetRaw(env.ctx, keeper.KeyReceiptKeys))
				require.Equal(t, beforeReceipts, env.ctx.KVStore(env.storeKey).Get(types.ReceiptKeysKey))
				activation, scheduled, err := env.keeper.LeaderSchedule(env.ctx.WithBlockHeight(11))
				require.NoError(t, err)
				require.Equal(t, int64(1), activation)
				require.Equal(t, keeper.GetLeadersParam(env.ctx, env.paramSpace), scheduled)
			})
		}
	}

	t.Run("invalid receipt rotation rolls back leaders", func(t *testing.T) {
		env := newModuleParamChangeEnv(t)
		beforeParam := bytes.Clone(env.paramSpace.GetRaw(env.ctx, keeper.KeyLeaders))
		beforeLeaders := bytes.Clone(env.ctx.KVStore(env.storeKey).Get(types.LeaderListKey))
		beforeReceiptParam := bytes.Clone(env.paramSpace.GetRaw(env.ctx, keeper.KeyReceiptKeys))
		beforeReceipts := bytes.Clone(env.ctx.KVStore(env.storeKey).Get(types.ReceiptKeysKey))
		keys := moduleGenesisKeys()
		keys[0].FirstHeight = 2
		p := proposal.NewParameterChangeProposal("sequencer", "refuse rewriting a pinned receipt key", []proposal.ParamChange{
			moduleParamChange(t, keeper.KeyLeaders, keeper.LeaderList{bytes.Repeat([]byte{4}, 32)}),
			moduleParamChange(t, keeper.KeyReceiptKeys, keys),
		}, false)
		require.ErrorIs(t, env.handler(env.ctx, p), keeper.ErrReceiptKeyRotation)
		require.Equal(t, beforeParam, env.paramSpace.GetRaw(env.ctx, keeper.KeyLeaders))
		require.Equal(t, beforeLeaders, env.ctx.KVStore(env.storeKey).Get(types.LeaderListKey))
		require.Equal(t, beforeReceiptParam, env.paramSpace.GetRaw(env.ctx, keeper.KeyReceiptKeys))
		require.Equal(t, beforeReceipts, env.ctx.KVStore(env.storeKey).Get(types.ReceiptKeysKey))
	})
}
