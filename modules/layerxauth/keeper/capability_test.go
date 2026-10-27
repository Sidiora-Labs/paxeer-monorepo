package keeper

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
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
	f.ctx = withStore(f.ctx, key)
	k := authzkeeper.NewKeeper(key, sdkcodec.NewProtoCodec(registry), baseapp.NewMsgServiceRouter())
	f.keeper.authzKeeper = k
	return k, key
}

func bindCapabilityKey(t *testing.T, f *authViewFixture, private ed25519.PrivateKey) common.Address {
	t.Helper()
	address := common.Address{0x27, 0x02, 2}
	evm := &testkeeper.EVMTestApp.EvmKeeper
	message, err := evmtypes.LayerXBindMessage(evm.ChainID(f.ctx), address, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(private, message))
	var key [32]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	_, err = evm.BindLayerX(f.ctx, address, key, signature)
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
		result, err := f.keeper.GrantCapability(f.ctx, actor, payload[8:], idempotency)
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), result.ResultCode)
		require.Equal(t, uint64(44), result.GlobalSequence)
		events, err := AuthEvents(result.Receipt)
		require.NoError(t, err)
		require.Equal(t, "CapabilityGranted", events[0].Name)
		require.Equal(t, grantID, events[0].GrantID)
		body, err := f.keeper.GetGrant(f.ctx, grantID)
		require.NoError(t, err)
		require.Equal(t, payload[8:], body)
		stored, expiry := az.GetCleanAuthorization(f.ctx, grantee, granter, capabilityAuthorizationTypeURL)
		require.IsType(t, &types.LayerXScopeAuthorization{}, stored)
		require.Equal(t, &types.LayerXScopeAuthorization{
			ModuleMask: 1 << 9, OrdinalMin: 4, OrdinalMax: 5,
			AssetId: f.asset[:], MaxPerActivity: "10", MaxTotal: "20", SpentTotal: "0",
			RevocationEpoch: 41, GrantId: grantID[:], MsgTypeUrl: capabilityAuthorizationTypeURL,
			Kind: 3, PeriodLength: 1000, MaxPerPeriod: "15", PurposeHash: f.purpose[:],
			NotBefore: f.timestamp - 1000, NotAfter: f.timestamp + 1000,
		}, stored)
		require.Equal(t, time.UnixMilli(int64(f.timestamp+1000)).UTC(), expiry)
		require.Len(t, az.GetAuthorizations(f.ctx, grantee, granter), 1)
		require.Empty(t, az.GetAuthorizations(f.ctx, granter, grantee))
		require.Empty(t, az.GetAuthorizations(f.ctx, granter, granter))
		_, err = f.keeper.GrantCapability(f.ctx, actor, payload[8:], idempotency)
		require.ErrorIs(t, err, types.ErrIdempotentReplay)
		again, againExpiry := az.GetCleanAuthorization(f.ctx, grantee, granter, capabilityAuthorizationTypeURL)
		require.Equal(t, stored, again)
		require.Equal(t, expiry, againExpiry)
		expired := f.ctx.WithBlockTime(expiry.Add(time.Millisecond))
		gone, _ := az.GetCleanAuthorization(expired, grantee, granter, capabilityAuthorizationTypeURL)
		require.Nil(t, gone)
	})

	t.Run("pause before encoding", func(t *testing.T) {
		f := newAuthViewFixture(t)
		actor := authExecuteActor(t, f)
		_, azKey := capabilityAuthz(t, f)
		params := f.keeper.GetParams(f.ctx)
		params.Paused = true
		require.NoError(t, f.keeper.SetParams(f.ctx, params))
		before := authViewStores(f.ctx, f.hostKey, azKey)
		_, err := f.keeper.GrantCapability(f.ctx, actor, nil, [32]byte{1})
		require.ErrorIs(t, err, govtypes.ErrPausedScope)
		require.Contains(t, err.Error(), "layerxauth.grantCapability")
		require.Equal(t, before, authViewStores(f.ctx, f.hostKey, azKey))
	})

	t.Run("unbound key rolls back issuance", func(t *testing.T) {
		f := newAuthViewFixture(t)
		actor := authExecuteActor(t, f)
		_, azKey := capabilityAuthz(t, f)
		payload, _ := authExecutePayload(t, f, "capability")
		before := authViewStores(f.ctx, f.hostKey, azKey)
		_, err := f.keeper.GrantCapability(f.ctx, actor, payload[8:], [32]byte{2})
		require.ErrorIs(t, err, types.ErrAuthScope)
		require.Equal(t, before, authViewStores(f.ctx, f.hostKey, azKey))
	})
}

func TestGrantCapabilityRefusesOwnKey(t *testing.T) {
	f := newAuthViewFixture(t)
	actor := authExecuteActor(t, f)
	az, azKey := capabilityAuthz(t, f)
	f.capabilityKey = f.public
	body := f.grantBody(3)
	grantID, err := codec.DomainHash(codec.DomainAuthorityHash, body)
	require.NoError(t, err)
	before := authViewStores(f.ctx, azKey)
	result, err := f.keeper.GrantCapability(f.ctx, actor, body, [32]byte{0x27, 3})
	require.ErrorIs(t, err, types.ErrAuthScope)
	require.Equal(t, lxphost.Code(-204), result.ResultCode)
	require.NotEmpty(t, result.Encoded)
	require.Equal(t, before, authViewStores(f.ctx, azKey))
	granter := sdk.AccAddress(common.Address{0x26, 0x64, 1}.Bytes())
	require.Empty(t, az.GetAuthorizations(f.ctx, granter, granter))
	_, err = f.host.ReadGrant(f.ctx, grantID)
	require.Equal(t, lxphost.Code(-7), err)
	for _, event := range result.Events {
		require.NotEqual(t, uint16(0x7148), event.Code)
	}
}
