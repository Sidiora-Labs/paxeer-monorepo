package keeper

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"
	"time"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	authzkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/keeper"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func authRevokeKeeper(t *testing.T, f *authViewFixture) (authzkeeper.Keeper, *sdk.KVStoreKey) {
	t.Helper()
	registry := cdctypes.NewInterfaceRegistry()
	authz.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	key := sdk.NewKVStoreKey("authz_revoke")
	f.ctx = withStore(f.ctx, key)
	keeper := authzkeeper.NewKeeper(key, codec.NewProtoCodec(registry), baseapp.NewMsgServiceRouter())
	f.keeper.authzKeeper = keeper
	return keeper, key
}

func authRevokeAuthorization(f *authViewFixture, grantID [32]byte, msgType string) *types.LayerXScopeAuthorization {
	return &types.LayerXScopeAuthorization{
		GrantId: bytes.Clone(grantID[:]), MsgTypeUrl: msgType, Kind: 3,
		ModuleMask: 1 << 9, OrdinalMin: 4, OrdinalMax: 5,
		AssetId: bytes.Clone(f.asset[:]), MaxPerActivity: "10", MaxTotal: "20",
		RevocationEpoch: 41, NotBefore: f.timestamp - 1000, NotAfter: f.timestamp + 1000,
		PeriodLength: 1000, MaxPerPeriod: "15", PurposeHash: bytes.Clone(f.purpose[:]),
	}
}

func TestRevokeGrantAdvancesRevocation(t *testing.T) {
	for _, kind := range []string{"capability", "session"} {
		t.Run(kind, func(t *testing.T) {
			f := newAuthViewFixture(t)
			actor := authExecuteActor(t, f)
			authzKeeper, _ := authRevokeKeeper(t, f)
			grantID := f.capabilityID
			if kind == "session" {
				grantID = f.sessionID
			}
			granter := sdk.AccAddress(bytes.Repeat([]byte{0xa1}, 20))
			grantee := sdk.AccAddress(bytes.Repeat([]byte{0xb1}, 20))
			otherGrantee := sdk.AccAddress(bytes.Repeat([]byte{0xb2}, 20))
			const send = "/cosmos.bank.v1beta1.MsgSend"
			const multiSend = "/cosmos.bank.v1beta1.MsgMultiSend"
			expiration := f.ctx.BlockTime().Add(time.Hour)
			matching := authRevokeAuthorization(f, grantID, send)
			require.NoError(t, matching.ValidateBasic())
			if kind == "capability" {
				require.NoError(t, authzKeeper.SaveGrant(f.ctx, grantee, granter, matching, expiration))
				require.NoError(t, authzKeeper.SaveGrant(f.ctx, otherGrantee, granter, matching, expiration))
			}
			unrelated := authRevokeAuthorization(f, [32]byte{0xfe}, multiSend)
			require.NoError(t, unrelated.ValidateBasic())
			require.NoError(t, authzKeeper.SaveGrant(f.ctx, grantee, granter, unrelated, expiration))
			generic := authz.NewGenericAuthorization(multiSend)
			require.NoError(t, authzKeeper.SaveGrant(f.ctx, otherGrantee, granter, generic, expiration))
			require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{Paused: true}))
			before, err := f.keeper.RevocationSequence(f.ctx, f.public)
			require.NoError(t, err)
			require.Equal(t, uint64(41), before)
			live, reason, err := f.keeper.IsLive(f.ctx, grantID, 43, f.timestamp)
			require.NoError(t, err)
			require.True(t, live)
			require.Empty(t, reason)
			f.ctx = f.ctx.WithEventManager(sdk.NewEventManager())

			result, err := f.keeper.RevokeGrant(f.ctx, actor, grantID, 1, 44, [32]byte{0x93})
			require.NoError(t, err)
			require.Equal(t, lxphost.Code(0), result.ResultCode)
			require.Equal(t, uint64(44), result.GlobalSequence)
			events, err := AuthEvents(result.Receipt)
			require.NoError(t, err)
			require.Len(t, events, 2)
			record := append(bytes.Clone(grantID[:]), 1)
			record = binary.BigEndian.AppendUint64(record, 44)
			require.Equal(t, Event{Name: "GrantRevoked", GrantID: grantID, Body: record}, events[0])
			require.Equal(t, "IdentityStateUpdated", events[1].Name)
			require.Equal(t, actor.Authority.Actor, events[1].DIDID)
			after, err := f.keeper.RevocationSequence(f.ctx, f.public)
			require.NoError(t, err)
			require.Equal(t, uint64(44), after)
			require.Greater(t, after, before)
			grant, err := f.host.ReadGrant(f.ctx, grantID)
			require.NoError(t, err)
			require.True(t, grant.Revoked)
			require.Equal(t, uint64(44), grant.RevokedAtSequence)
			live, reason, err = f.keeper.IsLive(f.ctx, grantID, 44, f.timestamp)
			require.NoError(t, err)
			require.False(t, live)
			require.Equal(t, "LXP_AUTH_REVOKED", reason)
			for _, address := range []sdk.AccAddress{grantee, otherGrantee} {
				remaining, _ := authzKeeper.GetCleanAuthorization(f.ctx, address, granter, send)
				require.Nil(t, remaining)
			}
			remaining, expiry := authzKeeper.GetCleanAuthorization(f.ctx, grantee, granter, multiSend)
			require.Equal(t, unrelated, remaining)
			require.Equal(t, expiration, expiry)
			remaining, _ = authzKeeper.GetCleanAuthorization(f.ctx, otherGrantee, granter, multiSend)
			require.Equal(t, generic, remaining)
			if kind == "capability" {
				require.Len(t, f.ctx.EventManager().Events(), 2)
				for _, event := range f.ctx.EventManager().Events() {
					require.Equal(t, "cosmos.authz.v1beta1.EventRevoke", event.Type)
				}
			}
			state := authViewStores(f.ctx, f.hostKey, f.harness.key)
			_, err = f.keeper.RevokeGrant(f.ctx, actor, grantID, 1, 44, [32]byte{0x93})
			require.ErrorIs(t, err, types.ErrIdempotentReplay)
			require.Equal(t, state, authViewStores(f.ctx, f.hostKey, f.harness.key))
		})
	}
}

func TestRevokeGrantRefusesOtherGrantor(t *testing.T) {
	f := newAuthViewFixture(t)
	_ = authExecuteActor(t, f)
	authzKeeper, authzKey := authRevokeKeeper(t, f)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{75}, ed25519.SeedSize))
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	caller := common.Address{0x27, 0x73, 2}
	evm := &testkeeper.EVMTestApp.EvmKeeper
	message, err := evmtypes.LayerXBindMessage(evm.ChainID(f.ctx), caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = evm.BindLayerX(f.ctx, caller, public, signature)
	require.NoError(t, err)
	actor, err := f.keeper.Actor(f.ctx, caller)
	require.NoError(t, err)
	require.NotEqual(t, f.didID, actor.Authority.Actor)
	granter := sdk.AccAddress(bytes.Repeat([]byte{0xc1}, 20))
	grantee := sdk.AccAddress(bytes.Repeat([]byte{0xd1}, 20))
	const msgType = "/cosmos.bank.v1beta1.MsgSend"
	authorization := authRevokeAuthorization(f, f.capabilityID, msgType)
	expiration := f.ctx.BlockTime().Add(time.Hour)
	require.NoError(t, authzKeeper.SaveGrant(f.ctx, grantee, granter, authorization, expiration))
	before := authViewStores(f.ctx, f.hostKey, f.harness.key, authzKey)
	eventsBefore := f.ctx.EventManager().Events()

	for _, grantID := range [][32]byte{f.capabilityID, f.sessionID} {
		_, err = f.keeper.RevokeGrant(f.ctx, actor, grantID, 1, 44, [32]byte{0x94})
		require.ErrorIs(t, err, types.ErrUnauthorizedDebit)
		require.Equal(t, before, authViewStores(f.ctx, f.hostKey, f.harness.key, authzKey))
		require.Equal(t, eventsBefore, f.ctx.EventManager().Events())
		live, reason, err := f.keeper.IsLive(f.ctx, grantID, 43, f.timestamp)
		require.NoError(t, err)
		require.True(t, live)
		require.Empty(t, reason)
	}
	remaining, expiry := authzKeeper.GetCleanAuthorization(f.ctx, grantee, granter, msgType)
	require.Equal(t, authorization, remaining)
	require.Equal(t, expiration, expiry)
	revocation, err := f.keeper.RevocationSequence(f.ctx, f.public)
	require.NoError(t, err)
	require.Equal(t, uint64(41), revocation)
}
