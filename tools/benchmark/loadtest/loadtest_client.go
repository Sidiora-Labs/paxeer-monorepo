package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/exp/slices"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/credentials/insecure"

	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	typestx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
)

const maxRetainedEvmTxHashes = 10000

type evmTxHashStore struct {
	mu       sync.RWMutex
	hashes   []common.Hash
	capacity int
	next     int
}

func newEvmTxHashStore(capacity int) *evmTxHashStore {
	if capacity <= 0 {
		panic("EVM transaction hash capacity must be positive")
	}
	return &evmTxHashStore{hashes: make([]common.Hash, 0, capacity), capacity: capacity}
}

func (s *evmTxHashStore) Add(hash common.Hash) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.hashes) < s.capacity {
		s.hashes = append(s.hashes, hash)
	} else {
		s.hashes[s.next] = hash
	}
	s.next = (s.next + 1) % s.capacity
}

func (s *evmTxHashStore) Snapshot() []common.Hash {
	s.mu.RLock()
	defer s.mu.RUnlock()
	hashes := make([]common.Hash, len(s.hashes))
	if len(s.hashes) < s.capacity {
		copy(hashes, s.hashes)
	} else {
		n := copy(hashes, s.hashes[s.next:])
		copy(hashes[n:], s.hashes[:s.next])
	}
	return hashes
}

type LoadTestClient struct {
	LoadTestConfig     Config
	TestConfig         EncodingConfig
	AccountKeys        []cryptotypes.PrivKey
	TxClients          []typestx.ServiceClient
	EvmTxClients       []*EvmTxClient
	SignerClient       *SignerClient
	ChainID            string
	GrpcConns          []*grpc.ClientConn
	StakingQueryClient stakingtypes.QueryClient
	// Staking specific variables
	Validators []stakingtypes.Validator
	// DelegationMap is a map of delegator -> validator -> delegated amount
	DelegationMap map[string]map[string]int
	// Tokenfactory specific variables
	TokenFactoryDenomOwner map[string]string
	// Only one admin message can go in per block
	generatedAdminMessageForBlock bool
	// Messages that has to be sent from the admin account
	isAdminMessageMapping map[string]bool
	mtx                   *sync.RWMutex
	monitors              *grpcMonitorGroup
}

func NewLoadTestClient(config Config) (*LoadTestClient, error) {
	txClients, grpcConns, err := BuildGrpcClients(&config)
	if err != nil {
		return nil, err
	}
	signerClient := NewSignerClient(config.NodeURI)
	keys := signerClient.GetTestAccountsKeys(int(config.MaxAccounts)) //nolint:gosec
	var evmTxClients []*EvmTxClient
	if config.EvmRpcEndpoints != "" {
		if config.ContainsAnyMessageTypes(EVM, ERC20, ERC721) {
			evmTxClients = BuildEvmTxClients(&config, keys)
		}
	}

	// Fill message type maps with empty values
	for _, messageType := range config.MessageTypes {
		producedCountPerMsgType[messageType] = new(int64)
		sentCountPerMsgType[messageType] = new(int64)
		prevSentCounterPerMsgType[messageType] = new(int64)
	}

	client := &LoadTestClient{
		LoadTestConfig:                config,
		AccountKeys:                   keys,
		TestConfig:                    TestConfig,
		TxClients:                     txClients,
		EvmTxClients:                  evmTxClients,
		SignerClient:                  signerClient,
		ChainID:                       config.ChainID,
		GrpcConns:                     grpcConns,
		StakingQueryClient:            stakingtypes.NewQueryClient(grpcConns[0]),
		DelegationMap:                 map[string]map[string]int{},
		TokenFactoryDenomOwner:        map[string]string{},
		generatedAdminMessageForBlock: false,
		isAdminMessageMapping:         map[string]bool{CollectRewards: true, DistributeRewards: true},
		mtx:                           &sync.RWMutex{},
	}
	client.startGrpcMonitors()
	return client, nil
}

func (c *LoadTestClient) SetValidators() {
	if slices.Contains(c.LoadTestConfig.MessageTypes, "staking") {
		resp, err := c.StakingQueryClient.Validators(context.Background(), &stakingtypes.QueryValidatorsRequest{})
		if err != nil {
			panic(err)
		}
		c.Validators = resp.Validators
	}
}

const grpcStartupTimeout = 5 * time.Second

// BuildGrpcClients bounds the entire startup and closes partial connections on error.
func BuildGrpcClients(config *Config) ([]typestx.ServiceClient, []*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), grpcStartupTimeout)
	defer cancel()
	return buildGrpcClients(ctx, config)
}

func buildGrpcClients(ctx context.Context, config *Config) ([]typestx.ServiceClient, []*grpc.ClientConn, error) {
	var clients []typestx.ServiceClient
	var conns []*grpc.ClientConn
	fail := func(err error) ([]typestx.ServiceClient, []*grpc.ClientConn, error) {
		for _, conn := range conns {
			_ = conn.Close()
		}
		return nil, nil, err
	}
	var roots *x509.CertPool
	if config.TLS && config.TLSCAFile != "" {
		raw, err := os.ReadFile(config.TLSCAFile)
		if err != nil {
			return fail(err)
		}
		roots = x509.NewCertPool()
		if !roots.AppendCertsFromPEM(raw) {
			return fail(errors.New("TLS CA file contains no certificates"))
		}
	}
	for _, value := range strings.Split(config.GrpcEndpoints, ",") {
		endpoint := strings.TrimSpace(value)
		if endpoint == "" {
			return fail(errors.New("empty gRPC endpoint"))
		}
		options := []grpc.DialOption{grpc.WithBlock(), grpc.WithReturnConnectionError(), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(20*1024*1024), grpc.MaxCallSendMsgSize(20*1024*1024))}
		if config.TLS {
			target := endpoint
			if strings.Contains(target, "://") {
				parsed, err := url.Parse(target)
				if err != nil {
					return fail(err)
				}
				target = strings.TrimPrefix(parsed.Path, "/")
				if target == "" {
					target = parsed.Host
				}
			}
			host, _, err := net.SplitHostPort(target)
			if err != nil || host == "" {
				return fail(fmt.Errorf("TLS endpoint requires host:port: %q", endpoint))
			}
			options = append(options, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: host})))
		} else {
			options = append(options, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
		conn, err := grpc.DialContext(ctx, endpoint, options...)
		if err != nil {
			return fail(fmt.Errorf("gRPC dial %s: %w", endpoint, err))
		}
		clients = append(clients, typestx.NewServiceClient(conn))
		conns = append(conns, conn)
	}
	return clients, conns, nil
}

type grpcMonitorGroup struct {
	cancel context.CancelFunc
	wg     sync.WaitGroup
	once   sync.Once
}

func (c *LoadTestClient) startGrpcMonitors() {
	ctx, cancel := context.WithCancel(context.Background())
	c.monitors = &grpcMonitorGroup{cancel: cancel}
	for _, conn := range c.GrpcConns {
		c.monitors.wg.Add(1)
		go func(conn *grpc.ClientConn) {
			defer c.monitors.wg.Done()
			for {
				state := conn.GetState()
				if state == connectivity.Shutdown || ctx.Err() != nil {
					return
				}
				if state == connectivity.Idle {
					conn.Connect()
				}
				if !conn.WaitForStateChange(ctx, state) {
					return
				}
			}
		}(conn)
	}
}

// BuildEvmTxClients build a list of EvmTxClients with a list of go-ethereum client
func BuildEvmTxClients(config *Config, keys []cryptotypes.PrivKey) []*EvmTxClient {
	clients := make([]*EvmTxClient, len(keys))
	ethEndpoints := strings.Split(config.EvmRpcEndpoints, ",")
	if len(ethEndpoints) == 0 {
		return clients
	}
	ethClients := make([]*ethclient.Client, len(ethEndpoints))
	for i, endpoint := range ethEndpoints {
		client, err := ethclient.Dial(endpoint)
		if err != nil {
			fmt.Printf("Failed to connect to endpoint %s with error %s", endpoint, err.Error())
		}
		ethClients[i] = client
	}
	// Get chainId
	chainID, err := ethClients[0].NetworkID(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Failed to get chain ID: %v \n", err))
	}
	// Get gas price
	gasPrice, err := ethClients[0].SuggestGasPrice(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Failed to suggest gas price: %v\n", err))
	}
	// Build one client per key
	for i, key := range keys {
		clients[i] = NewEvmTxClient(key, chainID, gasPrice, ethClients, config.EVMAddresses, config.EvmUseEip1559Txs)
	}
	return clients
}

func (c *LoadTestClient) Close() {
	if c.monitors == nil {
		for _, conn := range c.GrpcConns {
			_ = conn.Close()
		}
		return
	}
	c.monitors.once.Do(func() {
		c.monitors.cancel()
		for _, conn := range c.GrpcConns {
			_ = conn.Close()
		}
		c.monitors.wg.Wait()
	})
}

func (c *LoadTestClient) BuildTxs(
	ctx context.Context,
	txQueue chan SignedTx,
	keyIndex int,
	rateLimiter *rate.Limiter,
	producedCountPerMsgType map[string]*int64,
) error {
	config := c.LoadTestConfig
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := rateLimiter.Wait(ctx); err != nil {
				return err
			}
			// Generate a message type first
			messageType := c.getRandomMessageType(config.MessageTypes)
			loadtestMetrics.produceCount.Add(context.Background(), 1, otelmetric.WithAttributes(attribute.String("msg_type", messageType)))
			var signedTx SignedTx
			// Sign EVM and Cosmos TX differently
			switch messageType {
			case EVM, ERC20, ERC721:
				evmTx, err := c.EvmTxClients[keyIndex].getTxForMsgTypeContext(ctx, messageType)
				if err != nil {
					fmt.Printf("EVM producer waiting for reconciliation: %v\n", err)
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(100 * time.Millisecond):
						continue
					}
				}
				signedTx = SignedTx{EvmTx: evmTx, MsgType: messageType}
			default:
				msgTypeCount := atomic.LoadInt64(producedCountPerMsgType[messageType])
				txs, err := c.generateSignedCosmosTxs(keyIndex, messageType, msgTypeCount)
				if err != nil {
					return err
				}
				signedTx = SignedTx{TxBytes: txs, MsgType: messageType}
			}

			select {
			case txQueue <- signedTx:
				atomic.AddInt64(producedCountPerMsgType[messageType], 1)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

func (c *LoadTestClient) generateSignedEvmTx(keyIndex int, msgType string) (*ethtypes.Transaction, error) {
	return c.EvmTxClients[keyIndex].GetTxForMsgType(msgType)
}

func (c *LoadTestClient) generateSignedCosmosTxs(keyIndex int, msgType string, msgTypeCount int64) ([]byte, error) {
	key := c.AccountKeys[keyIndex]
	msgs, _, _, gas, fee := c.generateMessage(key, msgType)
	txBuilder := TestConfig.TxConfig.NewTxBuilder()
	if err := txBuilder.SetMsgs(msgs...); err != nil {
		return nil, fmt.Errorf("setMsgs: %w", err)
	}
	txBuilder.SetGasLimit(gas)
	txBuilder.SetFeeAmount([]types.Coin{
		types.NewCoin("uhpx", types.NewInt(fee)),
	})
	// Use random seqno to get around txs that might already be seen in mempool
	if err := c.SignerClient.SignTx(c.ChainID, &txBuilder, key, uint64(msgTypeCount)); err != nil { //nolint:gosec // msgTypeCount is a small positive counter; no overflow risk
		return nil, fmt.Errorf("signTx: %w", err)
	}
	txBytes, err := TestConfig.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("txEncoder: %w", err)
	}
	return txBytes, nil
}

func (c *LoadTestClient) SendTxs(
	ctx context.Context,
	txQueue chan SignedTx,
	keyIndex int,
	sentCountPerMsgType map[string]*int64,
	semaphore *semaphore.Weighted,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case tx, ok := <-txQueue:
			if !ok {
				return nil
			}
			loadtestMetrics.consumeCount.Add(context.Background(), 1, otelmetric.WithAttributes(attribute.String("msg_type", tx.MsgType)))
			// Acquire a semaphore
			if err := semaphore.Acquire(ctx, 1); err != nil {
				return fmt.Errorf("acquire consumer semaphore: %w", err)
			}
			sendCtx, cancel := context.WithTimeout(ctx, benchmarkRPCTimeout)
			if len(tx.TxBytes) > 0 {
				atomic.AddInt64(sentCountPerMsgType[tx.MsgType], 1)
				// Send Cosmos Transactions
				if SendTx(sendCtx, tx.TxBytes, typestx.BroadcastMode_BROADCAST_MODE_BLOCK, *c) {
					atomic.AddInt64(producedCountPerMsgType[tx.MsgType], 1)
				}
			} else if tx.EvmTx != nil {
				// Send EVM Transactions
				err := c.EvmTxClients[keyIndex].sendEvmTxContext(sendCtx, tx.EvmTx, func() {
					EvmTxHashes.Add(tx.EvmTx.Hash())
					atomic.AddInt64(sentCountPerMsgType[tx.MsgType], 1)
				})
				if err != nil {
					fmt.Printf("EVM send: %v\n", err)
				}
			}
			cancel()
			// Release the semaphore
			semaphore.Release(1)
		}
	}
}

//nolint:staticcheck
func (c *LoadTestClient) GetTxClient() typestx.ServiceClient {
	numClients := len(c.TxClients)
	if numClients <= 0 {
		panic("There's no Tx client available, make sure your connection are valid")
	}
	rand.Seed(time.Now().Unix())
	return c.TxClients[rand.Int()%len(c.TxClients)]
}
