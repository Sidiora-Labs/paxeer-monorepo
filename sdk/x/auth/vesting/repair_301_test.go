package vesting_test

import (
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	vestingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/stretchr/testify/require"
)

func repair301Dispatch(t *testing.T, a *app.App, ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
	t.Helper()
	txConfig := app.MakeEncodingConfig().TxConfig
	builder := txConfig.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	bz, err := txConfig.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	tx, err := txConfig.TxDecoder()(bz)
	require.NoError(t, err)
	require.Len(t, tx.GetMsgs(), 1)
	decoded := tx.GetMsgs()[0]
	handler := a.MsgServiceRouter().Handler(decoded)
	require.NotNil(t, handler)
	return handler(ctx, decoded)
}

func repair301StoreSnapshot(store sdk.KVStore) map[string]string {
	state := make(map[string]string)
	it := store.Iterator(nil, nil)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		state[string(it.Key())] = string(it.Value())
	}
	return state
}

func repair301Forbidden(t *testing.T, forbiddenSender bool) {
	t.Helper()
	a := app.Setup(t, false, false, false)
	root := a.NewContext(false, tmproto.Header{Height: a.LastBlockHeight() + 1, Time: time.Unix(1700000000, 0)})
	from := sdk.AccAddress([]byte("repair301_sender____"))
	to := sdk.AccAddress([]byte("repair301_recipient_"))
	denom := "factory/" + from.String() + "/restricted"
	otherDenom := "factory/" + from.String() + "/other"
	amount := sdk.NewCoins(sdk.NewInt64Coin(denom, 50), sdk.NewInt64Coin(otherDenom, 25), sdk.NewInt64Coin("test", 10))
	for _, delayed := range []bool{true, false} {
		name := "continuous"
		if delayed {
			name = "delayed"
		}
		t.Run(name, func(t *testing.T) {
			ctx, _ := root.CacheContext()
			a.AccountKeeper.SetAccount(ctx, a.AccountKeeper.NewAccountWithAddress(ctx, from))
			require.NoError(t, apptesting.FundAccount(a.BankKeeper, ctx, from, amount))
			allowed := from
			if forbiddenSender {
				allowed = to
			}
			a.BankKeeper.SetDenomAllowList(ctx, denom, banktypes.AllowList{Addresses: []string{allowed.String()}})
			a.BankKeeper.SetDenomAllowList(ctx, otherDenom, banktypes.AllowList{Addresses: []string{from.String(), to.String()}})
			require.Nil(t, a.AccountKeeper.GetAccount(ctx, to))
			accounts := repair301StoreSnapshot(ctx.KVStore(a.GetKey(authtypes.StoreKey)))
			bank := repair301StoreSnapshot(ctx.KVStore(a.GetKey(banktypes.StoreKey)))

			bankResult, bankErr := repair301Dispatch(t, a, ctx, banktypes.NewMsgSend(from, to, amount))
			require.ErrorIs(t, bankErr, sdkerrors.ErrUnauthorized)
			require.Nil(t, bankResult)
			msg := vestingtypes.NewMsgCreateVestingAccount(from, to, amount, ctx.BlockTime().Unix()+10000, delayed, nil)
			result, err := repair301Dispatch(t, a, ctx, msg)
			require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
			require.EqualError(t, err, bankErr.Error())
			require.Nil(t, result)
			require.Nil(t, a.AccountKeeper.GetAccount(ctx, to))
			require.Equal(t, amount, a.BankKeeper.GetAllBalances(ctx, from))
			require.True(t, a.BankKeeper.GetAllBalances(ctx, to).IsZero())
			require.Equal(t, accounts, repair301StoreSnapshot(ctx.KVStore(a.GetKey(authtypes.StoreKey))), "rejection must not allocate an account number or mutate any account")
			require.Equal(t, bank, repair301StoreSnapshot(ctx.KVStore(a.GetKey(banktypes.StoreKey))), "rejection must not mutate balances, supply or allowlists")
		})
	}
}

func TestRepair_301_ForbiddenSender(t *testing.T) {
	repair301Forbidden(t, true)
}

func TestRepair_301_ForbiddenRecipient(t *testing.T) {
	repair301Forbidden(t, false)
}

func TestRepair_301_AuthorizedTransfers(t *testing.T) {
	a := app.Setup(t, false, false, false)
	root := a.NewContext(false, tmproto.Header{Height: a.LastBlockHeight() + 1, Time: time.Unix(1700000000, 0)})
	from := sdk.AccAddress([]byte("repair301_sender____"))
	to := sdk.AccAddress([]byte("repair301_recipient_"))
	for _, policy := range []string{"restricted", "empty_allowlist", "native"} {
		for _, delayed := range []bool{true, false} {
			name := policy + "/continuous"
			if delayed {
				name = policy + "/delayed"
			}
			t.Run(name, func(t *testing.T) {
				ctx, _ := root.CacheContext()
				denom := "factory/" + from.String() + "/token"
				if policy == "native" {
					denom = "test"
				}
				amount := sdk.NewCoins(sdk.NewInt64Coin(denom, 100))
				a.AccountKeeper.SetAccount(ctx, a.AccountKeeper.NewAccountWithAddress(ctx, from))
				require.NoError(t, apptesting.FundAccount(a.BankKeeper, ctx, from, amount))
				if policy == "restricted" {
					a.BankKeeper.SetDenomAllowList(ctx, denom, banktypes.AllowList{Addresses: []string{from.String(), to.String()}})
				}
				msg := vestingtypes.NewMsgCreateVestingAccount(from, to, amount, ctx.BlockTime().Unix()+10000, delayed, nil)
				result, err := repair301Dispatch(t, a, ctx, msg)
				require.NoError(t, err)
				require.NotNil(t, result)
				account := a.AccountKeeper.GetAccount(ctx, to)
				if delayed {
					require.IsType(t, &vestingtypes.DelayedVestingAccount{}, account)
					require.Equal(t, amount, account.(*vestingtypes.DelayedVestingAccount).GetVestingCoins(ctx.BlockTime()))
				} else {
					require.IsType(t, &vestingtypes.ContinuousVestingAccount{}, account)
					require.Equal(t, amount, account.(*vestingtypes.ContinuousVestingAccount).GetVestingCoins(ctx.BlockTime()))
				}
				require.Equal(t, amount, a.BankKeeper.GetAllBalances(ctx, to))
				require.True(t, a.BankKeeper.GetAllBalances(ctx, from).IsZero())
				require.True(t, a.BankKeeper.SpendableCoins(ctx, to).IsZero())
			})
		}
	}
}

func TestRepair_301_ModuleTransferPermissions(t *testing.T) {
	a := app.Setup(t, false, false, false)
	ctx := a.NewContext(false, tmproto.Header{Height: a.LastBlockHeight() + 1, Time: time.Unix(1700000000, 0)})
	to := sdk.AccAddress([]byte("repair301_recipient_"))
	allowed := sdk.AccAddress([]byte("repair301_allowed___"))
	denom := "factory/" + allowed.String() + "/restricted"
	amount := sdk.NewCoins(sdk.NewInt64Coin(denom, 100))
	a.BankKeeper.SetDenomAllowList(ctx, denom, banktypes.AllowList{Addresses: []string{allowed.String()}})
	a.AccountKeeper.SetAccount(ctx, a.AccountKeeper.NewAccountWithAddress(ctx, to))
	// Funding uses the real mint module's module-to-account transfer to an unlisted recipient.
	require.NoError(t, apptesting.FundAccount(a.BankKeeper, ctx, to, amount))
	require.Equal(t, amount, a.BankKeeper.GetAllBalances(ctx, to))
}
