package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/rpc"
	"math/big"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/benchmark/loadtest/contracts/evm/bindings/erc20"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/benchmark/loadtest/contracts/evm/bindings/erc721"
)

const ERC20FundingAmount = 9000000 * 1000000000000

var (
	DefaultPriorityFee = big.NewInt(1000000000)    // 1gwei
	DefaultMaxFee      = big.NewInt(1000000000000) // 1000gwei
)

type EvmTxClient struct {
	accountAddress   common.Address
	nonce            atomic.Uint64
	chainId          *big.Int
	gasPrice         *big.Int
	ethClients       []*ethclient.Client
	mtx              sync.RWMutex
	privateKey       *ecdsa.PrivateKey
	evmAddresses     *EVMAddresses
	useEip1559       bool
	queued           map[common.Hash]*ethtypes.Transaction
	uncertain        *ethtypes.Transaction
	uncertainPeer    *ethclient.Client
	uncertainSuccess func()
	uncertainRefused bool
	needsReconcile   bool
	rpcTimeout       time.Duration
}

func NewEvmTxClient(
	key cryptotypes.PrivKey,
	chainId *big.Int,
	gasPrice *big.Int,
	ethClients []*ethclient.Client,
	evmAddresses *EVMAddresses,
	useEip1559 bool,
) *EvmTxClient {
	if evmAddresses == nil {
		evmAddresses = &EVMAddresses{}
	}
	txClient := &EvmTxClient{
		chainId:      chainId,
		gasPrice:     gasPrice,
		ethClients:   ethClients,
		mtx:          sync.RWMutex{},
		evmAddresses: evmAddresses,
	}
	privKeyHex := hex.EncodeToString(key.Bytes())
	privateKey, err := crypto.HexToECDSA(privKeyHex)
	if err != nil {
		fmt.Printf("Failed to load private key: %v \n", err)
	}
	txClient.privateKey = privateKey

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		panic("Cannot assert type: publicKey is not of type *ecdsa.PublicKey \n")
	}

	// Set starting nonce
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	ctx, cancel := txClient.callContext()
	defer cancel()
	nextNonce, err := ethClients[0].PendingNonceAt(ctx, fromAddress)
	if err != nil {
		panic(err)
	}
	txClient.nonce.Store(nextNonce)
	txClient.accountAddress = fromAddress
	txClient.useEip1559 = useEip1559

	return txClient
}

func (txClient *EvmTxClient) GetTxForMsgType(msgType string) (*ethtypes.Transaction, error) {
	txClient.mtx.Lock()
	defer txClient.mtx.Unlock()
	if err := txClient.prepareLocked(); err != nil {
		return nil, err
	}
	switch msgType {
	case EVM:
		return txClient.generateSendFundsTxLocked(), nil
	case ERC20:
		return txClient.generateERC20TransferTxLocked(), nil
	case ERC721:
		return txClient.generateERC721MintLocked(), nil
	default:
		return nil, errors.New("invalid EVM message type")
	}
}

func randomValue() *big.Int {
	return big.NewInt(rand.Int63n(9000000) * 1000000000000)
}

// GenerateSendFundsTx returns a random send funds tx
//
//nolint:staticcheck
func (txClient *EvmTxClient) GenerateSendFundsTx() *ethtypes.Transaction {
	txClient.mtx.Lock()
	defer txClient.mtx.Unlock()
	return txClient.generateSendFundsTxLocked()
}

func (txClient *EvmTxClient) generateSendFundsTxLocked() *ethtypes.Transaction {
	var tx *ethtypes.Transaction
	if !txClient.useEip1559 {
		tx = ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce:    txClient.nextNonce(),
			GasPrice: DefaultMaxFee,
			Gas:      uint64(21000),
			To:       &txClient.accountAddress,
			Value:    randomValue(),
		})
	} else {
		dynamicTx := &ethtypes.DynamicFeeTx{
			ChainID:   txClient.chainId,
			Nonce:     txClient.nextNonce(),
			GasTipCap: DefaultPriorityFee,
			GasFeeCap: DefaultMaxFee,
			Gas:       uint64(21000),
			To:        &txClient.accountAddress,
			Value:     randomValue(),
		}
		tx = ethtypes.NewTx(dynamicTx)
	}
	return txClient.sign(tx)
}

func (txClient *EvmTxClient) GenerateERC20FundingTx() *ethtypes.Transaction {
	txClient.mtx.Lock()
	defer txClient.mtx.Unlock()
	return txClient.generateERC20FundingTxLocked()
}

func (txClient *EvmTxClient) generateERC20FundingTxLocked() *ethtypes.Transaction {
	opts := txClient.getTransactOpts()
	opts.GasLimit = uint64(100000)
	token, err := erc20.NewErc20(txClient.evmAddresses.ERC20, GetNextEthClient(txClient.ethClients))
	if err != nil {
		panic(fmt.Sprintf("Failed to create ERC20 contract: %v \n", err))
	}
	tx, err := token.Mint(opts, txClient.accountAddress, big.NewInt(ERC20FundingAmount))
	if err != nil {
		panic(fmt.Sprintf("Failed to create ERC20 funding transaction: %v \n", err))
	}
	return txClient.sign(tx)
}

// GenerateERC20TransferTx returns a random self-transfer of the minted balance.
func (txClient *EvmTxClient) GenerateERC20TransferTx() *ethtypes.Transaction {
	txClient.mtx.Lock()
	defer txClient.mtx.Unlock()
	return txClient.generateERC20TransferTxLocked()
}

func (txClient *EvmTxClient) generateERC20TransferTxLocked() *ethtypes.Transaction {
	opts := txClient.getTransactOpts()
	// override gas limit for an ERC20 transfer
	opts.GasLimit = uint64(100000)
	tokenAddress := txClient.evmAddresses.ERC20
	token, err := erc20.NewErc20(tokenAddress, GetNextEthClient(txClient.ethClients))
	if err != nil {
		panic(fmt.Sprintf("Failed to create ERC20 contract: %v \n", err))
	}
	tx, err := token.Transfer(opts, txClient.accountAddress, randomValue())
	if err != nil {
		panic(fmt.Sprintf("Failed to create ERC20 transfer: %v \n", err))
	}
	return txClient.sign(tx)
}

func (txClient *EvmTxClient) GenerateERC721Mint() *ethtypes.Transaction {
	txClient.mtx.Lock()
	defer txClient.mtx.Unlock()
	return txClient.generateERC721MintLocked()
}

func (txClient *EvmTxClient) generateERC721MintLocked() *ethtypes.Transaction {
	opts := txClient.getTransactOpts()
	// override gas limit for an ERC20 transfer
	opts.GasLimit = uint64(100000)
	tokenAddress := txClient.evmAddresses.ERC721
	token, err := erc721.NewErc721(tokenAddress, GetNextEthClient(txClient.ethClients))
	if err != nil {
		panic(fmt.Sprintf("Failed to create ERC721 contract: %v \n", err))
	}
	tx, err := token.Mint(opts, txClient.accountAddress, randomValue())
	if err != nil {
		panic(fmt.Sprintf("Failed to create ERC20 transfer: %v \n", err))
	}
	return txClient.sign(tx)
}

func (txClient *EvmTxClient) getTransactOpts() *bind.TransactOpts {
	auth, err := bind.NewKeyedTransactorWithChainID(txClient.privateKey, txClient.chainId)
	if err != nil {
		panic(fmt.Sprintf("Failed to create transactor: %v \n", err))
	}
	if !txClient.useEip1559 {
		auth.GasPrice = DefaultMaxFee
	} else {
		auth.GasFeeCap = DefaultMaxFee
		auth.GasTipCap = DefaultPriorityFee
	}
	auth.Nonce = big.NewInt(int64(txClient.nextNonce())) //nolint:gosec
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(21000)
	auth.Context = context.Background()
	auth.From = txClient.accountAddress
	auth.NoSend = true
	return auth
}

func (txClient *EvmTxClient) sign(tx *ethtypes.Transaction) *ethtypes.Transaction {
	signedTx, err := ethtypes.SignTx(tx, ethtypes.NewLondonSigner(txClient.chainId), txClient.privateKey)
	if err != nil {
		// this should not happen
		panic(err)
	}
	if txClient.queued == nil {
		txClient.queued = make(map[common.Hash]*ethtypes.Transaction)
	}
	txClient.queued[signedTx.Hash()] = signedTx
	return signedTx
}

func (txClient *EvmTxClient) callContext() (context.Context, context.CancelFunc) {
	timeout := txClient.rpcTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return context.WithTimeout(context.Background(), timeout)
}

func (txClient *EvmTxClient) nextNonce() uint64 {
	if err := txClient.prepareLocked(); err != nil {
		panic(err)
	}
	return txClient.nonce.Add(1) - 1
}

func (txClient *EvmTxClient) PrepareNext() error {
	txClient.mtx.Lock()
	defer txClient.mtx.Unlock()
	return txClient.prepareLocked()
}

func (txClient *EvmTxClient) pendingNonce(peer *ethclient.Client) (uint64, error) {
	ctx, cancel := txClient.callContext()
	defer cancel()
	return peer.PendingNonceAt(ctx, txClient.accountAddress)
}

func (txClient *EvmTxClient) knownTransaction(peer *ethclient.Client, tx *ethtypes.Transaction) (bool, error) {
	ctx, cancel := txClient.callContext()
	defer cancel()
	found, _, err := peer.TransactionByHash(ctx, tx.Hash())
	if errors.Is(err, ethereum.NotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if found.Hash() != tx.Hash() {
		return false, errors.New("RPC returned a different transaction")
	}
	return true, nil
}

func (txClient *EvmTxClient) prepareLocked() error {
	if !txClient.needsReconcile {
		return nil
	}
	peer := txClient.uncertainPeer
	if peer == nil {
		peer = GetNextEthClient(txClient.ethClients)
	}
	accepted := false
	if txClient.uncertain != nil {
		known, err := txClient.knownTransaction(peer, txClient.uncertain)
		if err != nil {
			return fmt.Errorf("unresolved EVM submission: %w", err)
		}
		if !known && !txClient.uncertainRefused {
			return errors.New("unresolved EVM submission; generation is blocked until its hash is observed")
		}
		accepted = known
	}
	nonce, err := txClient.pendingNonce(peer)
	if err != nil {
		return fmt.Errorf("reconcile pending nonce: %w", err)
	}
	if accepted && nonce <= txClient.uncertain.Nonce() {
		return errors.New("RPC pending nonce has not reconciled the accepted transaction")
	}
	txClient.nonce.Store(nonce)
	txClient.queued = make(map[common.Hash]*ethtypes.Transaction)
	txClient.needsReconcile = false
	callback := txClient.uncertainSuccess
	txClient.uncertain, txClient.uncertainSuccess, txClient.uncertainPeer = nil, nil, nil
	txClient.uncertainRefused = false
	if accepted && callback != nil {
		callback()
	}
	return nil
}

// Only explicit pre-admission refusals permit nonce reuse after an absent hash.
// RPC timeouts/internal errors remain uncertain even when encoded as JSON-RPC errors.
func definitiveEVMRefusal(err error) bool {
	var response rpc.Error
	if !errors.As(err, &response) {
		return false
	}
	message := strings.ToLower(response.Error())
	for _, reason := range []string{"transaction refused", "insufficient funds", "intrinsic gas too low", "exceeds block gas limit", "transaction underpriced", "max fee per gas less than block base fee", "nonce too low", "nonce too high", "invalid sender"} {
		if strings.Contains(message, reason) {
			return true
		}
	}
	return false
}

// SendEvmTx serializes sends with generation and invalidates queued work on failure.
func (txClient *EvmTxClient) SendEvmTx(tx *ethtypes.Transaction, onSuccess func()) error {
	txClient.mtx.Lock()
	defer txClient.mtx.Unlock()
	if err := txClient.prepareLocked(); err != nil {
		return err
	}
	if tx == nil {
		return errors.New("nil EVM transaction")
	}
	if _, ok := txClient.queued[tx.Hash()]; !ok {
		return errors.New("discarding stale queued EVM transaction")
	}
	peer := GetNextEthClient(txClient.ethClients)
	ctx, cancel := txClient.callContext()
	err := peer.SendTransaction(ctx, tx)
	cancel()
	if err == nil {
		delete(txClient.queued, tx.Hash())
		if onSuccess != nil {
			onSuccess()
		}
		return nil
	}
	// Even an RPC error (already known, nonce too low) may describe an accepted
	// transaction. Reconcile the hash before deciding whether a nonce can be reused.
	txClient.needsReconcile = true
	txClient.uncertainPeer = peer
	txClient.queued = make(map[common.Hash]*ethtypes.Transaction)
	known, lookupErr := txClient.knownTransaction(peer, tx)
	refused := definitiveEVMRefusal(err)
	if lookupErr != nil || known || !refused {
		txClient.uncertain = tx
		txClient.uncertainSuccess = onSuccess
		txClient.uncertainRefused = refused
	}
	if reconcileErr := txClient.prepareLocked(); reconcileErr != nil {
		return fmt.Errorf("send failed (%v); %w", err, reconcileErr)
	}
	if known {
		return nil
	}
	return fmt.Errorf("EVM submission refused; queued transactions invalidated: %w", err)
}

// GetNextEthClient return the next available eth client randomly
//
//nolint:staticcheck
func GetNextEthClient(clients []*ethclient.Client) *ethclient.Client {
	numClients := len(clients)
	if numClients <= 0 {
		panic("There's no ETH client available, make sure your connection are valid")
	}
	rand.Seed(time.Now().Unix())
	return clients[rand.Int()%numClients]
}

// GetTxReceipt query the transaction receipt to check if the tx succeed or not
func (txClient *EvmTxClient) GetTxReceipt(txHash common.Hash) error {
	ctx, cancel := txClient.callContext()
	defer cancel()
	_, err := GetNextEthClient(txClient.ethClients).TransactionReceipt(ctx, txHash)
	if err != nil {
		return err
	}
	return nil
}

// check receipt success
func (txClient *EvmTxClient) EnsureTxSuccess(txHash common.Hash) {
	ctx, cancel := txClient.callContext()
	defer cancel()
	receipt, err := GetNextEthClient(txClient.ethClients).TransactionReceipt(ctx, txHash)
	if err != nil {
		panic(fmt.Sprintf("Failed to get receipt for tx %v: %v \n", txHash.Hex(), err))
	}
	if receipt.Status != 1 {
		panic(fmt.Sprintf("Tx %v failed with status %v \n", txHash.Hex(), receipt.Status))
	}
}

// ResetNonce reconciles before admitting more generated work.
func (txClient *EvmTxClient) ResetNonce() error {
	txClient.mtx.Lock()
	defer txClient.mtx.Unlock()
	txClient.needsReconcile = true
	return txClient.prepareLocked()
}
