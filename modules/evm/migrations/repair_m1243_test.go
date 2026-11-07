package migrations_test

import (
	"bytes"
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM1243OpenStore(t *testing.T, dir string) (*keeper.Keeper, sdk.Context, storetypes.CommitMultiStore, func()) {
	t.Helper()
	db, err := dbm.NewGoLevelDB("migration", dir)
	require.NoError(t, err)
	ms := store.NewCommitMultiStore(db)
	closed := false
	closeStore := func() {
		t.Helper()
		if !closed {
			closed = true
			require.NoError(t, ms.Close())
		}
	}
	t.Cleanup(closeStore)
	key := sdk.NewKVStoreKey(paramtypes.StoreKey)
	tkey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkey, sdk.StoreTypeTransient, db)
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, tmtypes.Header{Height: ms.LastCommitID().Version + 1}, false)
	ss := paramtypes.NewSubspace(codec.NewProtoCodec(codectypes.NewInterfaceRegistry()), codec.NewLegacyAmino(), key, tkey, types.ModuleName).WithKeyTable(types.ParamKeyTable())
	return &keeper.Keeper{Paramstore: ss}, ctx, ms, closeStore
}

func repairM1243GovernedParams(t *testing.T) types.Params {
	t.Helper()
	params := types.DefaultParams()
	params.PriorityNormalizer = sdk.NewDec(3)
	params.BaseFeePerGas = sdk.NewDec(123)
	params.MaximumFeePerGas = sdk.NewDec(999999999999)
	params.MinimumFeePerGas = sdk.NewDec(456)
	params.MaxDynamicBaseFeeUpwardAdjustment = sdk.NewDecWithPrec(7, 2)
	params.MaxDynamicBaseFeeDownwardAdjustment = sdk.NewDecWithPrec(2, 2)
	params.TargetGasUsedPerBlock = 765432
	params.DeliverTxHookWasmGasLimit = 345678
	params.PaxSstoreSetGasEip2200 = 23456
	params.RegisterPointerDisabled = true
	params.WhitelistedCwCodeHashesForDelegateCall = [][]byte{bytes.Repeat([]byte{0x42}, 32)}
	params.FeeTokenEnabled = true
	params.FeeTokenDistribution = true
	params.MaxFeeTokenRateAge = 321
	params.MaxFeeTokenSpread = sdk.NewDecWithPrec(9, 2)
	params.AllowedFeeDenoms = []types.AllowedFeeDenom{{Denom: "uasset", Rate: sdk.NewDec(42), RateUpdateHeight: 1}}
	require.NoError(t, params.Validate())
	return params
}

func TestRepairM_1243_MaxBaseFeePersistence(t *testing.T) {
	TestMigrateEip1559MaxBaseFee(t)
}

func TestRepairM_1243_Eip1559ParamsPersistence(t *testing.T) {
	TestMigrateEip1559Params(t)
}
