package evmrpc_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func TestEthclientBalanceAndNonceAt(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client, err := ethclient.DialContext(ctx, fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	t.Cleanup(client.Close)

	account := common.HexToAddress("0x1234567890123456789023456789012345678901")
	originalNonce := EVMKeeper.GetNonce(Ctx, account)
	t.Cleanup(func() { EVMKeeper.SetNonce(Ctx, account, originalNonce) })
	EVMKeeper.SetNonce(Ctx, account, 7)

	wantBalance := EVMKeeper.GetBalance(Ctx, EVMKeeper.GetPaxAddressOrDefault(Ctx, account))
	require.Positive(t, wantBalance.Sign(), "the test account must be funded")
	wantNonce := EVMKeeper.GetNonce(Ctx, account)
	require.Equal(t, uint64(7), wantNonce)

	balance, err := client.BalanceAt(ctx, account, nil)
	require.NoError(t, err)
	require.NotNil(t, balance)
	require.Zero(t, wantBalance.Cmp(balance), "BalanceAt must return the keeper balance")

	nonce, err := client.NonceAt(ctx, account, nil)
	require.NoError(t, err)
	require.Equal(t, wantNonce, nonce, "NonceAt must return the keeper nonce")
}

func TestEthclientCodeAndStorageAt(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client, err := ethclient.DialContext(ctx, fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	t.Cleanup(client.Close)

	contract := common.HexToAddress("0x1234567890123456789023456789012345678901")
	slot := common.Hash{}
	originalState := EVMKeeper.GetState(Ctx, contract, slot)
	t.Cleanup(func() { EVMKeeper.SetState(Ctx, contract, slot, originalState) })
	value := common.HexToHash("0x123456789abcdef00123456789abcdef0123456789abcdef00123456789abcdef0")
	EVMKeeper.SetState(Ctx, contract, slot, value)

	wantCode := EVMKeeper.GetCode(Ctx, contract)
	require.NotEmpty(t, wantCode, "the test contract must have deployed code")
	wantStorage := EVMKeeper.GetState(Ctx, contract, slot)
	require.Equal(t, value, wantStorage)

	code, err := client.CodeAt(ctx, contract, nil)
	require.NoError(t, err)
	require.Equal(t, wantCode, code, "CodeAt must return the keeper code")

	storage, err := client.StorageAt(ctx, contract, slot, nil)
	require.NoError(t, err)
	require.Len(t, storage, common.HashLength)
	require.Equal(t, wantStorage[:], storage, "StorageAt must return keeper slot 0")
}
