package simulation

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	paxappparams "github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	simtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/simulation"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/ante"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestRepairM_667_ZeroAccounts(t *testing.T) {
	repairM667Operation(t, 0, nil, 0)
}

func TestRepairM_667_OneAccount(t *testing.T) {
	repairM667Operation(t, 1, []int{0}, 0)
}

func TestRepairM_667_TwoAccounts(t *testing.T) {
	repairM667Operation(t, 2, []int{0, 1}, 2)
}

func TestRepairM_667_DuplicateOnlyAccount(t *testing.T) {
	repairM667Operation(t, 1, []int{0, 0, 0}, 0)
}

func TestRepairM_667_TwoDistinctAccountsWithDuplicates(t *testing.T) {
	repairM667Operation(t, 2, []int{0, 1, 0, 1}, 2)
}

func TestRepairM_667_SampleWithoutReplacement(t *testing.T) {
	repairM667Operation(t, 5, []int{0, 1, 2, 3, 4}, 3)
}

// Seed 1 requests three inputs in the production operation. All cases drive
// simulation -> signed protobuf transaction -> BaseApp -> bank MsgServer/store.
func repairM667Operation(t *testing.T, accountCount int, indices []int, wantInputs int) {
	t.Helper()
	const seed = int64(1)
	require.Equal(t, 3, rand.New(rand.NewSource(seed)).Intn(3)+1)

	encoding := paxappparams.MakeEncodingConfig()
	std.RegisterLegacyAminoCodec(encoding.Amino)
	std.RegisterInterfaces(encoding.InterfaceRegistry)
	authtypes.RegisterLegacyAminoCodec(encoding.Amino)
	authtypes.RegisterInterfaces(encoding.InterfaceRegistry)
	banktypes.RegisterLegacyAminoCodec(encoding.Amino)
	banktypes.RegisterInterfaces(encoding.InterfaceRegistry)

	app := baseapp.NewBaseApp(t.Name(), dbm.NewMemDB(), encoding.TxConfig.TxDecoder(), nil, &testutil.TestAppOpts{})
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	keys := sdk.NewKVStoreKeys(authtypes.StoreKey, banktypes.StoreKey, paramtypes.StoreKey)
	tkeys := sdk.NewTransientStoreKeys(paramtypes.TStoreKey)
	app.MountKVStores(keys)
	app.MountTransientStores(tkeys)
	pk := paramskeeper.NewKeeper(encoding.Marshaler, encoding.Amino, keys[paramtypes.StoreKey], tkeys[paramtypes.TStoreKey])
	ak := authkeeper.NewAccountKeeper(encoding.Marshaler, keys[authtypes.StoreKey], pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{
		"repair-minter": {authtypes.Minter}, authtypes.FeeCollectorName: nil,
	})
	bk := bankkeeper.NewBaseKeeper(encoding.Marshaler, keys[banktypes.StoreKey], ak, pk.Subspace(banktypes.ModuleName), nil)
	app.SetInterfaceRegistry(encoding.InterfaceRegistry)
	banktypes.RegisterMsgServer(app.MsgServiceRouter(), bankkeeper.NewMsgServerImpl(bk))
	anteHandler, err := ante.NewAnteHandler(ante.HandlerOptions{
		AccountKeeper: ak, BankKeeper: bk, ParamsKeeper: pk,
		SignModeHandler: encoding.TxConfig.SignModeHandler(), SigGasConsumer: ante.DefaultSigVerificationGasConsumer,
	})
	require.NoError(t, err)
	app.SetAnteHandler(anteHandler)
	require.NoError(t, app.LoadLatestVersion())
	_, err = app.InitChain(context.Background(), &abci.RequestInitChain{ChainId: "test-chain"})
	require.NoError(t, err)
	ctx := app.DeliverContext().WithBlockHeight(1)
	*app.DeliverContext() = ctx
	ak.SetParams(ctx, authtypes.DefaultParams())
	bk.SetParams(ctx, banktypes.DefaultParams())
	ak.GetModuleAccount(ctx, authtypes.FeeCollectorName)

	// A non-fee denomination makes exact transfer balances observable while the
	// real ante handler still checks signatures and advances signer sequences.
	const denom = "repaircoin"
	initial := sdk.NewInt(1_000_000)
	accounts := make([]simtypes.Account, accountCount)
	for i := range accounts {
		priv := secp256k1.GenPrivKeyFromSecret([]byte(fmt.Sprintf("repair-m667-%d", i)))
		accounts[i] = simtypes.Account{Address: sdk.AccAddress(priv.PubKey().Address()), PrivKey: priv, PubKey: priv.PubKey()}
		acc := ak.NewAccountWithAddress(ctx, accounts[i].Address)
		ak.SetAccount(ctx, acc)
		coins := sdk.NewCoins(sdk.NewCoin(denom, initial))
		require.NoError(t, bk.MintCoins(ctx, "repair-minter", coins))
		require.NoError(t, bk.SendCoinsFromModuleToAccount(ctx, "repair-minter", accounts[i].Address, coins))
	}
	selected := make([]simtypes.Account, len(indices))
	for i, index := range indices {
		selected[i] = accounts[index]
	}
	original := append([]simtypes.Account{}, selected...)

	type result struct {
		msg    simtypes.OperationMsg
		future []simtypes.FutureOperation
		err    error
	}
	done := make(chan result, 1)
	go func() {
		msg, future, err := SimulateMsgMultiSend(ak, bk)(rand.New(rand.NewSource(seed)), app, ctx, selected, "test-chain")
		done <- result{msg, future, err}
	}()
	var got result
	select {
	case got = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("multisend account selection did not terminate")
	}
	require.NoError(t, got.err)
	require.Empty(t, got.future)
	require.Equal(t, banktypes.ModuleName, got.msg.Route)
	require.Equal(t, banktypes.TypeMsgMultiSend, got.msg.Name)
	require.Equal(t, original, selected, "operation must not reorder caller accounts")
	if wantInputs == 0 {
		require.False(t, got.msg.OK)
		require.Equal(t, "fewer than two distinct accounts", got.msg.Comment)
		require.Empty(t, got.msg.Msg)
		for _, acc := range accounts {
			require.True(t, bk.GetBalance(ctx, acc.Address, denom).Amount.Equal(initial))
			require.Zero(t, ak.GetAccount(ctx, acc.Address).GetSequence())
		}
		return
	}
	require.True(t, got.msg.OK, got.msg.Comment)
	require.Empty(t, got.msg.Comment)
	var msg banktypes.MsgMultiSend
	require.NoError(t, banktypes.ModuleCdc.UnmarshalAsJSON(got.msg.Msg, &msg))
	require.NoError(t, msg.ValidateBasic())
	require.Len(t, msg.Inputs, wantInputs)
	seen := make(map[string]bool)
	expected := make(map[string]sdk.Int, len(accounts))
	for _, acc := range accounts {
		expected[acc.Address.String()] = initial
	}
	for _, input := range msg.Inputs {
		require.False(t, seen[input.Address], "duplicate signer")
		seen[input.Address] = true
		require.Contains(t, expected, input.Address)
		expected[input.Address] = expected[input.Address].Sub(input.Coins.AmountOf(denom))
	}
	for _, output := range msg.Outputs {
		require.Contains(t, expected, output.Address)
		expected[output.Address] = expected[output.Address].Add(output.Coins.AmountOf(denom))
	}
	for _, acc := range accounts {
		require.True(t, bk.GetBalance(ctx, acc.Address, denom).Amount.Equal(expected[acc.Address.String()]), "delivered balance for %s", acc.Address)
		wantSequence := uint64(0)
		if seen[acc.Address.String()] {
			wantSequence = 1
		}
		require.Equal(t, wantSequence, ak.GetAccount(ctx, acc.Address).GetSequence(), "signed input sequence")
	}
	require.True(t, bk.GetSupply(ctx, denom).Amount.Equal(initial.MulRaw(int64(accountCount))))
}
