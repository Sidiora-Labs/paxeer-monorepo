// Proves: lxphost.Host and keeper.Keeper.GrantSession/GrantCapability -> keeper.Result/lxphost.Receipt and exported Go views -> keeper.AuthEvents, SDK authz.Keeper.GetCleanAuthorization and TestProofLayerxauthKeeper consumers.
//
// The fixture registers identities in the real kernel, binds real signed EVM
// identities and mounts the real SDK stores; no host, codec or keeper is replaced.
// The consumer reads each result twice and mutates its first copy to rule out
// shared output buffers. The keeper owns no grant cache or private grant store.
// This repairs modules/layerxauth/keeper/events.go: AuthEvents previously accepted
// a grant summary with every GrantBody chunk missing. A truncated real producer
// receipt must be refused, including when an identity event follows the summary.
// modules/layerxauth/keeper/execute.go: Keeper.execute supplies the current account
// sequence on each call. Repeating an issued capability therefore currently
// returns LXP_SEQUENCE_REUSED, rather than the LXP_IDEMPOTENT_REPLAY expected by
// modules/layerxauth/keeper/capability_test.go: TestGrantCapability. This proof
// preserves that existing refusal and asserts the issued authz grant survives.
package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	keeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func proofLayerxauthReceipt(t *testing.T, result keeper.Result, id [32]byte, body []byte, summaryName string) {
	t.Helper()
	require.Equal(t, lxphost.Code(0), result.ResultCode)
	// Decode the bytes actually emitted by the host as well as the Go receipt.
	wire, err := codec.DecodeReceipt(result.Encoded)
	require.NoError(t, err)
	require.Equal(t, result.GlobalSequence, wire.GlobalSequence)
	require.Equal(t, int32(result.ResultCode), wire.ResultCode)
	var transported []lxphost.ReceiptEvent
	for _, effect := range wire.Effects {
		if effect.Kind == 3 {
			transported = append(transported, lxphost.ReceiptEvent{Code: effect.EventType, Body: effect.Body})
		}
	}
	require.Equal(t, result.Events, transported)
	for read := 0; read < 2; read++ {
		events, err := keeper.AuthEvents(result.Receipt)
		require.NoError(t, err)
		require.Len(t, events, 4)
		require.Equal(t, summaryName, events[0].Name)
		require.Equal(t, id, events[0].GrantID)
		require.Equal(t, "IdentityStateUpdated", events[3].Name)
		var observed []byte
		for i, event := range events[1:3] {
			require.Equal(t, "GrantBody", event.Name)
			require.Equal(t, id, event.GrantID)
			require.Equal(t, uint8(i), event.Chunk)
			observed = append(observed, event.Body...)
		}
		require.Equal(t, body, observed)
		for i := range events {
			events[i].Body[0] ^= 0xff
		}
	}

	// Derive malformed outputs only from the real receipt, without replacing
	// the producer or its transport. The unmodified receipt must still decode.
	for _, events := range [][]lxphost.ReceiptEvent{
		result.Events[:1],
		{result.Events[0], result.Events[len(result.Events)-1]},
	} {
		malformed := result.Receipt
		malformed.Events = events
		decoded, err := keeper.AuthEvents(malformed)
		require.ErrorIs(t, err, types.ErrNonCanonical)
		require.Nil(t, decoded)
	}
	malformed := result.Receipt
	malformed.Events = append([]lxphost.ReceiptEvent(nil), result.Events...)
	malformed.Events[0].Body = malformed.Events[0].Body[:len(malformed.Events[0].Body)-1]
	decoded, err := keeper.AuthEvents(malformed)
	require.ErrorIs(t, err, types.ErrNonCanonical)
	require.Nil(t, decoded)
	_, err = codec.DecodeReceipt(result.Encoded[:len(result.Encoded)-1])
	require.Error(t, err)
	_, err = keeper.AuthEvents(result.Receipt)
	require.NoError(t, err)
}

func TestProofLayerxauthKeeperSession(t *testing.T) {
	f := newAuthViewFixture(t)
	actor := authExecuteActor(t, f)
	_, grantID := authExecutePayload(t, f, "session")
	body := f.GrantBody(2)
	action, key := [32]byte{0xc1}, [32]byte{0xc2}
	const expiry = uint64(123)
	before := authViewStores(*f.Context(), f.HostKey(), f.StoreKey())
	refused, err := f.Keeper().GrantSession(*f.Context(), keeper.Actor{}, body, expiry, action, key)
	require.ErrorIs(t, err, types.ErrAuthScope)
	require.Equal(t, keeper.Result{}, refused)
	require.Equal(t, before, authViewStores(*f.Context(), f.HostKey(), f.StoreKey()))

	result, err := f.Keeper().GrantSession(*f.Context(), actor, body, expiry, action, key)
	require.NoError(t, err)
	proofLayerxauthReceipt(t, result, grantID, body, "SessionGranted")
	after := authViewStores(*f.Context(), f.HostKey(), f.StoreKey())
	for read := 0; read < 2; read++ {
		observed, err := f.Keeper().GetGrant(*f.Context(), grantID)
		require.NoError(t, err)
		require.Equal(t, body, observed)
		summary, err := f.Keeper().SessionKey(*f.Context(), grantID)
		require.NoError(t, err)
		require.Equal(t, result.Events[0].Body, summary)
		require.Equal(t, []byte("LXGS2"), summary[:5])
		require.Equal(t, grantID[:], summary[5:37])
		require.Equal(t, action[:], summary[101:133])
		require.Equal(t, f.Delegate()[:], summary[133:165])
		require.Equal(t, expiry, binary.BigEndian.Uint64(summary[165:173]))
		live, reason, err := f.Keeper().IsLive(*f.Context(), grantID, result.GlobalSequence, f.Timestamp())
		require.NoError(t, err)
		require.True(t, live)
		require.Empty(t, reason)
		observed[0] ^= 0xff
		summary[0] ^= 0xff
	}
	require.Equal(t, after, authViewStores(*f.Context(), f.HostKey(), f.StoreKey()))

	// A repeated producer call is refused and cannot replace the issued grant.
	replay, err := f.Keeper().GrantSession(*f.Context(), actor, body, expiry, action, key)
	require.ErrorIs(t, err, types.ErrSequenceReused)
	require.Equal(t, lxphost.Code(-301), replay.ResultCode)
	require.NotEmpty(t, replay.Encoded)
	observed, err := f.Keeper().GetGrant(*f.Context(), grantID)
	require.NoError(t, err)
	require.Equal(t, body, observed)
}

func TestProofLayerxauthKeeperCapability(t *testing.T) {
	t.Run("executable capability receipt endpoint", TestRepairM_1421_AuthProofEndpoint)

	f := newAuthViewFixture(t)
	actor := authExecuteActor(t, f)
	az, azKey := capabilityAuthz(t, f)
	payload, grantID := executableCapabilityPayload(t, f)
	body := bytes.Clone(payload[8:])
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{74}, ed25519.SeedSize))
	grantee := sdk.AccAddress(bindCapabilityKey(t, f, private).Bytes())
	granter := sdk.AccAddress(common.Address{0x26, 0x64, 1}.Bytes())
	key := [32]byte{0xd1}
	before := authViewStores(*f.Context(), f.HostKey(), f.StoreKey(), azKey)
	refused, err := f.Keeper().GrantCapability(*f.Context(), keeper.Actor{}, body, key)
	require.ErrorIs(t, err, types.ErrAuthScope)
	require.Equal(t, keeper.Result{}, refused)
	require.Equal(t, before, authViewStores(*f.Context(), f.HostKey(), f.StoreKey(), azKey))

	result, err := f.Keeper().GrantCapability(*f.Context(), actor, body, key)
	require.NoError(t, err)
	proofLayerxauthReceipt(t, result, grantID, body, "CapabilityGranted")
	after := authViewStores(*f.Context(), f.HostKey(), f.StoreKey(), azKey)
	for read := 0; read < 2; read++ {
		observed, err := f.Keeper().GetGrant(*f.Context(), grantID)
		require.NoError(t, err)
		require.Equal(t, body, observed)
		scope, err := f.Keeper().CapabilityScope(*f.Context(), grantID)
		require.NoError(t, err)
		require.Equal(t, body[114:294], scope)
		require.Equal(t, uint64(1<<1), binary.BigEndian.Uint64(scope[:8]))
		require.Equal(t, f.Asset()[:], scope[16:48])
		stored, expiration := az.GetCleanAuthorization(*f.Context(), grantee, granter, keeper.CapabilityAuthorizationTypeURL)
		require.IsType(t, &types.LayerXScopeAuthorization{}, stored)
		authorization := stored.(*types.LayerXScopeAuthorization)
		require.NoError(t, authorization.ValidateBasic())
		require.Equal(t, &types.LayerXScopeAuthorization{
			ModuleMask: 1 << 1, OrdinalMin: 4, OrdinalMax: 5,
			AssetId: f.Asset()[:], MaxPerActivity: "10", MaxTotal: "20", SpentTotal: "0",
			RevocationEpoch: 41, GrantId: grantID[:], MsgTypeUrl: keeper.CapabilityAuthorizationTypeURL,
			Kind: 3, PeriodLength: 1000, MaxPerPeriod: "15", PurposeHash: f.Purpose()[:],
			NotBefore: f.Timestamp() - 1000, NotAfter: f.Timestamp() + 1000,
		}, authorization)
		require.Equal(t, time.UnixMilli(int64(f.Timestamp()+1000)).UTC(), expiration)
		for _, check := range []struct {
			amount int64
			reason string
		}{{5, ""}, {11, "LXP_AUTH_SCOPE"}} {
			reason, err := f.Keeper().CheckActivity(*f.Context(), grantID, 0x00010004, *f.Asset(), sdk.NewInt(check.amount))
			require.NoError(t, err)
			require.Equal(t, check.reason, reason)
		}
		observed[0] ^= 0xff
		scope[0] ^= 0xff
		authorization.GrantId[0] ^= 0xff
		authorization.MaxPerActivity = "0"
	}
	require.Equal(t, after, authViewStores(*f.Context(), f.HostKey(), f.StoreKey(), azKey))
	for _, read := range []func(sdk.Context, [32]byte) ([]byte, error){f.Keeper().GetGrant, f.Keeper().CapabilityScope} {
		missing, err := read(*f.Context(), [32]byte{0xff})
		require.ErrorIs(t, err, keeper.FromKernel(lxphost.Code(-7)))
		require.Empty(t, missing)
	}
	replay, err := f.Keeper().GrantCapability(*f.Context(), actor, body, key)
	require.ErrorIs(t, err, types.ErrSequenceReused)
	require.Equal(t, lxphost.Code(-301), replay.ResultCode)
	require.NotEmpty(t, replay.Encoded)
	stored, _ := az.GetCleanAuthorization(*f.Context(), grantee, granter, keeper.CapabilityAuthorizationTypeURL)
	require.Equal(t, grantID[:], stored.(*types.LayerXScopeAuthorization).GrantId)
	message := banktypes.NewMsgSend(granter, grantee, sdk.NewCoins(sdk.NewInt64Coin("upax", 1)))
	authorization := *stored.(*types.LayerXScopeAuthorization)
	require.Equal(t, sdk.MsgTypeURL(message), authorization.MsgTypeUrl)
	expiration := time.UnixMilli(int64(authorization.NotAfter)).UTC()
	require.Equal(t, time.UnixMilli(int64(f.Timestamp()+1000)).UTC(), expiration)
	accepted, err := authorization.Accept(types.WithGrantorRevocation(*f.Context(), granter, authorization.RevocationEpoch), message)
	require.NoError(t, err)
	require.True(t, accepted.Accept)

	// A later host write must be visible, rather than a cached liveness result.
	sequence, err := lxphost.NextSequence(*f.Context(), f.HostKey())
	require.NoError(t, err)
	_, err = f.Keeper().RevokeGrant(*f.Context(), actor, grantID, 1, sequence, [32]byte{0xd2})
	require.NoError(t, err)
	revocation, err := f.Keeper().RevocationSequence(*f.Context(), *f.Public())
	require.NoError(t, err)
	require.Equal(t, sequence, revocation)
	for read := 0; read < 2; read++ {
		reason, err := f.Keeper().CheckActivity(*f.Context(), grantID, 0x00010004, *f.Asset(), sdk.NewInt(5))
		require.NoError(t, err)
		require.Equal(t, "LXP_AUTH_REVOKED", reason)
		stored, _ := az.GetCleanAuthorization(*f.Context(), grantee, granter, keeper.CapabilityAuthorizationTypeURL)
		require.IsType(t, &types.LayerXScopeAuthorization{}, stored)
		require.Equal(t, grantID[:], stored.(*types.LayerXScopeAuthorization).GrantId)
		require.Equal(t, uint64(41), stored.(*types.LayerXScopeAuthorization).RevocationEpoch)
		retained, expiry := az.GetCleanAuthorization(*f.Context(), grantee, granter, sdk.MsgTypeURL(message))
		require.Equal(t, &authorization, retained)
		require.Equal(t, expiration, expiry)
		refused, err := retained.Accept(types.WithGrantorRevocation(*f.Context(), granter, revocation), message)
		require.ErrorIs(t, err, types.ErrAuthRevoked)
		require.ErrorContains(t, err, "revocation epoch 41, grantor revocation ")
		require.False(t, refused.Accept)
	}
}

func TestProofLayerxauthKeeperParamsGenesis(t *testing.T) {
	h := newHarness(t)
	ctx, k := *h.Context(), h.Keeper()
	require.Equal(t, types.DefaultParams(), k.GetParams(ctx))
	require.False(t, ctx.KVStore(h.StoreKey()).Has(types.ParamsKey))
	params := types.Params{IdempotencyRetention: 64}
	k.InitGenesis(ctx, types.GenesisState{Params: params})
	for read := 0; read < 2; read++ {
		require.Equal(t, params, k.GetParams(ctx))
		exported := k.ExportGenesis(ctx)
		require.NoError(t, exported.Validate())
		require.Equal(t, params, exported.Params)
		encoded, err := json.Marshal(exported)
		require.NoError(t, err)
		var consumed types.GenesisState
		require.NoError(t, json.Unmarshal(encoded, &consumed))
		require.Equal(t, types.GenesisState{Params: params}, consumed)
		restored := newHarness(t)
		restored.Keeper().InitGenesis(*restored.Context(), consumed)
		require.Equal(t, params, restored.Keeper().GetParams(*restored.Context()))
		exported.Params.Paused = true
	}

	branch, _ := ctx.CacheContext()
	paused := params
	paused.Paused = true
	require.NoError(t, k.SetParams(branch, paused))
	for read := 0; read < 2; read++ {
		require.Equal(t, paused, k.GetParams(branch))
		require.Equal(t, paused, k.ExportGenesis(branch).Params)
		require.Equal(t, params, k.GetParams(ctx))
	}
	before := authViewStores(branch, h.HostKey(), h.StoreKey())
	_, err := k.GrantSession(branch, keeper.Actor{}, nil, 1, [32]byte{}, [32]byte{})
	require.ErrorIs(t, err, govtypes.ErrPausedScope)
	_, err = k.GrantCapability(branch, keeper.Actor{}, nil, [32]byte{})
	require.ErrorIs(t, err, govtypes.ErrPausedScope)
	require.Equal(t, before, authViewStores(branch, h.HostKey(), h.StoreKey()))
}
