package types_test

import (
	"encoding/json"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/types"
)

func TestRepairM_818_RejectInvalidPoolConfiguration(t *testing.T) {
	accounts := types.GenerateAccounts(2)
	cases := []struct {
		name string
		cfg  *types.AccountConfig
		want string
	}{
		{"nil config", nil, "configuration must not be nil"},
		{"zero config", &types.AccountConfig{}, "fixed accounts must be nonempty"},
		{"empty fixed pool", &types.AccountConfig{Accounts: []*types.Account{}}, "fixed accounts must be nonempty"},
		{"empty mixed pool", &types.AccountConfig{NewAccountRate: 0.5}, "fixed accounts must be nonempty"},
		{"almost fresh only", &types.AccountConfig{NewAccountRate: math.Nextafter(1, 0)}, "exactly 1"},
		{"negative rate", &types.AccountConfig{Accounts: accounts, NewAccountRate: -0.1}, "rate must be finite and between 0 and 1"},
		{"above one", &types.AccountConfig{Accounts: accounts, NewAccountRate: math.Nextafter(1, 2)}, "rate must be finite and between 0 and 1"},
		{"NaN rate", &types.AccountConfig{Accounts: accounts, NewAccountRate: math.NaN()}, "rate must be finite and between 0 and 1"},
		{"positive infinity", &types.AccountConfig{Accounts: accounts, NewAccountRate: math.Inf(1)}, "rate must be finite and between 0 and 1"},
		{"negative infinity", &types.AccountConfig{Accounts: accounts, NewAccountRate: math.Inf(-1)}, "rate must be finite and between 0 and 1"},
		{"nil fixed account", &types.AccountConfig{Accounts: []*types.Account{accounts[0], nil}}, "fixed account at index 1 must not be nil"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool, err := types.NewAccountPool(tc.cfg)
			require.ErrorContains(t, err, tc.want)
			require.Nil(t, pool, "invalid configurations must not reach account selection")
		})
	}
}

func TestRepairM_818_AdmittedPoolTransactionRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		profile string
	}{
		{"fresh only without fixed accounts", `{"chainId":7777,"accounts":{"count":0,"newAccountRate":1}}`},
		{"fixed accounts", `{"chainId":7777,"accounts":{"count":2,"newAccountRate":0}}`},
		{"mixed accounts", `{"chainId":7777,"accounts":{"count":2,"newAccountRate":0.5}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var profile config.LoadConfig
			require.NoError(t, json.Unmarshal([]byte(tc.profile), &profile))
			require.NotNil(t, profile.Accounts)
			accounts := types.GenerateAccounts(profile.Accounts.Accounts)
			pool, err := types.NewAccountPool(&types.AccountConfig{
				Accounts: accounts, NewAccountRate: profile.Accounts.NewAccountRate,
			})
			require.NoError(t, err)
			require.NotNil(t, pool)
			seen := make(map[common.Address]bool)
			for i := 0; i < 12; i++ {
				sender := pool.NextAccount()
				receiver := pool.NextAccount()
				require.NotNil(t, sender)
				require.NotNil(t, receiver)
				require.NotNil(t, sender.PrivKey)
				require.Equal(t, crypto.PubkeyToAddress(sender.PrivKey.PublicKey), sender.Address)
				if profile.Accounts.NewAccountRate == 1 {
					require.False(t, seen[sender.Address], "fresh-only selections must be distinct")
					seen[sender.Address] = true
					require.False(t, seen[receiver.Address], "fresh-only selections must be distinct")
					seen[receiver.Address] = true
				}
				if profile.Accounts.NewAccountRate == 0 {
					require.Same(t, accounts[1], sender)
					require.Same(t, accounts[0], receiver)
				}
				nonce := sender.GetAndIncrementNonce()
				signer := ethtypes.LatestSignerForChainID(profile.GetChainID())
				tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{
					ChainID: profile.GetChainID(), Nonce: nonce,
					GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2), Gas: 21000,
					To: &receiver.Address, Value: big.NewInt(7),
				}), signer, sender.PrivKey)
				require.NoError(t, err)
				loadTx := types.CreateTxFromEthTx(tx, &types.TxScenario{
					Name: tc.name, Sender: sender, Receiver: receiver.Address,
				})
				var request types.JSONRPCRequest
				require.NoError(t, json.Unmarshal(loadTx.JSONRPCPayload, &request))
				require.Equal(t, "2.0", request.Version)
				require.Equal(t, "eth_sendRawTransaction", request.Method)
				var params []string
				require.NoError(t, json.Unmarshal(request.Params, &params))
				require.Len(t, params, 1)
				raw, err := hexutil.Decode(params[0])
				require.NoError(t, err)
				require.Equal(t, loadTx.Payload, raw)
				var decoded ethtypes.Transaction
				require.NoError(t, decoded.UnmarshalBinary(raw))
				recoveredSender, err := ethtypes.Sender(signer, &decoded)
				require.NoError(t, err)
				require.Equal(t, sender.Address, recoveredSender)
				require.Equal(t, receiver.Address, *decoded.To())
				require.Equal(t, nonce, decoded.Nonce())
				require.Equal(t, tx.Hash(), decoded.Hash())
			}
		})
	}
}

func TestRepairM_818_AdmissionSurvivesConfigMutation(t *testing.T) {
	accounts := types.GenerateAccounts(2)
	expected := append([]*types.Account(nil), accounts...)
	cfg := &types.AccountConfig{Accounts: accounts}
	pool, err := types.NewAccountPool(cfg)
	require.NoError(t, err)
	accounts[0], accounts[1] = nil, nil
	cfg.Accounts = nil
	cfg.NewAccountRate = 1
	for i := 0; i < 6; i++ {
		require.Same(t, expected[(i+1)%len(expected)], pool.NextAccount())
	}

	cfg = &types.AccountConfig{NewAccountRate: 1}
	pool, err = types.NewAccountPool(cfg)
	require.NoError(t, err)
	cfg.NewAccountRate = 0
	account := pool.NextAccount()
	require.NotNil(t, account)
	require.NotNil(t, account.PrivKey)
	require.Equal(t, crypto.PubkeyToAddress(account.PrivKey.PublicKey), account.Address)
}
