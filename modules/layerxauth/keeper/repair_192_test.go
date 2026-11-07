package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func repair192Addresses(t *testing.T, f *authViewFixture) (sdk.AccAddress, sdk.AccAddress) {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{74}, ed25519.SeedSize))
	return sdk.AccAddress(common.Address{0x26, 0x64, 1}.Bytes()), sdk.AccAddress(bindCapabilityKey(t, f, private).Bytes())
}

func TestRepair_192_CapabilityMessageURLMsgExec(t *testing.T) {
	f := newAuthViewFixture(t)
	actor := authExecuteActor(t, f)
	az, _ := capabilityAuthz(t, f)
	payload, grantID := executableCapabilityPayload(t, f)
	granter, grantee := repair192Addresses(t, f)
	ctx := *f.Context()
	bank := testkeeper.EVMTestApp.BankKeeper
	recipient := sdk.AccAddress(bytes.Repeat([]byte{0x92}, 20))
	const denom = "upax"
	require.NoError(t, apptesting.FundAccount(bank, ctx, granter, sdk.NewCoins(sdk.NewInt64Coin(denom, 20))))
	before := bank.GetBalance(ctx, granter, denom).Amount
	recipientBefore := bank.GetBalance(ctx, recipient, denom).Amount
	result, err := f.Keeper().GrantCapability(ctx, actor, payload[8:], [32]byte{0x19, 0x20})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), result.ResultCode)
	issued, err := f.Host().ReadGrant(ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, uint64(1<<1), issued.Scope.ModuleMask)
	require.Equal(t, grantID, issued.GrantID)

	// Supply the production revocation view read from the same kernel that issued
	// the capability, then pass the actual MsgExec protobuf across the codec.
	sequence, err := f.Keeper().RevocationSequence(ctx, *f.Public())
	require.NoError(t, err)
	ctx = types.WithGrantorRevocation(ctx, granter, sequence)
	coins := sdk.NewCoins(sdk.NewInt64Coin(denom, 3))
	messages := []sdk.Msg{
		banktypes.NewMsgSend(granter, recipient, coins),
		banktypes.NewMsgMultiSend([]banktypes.Input{{Address: granter.String(), Coins: coins}},
			[]banktypes.Output{{Address: recipient.String(), Coins: coins}}),
	}
	require.Len(t, az.GetAuthorizations(ctx, grantee, granter), len(messages))
	for _, message := range messages {
		messageURL := sdk.MsgTypeURL(message)
		authorization, expiry := az.GetCleanAuthorization(ctx, grantee, granter, messageURL)
		require.IsType(t, &types.LayerXScopeAuthorization{}, authorization)
		scope := authorization.(*types.LayerXScopeAuthorization)
		require.Equal(t, grantID[:], scope.GrantId)
		require.Equal(t, messageURL, scope.MsgTypeUrl)
		require.Equal(t, issued.GrantorRevocationSequence, scope.RevocationEpoch)
		require.Equal(t, int64(issued.NotAfter), expiry.UnixMilli())
		exec := authz.NewMsgExec(grantee, []sdk.Msg{message})
		wire, err := testkeeper.EVMTestApp.AppCodec().Marshal(&exec)
		require.NoError(t, err)
		var transported authz.MsgExec
		require.NoError(t, testkeeper.EVMTestApp.AppCodec().Unmarshal(wire, &transported))
		decoded, err := transported.GetMessages()
		require.NoError(t, err)
		require.Len(t, decoded, 1)
		require.Equal(t, messageURL, sdk.MsgTypeURL(decoded[0]))
		response, err := az.Exec(sdk.WrapSDKContext(ctx), &transported)
		require.NoError(t, err)
		require.Len(t, response.Results, 1)
		remaining, remainingExpiry := az.GetCleanAuthorization(ctx, grantee, granter, messageURL)
		require.Equal(t, grantID[:], remaining.(*types.LayerXScopeAuthorization).GrantId)
		require.Equal(t, expiry, remainingExpiry)
	}
	require.Equal(t, before.SubRaw(6), bank.GetBalance(ctx, granter, denom).Amount)
	require.Equal(t, recipientBefore.AddRaw(6), bank.GetBalance(ctx, recipient, denom).Amount)
	absent, _ := az.GetCleanAuthorization(ctx, grantee, granter, "/paxprotocol.paxchain.layerxauth.LayerXScopeAuthorization")
	require.Nil(t, absent)
}

func TestRepair_192_OverlappingCapabilitiesAreAtomic(t *testing.T) {
	f := newAuthViewFixture(t)
	actor := authExecuteActor(t, f)
	az, azKey := capabilityAuthz(t, f)
	payload, firstID := executableCapabilityPayload(t, f)
	granter, grantee := repair192Addresses(t, f)
	ctx := *f.Context()
	_, err := f.Keeper().GrantCapability(ctx, actor, payload[8:], [32]byte{0x19, 0x21})
	require.NoError(t, err)
	first, err := f.Host().ReadGrant(ctx, firstID)
	require.NoError(t, err)
	before := authViewStores(ctx, f.HostKey(), f.StoreKey(), azKey)
	// Re-publishing the same kernel grant must not reset any existing mirror.
	require.ErrorIs(t, f.Keeper().SaveGrant(ctx, actor, first), types.ErrAuthScope)
	require.Equal(t, before, authViewStores(ctx, f.HostKey(), f.StoreKey(), azKey))

	// Occupy only the second message key, so a conflict is discovered after the
	// first URL has been staged. Neither that partial mirror nor kernel issuance
	// may escape the cached branch.
	require.NoError(t, az.DeleteGrant(ctx, grantee, granter, sdk.MsgTypeURL(&banktypes.MsgSend{})))
	// Kernel grants are unique per (grantor, key). Rotate the grantee's real
	// binding to a new key so issuance reaches the existing SDK address pair.
	evm := &testkeeper.EVMTestApp.EvmKeeper
	address := common.BytesToAddress(grantee)
	_, _, err = evm.UnbindLayerX(ctx, address)
	require.NoError(t, err)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{75}, ed25519.SeedSize))
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	bindMessage, err := evmtypes.LayerXBindMessage(evm.ChainID(ctx), address, evm.GetLayerXBindNonce(ctx, address))
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(private, bindMessage))
	_, err = evm.BindLayerX(ctx, address, public, signature)
	require.NoError(t, err)
	before = authViewStores(ctx, f.HostKey(), f.StoreKey(), azKey)
	secondBody := bytes.Clone(payload[8:])
	copy(secondBody[82:114], public[:])
	secondID, err := codec.DomainHash(codec.DomainAuthorityHash, secondBody)
	require.NoError(t, err)
	require.NotEqual(t, firstID, secondID)
	_, err = f.Keeper().GrantCapability(ctx, actor, secondBody, [32]byte{0x19, 0x22})
	require.ErrorIs(t, err, types.ErrAuthScope)
	require.ErrorContains(t, err, sdk.MsgTypeURL(&banktypes.MsgMultiSend{}))
	require.Equal(t, before, authViewStores(ctx, f.HostKey(), f.StoreKey(), azKey))
	_, err = f.Host().ReadGrant(ctx, secondID)
	require.Equal(t, lxphost.Code(-7), err)
	remaining, _ := az.GetCleanAuthorization(ctx, grantee, granter, sdk.MsgTypeURL(&banktypes.MsgMultiSend{}))
	require.Equal(t, firstID[:], remaining.(*types.LayerXScopeAuthorization).GrantId)
	missing, _ := az.GetCleanAuthorization(ctx, grantee, granter, sdk.MsgTypeURL(&banktypes.MsgSend{}))
	require.Nil(t, missing)
}

func TestRepair_192_UnsupportedScopesDoNotPublish(t *testing.T) {
	for _, check := range []struct {
		name       string
		moduleMask uint64
		min, max   uint16
	}{
		{"unmapped_program_activity", 1 << 9, 4, 5},
		{"send_below_range", 1 << 1, 6, 6},
		{"send_above_range", 1 << 1, 4, 4},
	} {
		t.Run(check.name, func(t *testing.T) {
			f := newAuthViewFixture(t)
			actor := authExecuteActor(t, f)
			_, azKey := capabilityAuthz(t, f)
			payload, _ := executableCapabilityPayload(t, f)
			repair192Addresses(t, f)
			body := bytes.Clone(payload[8:])
			binary.BigEndian.PutUint64(body[114:122], check.moduleMask)
			binary.BigEndian.PutUint16(body[122:124], check.min)
			binary.BigEndian.PutUint16(body[124:126], check.max)
			id, err := codec.DomainHash(codec.DomainAuthorityHash, body)
			require.NoError(t, err)
			ctx := *f.Context()
			before := authViewStores(ctx, f.HostKey(), f.StoreKey(), azKey)
			_, err = f.Keeper().GrantCapability(ctx, actor, body, [32]byte{0x19, 0x23})
			require.ErrorIs(t, err, types.ErrAuthScope)
			require.ErrorContains(t, err, "no supported executable SDK message")
			require.Equal(t, before, authViewStores(ctx, f.HostKey(), f.StoreKey(), azKey))
			_, err = f.Host().ReadGrant(ctx, id)
			require.Equal(t, lxphost.Code(-7), err)
		})
	}
}
