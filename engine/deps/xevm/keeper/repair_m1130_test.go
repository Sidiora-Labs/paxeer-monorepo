package keeper_test

import (
	"context"
	"testing"
	"time"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	dbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	receipts "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func newRepairM1130Keeper(t *testing.T) (*evmkeeper.Keeper, sdk.Context) {
	t.Helper()
	key := sdk.NewKVStoreKey("repair_m1130")
	tkey := sdk.NewTransientStoreKey("repair_m1130_transient")
	ctx := testutil.DefaultContext(key, tkey)
	cfg := dbconfig.DefaultReceiptStoreConfig()
	cfg.DBDirectory = t.TempDir()
	cfg.AsyncWriteBuffer = 0
	store, err := receipts.NewReceiptStore(cfg, key)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	params := paramtypes.NewSubspace(codec.NewProtoCodec(codectypes.NewInterfaceRegistry()), codec.NewLegacyAmino(), key, tkey, "evm")
	k := evmkeeper.NewKeeper(key, tkey, params, store, nil, nil, nil, transferkeeper.Keeper{}, nil, nil, nil)
	k.UseRegularStore = true
	k.SetParams(ctx, types.DefaultParams())
	return k, ctx
}

func clearRepairM1130MinimumFee(t *testing.T, k *evmkeeper.Keeper, ctx sdk.Context) {
	t.Helper()
	key := append([]byte(k.Paramstore.Name()+"/"), types.KeyMinFeePerGas...)
	ctx.KVStore(k.GetStoreKey()).Delete(key)
	require.False(t, k.Paramstore.Has(ctx, types.KeyMinFeePerGas))
	require.True(t, k.GetMinimumFeePerGas(ctx).IsNil())
}

func TestRepairM_1130_NilMinimumFeeFixtures(t *testing.T) {
	t.Run("dynamic fee", TestGetDynamicBaseFeePerGasWithNilMinFee)
	t.Run("previous block fee", TestGetPrevBlockBaseFeePerGasWithNilMinFee)
}

func TestRepairM_1130_ConfiguredMinimumFee(t *testing.T) {
	k, ctx := newRepairM1130Keeper(t)
	minimum := sdk.NewDec(123)
	k.Paramstore.Set(ctx, types.KeyMinFeePerGas, minimum)
	k.SetCurrBaseFeePerGas(ctx, sdk.NewDec(456))
	k.SetNextBaseFeePerGas(ctx, sdk.NewDec(789))
	k.GetKVStore(ctx).Delete(types.NextBaseFeePerGasPrefix)
	require.Equal(t, minimum, k.GetNextBaseFeePerGas(ctx))
	require.False(t, k.GetNextBaseFeePerGas(ctx).IsNil())
	stored := sdk.NewDec(987)
	k.SetNextBaseFeePerGas(ctx, stored)
	require.Equal(t, stored, k.GetNextBaseFeePerGas(ctx))
}

func TestRepairM_1130_InvalidAttempts(t *testing.T) {
	k, ctx := newRepairM1130Keeper(t)
	hash := common.HexToHash("0x1130")
	require.NoError(t, k.ReceiptStore().SetReceipts(ctx, []receipts.ReceiptRecord{{
		TxHash: hash, Receipt: &evmtypes.Receipt{TxHashHex: hash.Hex(), Status: 1},
	}}))
	for _, attempts := range []int{0, -1, -10} {
		r, err := k.GetReceiptWithRetry(ctx, hash, attempts)
		require.ErrorContains(t, err, "positive attempt count")
		require.Nil(t, r)
	}
}

func TestRepairM_1130_MissingReceipt(t *testing.T) {
	k, ctx := newRepairM1130Keeper(t)
	hash := common.HexToHash("0x1131")
	r, err := k.ReceiptStore().GetReceipt(ctx, hash)
	require.ErrorIs(t, err, receipts.ErrNotFound)
	require.Nil(t, r)
	for _, attempts := range []int{1, 2} {
		got, lookupErr := k.GetReceiptWithRetry(ctx, hash, attempts)
		require.ErrorIs(t, lookupErr, receipts.ErrNotFound)
		require.Nil(t, got)
	}
}

func TestRepairM_1130_StoreFailure(t *testing.T) {
	k, ctx := newRepairM1130Keeper(t)
	hash := common.HexToHash("0x1132")
	// An incomplete protobuf varint exercises the configured store's decode error.
	require.NoError(t, k.ReceiptStore().SetReceipts(ctx, []receipts.ReceiptRecord{{
		TxHash: hash, Receipt: &evmtypes.Receipt{TxHashHex: hash.Hex()}, ReceiptBytes: []byte{0x80},
	}}))
	_, storeErr := k.ReceiptStore().GetReceipt(ctx, hash)
	require.Error(t, storeErr)
	require.NotErrorIs(t, storeErr, receipts.ErrNotFound)
	// A valid legacy receipt must not hide a failure from the configured store.
	legacy := &types.Receipt{TxHashHex: hash.Hex(), Status: 1}
	bz, err := legacy.Marshal()
	require.NoError(t, err)
	k.GetKVStore(ctx).Set(types.ReceiptKey(hash), bz)
	started := time.Now()
	r, err := k.GetReceiptWithRetry(ctx, hash, 3)
	require.EqualError(t, err, storeErr.Error())
	require.Nil(t, r)
	require.Less(t, time.Since(started), 200*time.Millisecond)
}

func TestRepairM_1130_ReceiptAvailableAfterRetry(t *testing.T) {
	k, ctx := newRepairM1130Keeper(t)
	hash := common.HexToHash("0x1133")
	expected := &evmtypes.Receipt{TxHashHex: hash.Hex(), Status: 1, GasUsed: 21000, BlockNumber: 1}
	_, err := k.GetReceipt(ctx, hash)
	require.ErrorIs(t, err, receipts.ErrNotFound)
	written := make(chan error, 1)
	go func() {
		timer := time.NewTimer(50 * time.Millisecond)
		defer timer.Stop()
		<-timer.C
		written <- k.ReceiptStore().SetReceipts(ctx, []receipts.ReceiptRecord{{TxHash: hash, Receipt: expected}})
	}()
	r, err := k.GetReceiptWithRetry(ctx, hash, 3)
	require.NoError(t, <-written)
	require.NoError(t, err)
	require.NotNil(t, r)
	require.Equal(t, expected.TxHashHex, r.TxHashHex)
	require.Equal(t, expected.Status, r.Status)
	require.Equal(t, expected.GasUsed, r.GasUsed)
	require.Equal(t, expected.BlockNumber, r.BlockNumber)
	stored, err := k.GetReceiptFromReceiptStore(ctx, hash)
	require.NoError(t, err)
	require.Equal(t, r, stored)
}

func TestRepairM_1130_Cancellation(t *testing.T) {
	k, ctx := newRepairM1130Keeper(t)
	hash := common.HexToHash("0x1134")
	t.Run("during backoff", func(t *testing.T) {
		_, err := k.GetReceipt(ctx, hash)
		require.ErrorIs(t, err, receipts.ErrNotFound)
		request, cancel := context.WithCancel(context.Background())
		defer cancel()
		timer := time.AfterFunc(30*time.Millisecond, cancel)
		defer timer.Stop()
		started := time.Now()
		r, err := k.GetReceiptWithRetry(ctx.WithContext(request), hash, 3)
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, r)
		require.Less(t, time.Since(started), 500*time.Millisecond)
	})
	t.Run("deadline during backoff", func(t *testing.T) {
		request, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		started := time.Now()
		r, err := k.GetReceiptWithRetry(ctx.WithContext(request), hash, 3)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Nil(t, r)
		require.Less(t, time.Since(started), 500*time.Millisecond)
	})
	t.Run("already canceled with receipt available", func(t *testing.T) {
		require.NoError(t, k.ReceiptStore().SetReceipts(ctx, []receipts.ReceiptRecord{{
			TxHash: hash, Receipt: &evmtypes.Receipt{TxHashHex: hash.Hex(), Status: 1},
		}}))
		request, cancel := context.WithCancel(context.Background())
		cancel()
		r, err := k.GetReceiptWithRetry(ctx.WithContext(request), hash, 3)
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, r)
	})
}

func TestRepairM_1130_LegacyReceipt(t *testing.T) {
	k, ctx := newRepairM1130Keeper(t)
	hash := common.HexToHash("0x1135")
	expected := &types.Receipt{TxHashHex: hash.Hex(), Status: 1, GasUsed: 42000}
	bz, err := expected.Marshal()
	require.NoError(t, err)
	k.GetKVStore(ctx).Set(types.ReceiptKey(hash), bz)
	r, err := k.GetReceiptWithRetry(ctx, hash, 1)
	require.NoError(t, err)
	require.Equal(t, expected, r)
}
