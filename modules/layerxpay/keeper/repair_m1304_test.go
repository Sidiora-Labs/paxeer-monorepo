package keeper_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/streams"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairServiceKey(prefix string, id [32]byte) []byte {
	return types.KernelKVKey(types.ServiceModuleID, append([]byte(prefix), id[:]...))
}
func repairSnapshot(ctx sdk.Context, key sdk.StoreKey) map[string][]byte {
	values := make(map[string][]byte)
	it := ctx.KVStore(key).Iterator(nil, nil)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		values[string(it.Key())] = bytes.Clone(it.Value())
	}
	return values
}

func repairServiceProduce(t *testing.T, w *keeper.RepairM1304World) ([32]byte, [32]byte, types.ServiceDelivery) {
	t.Helper()
	report, delivery := [32]byte{0xa1}, [32]byte{0xa2}
	progressReceipt, err := w.Keeper.ServiceProgressReport(w.Context, w.Provider, report, w.Commitment, [32]byte{0xa3}, [32]byte{0xa4}, 5000, [32]byte{0xa5})
	require.NoError(t, err)
	_, err = w.Keeper.ServiceProgressReport(w.Context, w.Provider, [32]byte{0xb1}, w.Commitment, [32]byte{0xa3}, [32]byte{0xa4}, 8000, [32]byte{0xb2})
	require.NoError(t, err)
	delivered, err := w.Keeper.ServiceDeliver(w.Context, w.Provider, delivery, w.Agreement, []types.ServiceDeliverable{{Hash: w.Spec, ArtifactSize: 1024, AvailabilityReference: [32]byte{0xa6}}}, [32]byte{0xa7})
	require.NoError(t, err)
	progress, found, err := w.Keeper.ServiceViews().GetProgress(w.Context, report)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, progressReceipt.GlobalSequence, progress.GlobalSequence)
	require.Equal(t, uint32(5000), progress.ProgressBPS)
	require.Equal(t, w.Commitment, progress.CommitmentID)
	got, found, err := w.Keeper.ServiceViews().GetDelivery(w.Context, delivery)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, delivered.GlobalSequence, got.GlobalSequence)
	require.Equal(t, w.Agreement, got.AgreementID)
	require.Equal(t, []types.ServiceDeliverable{{Hash: w.Spec, ArtifactSize: 1024, AvailabilityReference: [32]byte{0xa6}}}, got.Deliverables)
	return report, delivery, got
}

func TestRepairM_1304_GenesisProofGap(t *testing.T) {
	// Both application JSON/reopen rounds and duplicate refusals remain part of
	// this task's focused proof, including the strengthened malformed-export case.
	TestProofPayGenesis(t)
	w := keeper.OpenRepairM1304World(t, false)
	_, delivery, _ := repairServiceProduce(t, w)
	exported := w.Keeper.ExportGenesis(w.Context)
	require.NoError(t, keeper.ValidateOwnedGenesis(*exported))
	wire, err := json.Marshal(exported)
	require.NoError(t, err)
	consumer := keeper.OpenRepairM1304World(t, false)
	for _, mutation := range []string{"offer_bytes", "offer_id", "progress_bounds", "missing_item", "item_version", "delivery_count", "orphan_item", "bad_key", "duplicate"} {
		t.Run(mutation, func(t *testing.T) {
			var malformed types.GenesisState
			require.NoError(t, json.Unmarshal(wire, &malformed))
			changed := false
			for i := range malformed.KernelRecords {
				r := &malformed.KernelRecords[i]
				if !bytes.HasPrefix(r.Key, types.ServiceKVPrefix) {
					continue
				}
				switch {
				case mutation == "offer_bytes" && bytes.HasPrefix(r.Key[3:], []byte("offer::1")):
					r.Value = r.Value[:1]
					changed = true
				case mutation == "offer_id" && bytes.HasPrefix(r.Key[3:], []byte("offer::1")):
					r.Value[1] ^= 1
					changed = true
				case mutation == "progress_bounds" && bytes.HasPrefix(r.Key[3:], []byte("progrs:1")):
					binary.BigEndian.PutUint32(r.Value[225:229], 10001)
					changed = true
				case mutation == "missing_item" && bytes.Equal(r.Key, append(repairServiceKey("delitm:1", delivery), 0)):
					malformed.KernelRecords = append(malformed.KernelRecords[:i], malformed.KernelRecords[i+1:]...)
					changed = true
				case mutation == "item_version" && bytes.Equal(r.Key, append(repairServiceKey("delitm:1", delivery), 0)):
					r.Value[0] = 2
					changed = true
				case mutation == "delivery_count" && bytes.Equal(r.Key, repairServiceKey("delivr:1", delivery)):
					r.Value[129] = 17
					changed = true
				case mutation == "orphan_item" && bytes.Equal(r.Key, append(repairServiceKey("delitm:1", delivery), 0)):
					r.Key[len(r.Key)-1] = 16
					changed = true
				case mutation == "bad_key" && bytes.HasPrefix(r.Key[3:], []byte("offer::1")):
					r.Key = append(r.Key, 0)
					changed = true
				case mutation == "duplicate" && bytes.HasPrefix(r.Key[3:], []byte("offer::1")):
					malformed.KernelRecords = append(malformed.KernelRecords, *r)
					changed = true
				}
				if changed {
					break
				}
			}
			require.True(t, changed)
			beforeHost, beforePay := repairSnapshot(consumer.Context, consumer.HostKey), repairSnapshot(consumer.Context, consumer.PayKey)
			validationErr := keeper.ValidateOwnedGenesis(malformed)
			require.ErrorIs(t, validationErr, types.ErrInvalidGenesis)
			detail := map[string]string{
				"offer_bytes":     "invalid offer::1 record body: LXP_NON_CANONICAL",
				"offer_id":        "offer::1 key/body id mismatch",
				"progress_bounds": "LXP_NON_CANONICAL: corrupt service state: record identifier",
				"missing_item":    "missing or invalid service delivery item",
				"item_version":    "invalid or orphaned service delivery item",
				"delivery_count":  "invalid service delivery header",
				"orphan_item":     "invalid or orphaned service delivery item",
				"bad_key":         "invalid service record key or version",
				"duplicate":       "duplicate kernel record",
			}[mutation]
			require.ErrorContains(t, validationErr, detail)
			if mutation == "progress_bounds" {
				require.EqualError(t, validationErr, detail+": invalid genesis")
			}
			for n := 0; n < 2; n++ {
				err := consumer.Keeper.InitGenesis(consumer.Context, malformed)
				require.ErrorIs(t, err, types.ErrInvalidGenesis)
				require.Equal(t, beforeHost, repairSnapshot(consumer.Context, consumer.HostKey))
				require.Equal(t, beforePay, repairSnapshot(consumer.Context, consumer.PayKey))
			}
			_, found, err := consumer.Keeper.ServiceViews().GetOffer(consumer.Context, [32]byte{0x81})
			require.NoError(t, err)
			require.True(t, found)
		})
	}
}

func TestRepairM_1304_ServiceCorruption(t *testing.T) {
	w := keeper.OpenRepairM1304World(t, false)
	report, delivery, _ := repairServiceProduce(t, w)
	views := w.Keeper.ServiceViews()
	cases := []struct {
		prefix string
		id     [32]byte
		read   func() (bool, error)
	}{
		{"offer::1", [32]byte{0x81}, func() (bool, error) { _, ok, err := views.GetOffer(w.Context, [32]byte{0x81}); return ok, err }},
		{"agree::1", w.Agreement, func() (bool, error) { _, ok, err := views.GetAgreement(w.Context, w.Agreement); return ok, err }},
		{"commit:1", w.Commitment, func() (bool, error) { _, ok, err := views.GetCommitment(w.Context, w.Commitment); return ok, err }},
		{"progrs:1", report, func() (bool, error) { _, ok, err := views.GetProgress(w.Context, report); return ok, err }},
		{"delivr:1", delivery, func() (bool, error) { _, ok, err := views.GetDelivery(w.Context, delivery); return ok, err }},
	}
	for _, tc := range cases {
		t.Run(tc.prefix, func(t *testing.T) {
			store := w.Context.KVStore(w.HostKey)
			key := repairServiceKey(tc.prefix, tc.id)
			original := bytes.Clone(store.Get(key))
			require.NotEmpty(t, original)
			for _, bad := range [][]byte{{1}, {255}, append(bytes.Clone(original), 0)} {
				store.Set(key, bad)
				beforeHost, beforePay := repairSnapshot(w.Context, w.HostKey), repairSnapshot(w.Context, w.PayKey)
				var first string
				for i := 0; i < 2; i++ {
					found, err := tc.read()
					require.False(t, found)
					require.ErrorIs(t, err, keeper.ErrServiceCorruptState)
					if i == 0 {
						first = err.Error()
					} else {
						require.Equal(t, first, err.Error())
					}
				}
				require.Equal(t, beforeHost, repairSnapshot(w.Context, w.HostKey))
				require.Equal(t, beforePay, repairSnapshot(w.Context, w.PayKey))
			}
			store.Delete(key)
			found, err := tc.read()
			require.False(t, found)
			require.NoError(t, err)
			store.Set(key, original)
			found, err = tc.read()
			require.True(t, found)
			require.NoError(t, err)
		})
	}
	itemKey := append(repairServiceKey("delitm:1", delivery), 0)
	item := bytes.Clone(w.Context.KVStore(w.HostKey).Get(itemKey))
	w.Context.KVStore(w.HostKey).Delete(itemKey)
	_, found, err := views.GetDelivery(w.Context, delivery)
	require.False(t, found)
	require.ErrorIs(t, err, keeper.ErrServiceCorruptState)
	w.Context.KVStore(w.HostKey).Set(itemKey, item)
	// Execution and dispute bytes are produced by their real signed keeper flows.
	keeper.ProveRepairM1304AdditionalServiceRecords(t)
}

func TestRepairM_1304_ServiceIndexes(t *testing.T) {
	w := keeper.OpenRepairM1304World(t, false)
	_, _, want := repairServiceProduce(t, w)
	read := func(ctx sdk.Context) uint64 {
		t.Helper()
		meter := sdk.NewInfiniteGasMeterWithMultiplier(ctx)
		ctx = ctx.WithGasMeter(meter)
		got, found, err := w.Keeper.ServiceViews().LatestDelivery(ctx, w.Agreement)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, want, got)
		high, found, err := w.Keeper.ServiceViews().ProgressHighWater(ctx, w.Commitment)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, uint32(8000), high)
		return uint64(meter.GasConsumed())
	}
	// Another agreement's later real production must not win either index.
	_, err := w.Keeper.ServiceOfferPublish(w.Context, w.Provider, [32]byte{0xe1}, [32]byte{0x86}, codec.U128{Lo: 50}, [32]byte{0xe2}, w.Spec, 1700000100000, 20000, 30000, 1, 1700000090000)
	require.NoError(t, err)
	_, err = w.Keeper.ServiceAgreementPropose(w.Context, w.Buyer, [32]byte{0xe3}, [32]byte{0xe1}, [32]byte{0xe2}, [32]byte{0xe4}, [32]byte{0xe5})
	require.NoError(t, err)
	_, err = w.Keeper.ServiceAgreementAccept(w.Context, w.Provider, [32]byte{0xe3}, [32]byte{0xe6})
	require.NoError(t, err)
	_, err = w.Keeper.ServiceCommitTask(w.Context, w.Provider, [32]byte{0xe7}, [32]byte{0xe3}, [32]byte{0xe8}, [32]byte{}, 1700000100000, 1000, [32]byte{0xe9})
	require.NoError(t, err)
	_, err = w.Keeper.ServiceProgressReport(w.Context, w.Provider, [32]byte{0xea}, [32]byte{0xe7}, [32]byte{0xeb}, [32]byte{0xec}, 10000, [32]byte{0xed})
	require.NoError(t, err)
	_, err = w.Keeper.ServiceDeliver(w.Context, w.Provider, [32]byte{0xee}, [32]byte{0xe3}, []types.ServiceDeliverable{{Hash: w.Spec, ArtifactSize: 2048, AvailabilityReference: [32]byte{0xef}}}, [32]byte{0xf1})
	require.NoError(t, err)
	cost := read(w.Context)
	// These unrelated malformed records would crash either former global scan.
	for i := uint64(0); i < 1024; i++ {
		var id [32]byte
		id[0] = 0xf0
		binary.BigEndian.PutUint64(id[24:], i)
		w.Context.KVStore(w.HostKey).Set(repairServiceKey("delivr:1", id), []byte{255})
		w.Context.KVStore(w.HostKey).Set(repairServiceKey("progrs:1", id), []byte{255})
	}
	beforeHost, beforePay := repairSnapshot(w.Context, w.HostKey), repairSnapshot(w.Context, w.PayKey)
	require.Equal(t, cost, read(w.Context))
	require.Equal(t, beforeHost, repairSnapshot(w.Context, w.HostKey))
	require.Equal(t, beforePay, repairSnapshot(w.Context, w.PayKey))
	w.Reopen(t)
	require.Equal(t, cost, read(w.Context))
	// Explicit rebuild refuses corruption atomically instead of silently repairing.
	beforePay = repairSnapshot(w.Context, w.PayKey)
	require.ErrorIs(t, w.Keeper.RebuildServiceIndexes(w.Context), keeper.ErrServiceCorruptState)
	require.Equal(t, beforePay, repairSnapshot(w.Context, w.PayKey))
}

func TestRepairM_1304_ServiceRecovery(t *testing.T) {
	producer := keeper.OpenRepairM1304World(t, false)
	_, _, want := repairServiceProduce(t, producer)
	exported := producer.Keeper.ExportGenesis(producer.Context)
	recovery := producer.Keeper.ExportRecoveryGenesis(producer.Context)
	// Export a corrupt live record through the real recovery producer, obtaining
	// a valid envelope checksum; refusal must come from owned-record validation.
	offerKey := repairServiceKey("offer::1", [32]byte{0x81})
	original := bytes.Clone(producer.Context.KVStore(producer.HostKey).Get(offerKey))
	producer.Context.KVStore(producer.HostKey).Set(offerKey, []byte{1})
	corruptRecovery := producer.Keeper.ExportRecoveryGenesis(producer.Context)
	producer.Context.KVStore(producer.HostKey).Set(offerKey, original)

	for _, chunked := range []bool{false, true} {
		t.Run(fmt.Sprint(chunked), func(t *testing.T) {
			consumer := keeper.OpenRepairM1304World(t, false)
			beforeHost, beforePay := repairSnapshot(consumer.Context, consumer.HostKey), repairSnapshot(consumer.Context, consumer.PayKey)
			for n := 0; n < 2; n++ {
				require.ErrorIs(t, consumer.Keeper.InitRecoveryGenesis(consumer.Context, *corruptRecovery), types.ErrInvalidGenesis)
			}
			require.Equal(t, beforeHost, repairSnapshot(consumer.Context, consumer.HostKey))
			require.Equal(t, beforePay, repairSnapshot(consumer.Context, consumer.PayKey))
			// A poisoned derived index is replaced only by the explicit import rebuild.
			key := append([]byte("service/latest-delivery/v1/"), consumer.Agreement[:]...)
			consumer.Context.KVStore(consumer.PayKey).Set(key, []byte{255})
			if chunked {
				wire, err := json.Marshal(recovery)
				require.NoError(t, err)
				var g keeper.RecoveryGenesis
				require.NoError(t, json.Unmarshal(wire, &g))
				require.NoError(t, consumer.Keeper.InitRecoveryGenesis(consumer.Context, g))
			} else {
				wire, err := json.Marshal(exported)
				require.NoError(t, err)
				var g types.GenesisState
				require.NoError(t, json.Unmarshal(wire, &g))
				require.NoError(t, consumer.Keeper.InitGenesis(consumer.Context, g))
			}
			consumer.Reopen(t)
			got, found, err := consumer.Keeper.ServiceViews().LatestDelivery(consumer.Context, consumer.Agreement)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, want, got)
			high, found, err := consumer.Keeper.ServiceViews().ProgressHighWater(consumer.Context, consumer.Commitment)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, uint32(8000), high)
			before := repairSnapshot(consumer.Context, consumer.PayKey)
			// Losing the version marker requires an explicit migration, never a query scan.
			consumer.Context.KVStore(consumer.PayKey).Delete([]byte("service/index-version"))
			_, found, err = consumer.Keeper.ServiceViews().LatestDelivery(consumer.Context, consumer.Agreement)
			require.False(t, found)
			require.ErrorIs(t, err, keeper.ErrServiceIndexesUnavailable)
			_, found, err = consumer.Keeper.ServiceViews().ProgressHighWater(consumer.Context, consumer.Commitment)
			require.False(t, found)
			require.ErrorIs(t, err, keeper.ErrServiceIndexesUnavailable)
			require.NoError(t, consumer.Keeper.RebuildServiceIndexes(consumer.Context))
			require.Equal(t, before, repairSnapshot(consumer.Context, consumer.PayKey))
			_, found, err = consumer.Keeper.ServiceViews().LatestDelivery(consumer.Context, [32]byte{0x99})
			require.NoError(t, err)
			require.False(t, found)
			high, found, err = consumer.Keeper.ServiceViews().ProgressHighWater(consumer.Context, [32]byte{0x99})
			require.NoError(t, err)
			require.False(t, found)
			require.Zero(t, high)
		})
	}
}

type repairAgent struct {
	world      *keeper.RepairM1304World
	precompile *agentcommon.Precompile
	db         *state.DBImpl
	machine    *vm.EVM
	composite  storetypes.CacheMultiStore
}

func newRepairAgent(t *testing.T) *repairAgent {
	t.Helper()
	w := keeper.OpenRepairM1304World(t, true)
	application := testkeeper.EVMTestApp
	appContext, _ := application.GetContextForDeliverTx(nil).CacheContext()
	stores := map[storetypes.StoreKey]storetypes.CacheWrapper{}
	keys := map[string]storetypes.StoreKey{}
	for _, source := range []sdk.MultiStore{appContext.MultiStore(), w.Context.MultiStore()} {
		for _, key := range source.StoreKeys() {
			var kv storetypes.KVStore
			func() { defer func() { _ = recover() }(); kv = source.GetKVStore(key) }()
			if kv != nil {
				stores[key] = kv
				keys[key.Name()] = key
			}
		}
	}
	composite := cachemulti.NewStore(dbm.NewMemDB(), stores, keys, nil, nil, nil, 0)
	ctx := w.Context.WithMultiStore(composite).WithBlockHeight(100).WithEventManager(sdk.NewEventManager())
	application.UpgradeKeeper.SetDone(ctx, "repair1304-merge")
	p, err := streams.NewPrecompile(w.Keeper, &application.UpgradeKeeper, "repair1304-merge")
	require.NoError(t, err)
	db := state.NewDBImpl(ctx, &application.EvmKeeper, false)
	block, err := application.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(application.EvmKeeper.ChainID(ctx))
	machine := vm.NewEVM(*block, db, chain, vm.Config{}, map[common.Address]vm.PrecompiledContract{p.Address(): p})
	return &repairAgent{w, p, db, machine, composite}
}
func (h *repairAgent) call(t *testing.T, caller common.Address, name string, args ...interface{}) []interface{} {
	t.Helper()
	input, err := h.precompile.GetABI().Pack(name, args...)
	require.NoError(t, err)
	out, _, err := h.machine.Call(caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "%s: %x", name, out)
	values, err := h.precompile.GetABI().Methods[name].Outputs.Unpack(out)
	require.NoError(t, err)
	return values
}
func (h *repairAgent) open(t *testing.T) *big.Int {
	t.Helper()
	return h.call(t, h.world.Provider, "open", h.world.Buyer, h.world.Token, big.NewInt(2), uint64(1700000000), uint64(1700000100), big.NewInt(100))[0].(*big.Int)
}
func (h *repairAgent) advance(seconds int64) {
	h.db.WithCtx(h.db.Ctx().WithBlockTime(time.Unix(1700000000+seconds, 0)).WithBlockHeight(h.db.Ctx().BlockHeight() + 1))
}
func (h *repairAgent) result(t *testing.T, id *big.Int, ordinal uint32, selector [4]byte, paid, refund uint64) {
	t.Helper()
	key := types.DeriveIdempotencyKey([32]byte(h.db.Ctx().EVMTxHash()), ordinal, selector)
	got, err := h.world.Keeper.GetStreamResult(h.db.Ctx(), key)
	require.NoError(t, err)
	var kernelID [32]byte
	id.FillBytes(kernelID[:])
	require.Equal(t, kernelID, got.StreamID)
	require.Equal(t, codec.U128{Lo: paid}, got.Paid)
	require.Equal(t, codec.U128{Lo: refund}, got.Refunded)
}

func TestRepairM_1304_DeclarationAgentOpen(t *testing.T) {
	h := newRepairAgent(t)
	id := h.open(t)
	auth, err := h.world.Keeper.AuthorityFor(h.db.Ctx(), h.world.Provider)
	require.NoError(t, err)
	_, payer, err := lxphost.MainAccountID(auth.AuthorityKey)
	require.NoError(t, err)
	want := types.DeriveStreamID([32]byte(h.db.Ctx().EVMTxHash()), 0, payer)
	require.Equal(t, new(big.Int).SetBytes(want[:]), id)
	record, err := h.world.Keeper.GetStream(h.db.Ctx(), want)
	require.NoError(t, err)
	require.Equal(t, uint64(1000), record.RateUnit)
	require.Equal(t, uint64(1700000000000), record.StartTimestamp)
	require.Equal(t, codec.U128{Lo: 100}, record.TotalCap)
	second := h.open(t)
	want = types.DeriveStreamID([32]byte(h.db.Ctx().EVMTxHash()), 1, payer)
	require.Equal(t, new(big.Int).SetBytes(want[:]), second)
	logs := h.db.GetAllLogs()
	require.Len(t, logs, 2)
	require.Equal(t, common.BigToHash(id), logs[0].Topics[1])
}
func TestRepairM_1304_DeclarationAgentSettle(t *testing.T) {
	h := newRepairAgent(t)
	id := h.open(t)
	h.advance(5)
	require.Equal(t, []interface{}{big.NewInt(10)}, h.call(t, h.world.Buyer, "settle", id))
	h.result(t, id, 1, [4]byte{0x8d, 0xf8, 0x28, 0x00}, 10, 0)
	view, found, err := h.world.Keeper.AgentStreamView(h.db.Ctx(), id)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, big.NewInt(10), view.Settled)
}
func TestRepairM_1304_DeclarationAgentClose(t *testing.T) {
	h := newRepairAgent(t)
	id := h.open(t)
	h.advance(5)
	require.Equal(t, []interface{}{big.NewInt(10)}, h.call(t, h.world.Provider, "close", id))
	h.result(t, id, 1, [4]byte{0x0a, 0xeb, 0xeb, 0x4e}, 10, 90)
	event := h.precompile.GetABI().Events["StreamClosed"]
	logs := h.db.GetAllLogs()
	require.Len(t, logs, 2)
	values, err := event.Inputs.NonIndexed().Unpack(logs[1].Data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{big.NewInt(10), big.NewInt(90)}, values)
	view, found, err := h.world.Keeper.AgentStreamView(h.db.Ctx(), id)
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, view.Active)
}
func TestRepairM_1304_DeclarationAgentRate(t *testing.T) {
	h := newRepairAgent(t)
	id := h.open(t)
	h.advance(5)
	require.Empty(t, h.call(t, h.world.Provider, "updateRate", id, big.NewInt(4)))
	h.result(t, id, 1, [4]byte{0x40, 0x5a, 0xbb, 0x41}, 10, 0)
	event := h.precompile.GetABI().Events["StreamRateUpdated"]
	logs := h.db.GetAllLogs()
	require.Len(t, logs, 3)
	values, err := event.Inputs.NonIndexed().Unpack(logs[1].Data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{big.NewInt(2), big.NewInt(4)}, values)
	h.advance(8)
	require.Equal(t, []interface{}{big.NewInt(12)}, h.call(t, h.world.Buyer, "settle", id))
}
func TestRepairM_1304_DeclarationAgentView(t *testing.T) {
	h := newRepairAgent(t)
	id := h.open(t)
	type streamTuple struct {
		Id                  *big.Int
		Payer, Payee, Token common.Address
		RatePerSecond, Cap  *big.Int
		StartTime, StopTime uint64
		Settled             *big.Int
		Active              bool
	}
	values := h.call(t, h.world.Buyer, "getStream", id)
	tuple := abi.ConvertType(values[0], new(streamTuple)).(*streamTuple)
	require.Equal(t, id, tuple.Id)
	require.Equal(t, h.world.Provider, tuple.Payer)
	require.Equal(t, h.world.Buyer, tuple.Payee)
	require.Equal(t, h.world.Token, tuple.Token)
	before, found, err := h.world.Keeper.AgentStreamView(h.db.Ctx(), id)
	require.NoError(t, err)
	require.True(t, found)
	_, err = h.db.Finalize()
	require.NoError(t, err)
	h.composite.Write()
	h.world.Reopen(t)
	got, found, err := h.world.Keeper.AgentStreamView(h.world.Context, id)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, before, got)
	var kernelID [32]byte
	id.FillBytes(kernelID[:])
	key := append([]byte(types.AgentStreamMetadataPrefix), kernelID[:]...)
	h.world.Context.KVStore(h.world.PayKey).Delete(key)
	_, found, err = h.world.Keeper.AgentStreamView(h.world.Context, id)
	require.False(t, found)
	require.ErrorContains(t, err, "address metadata")
}

func TestRepairM_1304_ServiceIndexWriterAtomicity(t *testing.T) {
	w := keeper.OpenRepairM1304World(t, false)
	report := [32]byte{0xa1}
	_, err := w.Keeper.ServiceProgressReport(w.Context, w.Provider, report, w.Commitment, [32]byte{0xa3}, [32]byte{0xa4}, 5000, [32]byte{0xa5})
	require.NoError(t, err)
	key := append([]byte("service/progress-high-water/v1/"), w.Commitment[:]...)
	w.Context.KVStore(w.PayKey).Set(key, []byte{255})
	beforeHost, beforePay := repairSnapshot(w.Context, w.HostKey), repairSnapshot(w.Context, w.PayKey)
	_, err = w.Keeper.ServiceProgressReport(w.Context, w.Provider, [32]byte{0xb1}, w.Commitment, [32]byte{0xa3}, [32]byte{0xa4}, 8000, [32]byte{0xb2})
	require.ErrorIs(t, err, keeper.ErrServiceCorruptState)
	require.Equal(t, beforeHost, repairSnapshot(w.Context, w.HostKey))
	require.Equal(t, beforePay, repairSnapshot(w.Context, w.PayKey))
	require.NoError(t, w.Keeper.RebuildServiceIndexes(w.Context))
	_, err = w.Keeper.ServiceProgressReport(w.Context, w.Provider, [32]byte{0xb1}, w.Commitment, [32]byte{0xa3}, [32]byte{0xa4}, 8000, [32]byte{0xb2})
	require.NoError(t, err)
	high, found, err := w.Keeper.ServiceViews().ProgressHighWater(w.Context, w.Commitment)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint32(8000), high)
}
