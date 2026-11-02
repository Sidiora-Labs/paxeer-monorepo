// Proves: types.NewMsgSetDenomPaused, keeper.SetIssuerDid, keeper.SetDenomPausedByIssuer -> sdk.EventManager/WrapServiceResult/Result -> sdk.NewABCIMessageLog/ParseABCILogs.
// The admin producer calls keeper.NewMsgServerImpl(...).SetDenomPaused; all three
// event producers use the real bank, auth and tokenfactory keepers and disk store.
// Each result crosses protobuf and ABCI log encoding after a store close/reopen,
// and is consumed twice into fresh values; truncated results and logs are refused
// by the real decoders. AttributePaused is checked on both pause producers.
// Repairs modules/tokenfactory/keeper/msg_server.go: msgServer.SetDenomPaused
// omitted ValidateBasic, allowing an empty sender to match a cleared admin and
// mutate pause state while emitting a success event without a valid signer.
package types_test

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

type proofEventsState struct {
	dir    string
	store  storetypes.CommitMultiStore
	ctx    sdk.Context
	keeper keeper.Keeper
	server types.MsgServer
	admin  string
}

func newProofEventsState(t *testing.T) *proofEventsState {
	t.Helper()
	s := &proofEventsState{
		dir:   t.TempDir(),
		admin: sdk.AccAddress(bytes.Repeat([]byte{1}, 20)).String(),
	}
	s.open(t)
	t.Cleanup(func() { require.NoError(t, s.store.Close()) })
	return s
}

func (s *proofEventsState) open(t *testing.T) {
	t.Helper()
	db, err := tmdb.NewDB("proof-events", tmdb.GoLevelDBBackend, s.dir)
	require.NoError(t, err)
	s.store = store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(types.StoreKey)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey("proof_params")
	memKey := storetypes.NewMemoryStoreKey("proof_mem")
	for _, k := range []*sdk.KVStoreKey{key, bankKey, authKey, paramsKey} {
		s.store.MountStoreWithDB(k, sdk.StoreTypeIAVL, nil)
	}
	s.store.MountStoreWithDB(memKey, sdk.StoreTypeMemory, nil)
	require.NoError(t, s.store.LoadLatestVersion())
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, amino, paramsKey, memKey, name)
	}
	ak := authkeeper.NewAccountKeeper(cdc, authKey, subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{
		types.ModuleName: {authtypes.Minter, authtypes.Burner},
	})
	bk := bankkeeper.NewBaseKeeper(cdc, bankKey, ak, subspace(banktypes.ModuleName), nil)
	s.keeper = keeper.NewKeeper(cdc, key, subspace(types.ModuleName), ak, bk)
	s.server = keeper.NewMsgServerImpl(s.keeper)
	s.ctx = sdk.NewContext(s.store, tmproto.Header{}, false)
}

func (s *proofEventsState) reopen(t *testing.T) {
	t.Helper()
	s.store.Commit(true)
	require.NoError(t, s.store.Close())
	s.open(t)
	require.Empty(t, s.ctx.EventManager().Events())
}

func (s *proofEventsState) create(t *testing.T, subdenom string) string {
	t.Helper()
	msg := types.NewMsgCreateDenom(s.admin, subdenom)
	require.NoError(t, msg.ValidateBasic())
	res, err := s.server.CreateDenom(sdk.WrapSDKContext(s.ctx), msg)
	require.NoError(t, err)
	require.NotNil(t, res)
	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	return res.NewTokenDenom
}

func proofEventsResult(t *testing.T, ctx sdk.Context, response proto.Message) []byte {
	t.Helper()
	result, err := sdk.WrapServiceResult(ctx, response, nil)
	require.NoError(t, err)
	require.Len(t, result.Events, 1)
	wire, err := result.Marshal()
	require.NoError(t, err)
	return wire
}

func proofEventsObserve(t *testing.T, wire []byte, eventType string, attributes map[string]string) {
	t.Helper()
	for read := 0; read < 2; read++ {
		var result sdk.Result
		require.NoError(t, result.Unmarshal(wire))
		require.Len(t, result.Events, 1)
		log := sdk.ABCIMessageLogs{sdk.NewABCIMessageLog(0, "", result.GetEvents())}.String()
		require.NotEmpty(t, log)
		observed, err := sdk.ParseABCILogs(log)
		require.NoError(t, err)
		require.Len(t, observed, 1)
		require.Len(t, observed[0].Events, 1)
		event := observed[0].Events[0]
		require.Equal(t, eventType, event.Type)
		require.Len(t, event.Attributes, len(attributes))
		got := make(map[string]string, len(event.Attributes))
		for _, attr := range event.Attributes {
			_, duplicate := got[attr.Key]
			require.False(t, duplicate, "duplicate event attribute %s", attr.Key)
			got[attr.Key] = attr.Value
		}
		require.Equal(t, attributes, got)
		if paused, ok := got[types.AttributePaused]; ok {
			value, err := strconv.ParseBool(paused)
			require.NoError(t, err)
			require.Equal(t, strconv.FormatBool(value), paused)
		}
		_, err = sdk.ParseABCILogs(log[:len(log)-1])
		require.Error(t, err, "consumer must reject a truncated producer log")
		// A later read must come from the wire, not these decoded objects.
		observed[0].Events[0].Attributes[0].Value = "discarded"
		result.Events[0].Attributes[0].Value[0] ^= 0xff
	}
	var malformed sdk.Result
	require.Error(t, malformed.Unmarshal(wire[:len(wire)-1]), "consumer must reject a truncated producer result")
}

func TestProofEventsAdminPauseTransport(t *testing.T) {
	s := newProofEventsState(t)
	denom := s.create(t, "admin")
	for _, paused := range []bool{true, false} {
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		msg := types.NewMsgSetDenomPaused(s.admin, denom, paused)
		require.NoError(t, msg.ValidateBasic())
		require.Equal(t, "set_denom_paused", msg.Type())
		require.Equal(t, []sdk.AccAddress{sdk.MustAccAddressFromBech32(s.admin)}, msg.GetSigners())
		res, err := s.server.SetDenomPaused(sdk.WrapSDKContext(s.ctx), msg)
		require.NoError(t, err)
		require.NotNil(t, res)
		wire := proofEventsResult(t, s.ctx, res)
		s.reopen(t)
		require.Equal(t, paused, s.keeper.IsDenomPaused(s.ctx, denom))
		proofEventsObserve(t, wire, "set_denom_paused", map[string]string{
			"denom": denom, "paused": strconv.FormatBool(paused),
		})
		res, err = s.server.SetDenomPaused(sdk.WrapSDKContext(s.ctx), msg)
		require.ErrorIs(t, err, types.ErrPauseUnchanged)
		require.Nil(t, res)
		require.Empty(t, s.ctx.EventManager().Events())
	}
	other := sdk.AccAddress(bytes.Repeat([]byte{2}, 20)).String()
	res, err := s.server.SetDenomPaused(sdk.WrapSDKContext(s.ctx), types.NewMsgSetDenomPaused(other, denom, true))
	require.ErrorIs(t, err, types.ErrUnauthorized)
	require.Nil(t, res)
	require.False(t, s.keeper.IsDenomPaused(s.ctx, denom))
	require.Empty(t, s.ctx.EventManager().Events())
	missing, err := types.GetTokenDenom(s.admin, "missing")
	require.NoError(t, err)
	res, err = s.server.SetDenomPaused(sdk.WrapSDKContext(s.ctx), types.NewMsgSetDenomPaused(s.admin, missing, true))
	require.ErrorIs(t, err, types.ErrDenomDoesNotExist)
	require.Nil(t, res)
	require.Empty(t, s.ctx.EventManager().Events())
}

func TestProofEventsIssuerTransport(t *testing.T) {
	s := newProofEventsState(t)
	denom := s.create(t, "issuer")
	issuer := "did:layerx:" + strings.Repeat("ab", 32)
	require.ErrorIs(t, s.keeper.SetIssuerDid(s.ctx, denom, "malformed"), types.ErrInvalidAuthorityMetadata)
	require.Empty(t, s.ctx.EventManager().Events())
	require.NoError(t, s.keeper.SetIssuerDid(s.ctx, denom, issuer))
	wire := proofEventsResult(t, s.ctx, nil)
	s.reopen(t)
	storedIssuer, err := s.keeper.IssuerDid(s.ctx, denom)
	require.NoError(t, err)
	require.Equal(t, issuer, storedIssuer)
	proofEventsObserve(t, wire, "set_issuer_did", map[string]string{"denom": denom, "issuer_did": issuer})
	require.NoError(t, s.keeper.SetIssuerDid(s.ctx, denom, issuer))
	require.Empty(t, s.ctx.EventManager().Events())
	require.ErrorIs(t, s.keeper.SetIssuerDid(s.ctx, denom, "did:layerx:"+strings.Repeat("cd", 32)), types.ErrInvalidAuthorityMetadata)
	require.Empty(t, s.ctx.EventManager().Events())
	res, err := s.server.SetDenomPaused(sdk.WrapSDKContext(s.ctx), types.NewMsgSetDenomPaused(s.admin, denom, true))
	require.ErrorIs(t, err, types.ErrIssuerMismatch)
	require.Nil(t, res)
	require.Empty(t, s.ctx.EventManager().Events())
	for _, paused := range []bool{true, false} {
		require.ErrorIs(t, s.keeper.SetDenomPausedByIssuer(s.ctx, denom, "did:layerx:"+strings.Repeat("cd", 32), paused), types.ErrIssuerMismatch)
		require.Empty(t, s.ctx.EventManager().Events())
		require.NoError(t, s.keeper.SetDenomPausedByIssuer(s.ctx, denom, issuer, paused))
		wire := proofEventsResult(t, s.ctx, nil)
		s.reopen(t)
		require.Equal(t, paused, s.keeper.IsDenomPaused(s.ctx, denom))
		proofEventsObserve(t, wire, "set_denom_paused", map[string]string{
			"denom": denom, "paused": strconv.FormatBool(paused), "issuer_did": issuer,
		})
		require.ErrorIs(t, s.keeper.SetDenomPausedByIssuer(s.ctx, denom, issuer, paused), types.ErrPauseUnchanged)
		require.Empty(t, s.ctx.EventManager().Events())
	}
}

func TestProofEventsMalformedSignerRefused(t *testing.T) {
	s := newProofEventsState(t)
	denom := s.create(t, "renounced")
	_, err := s.server.ChangeAdmin(sdk.WrapSDKContext(s.ctx), types.NewMsgChangeAdmin(s.admin, denom, ""))
	require.NoError(t, err)
	s.reopen(t)
	metadata, err := s.keeper.GetAuthorityMetadata(s.ctx, denom)
	require.NoError(t, err)
	require.Empty(t, metadata.Admin)
	msg := types.NewMsgSetDenomPaused("", denom, true)
	encoded, err := msg.Marshal()
	require.NoError(t, err)
	var decoded types.MsgSetDenomPaused
	require.NoError(t, decoded.Unmarshal(encoded))
	require.ErrorIs(t, decoded.ValidateBasic(), sdkerrors.ErrInvalidAddress)
	res, err := s.server.SetDenomPaused(sdk.WrapSDKContext(s.ctx), &decoded)
	require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
	require.Nil(t, res)
	require.Empty(t, s.ctx.EventManager().Events())
	s.reopen(t)
	require.False(t, s.keeper.IsDenomPaused(s.ctx, denom))
}
