package evmrpc_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/fixtures/example/contracts/simplestorage"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func dialEthclientCallTest(t *testing.T) (*ethclient.Client, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	client, err := ethclient.DialContext(ctx, fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client, ctx
}

func TestEthclientCallContract(t *testing.T) {
	client, ctx := dialEthclientCallTest(t)

	creationHex, err := os.ReadFile("../tests/fixtures/example/contracts/simplestorage/SimpleStorage.bin")
	require.NoError(t, err)
	creation, err := hex.DecodeString(string(creationHex))
	require.NoError(t, err)
	code, _, _, err := runtime.Create(creation, &runtime.Config{GasLimit: 1_000_000})
	require.NoError(t, err)
	require.NotEmpty(t, code)

	_, contract := testkeeper.MockAddressPair()
	want := common.BigToHash(big.NewInt(42))
	EVMKeeper.SetCode(Ctx, contract, code)
	EVMKeeper.SetState(Ctx, contract, common.Hash{}, want)
	t.Cleanup(func() {
		EVMKeeper.SetState(Ctx, contract, common.Hash{}, common.Hash{})
		EVMKeeper.SetCode(Ctx, contract, nil)
	})

	abi, err := simplestorage.SimplestorageMetaData.GetAbi()
	require.NoError(t, err)
	input, err := abi.Pack("get")
	require.NoError(t, err)
	output, err := client.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: input}, nil)
	require.NoError(t, err)
	require.Equal(t, want.Bytes(), output)
	require.Equal(t, want, EVMKeeper.GetState(Ctx, contract, common.Hash{}))
}

func TestEthclientEstimateGas(t *testing.T) {
	client, ctx := dialEthclientCallTest(t)
	_, from := testkeeper.MockAddressPair()
	_, to := testkeeper.MockAddressPair()
	funds := sdk.NewCoins(sdk.NewCoin(EVMKeeper.GetBaseDenom(Ctx), sdk.NewInt(20)))
	require.NoError(t, EVMKeeper.BankKeeper().MintCoins(Ctx, types.ModuleName, funds))
	require.NoError(t, EVMKeeper.BankKeeper().SendCoinsFromModuleToAccount(Ctx, types.ModuleName, sdk.AccAddress(from[:]), funds))

	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  from,
		To:    &to,
		Value: big.NewInt(16),
	})
	require.NoError(t, err)
	require.Equal(t, uint64(21_000), gas)
}
