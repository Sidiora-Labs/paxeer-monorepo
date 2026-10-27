package keeper

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestBudgetViewsReadKernelRecord(t *testing.T) {
	h := newPayHarness(t)
	enableKey, err := types.KernelModuleEnableKey(types.BudgetModuleID)
	require.NoError(t, err)
	enabled := make([]byte, 32)
	enabled[31] = 1
	h.ctx.KVStore(h.hostKey).Set(enableKey, enabled)
	owner, recipient := h.actors[0], h.actors[1]
	ownerName, ownerAccount, err := lxphost.MainAccountID(owner.public)
	require.NoError(t, err)
	ownerRecord := lxphost.AccountRecord{
		ID: ownerAccount, Name: ownerName, Kind: 1, Balance: sdk.ZeroInt(),
		CreatedAtSequence: h.sequence, AuthorityKey: owner.public, HasAuthorityKey: true,
	}
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	row := field(nil, ownerRecord.ID[:])
	row = field(row, []byte(ownerRecord.Name))
	row = append(row, ownerRecord.Kind)
	row = append(row, make([]byte, 16)...)
	row = field(row, ownerRecord.AssetID[:])
	row = append(row, 0)
	row = binary.BigEndian.AppendUint64(row, ownerRecord.NextSequence)
	row = binary.BigEndian.AppendUint64(row, ownerRecord.CreatedAtSequence)
	row = append(row, 0, 0)
	row = field(row, ownerRecord.AuthorityKey[:])
	row = append(row, 1)
	h.ctx.KVStore(h.hostKey).Set(append([]byte{6}, ownerAccount[:]...), row)
	restoredOwner, err := h.host.ReadAccount(h.ctx, ownerName, ownerAccount)
	require.NoError(t, err)
	require.Equal(t, ownerRecord, restoredOwner)
	sequences := map[[32]byte]uint64{owner.id: 1, recipient.id: 1}
	execute := func(actor payActor, activityType uint32, payload []byte) {
		t.Helper()
		activity := lxphost.Activity{
			ProtocolVersion: 3, Type: activityType, ActorDID: actor.did, AuthorityKey: actor.public,
			AccountSequence: sequences[actor.id], NotBefore: uint64(payHarnessTimeMs - 1000),
			NotAfter: uint64(payHarnessTimeMs + 1000), Payload: payload,
		}
		material := binary.BigEndian.AppendUint64(append([]byte(nil), payload...), h.sequence+1)
		activity.IdempotencyKey = sha256.Sum256(material)
		hash, err := h.host.SigningHash(activity)
		require.NoError(t, err)
		activity.Signature = ed25519.Sign(actor.private, hash[:])
		h.sequence++
		h.ctx = lxphost.WithSequence(h.ctx, h.sequence)
		authority := actor.authority()
		if activityType>>16 == 3 {
			_, authority.Principal, err = lxphost.MainAccountID(actor.public)
			require.NoError(t, err)
		}
		receipt, err := h.host.Execute(h.ctx, activity, authority)
		require.NoError(t, err, "activity %#x", activityType)
		require.Equal(t, lxphost.Code(0), receipt.ResultCode)
		sequences[actor.id]++
	}

	salt := bytes.Repeat([]byte{0x33}, 32)
	material := append([]byte("LX:ASSET:v1"), owner.id[:]...)
	assetID := sha256.Sum256(append(material, salt...))
	registration := append([]byte{0, 1}, assetID[:]...)
	registration = append(registration, salt...)
	registration = append(registration, 1, 'T', 1, 'T', 6)
	registration = binary.BigEndian.AppendUint64(registration, 0)
	registration = binary.BigEndian.AppendUint64(registration, 1000)
	registration = append(registration, 1, 0)
	execute(owner, 0x00010001, registration)
	execute(owner, 0x00010004, append([]byte{0, 1}, assetID[:]...))
	execute(recipient, 0x00010004, append([]byte{0, 1}, assetID[:]...))
	_, sourceID, err := lxphost.AssetAccountID(owner.public, assetID)
	require.NoError(t, err)
	_, recipientID, err := lxphost.AssetAccountID(recipient.public, assetID)
	require.NoError(t, err)
	mint := append(append([]byte{0, 1}, assetID[:]...), sourceID[:]...)
	mint = binary.BigEndian.AppendUint64(mint, 0)
	mint = binary.BigEndian.AppendUint64(mint, 1000)
	execute(owner, 0x0001000a, mint)

	budgetID := sha256.Sum256([]byte("budget views"))
	budgetName := "agent:" + string(owner.did) + ":budget:" + hex.EncodeToString(budgetID[:])
	budgetAccount, err := codec.DeriveAccountID([]byte(budgetName))
	require.NoError(t, err)
	purpose := sha256.Sum256([]byte("budget purpose"))
	create, err := types.EncodeBudgetCreateNative(budgetID, budgetAccount, assetID, purpose,
		codec.U128{Lo: 500}, codec.U128{Lo: 50}, codec.U128{Lo: 500},
		60000, uint64(payHarnessTimeMs), uint64(payHarnessTimeMs+120000), 7, 2, sourceID, 0)
	require.NoError(t, err)
	execute(owner, 0x00030001, create)
	delegate, err := types.EncodeBudgetDelegate(budgetID, recipient.id)
	require.NoError(t, err)
	execute(owner, 0x00030004, delegate)
	spend, err := types.EncodeBudgetSpend(budgetID, recipientID, codec.U128{Lo: 120})
	require.NoError(t, err)
	execute(owner, 0x00030006, spend)

	key := append([]byte("budget:"), budgetID[:]...)
	require.Len(t, key, 39)
	storeKey := types.KernelKVKey(3, key)
	require.Equal(t, append([]byte{1, 0, 3}, key...), storeKey)
	raw := bytes.Clone(h.ctx.KVStore(h.hostKey).Get(storeKey))
	require.NotEmpty(t, raw)
	want := types.BudgetRecord{
		NativeSource: true, SourceAccount: sourceID, BudgetID: budgetID, Owner: ownerAccount,
		BudgetAccount: budgetAccount, AssetID: assetID, PurposeHash: purpose,
		PerPeriodLimit: codec.U128{Lo: 500}, ConfiguredPeriodLimit: codec.U128{Lo: 500},
		CarryCap: codec.U128{Lo: 50}, SpentThisPeriod: codec.U128{Lo: 120},
		PeriodLengthMs: 60000, PeriodStartMs: uint64(payHarnessTimeMs),
		ExpiryMs: uint64(payHarnessTimeMs + 120000), RevocationSequence: 7,
		RolloverPolicy: 2, Delegates: [][32]byte{recipient.id},
	}
	decoded, err := types.DecodeBudgetRecord(raw)
	require.NoError(t, err)
	require.Equal(t, want, decoded)

	assertViews := func(ctx sdk.Context, k Keeper, expected types.BudgetRecord, remaining sdk.Int) {
		t.Helper()
		hostBefore := payStoreSnapshot(ctx, h.hostKey)
		payBefore := payStoreSnapshot(ctx, h.storeKey)
		record, found, err := k.GetBudget(ctx, budgetID)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, expected, record)
		amount, err := k.BudgetRemaining(ctx, budgetID)
		require.NoError(t, err)
		require.Equal(t, remaining, amount)
		require.Equal(t, hostBefore, payStoreSnapshot(ctx, h.hostKey))
		require.Equal(t, payBefore, payStoreSnapshot(ctx, h.storeKey))
	}
	assertViews(h.ctx, h.keeper, want, sdk.NewInt(380))

	t.Run("host store and fresh keeper", func(t *testing.T) {
		h.ctx.KVStore(h.storeKey).Set(storeKey, []byte("module store decoy"))
		h.ctx.KVStore(h.hostKey).Set(types.KernelKVKey(2, key), []byte("other module decoy"))
		fresh, err := NewKeeper(h.cdc, h.storeKey, h.hostKey, h.host, h.evm, h.custody, "pay-governance")
		require.NoError(t, err)
		assertViews(h.ctx, fresh, want, sdk.NewInt(380))
		h.ctx.KVStore(h.storeKey).Delete(storeKey)
		h.ctx.KVStore(h.hostKey).Delete(types.KernelKVKey(2, key))
	})

	t.Run("unknown id", func(t *testing.T) {
		before := payStoreSnapshot(h.ctx, h.hostKey)
		for _, id := range [][32]byte{{}, sha256.Sum256([]byte("unknown budget"))} {
			record, found, err := h.keeper.GetBudget(h.ctx, id)
			require.NoError(t, err)
			require.False(t, found)
			require.Equal(t, types.BudgetRecord{}, record)
			remaining, err := h.keeper.BudgetRemaining(h.ctx, id)
			require.NoError(t, err)
			require.Equal(t, sdk.ZeroInt(), remaining)
		}
		require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	})

	t.Run("current context and full width allowance", func(t *testing.T) {
		branch, _ := h.ctx.CacheContext()
		updated := bytes.Clone(raw)
		binary.BigEndian.PutUint64(updated[162:170], 1)
		binary.BigEndian.PutUint64(updated[170:178], 5)
		binary.BigEndian.PutUint64(updated[210:218], 0)
		binary.BigEndian.PutUint64(updated[218:226], 10)
		branch.KVStore(h.hostKey).Set(storeKey, updated)
		branchWant := want
		branchWant.PerPeriodLimit = codec.U128{Hi: 1, Lo: 5}
		branchWant.SpentThisPeriod = codec.U128{Lo: 10}
		assertViews(branch, h.keeper, branchWant, sdk.NewInt(380))
		assertViews(h.ctx, h.keeper, want, sdk.NewInt(380))
	})

	t.Run("exhausted allowance", func(t *testing.T) {
		branch, _ := h.ctx.CacheContext()
		updated := bytes.Clone(raw)
		binary.BigEndian.PutUint64(updated[218:226], 500)
		branch.KVStore(h.hostKey).Set(storeKey, updated)
		exhausted := want
		exhausted.SpentThisPeriod = codec.U128{Lo: 500}
		assertViews(branch, h.keeper, exhausted, sdk.ZeroInt())
	})

	t.Run("missing budget account surfaces error", func(t *testing.T) {
		branch, _ := h.ctx.CacheContext()
		branch.KVStore(h.hostKey).Delete(append([]byte{6}, budgetAccount[:]...))
		before := payStoreSnapshot(branch, h.hostKey)
		remaining, err := h.keeper.BudgetRemaining(branch, budgetID)
		require.Error(t, err)
		require.Equal(t, sdk.ZeroInt(), remaining)
		require.Equal(t, before, payStoreSnapshot(branch, h.hostKey))
	})

	t.Run("invalid records surface errors", func(t *testing.T) {
		for name, mutate := range map[string]func([]byte) []byte{
			"truncated": func(b []byte) []byte { return b[:len(b)-1] },
			"version":   func(b []byte) []byte { b[1] = 3; return b },
			"wrong id":  func(b []byte) []byte { b[2] ^= 1; return b },
		} {
			t.Run(name, func(t *testing.T) {
				branch, _ := h.ctx.CacheContext()
				branch.KVStore(h.hostKey).Set(storeKey, mutate(bytes.Clone(raw)))
				before := payStoreSnapshot(branch, h.hostKey)
				record, found, err := h.keeper.GetBudget(branch, budgetID)
				require.Error(t, err)
				require.False(t, found)
				require.Equal(t, types.BudgetRecord{}, record)
				remaining, err := h.keeper.BudgetRemaining(branch, budgetID)
				require.Error(t, err)
				require.Equal(t, sdk.ZeroInt(), remaining)
				require.Equal(t, before, payStoreSnapshot(branch, h.hostKey))
			})
		}
		branch, _ := h.ctx.CacheContext()
		overspent := bytes.Clone(raw)
		binary.BigEndian.PutUint64(overspent[218:226], 501)
		branch.KVStore(h.hostKey).Set(storeKey, overspent)
		remaining, err := h.keeper.BudgetRemaining(branch, budgetID)
		require.Equal(t, lxphost.Code(-1001), err)
		require.Equal(t, sdk.ZeroInt(), remaining)
	})

	fund, err := types.EncodeBudgetFundNative(budgetID, codec.U128{Lo: 200}, 1)
	require.NoError(t, err)
	execute(owner, 0x00030002, fund)
	assertViews(h.ctx, h.keeper, want, sdk.NewInt(380))
	defund, err := types.EncodeBudgetDefund(budgetID, codec.U128{Lo: 300})
	require.NoError(t, err)
	execute(owner, 0x00030008, defund)
	defunded := want
	defunded.PerPeriodLimit = codec.U128{Lo: 400}
	assertViews(h.ctx, h.keeper, defunded, sdk.NewInt(280))
}
