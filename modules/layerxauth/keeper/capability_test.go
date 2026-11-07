package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
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
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func capabilityAuthz(t *testing.T, f *authViewFixture) (authzkeeper.Keeper, *sdk.KVStoreKey) {
	t.Helper()
	registry := cdctypes.NewInterfaceRegistry()
	authz.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	key := sdk.NewKVStoreKey("capability_authz")
	*f.Context() = withStore(*f.Context(), key)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	banktypes.RegisterMsgServer(router, bankkeeper.NewMsgServerImpl(testkeeper.EVMTestApp.BankKeeper))
	k := authzkeeper.NewKeeper(key, sdkcodec.NewProtoCodec(registry), router)
	*f.Keeper().TestAuthzKeeper() = k
	return k, key
}

func executableCapabilityPayload(t *testing.T, f *authViewFixture) ([]byte, [32]byte) {
	t.Helper()
	payload, _ := authExecutePayload(t, f, "capability")
	body := bytes.Clone(payload[8:])
	binary.BigEndian.PutUint64(body[114:122], 1<<1)
	id, err := codec.DomainHash(codec.DomainAuthorityHash, body)
	require.NoError(t, err)
	payload, err = types.EncodeCapabilityPayload(body)
	require.NoError(t, err)
	return payload, id
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
	t.Run("executable capability endpoint", TestRepairM_1421_CapabilityEndpoint)

	t.Run("mirror", func(t *testing.T) {
		f := newAuthViewFixture(t)
		actor := authExecuteActor(t, f)
		az, _ := capabilityAuthz(t, f)
		payload, grantID := executableCapabilityPayload(t, f)
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
			ModuleMask: 1 << 1, OrdinalMin: 4, OrdinalMax: 5,
			AssetId: f.Asset()[:], MaxPerActivity: "10", MaxTotal: "20", SpentTotal: "0",
			RevocationEpoch: 41, GrantId: grantID[:], MsgTypeUrl: keeper.CapabilityAuthorizationTypeURL,
			Kind: 3, PeriodLength: 1000, MaxPerPeriod: "15", PurposeHash: f.Purpose()[:],
			NotBefore: f.Timestamp() - 1000, NotAfter: f.Timestamp() + 1000,
		}, stored)
		require.Equal(t, time.UnixMilli(int64(f.Timestamp()+1000)).UTC(), expiry)
		require.Len(t, az.GetAuthorizations(*f.Context(), grantee, granter), 2)
		multi, multiExpiry := az.GetCleanAuthorization(*f.Context(), grantee, granter, sdk.MsgTypeURL(&banktypes.MsgMultiSend{}))
		expectedMulti := *stored.(*types.LayerXScopeAuthorization)
		expectedMulti.MsgTypeUrl = sdk.MsgTypeURL(&banktypes.MsgMultiSend{})
		require.Equal(t, &expectedMulti, multi)
		require.Equal(t, expiry, multiExpiry)
		absent, _ := az.GetCleanAuthorization(*f.Context(), grantee, granter, "/paxprotocol.paxchain.layerxauth.LayerXScopeAuthorization")
		require.Nil(t, absent)
		require.Empty(t, az.GetAuthorizations(*f.Context(), granter, grantee))
		require.Empty(t, az.GetAuthorizations(*f.Context(), granter, granter))
		replayed, err := f.Keeper().GrantCapability(*f.Context(), actor, payload[8:], idempotency)
		require.ErrorIs(t, err, types.ErrSequenceReused)
		require.Equal(t, lxphost.Code(-301), replayed.ResultCode)
		require.NotEmpty(t, replayed.Encoded)
		body, err = f.Keeper().GetGrant(*f.Context(), grantID)
		require.NoError(t, err)
		require.Equal(t, payload[8:], body)
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
		payload, _ := executableCapabilityPayload(t, f)
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
