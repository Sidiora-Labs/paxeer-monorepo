package benchmarks

import (
	"encoding/json"
	"flag"
	"fmt"
	"testing"
	"time"

	storageconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/ss"
	"github.com/stretchr/testify/require"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	wasmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
)

func TestRepairM_1271_SelectedDatabaseAndOptions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		database func(testing.TB) benchmarkDatabase
	}{{"pebbledb", buildPebbleDB}, {"pebbledb buffered", buildBufferedPebbleDB}} {
		t.Run(tc.name, func(t *testing.T) {
			db := tc.database(t)
			var info AppInfo
			var recipient sdk.AccAddress
			var height int64
			t.Run("deliver", func(t *testing.T) {
				info = InitializeWasmApp(t, db, 2, wasmkeeper.WithMaxQueryStackSize(0))
				txs := GenSequenceOfTxs(t, &info, bankSendMsg, 1)
				msg := txs[0].GetMsgs()[0].(*banktypes.MsgSend)
				var err error
				recipient, err = sdk.AccAddressFromBech32(msg.ToAddress)
				require.NoError(t, err)
				require.True(t, info.App.BankKeeper.GetBalance(info.App.GetCheckCtx(), recipient, info.Denom).IsZero())
				require.Equal(t, 1, deliverBenchmarkTxs(t, &info, txs, 20))

				contract, err := sdk.AccAddressFromBech32(info.ContractAddr)
				require.NoError(t, err)
				query := []byte(fmt.Sprintf(`{"balance":{"address":%q}}`, info.MinterAddr.String()))
				_, err = info.App.WasmKeeper.QuerySmart(info.App.GetCheckCtx(), contract, query)
				require.ErrorIs(t, err, wasmtypes.ErrExceedMaxQueryStackSize, "the constructor must apply the supplied WASM option")
				height = info.App.LastBlockHeight()
				require.Eventually(t, func() bool { return info.App.GetStateStore().GetLatestVersion() == height }, 5*time.Second, 10*time.Millisecond)
				for _, want := range []struct {
					address sdk.AccAddress
					amount  int64
				}{{recipient, 100}, {info.MinterAddr, 100000000000 - 100}} {
					raw, err := info.App.GetStateStore().Get(banktypes.StoreKey, height, banktypes.CreatePrefixedAccountStoreKey(want.address, []byte(info.Denom)))
					require.NoError(t, err)
					require.NotEmpty(t, raw, "delivered state must reside in %s", tc.name)
					var coin sdk.Coin
					require.NoError(t, info.App.AppCodec().Unmarshal(raw, &coin))
					require.Equal(t, sdk.NewInt64Coin(info.Denom, want.amount), coin)
					require.Equal(t, coin, info.App.BankKeeper.GetBalance(info.App.GetCheckCtx(), want.address, info.Denom))
				}
			})
			require.False(t, t.Failed())
			// The child cleanup stops the app before reopening the configured backend directory.
			cfg := storageconfig.DefaultStateStoreConfig()
			cfg.Backend, cfg.DBDirectory, cfg.AsyncWriteBuffer = db.backend, db.directory, db.writeBuffer
			reopened, err := ss.NewStateStore(t.TempDir(), cfg)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, reopened.Close()) })
			require.Equal(t, height, reopened.GetLatestVersion())
			for _, want := range []struct {
				address sdk.AccAddress
				amount  int64
			}{{recipient, 100}, {info.MinterAddr, 100000000000 - 100}} {
				raw, err := reopened.Get(banktypes.StoreKey, height, banktypes.CreatePrefixedAccountStoreKey(want.address, []byte(info.Denom)))
				require.NoError(t, err)
				require.NotEmpty(t, raw, "the selected directory must retain delivered state after close")
				var coin sdk.Coin
				require.NoError(t, info.App.AppCodec().Unmarshal(raw, &coin))
				require.Equal(t, sdk.NewInt64Coin(info.Denom, want.amount), coin)
			}
		})
	}
}

func TestRepairM_1271_BenchmarkInitialization(t *testing.T) {
	benchtime := flag.Lookup("test.benchtime")
	require.NotNil(t, benchtime)
	previous := benchtime.Value.String()
	require.NoError(t, benchtime.Value.Set("1x"))
	t.Cleanup(func() { require.NoError(t, benchtime.Value.Set(previous)) })
	cases := txSendingBenchmarkCases()
	for _, name := range []string{"basic send - pebbledb", "basic send - pebbledb buffered", "cw20 transfer - pebbledb", "cw20 transfer - pebbledb buffered"} {
		t.Run(name, func(t *testing.T) {
			// This is the same entrypoint registered with b.Run by BenchmarkTxSending.
			result := testing.Benchmark(cases[name].run)
			require.Equal(t, 1, result.N)
			require.Equal(t, float64(result.N), result.Extra["delivered-txs"], "initialization must reach successful measured delivery")
		})
	}
}

func TestRepairM_1271_AllTransactionsIncludingPartialBlocks(t *testing.T) {
	info := InitializeWasmApp(t, buildPebbleDB(t), 2)
	for _, tc := range []struct{ count, blockSize int }{{0, 20}, {1, 20}, {19, 20}, {20, 20}, {21, 20}, {43, 20}, {1001, 1000}} {
		t.Run(fmt.Sprintf("count_%d_block_%d", tc.count, tc.blockSize), func(t *testing.T) {
			height := info.App.LastBlockHeight()
			before := info.App.BankKeeper.GetBalance(info.App.GetCheckCtx(), info.MinterAddr, info.Denom)
			feeBalance := info.App.BankKeeper.GetBalance(info.App.GetCheckCtx(), info.MinterAddr, sdk.DefaultBondDenom)
			txs := GenSequenceOfTxs(t, &info, bankSendMsg, tc.count)
			fees := sdk.ZeroInt()
			params := info.App.ParamsKeeper.GetFeesParams(info.App.GetCheckCtx())
			for _, tx := range txs {
				feeTx := tx.(sdk.FeeTx)
				minimum := params.GetGlobalMinimumGasPrices().AmountOf(sdk.DefaultBondDenom).
					Mul(sdk.NewDecFromInt(sdk.NewIntFromUint64(feeTx.GetGas()))).Ceil().RoundInt()
				require.True(t, minimum.IsPositive())
				require.Equal(t, minimum, feeTx.GetFee().AmountOf(sdk.DefaultBondDenom))
				fees = fees.Add(minimum)
			}
			require.Equal(t, tc.count, deliverBenchmarkTxs(t, &info, txs, tc.blockSize))
			require.Equal(t, height+int64((tc.count+tc.blockSize-1)/tc.blockSize), info.App.LastBlockHeight())
			after := info.App.BankKeeper.GetBalance(info.App.GetCheckCtx(), info.MinterAddr, info.Denom)
			require.Equal(t, before.Amount.SubRaw(int64(100*tc.count)), after.Amount)
			require.Equal(t, feeBalance.Amount.Sub(fees), info.App.BankKeeper.GetBalance(info.App.GetCheckCtx(), info.MinterAddr, sdk.DefaultBondDenom).Amount)
			account := info.App.AccountKeeper.GetAccount(info.App.GetCheckCtx(), info.MinterAddr)
			require.NotNil(t, account)
			require.Equal(t, info.SeqNum, account.GetSequence())
			for _, tx := range txs {
				msg := tx.GetMsgs()[0].(*banktypes.MsgSend)
				recipient, err := sdk.AccAddressFromBech32(msg.ToAddress)
				require.NoError(t, err)
				require.Equal(t, sdk.NewInt64Coin(info.Denom, 100), info.App.BankKeeper.GetBalance(info.App.GetCheckCtx(), recipient, info.Denom))
			}
		})
	}
}

func TestRepairM_1271_BlockLifecycleAndCommittedHeights(t *testing.T) {
	info := InitializeWasmApp(t, buildPebbleDB(t), 2)
	first := info.App.LastBlockHeight() + 1
	var committed []int64
	info.App.SetCommitListener(func(height int64, hash []byte) error {
		committed = append(committed, height)
		require.NotEmpty(t, hash)
		return nil
	})
	txs := GenSequenceOfTxs(t, &info, bankSendMsg, 43)
	require.Equal(t, 43, deliverBenchmarkTxs(t, &info, txs, 20))
	require.Equal(t, []int64{first, first + 1, first + 2}, committed)
	require.Equal(t, first+2, info.App.GetCheckCtx().BlockHeight())
	for _, height := range committed {
		history, found := info.App.StakingKeeper.GetHistoricalInfo(info.App.GetCheckCtx(), height)
		require.True(t, found, "the real staking BeginBlock hook must run at height %d", height)
		require.Equal(t, height, history.Header.Height)
		require.Equal(t, info.App.ChainID, history.Header.ChainID)
		require.Equal(t, time.Unix(height, 0).UTC(), history.Header.Time)
	}
}

func TestRepairM_1271_CW20PartialBlockDelivery(t *testing.T) {
	info := InitializeWasmApp(t, buildPebbleDB(t), 2)
	contract, err := sdk.AccAddressFromBech32(info.ContractAddr)
	require.NoError(t, err)
	balanceOf := func(address string) string {
		query, err := json.Marshal(map[string]interface{}{"balance": map[string]string{"address": address}})
		require.NoError(t, err)
		raw, err := info.App.WasmKeeper.QuerySmart(info.App.GetCheckCtx(), contract, query)
		require.NoError(t, err)
		var res struct {
			Balance string `json:"balance"`
		}
		require.NoError(t, json.Unmarshal(raw, &res))
		return res.Balance
	}
	require.Equal(t, "1000000000", balanceOf(info.MinterAddr.String()))
	initialHeight := info.App.LastBlockHeight()
	txs := GenSequenceOfTxs(t, &info, cw20TransferMsg, 23)
	require.Equal(t, 23, deliverBenchmarkTxs(t, &info, txs, 20))
	require.Equal(t, "999982405", balanceOf(info.MinterAddr.String()))
	for _, tx := range txs {
		msg := tx.GetMsgs()[0].(*wasmtypes.MsgExecuteContract)
		var transfer cw20ExecMsg
		require.NoError(t, json.Unmarshal(msg.Msg, &transfer))
		require.NotNil(t, transfer.Transfer)
		require.Equal(t, "765", balanceOf(transfer.Transfer.Recipient))
	}
	require.EqualValues(t, 25, info.App.AccountKeeper.GetAccount(info.App.GetCheckCtx(), info.MinterAddr).GetSequence())
	require.Equal(t, initialHeight+2, info.App.LastBlockHeight())
}
