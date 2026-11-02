package ante

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type revocationFixture struct {
	ctx    sdk.Context
	keeper *keeper.Keeper
	evm    *evmkeeper.Keeper
	host   *lxphost.Host
	cdc    *codec.ProtoCodec
}

func newRevocationFixture(t *testing.T) revocationFixture {
	t.Helper()
	keys := sdk.NewKVStoreKeys("lxphost", types.StoreKey, evmtypes.StoreKey, authtypes.StoreKey, paramtypes.StoreKey)
	transient := sdk.NewTransientStoreKeys("transient_evm", paramtypes.TStoreKey)
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	for _, key := range transient {
		cms.MountStoreWithDB(key, sdk.StoreTypeTransient, nil)
	}
	require.NoError(t, cms.LoadLatestVersion())
	registry := cdctypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	authz.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), keys[paramtypes.StoreKey], transient[paramtypes.TStoreKey], name)
	}
	accounts := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, nil)
	evm := evmkeeper.NewKeeper(keys[evmtypes.StoreKey], transient["transient_evm"], subspace(evmtypes.ModuleName),
		nil, nil, &accounts, &stakingkeeper.Keeper{}, transferkeeper.Keeper{}, nil, nil, nil)
	host, err := lxphost.NewHost(lxphost.Config{
		StoreKey: keys["lxphost"], NetworkID: 7, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 1000000,
	})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	baseContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return revocationFixture{
		ctx:    sdk.NewContext(cms, tmproto.Header{ChainID: "hyperpax_125-1", Height: 11, Time: time.UnixMilli(1_800_000_000_000)}, false).WithContext(baseContext),
		keeper: keeper.NewKeeper(cdc, keys[types.StoreKey], host, nil, evm, ""),
		evm:    evm, host: host, cdc: cdc,
	}
}

func (f revocationFixture) bind(t *testing.T, address common.Address, seed byte, register bool) [32]byte {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(f.evm.ChainID(f.ctx), address, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = f.evm.BindLayerX(f.ctx, address, public, signature)
	require.NoError(t, err)
	if register {
		did := []byte(evmtypes.LayerXDid(public))
		id, err := lxphost.DIDID(did)
		require.NoError(t, err)
		payload := append(append([]byte{0x71, 1, 0, 2}, id[:]...), public[:]...)
		timestamp := uint64(f.ctx.BlockTime().UnixMilli())
		activity := lxphost.Activity{
			ProtocolVersion: 3, Type: 0x00070001, ActorDID: did, AuthorityKey: public,
			NotBefore: timestamp - 1000, NotAfter: timestamp + 1000,
			IdempotencyKey: public, Payload: payload,
		}
		hash, err := f.host.SigningHash(activity)
		require.NoError(t, err)
		activity.Signature = ed25519.Sign(private, hash[:])
		receipt, err := f.host.Execute(lxphost.WithSequence(f.ctx, 41), activity, lxphost.KernelAuthority{
			Actor: id, Principal: id, VerifiedKey: public,
		})
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), receipt.ResultCode)
	}
	return public
}

func (f revocationFixture) tx(t *testing.T, msgs ...sdk.Msg) sdk.Tx {
	t.Helper()
	builder := authtx.NewTxConfig(f.cdc, authtx.DefaultSignModes).NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msgs...))
	return builder.GetTx()
}

func revocationAddress(seed byte) sdk.AccAddress {
	return sdk.AccAddress(bytes.Repeat([]byte{seed}, 20))
}

func revocationSend(grantor sdk.AccAddress) *banktypes.MsgSend {
	return banktypes.NewMsgSend(grantor, revocationAddress(0x99), sdk.NewCoins(sdk.NewInt64Coin("upax", 1)))
}

func TestRevocationDecoratorSetsGrantorSequence(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(map[bool]string{false: "multiple_execs", true: "nested_exec"}[nested], func(t *testing.T) {
			f := newRevocationFixture(t)
			grantor, grantee := revocationAddress(0x31), revocationAddress(0x32)
			evmAddress := common.BytesToAddress(bytes.Repeat([]byte{0x91}, 20))
			f.evm.SetAddressMapping(f.ctx, grantor, evmAddress)
			public := f.bind(t, evmAddress, 71, true)
			sequence, err := f.keeper.RevocationSequence(f.ctx, public)
			require.NoError(t, err)
			require.Equal(t, uint64(41), sequence)
			send := revocationSend(grantor)
			exec := authz.NewMsgExec(grantee, []sdk.Msg{send, revocationSend(grantor)})
			if nested {
				inner := exec
				exec = authz.NewMsgExec(revocationAddress(0x33), []sdk.Msg{&inner})
			}
			tx := f.tx(t, revocationSend(grantee), &exec, &exec)
			authorization := types.LayerXScopeAuthorization{MsgTypeUrl: sdk.MsgTypeURL(send), RevocationEpoch: 41}
			before, err := authorization.Accept(f.ctx, send)
			require.ErrorIs(t, err, types.ErrAuthRevoked)
			require.False(t, before.Accept)
			calls := 0
			ctx, err := NewRevocationDecorator(f.keeper, f.evm).AnteHandle(f.ctx, tx, nested,
				func(ctx sdk.Context, got sdk.Tx, simulate bool) (sdk.Context, error) {
					calls++
					require.Same(t, tx, got)
					require.Equal(t, nested, simulate)
					accepted, err := authorization.Accept(ctx, send)
					require.NoError(t, err)
					require.True(t, accepted.Accept)
					stale := authorization
					stale.RevocationEpoch = 40
					refused, err := stale.Accept(ctx, send)
					require.ErrorIs(t, err, types.ErrAuthRevoked)
					require.False(t, refused.Accept)
					_, err = authorization.Accept(ctx, revocationSend(grantee))
					require.ErrorIs(t, err, types.ErrAuthRevoked)
					return ctx.WithPriority(17), nil
				})
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			require.Equal(t, int64(17), ctx.Priority())
			_, err = authorization.Accept(f.ctx, send)
			require.ErrorIs(t, err, types.ErrAuthRevoked)
		})
	}
	t.Run("identity_read_error", func(t *testing.T) {
		f := newRevocationFixture(t)
		grantor := revocationAddress(0x41)
		address := common.BytesToAddress(bytes.Repeat([]byte{0xa1}, 20))
		f.evm.SetAddressMapping(f.ctx, grantor, address)
		f.bind(t, address, 72, false)
		exec := authz.NewMsgExec(revocationAddress(0x42), []sdk.Msg{revocationSend(grantor)})
		called := false
		_, err := NewRevocationDecorator(f.keeper, f.evm).AnteHandle(f.ctx, f.tx(t, &exec), false,
			func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
				called = true
				return ctx, nil
			})
		require.Equal(t, lxphost.Code(-200), err)
		require.False(t, called)
	})
}

func TestRevocationDecoratorSkipsUnbound(t *testing.T) {
	for _, scenario := range []string{"no_association", "no_did", "ordinary_message"} {
		t.Run(scenario, func(t *testing.T) {
			f := newRevocationFixture(t)
			grantor := revocationAddress(0x51)
			address := common.BytesToAddress(grantor)
			if scenario != "no_association" {
				f.evm.SetAddressMapping(f.ctx, grantor, address)
			}
			if scenario != "no_did" {
				f.bind(t, address, 73, true)
			}
			send := revocationSend(grantor)
			var msg sdk.Msg = send
			if scenario != "ordinary_message" {
				exec := authz.NewMsgExec(revocationAddress(0x52), []sdk.Msg{send})
				msg = &exec
			}
			tx := f.tx(t, msg)
			calls := 0
			ctx, err := NewRevocationDecorator(f.keeper, f.evm).AnteHandle(f.ctx, tx, true,
				func(ctx sdk.Context, got sdk.Tx, simulate bool) (sdk.Context, error) {
					calls++
					require.Same(t, f.ctx.Context(), ctx.Context())
					require.Same(t, f.ctx.MultiStore(), ctx.MultiStore())
					require.Same(t, tx, got)
					require.True(t, simulate)
					authorization := types.LayerXScopeAuthorization{MsgTypeUrl: sdk.MsgTypeURL(send), RevocationEpoch: 41}
					accepted, err := authorization.Accept(ctx, send)
					require.ErrorIs(t, err, types.ErrAuthRevoked)
					require.False(t, accepted.Accept)
					return ctx, nil
				})
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			require.Same(t, f.ctx.Context(), ctx.Context())
		})
	}
}
