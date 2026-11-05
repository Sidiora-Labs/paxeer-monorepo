package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	layerxante "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/ante"
	keeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1421_CapabilityEndpoint(t *testing.T) {
	t.Run("per_activity_limit", func(t *testing.T) { repairM1421Endpoint(t, false, 11) })
	repairM1421Endpoint(t, false, 5)
}

func TestRepairM_1421_AuthProofEndpoint(t *testing.T) {
	t.Run("per_activity_limit", func(t *testing.T) { repairM1421Endpoint(t, true, 11) })
	repairM1421Endpoint(t, true, 5)
}

func repairM1421Endpoint(t *testing.T, proveReceipt bool, paymentAmount int64) {
	t.Helper()
	f := newAuthViewFixture(t)
	actor := authExecuteActor(t, f)
	az, azKey := capabilityAuthz(t, f)
	granter := sdk.AccAddress(common.Address{0x26, 0x64, 1}.Bytes())
	recipient := sdk.AccAddress(bytes.Repeat([]byte{0x42}, 20))
	denom := ledgertypes.MirrorDenom(*f.Asset())
	purpose, err := types.BankSendPurpose(banktypes.NewMsgSend(granter, recipient,
		sdk.NewCoins(sdk.NewInt64Coin(denom, paymentAmount))))
	require.NoError(t, err)
	*f.Purpose() = purpose
	payload, _ := executableCapabilityPayload(t, f)
	body := bytes.Clone(payload[8:])
	// Leave time to exercise two complete budget periods before expiration.
	binary.BigEndian.PutUint64(body[302:310], f.Timestamp()+3000)
	grantID, err := codec.DomainHash(codec.DomainAuthorityHash, body)
	require.NoError(t, err)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{74}, ed25519.SeedSize))
	grantee := sdk.AccAddress(bindCapabilityKey(t, f, private).Bytes())
	require.NotEqual(t, granter, grantee)
	app := testkeeper.EVMTestApp
	ctx := *f.Context()
	app.EvmKeeper.SetAddressMapping(ctx, granter, common.BytesToAddress(granter))
	app.EvmKeeper.SetAddressMapping(ctx, grantee, common.BytesToAddress(grantee))
	require.NoError(t, apptesting.FundAccount(app.BankKeeper, ctx, granter,
		sdk.NewCoins(sdk.NewInt64Coin(denom, 100))))
	granterBefore := app.BankKeeper.GetBalance(ctx, granter, denom).Amount
	recipientBefore := app.BankKeeper.GetBalance(ctx, recipient, denom).Amount
	result, err := f.Keeper().GrantCapability(ctx, actor, body, [32]byte{0x14, 0x21})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), result.ResultCode)
	if proveReceipt {
		proofLayerxauthReceipt(t, result, grantID, body, "CapabilityGranted")
	} else {
		events, err := keeper.AuthEvents(result.Receipt)
		require.NoError(t, err)
		require.NotEmpty(t, events)
		require.Equal(t, "CapabilityGranted", events[0].Name)
		require.Equal(t, grantID, events[0].GrantID)
	}
	issued, err := f.Host().ReadGrant(ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, grantID, issued.GrantID)
	require.False(t, issued.Revoked)
	messageURL := sdk.MsgTypeURL(&banktypes.MsgSend{})
	expiration := time.UnixMilli(int64(f.Timestamp() + 3000)).UTC()
	assertSpending := func(t *testing.T, ctx sdk.Context, total, period string) {
		t.Helper()
		stored, expiry := az.GetCleanAuthorization(ctx, grantee, granter, messageURL)
		require.IsType(t, &types.LayerXScopeAuthorization{}, stored)
		scope := stored.(*types.LayerXScopeAuthorization)
		require.NoError(t, scope.ValidateBasic())
		require.Equal(t, messageURL, scope.MsgTypeUrl)
		require.Equal(t, grantID[:], scope.GrantId)
		require.Equal(t, purpose[:], scope.PurposeHash)
		require.Equal(t, issued.GrantorRevocationSequence, scope.RevocationEpoch)
		require.Equal(t, "10", scope.MaxPerActivity)
		require.Equal(t, "20", scope.MaxTotal)
		require.Equal(t, "15", scope.MaxPerPeriod)
		require.Equal(t, total, scope.SpentTotal)
		require.Equal(t, period, scope.SpentThisPeriod)
		require.Equal(t, expiration, expiry)
	}
	assertBalances := func(t *testing.T, ctx sdk.Context, spent int64) {
		t.Helper()
		require.Equal(t, granterBefore.SubRaw(spent), app.BankKeeper.GetBalance(ctx, granter, denom).Amount)
		require.Equal(t, recipientBefore.AddRaw(spent), app.BankKeeper.GetBalance(ctx, recipient, denom).Amount)
	}
	dispatch := func(t *testing.T, ctx sdk.Context, amount int64) (*authz.MsgExecResponse, error) {
		t.Helper()
		message := banktypes.NewMsgSend(granter, recipient, sdk.NewCoins(sdk.NewInt64Coin(denom, amount)))
		exec := authz.NewMsgExec(grantee, []sdk.Msg{message})
		txConfig := app.GetTxConfig()
		builder := txConfig.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(&exec))
		wire, err := txConfig.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		transported, err := txConfig.TxDecoder()(wire)
		require.NoError(t, err)
		require.Len(t, transported.GetMsgs(), 1)
		decoded := transported.GetMsgs()[0].(*authz.MsgExec)
		messages, err := decoded.GetMessages()
		require.NoError(t, err)
		require.Len(t, messages, 1)
		require.Equal(t, message, messages[0])
		branch, write := ctx.WithEventManager(sdk.NewEventManager()).CacheContext()
		decorator := layerxante.NewRevocationDecorator(f.Keeper(), &app.EvmKeeper)
		anteCtx, err := sdk.ChainAnteDecorators(decorator)(branch, transported, false)
		if err != nil {
			return nil, err
		}
		response, err := az.Exec(sdk.WrapSDKContext(anteCtx), decoded)
		if err == nil {
			require.NotNil(t, response)
			require.Len(t, response.Results, 1)
			write()
		}
		return response, err
	}
	assertRefusal := func(t *testing.T, ctx sdk.Context, amount int64, expected error, reason string) {
		t.Helper()
		keys := []sdk.StoreKey{f.HostKey(), f.StoreKey(), azKey, app.GetKey(banktypes.StoreKey)}
		before := authViewStores(ctx, keys...)
		response, err := dispatch(t, ctx, amount)
		require.ErrorIs(t, err, expected)
		require.ErrorContains(t, err, reason)
		require.Nil(t, response)
		require.Equal(t, before, authViewStores(ctx, keys...))
	}

	if paymentAmount == 11 {
		assertRefusal(t, ctx, 11, types.ErrAuthScope, "above max per activity 10")
		assertBalances(t, ctx, 0)
		assertSpending(t, ctx, "0", "")
		return
	}

	t.Run("spending_limits", func(t *testing.T) {
		branch, _ := ctx.CacheContext()
		assertSpending(t, branch, "0", "")
		for _, amount := range []int64{5, 5, 5} {
			_, err := dispatch(t, branch, amount)
			require.NoError(t, err)
		}
		assertBalances(t, branch, 15)
		assertSpending(t, branch, "15", "15")
		assertRefusal(t, branch, 5, types.ErrAuthScope, "period budget exceeded")
		assertBalances(t, branch, 15)
		assertSpending(t, branch, "15", "15")
		nextPeriod := branch.WithBlockTime(branch.BlockTime().Add(time.Second))
		_, err := dispatch(t, nextPeriod, 5)
		require.NoError(t, err)
		assertBalances(t, nextPeriod, 20)
		assertSpending(t, nextPeriod, "20", "5")
		assertRefusal(t, nextPeriod, 5, types.ErrAuthScope, "spent total 25 above max total 20")
		assertBalances(t, nextPeriod, 20)
		assertSpending(t, nextPeriod, "20", "5")
	})

	t.Run("kernel_revocation", func(t *testing.T) {
		branch, _ := ctx.CacheContext()
		_, err := dispatch(t, branch, 5)
		require.NoError(t, err)
		assertBalances(t, branch, 5)
		assertSpending(t, branch, "5", "5")
		sequence, err := lxphost.NextSequence(branch, f.HostKey())
		require.NoError(t, err)
		revoked, err := f.Keeper().RevokeGrant(branch, actor, grantID, 1, sequence, [32]byte{0x14, 0x22})
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), revoked.ResultCode)
		grant, err := f.Host().ReadGrant(branch, grantID)
		require.NoError(t, err)
		require.True(t, grant.Revoked)
		epoch, err := f.Keeper().RevocationSequence(branch, *f.Public())
		require.NoError(t, err)
		require.Equal(t, sequence, epoch)
		require.NotEqual(t, issued.GrantorRevocationSequence, epoch)
		assertRefusal(t, branch, 5, types.ErrAuthRevoked, "revocation epoch 41, grantor revocation")
		assertBalances(t, branch, 5)
		assertSpending(t, branch, "5", "5")
	})
}
