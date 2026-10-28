package keeper

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
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

func newBudgetPullHarness(t *testing.T) (*payHarness, *sdk.TransientStoreKey) {
	t.Helper()
	h := newPayHarness(t)
	h.cms.Commit(true)
	key := sdk.NewTransientStoreKey(evmtypes.TransientStoreKey)
	h.cms.MountStoreWithDB(key, sdk.StoreTypeTransient, nil)
	require.NoError(t, h.cms.LoadLatestVersion())
	h.ctx = sdk.NewContext(h.cms, h.ctx.BlockHeader(), false)
	return h, key
}

func budgetPullGrant(t *testing.T, payer payActor, from, recipient, asset, purpose [32]byte) ([]byte, [32]byte) {
	t.Helper()
	body := append([]byte(nil), from[:]...)
	body = append(body, recipient[:]...)
	body = append(body, asset[:]...)
	perDraw, allowance := (codec.U128{Lo: 40}).Bytes(), (codec.U128{Lo: 100}).Bytes()
	body = append(body, perDraw[:]...)
	body = append(body, allowance[:]...)
	body = append(body, 1)
	body = binary.BigEndian.AppendUint64(body, 60000)
	body = binary.BigEndian.AppendUint64(body, uint64(payHarnessTimeMs)+1000000)
	body = append(body, purpose[:]...)
	body = append(body, 0)
	body = append(body, make([]byte, 32)...)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = append(body, payer.public[:]...)
	id, err := codec.DomainHash(codec.DomainAuthorityHash, append([]byte("LXP:GRANT:v1"), body...))
	require.NoError(t, err)
	grant := append(append([]byte(nil), id[:]...), body...)
	grant = append(grant, ed25519.Sign(payer.private, id[:])...)
	require.Len(t, grant, types.PayerGrantBytes)
	return grant, id
}

func TestBudgetPullWithPayerGrant(t *testing.T) {
	h, transientKey := newBudgetPullHarness(t)
	payer, puller := h.actors[0], h.actors[1]
	budgetID, purpose := [32]byte{0x31}, [32]byte{0x55}
	paySubmitFundedEscrow(t, h, [32]byte{0x32})
	h.keeper.SetParams(h.ctx, types.Params{BudgetEnabled: true})
	gate, err := types.KernelModuleEnableKey(types.BudgetModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(gate, payGateEnabledRecord())
	salt := [32]byte{0x73}
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), payer.id[:]...), salt[:]...))
	_, payerMain, err := lxphost.MainAccountID(payer.public)
	require.NoError(t, err)
	_, recipient, err := lxphost.MainAccountID(puller.public)
	require.NoError(t, err)
	budgetAccount, err := codec.DeriveAccountID([]byte("agent:" + string(payer.did) + ":budget:" + hex.EncodeToString(budgetID[:])))
	require.NoError(t, err)
	create, err := types.EncodeBudgetCreate(budgetID, budgetAccount, asset, purpose,
		codec.U128{Lo: 150}, codec.U128{}, codec.U128{Lo: 150}, 60000,
		uint64(payHarnessTimeMs), uint64(payHarnessTimeMs)+1000000, 0, 1)
	require.NoError(t, err)
	created, err := h.keeper.Submit(h.ctx, payer.address, "budgetCreate", create, [32]byte{0x33})
	require.NoError(t, err)
	require.Zero(t, created.ResultCode)
	grant, grantID := budgetPullGrant(t, payer, payerMain, recipient, asset, purpose)
	paySubmitExecute(t, h, payer, 0x00010007, grant, [32]byte{0x34})
	before, err := h.host.ReadPayerGrant(h.ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, grant, before.Grant)
	require.Equal(t, sdk.ZeroInt(), before.DrawnTotal)
	require.Equal(t, sdk.ZeroInt(), before.DrawnThisPeriod)

	wire, err := os.ReadFile("../../../precompiles/layerxpay/abi.json")
	require.NoError(t, err)
	contract, err := abi.JSON(strings.NewReader(string(wire)))
	require.NoError(t, err)
	var selector [4]byte
	copy(selector[:], contract.Methods["budgetPull"].ID)
	txHash := [32]byte{0xa1}
	h.ctx = h.ctx.WithEVMTxHash(common.Hash(txHash))
	key := types.DeriveIdempotencyKey(txHash, 0, selector)
	require.NotEqual(t, [32]byte{}, key)
	amount := codec.U128{Lo: 40}
	payload, err := types.EncodeBudgetPull(budgetID, recipient, amount, key, grant)
	require.NoError(t, err)
	require.Len(t, payload, types.BudgetPullPayloadBytes)
	require.Equal(t, key[:], payload[82:114])
	require.Equal(t, grant, payload[114:])
	require.Equal(t, uint32(0x0003000b), types.MethodActivity["budgetPull"].Type)
	expectedBranch, _ := h.ctx.CacheContext()
	expected, err := h.keeper.Submit(expectedBranch, puller.address, "budgetPull", payload, key)
	require.NoError(t, err)
	hostBefore, moduleBefore := payStoreSnapshot(h.ctx, h.hostKey), payStoreSnapshot(h.ctx, h.storeKey)
	last, err := h.host.ReadLastSequence(h.ctx, puller.did)
	require.NoError(t, err)
	identitySequence, err := h.host.ReadIdentitySequence(h.ctx, puller.did)
	require.NoError(t, err)
	grantBefore := bytes.Clone(grant)
	receipt, err := h.keeper.BudgetPull(h.ctx, puller.address, budgetID, recipient, amount, grant)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	require.Equal(t, expected, receipt)
	require.Equal(t, last+1, receipt.GlobalSequence)
	require.NotEqual(t, receipt.PreviousStateRoot, receipt.ResultingStateRoot)
	require.Equal(t, grantBefore, grant)
	require.NotEqual(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	require.Equal(t, payStoreSnapshot(expectedBranch, h.hostKey), payStoreSnapshot(h.ctx, h.hostKey))
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, types.BudgetModuleID, decoded.ModuleID)
	require.Equal(t, receipt.GlobalSequence, decoded.GlobalSequence)
	require.Equal(t, receipt.ResultingStateRoot, decoded.ResultingStateRoot)
	afterSequence, err := h.host.ReadIdentitySequence(h.ctx, puller.did)
	require.NoError(t, err)
	require.Equal(t, identitySequence+1, afterSequence)
	balance, err := h.host.ReadBalance(h.ctx, recipient, asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(40), balance)
	balance, err = h.host.ReadBalance(h.ctx, budgetAccount, asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(110), balance)

	t.Run("call_ordinals", func(t *testing.T) {
		branch, _ := h.ctx.CacheContext()
		ordinal, err := nextPayCallOrdinal(branch, txHash)
		require.NoError(t, err)
		require.Equal(t, uint32(1), ordinal)
		require.NotEqual(t, key, types.DeriveIdempotencyKey(txHash, ordinal, selector))
		ordinal, err = nextPayCallOrdinal(branch, [32]byte{0xa2})
		require.NoError(t, err)
		require.Zero(t, ordinal)
		nextKey := types.DeriveIdempotencyKey(txHash, 1, selector)
		nextPayload, err := types.EncodeBudgetPull(budgetID, recipient, amount, nextKey, grant)
		require.NoError(t, err)
		expectedBranch, _ := h.ctx.CacheContext()
		want, err := h.keeper.Submit(expectedBranch, puller.address, "budgetPull", nextPayload, nextKey)
		require.NoError(t, err)
		secondBranch, _ := h.ctx.CacheContext()
		got, err := h.keeper.BudgetPull(secondBranch, puller.address, budgetID, recipient, amount, grant)
		require.NoError(t, err)
		require.Equal(t, want, got)
		require.Equal(t, payStoreSnapshot(expectedBranch, h.hostKey), payStoreSnapshot(secondBranch, h.hostKey))
		refusalBranch, _ := h.ctx.CacheContext()
		transientBefore := payStoreSnapshot(refusalBranch, transientKey)
		hostBefore := payStoreSnapshot(refusalBranch, h.hostKey)
		_, err = h.keeper.BudgetPull(refusalBranch, common.Address{}, budgetID, recipient, amount, grant)
		require.ErrorIs(t, err, types.ErrUnboundCaller)
		require.Equal(t, transientBefore, payStoreSnapshot(refusalBranch, transientKey))
		require.Equal(t, hostBefore, payStoreSnapshot(refusalBranch, h.hostKey))
	})
	t.Run("missing_transaction_hash", func(t *testing.T) {
		branch := h.ctx.WithEVMTxHash(common.Hash{})
		hostBefore := payStoreSnapshot(branch, h.hostKey)
		transientBefore := payStoreSnapshot(branch, transientKey)
		receipt, err := h.keeper.BudgetPull(branch, puller.address, budgetID, recipient, amount, grant)
		require.ErrorContains(t, err, "LXP_NON_CANONICAL: layerxpay missing transaction hash")
		require.Equal(t, lxphost.Receipt{}, receipt)
		require.Equal(t, hostBefore, payStoreSnapshot(branch, h.hostKey))
		require.Equal(t, transientBefore, payStoreSnapshot(branch, transientKey))
	})

	after, err := h.host.ReadPayerGrant(h.ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, grant, after.Grant)
	require.Equal(t, sdk.NewInt(40), after.DrawnTotal)
	require.Equal(t, sdk.NewInt(40), after.DrawnThisPeriod)
}

func TestBudgetPullRefusesMalformedGrant(t *testing.T) {
	h, transientKey := newBudgetPullHarness(t)
	actor := h.actors[0]
	budgetID, recipient, amount := [32]byte{1}, [32]byte{2}, codec.U128{Lo: 40}
	cases := []struct {
		name  string
		grant []byte
	}{}
	for _, length := range []int{0, 1, types.PayerGrantBytes - 1, types.PayerGrantBytes + 1} {
		cases = append(cases, struct {
			name  string
			grant []byte
		}{fmt.Sprintf("length_%d", length), make([]byte, length)})
	}
	for _, offset := range []int{160, 209} {
		grant := make([]byte, types.PayerGrantBytes)
		grant[offset] = 2
		cases = append(cases, struct {
			name  string
			grant []byte
		}{fmt.Sprintf("invalid_flag_%d", offset), grant})
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			grant := test.grant
			_, encoderErr := types.EncodeBudgetPull(budgetID, recipient, amount, [32]byte{1}, grant)
			require.EqualError(t, encoderErr, "LXP_NON_CANONICAL")
			hostBefore, moduleBefore := payStoreSnapshot(h.ctx, h.hostKey), payStoreSnapshot(h.ctx, h.storeKey)
			transientBefore := payStoreSnapshot(h.ctx, transientKey)
			last, err := h.host.ReadLastSequence(h.ctx, actor.did)
			require.NoError(t, err)
			receipt, err := h.keeper.BudgetPull(h.ctx, actor.address, budgetID, recipient, amount, grant)
			require.ErrorIs(t, err, encoderErr)
			require.Equal(t, lxphost.Receipt{}, receipt)
			after, err := h.host.ReadLastSequence(h.ctx, actor.did)
			require.NoError(t, err)
			require.Equal(t, last, after)
			require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
			require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
			require.Equal(t, transientBefore, payStoreSnapshot(h.ctx, transientKey))
			_, err = (Keeper{}).BudgetPull(sdk.Context{}, actor.address, budgetID, recipient, amount, grant)
			require.ErrorIs(t, err, encoderErr)
		})
	}
}
