package internaltest

import (
	"context"
	"math/big"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
)

type MarketChain struct {
	RPC        *ethclient.Client
	RPCURL     string
	Creation   *types.Receipt
	Swaps      []*types.Receipt
	Headers    map[int64]chainread.Block
	Pool       common.Address
	Token      common.Address
	Settlement common.Address
	Sender     common.Address
}

func marketEventRuntime(topics []common.Hash, data []byte) []byte {
	if len(data) > 255 || len(topics) > 4 {
		panic("market event fixture exceeds EVM immediate bounds")
	}
	offset := 14 + 33*len(topics)
	dataLength := big.NewInt(int64(len(data))).Bytes()[0]
	offsetWord := big.NewInt(int64(offset)).FillBytes(make([]byte, 2))
	topicOpcode := big.NewInt(int64(0xa0 + len(topics))).Bytes()[0]
	code := []byte{0x60, dataLength, 0x61, offsetWord[0], offsetWord[1], 0x60, 0, 0x39}
	for i := len(topics) - 1; i >= 0; i-- {
		code = append(code, 0x7f)
		code = append(code, topics[i].Bytes()...)
	}
	code = append(code, 0x60, dataLength, 0x60, 0, topicOpcode, 0)
	return append(code, data...)
}

func marketWord(value *big.Int) []byte { return common.LeftPadBytes(value.Bytes(), 32) }

// NewMarketChain commits a pool creation and two swap logs on an owned chain.
func NewMarketChain(t *testing.T) MarketChain {
	return NewMarketChainAt(t, 0, 0)
}

// NewMarketChainAt commits an idle block span before the owned market events.
func NewMarketChainAt(t *testing.T, timestamp uint64, idleSpan time.Duration) MarketChain {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(key.PublicKey)
	address := func(label string) common.Address {
		return common.BytesToAddress(crypto.Keccak256(append(sender.Bytes(), []byte(label)...))[12:])
	}
	pool, token, settlement := address("pool"), address("token"), address("settlement")
	factory, emitter := address("factory"), address("emitter")
	created := append(marketWord(big.NewInt(1)), marketWord(new(big.Int).SetBytes(token.Bytes()))...)
	marketCreatedTopic := crypto.Keccak256Hash([]byte("MarketCreated(address,address,address,uint256,address)"))
	swapTopic := crypto.Keccak256Hash([]byte("Swap(bytes32,address,bool,uint256,uint256,uint256,uint256,uint256,uint256)"))
	factoryCode := marketEventRuntime([]common.Hash{marketCreatedTopic, common.BytesToHash(token.Bytes()), common.BytesToHash(pool.Bytes()), common.BytesToHash(sender.Bytes())}, created)
	swapCode := []byte{0x60, 0xa0, 0x60, 0x60, 0x60, 0x00, 0x37, 0x42, 0x60, 0xa0, 0x52, 0x43, 0x60, 0xc0, 0x52, 0x60, 0x40, 0x35, 0x60, 0x20, 0x35, 0x60, 0x00, 0x35, 0x60, 0xe0, 0x60, 0x00, 0xa3, 0x00}
	backend := simulated.NewBackend(types.GenesisAlloc{sender: {Balance: new(big.Int).Exp(big.NewInt(10), big.NewInt(21), nil)}, factory: {Code: factoryCode}, emitter: {Code: swapCode}}, func(n *node.Config, eth *ethconfig.Config) {
		chain := *eth.Genesis.Config
		chain.ChainID = big.NewInt(125)
		eth.Genesis.Config = &chain
		if timestamp > 0 {
			eth.Genesis.Timestamp = timestamp
		}
		n.HTTPHost = net.IPv4(127, 0, 0, 1).String()
		n.HTTPPort = port
		n.HTTPModules = []string{"eth"}
	})
	t.Cleanup(func() { _ = backend.Close() })
	if timestamp > 0 {
		if err := backend.AdjustTime(time.Second); err != nil {
			t.Fatal(err)
		}
		if idleSpan > 0 {
			if err := backend.AdjustTime(idleSpan); err != nil {
				t.Fatal(err)
			}
		}
	} else {
		backend.Commit()
		if idleSpan > 0 {
			if err := backend.AdjustTime(idleSpan); err != nil {
				t.Fatal(err)
			}
		}
	}
	ctx := context.Background()
	client := backend.Client()
	request := func(nonce uint64, to common.Address, data []byte) *types.Receipt {
		t.Helper()
		head, e := client.HeaderByNumber(ctx, nil)
		if e != nil {
			t.Fatal(e)
		}
		tip, e := client.SuggestGasTipCap(ctx)
		if e != nil {
			t.Fatal(e)
		}
		transaction, e := types.SignNewTx(key, types.LatestSignerForChainID(big.NewInt(125)), &types.DynamicFeeTx{ChainID: big.NewInt(125), Nonce: nonce, Gas: 300000, GasTipCap: tip, GasFeeCap: new(big.Int).Add(tip, new(big.Int).Mul(head.BaseFee, big.NewInt(2))), To: &to, Data: data})
		if e != nil {
			t.Fatal(e)
		}
		if e = client.SendTransaction(ctx, transaction); e != nil {
			t.Fatal(e)
		}
		backend.Commit()
		receipt, e := client.TransactionReceipt(ctx, transaction.Hash())
		if e != nil || receipt.Status != types.ReceiptStatusSuccessful || len(receipt.Logs) != 1 {
			t.Fatalf("canonical event receipt: %+v, %v", receipt, e)
		}
		return receipt
	}
	creation := request(0, factory, nil)
	swapData := append([]byte{}, swapTopic.Bytes()...)
	swapData = append(swapData, common.BytesToHash(pool.Bytes()).Bytes()...)
	swapData = append(swapData, common.BytesToHash(sender.Bytes()).Bytes()...)
	for _, amount := range []int64{1, 1000000, 2000000, 1000, 500000} {
		swapData = append(swapData, marketWord(big.NewInt(amount))...)
	}
	swaps := []*types.Receipt{request(1, emitter, swapData), request(2, emitter, swapData)}
	endpoint := "http://127.0.0.1:" + strconv.Itoa(port)
	rpc, err := ethclient.Dial(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rpc.Close)
	headers := map[int64]chainread.Block{}
	for height := int64(1); height <= swaps[1].BlockNumber.Int64(); height++ {
		block, e := (chainread.Reader{RPC: rpc}).Block(ctx, height)
		if e != nil {
			t.Fatal(e)
		}
		headers[height] = block
	}
	return MarketChain{RPC: rpc, RPCURL: endpoint, Creation: creation, Swaps: swaps, Headers: headers, Pool: pool, Token: token, Settlement: settlement, Sender: sender}
}
