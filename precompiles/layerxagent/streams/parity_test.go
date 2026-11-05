package streams_test

import (
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestStreamsAndPayStreamRecordsIdentical(t *testing.T) {
	seed := newStreamsHarness(t)
	seedState := seed.keeper.ExportGenesis(seed.db.Ctx())
	pay, err := lp.NewPrecompile(seed.keeper)
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress("0x0906"), seed.precompile.Address())
	require.Equal(t, common.HexToAddress("0x101b"), pay.Address())

	type settlement struct {
		atMs, paid int64
	}
	for _, tc := range []struct {
		name                       string
		rate, cap                  int64
		stop                       uint64
		settlements                []settlement
		closeMs, finalPaid, refund int64
	}{
		{"refund", 2, 100, 100, []settlement{{0, 0}, {2000, 4}, {2000, 0}, {5000, 6}}, 8000, 6, 84},
		{"cap_exhausted", 2, 10, 100, []settlement{{2000, 4}, {5000, 6}, {7000, 0}}, 8000, 0, 0},
		{"end_time", 2, 100, 5, []settlement{{2000, 4}, {7000, 6}, {8000, 0}}, 9000, 0, 90},
		{"fractional_accrual", 3, 100, 100, []settlement{{1250, 3}, {1500, 1}, {1500, 0}, {2000, 2}}, 2250, 0, 94},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fork := func() *streamsHarness {
				h := *seed
				h.t = t
				h.db = seed.db.Copy().(*state.DBImpl)
				chain := evmtypes.DefaultChainConfig().EthereumConfig(testkeeper.EVMTestApp.EvmKeeper.ChainID(h.db.Ctx()))
				h.evm = vm.NewEVM(seed.evm.Context, h.db, chain, vm.Config{}, map[common.Address]vm.PrecompiledContract{
					h.precompile.Address(): h.precompile,
					pay.Address():          pay,
				})
				return &h
			}
			agent, direct := fork(), fork()
			txHash := [32]byte(seed.db.Ctx().EVMTxHash())
			id := paytypes.DeriveStreamID(txHash, 0, seed.payer.account)
			streamAccount, err := codec.DeriveAccountID([]byte("agent:" + string(seed.payer.did) + ":stream:" + hex.EncodeToString(id[:])))
			require.NoError(t, err)
			payWrite := func(caller common.Address, name string, args ...interface{}) {
				t.Helper()
				input, err := pay.GetABI().Pack(name, args...)
				require.NoError(t, err)
				out, _, err := direct.evm.Call(caller, pay.Address(), input, 2000000, uint256.NewInt(0))
				require.NoError(t, err, "%s: %x", name, out)
				require.NoError(t, direct.db.Err())
				require.Empty(t, out)
			}
			compare := func(total int64, closed bool) {
				t.Helper()
				left, err := agent.keeper.GetStream(agent.db.Ctx(), id)
				require.NoError(t, err)
				right, err := direct.keeper.GetStream(direct.db.Ctx(), id)
				require.NoError(t, err)
				require.Equal(t, left, right)
				require.Equal(t, id, left.StreamID)
				require.Equal(t, seed.payer.account, left.Payer)
				require.Equal(t, seed.payee.account, left.Recipient)
				require.Equal(t, streamAccount, left.StreamAccount)
				require.Equal(t, seed.asset, left.AssetID)
				require.Equal(t, paytypes.StreamModeTime, left.Mode)
				require.Equal(t, codec.U128{Lo: uint64(tc.rate)}, left.Rate)
				require.Equal(t, uint64(1000), left.RateUnit)
				require.Equal(t, streamsStart*1000, left.StartTimestamp)
				require.Equal(t, (streamsStart+tc.stop)*1000, left.EndTimestamp)
				require.Equal(t, codec.U128{Lo: uint64(tc.cap)}, left.TotalCap)
				require.Equal(t, codec.U128{Lo: uint64(total)}, left.AccruedTotal)
				require.Equal(t, codec.U128{Lo: uint64(total)}, left.SettledTotal)
				require.Equal(t, closed, left.Closed)
				require.False(t, left.Paused)
				require.False(t, left.Underfunded)
				payer, custody := int64(1000)-tc.cap, tc.cap-total
				if closed {
					payer, custody = 1000-total, 0
				}
				for account, want := range map[[32]byte]int64{left.Payer: payer, left.Recipient: total, left.StreamAccount: custody} {
					leftBalance, err := agent.keeper.Host().ReadBalance(agent.db.Ctx(), account, seed.asset)
					require.NoError(t, err)
					rightBalance, err := direct.keeper.Host().ReadBalance(direct.db.Ctx(), account, seed.asset)
					require.NoError(t, err)
					require.Equal(t, leftBalance, rightBalance)
					require.Equal(t, big.NewInt(want).String(), leftBalance.String())
				}
			}
			compareResult := func(key [32]byte, paid, refund int64) {
				t.Helper()
				left, err := agent.keeper.GetStreamResult(agent.db.Ctx(), key)
				require.NoError(t, err)
				right, err := direct.keeper.GetStreamResult(direct.db.Ctx(), key)
				require.NoError(t, err)
				require.Equal(t, left, right)
				require.Equal(t, id, left.StreamID)
				require.Equal(t, codec.U128{Lo: uint64(paid)}, left.Paid)
				require.Equal(t, codec.U128{Lo: uint64(refund)}, left.Refunded)
			}
			advance := func(ms int64) {
				for _, h := range []*streamsHarness{agent, direct} {
					h.db.WithCtx(h.db.Ctx().WithBlockHeight(h.db.Ctx().BlockHeight() + 1).WithBlockTime(time.UnixMilli(int64(streamsStart)*1000 + ms)))
				}
			}

			opened := agent.write(agent.payer.caller, "open", agent.payee.caller, agent.token, big.NewInt(tc.rate), streamsStart, streamsStart+tc.stop, big.NewInt(tc.cap))[0].(*big.Int)
			require.Equal(t, new(big.Int).SetBytes(id[:]), opened)
			require.Equal(t, seedState, direct.keeper.ExportGenesis(direct.db.Ctx()))
			payWrite(direct.payer.caller, "streamOpen", id, streamAccount, direct.payee.account, direct.asset,
				paytypes.StreamModeTime, big.NewInt(tc.rate), uint64(1000), streamsStart*1000, (streamsStart+tc.stop)*1000,
				big.NewInt(tc.cap), big.NewInt(tc.cap), [][32]byte{})
			compare(0, false)

			total := int64(0)
			for i, step := range tc.settlements {
				advance(step.atMs)
				caller := agent.payee.caller
				if i%2 != 0 {
					caller = agent.payer.caller
				}
				key := paytypes.DeriveIdempotencyKey(txHash, uint32(i+1), [4]byte(agent.precompile.GetABI().Methods["settle"].ID))
				paid := agent.write(caller, "settle", opened)
				require.Len(t, paid, 1)
				require.Zero(t, paid[0].(*big.Int).Cmp(big.NewInt(step.paid)))
				payWrite(caller, "streamSettle", id, key)
				total += step.paid
				compare(total, false)
				compareResult(key, step.paid, 0)
			}

			advance(tc.closeMs)
			key := paytypes.DeriveIdempotencyKey(txHash, uint32(len(tc.settlements)+1), [4]byte(agent.precompile.GetABI().Methods["close"].ID))
			finalPaid := agent.write(agent.payer.caller, "close", opened)
			require.Len(t, finalPaid, 1)
			require.Zero(t, finalPaid[0].(*big.Int).Cmp(big.NewInt(tc.finalPaid)))
			payWrite(direct.payer.caller, "streamClose", id, key)
			compare(total+tc.finalPaid, true)
			compareResult(key, tc.finalPaid, tc.refund)
			require.Equal(t, seedState, seed.keeper.ExportGenesis(seed.db.Ctx()))
		})
	}
}
