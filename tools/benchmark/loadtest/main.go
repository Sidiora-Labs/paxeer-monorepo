package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	distributiontypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/benchmark/loadtest/operations"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"

	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

var TestConfig EncodingConfig

const (
	VortexData = "{\"position_effect\":\"Open\",\"leverage\":\"1\"}"
)

var (
	FromMili                  = sdk.NewDec(1000000)
	producedCountPerMsgType   = make(map[string]*int64)
	sentCountPerMsgType       = make(map[string]*int64)
	prevSentCounterPerMsgType = make(map[string]*int64)

	BlockHeightsWithTxs = []int{}
	EvmTxHashes         = newEvmTxHashStore(maxRetainedEvmTxHashes)
	observationMu       sync.Mutex
)

type BlockData struct {
	Txs []string `json:"txs"`
}

type BlockHeader struct {
	Time string `json:"time"`
}

func init() {
	cdc := codec.NewLegacyAmino()
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)

	TestConfig = EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Marshaler:         marshaler,
		TxConfig:          tx.NewTxConfig(marshaler, tx.DefaultSignModes),
		Amino:             cdc,
	}
	std.RegisterLegacyAminoCodec(TestConfig.Amino)
	std.RegisterInterfaces(TestConfig.InterfaceRegistry)
	app.ModuleBasics.RegisterLegacyAminoCodec(TestConfig.Amino)
	app.ModuleBasics.RegisterInterfaces(TestConfig.InterfaceRegistry)
	// Add this so that we don't end up getting disconnected for EVM client
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 100
}

// deployEvmContract executes the deployment entry point and validates its address.
func deployEvmContract(scriptPath string, config *Config) (common.Address, error) {
	return deployEvmContractWithCommands(context.Background(), operations.Executables{}, scriptPath, config)
}

func deployEvmContractWithCommands(ctx context.Context, commands operations.Commands, scriptPath string, config *Config) (common.Address, error) {
	raw, err := commands.Run(ctx, operations.Command{Args: []string{scriptPath, config.EVMRpcEndpoint()}})
	if err != nil {
		return common.Address{}, err
	}
	encoded, err := json.Marshal(map[string]string{"deployedTo": strings.TrimSpace(string(raw))})
	if err != nil {
		return common.Address{}, err
	}
	address, err := operations.DeploymentAddress(encoded)
	if err != nil {
		return common.Address{}, err
	}
	return common.HexToAddress(address), nil
}

//nolint:gosec
func deployEvmContracts(config *Config) {
	config.EVMAddresses = &EVMAddresses{}
	if config.ContainsAnyMessageTypes(ERC20) {
		fmt.Println("Deploying ERC20 contract")
		erc20, err := deployEvmContract("tools/benchmark/loadtest/contracts/deploy_erc20.sh", config)
		if err != nil {
			fmt.Println("error deploying, make sure 0xF87A299e6bC7bEba58dbBe5a5Aa21d49bCD16D52 is funded")
			panic(err)
		}
		config.EVMAddresses.ERC20 = erc20
	}
	if config.ContainsAnyMessageTypes(ERC721) {
		fmt.Println("Deploying ERC721 contract")
		erc721, err := deployEvmContract("tools/benchmark/loadtest/contracts/deploy_erc721.sh", config)
		if err != nil {
			fmt.Println("error deploying, make sure 0xF87A299e6bC7bEba58dbBe5a5Aa21d49bCD16D52 is funded")
			panic(err)
		}
		config.EVMAddresses.ERC721 = erc721
	}
}

func run(config *Config) {
	if err := config.Validate(); err != nil {
		panic(err)
	}
	config.EVMAddresses = &EVMAddresses{}
	// Start metrics collector in another thread
	metricsServer := MetricsServer{}
	go metricsServer.StartMetricsClient(*config)

	client, err := NewLoadTestClient(*config)
	if err != nil {
		panic(err)
	}
	defer client.Close()
	client.SetValidators()
	deployEvmContracts(config)

	// initialize clients with addresses on clients
	for _, txClient := range client.EvmTxClients {
		txClient.evmAddresses = config.EVMAddresses
	}

	if config.ContainsAnyMessageTypes(ERC20) {
		for _, txClient := range client.EvmTxClients {
			func() {
				defer func() {
					if err := recover(); err != nil {
						panic(fmt.Sprintf("Failed to fund ERC20 account %s: %v", txClient.accountAddress.Hex(), err))
					}
				}()
				fundingTx := txClient.GenerateERC20FundingTx()
				sent := false
				txClient.SendEvmTx(fundingTx, func() { sent = true })
				if !sent {
					panic("Failed to send ERC20 funding transaction")
				}
				if _, err := bind.WaitMined(context.Background(), GetNextEthClient(txClient.ethClients), fundingTx); err != nil {
					panic(err)
				}
				txClient.EnsureTxSuccess(fundingTx.Hash())
			}()
		}
	}

	if err := startLoadtestWorkers(client, *config); err != nil {
		panic(fmt.Errorf("benchmark incomplete: %w", err))
	}
	if err := runEvmQueries(*config); err != nil {
		panic(fmt.Errorf("post-load queries incomplete: %w", err))
	}
}

var errBenchmarkComplete = errors.New("benchmark ticks complete")

func startLoadtestWorkers(client *LoadTestClient, config Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	return runLoadtestWorkers(ctx, client, config, ticker.C)
}

func runLoadtestWorkers(parent context.Context, client *LoadTestClient, config Config, ticks <-chan time.Time) error {
	if config.TargetTps == 0 || config.TargetTps > uint64(^uint(0)>>1) {
		return fmt.Errorf("invalid target TPS: %d", config.TargetTps)
	}
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	startHeight, err := getLastHeight(ctx, config.BlockchainEndpoint)
	if err != nil {
		return fmt.Errorf("benchmark startup height: %w", err)
	}
	observationMu.Lock()
	BlockHeightsWithTxs = nil
	EvmTxHashes = newEvmTxHashStore(maxRetainedEvmTxHashes)
	observationMu.Unlock()
	configString, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("benchmark config: %w", err)
	}
	fmt.Printf("Running with \n %s \n", configString)
	txQueues := make([]chan SignedTx, len(client.AccountKeys))
	producerRateLimiter := rate.NewLimiter(rate.Limit(config.TargetTps), int(config.TargetTps))
	consumerSemaphore := semaphore.NewWeighted(int64(config.TargetTps))
	var wg sync.WaitGroup
	for i := range txQueues {
		txQueues[i] = make(chan SignedTx, 10)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := client.BuildTxs(ctx, txQueues[i], i, producerRateLimiter, producedCountPerMsgType); err != nil && ctx.Err() == nil {
				cancel(fmt.Errorf("producer %d: %w", i, err))
			}
		}(i)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := client.SendTxs(ctx, txQueues[i], i, sentCountPerMsgType, consumerSemaphore); err != nil && ctx.Err() == nil {
				cancel(fmt.Errorf("consumer %d: %w", i, err))
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		nextHeight := startHeight
		start := time.Now()
		var completed uint64
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-ticks:
				if !ok {
					cancel(errors.New("statistics ticker closed before benchmark completion"))
					return
				}
				heights, times, err := scrapeStatistics(ctx, config.BlockchainEndpoint, &nextHeight)
				if err != nil {
					cancel(fmt.Errorf("statistics scrape: %w", err))
					return
				}
				if len(heights) == 0 {
					fmt.Println("No new committed blocks in reporting interval")
				}
				printStats(start, producedCountPerMsgType, sentCountPerMsgType, prevSentCounterPerMsgType, heights, times)
				start = time.Now()
				for msgType, count := range sentCountPerMsgType {
					atomic.StoreInt64(prevSentCounterPerMsgType[msgType], atomic.LoadInt64(count))
				}
				completed++
				if config.Ticks > 0 && completed >= config.Ticks {
					cancel(errBenchmarkComplete)
					return
				}
			}
		}
	}()
	<-ctx.Done()
	wg.Wait()
	for _, queue := range txQueues {
		close(queue)
	}
	cause := context.Cause(ctx)
	if errors.Is(cause, errBenchmarkComplete) {
		return nil
	}
	return fmt.Errorf("benchmark stopped: %w", cause)
}

func scrapeStatistics(ctx context.Context, endpoint string, nextHeight *int) ([]int, []string, error) {
	currHeight, err := getLastHeight(ctx, endpoint)
	if err != nil {
		return nil, nil, err
	}
	var heights []int
	var times []string
	for *nextHeight <= currHeight {
		height := *nextHeight
		count, blockTime, err := getTxBlockInfo(ctx, endpoint, strconv.Itoa(height))
		if err != nil {
			return heights, times, fmt.Errorf("height %d: %w", height, err)
		}
		heights = append(heights, height)
		times = append(times, blockTime)
		if count > 0 {
			observationMu.Lock()
			BlockHeightsWithTxs = append(BlockHeightsWithTxs, height)
			observationMu.Unlock()
		}
		*nextHeight = height + 1
	}
	return heights, times, nil
}

func printStats(
	startTime time.Time,
	producedCountPerMsgType map[string]*int64,
	sentCountPerMsgType map[string]*int64,
	prevSentPerCounterPerMsgType map[string]*int64,
	blockHeights []int,
	blockTimes []string,
) {
	elapsed := time.Since(startTime)

	totalSent := int64(0)
	totalProduced := int64(0)
	//nolint:gosec
	for msg_type := range sentCountPerMsgType {
		totalSent += atomic.LoadInt64(sentCountPerMsgType[msg_type])
	}
	//nolint:gosec
	for msg_type := range producedCountPerMsgType {
		totalProduced += atomic.LoadInt64(producedCountPerMsgType[msg_type])
	}

	var totalTps float64 = 0
	for msgType := range sentCountPerMsgType {
		sentCount := atomic.LoadInt64(sentCountPerMsgType[msgType])
		prevTotalSent := atomic.LoadInt64(prevSentPerCounterPerMsgType[msgType])
		//nolint:gosec
		tps := float64(sentCount-prevTotalSent) / elapsed.Seconds()
		totalTps += tps
		defer loadtestMetrics.tps.Record(context.Background(), tps, otelmetric.WithAttributes(attribute.String("msg_type", msgType)))
	}

	var totalDuration time.Duration
	var prevTime time.Time
	for i, blockTimeStr := range blockTimes {
		blockTime, _ := time.Parse(time.RFC3339Nano, blockTimeStr)
		if i > 0 {
			duration := blockTime.Sub(prevTime)
			totalDuration += duration
		}
		prevTime = blockTime
	}
	if len(blockTimes)-1 < 1 {

		fmt.Printf("Unable to calculate stats, not enough data. Skipping...\n")
	} else {
		avgDuration := totalDuration.Milliseconds() / int64(len(blockTimes)-1)
		fmt.Printf("High Level - Time Elapsed: %v, Produced: %d, Sent: %d, TPS: %f, Avg Block Time: %d ms\nBlock Heights %v\n\n", elapsed, totalProduced, totalSent, totalTps, avgDuration, blockHeights)
	}
}

// Generate a random message, only generate one admin message per block to prevent acc seq errors
func (c *LoadTestClient) getRandomMessageType(messageTypes []string) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	messageType := messageTypes[r.Intn(len(messageTypes))]
	for c.generatedAdminMessageForBlock && c.isAdminMessageMapping[messageType] {
		messageType = messageTypes[r.Intn(len(messageTypes))]
	}

	if c.isAdminMessageMapping[messageType] {
		c.generatedAdminMessageForBlock = true
	}
	return messageType
}

func (c *LoadTestClient) generateMessage(key cryptotypes.PrivKey, msgType string) ([]sdk.Msg, bool, cryptotypes.PrivKey, uint64, int64) {
	var msgs []sdk.Msg
	config := c.LoadTestConfig
	msgPerTx := config.MsgsPerTx
	signer := key

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	defaultMessageTypeConfig := config.PerMessageConfigs["default"]
	gas := defaultMessageTypeConfig.Gas
	fee := defaultMessageTypeConfig.Fee

	messageTypeConfig, ok := config.PerMessageConfigs[msgType]
	if ok {
		gas = messageTypeConfig.Gas
		fee = messageTypeConfig.Fee
	}

	switch msgType {
	case WasmMintNft:
		contract := config.WasmMsgTypes.MintNftType.ContractAddr
		// TODO: Potentially just hard code the Funds amount here
		price := config.PriceDistr.Sample()
		quantity := config.QuantityDistr.Sample()
		amount, err := sdk.ParseCoinsNormalized(fmt.Sprintf("%d%s", price.Mul(quantity).Ceil().RoundInt64(), "uhpx"))
		if err != nil {
			panic(err)
		}
		msgs = []sdk.Msg{&wasmtypes.MsgExecuteContract{
			Sender:   sdk.AccAddress(key.PubKey().Address()).String(),
			Contract: contract,
			Msg:      wasmtypes.RawContractMessage([]byte("{\"mint\":{\"owner\": \"pax1a27kj2j27c6uz58rn9zmhcjee9s3h3nh458z8m\"}}")),
			Funds:    amount,
		}}
	case WasmInstantiate:
		msgs = []sdk.Msg{&wasmtypes.MsgInstantiateContract{
			Sender: sdk.AccAddress(key.PubKey().Address()).String(),
			CodeID: config.WasmMsgTypes.Instantiate.CodeID,
			Label:  "test",
			Msg:    wasmtypes.RawContractMessage([]byte(config.WasmMsgTypes.Instantiate.Payload)),
			Funds: sdk.NewCoins(sdk.Coin{
				Denom:  "uhpx",
				Amount: sdk.NewInt(1),
			}), // maybe make this configurable as well in the future
		}}
	case Bank:
		msgs = []sdk.Msg{}
		for i := 0; i < int(msgPerTx); i++ { //nolint:gosec

			msgs = append(msgs, &banktypes.MsgSend{
				FromAddress: sdk.AccAddress(key.PubKey().Address()).String(),
				ToAddress:   sdk.AccAddress(key.PubKey().Address()).String(),
				Amount: sdk.NewCoins(sdk.Coin{
					Denom:  "uhpx",
					Amount: sdk.NewInt(1),
				}),
			})
		}
	case DistributeRewards:
		adminKey := c.SignerClient.GetAdminKey()
		msgs = []sdk.Msg{&banktypes.MsgSend{
			FromAddress: sdk.AccAddress(adminKey.PubKey().Address()).String(),
			ToAddress:   sdk.AccAddress(key.PubKey().Address()).String(),
			Amount: sdk.NewCoins(sdk.Coin{
				Denom:  "uhpx",
				Amount: sdk.NewInt(10000000),
			}),
		}}
		signer = adminKey
		gas = 10000000
		fee = 1000000
		fmt.Printf("Distribute rewards to %s \n", sdk.AccAddress(key.PubKey().Address()).String())
	case CollectRewards:
		adminKey := c.SignerClient.GetAdminKey()
		delegatorAddr := sdk.AccAddress(adminKey.PubKey().Address())
		operatorAddress := c.Validators[r.Intn(len(c.Validators))].OperatorAddress
		randomValidatorAddr, err := sdk.ValAddressFromBech32(operatorAddress)
		if err != nil {
			panic(err.Error())
		}
		msgs = []sdk.Msg{distributiontypes.NewMsgWithdrawDelegatorReward(
			delegatorAddr,
			randomValidatorAddr,
		)}
		fmt.Printf("Collecting rewards from %s \n", operatorAddress)
		signer = adminKey
		gas = 10000000
		fee = 1000000
	case Staking:
		delegatorAddr := sdk.AccAddress(key.PubKey().Address()).String()
		chosenValidator := c.Validators[r.Intn(len(c.Validators))].OperatorAddress
		// Randomly pick someone to redelegate / unbond from
		srcAddr := ""
		c.mtx.RLock()
		for k := range c.DelegationMap[delegatorAddr] {
			if k == chosenValidator {
				continue
			}
			srcAddr = k
			break
		}
		c.mtx.RUnlock()
		msgs = []sdk.Msg{c.generateStakingMsg(delegatorAddr, chosenValidator, srcAddr)}
	case Tokenfactory:
		denomCreatorAddr := sdk.AccAddress(key.PubKey().Address()).String()
		// No denoms, let's mint
		randNum := r.Float64()
		denom, ok := c.TokenFactoryDenomOwner[denomCreatorAddr]
		switch {
		case !ok || randNum <= 0.33:
			subDenom := fmt.Sprintf("tokenfactory-created-denom-%d", time.Now().UnixMilli())
			denom = fmt.Sprintf("factory/%s/%s", denomCreatorAddr, subDenom)
			msgs = []sdk.Msg{&tokenfactorytypes.MsgCreateDenom{
				Sender:   denomCreatorAddr,
				Subdenom: subDenom,
			}}
			c.TokenFactoryDenomOwner[denomCreatorAddr] = denom
		case randNum <= 0.66:
			msgs = []sdk.Msg{&tokenfactorytypes.MsgMint{
				Sender: denomCreatorAddr,
				Amount: sdk.Coin{Denom: denom, Amount: sdk.NewInt(10)},
			}}
		default:
			msgs = []sdk.Msg{&tokenfactorytypes.MsgBurn{
				Sender: denomCreatorAddr,
				Amount: sdk.Coin{Denom: denom, Amount: sdk.NewInt(1)},
			}}
		}
	case FailureBankMalformed:
		var denom string
		if r.Float64() < 0.5 {
			denom = "unknown"
		} else {
			denom = "other"
		}
		msgs = []sdk.Msg{&banktypes.MsgSend{
			FromAddress: sdk.AccAddress(key.PubKey().Address()).String(),
			ToAddress:   sdk.AccAddress(key.PubKey().Address()).String(),
			Amount: sdk.NewCoins(sdk.Coin{
				Denom:  denom,
				Amount: sdk.NewInt(1),
			}),
		}}
	case FailureBankInvalid:
		var amountUhpx int64
		amountUhpx = 1000000000000000000
		msgs = []sdk.Msg{&banktypes.MsgSend{
			FromAddress: sdk.AccAddress(key.PubKey().Address()).String(),
			ToAddress:   sdk.AccAddress(key.PubKey().Address()).String(),
			Amount: sdk.NewCoins(sdk.Coin{
				Denom:  "uhpx",
				Amount: sdk.NewInt(amountUhpx),
			}),
		}}
	default:
		fmt.Printf("Unrecognized message type %s", msgType)
	}

	if strings.Contains(msgType, "failure") {
		return msgs, true, signer, gas, int64(fee) // nolint:gosec
	}
	return msgs, false, signer, gas, int64(fee) // nolint:gosec
}

func (c *LoadTestClient) generateStakingMsg(delegatorAddr string, chosenValidator string, srcAddr string) sdk.Msg {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	// Randomly unbond, redelegate or delegate
	// However, if there are no delegations, do so first
	var msg sdk.Msg
	msgType := c.LoadTestConfig.MsgTypeDistr.SampleStakingMsgs()
	if _, ok := c.DelegationMap[delegatorAddr]; !ok || msgType == "delegate" || srcAddr == "" {
		msg = &stakingtypes.MsgDelegate{
			DelegatorAddress: delegatorAddr,
			ValidatorAddress: chosenValidator,
			Amount:           sdk.Coin{Denom: "uhpx", Amount: sdk.NewInt(1)},
		}
		c.DelegationMap[delegatorAddr] = map[string]int{}
		c.DelegationMap[delegatorAddr][chosenValidator] = 1
	} else {
		if msgType == "redelegate" {
			msg = &stakingtypes.MsgBeginRedelegate{
				DelegatorAddress:    delegatorAddr,
				ValidatorSrcAddress: srcAddr,
				ValidatorDstAddress: chosenValidator,
				Amount:              sdk.Coin{Denom: "uhpx", Amount: sdk.NewInt(1)},
			}
			c.DelegationMap[delegatorAddr][chosenValidator]++
		} else {
			msg = &stakingtypes.MsgUndelegate{
				DelegatorAddress: delegatorAddr,
				ValidatorAddress: srcAddr,
				Amount:           sdk.Coin{Denom: "uhpx", Amount: sdk.NewInt(1)},
			}
		}
		// Update delegation map
		c.DelegationMap[delegatorAddr][srcAddr]--
		if c.DelegationMap[delegatorAddr][srcAddr] == 0 {
			delete(c.DelegationMap, delegatorAddr)
		}
	}
	return msg
}

const (
	benchmarkRPCTimeout       = 5 * time.Second
	maxBenchmarkResponseBytes = 16 << 20
	postQueryConcurrency      = 16
)

var benchmarkHTTPClient = &http.Client{Timeout: benchmarkRPCTimeout}

func benchmarkEndpoint(endpoint string, allowWebsocket bool) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}
	if u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https" && !(allowWebsocket && (u.Scheme == "ws" || u.Scheme == "wss"))) {
		return nil, fmt.Errorf("invalid RPC endpoint %q", endpoint)
	}
	return u, nil
}

func readBenchmarkRPC(ctx context.Context, endpoint, path string, query url.Values, target any) error {
	u, err := benchmarkEndpoint(endpoint, false)
	if err != nil {
		return err
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawQuery = query.Encode()
	ctx, cancel := context.WithTimeout(ctx, benchmarkRPCTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := benchmarkHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s: HTTP %s", u, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBenchmarkResponseBytes+1))
	if err != nil {
		return fmt.Errorf("GET %s read: %w", u, err)
	}
	if len(body) > maxBenchmarkResponseBytes {
		return fmt.Errorf("GET %s: response exceeds %d bytes", u, maxBenchmarkResponseBytes)
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int             `json:"code"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("GET %s JSON: %w", u, err)
	}
	if envelope.Error != nil {
		return fmt.Errorf("GET %s RPC %d: %s (%s)", u, envelope.Error.Code, envelope.Error.Message, envelope.Error.Data)
	}
	if envelope.Result != nil {
		body = envelope.Result
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("GET %s payload: %w", u, err)
	}
	return nil
}

func getLastHeight(ctx context.Context, endpoint string) (int, error) {
	var data struct {
		LastHeight *string `json:"last_height"`
	}
	if err := readBenchmarkRPC(ctx, endpoint, "/blockchain", nil, &data); err != nil {
		return 0, err
	}
	if data.LastHeight == nil {
		return 0, errors.New("blockchain response missing last_height")
	}
	height, err := strconv.Atoi(*data.LastHeight)
	if err != nil || height <= 0 || height == int(^uint(0)>>1) {
		return 0, fmt.Errorf("invalid last_height %q", *data.LastHeight)
	}
	return height, nil
}

func getTxBlockInfo(ctx context.Context, endpoint, height string) (int, string, error) {
	var response struct {
		Block *struct {
			Header *struct {
				Height string `json:"height"`
				Time   string `json:"time"`
			} `json:"header"`
			Data *struct {
				Txs json.RawMessage `json:"txs"`
			} `json:"data"`
		} `json:"block"`
	}
	if err := readBenchmarkRPC(ctx, endpoint, "/block", url.Values{"height": {height}}, &response); err != nil {
		return 0, "", err
	}
	if response.Block == nil || response.Block.Header == nil || response.Block.Data == nil || response.Block.Data.Txs == nil {
		return 0, "", errors.New("block response missing block, header, data or txs")
	}
	header := response.Block.Header
	if header.Height != height {
		return 0, "", fmt.Errorf("block height %q differs from requested %s", header.Height, height)
	}
	timestamp, err := time.Parse(time.RFC3339Nano, header.Time)
	if err != nil || timestamp.IsZero() {
		return 0, "", fmt.Errorf("invalid block time %q", header.Time)
	}
	var txs []string
	if err := json.Unmarshal(response.Block.Data.Txs, &txs); err != nil {
		return 0, "", fmt.Errorf("invalid block transactions: %w", err)
	}
	for i, tx := range txs {
		if tx == "" {
			return 0, "", fmt.Errorf("invalid empty block transaction %d", i)
		}
	}
	return len(txs), header.Time, nil
}

func runEvmQueries(config Config) error {
	return runEvmQueriesContext(context.Background(), config)
}

func runEvmQueriesContext(ctx context.Context, config Config) error {
	queries := config.PostTxEvmQueries
	if queries.BlockByNumber < 0 || queries.Receipt < 0 || queries.Filters < 0 {
		return errors.New("negative post-load query count")
	}
	if queries.BlockByNumber == 0 && queries.Receipt == 0 && queries.Filters == 0 {
		return nil
	}
	observationMu.Lock()
	heights := append([]int(nil), BlockHeightsWithTxs...)
	hashes := EvmTxHashes.Snapshot()
	observationMu.Unlock()
	if queries.BlockByNumber > 0 && len(heights) == 0 {
		return errors.New("block queries require observed committed transaction heights")
	}
	if queries.Receipt > 0 && len(hashes) == 0 {
		return errors.New("receipt queries require successfully submitted EVM transactions")
	}
	if queries.Filters > 0 && (config.EVMAddresses == nil || config.EVMAddresses.ERC20 == (common.Address{})) {
		return errors.New("filter queries require a deployed ERC20 address")
	}
	var clients []*ethclient.Client
	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()
	for _, endpoint := range strings.Split(config.EvmRpcEndpoints, ",") {
		u, err := benchmarkEndpoint(endpoint, true)
		if err != nil {
			return err
		}
		dialCtx, cancel := context.WithTimeout(ctx, benchmarkRPCTimeout)
		client, err := ethclient.DialContext(dialCtx, u.String())
		if err == nil {
			clients = append(clients, client)
			_, err = client.NetworkID(dialCtx)
		}
		cancel()
		if err != nil {
			return fmt.Errorf("connect EVM endpoint %s: %w", u, err)
		}
	}
	families := []struct {
		name  string
		count int
		query func(context.Context, *ethclient.Client, int) error
	}{
		{"blocks", queries.BlockByNumber, func(ctx context.Context, c *ethclient.Client, i int) error {
			_, err := c.BlockByNumber(ctx, big.NewInt(int64(heights[i%len(heights)])))
			return err
		}},
		{"receipts", queries.Receipt, func(ctx context.Context, c *ethclient.Client, i int) error {
			_, err := c.TransactionReceipt(ctx, hashes[i%len(hashes)])
			return err
		}},
		{"filter logs", queries.Filters, func(ctx context.Context, c *ethclient.Client, i int) error {
			_, err := c.FilterLogs(ctx, ethereum.FilterQuery{Addresses: []common.Address{config.EVMAddresses.ERC20}})
			return err
		}},
	}
	for _, family := range families {
		if family.count == 0 {
			continue
		}
		start := time.Now()
		familyCtx, cancel := context.WithCancelCause(ctx)
		var wg sync.WaitGroup
		jobs := make(chan int)
		for worker := 0; worker < postQueryConcurrency && worker < family.count; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range jobs {
					queryCtx, stop := context.WithTimeout(familyCtx, benchmarkRPCTimeout)
					err := family.query(queryCtx, clients[i%len(clients)], i)
					stop()
					if err != nil {
						cancel(fmt.Errorf("%s query %d: %w", family.name, i, err))
						return
					}
				}
			}()
		}
	dispatch:
		for i := 0; i < family.count; i++ {
			select {
			case jobs <- i:
			case <-familyCtx.Done():
				break dispatch
			}
		}
		close(jobs)
		wg.Wait()
		err := context.Cause(familyCtx)
		cancel(nil)
		if err != nil {
			return err
		}
		fmt.Printf("Querying %d %s with concurrency <= %d took %fs\n", family.count, family.name, postQueryConcurrency, time.Since(start).Seconds())
	}
	return nil
}

func GetDefaultConfigFilePath() string {
	pwd, _ := os.Getwd()
	return pwd + "/tools/benchmark/loadtest/config.json"
}

func ReadConfig(path string) Config {
	config := Config{}
	file, _ := os.ReadFile(filepath.Clean(path))
	if err := json.Unmarshal(file, &config); err != nil {
		panic(err)
	}
	if err := config.Validate(); err != nil {
		panic(err)
	}
	return config
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "benchmark-ops" {
		ctx, stop := operations.SignalContext()
		err := operations.RunCLI(ctx, os.Args[2:], os.Stdin, os.Stdout, operations.Executables{})
		stop()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	configFilePath := flag.String("config-file", GetDefaultConfigFilePath(), "Path to the config.json file to use for this run")
	flag.Parse()

	config := ReadConfig(*configFilePath)
	fmt.Printf("Using config file: %s\n", *configFilePath)
	run(&config)
}
