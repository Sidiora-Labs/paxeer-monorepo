package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	keeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdkcodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	authzkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/keeper"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func capabilityAuthz(t *testing.T, f *authViewFixture) (authzkeeper.Keeper, *sdk.KVStoreKey) {
	t.Helper()
	registry := cdctypes.NewInterfaceRegistry()
	authz.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	key := sdk.NewKVStoreKey("capability_authz")
	*f.Context() = withStore(*f.Context(), key)
	k := authzkeeper.NewKeeper(key, sdkcodec.NewProtoCodec(registry), baseapp.NewMsgServiceRouter())
	*f.Keeper().TestAuthzKeeper() = k
	return k, key
}

func bindCapabilityKey(t *testing.T, f *authViewFixture, private ed25519.PrivateKey) common.Address {
	t.Helper()
	address := common.Address{0x27, 0x02, 2}
	evm := &testkeeper.EVMTestApp.EvmKeeper
	message, err := evmtypes.LayerXBindMessage(evm.ChainID(*f.Context()), address, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(private, message))
	var key [32]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	_, err = evm.BindLayerX(*f.Context(), address, key, signature)
	require.NoError(t, err)
	return address
}

func TestGrantCapability(t *testing.T) {
	t.Run("mirror", func(t *testing.T) {
		f := newAuthViewFixture(t)
		actor := authExecuteActor(t, f)
		az, _ := capabilityAuthz(t, f)
		payload, grantID := authExecutePayload(t, f, "capability")
		private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{74}, ed25519.SeedSize))
		grantee := sdk.AccAddress(bindCapabilityKey(t, f, private).Bytes())
		granter := sdk.AccAddress(common.Address{0x26, 0x64, 1}.Bytes())
		idempotency := [32]byte{0x27, 2}
		result, err := f.Keeper().GrantCapability(*f.Context(), actor, payload[8:], idempotency)
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), result.ResultCode)
		require.Equal(t, uint64(44), result.GlobalSequence)
		events, err := keeper.AuthEvents(result.Receipt)
		require.NoError(t, err)
		require.Equal(t, "CapabilityGranted", events[0].Name)
		require.Equal(t, grantID, events[0].GrantID)
		body, err := f.Keeper().GetGrant(*f.Context(), grantID)
		require.NoError(t, err)
		require.Equal(t, payload[8:], body)
		stored, expiry := az.GetCleanAuthorization(*f.Context(), grantee, granter, keeper.CapabilityAuthorizationTypeURL)
		require.IsType(t, &types.LayerXScopeAuthorization{}, stored)
		require.Equal(t, &types.LayerXScopeAuthorization{
			ModuleMask: 1 << 9, OrdinalMin: 4, OrdinalMax: 5,
			AssetId: f.Asset()[:], MaxPerActivity: "10", MaxTotal: "20", SpentTotal: "0",
			RevocationEpoch: 41, GrantId: grantID[:], MsgTypeUrl: keeper.CapabilityAuthorizationTypeURL,
			Kind: 3, PeriodLength: 1000, MaxPerPeriod: "15", PurposeHash: f.Purpose()[:],
			NotBefore: f.Timestamp() - 1000, NotAfter: f.Timestamp() + 1000,
		}, stored)
		require.Equal(t, time.UnixMilli(int64(f.Timestamp()+1000)).UTC(), expiry)
		require.Len(t, az.GetAuthorizations(*f.Context(), grantee, granter), 1)
		require.Empty(t, az.GetAuthorizations(*f.Context(), granter, grantee))
		require.Empty(t, az.GetAuthorizations(*f.Context(), granter, granter))
		_, err = f.Keeper().GrantCapability(*f.Context(), actor, payload[8:], idempotency)
		require.ErrorIs(t, err, types.ErrIdempotentReplay)
		again, againExpiry := az.GetCleanAuthorization(*f.Context(), grantee, granter, keeper.CapabilityAuthorizationTypeURL)
		require.Equal(t, stored, again)
		require.Equal(t, expiry, againExpiry)
		expired := f.Context().WithBlockTime(expiry.Add(time.Millisecond))
		gone, _ := az.GetCleanAuthorization(expired, grantee, granter, keeper.CapabilityAuthorizationTypeURL)
		require.Nil(t, gone)
	})

	t.Run("pause before encoding", func(t *testing.T) {
		f := newAuthViewFixture(t)
		actor := authExecuteActor(t, f)
		_, azKey := capabilityAuthz(t, f)
		params := f.Keeper().GetParams(*f.Context())
		params.Paused = true
		require.NoError(t, f.Keeper().SetParams(*f.Context(), params))
		before := authViewStores(*f.Context(), f.HostKey(), azKey)
		_, err := f.Keeper().GrantCapability(*f.Context(), actor, nil, [32]byte{1})
		require.ErrorIs(t, err, govtypes.ErrPausedScope)
		require.Contains(t, err.Error(), "layerxauth.grantCapability")
		require.Equal(t, before, authViewStores(*f.Context(), f.HostKey(), azKey))
	})

	t.Run("unbound key rolls back issuance", func(t *testing.T) {
		f := newAuthViewFixture(t)
		actor := authExecuteActor(t, f)
		_, azKey := capabilityAuthz(t, f)
		payload, _ := authExecutePayload(t, f, "capability")
		before := authViewStores(*f.Context(), f.HostKey(), azKey)
		_, err := f.Keeper().GrantCapability(*f.Context(), actor, payload[8:], [32]byte{2})
		require.ErrorIs(t, err, types.ErrAuthScope)
		require.Equal(t, before, authViewStores(*f.Context(), f.HostKey(), azKey))
	})
}

func TestGrantCapabilityRefusesOwnKey(t *testing.T) {
	f := newAuthViewFixture(t)
	actor := authExecuteActor(t, f)
	az, azKey := capabilityAuthz(t, f)
	*f.CapabilityKey() = *f.Public()
	body := f.GrantBody(3)
	grantID, err := codec.DomainHash(codec.DomainAuthorityHash, body)
	require.NoError(t, err)
	before := authViewStores(*f.Context(), azKey)
	result, err := f.Keeper().GrantCapability(*f.Context(), actor, body, [32]byte{0x27, 3})
	require.ErrorIs(t, err, types.ErrAuthScope)
	require.Equal(t, lxphost.Code(-204), result.ResultCode)
	require.NotEmpty(t, result.Encoded)
	require.Equal(t, before, authViewStores(*f.Context(), azKey))
	granter := sdk.AccAddress(common.Address{0x26, 0x64, 1}.Bytes())
	require.Empty(t, az.GetAuthorizations(*f.Context(), granter, granter))
	_, err = f.Host().ReadGrant(*f.Context(), grantID)
	require.Equal(t, lxphost.Code(-7), err)
	for _, event := range result.Events {
		require.NotEqual(t, uint16(0x7148), event.Code)
	}
}
