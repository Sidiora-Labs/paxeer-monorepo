package types_test

import (
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM659Codec() *codec.ProtoCodec {
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	return codec.NewProtoCodec(registry)
}

func repairM659GenesisRoundTrip(t *testing.T, cdc *codec.ProtoCodec, acc types.GenesisAccount) types.GenesisState {
	t.Helper()
	genesis := types.NewGenesisState(types.DefaultParams(), types.GenesisAccounts{acc})
	bz, err := cdc.MarshalAsJSON(genesis)
	require.NoError(t, err)
	var decoded types.GenesisState
	require.NoError(t, cdc.UnmarshalAsJSON(bz, &decoded))
	accounts, err := types.UnpackAccounts(decoded.Accounts)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	decodedBz, err := cdc.MarshalAsJSON(&decoded)
	require.NoError(t, err)
	require.JSONEq(t, string(bz), string(decodedBz))
	require.Equal(t, acc, accounts[0])
	return decoded
}

func TestRepairM_659_InvalidNilKeyAddresses(t *testing.T) {
	pubkey := secp256k1.GenPrivKey().PubKey()
	addr := sdk.AccAddress(pubkey.Address())
	valid := addr.String()
	badChecksum := valid[:len(valid)-1] + "q"
	if badChecksum == valid {
		badChecksum = valid[:len(valid)-1] + "p"
	}

	for _, tc := range []struct {
		name    string
		address string
	}{
		{"empty", ""},
		{"whitespace", "   "},
		{"malformed", "not-bech32"},
		{"checksum", badChecksum},
		{"validator_prefix", sdk.ValAddress(addr).String()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			acc := types.NewBaseAccountWithAddress(addr)
			acc.Address = tc.address
			require.Nil(t, acc.PubKey)
			require.Nil(t, acc.GetPubKey())
			_, parseErr := sdk.AccAddressFromBech32(tc.address)
			require.Error(t, parseErr)
			require.EqualError(t, acc.Validate(), parseErr.Error())

			genesis := repairM659GenesisRoundTrip(t, repairM659Codec(), acc)
			require.ErrorContains(t, types.ValidateGenesis(genesis), "invalid account found in genesis state")
			require.ErrorContains(t, types.ValidateGenesis(genesis), parseErr.Error())
		})
	}
}

func TestRepairM_659_KeyAddressAgreement(t *testing.T) {
	pubkey := secp256k1.GenPrivKey().PubKey()
	addr := sdk.AccAddress(pubkey.Address())
	otherKey := secp256k1.GenPrivKey().PubKey()
	for _, tc := range []struct {
		name string
		acc  *types.BaseAccount
		err  string
	}{
		{"unset_key", types.NewBaseAccount(addr, nil, 7, 3), ""},
		{"matching_key", types.NewBaseAccount(addr, pubkey, 7, 3), ""},
		{"mismatched_key", types.NewBaseAccount(addr, otherKey, 7, 3), "account address and pubkey address do not match"},
		{"empty_address_with_key", types.NewBaseAccount(nil, pubkey, 7, 3), "empty address string is not allowed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			genesis := repairM659GenesisRoundTrip(t, repairM659Codec(), tc.acc)
			if tc.err == "" {
				require.NoError(t, tc.acc.Validate())
				require.NoError(t, types.ValidateGenesis(genesis))
			} else {
				require.EqualError(t, tc.acc.Validate(), tc.err)
				require.ErrorContains(t, types.ValidateGenesis(genesis), tc.err)
			}
		})
	}
}

func TestRepairM_659_FundedAccountWithoutKey(t *testing.T) {
	cdc := repairM659Codec()
	cms := store.NewCommitMultiStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	authKey := sdk.NewKVStoreKey(types.StoreKey)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	for _, key := range []*sdk.KVStoreKey{authKey, bankKey, paramsKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	}
	cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1}, false)
	amino := codec.NewLegacyAmino()
	authParams := paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, types.ModuleName)
	bankParams := paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, banktypes.ModuleName)
	ak := authkeeper.NewAccountKeeper(cdc, authKey, authParams, types.ProtoBaseAccount, map[string][]string{})
	bk := bankkeeper.NewBaseKeeper(cdc, bankKey, ak, bankParams, map[string]bool{})
	ak.SetParams(ctx, types.DefaultParams())

	addr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	acc := ak.NewAccountWithAddress(ctx, addr)
	ak.SetAccount(ctx, acc)
	coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 123))
	bankGenesis := banktypes.DefaultGenesisState()
	bankGenesis.Balances = []banktypes.Balance{{Address: addr.String(), Coins: coins}}
	bankGenesis.Supply = coins
	require.NoError(t, bankGenesis.Validate())
	bk.InitGenesis(ctx, bankGenesis)
	require.Equal(t, coins, bk.GetAllBalances(ctx, addr))

	stored := ak.GetAccount(ctx, addr)
	require.NotNil(t, stored)
	require.Equal(t, addr, stored.GetAddress())
	require.Nil(t, stored.GetPubKey())
	genesisAccount, ok := stored.(types.GenesisAccount)
	require.True(t, ok)
	require.NoError(t, genesisAccount.Validate())
	genesis := repairM659GenesisRoundTrip(t, cdc, genesisAccount)
	require.NoError(t, types.ValidateGenesis(genesis))
	decoded, err := types.UnpackAccounts(genesis.Accounts)
	require.NoError(t, err)
	require.Nil(t, decoded[0].GetPubKey())
	require.Nil(t, decoded[0].(*types.BaseAccount).PubKey)
	require.Equal(t, coins, bk.GetAllBalances(ctx, decoded[0].GetAddress()))
}
