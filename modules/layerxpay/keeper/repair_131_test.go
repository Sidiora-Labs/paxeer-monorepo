package keeper

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepair_131_SnapshotGrowth(t *testing.T) {
	h, stream, token := newAgentStreamHarness(t, "pointer")
	payer, payee := h.actors[0], h.actors[1]
	params := types.Params{EscrowEnabled: true, BudgetEnabled: true, StreamEnabled: true, ServiceEnabled: true}
	h.keeper.SetParams(h.ctx, params)
	for _, module := range []uint16{types.EscrowModuleID, types.BudgetModuleID, types.ServiceModuleID} {
		key, err := types.KernelModuleEnableKey(module)
		require.NoError(t, err)
		h.ctx.KVStore(h.hostKey).Set(key, payGateEnabledRecord())
	}
	derive := func(family string, id [32]byte) [32]byte {
		account, err := codec.DeriveAccountID([]byte("agent:" + string(payer.did) + ":" + family + ":" + hex.EncodeToString(id[:])))
		require.NoError(t, err)
		return account
	}
	escrowID, budgetID, offerID := [32]byte{0xc1}, [32]byte{0xc2}, [32]byte{0xc3}
	_, owner, err := lxphost.MainAccountID(payer.public)
	require.NoError(t, err)
	_, err = h.keeper.EscrowOpen(h.ctx, payer.address, EscrowOpenArgs{
		EscrowID: escrowID, Owner: owner, EscrowAccount: derive("escrow", escrowID), Beneficiary: stream.Recipient,
		AssetID: stream.AssetID, Amount: codec.U128{Lo: 10}, ExpiryMs: uint64(payHarnessTimeMs + 1000000),
	})
	require.NoError(t, err)
	_, err = h.keeper.BudgetCreate(h.ctx, payer.address, BudgetCreateArgs{
		BudgetID: budgetID, BudgetAccount: derive("budget", budgetID), AssetID: stream.AssetID,
		PerPeriodLimit: codec.U128{Lo: 20}, Amount: codec.U128{Lo: 20}, PeriodLengthMs: 60000,
		PeriodStartMs: uint64(payHarnessTimeMs), ExpiryMs: uint64(payHarnessTimeMs + 1000000), RolloverPolicy: 1,
	})
	require.NoError(t, err)
	_, err = h.keeper.ServiceOfferPublish(h.ctx, payee.address, offerID, stream.AssetID, codec.U128{Lo: 1},
		[32]byte{1}, [32]byte{2}, uint64(payHarnessTimeMs+60000), 1000, 1000, 1, uint64(payHarnessTimeMs+30000))
	require.NoError(t, err)
	views := make(map[string]AgentStream)
	for i := 0; i < types.MaxKernelRecords; i++ {
		id, err := h.keeper.OpenAgentStream(h.ctx, payer.address, payee.address, token, big.NewInt(1),
			uint64(payHarnessTimeMs/1000), uint64(payHarnessTimeMs/1000+60), big.NewInt(1))
		require.NoError(t, err, "admitted stream %d", i)
		var streamID, closeKey [32]byte
		id.FillBytes(streamID[:])
		binary.BigEndian.PutUint64(closeKey[24:], uint64(i+1))
		closeKey[0] = 0xe1
		_, err = h.keeper.StreamClose(h.ctx, payer.address, streamID, closeKey)
		require.NoError(t, err)
		view, found, err := h.keeper.AgentStreamView(h.ctx, id)
		require.NoError(t, err)
		require.True(t, found)
		require.False(t, view.Active)
		views[id.String()] = view
	}
	beforeHost, beforePay := payStoreSnapshot(h.ctx, h.hostKey), payStoreSnapshot(h.ctx, h.storeKey)
	id, err := h.keeper.OpenAgentStream(h.ctx, payer.address, payee.address, token, big.NewInt(1),
		uint64(payHarnessTimeMs/1000), uint64(payHarnessTimeMs/1000+60), big.NewInt(1))
	require.ErrorIs(t, err, lxphost.Code(-5))
	require.Nil(t, id)
	require.Equal(t, beforeHost, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, beforePay, payStoreSnapshot(h.ctx, h.storeKey))
	exported := h.keeper.ExportRecoveryGenesis(h.ctx)
	require.Greater(t, exported.KernelRecordCount, uint64(types.MaxKernelRecords))
	require.Equal(t, uint64(types.MaxKernelRecords), exported.MetadataRecordCount)
	require.NoError(t, exported.Validate())
	for _, chunk := range exported.Chunks {
		require.LessOrEqual(t, len(chunk.KernelRecords)+len(chunk.AgentStreamMetadata), types.MaxKernelRecords)
	}
	wire, err := json.Marshal(exported)
	require.NoError(t, err)
	var imported RecoveryGenesis
	require.NoError(t, json.Unmarshal(wire, &imported))
	require.Equal(t, exported, &imported)
	directory := t.TempDir()
	destination, stop := proofPayKeeperOpen(t, directory)
	beforeHost, beforePay = payStoreSnapshot(destination.ctx, destination.hostKey), payStoreSnapshot(destination.ctx, destination.storeKey)
	corrupt := func(mutator func(*RecoveryGenesis)) {
		var damaged RecoveryGenesis
		require.NoError(t, json.Unmarshal(wire, &damaged))
		mutator(&damaged)
		require.ErrorIs(t, destination.keeper.InitRecoveryGenesis(destination.ctx, damaged), types.ErrInvalidGenesis)
		require.Equal(t, beforeHost, payStoreSnapshot(destination.ctx, destination.hostKey))
		require.Equal(t, beforePay, payStoreSnapshot(destination.ctx, destination.storeKey))
	}
	corrupt(func(g *RecoveryGenesis) { g.Chunks = g.Chunks[:len(g.Chunks)-1] })
	corrupt(func(g *RecoveryGenesis) { g.MetadataRecordCount++ })
	corrupt(func(g *RecoveryGenesis) { g.Chunks[0], g.Chunks[1] = g.Chunks[1], g.Chunks[0] })
	corrupt(func(g *RecoveryGenesis) { g.Chunks[0].KernelRecords[0].Value[0] ^= 1 })
	corrupt(func(g *RecoveryGenesis) {
		g.Chunks[1].KernelRecords[0] = g.Chunks[0].KernelRecords[0]
		g.Checksum = g.checksum()
	})
	corrupt(func(g *RecoveryGenesis) {
		g.Chunks[0].KernelRecords[0].Value = make([]byte, types.MaxKernelValueBytes+1)
		g.Checksum = g.checksum()
	})
	corrupt(func(g *RecoveryGenesis) {
		g.Chunks[0].KernelRecords = append(g.Chunks[0].KernelRecords, g.Chunks[1].KernelRecords[0])
		g.Checksum = g.checksum()
	})
	corrupt(func(g *RecoveryGenesis) {
		last := g.Chunks[len(g.Chunks)-1].AgentStreamMetadata
		row := last[len(last)-1]
		row.Key = append([]byte(nil), row.Key...)
		for i := len(row.Key) - 1; i >= len(agentStreamMetadataPrefix); i-- {
			row.Key[i]++
			if row.Key[i] != 0 {
				break
			}
		}
		g.Chunks = append(g.Chunks, RecoveryChunk{AgentStreamMetadata: []kv.Pair{row}})
		g.MetadataRecordCount++
		g.Checksum = g.checksum()
	})
	require.NoError(t, destination.keeper.InitRecoveryGenesis(destination.ctx, imported))
	require.Equal(t, exported, destination.keeper.ExportRecoveryGenesis(destination.ctx))
	destination.cms.Commit(true)
	stop()
	recovered, _ := proofPayKeeperOpen(t, directory)
	require.Equal(t, exported, recovered.keeper.ExportRecoveryGenesis(recovered.ctx))
	for _, chunk := range imported.Chunks {
		for _, row := range chunk.KernelRecords {
			require.Equal(t, row.Value, recovered.ctx.KVStore(recovered.hostKey).Get(row.Key))
		}
		for _, row := range chunk.AgentStreamMetadata {
			require.Equal(t, row.Value, recovered.ctx.KVStore(recovered.storeKey).Get(row.Key))
		}
	}
	for key, want := range views {
		id, ok := new(big.Int).SetString(key, 10)
		require.True(t, ok)
		got, found, err := recovered.keeper.AgentStreamView(recovered.ctx, id)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, want, got)
	}
	beforeEscrow, _ := h.keeper.GetEscrow(h.ctx, escrowID)
	afterEscrow, found := recovered.keeper.GetEscrow(recovered.ctx, escrowID)
	require.True(t, found)
	require.Equal(t, beforeEscrow, afterEscrow)
	beforeBudget, _, err := h.keeper.GetBudget(h.ctx, budgetID)
	require.NoError(t, err)
	afterBudget, found, err := recovered.keeper.GetBudget(recovered.ctx, budgetID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, beforeBudget, afterBudget)
	beforeOffer, _ := h.keeper.GetOffer(h.ctx, offerID)
	afterOffer, found := recovered.keeper.GetOffer(recovered.ctx, offerID)
	require.True(t, found)
	require.Equal(t, beforeOffer, afterOffer)

	t.Run("declared_genesis_contract", func(t *testing.T) {
		// Keep the declared JSON import criterion executable until the out-of-scope
		// types and application boundary adopt the complete recovery envelope.
		legacyWire, err := json.Marshal(h.keeper.ExportGenesis(h.ctx))
		require.NoError(t, err)
		var legacy types.GenesisState
		require.NoError(t, json.Unmarshal(legacyWire, &legacy))
		assert.NoError(t, legacy.Validate(), "snapshot-growth: declared JSON genesis import must admit all live state")
		branch, _ := recovered.ctx.CacheContext()
		assert.NoError(t, recovered.keeper.InitGenesis(branch, legacy))
	})
}

func TestRepair_131_EscrowReceiptBinding(t *testing.T) {
	for _, operation := range []string{"release", "capture", "partial", "timeout", "split"} {
		t.Run(operation, func(t *testing.T) {
			directory := t.TempDir()
			h, stop := proofPayKeeperOpen(t, directory)
			h.actors = []payActor{h.registerActor(t, 1), h.registerActor(t, 2)}
			id, key := [32]byte{0xd1}, [32]byte{0xd2}
			payload := paySubmitFundedEscrow(t, h, id)
			if operation == "split" {
				copy(payload[128:160], payload[32:64])
				binary.BigEndian.PutUint64(payload[216:224], uint64(payHarnessTimeMs+10000))
			}
			_, err := h.keeper.Submit(h.ctx, h.actors[0].address, "escrowOpen", payload, id)
			require.NoError(t, err)
			hold, found := h.keeper.GetEscrow(h.ctx, id)
			require.True(t, found)
			if operation == "timeout" {
				h.ctx = h.ctx.WithBlockTime(time.UnixMilli(int64(hold.ExpiryMs)))
			}
			if operation == "split" {
				_, err = h.keeper.EscrowDisputeOpen(h.ctx, h.actors[1].address, id, [32]byte{0xd3})
				require.NoError(t, err)
			}
			settle := func(h *payHarness) (lxphost.Receipt, error) {
				switch operation {
				case "capture":
					return h.keeper.EscrowCapture(h.ctx, h.actors[1].address, id, codec.U128{Lo: 50}, key)
				case "partial":
					return h.keeper.EscrowPartialCapture(h.ctx, h.actors[1].address, id, codec.U128{Lo: 20}, key)
				case "timeout":
					return h.keeper.EscrowTimeout(h.ctx, h.actors[1].address, id, key)
				case "split":
					return h.keeper.EscrowDisputeResolve(h.ctx, h.actors[0].address, id, 3333, key)
				default:
					return h.keeper.EscrowRelease(h.ctx, h.actors[0].address, id, key)
				}
			}
			receipt, err := settle(h)
			require.NoError(t, err)
			result, found := h.keeper.GetEscrowResult(h.ctx, key)
			require.True(t, found)
			require.NotEqual(t, [32]byte{}, result.TransferSetRoot)
			checkReceipt := func(h *payHarness, receipt lxphost.Receipt) {
				canonical, err := codec.DecodeReceipt(receipt.Encoded)
				require.NoError(t, err)
				assert.Equal(t, result.TransferSetRoot, canonical.TransferSetRoot, "escrow-receipt-binding: canonical outer root must match economic result")
				require.NotEmpty(t, canonical.Effects)
				require.Equal(t, result.TransferSetRoot, canonical.Effects[len(canonical.Effects)-1].TransferSetRoot)
				require.Equal(t, receipt.Encoded, canonical.CanonicalBytes())
				stored, found, err := h.keeper.GetEscrowReceipt(h.ctx, receipt.GlobalSequence)
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, receipt.Encoded, stored.CanonicalBytes())
				require.Equal(t, result.TransferSetRoot, stored.TransferSetRoot, "durable outer receipt binds the economic result")
				require.Equal(t, canonical.Digest(), stored.Digest())
			}
			checkReceipt(h, receipt)
			ownerAfter, beneficiaryAfter, lockedAfter := uint64(200), uint64(0), uint64(0)
			switch operation {
			case "capture":
				ownerAfter, beneficiaryAfter = 150, 50
			case "partial":
				ownerAfter, beneficiaryAfter, lockedAfter = 150, 20, 30
			case "split":
				ownerAfter, beneficiaryAfter = 184, 16
			}
			require.Equal(t, hold.AssetID, result.AssetID)
			require.Equal(t, hold.EscrowAccount, result.From)
			if operation == "release" || operation == "timeout" {
				require.Equal(t, hold.Owner, result.To)
				require.Equal(t, codec.U128{Lo: 50}, result.Amount)
			} else {
				require.Equal(t, hold.Beneficiary, result.To)
				require.Equal(t, codec.U128{Lo: beneficiaryAfter}, result.Amount)
			}
			if operation == "split" {
				require.Equal(t, codec.U128{Lo: 34}, result.SecondaryAmount)
				require.Len(t, receipt.Transfers, 2)
			} else {
				require.Equal(t, codec.U128{}, result.SecondaryAmount)
				require.Len(t, receipt.Transfers, 1)
			}
			escrowSettleBalance(t, h, hold.Owner, hold.AssetID, ownerAfter)
			escrowSettleBalance(t, h, hold.Beneficiary, hold.AssetID, beneficiaryAfter)
			escrowSettleBalance(t, h, hold.EscrowAccount, hold.AssetID, lockedAfter)
			require.NotEmpty(t, receipt.Transfers)
			for _, transfer := range receipt.Transfers {
				require.Equal(t, hold.EscrowAccount, transfer.From)
				require.Equal(t, hold.AssetID, transfer.Asset)
				if transfer.To == hold.Owner {
					require.Equal(t, fmt.Sprint(ownerAfter-150), transfer.Amount.String())
				} else {
					require.Equal(t, hold.Beneficiary, transfer.To)
					require.Equal(t, fmt.Sprint(beneficiaryAfter), transfer.Amount.String())
				}
			}
			replay, err := settle(h)
			require.NoError(t, err)
			checkReceipt(h, replay)
			require.Empty(t, replay.Transfers)
			unchanged, found := h.keeper.GetEscrowResult(h.ctx, key)
			require.True(t, found)
			require.Equal(t, result, unchanged)
			h.cms.Commit(true)
			stop()
			reopened, _ := proofPayKeeperOpen(t, directory)
			reopened.actors = h.actors
			reopened.ctx = reopened.ctx.WithBlockTime(h.ctx.BlockTime())
			for i := 0; i < 2; i++ {
				got, found := reopened.keeper.GetEscrowResult(reopened.ctx, key)
				require.True(t, found)
				require.Equal(t, result, got)
				checkReceipt(reopened, receipt)
				checkReceipt(reopened, replay)
			}
			replay, err = settle(reopened)
			require.NoError(t, err)
			checkReceipt(reopened, replay)
			require.Empty(t, replay.Transfers)
			escrowSettleBalance(t, reopened, hold.Owner, hold.AssetID, ownerAfter)
			escrowSettleBalance(t, reopened, hold.Beneficiary, hold.AssetID, beneficiaryAfter)
			escrowSettleBalance(t, reopened, hold.EscrowAccount, hold.AssetID, lockedAfter)
		})
	}
}

func TestRepair_131_UnusedBudgetRevoke(t *testing.T) {
	for _, spent := range []uint64{0, 20} {
		t.Run(fmt.Sprint(spent), func(t *testing.T) {
			directory := t.TempDir()
			h, stop := proofPayKeeperOpen(t, directory)
			h.actors = []payActor{h.registerActor(t, 1), h.registerActor(t, 2)}
			id := [32]byte{0xe2}
			seed := paySubmitFundedEscrow(t, h, id)
			var owner, recipient, asset [32]byte
			copy(owner[:], seed[32:64])
			copy(recipient[:], seed[96:128])
			copy(asset[:], seed[160:192])
			account, err := codec.DeriveAccountID([]byte("agent:" + string(h.actors[0].did) + ":budget:" + hex.EncodeToString(id[:])))
			require.NoError(t, err)
			h.keeper.SetParams(h.ctx, types.Params{BudgetEnabled: true})
			gate, err := types.KernelModuleEnableKey(types.BudgetModuleID)
			require.NoError(t, err)
			h.ctx.KVStore(h.hostKey).Set(gate, payGateEnabledRecord())
			_, err = h.keeper.BudgetCreate(h.ctx, h.actors[0].address, BudgetCreateArgs{
				BudgetID: id, BudgetAccount: account, AssetID: asset, PerPeriodLimit: codec.U128{Lo: 100}, Amount: codec.U128{Lo: 100},
				PeriodLengthMs: 100000, PeriodStartMs: uint64(payHarnessTimeMs), ExpiryMs: uint64(payHarnessTimeMs + 1000000), RevocationSequence: 7, RolloverPolicy: 1,
			})
			require.NoError(t, err)
			if spent != 0 {
				payload, err := types.EncodeBudgetSpend(id, recipient, codec.U128{Lo: spent})
				require.NoError(t, err)
				_, err = h.keeper.Submit(h.ctx, h.actors[0].address, "budgetSpend", payload, [32]byte{0xe3})
				require.NoError(t, err)
			}
			before := payStoreSnapshot(h.ctx, h.hostKey)
			_, err = h.keeper.BudgetRevoke(h.ctx, h.actors[1].address, id, 8, [32]byte{0xe4})
			require.Error(t, err)
			require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
			_, err = h.keeper.BudgetRevoke(h.ctx, h.actors[0].address, id, 7, [32]byte{0xe4})
			require.ErrorIs(t, err, lxphost.Code(-212))
			require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
			receipt, err := h.keeper.BudgetRevoke(h.ctx, h.actors[0].address, id, 8, [32]byte{0xe4})
			require.NoError(t, err)
			require.Len(t, receipt.Transfers, 1)
			require.Equal(t, account, receipt.Transfers[0].From)
			require.Equal(t, owner, receipt.Transfers[0].To)
			require.Equal(t, fmt.Sprint(100-spent), receipt.Transfers[0].Amount.String())
			require.NotEmpty(t, receipt.Events)
			event := receipt.Events[len(receipt.Events)-1]
			require.Equal(t, uint16(9), event.Code)
			require.Equal(t, id[:], event.Body[:32])
			refund := (codec.U128{Lo: 100 - spent}).Bytes()
			require.Equal(t, refund[:], event.Body[32:48])
			require.Equal(t, uint64(8), binary.BigEndian.Uint64(event.Body[48:]))
			record := budgetExitRecord(t, h, id)
			require.True(t, record.Revoked)
			require.False(t, record.Closed)
			require.Equal(t, codec.U128{Lo: spent}, record.SpentThisPeriod)
			require.False(t, record.PerPeriodLimit.IsZero(), "terminal record remains canonical")
			budgetExitBalance(t, h, owner, asset, fmt.Sprint(200-spent))
			budgetExitBalance(t, h, account, asset, "0")
			budgetExitBalance(t, h, recipient, asset, fmt.Sprint(spent))
			remaining, err := h.keeper.BudgetRemaining(h.ctx, id)
			require.NoError(t, err)
			require.True(t, remaining.IsZero())
			h.cms.Commit(true)
			stop()
			reopened, _ := proofPayKeeperOpen(t, directory)
			require.Equal(t, record, budgetExitRecord(t, reopened, id))
			before = payStoreSnapshot(reopened.ctx, reopened.hostKey)
			for _, sequence := range []uint64{8, 9} {
				_, err = reopened.keeper.BudgetRevoke(reopened.ctx, h.actors[0].address, id, sequence, [32]byte{0xe4})
				require.ErrorIs(t, err, lxphost.Code(-716))
				require.Equal(t, before, payStoreSnapshot(reopened.ctx, reopened.hostKey))
			}
			payload, err := types.EncodeBudgetSpend(id, recipient, codec.U128{Lo: 1})
			require.NoError(t, err)
			_, err = reopened.keeper.Submit(reopened.ctx, h.actors[0].address, "budgetSpend", payload, [32]byte{0xe5})
			require.ErrorIs(t, err, lxphost.Code(-716))
			require.Equal(t, before, payStoreSnapshot(reopened.ctx, reopened.hostKey))
			budgetExitBalance(t, reopened, owner, asset, fmt.Sprint(200-spent))
			budgetExitBalance(t, reopened, account, asset, "0")
		})
	}
}

// Repair131BudgetWorld exposes real persistent producers to the external-package
// 0x101B proof without introducing an import cycle through the precompile.
type Repair131BudgetWorld struct {
	Keeper    *Keeper
	Context   sdk.Context
	Caller    common.Address
	Stranger  common.Address
	ID        [32]byte
	Owner     [32]byte
	Recipient [32]byte
	Asset     [32]byte
	Account   [32]byte
	directory string
	harness   *payHarness
	stop      func()
}

func OpenRepair131BudgetWorld(t *testing.T) *Repair131BudgetWorld {
	t.Helper()
	directory := t.TempDir()
	h, stop := proofPayKeeperOpen(t, directory)
	h.actors = []payActor{h.registerActor(t, 1), h.registerActor(t, 2)}
	w := &Repair131BudgetWorld{Keeper: &h.keeper, Context: h.ctx, Caller: h.actors[0].address, Stranger: h.actors[1].address,
		ID: [32]byte{0xf1}, directory: directory, harness: h, stop: stop}
	seed := paySubmitFundedEscrow(t, h, w.ID)
	copy(w.Owner[:], seed[32:64])
	copy(w.Recipient[:], seed[96:128])
	copy(w.Asset[:], seed[160:192])
	var err error
	w.Account, err = codec.DeriveAccountID([]byte("agent:" + string(h.actors[0].did) + ":budget:" + hex.EncodeToString(w.ID[:])))
	require.NoError(t, err)
	h.keeper.SetParams(h.ctx, types.Params{BudgetEnabled: true})
	gate, err := types.KernelModuleEnableKey(types.BudgetModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(gate, payGateEnabledRecord())
	return w
}

func (w *Repair131BudgetWorld) Reopen(t *testing.T) {
	t.Helper()
	w.harness.cms.Commit(true)
	w.stop()
	h, stop := proofPayKeeperOpen(t, w.directory)
	w.harness, w.stop, w.Keeper, w.Context = h, stop, &h.keeper, h.ctx
}
