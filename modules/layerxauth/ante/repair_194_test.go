package ante_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	layerxante "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/ante"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type repair194Fixture struct {
	app       *app.App
	ctx       sdk.Context
	grantors  [2]sdk.AccAddress
	grantee   sdk.AccAddress
	recipient sdk.AccAddress
	actors    [2]keeper.Actor
	grants    [2][32]byte
}

func newRepair194Fixture(t *testing.T) repair194Fixture {
	t.Helper()
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx, _ := a.NewContext(false, tmproto.Header{
		ChainID: "hyperpax_125-1", Height: 11, Time: time.Unix(1_800_000_000, 0).UTC(),
	}).CacheContext()
	f := repair194Fixture{app: a, ctx: ctx,
		grantors: [2]sdk.AccAddress{bytes.Repeat([]byte{0x81}, 20), bytes.Repeat([]byte{0x82}, 20)},
		grantee:  bytes.Repeat([]byte{0x83}, 20), recipient: bytes.Repeat([]byte{0x84}, 20)}
	var delegate [32]byte
	for i, address := range []sdk.AccAddress{f.grantors[0], f.grantors[1], f.grantee} {
		private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(81 + i)}, ed25519.SeedSize))
		var public [32]byte
		copy(public[:], private.Public().(ed25519.PublicKey))
		evmAddress := common.BytesToAddress(address)
		a.EvmKeeper.SetAddressMapping(ctx, address, evmAddress)
		message, err := types.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), evmAddress, 0)
		require.NoError(t, err)
		var signature [64]byte
		copy(signature[:], ed25519.Sign(private, message))
		_, err = a.EvmKeeper.BindLayerX(ctx, evmAddress, public, signature)
		require.NoError(t, err)
		actor, err := a.LayerXAuthKeeper.Actor(ctx, evmAddress)
		require.NoError(t, err)
		timestamp := uint64(ctx.BlockTime().UnixMilli())
		registration := append(append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...), public[:]...)
		receipt, err := a.LayerXHost.Execute(lxphost.WithSequence(ctx, uint64(41+10*i)), lxphost.Activity{
			ProtocolVersion: 3, Type: 0x00070001, ActorDID: actor.DID, AuthorityKey: public,
			NotBefore: timestamp, NotAfter: timestamp, IdempotencyKey: public, Payload: registration,
		}, actor.Authority)
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), receipt.ResultCode)
		if i < 2 {
			f.actors[i] = actor
		} else {
			delegate = public
		}
	}
	for i, grantor := range f.grantors {
		require.NoError(t, apptesting.FundAccount(a.BankKeeper, ctx, grantor,
			sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 100))))
		sequence, err := a.LayerXAuthKeeper.RevocationSequence(ctx, f.actors[i].DIDKey)
		require.NoError(t, err)
		require.Equal(t, uint64(41+10*i), sequence)
		body := repair194GrantBody(f.actors[i].Authority.Actor, delegate, uint64(ctx.BlockTime().UnixMilli()), sequence)
		result, err := a.LayerXAuthKeeper.GrantCapability(ctx, f.actors[i], body, [32]byte{0x94, byte(i + 1)})
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), result.ResultCode)
		stored, _ := a.AuthzKeeper.GetCleanAuthorization(ctx, f.grantee, grantor, sdk.MsgTypeURL(f.send(grantor)))
		require.IsType(t, &authtypes.LayerXScopeAuthorization{}, stored)
		scope := stored.(*authtypes.LayerXScopeAuthorization)
		require.Equal(t, sequence, scope.RevocationEpoch)
		copy(f.grants[i][:], scope.GrantId)
		issued, err := a.LayerXHost.ReadGrant(ctx, f.grants[i])
		require.NoError(t, err)
		require.Equal(t, issued.GrantorRevocationSequence, scope.RevocationEpoch)
		require.Equal(t, issued.GrantID[:], scope.GrantId)
	}
	return f
}

func repair194GrantBody(actor, delegate [32]byte, timestamp, epoch uint64) []byte {
	body := []byte{0, 1, 0x20, 1, 1}
	field := func(value []byte) {
		body = binary.BigEndian.AppendUint32(body, uint32(len(value)))
		body = append(body, value...)
	}
	u64 := func(value uint64) { body = binary.BigEndian.AppendUint64(body, value) }
	u128 := func(value uint64) { u64(0); u64(value) }
	field(actor[:])
	field(actor[:])
	body = append(body, 3)
	field(delegate[:])
	u64(1 << 1)
	body = binary.BigEndian.AppendUint16(body, 4)
	body = binary.BigEndian.AppendUint16(body, 5)
	field(bytes.Repeat([]byte{0x11}, 32))
	u128(10)
	u128(20)
	u128(0)
	u64(1000)
	u128(15)
	u128(0)
	u64(timestamp - 1000)
	field(bytes.Repeat([]byte{0x22}, 32))
	u64(timestamp - 1000)
	u64(timestamp + 1000)
	u64(epoch)
	body = append(body, 0)
	u64(0)
	field(make([]byte, ed25519.SignatureSize))
	return body
}

func (f repair194Fixture) send(grantor sdk.AccAddress) *banktypes.MsgSend {
	return banktypes.NewMsgSend(grantor, f.recipient, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1)))
}

func (f repair194Fixture) deliver(t *testing.T, ctx sdk.Context, messages ...sdk.Msg) (sdk.Context, error) {
	t.Helper()
	encoding := app.MakeEncodingConfig()
	builder := encoding.TxConfig.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(messages...))
	wire, err := encoding.TxConfig.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	decoded, err := encoding.TxConfig.TxDecoder()(wire)
	require.NoError(t, err)
	branch, write := ctx.WithEventManager(sdk.NewEventManager()).CacheContext()
	decorator := layerxante.NewRevocationDecorator(f.app.LayerXAuthKeeper, &f.app.EvmKeeper)
	anteCtx, err := sdk.ChainAnteDecorators(decorator)(branch, decoded, false)
	if err != nil {
		return anteCtx, err
	}
	for _, message := range decoded.GetMsgs() {
		exec := message.(*authz.MsgExec)
		response, err := f.app.AuthzKeeper.Exec(sdk.WrapSDKContext(anteCtx), exec)
		if err != nil {
			return anteCtx, err
		}
		require.Len(t, response.Results, len(exec.Msgs))
	}
	write()
	return anteCtx, nil
}

func (f repair194Fixture) assertBalance(t *testing.T, address sdk.AccAddress, amount int64) {
	t.Helper()
	require.Equal(t, sdk.NewInt(amount), f.app.BankKeeper.GetBalance(f.ctx, address, sdk.DefaultBondDenom).Amount)
}

func TestRepair_194_MixedGrantors(t *testing.T) {
	f := newRepair194Fixture(t)
	for _, order := range [][2]int{{0, 1}, {1, 0}} {
		exec := authz.NewMsgExec(f.grantee, []sdk.Msg{f.send(f.grantors[order[0]]), f.send(f.grantors[order[1]])})
		_, err := f.deliver(t, f.ctx, &exec)
		require.NoError(t, err)
	}
	first := authz.NewMsgExec(f.grantee, []sdk.Msg{f.send(f.grantors[0])})
	second := authz.NewMsgExec(f.grantee, []sdk.Msg{f.send(f.grantors[1])})
	_, err := f.deliver(t, f.ctx, &first, &second)
	require.NoError(t, err)
	f.assertBalance(t, f.grantors[0], 97)
	f.assertBalance(t, f.grantors[1], 97)
	f.assertBalance(t, f.recipient, 6)
}

func TestRepair_194_MixedBoundUnbound(t *testing.T) {
	f := newRepair194Fixture(t)
	for i, associated := range []bool{false, true} {
		unbound := sdk.AccAddress(bytes.Repeat([]byte{byte(0x85 + i)}, 20))
		if associated {
			f.app.EvmKeeper.SetAddressMapping(f.ctx, unbound, common.BytesToAddress(unbound))
		}
		require.NoError(t, apptesting.FundAccount(f.app.BankKeeper, f.ctx, unbound,
			sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10))))
		grant, err := authz.NewMsgGrant(unbound, f.grantee, authz.NewGenericAuthorization(sdk.MsgTypeURL(f.send(unbound))), f.ctx.BlockTime().Add(time.Hour))
		require.NoError(t, err)
		_, err = f.app.AuthzKeeper.Grant(sdk.WrapSDKContext(f.ctx), grant)
		require.NoError(t, err)
		exec := authz.NewMsgExec(f.grantee, []sdk.Msg{f.send(unbound), f.send(f.grantors[0]), f.send(unbound), f.send(f.grantors[1]), f.send(unbound)})
		ctx, err := f.deliver(t, f.ctx, &exec)
		require.NoError(t, err)
		stored, _ := f.app.AuthzKeeper.GetCleanAuthorization(f.ctx, f.grantee, f.grantors[0], sdk.MsgTypeURL(f.send(unbound)))
		response, err := stored.Accept(ctx, f.send(unbound))
		require.ErrorIs(t, err, authtypes.ErrAuthRevoked)
		require.False(t, response.Accept)
		f.assertBalance(t, unbound, 7)
	}
	f.assertBalance(t, f.grantors[0], 98)
	f.assertBalance(t, f.grantors[1], 98)
	f.assertBalance(t, f.recipient, 10)
}

func TestRepair_194_NestedMsgExec(t *testing.T) {
	f := newRepair194Fixture(t)
	inner := authz.NewMsgExec(f.grantee, []sdk.Msg{f.send(f.grantors[1]), f.send(f.grantors[0])})
	middle := authz.NewMsgExec(f.grantee, []sdk.Msg{&inner, f.send(f.grantors[1])})
	outer := authz.NewMsgExec(f.grantee, []sdk.Msg{f.send(f.grantors[0]), &middle, f.send(f.grantors[1])})
	_, err := f.deliver(t, f.ctx, &outer)
	require.NoError(t, err)
	f.assertBalance(t, f.grantors[0], 98)
	f.assertBalance(t, f.grantors[1], 97)
	f.assertBalance(t, f.recipient, 5)
}

func TestRepair_194_DetachedContextsAndRevocation(t *testing.T) {
	f := newRepair194Fixture(t)
	decorator := layerxante.NewRevocationDecorator(f.app.LayerXAuthKeeper, &f.app.EvmKeeper)
	parent, err := decorator.WithGrantorRevocation(f.ctx, f.grantors[0])
	require.NoError(t, err)
	child, err := decorator.WithGrantorRevocation(parent, f.grantors[1])
	require.NoError(t, err)
	stored := make([]authz.Authorization, 2)
	for i, grantor := range f.grantors {
		stored[i], _ = f.app.AuthzKeeper.GetCleanAuthorization(f.ctx, f.grantee, grantor, sdk.MsgTypeURL(f.send(grantor)))
		accepted, err := stored[i].Accept(child, f.send(grantor))
		require.NoError(t, err)
		require.True(t, accepted.Accept)
		_, err = stored[i].Accept(f.ctx, f.send(grantor))
		require.ErrorIs(t, err, authtypes.ErrAuthRevoked)
	}
	_, err = stored[1].Accept(parent, f.send(f.grantors[1]))
	require.ErrorIs(t, err, authtypes.ErrAuthRevoked)
	branch, _ := child.CacheContext()
	sequence, err := lxphost.NextSequence(branch, f.app.GetKey(app.LayerXHostStoreKey))
	require.NoError(t, err)
	require.Greater(t, sequence, uint64(51))
	result, err := f.app.LayerXAuthKeeper.RevokeGrant(branch, f.actors[0], f.grants[0], 1, sequence, [32]byte{0x94, 3})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), result.ResultCode)
	require.Equal(t, sequence, result.GlobalSequence)
	refreshed, err := decorator.WithGrantorRevocation(branch, f.grantors[0])
	require.NoError(t, err)
	_, err = stored[0].Accept(refreshed, f.send(f.grantors[0]))
	require.ErrorIs(t, err, authtypes.ErrAuthRevoked)
	require.ErrorContains(t, err, fmt.Sprintf("revocation epoch 41, grantor revocation %d", sequence))
	for _, previous := range []sdk.Context{parent, child} {
		accepted, err := stored[0].Accept(previous, f.send(f.grantors[0]))
		require.NoError(t, err)
		require.True(t, accepted.Accept)
	}
	live := authz.NewMsgExec(f.grantee, []sdk.Msg{f.send(f.grantors[1])})
	_, err = f.deliver(t, branch, &live)
	require.NoError(t, err)
	for _, order := range [][2]int{{0, 1}, {1, 0}} {
		exec := authz.NewMsgExec(f.grantee, []sdk.Msg{f.send(f.grantors[order[0]]), f.send(f.grantors[order[1]])})
		_, err = f.deliver(t, branch, &exec)
		require.ErrorIs(t, err, authtypes.ErrAuthRevoked)
	}
	require.Equal(t, sdk.NewInt(100), f.app.BankKeeper.GetBalance(branch, f.grantors[0], sdk.DefaultBondDenom).Amount)
	require.Equal(t, sdk.NewInt(99), f.app.BankKeeper.GetBalance(branch, f.grantors[1], sdk.DefaultBondDenom).Amount)
	require.Equal(t, sdk.NewInt(1), f.app.BankKeeper.GetBalance(branch, f.recipient, sdk.DefaultBondDenom).Amount)
	f.assertBalance(t, f.recipient, 0)
}
