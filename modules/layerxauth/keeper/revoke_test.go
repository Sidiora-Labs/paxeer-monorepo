package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"
	"time"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	keeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	authzkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
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
	*f.Context() = withStore(*f.Context(), key)
	keeper := authzkeeper.NewKeeper(key, codec.NewProtoCodec(registry), baseapp.NewMsgServiceRouter())
	*f.Keeper().TestAuthzKeeper() = keeper
	return keeper, key
}

func authRevokeAuthorization(f *authViewFixture, grantID [32]byte, msgType string) *types.LayerXScopeAuthorization {
	return &types.LayerXScopeAuthorization{
		GrantId: bytes.Clone(grantID[:]), MsgTypeUrl: msgType, Kind: 3,
		ModuleMask: 1 << 9, OrdinalMin: 4, OrdinalMax: 5,
		AssetId: bytes.Clone(f.Asset()[:]), MaxPerActivity: "10", MaxTotal: "20",
		RevocationEpoch: 41, NotBefore: f.Timestamp() - 1000, NotAfter: f.Timestamp() + 1000,
		PeriodLength: 1000, MaxPerPeriod: "15", PurposeHash: bytes.Clone(f.Purpose()[:]),
	}
}

func TestRevokeGrantAdvancesRevocation(t *testing.T) {
	for _, kind := range []string{"capability", "session"} {
		t.Run(kind, func(t *testing.T) {
			f := newAuthViewFixture(t)
			actor := authExecuteActor(t, f)
			authzKeeper, authzKey := authRevokeKeeper(t, f)
			grantID := *f.CapabilityID()
			if kind == "session" {
				grantID = *f.SessionID()
			}
			granter := sdk.AccAddress(bytes.Repeat([]byte{0xa1}, 20))
			grantee := sdk.AccAddress(bytes.Repeat([]byte{0xb1}, 20))
			otherGrantee := sdk.AccAddress(bytes.Repeat([]byte{0xb2}, 20))
			const send = "/cosmos.bank.v1beta1.MsgSend"
			const multiSend = "/cosmos.bank.v1beta1.MsgMultiSend"
			expiration := f.Context().BlockTime().Add(time.Hour)
			message := banktypes.NewMsgSend(granter, grantee, sdk.NewCoins(sdk.NewInt64Coin(ledgertypes.MirrorDenom(*f.Asset()), 1)))
			purpose, err := types.BankSendPurpose(message)
			require.NoError(t, err)
			matching := authRevokeAuthorization(f, grantID, send)
			matching.ModuleMask = 1 << types.BankScopeModule
			matching.PurposeHash = bytes.Clone(purpose[:])
			require.NoError(t, matching.ValidateBasic())
			if kind == "session" {
				matching.Kind = 2
			}
			require.NoError(t, authzKeeper.SaveGrant(*f.Context(), grantee, granter, matching, expiration))
			require.NoError(t, authzKeeper.SaveGrant(*f.Context(), otherGrantee, granter, matching, expiration))
			accepted, err := matching.Accept(types.WithGrantorRevocation(*f.Context(), granter, matching.RevocationEpoch), message)
			require.NoError(t, err)
			require.True(t, accepted.Accept)
			unrelated := authRevokeAuthorization(f, [32]byte{0xfe}, multiSend)
			require.NoError(t, unrelated.ValidateBasic())
			require.NoError(t, authzKeeper.SaveGrant(*f.Context(), grantee, granter, unrelated, expiration))
			generic := authz.NewGenericAuthorization(multiSend)
			require.NoError(t, authzKeeper.SaveGrant(*f.Context(), otherGrantee, granter, generic, expiration))
			require.NoError(t, f.Keeper().SetParams(*f.Context(), types.Params{Paused: true}))
			before, err := f.Keeper().RevocationSequence(*f.Context(), *f.Public())
			require.NoError(t, err)
			require.Equal(t, uint64(41), before)
			live, reason, err := f.Keeper().IsLive(*f.Context(), grantID, 43, f.Timestamp())
			require.NoError(t, err)
			require.True(t, live)
			require.Empty(t, reason)
			*f.Context() = f.Context().WithEventManager(sdk.NewEventManager())

			result, err := f.Keeper().RevokeGrant(*f.Context(), actor, grantID, 1, 44, [32]byte{0x93})
			require.NoError(t, err)
			require.Equal(t, lxphost.Code(0), result.ResultCode)
			require.Equal(t, uint64(44), result.GlobalSequence)
			events, err := keeper.AuthEvents(result.Receipt)
			require.NoError(t, err)
			require.Len(t, events, 2)
			record := append(bytes.Clone(grantID[:]), 1)
			record = binary.BigEndian.AppendUint64(record, 44)
			require.Equal(t, keeper.Event{Name: "GrantRevoked", GrantID: grantID, Body: record}, events[0])
			require.Equal(t, "IdentityStateUpdated", events[1].Name)
			require.Equal(t, actor.Authority.Actor, events[1].DIDID)
			after, err := f.Keeper().RevocationSequence(*f.Context(), *f.Public())
			require.NoError(t, err)
			require.Equal(t, uint64(44), after)
			require.Greater(t, after, before)
			grant, err := f.Host().ReadGrant(*f.Context(), grantID)
			require.NoError(t, err)
			require.True(t, grant.Revoked)
			require.Equal(t, uint64(44), grant.RevokedAtSequence)
			live, reason, err = f.Keeper().IsLive(*f.Context(), grantID, 44, f.Timestamp())
			require.NoError(t, err)
			require.False(t, live)
			require.Equal(t, "LXP_AUTH_REVOKED", reason)
			for _, address := range []sdk.AccAddress{grantee, otherGrantee} {
				remaining, expiry := authzKeeper.GetCleanAuthorization(*f.Context(), address, granter, send)
				require.Equal(t, matching, remaining)
				require.Equal(t, expiration, expiry)
				require.Equal(t, before, remaining.(*types.LayerXScopeAuthorization).RevocationEpoch)
				refused, err := remaining.Accept(types.WithGrantorRevocation(*f.Context(), granter, after), message)
				require.ErrorIs(t, err, types.ErrAuthRevoked)
				require.ErrorContains(t, err, "revocation epoch 41, grantor revocation 44")
				require.False(t, refused.Accept)
			}
			remaining, expiry := authzKeeper.GetCleanAuthorization(*f.Context(), grantee, granter, multiSend)
			require.Equal(t, unrelated, remaining)
			require.Equal(t, expiration, expiry)
			remaining, _ = authzKeeper.GetCleanAuthorization(*f.Context(), otherGrantee, granter, multiSend)
			require.Equal(t, generic, remaining)
			require.Empty(t, f.Context().EventManager().Events())
			state := authViewStores(*f.Context(), f.HostKey(), f.StoreKey(), authzKey)
			// Replays must be refused by kernel execution, without a module marker.
			_, err = f.Keeper().RevokeGrant(*f.Context(), actor, grantID, 1, 44, [32]byte{0x93})
			require.ErrorIs(t, err, types.ErrAuthScope)
			require.Equal(t, state, authViewStores(*f.Context(), f.HostKey(), f.StoreKey(), authzKey))
			_, err = f.Keeper().RevokeGrant(*f.Context(), keeper.Actor{}, grantID, 0, 0, [32]byte{0x93})
			require.ErrorIs(t, err, types.ErrAuthScope)
			_, err = f.Keeper().RevokeGrant(*f.Context(), keeper.Actor{}, grantID, 1, 44, [32]byte{0x94})
			require.ErrorIs(t, err, types.ErrAuthScope)
			_, err = f.Keeper().RevokeGrant(*f.Context(), keeper.Actor{}, [32]byte{0xff}, 1, 44, [32]byte{0x93})
			require.ErrorIs(t, err, types.ErrAuthScope)
			require.Equal(t, state, authViewStores(*f.Context(), f.HostKey(), f.StoreKey(), authzKey))
			require.Empty(t, f.Context().EventManager().Events())
		})
	}
}

func TestRevokeGrantReplayRequiresCommittedSuccess(t *testing.T) {
	for _, kind := range []string{"capability", "session"} {
		t.Run(kind, func(t *testing.T) {
			f := newAuthViewFixture(t)
			actor := authExecuteActor(t, f)
			grantID := *f.CapabilityID()
			if kind == "session" {
				grantID = *f.SessionID()
			}
			key := [32]byte{0x95}
			before := authViewStores(*f.Context(), f.HostKey(), f.StoreKey())
			_, err := f.Keeper().RevokeGrant(*f.Context(), actor, grantID, 0, 44, key)
			require.ErrorIs(t, err, types.ErrAuthScope)
			require.Equal(t, before, authViewStores(*f.Context(), f.HostKey(), f.StoreKey()))
			branch, _ := f.Context().CacheContext()
			_, err = f.Keeper().RevokeGrant(branch, actor, grantID, 1, 44, key)
			require.NoError(t, err)
			_, err = f.Keeper().RevokeGrant(branch, actor, grantID, 1, 44, key)
			require.ErrorIs(t, err, types.ErrAuthScope)
			require.Equal(t, before, authViewStores(*f.Context(), f.HostKey(), f.StoreKey()))
			_, err = f.Keeper().RevokeGrant(*f.Context(), actor, grantID, 1, 44, key)
			require.NoError(t, err)
			committed := authViewStores(*f.Context(), f.HostKey(), f.StoreKey())
			_, err = f.Keeper().RevokeGrant(*f.Context(), actor, grantID, 1, 44, key)
			require.ErrorIs(t, err, types.ErrAuthScope)
			require.Equal(t, committed, authViewStores(*f.Context(), f.HostKey(), f.StoreKey()))
			_, err = f.Keeper().RevokeGrant(*f.Context(), keeper.Actor{}, grantID, 1, 44, key)
			require.ErrorIs(t, err, types.ErrAuthScope)
			require.Equal(t, committed, authViewStores(*f.Context(), f.HostKey(), f.StoreKey()))
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
	message, err := evmtypes.LayerXBindMessage(evm.ChainID(*f.Context()), caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = evm.BindLayerX(*f.Context(), caller, public, signature)
	require.NoError(t, err)
	actor, err := f.Keeper().Actor(*f.Context(), caller)
	require.NoError(t, err)
	require.NotEqual(t, *f.DIDID(), actor.Authority.Actor)
	granter := sdk.AccAddress(bytes.Repeat([]byte{0xc1}, 20))
	grantee := sdk.AccAddress(bytes.Repeat([]byte{0xd1}, 20))
	const msgType = "/cosmos.bank.v1beta1.MsgSend"
	authorization := authRevokeAuthorization(f, *f.CapabilityID(), msgType)
	expiration := f.Context().BlockTime().Add(time.Hour)
	require.NoError(t, authzKeeper.SaveGrant(*f.Context(), grantee, granter, authorization, expiration))
	before := authViewStores(*f.Context(), f.HostKey(), f.StoreKey(), authzKey)
	eventsBefore := f.Context().EventManager().Events()

	for _, grantID := range [][32]byte{*f.CapabilityID(), *f.SessionID()} {
		_, err = f.Keeper().RevokeGrant(*f.Context(), actor, grantID, 1, 44, [32]byte{0x94})
		require.ErrorIs(t, err, types.ErrUnauthorizedDebit)
		require.Equal(t, before, authViewStores(*f.Context(), f.HostKey(), f.StoreKey(), authzKey))
		require.Equal(t, eventsBefore, f.Context().EventManager().Events())
		live, reason, err := f.Keeper().IsLive(*f.Context(), grantID, 43, f.Timestamp())
		require.NoError(t, err)
		require.True(t, live)
		require.Empty(t, reason)
	}
	remaining, expiry := authzKeeper.GetCleanAuthorization(*f.Context(), grantee, granter, msgType)
	require.Equal(t, authorization, remaining)
	require.Equal(t, expiration, expiry)
	revocation, err := f.Keeper().RevocationSequence(*f.Context(), *f.Public())
	require.NoError(t, err)
	require.Equal(t, uint64(41), revocation)
}
