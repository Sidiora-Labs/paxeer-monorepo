package evmrpc_test

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func TestEthclientSendTransactionRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var client *ethclient.Client
	client, err := ethclient.DialContext(ctx, fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	t.Cleanup(client.Close)

	mnemonic := "fish mention unlock february marble dove vintage sand hub ordinary fade found inject room embark supply fabric improve spike stem give current similar glimpse"
	derived, err := hd.Secp256k1.Derive()(mnemonic, "", "")
	require.NoError(t, err)
	key, err := crypto.ToECDSA(hd.Secp256k1.Generate()(derived).Bytes())
	require.NoError(t, err)
	chainID := EVMKeeper.ChainID(Ctx)
	chainConfig := types.DefaultChainConfig().EthereumConfig(chainID)
	signer := ethtypes.MakeSigner(chainConfig, big.NewInt(Ctx.BlockHeight()), uint64(Ctx.BlockTime().Unix()))
	to := common.HexToAddress("010203")
	signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     2,
		GasFeeCap: big.NewInt(10),
		Gas:       1000,
		To:        &to,
		Value:     big.NewInt(1000),
		Data:      []byte("abc"),
	}), signer, key)
	require.NoError(t, err)
	require.Equal(t, multiTxBlockTx1.Hash(), signed.Hash(), "signed transfer must match the package's committed fixture")

	require.NoError(t, client.SendTransaction(ctx, signed))
	returned, pending, err := client.TransactionByHash(ctx, signed.Hash())
	require.NoError(t, err)
	require.NotNil(t, returned)
	require.False(t, pending, "the package fixture is committed")
	require.Equal(t, signed.Hash(), returned.Hash())
	wantBytes, err := signed.MarshalBinary()
	require.NoError(t, err)
	gotBytes, err := returned.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, wantBytes, gotBytes, "transaction fields and signature must survive the HTTP round trip")
	sender, err := ethtypes.Sender(signer, returned)
	require.NoError(t, err)
	require.Equal(t, crypto.PubkeyToAddress(key.PublicKey), sender)

	receipt, err := client.TransactionReceipt(ctx, signed.Hash())
	require.NoError(t, err)
	require.NotNil(t, receipt)
	require.Equal(t, signed.Hash(), receipt.TxHash)
	require.NotNil(t, receipt.BlockNumber)
	require.Equal(t, int64(MockHeight2), receipt.BlockNumber.Int64())
	require.Equal(t, common.HexToHash(MultiTxBlockHash), receipt.BlockHash)
	require.Equal(t, uint(0), receipt.TransactionIndex)
}
