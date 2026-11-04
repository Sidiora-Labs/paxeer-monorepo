package keeper

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// The exported fixture connects the package's committed real host/store/binding
// producers to external ABI consumers without substituting the keeper or host.
type RepairM1303World struct {
	Keeper              *Keeper
	Context             sdk.Context
	HostKey, PayKey     sdk.StoreKey
	Caller, Beneficiary common.Address
	ID, ResultKey       [32]byte
	Open                EscrowOpenArgs
	Receipt             lxphost.Receipt
	directory           string
	harness             *payHarness
	stop                func()
}

func OpenRepairM1303World(t *testing.T) *RepairM1303World {
	t.Helper()
	directory := t.TempDir()
	h, stop := proofPayKeeperOpen(t, directory)
	h.actors = []payActor{h.registerActor(t, 1), h.registerActor(t, 2)}
	a := payEscrowFundedArgs(t, h)
	w := &RepairM1303World{Keeper: &h.keeper, Context: h.ctx, HostKey: h.hostKey, PayKey: h.storeKey,
		Caller: h.actors[0].address, Beneficiary: h.actors[1].address, ID: a.EscrowID,
		ResultKey: [32]byte{0x93}, Open: a, directory: directory, harness: h, stop: stop}
	var err error
	w.Receipt, err = h.keeper.EscrowOpen(h.ctx, w.Caller, a)
	require.NoError(t, err)
	return w
}

func (w *RepairM1303World) Capture(t *testing.T) {
	t.Helper()
	var err error
	w.Receipt, err = w.Keeper.EscrowPartialCapture(w.Context, w.Beneficiary, w.ID, codec.U128{Lo: 25}, w.ResultKey)
	require.NoError(t, err)
}

func (w *RepairM1303World) Reopen(t *testing.T) {
	t.Helper()
	at := w.Context.BlockTime()
	w.harness.cms.Commit(true)
	w.stop()
	h, stop := proofPayKeeperOpen(t, w.directory)
	w.harness, w.stop, w.Keeper = h, stop, &h.keeper
	w.Context, w.HostKey, w.PayKey = h.ctx.WithBlockTime(at), h.hostKey, h.storeKey
}

func ProveRepairM1303Authority(t *testing.T) {
	t.Helper()
	h := newPayHarness(t)
	for _, actor := range h.actors {
		key, found := h.evm.GetLayerXDid(h.ctx, actor.address)
		require.True(t, found)
		authority, err := h.keeper.AuthorityFor(h.ctx, actor.address)
		require.NoError(t, err)
		require.Equal(t, actor.did, authority.ActorDID)
		require.Equal(t, key, authority.AuthorityKey)
		require.Equal(t, actor.authority(), authority.Kernel)
		identity, err := h.host.ReadIdentityState(h.ctx, authority.ActorDID)
		require.NoError(t, err)
		require.Equal(t, authority.AuthorityKey, identity.PrimaryKey)
	}
	_, err := h.keeper.AuthorityFor(h.ctx, common.Address{})
	require.ErrorIs(t, err, types.ErrUnboundCaller)
}

func ProveRepairM1303Constructor(t *testing.T) {
	t.Helper()
	h := newPayHarness(t)
	k, err := NewKeeper(h.cdc, h.storeKey, h.hostKey, h.host, h.evm, h.custody, "pay-governance")
	require.NoError(t, err)
	require.Same(t, h.host, k.Host())
	require.Same(t, h.evm, k.evm)
	require.Same(t, h.custody, k.custody)
	require.Equal(t, "pay-governance", k.Authority())
	for _, missing := range []string{"module", "hostKey", "host", "binding", "authority"} {
		t.Run(missing, func(t *testing.T) {
			var moduleKey, hostKey sdk.StoreKey = h.storeKey, h.hostKey
			host, authority := h.host, k.Authority()
			var binding EVMBinding = h.evm
			switch missing {
			case "module":
				moduleKey = nil
			case "hostKey":
				hostKey = nil
			case "host":
				host = nil
			case "binding":
				binding = nil
			case "authority":
				authority = ""
			}
			refused, err := NewKeeper(h.cdc, moduleKey, hostKey, host, binding, h.custody, authority)
			require.Error(t, err)
			require.Equal(t, Keeper{}, refused)
		})
	}
	// Custody is optional at construction, but asset-dependent consumers refuse
	// an unavailable reader rather than inventing an asset.
	optional, err := NewKeeper(h.cdc, h.storeKey, h.hostKey, h.host, h.evm, nil, k.Authority())
	require.NoError(t, err)
	_, err = optional.agentStreamAsset(h.ctx, common.HexToAddress("0x1303"))
	require.ErrorContains(t, err, "LXP_AGENT_STREAMS_UNKNOWN_TOKEN")
}

func ProveRepairM1303Keeper(t *testing.T) {
	t.Helper()
	for _, kind := range []string{"pointer", "native", "factory"} {
		t.Run(kind, func(t *testing.T) {
			h, fixture, token := newAgentStreamHarness(t, kind)
			var binding EVMBinding = h.evm
			var custody CustodyReader = h.custody
			k, err := NewKeeper(h.cdc, h.storeKey, h.hostKey, h.host, binding, custody, "pay-governance")
			require.NoError(t, err)
			key, found := binding.GetLayerXDid(h.ctx, h.actors[0].address)
			require.True(t, found)
			require.Equal(t, h.actors[0].public, key)
			asset, err := k.agentStreamAsset(h.ctx, token)
			require.NoError(t, err)
			require.Equal(t, fixture.AssetID, asset)
			if kind == "pointer" {
				mapping, found := custody.GetAssetByPointer(h.ctx, token)
				require.True(t, found)
				require.Equal(t, hex.EncodeToString(asset[:]), mapping.AssetId)
			} else {
				denom := "factory/" + sdk.AccAddress(h.actors[0].address.Bytes()).String() + "/stream"
				if kind == "native" {
					denom = sdk.MustGetBaseDenom()
				}
				mapping, found := custody.GetAssetByDenom(h.ctx, denom)
				require.True(t, found)
				require.Equal(t, hex.EncodeToString(asset[:]), mapping.AssetId)
			}
			id, err := k.OpenAgentStream(h.ctx, h.actors[0].address, h.actors[1].address, token,
				big.NewInt(2), uint64(payHarnessTimeMs/1000), uint64(payHarnessTimeMs/1000+60), big.NewInt(50))
			require.NoError(t, err)
			view, found, err := k.AgentStreamView(h.ctx, id)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, token, view.Token)
			require.Equal(t, big.NewInt(2), view.RatePerSecond)
			require.Equal(t, big.NewInt(50), view.Cap)
		})
	}
}

func ProveRepairM1303BudgetTime(t *testing.T) {
	t.Helper()
	for _, tc := range []struct {
		name             string
		policy           uint8
		spent            uint64
		offset           int64
		remaining, limit uint64
		refusal          string
	}{
		{"before_boundary", 2, 20, 9999, 80, 100, ""},
		{"exact_boundary_capped", 2, 20, 10000, 140, 140, ""},
		{"after_boundary_capped", 2, 20, 10001, 140, 140, ""},
		{"carry_below_cap", 2, 80, 10000, 120, 120, ""},
		{"exhausted_old_period", 2, 100, 10000, 100, 100, ""},
		{"skipped_periods_one_carry", 2, 20, 50000, 140, 140, ""},
		{"reset_policy", 1, 20, 10000, 100, 100, ""},
		{"last_unexpired_ms", 2, 20, 99999, 140, 140, ""},
		{"exact_expiry", 2, 20, 100000, 0, 0, "LXP_EXPIRED"},
		{"after_expiry", 2, 20, 100001, 0, 0, "LXP_EXPIRED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newPayHarness(t)
			id := [32]byte{0x94}
			asset, account, recipient := paySpendBudget(t, h, id)
			actor := h.actors[0].address
			_, err := h.keeper.BudgetFund(h.ctx, actor, id, codec.U128{Lo: 50}, [32]byte{0x95})
			require.NoError(t, err)
			cap := codec.U128{}
			if tc.policy == 2 {
				cap.Lo = 40
			}
			_, err = h.keeper.BudgetAmend(h.ctx, actor, id, codec.U128{Lo: 100}, cap,
				uint64(payHarnessTimeMs+100000), tc.policy, [32]byte{0x96})
			require.NoError(t, err)
			_, err = h.keeper.BudgetSpend(h.ctx, actor, id, recipient, codec.U128{Lo: tc.spent})
			require.NoError(t, err)
			ctx := h.ctx.WithBlockTime(time.UnixMilli(payHarnessTimeMs + tc.offset))
			hostBefore, payBefore := payStoreSnapshot(ctx, h.hostKey), payStoreSnapshot(ctx, h.storeKey)
			amount, err := h.keeper.BudgetRemaining(ctx, id)
			require.NoError(t, err)
			require.Equal(t, sdk.NewIntFromUint64(tc.remaining), amount)
			require.Equal(t, hostBefore, payStoreSnapshot(ctx, h.hostKey))
			require.Equal(t, payBefore, payStoreSnapshot(ctx, h.storeKey))
			spend := tc.remaining
			if spend == 0 {
				spend = 1
			}
			balance, err := h.host.ReadBalance(ctx, recipient, asset)
			require.NoError(t, err)
			_, err = h.keeper.BudgetSpend(ctx, actor, id, recipient, codec.U128{Lo: spend})
			if tc.refusal != "" {
				require.ErrorContains(t, err, tc.refusal)
				require.Equal(t, hostBefore, payStoreSnapshot(ctx, h.hostKey))
				return
			}
			require.NoError(t, err)
			after, found, err := h.keeper.GetBudget(ctx, id)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, codec.U128{Lo: tc.limit}, after.PerPeriodLimit)
			require.Equal(t, uint64(payHarnessTimeMs)+(uint64(tc.offset)/10000)*10000, after.PeriodStartMs)
			afterBalance, err := h.host.ReadBalance(ctx, recipient, asset)
			require.NoError(t, err)
			require.Equal(t, balance.Add(amount), afterBalance)
			left, err := h.keeper.BudgetRemaining(ctx, id)
			require.NoError(t, err)
			require.True(t, left.IsZero())
			budgetBalance, err := h.host.ReadBalance(ctx, account, asset)
			require.NoError(t, err)
			require.Equal(t, sdk.NewIntFromUint64(200-tc.spent-tc.remaining), budgetBalance)
		})
	}
	t.Run("balance_cap_and_timestamp_refusal", func(t *testing.T) {
		h := newPayHarness(t)
		id := [32]byte{0x97}
		_, _, recipient := paySpendBudget(t, h, id)
		_, err := h.keeper.BudgetSpend(h.ctx, h.actors[0].address, id, recipient, codec.U128{Lo: 100})
		require.NoError(t, err)
		ctx := h.ctx.WithBlockTime(time.UnixMilli(payHarnessTimeMs + 10000))
		before := payStoreSnapshot(ctx, h.hostKey)
		amount, err := h.keeper.BudgetRemaining(ctx, id)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(50), amount)
		require.Equal(t, before, payStoreSnapshot(ctx, h.hostKey))
		_, err = h.keeper.BudgetSpend(ctx, h.actors[0].address, id, recipient, codec.U128{Lo: 50})
		require.NoError(t, err)
		regressed := h.ctx.WithBlockTime(time.UnixMilli(payHarnessTimeMs - 1))
		_, err = h.keeper.BudgetRemaining(regressed, id)
		require.ErrorIs(t, err, lxphost.Code(-802))
		_, err = h.keeper.BudgetRemaining(h.ctx.WithBlockTime(time.UnixMilli(-1)), id)
		require.ErrorIs(t, err, lxphost.Code(-802))
	})
}

func ProveRepairM1303EscrowRecovery(t *testing.T) {
	t.Helper()
	w := OpenRepairM1303World(t)
	w.Capture(t)
	good := w.Keeper.ExportGenesis(w.Context)
	wire, err := json.Marshal(good)
	require.NoError(t, err)
	var decoded types.GenesisState
	require.NoError(t, json.Unmarshal(wire, &decoded))
	require.Equal(t, good, &decoded)
	good = &decoded
	recovery := w.Keeper.ExportRecoveryGenesis(w.Context)
	require.NoError(t, ValidateOwnedGenesis(*good))
	require.NoError(t, recovery.Validate())
	for _, family := range []string{"hold:", "result:"} {
		for _, mutation := range []string{"short", "trailing", "zero_id", "state", "key_short", "key_zero", "unknown_prefix", "version_or_id", "orphan_or_mismatch"} {
			t.Run(family+mutation, func(t *testing.T) {
				// Export afresh: each malformed input starts as bytes from the real producer.
				bad := w.Keeper.ExportGenesis(w.Context)
				changed := false
				for i := range bad.KernelRecords {
					pair := &bad.KernelRecords[i]
					if !bytes.HasPrefix(pair.Key, types.KernelKVKey(types.EscrowModuleID, []byte(family))) {
						continue
					}
					switch mutation {
					case "short":
						pair.Value = pair.Value[:len(pair.Value)-1]
					case "trailing":
						pair.Value = append(pair.Value, 0)
					case "zero_id":
						clear(pair.Value[:32])
					case "state":
						if family == "hold:" {
							pair.Value[224] = 0
						} else {
							pair.Value[34] = 0
						}
					case "key_short":
						pair.Key = pair.Key[:len(pair.Key)-1]
					case "key_zero":
						clear(pair.Key[len(pair.Key)-32:])
					case "unknown_prefix":
						pair.Key[3] ^= 1
					case "version_or_id":
						if family == "hold:" {
							pair.Value[0] ^= 1
						} else {
							pair.Value[243] = 1
						}
					case "orphan_or_mismatch":
						if family == "hold:" {
							pair.Value[160] ^= 1
						} else {
							pair.Value[0] ^= 1
						}
					}
					changed = true
					break
				}
				require.True(t, changed)
				beforeHost, beforePay := payStoreSnapshot(w.Context, w.HostKey), payStoreSnapshot(w.Context, w.PayKey)
				require.ErrorIs(t, ValidateOwnedGenesis(*bad), types.ErrInvalidGenesis)
				require.ErrorIs(t, w.Keeper.InitGenesis(w.Context, *bad), types.ErrInvalidGenesis)
				require.Equal(t, beforeHost, payStoreSnapshot(w.Context, w.HostKey))
				require.Equal(t, beforePay, payStoreSnapshot(w.Context, w.PayKey))
				// The chunked path must validate bytes even with a valid envelope checksum.
				badRecovery := w.Keeper.ExportRecoveryGenesis(w.Context)
				for ci := range badRecovery.Chunks {
					for ri := range badRecovery.Chunks[ci].KernelRecords {
						row := &badRecovery.Chunks[ci].KernelRecords[ri]
						for i, original := range good.KernelRecords {
							if bytes.Equal(row.Key, original.Key) {
								*row = bad.KernelRecords[i]
								break
							}
						}
					}
				}
				badRecovery.Checksum = badRecovery.checksum()
				require.ErrorIs(t, w.Keeper.InitRecoveryGenesis(w.Context, *badRecovery), types.ErrInvalidGenesis)
				require.Equal(t, beforeHost, payStoreSnapshot(w.Context, w.HostKey))
				require.Equal(t, beforePay, payStoreSnapshot(w.Context, w.PayKey))
			})
		}
	}
	for _, chunked := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "chunks"}[chunked], func(t *testing.T) {
			directory := t.TempDir()
			destination, stop := proofPayKeeperOpen(t, directory)
			if chunked {
				require.NoError(t, destination.keeper.InitRecoveryGenesis(destination.ctx, *recovery))
			} else {
				require.NoError(t, destination.keeper.InitGenesis(destination.ctx, *good))
			}
			destination.cms.Commit(true)
			stop()
			reopened, _ := proofPayKeeperOpen(t, directory)
			require.Equal(t, good, reopened.keeper.ExportGenesis(reopened.ctx))
			want, found, err := w.Keeper.GetEscrow(w.Context, w.ID)
			require.NoError(t, err)
			require.True(t, found)
			got, found, err := reopened.keeper.GetEscrow(reopened.ctx, w.ID)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, want, got)
			wantResult, found, err := w.Keeper.GetEscrowResult(w.Context, w.ResultKey)
			require.NoError(t, err)
			require.True(t, found)
			gotResult, found, err := reopened.keeper.GetEscrowResult(reopened.ctx, w.ResultKey)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, wantResult, gotResult)
		})
	}
}

func ProveRepairM1303ServiceEscrowCorruption(t *testing.T) {
	t.Helper()
	h, _, escrowID, agreementID, disputeID := serviceDisputeSetup(t, true, false)
	key := types.KernelKVKey(types.EscrowModuleID, append([]byte("hold:"), escrowID[:]...))
	wire := bytes.Clone(h.ctx.KVStore(h.hostKey).Get(key))
	h.ctx.KVStore(h.hostKey).Set(key, wire[:len(wire)-1])
	beforeHost, beforePay := payStoreSnapshot(h.ctx, h.hostKey), payStoreSnapshot(h.ctx, h.storeKey)
	var err error
	require.NotPanics(t, func() {
		_, err = h.keeper.ServiceDisputeOpen(h.ctx, h.actors[0].address, disputeID, agreementID, [][32]byte{{0xa1}})
	})
	require.ErrorIs(t, err, ErrEscrowCorruptState)
	require.Equal(t, beforeHost, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, beforePay, payStoreSnapshot(h.ctx, h.storeKey))
}
