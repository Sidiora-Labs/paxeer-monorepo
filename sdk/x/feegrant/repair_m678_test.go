package feegrant_test

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/feegrant"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/feegrant/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repairM678Env struct {
	app     *baseapp.BaseApp
	k       keeper.Keeper
	ctx     sdk.Context
	cdc     *codec.ProtoCodec
	key     *sdk.KVStoreKey
	granter sdk.AccAddress
	grantee sdk.AccAddress
}

func repairM678Setup(t *testing.T) repairM678Env {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	app := baseapp.NewBaseApp(t.Name(), db, nil, nil, testutil.TestAppOpts{})
	feeKey := sdk.NewKVStoreKey(feegrant.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	app.MountStores(feeKey, authKey, paramsKey, transientKey)
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	feegrant.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	app.SetInterfaceRegistry(registry)
	params := paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), paramsKey, transientKey, authtypes.ModuleName)
	ak := authkeeper.NewAccountKeeper(cdc, authKey, params, authtypes.ProtoBaseAccount, nil)
	k := keeper.NewKeeper(cdc, feeKey, ak)
	feegrant.RegisterMsgServer(app.MsgServiceRouter(), keeper.NewMsgServerImpl(k))
	require.NoError(t, app.LoadLatestVersion())
	ctx := sdk.NewContext(app.CommitMultiStore(), tmproto.Header{Time: time.Unix(1700000000, 0).UTC()}, false)
	return repairM678Env{app: app, k: k, ctx: ctx, cdc: cdc, key: feeKey,
		granter: sdk.AccAddress(bytes.Repeat([]byte{1}, 20)),
		grantee: sdk.AccAddress(bytes.Repeat([]byte{2}, 20))}
}

func (e repairM678Env) grantMessage(t *testing.T, allowance feegrant.FeeAllowanceI) *feegrant.MsgGrantAllowance {
	t.Helper()
	msg, err := feegrant.NewMsgGrantAllowance(allowance, e.granter, e.grantee)
	require.NoError(t, err)
	wire, err := e.cdc.Marshal(msg)
	require.NoError(t, err)
	var decoded feegrant.MsgGrantAllowance
	require.NoError(t, e.cdc.Unmarshal(wire, &decoded))
	return &decoded
}

func (e repairM678Env) send() *banktypes.MsgSend {
	return banktypes.NewMsgSend(e.grantee, e.granter, sdk.NewCoins(sdk.NewInt64Coin("atom", 1)))
}

func repairM678Events(t *testing.T, events []abci.Event, granter, grantee sdk.AccAddress, types ...string) {
	t.Helper()
	require.Len(t, events, len(types))
	for i, event := range events {
		require.Equal(t, types[i], event.Type)
		require.Len(t, event.Attributes, 2)
		attributes := make(map[string]string)
		for _, attr := range event.Attributes {
			attributes[string(attr.Key)] = string(attr.Value)
		}
		require.Equal(t, map[string]string{"granter": granter.String(), "grantee": grantee.String()}, attributes)
	}
}

func TestRepairM_678_FeegrantErrorAssertions(t *testing.T) {
	for _, periodic := range []bool{false, true} {
		for _, wrapped := range []bool{false, true} {
			for _, limited := range []bool{false, true} {
				t.Run(fmt.Sprintf("periodic_%t_wrapped_%t_limited_%t", periodic, wrapped, limited), func(t *testing.T) {
					e := repairM678Setup(t)
					expiration := e.ctx.BlockTime().Add(time.Hour)
					basic := feegrant.BasicAllowance{Expiration: &expiration}
					if limited {
						basic.SpendLimit = sdk.NewCoins(sdk.NewInt64Coin("atom", 10))
					}
					var allowance feegrant.FeeAllowanceI = &basic
					if periodic {
						allowance = &feegrant.PeriodicAllowance{Basic: basic, Period: time.Hour,
							PeriodSpendLimit: sdk.NewCoins(sdk.NewInt64Coin("atom", 10)),
							PeriodCanSpend:   sdk.NewCoins(sdk.NewInt64Coin("atom", 10)), PeriodReset: expiration}
					}
					if wrapped {
						var err error
						allowance, err = feegrant.NewAllowedMsgAllowance(allowance, []string{sdk.MsgTypeURL(e.send())})
						require.NoError(t, err)
					}
					_, err := e.app.RunMsgs(e.ctx, []sdk.Msg{e.grantMessage(t, allowance)})
					require.NoError(t, err)
					key := feegrant.FeeAllowanceKey(e.granter, e.grantee)
					before := append([]byte(nil), e.ctx.KVStore(e.key).Get(key)...)
					msgs := []sdk.Msg{e.send()}
					fee := sdk.NewCoins(sdk.NewInt64Coin("atom", 1))
					stored, err := e.k.GetAllowance(e.ctx, e.granter, e.grantee)
					require.NoError(t, err)
					if limited || periodic {
						remove, err := stored.Accept(e.ctx, sdk.NewCoins(sdk.NewInt64Coin("atom", 11)), msgs)
						require.ErrorIs(t, err, feegrant.ErrFeeLimitExceeded)
						require.False(t, remove)
						require.ErrorIs(t, e.k.UseGrantedFees(e.ctx, e.granter, e.grantee, sdk.NewCoins(sdk.NewInt64Coin("atom", 11)), msgs), feegrant.ErrFeeLimitExceeded)
						require.Equal(t, before, e.ctx.KVStore(e.key).Get(key))
					}
					e.ctx = e.ctx.WithBlockTime(expiration)
					require.NoError(t, e.k.UseGrantedFees(e.ctx, e.granter, e.grantee, fee, msgs), "expiration is exclusive")
					stored, err = e.k.GetAllowance(e.ctx, e.granter, e.grantee)
					require.NoError(t, err)
					e.ctx = e.ctx.WithBlockTime(expiration.Add(time.Nanosecond)).WithEventManager(sdk.NewEventManager())
					remove, err := stored.Accept(e.ctx, fee, msgs)
					require.ErrorIs(t, err, feegrant.ErrFeeLimitExpired)
					require.True(t, remove, "removal is independent of rejection")
					require.ErrorIs(t, e.k.UseGrantedFees(e.ctx, e.granter, e.grantee, fee, msgs), feegrant.ErrFeeLimitExpired)
					require.False(t, e.ctx.KVStore(e.key).Has(key))
					repairM678Events(t, e.ctx.EventManager().ABCIEvents(), e.granter, e.grantee, "revoke_feegrant")
					_, err = e.k.GetAllowance(e.ctx, e.granter, e.grantee)
					require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
				})
			}
		}
	}
}

func TestRepairM_678_ZeroPeriod(t *testing.T) {
	for _, period := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond, time.Hour} {
		for _, wrapped := range []bool{false, true} {
			t.Run(fmt.Sprintf("period_%s_wrapped_%t", period, wrapped), func(t *testing.T) {
				e := repairM678Setup(t)
				budget := sdk.NewCoins(sdk.NewInt64Coin("atom", 2))
				var allowance feegrant.FeeAllowanceI = &feegrant.PeriodicAllowance{
					Basic:  feegrant.BasicAllowance{SpendLimit: sdk.NewCoins(sdk.NewInt64Coin("atom", 10))},
					Period: period, PeriodSpendLimit: budget, PeriodCanSpend: budget, PeriodReset: e.ctx.BlockTime()}
				if wrapped {
					var err error
					allowance, err = feegrant.NewAllowedMsgAllowance(allowance, []string{sdk.MsgTypeURL(e.send())})
					require.NoError(t, err)
				}
				decoded := e.grantMessage(t, allowance)
				decodedAllowance, err := decoded.GetFeeAllowanceI()
				require.NoError(t, err)
				key := feegrant.FeeAllowanceKey(e.granter, e.grantee)
				msgs := []sdk.Msg{e.send()}
				if period <= 0 {
					require.ErrorIs(t, decodedAllowance.ValidateBasic(), feegrant.ErrInvalidDuration)
					require.ErrorIs(t, decoded.ValidateBasic(), feegrant.ErrInvalidDuration)
					result, err := e.app.RunMsgs(e.ctx, []sdk.Msg{decoded})
					require.ErrorIs(t, err, feegrant.ErrInvalidDuration)
					require.Nil(t, result)
					require.False(t, e.ctx.KVStore(e.key).Has(key))
					// The keeper serializes a legacy grant that bypassed message validation.
					require.NoError(t, e.k.GrantAllowance(e.ctx, e.granter, e.grantee, decodedAllowance))
					before := append([]byte(nil), e.ctx.KVStore(e.key).Get(key)...)
					for i := 0; i < 2; i++ {
						e.ctx = e.ctx.WithEventManager(sdk.NewEventManager())
						stored, err := e.k.GetAllowance(e.ctx, e.granter, e.grantee)
						require.NoError(t, err)
						remove, err := stored.Accept(e.ctx, budget, msgs)
						require.ErrorIs(t, err, feegrant.ErrInvalidDuration)
						require.False(t, remove)
						require.ErrorIs(t, e.k.UseGrantedFees(e.ctx, e.granter, e.grantee, budget, msgs), feegrant.ErrInvalidDuration)
						require.Equal(t, before, e.ctx.KVStore(e.key).Get(key), "invalid period cannot reset or spend")
						require.Empty(t, e.ctx.EventManager().Events())
						e.ctx = e.ctx.WithBlockTime(e.ctx.BlockTime().Add(time.Hour))
					}
					return
				}
				require.NoError(t, decoded.ValidateBasic())
				_, err = e.app.RunMsgs(e.ctx, []sdk.Msg{decoded})
				require.NoError(t, err)
				require.NoError(t, e.k.UseGrantedFees(e.ctx, e.granter, e.grantee, budget, msgs))
				before := append([]byte(nil), e.ctx.KVStore(e.key).Get(key)...)
				require.ErrorIs(t, e.k.UseGrantedFees(e.ctx, e.granter, e.grantee, budget, msgs), feegrant.ErrFeeLimitExceeded)
				require.Equal(t, before, e.ctx.KVStore(e.key).Get(key))
				e.ctx = e.ctx.WithBlockTime(e.ctx.BlockTime().Add(period))
				require.NoError(t, e.k.UseGrantedFees(e.ctx, e.granter, e.grantee, budget, msgs))
				stored, err := e.k.GetAllowance(e.ctx, e.granter, e.grantee)
				require.NoError(t, err)
				if wrapped {
					stored, err = stored.(*feegrant.AllowedMsgAllowance).GetAllowance()
					require.NoError(t, err)
				}
				periodic := stored.(*feegrant.PeriodicAllowance)
				require.True(t, periodic.PeriodCanSpend.IsZero())
				require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("atom", 6)), periodic.Basic.SpendLimit)
				require.Equal(t, e.ctx.BlockTime().Add(period), periodic.PeriodReset)
			})
		}
	}
}

func TestRepairM_678_FeegrantTypesDoc(t *testing.T) {
	doc, err := os.ReadFile("spec/01_concepts.md")
	require.NoError(t, err)
	for _, text := range []string{"BasicAllowance", "PeriodicAllowance", "AllowedMsgAllowance", "Any", "message type URLs", "strictly positive", "removal flag", "even on error"} {
		require.Contains(t, string(doc), text)
	}
	for _, kind := range []string{"BasicAllowance", "PeriodicAllowance", "AllowedMsgAllowance"} {
		t.Run(kind, func(t *testing.T) {
			e := repairM678Setup(t)
			limit := sdk.NewCoins(sdk.NewInt64Coin("atom", 3))
			var allowance feegrant.FeeAllowanceI = &feegrant.BasicAllowance{SpendLimit: limit}
			if kind == "PeriodicAllowance" {
				allowance = &feegrant.PeriodicAllowance{Period: time.Hour, PeriodSpendLimit: limit,
					PeriodCanSpend: limit, PeriodReset: e.ctx.BlockTime().Add(time.Hour)}
			} else if kind == "AllowedMsgAllowance" {
				allowance, err = feegrant.NewAllowedMsgAllowance(allowance, []string{sdk.MsgTypeURL(e.send())})
				require.NoError(t, err)
			}
			decoded := e.grantMessage(t, allowance)
			require.Equal(t, "/cosmos.feegrant.v1beta1."+kind, decoded.Allowance.TypeUrl)
			require.NoError(t, decoded.ValidateBasic())
			_, err := e.app.RunMsgs(e.ctx, []sdk.Msg{decoded})
			require.NoError(t, err)
			if kind == "AllowedMsgAllowance" {
				key := feegrant.FeeAllowanceKey(e.granter, e.grantee)
				before := append([]byte(nil), e.ctx.KVStore(e.key).Get(key)...)
				// A different bank message is not included by allowing MsgSend.
				other := banktypes.NewMsgMultiSend([]banktypes.Input{banktypes.NewInput(e.grantee, limit)}, []banktypes.Output{banktypes.NewOutput(e.granter, limit)})
				require.NoError(t, other.ValidateBasic())
				err = e.k.UseGrantedFees(e.ctx, e.granter, e.grantee, limit, []sdk.Msg{e.send(), other})
				require.ErrorIs(t, err, feegrant.ErrMessageNotAllowed)
				require.Equal(t, before, e.ctx.KVStore(e.key).Get(key))
				stored, err := e.k.GetAllowance(e.ctx, e.granter, e.grantee)
				require.NoError(t, err)
				filtered := stored.(*feegrant.AllowedMsgAllowance)
				require.Equal(t, "/cosmos.feegrant.v1beta1.BasicAllowance", filtered.Allowance.TypeUrl)
				require.Equal(t, []string{"/cosmos.bank.v1beta1.MsgSend"}, filtered.AllowedMessages)
			}
			require.NoError(t, e.k.UseGrantedFees(e.ctx, e.granter, e.grantee, limit, []sdk.Msg{e.send()}))
			_, err = e.k.GetAllowance(e.ctx, e.granter, e.grantee)
			if kind == "PeriodicAllowance" {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
			}
		})
	}
}

func TestRepairM_678_FeegrantEventsDoc(t *testing.T) {
	e := repairM678Setup(t)
	doc, err := os.ReadFile("spec/04_events.md")
	require.NoError(t, err)
	grant := e.grantMessage(t, &feegrant.BasicAllowance{SpendLimit: sdk.NewCoins(sdk.NewInt64Coin("atom", 3))})
	revokeMsg := feegrant.NewMsgRevokeAllowance(e.granter, e.grantee)
	wire, err := e.cdc.Marshal(&revokeMsg)
	require.NoError(t, err)
	var revoke feegrant.MsgRevokeAllowance
	require.NoError(t, e.cdc.Unmarshal(wire, &revoke))
	for _, msg := range []interface {
		sdk.Msg
		Type() string
	}{grant, &revoke, e.send()} {
		require.Contains(t, string(doc), sdk.MsgTypeURL(msg))
		require.Contains(t, string(doc), msg.Type())
	}
	for _, msg := range []interface {
		sdk.Msg
		Type() string
	}{grant, &revoke} {
		require.Equal(t, sdk.MsgTypeURL(msg), msg.Type(), "feegrant legacy actions are full type URLs")
	}
	require.Equal(t, "send", e.send().Type())
	for _, event := range []string{"set_feegrant", "revoke_feegrant", "use_feegrant"} {
		require.Contains(t, string(doc), event)
	}
	result, err := e.app.RunMsgs(e.ctx, []sdk.Msg{grant})
	require.NoError(t, err)
	require.Len(t, result.Events, 2)
	require.Equal(t, "message", result.Events[0].Type)
	require.Len(t, result.Events[0].Attributes, 1)
	require.Equal(t, "action", string(result.Events[0].Attributes[0].Key))
	require.Equal(t, sdk.MsgTypeURL(grant), string(result.Events[0].Attributes[0].Value))
	repairM678Events(t, result.Events[1:], e.granter, e.grantee, "set_feegrant")

	e.ctx = e.ctx.WithEventManager(sdk.NewEventManager())
	require.NoError(t, e.k.UseGrantedFees(e.ctx, e.granter, e.grantee, sdk.NewCoins(sdk.NewInt64Coin("atom", 1)), []sdk.Msg{e.send()}))
	repairM678Events(t, e.ctx.EventManager().ABCIEvents(), e.granter, e.grantee, "use_feegrant", "set_feegrant")
	result, err = e.app.RunMsgs(e.ctx, []sdk.Msg{&revoke})
	require.NoError(t, err)
	require.Len(t, result.Events, 2)
	require.Equal(t, "message", result.Events[0].Type)
	require.Len(t, result.Events[0].Attributes, 1)
	require.Equal(t, "action", string(result.Events[0].Attributes[0].Key))
	require.Equal(t, sdk.MsgTypeURL(&revoke), string(result.Events[0].Attributes[0].Value))
	repairM678Events(t, result.Events[1:], e.granter, e.grantee, "revoke_feegrant")
	_, err = e.app.RunMsgs(e.ctx, []sdk.Msg{grant})
	require.NoError(t, err)
	e.ctx = e.ctx.WithEventManager(sdk.NewEventManager())
	require.NoError(t, e.k.UseGrantedFees(e.ctx, e.granter, e.grantee, sdk.NewCoins(sdk.NewInt64Coin("atom", 3)), []sdk.Msg{e.send()}))
	repairM678Events(t, e.ctx.EventManager().ABCIEvents(), e.granter, e.grantee, "revoke_feegrant", "use_feegrant")
	_, err = e.k.GetAllowance(e.ctx, e.granter, e.grantee)
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
}
