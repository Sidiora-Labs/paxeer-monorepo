package simapp

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmed25519 "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp/helpers"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1278_ReadmeBondDenom(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	require.NoError(t, err)
	fundingExample := regexp.MustCompile(`(?s)\./simd add-genesis-account\s+.*?\s+(\d+[a-z]+) --home`).FindSubmatch(readme)
	gentxExample := regexp.MustCompile(`\./simd gentx validator (\d+[a-z]+)`).FindSubmatch(readme)
	require.Len(t, fundingExample, 2)
	require.Len(t, gentxExample, 2)
	funding, err := sdk.ParseCoinNormalized(string(fundingExample[1]))
	require.NoError(t, err)
	delegation, err := sdk.ParseCoinNormalized(string(gentxExample[1]))
	require.NoError(t, err)

	app, genesisState := setup(true, 0)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	cdc, txCfg := app.AppCodec(), app.GetTxConfig()
	var stakingGenesis stakingtypes.GenesisState
	cdc.MustUnmarshalJSON(genesisState[stakingtypes.ModuleName], &stakingGenesis)
	require.Equal(t, "uhpx", stakingGenesis.Params.BondDenom)
	require.Equal(t, stakingGenesis.Params.BondDenom, funding.Denom)
	require.Equal(t, funding.Denom, delegation.Denom)
	require.True(t, funding.Amount.GTE(delegation.Amount))

	priv := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(priv.PubKey().Address())
	account := authtypes.NewBaseAccount(addr, priv.PubKey(), 0, 0)
	genesisState[authtypes.ModuleName] = cdc.MustMarshalJSON(authtypes.NewGenesisState(
		authtypes.DefaultParams(), []authtypes.GenesisAccount{account},
	))
	bankGenesis := banktypes.DefaultGenesisState()
	bankGenesis.Balances = []banktypes.Balance{{Address: addr.String(), Coins: sdk.NewCoins(funding)}}
	genesisState[banktypes.ModuleName] = cdc.MustMarshalJSON(bankGenesis)
	require.NoError(t, genutil.ValidateAccountInGenesis(genesisState, banktypes.GenesisBalancesIterator{}, addr, sdk.NewCoins(delegation), cdc))

	msg, err := stakingtypes.NewMsgCreateValidator(
		sdk.ValAddress(addr), ed25519.GenPrivKey().PubKey(), delegation,
		stakingtypes.NewDescription("validator-a", "", "", "", ""),
		stakingtypes.NewCommissionRates(sdk.NewDecWithPrec(1, 1), sdk.NewDecWithPrec(2, 1), sdk.NewDecWithPrec(1, 2)),
		sdk.OneInt(),
	)
	require.NoError(t, err)
	require.NoError(t, msg.ValidateBasic())
	const chainID = "ibc-local-1"
	tx, err := helpers.GenTx(txCfg, []sdk.Msg{msg}, sdk.NewCoins(), helpers.DefaultGenTxGas, chainID, []uint64{0}, []uint64{0}, priv)
	require.NoError(t, err)
	genesisState, err = genutil.SetGenTxsInAppGenesisState(cdc, txCfg.TxJSONEncoder(), genesisState, []sdk.Tx{tx})
	require.NoError(t, err)
	stateBytes, err := json.Marshal(genesisState)
	require.NoError(t, err)
	response, err := app.InitChain(t.Context(), &abci.RequestInitChain{
		ChainId: chainID, Time: time.Unix(1700000000, 0).UTC(),
		ConsensusParams: DefaultConsensusParams, AppStateBytes: stateBytes,
	})
	require.NoError(t, err)
	require.Len(t, response.Validators, 1)
	require.Equal(t, delegation.Amount.Quo(sdk.DefaultPowerReduction).Int64(), response.Validators[0].Power)
	app.SetProcessProposalStateToCommit()
	_, err = app.Commit(t.Context())
	require.NoError(t, err)
	ctx := app.NewContext(true, tmproto.Header{ChainID: chainID})
	require.Equal(t, funding.Sub(delegation), app.BankKeeper.GetBalance(ctx, addr, funding.Denom))
	validator, found := app.StakingKeeper.GetValidator(ctx, sdk.ValAddress(addr))
	require.True(t, found)
	require.True(t, validator.IsBonded())
	require.Equal(t, delegation.Amount, validator.Tokens)
	selfDelegation, found := app.StakingKeeper.GetDelegation(ctx, addr, sdk.ValAddress(addr))
	require.True(t, found)
	require.Equal(t, delegation.Amount.ToDec(), selfDelegation.Shares)
}

func TestRepairM_1278_DeliveryErrorSurvivesCommit(t *testing.T) {
	const chainID = "test-chain"
	priv := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(priv.PubKey().Address())
	recipient := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	account := authtypes.NewBaseAccount(addr, priv.PubKey(), 0, 0)
	val := tmtypes.NewValidator(tmed25519.GenerateSecretKey().Public(), 1)
	funding := sdk.NewInt64Coin(sdk.DefaultBondDenom, 1000000000)
	app := SetupWithGenesisValSet(t, tmtypes.NewValidatorSet([]*tmtypes.Validator{val}),
		[]authtypes.GenesisAccount{account}, chainID, sdk.DefaultPowerReduction,
		banktypes.Balance{Address: addr.String(), Coins: sdk.NewCoins(funding)},
	)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	header := app.GetContextForDeliverTx(nil).BlockHeader()
	beforeCommit := app.LastCommitID()
	msg := banktypes.NewMsgSend(addr, recipient, sdk.NewCoins(funding.AddAmount(sdk.OneInt())))
	gas, result, deliveryErr := SignAndDeliver(t, app.GetTxConfig(), app.BaseApp, app.IBCKeeper,
		app.StakingKeeper, app.CapabilityKeeper, &app.DistrKeeper, &app.SlashingKeeper, &app.EvidenceKeeper,
		header, []sdk.Msg{msg}, chainID, []uint64{account.GetAccountNumber()}, []uint64{0}, true, false, priv,
	)
	require.ErrorIs(t, deliveryErr, sdkerrors.ErrInsufficientFunds)
	require.Nil(t, result)
	require.Positive(t, gas.GasUsed)
	require.Equal(t, beforeCommit.Version+1, app.LastCommitID().Version)
	require.NotEmpty(t, app.LastCommitID().Hash)
	ctx := app.NewContext(true, header)
	require.Equal(t, funding, app.BankKeeper.GetBalance(ctx, addr, funding.Denom))
	require.True(t, app.BankKeeper.GetBalance(ctx, recipient, funding.Denom).IsZero())
	sender := app.AccountKeeper.GetAccount(ctx, addr)
	require.Equal(t, uint64(1), sender.GetSequence())

	header.Height = app.LastBlockHeight() + 1
	_, err := app.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{Header: &header})
	require.NoError(t, err)
	amount := sdk.NewInt64Coin(funding.Denom, 100)
	msg = banktypes.NewMsgSend(addr, recipient, sdk.NewCoins(amount))
	gas, result, deliveryErr = SignAndDeliver(t, app.GetTxConfig(), app.BaseApp, app.IBCKeeper,
		app.StakingKeeper, app.CapabilityKeeper, &app.DistrKeeper, &app.SlashingKeeper, &app.EvidenceKeeper,
		header, []sdk.Msg{msg}, chainID, []uint64{sender.GetAccountNumber()}, []uint64{sender.GetSequence()}, true, true, priv,
	)
	require.NoError(t, deliveryErr)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Events)
	require.Positive(t, gas.GasUsed)
	require.Equal(t, beforeCommit.Version+2, app.LastCommitID().Version)
	ctx = app.NewContext(true, header)
	require.Equal(t, amount, app.BankKeeper.GetBalance(ctx, recipient, funding.Denom))
	require.Equal(t, funding.Sub(amount), app.BankKeeper.GetBalance(ctx, addr, funding.Denom))
	require.Equal(t, uint64(2), app.AccountKeeper.GetAccount(ctx, addr).GetSequence())
}
