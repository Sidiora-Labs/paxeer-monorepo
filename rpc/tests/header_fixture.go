package tests

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// SetupCommittedHeaderFixture initializes the same in-process application and
// consensus client as SetupTestServer, without starting an HTTP listener.
func SetupCommittedHeaderFixture(t *testing.T) (*app.App, *MockClient) {
	t.Helper()
	contract := common.HexToAddress("0x1460")
	a, c := initializeApp(t, "pax-test", mnemonicInitializer(mnemonic1), func(ctx sdk.Context, a *app.App) {
		a.EvmKeeper.SetCode(ctx, contract, common.FromHex("0x602a600052600160206000a100"))
		a.EvmKeeper.SetNonce(ctx, contract, 1)
	})
	return a, c
}

// RecordCommittedBlock records the actual block and results after the fixture's
// application has finalized and committed them.
func (c *MockClient) RecordCommittedBlock(block *coretypes.ResultBlock, results *coretypes.ResultBlockResults) {
	if c.mockedBlockResults == nil {
		c.mockedBlockResults = make(map[int64]*coretypes.ResultBlock)
		c.mockedBlockByHashResults = make(map[string]*coretypes.ResultBlock)
		c.mockedBlockResultsResults = make(map[int64]*coretypes.ResultBlockResults)
	}
	c.mockedBlockResults[block.Block.Height] = block
	c.mockedBlockByHashResults[block.BlockID.Hash.String()] = block
	c.mockedBlockResultsResults[block.Block.Height] = results
}

// CommittedHeaderTransactions signs the bank and EVM transactions using the
// fixture's funded account and log-emitting EVM contract.
func CommittedHeaderTransactions(t *testing.T, a *app.App) ([]byte, [][]byte) {
	t.Helper()
	t.Helper()
	address := getPaxAddrWithMnemonic(mnemonic1)
	account := a.AccountKeeper.GetAccount(a.RPCContextProvider(evmrpc.LatestCtxHeight), address)
	if account == nil {
		t.Fatal("header fixture funded account is unavailable")
	}
	bank := banktypes.NewMsgSend(address, address, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1)))
	contract := common.HexToAddress("0x1460")
	transactions := make([][]byte, 2)
	for i := range transactions {
		transactions[i] = signAndEncodeTx(&ethtypes.DynamicFeeTx{
			ChainID: a.EvmKeeper.ChainID(a.RPCContextProvider(evmrpc.LatestCtxHeight)), Nonce: uint64(i), Gas: 100000,
			GasFeeCap: big.NewInt(1000000000), To: &contract,
		}, mnemonic1)
	}
	return signAndEncodeCosmosTx(bank, mnemonic1, account.GetAccountNumber(), 0), transactions
}
