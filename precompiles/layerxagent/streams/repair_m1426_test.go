package streams_test

import (
	"bytes"
	"math"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/streams"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	storev2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type proofStreamsStore struct {
	h      *streamsHarness
	dir    string
	keys   []sdk.StoreKey
	payKey sdk.StoreKey
	stores sdk.CommitMultiStore
	ctx    sdk.Context
	pay    precompiles.IPrecompile
}

func newProofStreamsStore(t *testing.T) *proofStreamsStore {
	t.Helper()
	h := newStreamsHarness(t)
	source := h.db.Ctx()
	s := &proofStreamsStore{h: h, dir: t.TempDir(), keys: source.MultiStore().StoreKeys(), ctx: source}
	for _, key := range s.keys {
		if key.Name() == paytypes.StoreKey {
			s.payKey = key
		}
	}
	require.NotNil(t, s.payKey)
	s.open()
	// Seed application fixtures once, before any stream exists. Recovery only
	// loads the committed database; it never copies stream state or genesis.
	for _, key := range s.keys {
		it := source.KVStore(key).Iterator(nil, nil)
		for ; it.Valid(); it.Next() {
			s.stores.GetKVStore(key).Set(bytes.Clone(it.Key()), bytes.Clone(it.Value()))
		}
		require.NoError(t, it.Close())
	}
	s.stores.Commit(true)
	h.keeper.Host().Close()
	s.bind()
	t.Cleanup(func() {
		s.h.keeper.Host().Close()
		require.NoError(t, s.stores.Close())
	})
	return s
}

func (s *proofStreamsStore) open() {
	t := s.h.t
	t.Helper()
	cfg := config.DefaultStateCommitConfig()
	cfg.AsyncCommitBuffer = 0
	cfg.MemIAVLConfig.AsyncCommitBuffer = 0
	cfg.MemIAVLConfig.SnapshotInterval = 1
	cfg.MemIAVLConfig.SnapshotMinTimeInterval = 0
	s.stores = storev2.NewStore(s.dir, cfg, config.StateStoreConfig{}, nil)
	for _, key := range s.keys {
		kind := sdk.StoreTypeIAVL
		switch key.(type) {
		case *sdk.TransientStoreKey:
			kind = sdk.StoreTypeTransient
		case *sdk.MemoryStoreKey:
			kind = sdk.StoreTypeMemory
		}
		s.stores.MountStoreWithDB(key, kind, nil)
	}
	require.NoError(t, s.stores.LoadLatestVersion())
}

func (s *proofStreamsStore) bind() {
	h, application := s.h, testkeeper.EVMTestApp
	t := h.t
	t.Helper()
	ctx := s.ctx.WithMultiStore(s.stores.CacheMultiStore()).WithEventManager(sdk.NewEventManager())
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: h.hostKey, NetworkID: 7, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	keeper, err := paykeeper.NewKeeper(application.AppCodec(), s.payKey, h.hostKey, host, &application.EvmKeeper, application.LayerXCustodyKeeper, "pay-governance")
	require.NoError(t, err)
	h.keeper = &keeper
	built, err := streams.NewMergePrecompile(payKeepers{application.GetPrecompileKeepers(), h.keeper}, streamsPlan)
	require.NoError(t, err)
	h.precompile = built.(*agentcommon.Precompile)
	s.pay, err = lp.NewPrecompile(h.keeper)
	require.NoError(t, err)
	h.db = state.NewDBImpl(ctx, &application.EvmKeeper, false)
	block, err := application.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(application.EvmKeeper.ChainID(ctx))
	h.evm = vm.NewEVM(*block, h.db, chain, vm.Config{}, map[common.Address]vm.PrecompiledContract{
		built.Address(): built, s.pay.Address(): s.pay,
	})
	s.ctx = ctx
}

func (s *proofStreamsStore) reopen() {
	h, t := s.h, s.h.t
	t.Helper()
	latest := h.db.Ctx()
	s.ctx = s.ctx.WithBlockHeight(latest.BlockHeight()).WithBlockTime(latest.BlockTime())
	_, err := h.db.Finalize()
	require.NoError(t, err)
	// Finalize flushes the EVM's nested snapshots to the original base cache.
	// That cache, retained at bind time, must reach the committed multistore.
	// Preserve the latest block time without retaining the EVM's nested cache.
	s.ctx.MultiStore().(sdk.CacheMultiStore).Write()
	committed := s.stores.Commit(true)
	require.NotEmpty(t, committed.Hash)
	s.ctx = s.ctx.WithEVMTxHash(common.BigToHash(big.NewInt(committed.Version)))
	h.keeper.Host().Close()
	require.NoError(t, s.stores.Close())
	h.evm, h.db, h.precompile, h.keeper, s.pay = nil, nil, nil, nil, nil
	s.open()
	require.Equal(t, committed, s.stores.LastCommitID())
	s.bind()
}

func (s *proofStreamsStore) payCall(caller common.Address, name string, args ...interface{}) []interface{} {
	t, h := s.h.t, s.h
	t.Helper()
	method := s.pay.GetABI().Methods[name]
	input, err := s.pay.GetABI().Pack(name, args...)
	require.NoError(t, err)
	var output []byte
	if method.IsConstant() {
		output, _, err = h.evm.StaticCall(caller, s.pay.Address(), input, 2000000)
	} else {
		output, _, err = h.evm.Call(caller, s.pay.Address(), input, 2000000, uint256.NewInt(0))
	}
	require.NoError(t, err, "%s: %x", name, output)
	require.NoError(t, h.db.Err())
	values, err := method.Outputs.Unpack(output)
	require.NoError(t, err)
	return values
}

func TestRepairM_1426_StreamsPersistAcrossDatabaseReopen(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		cap, stop             int64
		settled, paid, refund int64
		closeViaPay           bool
	}{
		{"agent_close_refund", 100, 100, 19, 9, 72, false},
		{"pay_close_refund", 100, 100, 19, 9, 72, true},
		{"cap_exhausted", 18, 100, 18, 0, 0, true},
		{"end_time", 100, 9, 19, 0, 81, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newProofStreamsStore(t)
			h := s.h
			require.Equal(t, common.HexToAddress("0x0906"), h.precompile.Address())
			require.Equal(t, common.HexToAddress("0x101b"), s.pay.Address())
			id := h.write(h.payer.caller, "open", h.payee.caller, h.token, big.NewInt(2), streamsStart, streamsStart+uint64(tc.stop), big.NewInt(tc.cap))[0].(*big.Int)
			var kernelID [32]byte
			id.FillBytes(kernelID[:])
			payment := func(caller common.Address, method string, want int64) {
				t.Helper()
				output, err := h.call(caller, method, id)
				require.NoError(t, err, "%s: %x", method, output)
				// Compare the exact ABI result; big.Int has multiple internal
				// representations of zero after ABI decoding.
				expected, err := h.precompile.GetABI().Methods[method].Outputs.Pack(big.NewInt(want))
				require.NoError(t, err)
				require.Equal(t, expected, output)
			}
			check := func(total int64, closed bool) {
				t.Helper()
				r := h.record(id)
				require.Equal(t, codec.U128{Lo: 3}, r.Rate)
				require.Equal(t, uint64(1000), r.RateUnit)
				require.Equal(t, codec.U128{Lo: uint64(tc.cap)}, r.TotalCap)
				require.Equal(t, codec.U128{Lo: uint64(total)}, r.SettledTotal)
				require.Equal(t, r.SettledTotal, r.AccruedTotal)
				require.Equal(t, closed, r.Closed)
				for read := 0; read < 2; read++ {
					got := abi.ConvertType(h.view("getStream", id)[0], new(streamOutput)).(*streamOutput)
					requireStreamTuple(t, h, &streamOutput{Id: id, Payer: h.payer.caller, Payee: h.payee.caller, Token: h.token, RatePerSecond: big.NewInt(3), Cap: big.NewInt(tc.cap), StartTime: streamsStart, StopTime: streamsStart + uint64(tc.stop), Settled: big.NewInt(total), Active: !closed}, got)
					pay := abi.ConvertType(s.payCall(h.other.caller, "getStream", kernelID)[0], new(paytypes.StreamTuple)).(*paytypes.StreamTuple)
					want, err := s.pay.GetABI().Methods["getStream"].Outputs.Pack(r.ABITuple())
					require.NoError(t, err)
					actual, err := s.pay.GetABI().Methods["getStream"].Outputs.Pack(pay)
					require.NoError(t, err)
					require.Equal(t, want, actual)
				}
				payer, held := 1000-tc.cap, tc.cap-total
				if closed {
					payer, held = 1000-total, 0
				}
				for account, want := range map[[32]byte]int64{r.Payer: payer, r.Recipient: total, r.StreamAccount: held} {
					balance, err := h.keeper.Host().ReadBalance(h.db.Ctx(), account, h.asset)
					require.NoError(t, err)
					require.Equal(t, big.NewInt(want).String(), balance.String())
				}
			}
			h.advance(5)
			require.Equal(t, []interface{}{big.NewInt(10)}, h.write(h.payee.caller, "settle", id))
			h.advance(8)
			require.Empty(t, h.write(h.payer.caller, "updateRate", id, big.NewInt(3)))
			before := h.record(id)
			s.reopen()
			require.Equal(t, before, h.record(id))
			check(16, false)
			payment(h.payee.caller, "settle", 0)
			check(16, false)
			h.advance(9)
			key := [32]byte{0x61}
			s.payCall(h.payee.caller, "streamSettle", kernelID, key)
			result, err := h.keeper.GetStreamResult(h.db.Ctx(), key)
			require.NoError(t, err)
			require.Equal(t, codec.U128{Lo: uint64(tc.settled - 16)}, result.Paid)
			check(tc.settled, false)
			s.reopen()
			check(tc.settled, false)
			s.payCall(h.payee.caller, "streamSettle", kernelID, key)
			recovered, err := h.keeper.GetStreamResult(h.db.Ctx(), key)
			require.NoError(t, err)
			require.Equal(t, result, recovered)
			check(tc.settled, false)
			payment(h.payee.caller, "settle", 0)
			check(tc.settled, false)
			h.advance(12)
			if tc.closeViaPay {
				closeKey := [32]byte{0x62}
				s.payCall(h.payer.caller, "streamClose", kernelID, closeKey)
				closed, err := h.keeper.GetStreamResult(h.db.Ctx(), closeKey)
				require.NoError(t, err)
				require.Equal(t, codec.U128{Lo: uint64(tc.paid)}, closed.Paid)
				require.Equal(t, codec.U128{Lo: uint64(tc.refund)}, closed.Refunded)
			} else {
				index := len(h.db.GetAllLogs())
				payment(h.payer.caller, "close", tc.paid)
				h.log(index, "StreamClosed", "0xbd03ee89416a2d44203c48d8655c11fea0add5437f20b8026356e9a22481aca0", []common.Hash{common.BigToHash(id)}, big.NewInt(tc.paid), big.NewInt(tc.refund))
			}
			check(tc.settled+tc.paid, true)
			s.reopen()
			check(tc.settled+tc.paid, true)
		})
	}
}
