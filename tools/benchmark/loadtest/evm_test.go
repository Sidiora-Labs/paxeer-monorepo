package main

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/benchmark/loadtest/contracts/evm/bindings/erc20"
)

func TestERC20FundingTxMintsTransferCover(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	client, err := ethclient.Dial("http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	tokenABI, err := erc20.Erc20MetaData.GetAbi()
	if err != nil {
		t.Fatal(err)
	}
	account := crypto.PubkeyToAddress(key.PublicKey)
	tokenAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	for _, useEip1559 := range []bool{false, true} {
		name := "legacy"
		if useEip1559 {
			name = "eip1559"
		}
		t.Run(name, func(t *testing.T) {
			txClient := &EvmTxClient{
				accountAddress: account,
				chainId:        big.NewInt(125),
				privateKey:     key,
				ethClients:     []*ethclient.Client{client},
				evmAddresses:   &EVMAddresses{ERC20: tokenAddress},
				useEip1559:     useEip1559,
			}
			fundingTx := txClient.GenerateERC20FundingTx()
			if fundingTx.To() == nil || *fundingTx.To() != tokenAddress {
				t.Fatalf("funding target = %v, want %s", fundingTx.To(), tokenAddress.Hex())
			}
			if fundingTx.ChainId().Cmp(big.NewInt(125)) != 0 {
				t.Fatalf("chain ID = %s, want 125", fundingTx.ChainId())
			}
			sender, err := ethtypes.Sender(ethtypes.NewLondonSigner(big.NewInt(125)), fundingTx)
			if err != nil {
				t.Fatal(err)
			}
			if sender != account {
				t.Fatalf("sender = %s, want %s", sender.Hex(), account.Hex())
			}
			if fundingTx.Gas() != 100000 || fundingTx.Value().Sign() != 0 {
				t.Fatalf("funding gas/value = %d/%s, want 100000/0", fundingTx.Gas(), fundingTx.Value())
			}
			data := fundingTx.Data()
			if len(data) < 4 {
				t.Fatalf("funding calldata is too short: %x", data)
			}
			method, err := tokenABI.MethodById(data[:4])
			if err != nil {
				t.Fatal(err)
			}
			if method.Sig != "mint(address,uint256)" {
				t.Fatalf("funding method = %s, want mint(address,uint256)", method.Sig)
			}
			args, err := method.Inputs.Unpack(data[4:])
			if err != nil {
				t.Fatal(err)
			}
			if len(args) != 2 {
				t.Fatalf("mint has %d arguments, want 2", len(args))
			}
			if to, ok := args[0].(common.Address); !ok || to != account {
				t.Fatalf("mint recipient = %v, want %s", args[0], account.Hex())
			}
			amount, ok := args[1].(*big.Int)
			if !ok {
				t.Fatalf("mint amount has type %T, want *big.Int", args[1])
			}
			if amount.Cmp(big.NewInt(ERC20FundingAmount)) != 0 || amount.Cmp(big.NewInt(9000000*1000000000000)) != 0 {
				t.Fatalf("mint amount = %s, want 9000000 * 10^12", amount)
			}
			maxTransfer := big.NewInt(8999999 * 1000000000000)
			if amount.Cmp(maxTransfer) <= 0 {
				t.Fatalf("mint amount %s must exceed maximum random transfer %s", amount, maxTransfer)
			}
			for i := 0; i < 1000; i++ {
				value := randomValue()
				if value.Sign() < 0 || value.Cmp(amount) >= 0 {
					t.Fatalf("random transfer %s is not covered by mint amount %s", value, amount)
				}
			}
		})
	}
}
