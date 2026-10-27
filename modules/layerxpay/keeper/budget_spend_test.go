package keeper

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func paySpendBudget(t *testing.T, h *payHarness, id [32]byte) (asset, account, recipient [32]byte) {
	t.Helper()
	paySubmitFundedEscrow(t, h, id)
	actor := h.actors[0]
	salt := [32]byte{0x73}
	asset = sha256.Sum256(append(append([]byte("LX:ASSET:v1"), actor.id[:]...), salt[:]...))
	_, recipient, err := lxphost.MainAccountID(h.actors[1].public)
	require.NoError(t, err)
	account, err = codec.DeriveAccountID([]byte("agent:" + string(actor.did) + ":budget:" + hex.EncodeToString(id[:])))
	require.NoError(t, err)
	h.keeper.SetParams(h.ctx, types.Params{BudgetEnabled: true})
	gate, err := types.KernelModuleEnableKey(types.BudgetModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(gate, payGateEnabledRecord())
	payload, err := types.EncodeBudgetCreate(id, account, asset, [32]byte{0x38}, codec.U128{Lo: 100}, codec.U128{}, codec.U128{Lo: 150}, 10000, uint64(payHarnessTimeMs), uint64(payHarnessTimeMs)+100000, 0, 1)
	require.NoError(t, err)
	receipt, err := h.keeper.Submit(h.ctx, actor.address, "budgetCreate", payload, [32]byte{0x39})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	return asset, account, recipient
}

func payBudgetRecord(t *testing.T, h *payHarness, id [32]byte) types.BudgetRecord {
	t.Helper()
	key := types.KernelKVKey(types.BudgetModuleID, append([]byte("budget:"), id[:]...))
	record, err := types.DecodeBudgetRecord(h.ctx.KVStore(h.hostKey).Get(key))
	require.NoError(t, err)
	return record
}

func payBudgetExpected(t *testing.T, h *payHarness, ctx sdk.Context, actor payActor, activityType uint32, payload []byte, key [32]byte) lxphost.Receipt {
	t.Helper()
	last, err := h.host.ReadLastSequence(ctx, actor.did)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(ctx, actor.did)
	require.NoError(t, err)
	_, principal, err := lxphost.MainAccountID(actor.public)
	require.NoError(t, err)
	authority := actor.authority()
	authority.Bound, authority.Principal = true, principal
	branch, _ := ctx.CacheContext()
	receipt, err := h.host.HostExecute(lxphost.WithSequence(branch, last+1), lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.did, AuthorityKey: actor.public,
		AccountSequence: sequence, NotBefore: uint64(ctx.BlockTime().UnixMilli()),
		NotAfter:       uint64(ctx.BlockTime().UnixMilli()) + h.host.TimestampWindowMs(),
		IdempotencyKey: key, Payload: payload,
	}, authority)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	return receipt
}

func TestBudgetSpendWithinPeriodLimit(t *testing.T) {
	h := newPayHarness(t)
	id := [32]byte{0x41}
	asset, account, recipient := paySpendBudget(t, h, id)
	actor := h.actors[0]
	payload, err := types.EncodeBudgetSpend(id, recipient, codec.U128{Lo: 60})
	require.NoError(t, err)
	require.Len(t, payload, 82)
	expected := payBudgetExpected(t, h, h.ctx, actor, 0x00030006, payload, [32]byte{})
	receipt, err := h.keeper.BudgetSpend(h.ctx, actor.address, id, recipient, codec.U128{Lo: 60})
	require.NoError(t, err)
	require.Equal(t, expected, receipt)
	require.Equal(t, codec.U128{Lo: 60}, payBudgetRecord(t, h, id).SpentThisPeriod)
	balance, err := h.host.ReadBalance(h.ctx, account, asset)
	require.NoError(t, err)
	require.Equal(t, "90", balance.String())
	balance, err = h.host.ReadBalance(h.ctx, recipient, asset)
	require.NoError(t, err)
	require.Equal(t, "60", balance.String())

	before := payStoreSnapshot(h.ctx, h.hostKey)
	moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
	refused, err := h.keeper.BudgetSpend(h.ctx, actor.address, id, recipient, codec.U128{Lo: 41})
	require.Error(t, err)
	require.ErrorContains(t, err, "LXP_BUDGET_ALLOWANCE_EXCEEDED")
	require.NotZero(t, refused.ResultCode)
	require.ErrorIs(t, err, refused.ResultCode)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	_, err = h.keeper.BudgetSpend(h.ctx, actor.address, id, recipient, codec.U128{Lo: 40})
	require.NoError(t, err)
	require.Equal(t, codec.U128{Lo: 100}, payBudgetRecord(t, h, id).SpentThisPeriod)

	t.Run("canonical_payload_and_write_checks", func(t *testing.T) {
		before := payStoreSnapshot(h.ctx, h.hostKey)
		_, err := h.keeper.BudgetSpend(h.ctx, actor.address, id, recipient, codec.U128{})
		require.ErrorContains(t, err, "LXP_INVALID_AMOUNT")
		_, err = h.keeper.BudgetSpend(h.ctx, actor.address, [32]byte{}, recipient, codec.U128{Lo: 1})
		require.ErrorContains(t, err, "LXP_NON_CANONICAL")
		_, err = h.keeper.BudgetDelegateAdd(h.ctx, actor.address, id, [32]byte{})
		require.ErrorContains(t, err, "LXP_NON_CANONICAL")
		_, err = h.keeper.BudgetDelegateRemove(h.ctx, actor.address, [32]byte{}, actor.id)
		require.ErrorContains(t, err, "LXP_NON_CANONICAL")
		_, err = h.keeper.BudgetDelegateAdd(h.ctx, common.Address{}, id, actor.id)
		require.ErrorIs(t, err, types.ErrUnboundCaller)
		h.keeper.SetParams(h.ctx, types.Params{})
		_, err = h.keeper.BudgetSpend(h.ctx, actor.address, id, recipient, codec.U128{Lo: 1})
		require.ErrorIs(t, err, types.ErrFamilyDisabled)
		require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	})
}

func TestBudgetDelegateSpendDerivedKey(t *testing.T) {
	h := newPayHarness(t)
	h.cms.Commit(true)
	transient := sdk.NewTransientStoreKey(evmtypes.TransientStoreKey)
	h.cms.MountStoreWithDB(transient, sdk.StoreTypeTransient, nil)
	require.NoError(t, h.cms.LoadLatestVersion())
	id := [32]byte{0x51}
	asset, account, recipient := paySpendBudget(t, h, id)
	owner, delegate := h.actors[0], h.actors[1]
	payload, err := types.EncodeBudgetDelegate(id, delegate.id)
	require.NoError(t, err)
	require.Len(t, payload, 66)
	expected := payBudgetExpected(t, h, h.ctx, owner, 0x00030004, payload, [32]byte{})
	added, err := h.keeper.BudgetDelegateAdd(h.ctx, owner.address, id, delegate.id)
	require.NoError(t, err)
	require.Equal(t, expected, added)
	require.Equal(t, [][32]byte{delegate.id}, payBudgetRecord(t, h, id).Delegates)

	capability := make([]byte, 195)
	capability[1] = 1
	copy(capability[2:34], delegate.id[:])
	copy(capability[34:66], asset[:])
	copy(capability[66:98], recipient[:])
	purpose := payBudgetRecord(t, h, id).PurposeHash
	copy(capability[98:130], purpose[:])
	maximumSpend, maximumTotal := (codec.U128{Lo: 25}).Bytes(), (codec.U128{Lo: 100}).Bytes()
	copy(capability[130:146], maximumSpend[:])
	copy(capability[146:162], maximumTotal[:])
	binary.BigEndian.PutUint64(capability[178:186], uint64(payHarnessTimeMs)+100000)
	capabilityKey := types.KernelKVKey(types.BudgetModuleID, append(append([]byte("budget-delegate:"), id[:]...), delegate.id[:]...))
	h.ctx.KVStore(h.hostKey).Set(capabilityKey, capability)

	bytes32, err := abi.NewType("bytes32", "", nil)
	require.NoError(t, err)
	uint128, err := abi.NewType("uint128", "", nil)
	require.NoError(t, err)
	method := abi.NewMethod("budgetDelegateSpend", "budgetDelegateSpend", abi.Function, "nonpayable", false, false,
		abi.Arguments{{Type: bytes32}, {Type: bytes32}, {Type: uint128}}, nil)
	selector := [4]byte(method.ID)
	txHash := common.HexToHash("0x1234")
	h.ctx = h.ctx.WithEVMTxHash(txHash)
	amount := codec.U128{Lo: 10}
	keys := make(map[[32]byte]bool)
	for ordinal := uint32(0); ordinal < 2; ordinal++ {
		key := types.DeriveIdempotencyKey(txHash, ordinal, selector)
		require.NotEqual(t, [32]byte{}, key)
		require.False(t, keys[key])
		keys[key] = true
		payload, err := types.EncodeBudgetDelegateSpend(id, recipient, amount, key)
		require.NoError(t, err)
		require.Len(t, payload, 114)
		require.Equal(t, key[:], payload[82:114])
		expected := payBudgetExpected(t, h, h.ctx, delegate, 0x0003000a, payload, key)
		branch, commit := h.ctx.CacheContext()
		receipt, err := h.keeper.BudgetDelegateSpend(branch, delegate.address, id, recipient, amount)
		require.NoError(t, err)
		require.Equal(t, expected, receipt)
		commit()
	}
	require.Equal(t, codec.U128{Lo: 20}, payBudgetRecord(t, h, id).SpentThisPeriod)
	balance, err := h.host.ReadBalance(h.ctx, account, asset)
	require.NoError(t, err)
	require.Equal(t, "130", balance.String())
	require.Equal(t, (codec.U128{Lo: 20}).Bytes(), [16]byte(h.ctx.KVStore(h.hostKey).Get(capabilityKey)[162:178]))

	otherHash := common.HexToHash("0x5678")
	other := h.ctx.WithEVMTxHash(otherHash)
	key := types.DeriveIdempotencyKey(otherHash, 0, selector)
	payload, err = types.EncodeBudgetDelegateSpend(id, recipient, amount, key)
	require.NoError(t, err)
	expected = payBudgetExpected(t, h, other, delegate, 0x0003000a, payload, key)
	spent, err := h.keeper.BudgetDelegateSpend(other, delegate.address, id, recipient, amount)
	require.NoError(t, err)
	require.Equal(t, expected, spent)

	payload, err = types.EncodeBudgetDelegate(id, delegate.id)
	require.NoError(t, err)
	require.Len(t, payload, 66)
	expected = payBudgetExpected(t, h, h.ctx, owner, 0x00030005, payload, [32]byte{})
	removed, err := h.keeper.BudgetDelegateRemove(h.ctx, owner.address, id, delegate.id)
	require.NoError(t, err)
	require.Equal(t, expected, removed)
	require.Empty(t, payBudgetRecord(t, h, id).Delegates)
	before := payStoreSnapshot(h.ctx, h.hostKey)
	refused, err := h.keeper.BudgetDelegateSpend(h.ctx, delegate.address, id, recipient, amount)
	require.ErrorContains(t, err, "LXP_UNAUTHORIZED_DELEGATE")
	require.ErrorIs(t, err, refused.ResultCode)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))

	t.Run("transaction_context_and_counter_bounds", func(t *testing.T) {
		_, err := h.keeper.BudgetDelegateSpend(h.ctx.WithEVMTxHash(common.Hash{}), delegate.address, id, recipient, amount)
		require.ErrorContains(t, err, "missing transaction hash")
		counterKey := append([]byte(payCallOrdinalPrefix), txHash[:]...)
		h.ctx.TransientStore(transient).Set(counterKey, []byte{1})
		_, err = nextPayIdempotencyKey(h.ctx, selector)
		require.ErrorContains(t, err, "invalid call ordinal")
		var maximum [4]byte
		binary.BigEndian.PutUint32(maximum[:], math.MaxUint32-1)
		h.ctx.TransientStore(transient).Set(counterKey, maximum[:])
		key, err := nextPayIdempotencyKey(h.ctx, selector)
		require.NoError(t, err)
		require.Equal(t, types.DeriveIdempotencyKey(txHash, math.MaxUint32, selector), key)
		_, err = nextPayIdempotencyKey(h.ctx, selector)
		require.ErrorContains(t, err, "call ordinal exhausted")
		h.cms.Commit(true)
		key, err = nextPayIdempotencyKey(h.ctx, selector)
		require.NoError(t, err)
		require.Equal(t, types.DeriveIdempotencyKey(txHash, 0, selector), key)
		require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	})
}
