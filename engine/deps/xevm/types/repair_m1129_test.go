package types_test

import (
	"math"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func newRepairM1129Keeper(t *testing.T) (*evmkeeper.Keeper, sdk.Context, *codec.ProtoCodec) {
	t.Helper()
	key := sdk.NewKVStoreKey("repair_m1129")
	authKey := sdk.NewKVStoreKey("repair_m1129_auth")
	paramsKey := sdk.NewKVStoreKey("repair_m1129_params")
	tkey := sdk.NewTransientStoreKey("repair_m1129_transient")
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	cms := store.NewCommitMultiStore(db)
	for _, storeKey := range []*sdk.KVStoreKey{key, authKey, paramsKey} {
		cms.MountStoreWithDB(storeKey, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(tkey, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{}, false)
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	accounts := authkeeper.NewAccountKeeper(cdc, authKey,
		paramtypes.NewSubspace(cdc, amino, paramsKey, tkey, "auth"), authtypes.ProtoBaseAccount,
		map[string][]string{
			types.ModuleName:           {authtypes.Minter, authtypes.Burner},
			authtypes.FeeCollectorName: nil,
		})
	params := paramtypes.NewSubspace(cdc, amino, paramsKey, tkey, "evm")
	k := evmkeeper.NewKeeper(key, tkey, params, nil, nil, &accounts, nil, transferkeeper.Keeper{}, nil, nil, nil)
	k.UseRegularStore = true
	k.InitGenesis(ctx, *types.DefaultGenesis())
	return k, ctx, cdc
}

func repairM1129WireStream(t *testing.T, cdc *codec.ProtoCodec, chunks []types.GenesisState) error {
	t.Helper()
	decoded := make([]types.GenesisState, len(chunks))
	for i := range chunks {
		wire, err := cdc.MarshalAsJSON(&chunks[i])
		require.NoError(t, err)
		require.NoError(t, cdc.UnmarshalAsJSON(wire, &decoded[i]))
	}
	stream := make(chan types.GenesisState)
	sent := make(chan struct{})
	go func() {
		defer close(sent)
		defer close(stream)
		for _, chunk := range decoded {
			stream <- chunk
		}
	}()
	result := make(chan error, 1)
	go func() { result <- types.ValidateStream(stream) }()
	select {
	case err := <-result:
		select {
		case <-sent:
			return err
		case <-time.After(5 * time.Second):
			t.Fatal("validator returned before draining the importer stream")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stream validation did not finish")
	}
	return nil
}

func TestRepairM_1129_StreamImport(t *testing.T) {
	source, sourceCtx, cdc := newRepairM1129Keeper(t)
	params := types.DefaultParams()
	params.MinimumFeePerGas = sdk.NewDec(11)
	params.MaximumFeePerGas = sdk.NewDec(19)
	params.TargetGasUsedPerBlock = 500
	params.WhitelistedCwCodeHashesForDelegateCall = [][]byte{common.HexToHash("0x1129").Bytes()}
	require.NoError(t, params.Validate())
	source.SetParams(sourceCtx, params)
	paxAddr := authtypes.NewModuleAddress("repair_m1129_source")
	evmAddr := common.HexToAddress("0x1129000000000000000000000000000000000001")
	source.SetAddressMapping(sourceCtx, paxAddr, evmAddr)
	first := *types.DefaultGenesis()
	first.Params = source.GetParams(sourceCtx)
	second := *types.DefaultGenesis()
	second.Params = source.GetParams(sourceCtx)
	source.IteratePaxAddressMapping(sourceCtx, func(evmAddr common.Address, paxAddr sdk.AccAddress) bool {
		decoded, err := sdk.AccAddressFromBech32(paxAddr.String())
		require.NoError(t, err)
		require.Equal(t, paxAddr, decoded)
		second.AddressAssociations = append(second.AddressAssociations, &types.AddressAssociation{
			PaxAddress: paxAddr.String(), EthAddress: evmAddr.Hex(),
		})
		return false
	})
	require.NotEmpty(t, second.AddressAssociations)
	invalid := first
	invalid.Params.MinimumFeePerGas = sdk.NewDec(-1)
	for _, tc := range []struct {
		name   string
		chunks []types.GenesisState
		valid  bool
	}{
		{"single", []types.GenesisState{first}, true},
		{"repeated_params_and_state", []types.GenesisState{first, second}, true},
		{"invalid_only", []types.GenesisState{invalid}, false},
		{"valid_invalid", []types.GenesisState{first, invalid}, false},
		{"invalid_valid", []types.GenesisState{invalid, second}, false},
		{"valid_invalid_valid", []types.GenesisState{first, invalid, second}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := repairM1129WireStream(t, cdc, tc.chunks)
			if !tc.valid {
				require.ErrorContains(t, err, "invalid genesis chunk")
				return
			}
			require.NoError(t, err)
			dest, destCtx, destCodec := newRepairM1129Keeper(t)
			require.NotEqual(t, params, dest.GetParams(destCtx))
			require.NotEqual(t, evmAddr, dest.GetEVMAddressOrDefault(destCtx, paxAddr))
			for _, chunk := range tc.chunks {
				var imported types.GenesisState
				require.NoError(t, destCodec.UnmarshalAsJSON(cdc.MustMarshalJSON(&chunk), &imported))
				dest.InitGenesis(destCtx, imported)
			}
			require.Equal(t, source.GetParams(sourceCtx), dest.GetParams(destCtx))
			for _, chunk := range tc.chunks {
				for _, aa := range chunk.AddressAssociations {
					address, err := sdk.AccAddressFromBech32(aa.PaxAddress)
					require.NoError(t, err)
					require.Equal(t, aa.EthAddress, dest.GetEVMAddressOrDefault(destCtx, address).Hex())
				}
			}
			if len(tc.chunks) == 2 {
				require.Equal(t, evmAddr, dest.GetEVMAddressOrDefault(destCtx, paxAddr))
			}
		})
	}
}

func TestRepairM_1129_StreamMalformedAddress(t *testing.T) {
	_, _, cdc := newRepairM1129Keeper(t)
	valid := *types.DefaultGenesis()
	paxAddress := authtypes.NewModuleAddress("repair_m1129_valid").String()
	ethAddress := common.HexToAddress("0x1129").Hex()
	malformed := sdk.AccAddress(make([]byte, 59)).String()
	require.Len(t, malformed, 105)
	_, err := sdk.AccAddressFromBech32(malformed)
	require.Error(t, err)
	for _, tc := range []struct {
		name, pax, eth, want string
	}{
		{"overlong_bech32", malformed, ethAddress, "Pax address"},
		{"empty_bech32", "", ethAddress, "Pax address"},
		{"bad_checksum", paxAddress[:len(paxAddress)-1] + "!", ethAddress, "Pax address"},
		{"malformed_hex", paxAddress, "0xzz", "Ethereum address"},
		{"empty_hex", paxAddress, "", "Ethereum address"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := valid
			invalid.AddressAssociations = []*types.AddressAssociation{{PaxAddress: tc.pax, EthAddress: tc.eth}}
			require.ErrorContains(t, invalid.Validate(), tc.want)
			for _, chunks := range [][]types.GenesisState{
				{invalid}, {valid, invalid}, {invalid, valid}, {valid, invalid, valid},
			} {
				require.ErrorContains(t, repairM1129WireStream(t, cdc, chunks), tc.want)
			}
		})
	}
	invalid := valid
	invalid.AddressAssociations = []*types.AddressAssociation{nil}
	require.ErrorContains(t, invalid.Validate(), "missing address association")
	stream := make(chan types.GenesisState, 2)
	stream <- invalid
	stream <- valid
	close(stream)
	require.ErrorContains(t, types.ValidateStream(stream), "missing address association")
}

func TestRepairM_1129_StreamMissingParams(t *testing.T) {
	_, _, cdc := newRepairM1129Keeper(t)
	require.Error(t, types.ValidateStream(nil))
	require.ErrorContains(t, repairM1129WireStream(t, cdc, nil), "empty genesis stream")
	valid := *types.DefaultGenesis()
	missing := types.GenesisState{}
	partial := valid
	partial.Params.MaximumFeePerGas = sdk.Dec{}
	for _, chunks := range [][]types.GenesisState{
		{missing}, {valid, missing}, {missing, valid}, {valid, partial}, {partial, valid},
	} {
		require.Error(t, repairM1129WireStream(t, cdc, chunks))
	}
	require.Error(t, missing.Validate())
	for _, clear := range []func(*types.Params){
		func(p *types.Params) { p.PriorityNormalizer = sdk.Dec{} },
		func(p *types.Params) { p.BaseFeePerGas = sdk.Dec{} },
		func(p *types.Params) { p.MinimumFeePerGas = sdk.Dec{} },
		func(p *types.Params) { p.MaximumFeePerGas = sdk.Dec{} },
		func(p *types.Params) { p.MaxDynamicBaseFeeUpwardAdjustment = sdk.Dec{} },
		func(p *types.Params) { p.MaxDynamicBaseFeeDownwardAdjustment = sdk.Dec{} },
	} {
		p := types.DefaultParams()
		clear(&p)
		require.Error(t, p.Validate())
	}
}

func TestRepairM_1129_GasTargetGovernance(t *testing.T) {
	k, ctx, _ := newRepairM1129Keeper(t)
	key := types.KeyTargetGasUsedPerBlock
	for _, value := range []interface{}{nil, int64(1), "1", uint32(1), uint64(math.MaxInt64) + 1, uint64(math.MaxUint64)} {
		require.Error(t, k.Paramstore.Validate(ctx, key, value), "value %v (%T)", value, value)
	}
	amino := codec.NewLegacyAmino()
	for _, target := range []uint64{math.MaxInt64 + 1, math.MaxUint64} {
		wire, err := amino.MarshalAsJSON(target)
		require.NoError(t, err)
		require.Error(t, k.Paramstore.Update(ctx, key, wire))
		require.Equal(t, types.DefaultTargetGasUsedPerBlock, k.GetTargetGasUsedPerBlock(ctx))
		genesis := types.DefaultGenesis()
		genesis.Params.TargetGasUsedPerBlock = target
		require.Error(t, genesis.Validate())
	}
	for _, target := range []uint64{0, 1, math.MaxInt64} {
		wire, err := amino.MarshalAsJSON(target)
		require.NoError(t, err)
		require.NoError(t, k.Paramstore.Update(ctx, key, wire))
		require.Equal(t, target, k.GetTargetGasUsedPerBlock(ctx))
		require.NoError(t, k.GetParams(ctx).Validate())
	}
	for _, legacy := range []paramtypes.ParamSet{&types.ParamsPreV601{}, &types.ParamsPreV606{}} {
		found := 0
		for _, pair := range legacy.ParamSetPairs() {
			if string(pair.Key) == string(key) {
				found++
				require.Error(t, pair.ValidatorFn(uint64(math.MaxUint64)))
				require.Error(t, pair.ValidatorFn("1"))
				require.NoError(t, pair.ValidatorFn(uint64(math.MaxInt64)))
			}
		}
		require.Equal(t, 1, found)
	}
}

func TestRepairM_1129_GenesisFeeBounds(t *testing.T) {
	_, _, cdc := newRepairM1129Keeper(t)
	for _, tc := range []struct {
		min, max int64
		valid    bool
	}{
		{0, 0, true}, {10, 10, true}, {10, 11, true}, {11, 10, false},
	} {
		genesis := types.DefaultGenesis()
		genesis.Params.MinimumFeePerGas = sdk.NewDec(tc.min)
		genesis.Params.MaximumFeePerGas = sdk.NewDec(tc.max)
		var imported types.GenesisState
		require.NoError(t, cdc.UnmarshalAsJSON(cdc.MustMarshalJSON(genesis), &imported))
		if tc.valid {
			require.NoError(t, imported.Validate())
		} else {
			require.ErrorContains(t, imported.Validate(), "minimum fee cannot be higher than maximum fee")
			require.Error(t, repairM1129WireStream(t, cdc, []types.GenesisState{*types.DefaultGenesis(), imported}))
		}
	}
}

func TestRepairM_1129_DynamicFee(t *testing.T) {
	k, ctx, cdc := newRepairM1129Keeper(t)
	ctx = ctx.WithConsensusParams(&tmproto.ConsensusParams{Block: &tmproto.BlockParams{MaxGas: 1000}})
	for _, target := range []uint64{0, 1, 500, 1000, math.MaxInt64} {
		for _, equalBounds := range []bool{false, true} {
			genesis := types.DefaultGenesis()
			genesis.Params.TargetGasUsedPerBlock = target
			genesis.Params.MinimumFeePerGas = sdk.NewDec(10)
			genesis.Params.MaximumFeePerGas = sdk.NewDec(20)
			genesis.Params.MaxDynamicBaseFeeUpwardAdjustment = sdk.OneDec()
			genesis.Params.MaxDynamicBaseFeeDownwardAdjustment = sdk.OneDec()
			if equalBounds {
				genesis.Params.MaximumFeePerGas = genesis.Params.MinimumFeePerGas
			}
			var imported types.GenesisState
			require.NoError(t, cdc.UnmarshalAsJSON(cdc.MustMarshalJSON(genesis), &imported))
			require.NoError(t, imported.Validate())
			k.InitGenesis(ctx, imported)
			for _, initial := range []int64{10, 15, 20} {
				if equalBounds && initial != 10 {
					continue
				}
				for _, used := range []uint64{0, 1, 500, 999, 1000, 2000} {
					k.SetNextBaseFeePerGas(ctx, sdk.NewDec(initial))
					fee := k.AdjustDynamicBaseFeePerGas(ctx, used)
					require.NotNil(t, fee)
					require.True(t, fee.GTE(imported.Params.MinimumFeePerGas), "target=%d used=%d fee=%s", target, used, fee)
					require.True(t, fee.LTE(imported.Params.MaximumFeePerGas), "target=%d used=%d fee=%s", target, used, fee)
					require.Equal(t, *fee, k.GetNextBaseFeePerGas(ctx))
					if target == 500 && initial == 15 && !equalBounds {
						if used == 0 {
							require.Equal(t, sdk.NewDec(10), *fee)
						}
						if used == 1000 {
							require.Equal(t, sdk.NewDec(20), *fee)
						}
						if used == 500 {
							require.Equal(t, sdk.NewDec(15), *fee)
						}
					}
				}
			}
		}
	}
}
