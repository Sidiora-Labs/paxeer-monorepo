package keeper

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/stretchr/testify/require"
)

func TestPayGenesisRehearsalKeepsKernelState(t *testing.T) {
	source := newPayHarness(t)
	escrow := payEscrowFundedArgs(t, source)
	buyer, provider := source.actors[0], source.actors[1]
	params := types.Params{EscrowEnabled: true, BudgetEnabled: true, StreamEnabled: true, ServiceEnabled: true}
	source.keeper.SetParams(source.ctx, params)
	for _, moduleID := range []uint16{
		types.EscrowModuleID, types.BudgetModuleID, types.StreamModuleID, types.ServiceModuleID, 6, 10, 11,
	} {
		key, err := types.KernelModuleEnableKey(moduleID)
		require.NoError(t, err)
		flag := make([]byte, 32)
		if moduleID <= types.ServiceModuleID || moduleID == 10 {
			flag[31] = 1
		}
		source.ctx.KVStore(source.hostKey).Set(key, flag)
	}
	deriveAccount := func(family string, id [32]byte) [32]byte {
		account, err := codec.DeriveAccountID([]byte("agent:" + string(buyer.did) + ":" + family + ":" + hex.EncodeToString(id[:])))
		require.NoError(t, err)
		return account
	}
	agreementID, offerID := [32]byte{0xa1}, [32]byte{0xa2}
	escrow.Amount = codec.U128{Lo: 200}
	escrow.AgreementReference = agreementID
	opened, err := source.keeper.EscrowOpen(source.ctx, buyer.address, escrow)
	require.NoError(t, err)
	require.Zero(t, opened.ResultCode)
	budgetID := [32]byte{0xb1}
	budget := BudgetCreateArgs{
		BudgetID: budgetID, BudgetAccount: deriveAccount("budget", budgetID), AssetID: escrow.AssetID,
		PurposeHash: [32]byte{0xb2}, PerPeriodLimit: codec.U128{Lo: 500}, CarryCap: codec.U128{Lo: 50},
		Amount: codec.U128{Lo: 300}, PeriodLengthMs: 60000, PeriodStartMs: uint64(payHarnessTimeMs),
		ExpiryMs: uint64(payHarnessTimeMs) + 120000, RevocationSequence: 7, RolloverPolicy: 2,
	}
	created, err := source.keeper.BudgetCreate(source.ctx, buyer.address, budget)
	require.NoError(t, err)
	require.Zero(t, created.ResultCode)
	streamID := [32]byte{0xc1}
	stream := types.StreamOpen{
		StreamID: streamID, StreamAccount: deriveAccount("stream", streamID), Recipient: escrow.Beneficiary,
		AssetID: escrow.AssetID, Mode: types.StreamModeTime, Rate: codec.U128{Lo: 2}, RateUnit: 1000,
		StartTimestamp: uint64(payHarnessTimeMs), EndTimestamp: uint64(payHarnessTimeMs) + 60000,
		TotalCap: codec.U128{Lo: 100}, InitialFunding: codec.U128{Lo: 100},
	}
	payload, err := types.EncodeStreamOpen(stream)
	require.NoError(t, err)
	streamOpened, err := source.keeper.Submit(source.ctx, buyer.address, "streamOpen", payload, streamID)
	require.NoError(t, err)
	require.Zero(t, streamOpened.ResultCode)
	payload, err = types.EncodeServiceOffer(offerID, escrow.AssetID, escrow.Amount, escrow.TermsHash,
		[32]byte{0xa3}, uint64(payHarnessTimeMs)+60000, 10000, escrow.DisputeWindowMs, 1, uint64(payHarnessTimeMs)+30000)
	require.NoError(t, err)
	offered, err := source.keeper.Submit(source.ctx, provider.address, "serviceOfferPublish", payload, offerID)
	require.NoError(t, err)
	require.Zero(t, offered.ResultCode)
	payload, err = types.EncodeServicePropose(agreementID, offerID, escrow.TermsHash, escrow.EscrowID)
	require.NoError(t, err)
	proposed, err := source.keeper.Submit(source.ctx, buyer.address, "serviceAgreementPropose", payload, agreementID)
	require.NoError(t, err)
	require.Zero(t, proposed.ResultCode)
	payload, err = types.EncodeServiceIdentifier(agreementID)
	require.NoError(t, err)
	accepted, err := source.keeper.Submit(source.ctx, provider.address, "serviceAgreementAccept", payload, [32]byte{0xa4})
	require.NoError(t, err)
	require.Zero(t, accepted.ResultCode)
	last, err := source.host.ReadLastSequence(source.ctx, buyer.did)
	require.NoError(t, err)
	require.Equal(t, accepted.GlobalSequence, last)
	require.Greater(t, last, source.sequence)

	hostBefore := payStoreSnapshot(source.ctx, source.hostKey)
	moduleBefore := payStoreSnapshot(source.ctx, source.storeKey)
	exported := source.keeper.ExportGenesis(source.ctx)
	require.NoError(t, exported.Validate())
	require.Equal(t, params, exported.Params)
	require.NotEmpty(t, exported.KernelRecords)
	for _, prefix := range [][]byte{types.EscrowKVPrefix, types.BudgetKVPrefix, types.StreamKVPrefix, types.ServiceKVPrefix} {
		count := 0
		for key := range hostBefore {
			if bytes.HasPrefix([]byte(key), prefix) {
				count++
			}
		}
		require.Positive(t, count, "source must hold a record for family %x", prefix)
	}
	exportBytes, err := json.Marshal(exported)
	require.NoError(t, err)
	var imported types.GenesisState
	require.NoError(t, json.Unmarshal(exportBytes, &imported))

	restored := newPayHarness(t)
	require.NotSame(t, source.host, restored.host)
	restoredLast, err := restored.host.ReadLastSequence(restored.ctx, buyer.did)
	require.NoError(t, err)
	require.Equal(t, restored.sequence, restoredLast)
	require.Less(t, restoredLast, last)
	restoredNext, err := lxphost.NextSequence(restored.ctx, restored.hostKey)
	require.NoError(t, err)
	require.Equal(t, restoredLast+1, restoredNext)
	for _, prefix := range [][]byte{types.EscrowKVPrefix, types.BudgetKVPrefix, types.StreamKVPrefix, types.ServiceKVPrefix} {
		for key := range payStoreSnapshot(restored.ctx, restored.hostKey) {
			require.False(t, bytes.HasPrefix([]byte(key), prefix))
		}
	}
	_, found := restored.keeper.GetEscrow(restored.ctx, escrow.EscrowID)
	require.False(t, found)
	_, found, err = restored.keeper.GetBudget(restored.ctx, budgetID)
	require.NoError(t, err)
	require.False(t, found)
	_, err = restored.keeper.GetStream(restored.ctx, streamID)
	require.Error(t, err)
	_, found = restored.keeper.GetOffer(restored.ctx, offerID)
	require.False(t, found)
	_, found = restored.keeper.GetAgreement(restored.ctx, agreementID)
	require.False(t, found)
	require.NoError(t, restored.keeper.InitGenesis(restored.ctx, imported))
	reexported := restored.keeper.ExportGenesis(restored.ctx)
	require.NoError(t, reexported.Validate())
	reexportBytes, err := json.Marshal(reexported)
	require.NoError(t, err)
	require.Equal(t, exportBytes, reexportBytes)
	require.Equal(t, moduleBefore, payStoreSnapshot(restored.ctx, restored.storeKey))

	t.Run("kernel_records", func(t *testing.T) {
		records := make(map[string][]byte, len(exported.KernelRecords))
		for _, record := range exported.KernelRecords {
			require.NotContains(t, records, string(record.Key))
			records[string(record.Key)] = bytes.Clone(record.Value)
			require.Equal(t, hostBefore[string(record.Key)], record.Value)
			require.Equal(t, record.Value, restored.ctx.KVStore(restored.hostKey).Get(record.Key))
			require.False(t, restored.ctx.KVStore(restored.storeKey).Has(record.Key))
		}
		for _, prefix := range [][]byte{
			types.EscrowKVPrefix, types.BudgetKVPrefix, types.StreamKVPrefix,
			types.ServiceKVPrefix, types.GovernanceKVPrefix,
		} {
			for key, value := range hostBefore {
				if !bytes.HasPrefix([]byte(key), prefix) {
					continue
				}
				t.Run(fmt.Sprintf("%x", key), func(t *testing.T) {
					require.Contains(t, records, key, "kernel record must be exported")
					require.Equal(t, value, records[key])
				})
			}
		}
	})
	t.Run("module_enable_records", func(t *testing.T) {
		for _, moduleID := range []uint16{types.EscrowModuleID, types.BudgetModuleID, types.StreamModuleID, types.ServiceModuleID, 6, 10, 11} {
			key, err := types.KernelModuleEnableKey(moduleID)
			require.NoError(t, err)
			require.Len(t, hostBefore[string(key)], 32)
			require.Equal(t, hostBefore[string(key)], restored.ctx.KVStore(restored.hostKey).Get(key))
		}
		for _, method := range []string{"escrowOpen", "budgetCreate", "streamOpen", "serviceOfferPublish"} {
			require.NoError(t, restored.keeper.CheckWrite(restored.ctx, method))
		}
	})
	t.Run("escrow", func(t *testing.T) {
		before, found := source.keeper.GetEscrow(source.ctx, escrow.EscrowID)
		require.True(t, found)
		require.Equal(t, escrow.Amount, before.LockedAmount)
		after, found := restored.keeper.GetEscrow(restored.ctx, escrow.EscrowID)
		require.True(t, found)
		require.Equal(t, before, after)
		beforeResult, found := source.keeper.GetEscrowResult(source.ctx, escrow.EscrowID)
		require.True(t, found)
		afterResult, found := restored.keeper.GetEscrowResult(restored.ctx, escrow.EscrowID)
		require.True(t, found)
		require.Equal(t, beforeResult, afterResult)
	})
	t.Run("budget", func(t *testing.T) {
		before, found, err := source.keeper.GetBudget(source.ctx, budgetID)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, budget.BudgetAccount, before.BudgetAccount)
		after, found, err := restored.keeper.GetBudget(restored.ctx, budgetID)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, before, after)
		beforeRemaining, err := source.keeper.BudgetRemaining(source.ctx, budgetID)
		require.NoError(t, err)
		require.Equal(t, "300", beforeRemaining.String())
	})
	t.Run("stream", func(t *testing.T) {
		before, err := source.keeper.GetStream(source.ctx, streamID)
		require.NoError(t, err)
		require.Equal(t, stream.StreamAccount, before.StreamAccount)
		after, err := restored.keeper.GetStream(restored.ctx, streamID)
		require.NoError(t, err)
		require.Equal(t, before, after)
		at := uint64(payHarnessTimeMs) + 5000
		beforeAccrued, err := source.host.ReadStreamAccrued(source.ctx, streamID, at)
		require.NoError(t, err)
		require.Equal(t, "10", beforeAccrued.String())
		afterAccrued, err := restored.host.ReadStreamAccrued(restored.ctx, streamID, at)
		require.NoError(t, err)
		require.Equal(t, beforeAccrued.String(), afterAccrued.String())
	})
	t.Run("service", func(t *testing.T) {
		beforeOffer, found := source.keeper.GetOffer(source.ctx, offerID)
		require.True(t, found)
		require.True(t, beforeOffer.Accepted)
		afterOffer, found := restored.keeper.GetOffer(restored.ctx, offerID)
		require.True(t, found)
		require.Equal(t, beforeOffer, afterOffer)
		before, found := source.keeper.GetAgreement(source.ctx, agreementID)
		require.True(t, found)
		require.Equal(t, escrow.EscrowID, before.EscrowID)
		require.Equal(t, accepted.GlobalSequence, before.AcceptedSequence)
		after, found := restored.keeper.GetAgreement(restored.ctx, agreementID)
		require.True(t, found)
		require.Equal(t, before, after)
	})
	t.Run("last_global_sequence", func(t *testing.T) {
		after, err := restored.host.ReadLastSequence(restored.ctx, buyer.did)
		require.NoError(t, err)
		require.Equal(t, restoredLast, after)
		next, err := lxphost.NextSequence(restored.ctx, restored.hostKey)
		require.NoError(t, err)
		require.Equal(t, restoredNext, next)
	})
	require.Equal(t, hostBefore, payStoreSnapshot(source.ctx, source.hostKey))
	require.Equal(t, moduleBefore, payStoreSnapshot(source.ctx, source.storeKey))
}
