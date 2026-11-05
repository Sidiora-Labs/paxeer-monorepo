package keeper

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func payBudgetArgs(t *testing.T, h *payHarness) BudgetCreateArgs {
	t.Helper()
	funded := payEscrowFundedArgs(t, h)
	h.keeper.SetParams(h.ctx, types.Params{BudgetEnabled: true})
	enableKey, err := types.KernelModuleEnableKey(types.BudgetModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(enableKey, payGateEnabledRecord())
	id := [32]byte{0xb1}
	account, err := codec.DeriveAccountID([]byte("agent:" + string(h.actors[0].did) + ":budget:" + hex.EncodeToString(id[:])))
	require.NoError(t, err)
	return BudgetCreateArgs{
		BudgetID: id, BudgetAccount: account, AssetID: funded.AssetID, PurposeHash: [32]byte{0xb2},
		PerPeriodLimit: codec.U128{Lo: 900}, CarryCap: codec.U128{Lo: 50}, Amount: codec.U128{Lo: 600},
		PeriodLengthMs: 60000, PeriodStartMs: uint64(payHarnessTimeMs),
		ExpiryMs: uint64(payHarnessTimeMs) + 120000, RevocationSequence: 7, RolloverPolicy: 2,
	}
}

func payBudgetExpectedReceipt(t *testing.T, h *payHarness, activityType uint32, payload []byte, key [32]byte) lxphost.Receipt {
	t.Helper()
	actor := h.actors[0]
	last, err := h.host.ReadLastSequence(h.ctx, actor.did)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(h.ctx, actor.did)
	require.NoError(t, err)
	_, principal, err := lxphost.MainAccountID(actor.public)
	require.NoError(t, err)
	authority := actor.authority()
	authority.Bound, authority.Principal = true, principal
	branch, _ := h.ctx.CacheContext()
	receipt, err := h.host.HostExecute(lxphost.WithSequence(branch, last+1), lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.did, AuthorityKey: actor.public,
		AccountSequence: sequence, NotBefore: uint64(h.ctx.BlockTime().UnixMilli()),
		NotAfter:       uint64(h.ctx.BlockTime().UnixMilli()) + h.host.TimestampWindowMs(),
		IdempotencyKey: key, Payload: payload,
	}, authority)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	return receipt
}

func payBudgetReceipt(t *testing.T, expected, receipt lxphost.Receipt) {
	t.Helper()
	require.Equal(t, expected, receipt)
	require.Zero(t, receipt.ResultCode)
	require.NotEqual(t, receipt.PreviousStateRoot, receipt.ResultingStateRoot)
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, uint16(3), decoded.ModuleID)
}

func TestBudgetCreateAndFund(t *testing.T) {
	h := newPayHarness(t)
	a := payBudgetArgs(t, h)
	actor := h.actors[0]
	_, owner, err := lxphost.MainAccountID(actor.public)
	require.NoError(t, err)
	payload, err := types.EncodeBudgetCreate(a.BudgetID, a.BudgetAccount, a.AssetID, a.PurposeHash,
		a.PerPeriodLimit, a.CarryCap, a.Amount, a.PeriodLengthMs, a.PeriodStartMs,
		a.ExpiryMs, a.RevocationSequence, a.RolloverPolicy)
	require.NoError(t, err)
	require.Len(t, payload, 211)
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(payload[:2]))
	expected := payBudgetExpectedReceipt(t, h, 0x00030001, payload, a.BudgetID)
	created, err := h.keeper.BudgetCreate(h.ctx, actor.address, a)
	require.NoError(t, err)
	payBudgetReceipt(t, expected, created)
	record, found, err := h.keeper.GetBudget(h.ctx, a.BudgetID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, types.BudgetRecord{
		BudgetID: a.BudgetID, Owner: owner, BudgetAccount: a.BudgetAccount, AssetID: a.AssetID,
		PurposeHash: a.PurposeHash, PerPeriodLimit: a.PerPeriodLimit, ConfiguredPeriodLimit: a.PerPeriodLimit,
		CarryCap: a.CarryCap, PeriodLengthMs: a.PeriodLengthMs, PeriodStartMs: a.PeriodStartMs,
		ExpiryMs: a.ExpiryMs, RevocationSequence: a.RevocationSequence, RolloverPolicy: a.RolloverPolicy,
		Delegates: [][32]byte{},
	}, record)
	payEscrowBalance(t, h, owner, a.AssetID, "400")
	payEscrowBalance(t, h, record.BudgetAccount, record.AssetID, "600")
	remaining, err := h.keeper.BudgetRemaining(h.ctx, a.BudgetID)
	require.NoError(t, err)
	require.Equal(t, "600", remaining.String())

	amount, key := codec.U128{Lo: 150}, [32]byte{0xb3}
	payload, err = types.EncodeBudgetFund(a.BudgetID, amount)
	require.NoError(t, err)
	require.Len(t, payload, 50)
	expected = payBudgetExpectedReceipt(t, h, 0x00030002, payload, key)
	funded, err := h.keeper.BudgetFund(h.ctx, actor.address, a.BudgetID, amount, key)
	require.NoError(t, err)
	payBudgetReceipt(t, expected, funded)
	require.Equal(t, created.GlobalSequence+1, funded.GlobalSequence)
	payEscrowBalance(t, h, owner, a.AssetID, "250")
	payEscrowBalance(t, h, record.BudgetAccount, record.AssetID, "750")
	remaining, err = h.keeper.BudgetRemaining(h.ctx, a.BudgetID)
	require.NoError(t, err)
	require.Equal(t, "750", remaining.String())
	after, found, err := h.keeper.GetBudget(h.ctx, a.BudgetID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, record, after)

	before := payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.BudgetCreate(h.ctx, actor.address, a)
	require.ErrorIs(t, err, lxphost.Code(-301))
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	_, err = h.keeper.BudgetFund(h.ctx, actor.address, a.BudgetID, codec.U128{Lo: 251}, [32]byte{0xb4})
	require.Error(t, err)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	_, err = h.keeper.BudgetFund(h.ctx, common.Address{}, a.BudgetID, amount, [32]byte{0xb5})
	require.ErrorIs(t, err, types.ErrUnboundCaller)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
}

func TestBudgetNativeVariantsAndAmend(t *testing.T) {
	h := newPayHarness(t)
	a := payBudgetArgs(t, h)
	actor := h.actors[0]
	ownerAsset := a.AssetID
	salt := [32]byte{0xc0}
	a.AssetID = sha256.Sum256(append(append([]byte("LX:ASSET:v1"), actor.id[:]...), salt[:]...))
	registration := append(append([]byte{0, 1}, a.AssetID[:]...), salt[:]...)
	registration = append(registration, 1, 'N', 1, 'N', 6)
	registration = binary.BigEndian.AppendUint64(registration, 0)
	registration = binary.BigEndian.AppendUint64(registration, 1000)
	registration = append(registration, 1, 0)
	paySubmitExecute(t, h, actor, 0x00010001, registration, [32]byte{0xc4})
	paySubmitExecute(t, h, actor, 0x00010004, append([]byte{0, 1}, a.AssetID[:]...), [32]byte{0xc5})
	sourceName, source, err := lxphost.AssetAccountID(actor.public, a.AssetID)
	require.NoError(t, err)
	mint := append(append([]byte{0, 1}, a.AssetID[:]...), source[:]...)
	mint = binary.BigEndian.AppendUint64(mint, 0)
	mint = binary.BigEndian.AppendUint64(mint, 1000)
	paySubmitExecute(t, h, actor, 0x0001000a, mint, [32]byte{0xc1})
	sourceBefore, err := h.host.ReadAccount(h.ctx, sourceName, source)
	require.NoError(t, err)
	require.Equal(t, uint64(0), sourceBefore.NextSequence)
	identitySequence, err := h.host.ReadIdentitySequence(h.ctx, actor.did)
	require.NoError(t, err)
	require.NotEqual(t, identitySequence, sourceBefore.NextSequence)
	payload, err := types.EncodeBudgetCreateNative(a.BudgetID, a.BudgetAccount, a.AssetID, a.PurposeHash,
		a.PerPeriodLimit, a.CarryCap, a.Amount, a.PeriodLengthMs, a.PeriodStartMs,
		a.ExpiryMs, a.RevocationSequence, a.RolloverPolicy, source, sourceBefore.NextSequence)
	require.NoError(t, err)
	require.Len(t, payload, 251)
	require.Equal(t, uint16(2), binary.BigEndian.Uint16(payload[:2]))
	require.Equal(t, source[:], payload[211:243])
	require.Equal(t, sourceBefore.NextSequence, binary.BigEndian.Uint64(payload[243:251]))
	before := payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.BudgetCreateNative(h.ctx, actor.address, a, source, sourceBefore.NextSequence+1)
	require.ErrorIs(t, err, lxphost.Code(-213))
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	expected := payBudgetExpectedReceipt(t, h, 0x00030001, payload, a.BudgetID)
	created, err := h.keeper.BudgetCreateNative(h.ctx, actor.address, a, source, sourceBefore.NextSequence)
	require.NoError(t, err)
	payBudgetReceipt(t, expected, created)
	record, found, err := h.keeper.GetBudget(h.ctx, a.BudgetID)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, record.NativeSource)
	require.Equal(t, source, record.SourceAccount)
	require.Equal(t, a.BudgetAccount, record.BudgetAccount)
	payEscrowBalance(t, h, source, a.AssetID, "400")
	payEscrowBalance(t, h, record.Owner, ownerAsset, "1000")
	payEscrowBalance(t, h, a.BudgetAccount, a.AssetID, "600")
	sourceAfter, err := h.host.ReadAccount(h.ctx, sourceName, source)
	require.NoError(t, err)
	require.Equal(t, sourceBefore.NextSequence+1, sourceAfter.NextSequence)

	amount, key := codec.U128{Lo: 150}, [32]byte{0xc2}
	payload, err = types.EncodeBudgetFundNative(a.BudgetID, amount, sourceAfter.NextSequence)
	require.NoError(t, err)
	require.Len(t, payload, 58)
	require.Equal(t, uint16(2), binary.BigEndian.Uint16(payload[:2]))
	require.Equal(t, sourceAfter.NextSequence, binary.BigEndian.Uint64(payload[50:58]))
	before = payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.BudgetFundNative(h.ctx, actor.address, a.BudgetID, amount, sourceBefore.NextSequence, key)
	require.ErrorIs(t, err, lxphost.Code(-213))
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	expected = payBudgetExpectedReceipt(t, h, 0x00030002, payload, key)
	funded, err := h.keeper.BudgetFundNative(h.ctx, actor.address, a.BudgetID, amount, sourceAfter.NextSequence, key)
	require.NoError(t, err)
	payBudgetReceipt(t, expected, funded)
	require.Equal(t, created.GlobalSequence+1, funded.GlobalSequence)
	payEscrowBalance(t, h, source, a.AssetID, "250")
	payEscrowBalance(t, h, a.BudgetAccount, a.AssetID, "750")
	sourceFunded, err := h.host.ReadAccount(h.ctx, sourceName, source)
	require.NoError(t, err)
	require.Equal(t, sourceAfter.NextSequence+1, sourceFunded.NextSequence)

	limit, carry, expiry, policy := codec.U128{Lo: 800}, codec.U128{Lo: 80}, a.ExpiryMs+60000, uint8(2)
	amendKey := [32]byte{0xc3}
	payload, err = types.EncodeBudgetAmend(a.BudgetID, limit, carry, expiry, policy)
	require.NoError(t, err)
	require.Len(t, payload, 75)
	expected = payBudgetExpectedReceipt(t, h, 0x00030003, payload, amendKey)
	amended, err := h.keeper.BudgetAmend(h.ctx, actor.address, a.BudgetID, limit, carry, expiry, policy, amendKey)
	require.NoError(t, err)
	payBudgetReceipt(t, expected, amended)
	require.Equal(t, funded.GlobalSequence+1, amended.GlobalSequence)
	record.PerPeriodLimit, record.ConfiguredPeriodLimit = limit, limit
	record.CarryCap, record.ExpiryMs, record.RolloverPolicy = carry, expiry, policy
	after, found, err := h.keeper.GetBudget(h.ctx, a.BudgetID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, record, after)
	payEscrowBalance(t, h, a.BudgetAccount, a.AssetID, "750")
	payEscrowBalance(t, h, source, a.AssetID, "250")

	t.Run("encoder_refusals_preserve_state", func(t *testing.T) {
		before := payStoreSnapshot(h.ctx, h.hostKey)
		invalid := a
		invalid.Amount = codec.U128{}
		calls := []struct {
			name string
			call func() (lxphost.Receipt, error)
			code string
		}{
			{"create", func() (lxphost.Receipt, error) { return h.keeper.BudgetCreate(h.ctx, actor.address, invalid) }, "LXP_INVALID_AMOUNT"},
			{"create_native", func() (lxphost.Receipt, error) {
				return h.keeper.BudgetCreateNative(h.ctx, actor.address, a, source, math.MaxUint64)
			}, "LXP_NON_CANONICAL"},
			{"fund", func() (lxphost.Receipt, error) {
				return h.keeper.BudgetFund(h.ctx, actor.address, a.BudgetID, codec.U128{}, key)
			}, "LXP_INVALID_AMOUNT"},
			{"fund_native", func() (lxphost.Receipt, error) {
				return h.keeper.BudgetFundNative(h.ctx, actor.address, a.BudgetID, amount, math.MaxUint64, key)
			}, "LXP_NON_CANONICAL"},
			{"amend", func() (lxphost.Receipt, error) {
				return h.keeper.BudgetAmend(h.ctx, actor.address, a.BudgetID, codec.U128{}, carry, expiry, policy, amendKey)
			}, "LXP_INVALID_AMOUNT"},
		}
		for _, test := range calls {
			t.Run(test.name, func(t *testing.T) {
				receipt, err := test.call()
				require.ErrorContains(t, err, test.code)
				require.Equal(t, lxphost.Receipt{}, receipt)
				require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
			})
		}
	})
}
