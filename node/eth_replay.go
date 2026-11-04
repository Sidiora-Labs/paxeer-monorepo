package app

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/utils"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethcore "github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
	ethtests "github.com/ethereum/go-ethereum/tests"
	"github.com/holiman/uint256"
)

type ReplayOptions struct {
	RequestTimeout time.Duration
	RetryDelay     time.Duration
	PollInterval   time.Duration
	MaxRetries     int
}

var DefaultReplayOptions = ReplayOptions{RequestTimeout: 30 * time.Second, RetryDelay: time.Second, PollInterval: time.Minute, MaxRetries: 3}

// Replay retains the daemon entry point and cancels outstanding requests on shutdown.
func Replay(a *App) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := ReplayContext(ctx, a, DefaultReplayOptions); err != nil && !errors.Is(err, context.Canceled) {
		panic(err)
	}
}

type replayTransport struct {
	ctx  context.Context
	base http.RoundTripper
}

func (t replayTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(t.ctx, cancel)
	response, err := t.base.RoundTrip(req.Clone(ctx))
	if err != nil {
		stop()
		cancel()
		return nil, err
	}
	response.Body = &replayResponseBody{ReadCloser: response.Body, release: func() { stop(); cancel() }}
	return response, nil
}

type replayResponseBody struct {
	io.ReadCloser
	release func()
}

func (b *replayResponseBody) Close() error { defer b.release(); return b.ReadCloser.Close() }

func replayRetriable(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.EPIPE) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, ethereum.NotFound) {
		return true
	}
	var network net.Error
	if errors.As(err, &network) {
		return network.Timeout() || network.Temporary()
	}
	var response ethrpc.HTTPError
	if errors.As(err, &response) {
		return response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusRequestTimeout || response.StatusCode >= 500
	}
	var rpcError ethrpc.Error
	if errors.As(err, &rpcError) {
		return rpcError.ErrorCode() == -32005 || rpcError.ErrorCode() == -32002
	}
	return false
}

func replayWait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func replayRequest[T any](ctx context.Context, opts ReplayOptions, request func(context.Context) (T, error)) (T, error) {
	for attempt := 0; ; attempt++ {
		var zero T
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		requestCtx, cancel := context.WithTimeout(ctx, opts.RequestTimeout)
		result, err := request(requestCtx)
		cancel()
		if err == nil {
			return result, nil
		}
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}
		if !replayRetriable(err) || attempt >= opts.MaxRetries {
			return zero, err
		}
		if err := replayWait(ctx, opts.RetryDelay); err != nil {
			return zero, err
		}
	}
}

// ReplayContext resumes only from committed keeper state. Failed RPCs never
// publish a cursor; after execution starts, failure returns to the owner for restart.
func ReplayContext(ctx context.Context, a *App, opts ReplayOptions) error {
	if opts.RequestTimeout <= 0 || opts.RetryDelay <= 0 || opts.PollInterval <= 0 || opts.MaxRetries < 0 {
		return fmt.Errorf("invalid replay options")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dialCtx, cancelDial := context.WithTimeout(ctx, opts.RequestTimeout)
	rpcClient, err := ethrpc.DialOptions(dialCtx, a.EvmKeeper.EthReplayConfig.EthRPC, ethrpc.WithHTTPClient(&http.Client{
		Timeout: opts.RequestTimeout, Transport: replayTransport{ctx: ctx, base: http.DefaultTransport},
	}))
	cancelDial()
	if err != nil {
		return fmt.Errorf("replay RPC: %w", err)
	}
	previousClient := a.EvmKeeper.EthClient
	a.EvmKeeper.EthClient = ethclient.NewClient(rpcClient)
	stopClient := context.AfterFunc(ctx, rpcClient.Close)
	defer func() { stopClient(); rpcClient.Close(); a.EvmKeeper.EthClient = previousClient }()
	h := a.EvmKeeper.GetReplayedHeight(a.GetCheckCtx()) + 1
	initHeight := a.EvmKeeper.GetReplayInitialHeight(a.GetCheckCtx())
	if h == 1 {
		gendoc, err := tmtypes.GenesisDocFromFile(filepath.Join(DefaultNodeHome, "config/genesis.json"))
		if err != nil {
			return err
		}
		_, err = a.InitChain(ctx, &abci.RequestInitChain{Time: time.Now(), ChainId: gendoc.ChainID, AppStateBytes: gendoc.AppState})
		if err != nil {
			return err
		}
		initHeight = a.EvmKeeper.GetReplayInitialHeight(a.GetContextForDeliverTx(nil))
	}
	for {
		latestBlock, err := replayRequest(ctx, opts, a.EvmKeeper.EthClient.BlockNumber)
		if err != nil {
			return fmt.Errorf("replay head at committed height %d: %w", h-1, err)
		}
		if latestBlock < uint64(h+initHeight) {
			if err := replayWait(ctx, opts.PollInterval); err != nil {
				return err
			}
			continue
		}
		b, err := replayRequest(ctx, opts, func(requestCtx context.Context) (*ethtypes.Block, error) {
			return a.EvmKeeper.EthClient.BlockByNumber(requestCtx, big.NewInt(h+initHeight))
		})
		if err != nil {
			return fmt.Errorf("replay block %d: %w", h+initHeight, err)
		}
		if (h+initHeight >= 19426587) != (evmtypes.DefaultChainConfig().CancunTime >= 0) {
			return fmt.Errorf("replay height %d disagrees with configured Cancun activation", h+initHeight)
		}
		if a.EvmKeeper.DB == nil {
			a.EvmKeeper.OpenEthDatabase()
		}
		if err := replayBlock(ctx, a, b, h, opts.RequestTimeout); err != nil {
			return err
		}
		h++
	}
}

func replayBlock(ctx context.Context, a *App, b *ethtypes.Block, height int64, timeout time.Duration) (err error) {
	blockCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	stopClient := context.AfterFunc(blockCtx, a.EvmKeeper.EthClient.Client().Close)
	defer stopClient()
	defer func() {
		if r := recover(); r != nil {
			if blockCtx.Err() != nil {
				err = blockCtx.Err()
				return
			}
			if rpcErr, ok := r.(error); ok && replayRetriable(rpcErr) {
				err = rpcErr
				return
			}
			panic(r)
		}
	}()
	a.EvmKeeper.ReplayBlock = b
	hash := make([]byte, 8)
	binary.BigEndian.PutUint64(hash, uint64(height))
	_, err = a.FinalizeBlock(blockCtx, &abci.RequestFinalizeBlock{
		Txs:               utils.Map(b.Txs, func(tx *ethtypes.Transaction) []byte { return encodeTx(tx, a.GetTxConfig()) }),
		DecidedLastCommit: abci.CommitInfo{Votes: []abci.VoteInfo{}}, Hash: hash,
		Header: &tmproto.Header{ChainID: a.ChainID, Height: height, Time: time.Now()},
	})
	if err != nil {
		return err
	}
	stateCtx := a.GetContextForDeliverTx(nil).WithContext(blockCtx)
	s := state.NewDBImpl(stateCtx, &a.EvmKeeper, false)
	for _, w := range b.Withdrawals() {
		amount := new(big.Int).Mul(new(big.Int).SetUint64(w.Amount), big.NewInt(params.GWei))
		s.AddBalance(w.Address, uint256.MustFromBig(amount), tracing.BalanceIncreaseWithdrawal)
	}
	if _, err := s.Finalize(); err != nil {
		return err
	}
	for _, tx := range b.Txs {
		if tx.To() != nil {
			a.EvmKeeper.VerifyBalance(stateCtx, *tx.To())
			if a.EvmKeeper.EthReplayConfig.ContractStateChecks {
				a.EvmKeeper.VerifyState(stateCtx, *tx.To())
			}
		}
		a.EvmKeeper.VerifyTxResult(stateCtx, tx.Hash())
	}
	if err := blockCtx.Err(); err != nil {
		return err
	}
	_, err = a.Commit(blockCtx)
	return err
}

func BlockTest(a *App, bt *ethtests.BlockTest, home string) {
	a.EvmKeeper.BlockTest = bt
	a.EvmKeeper.EthBlockTestConfig.Enabled = true

	gendoc, err := tmtypes.GenesisDocFromFile(filepath.Join(home, "config/genesis.json"))
	if err != nil {
		panic(err)
	}
	_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
		Time:          time.Now(),
		ChainId:       gendoc.ChainID,
		AppStateBytes: gendoc.AppState,
	})
	if err != nil {
		panic(err)
	}

	ethblocks := make([]*ethtypes.Block, len(bt.Json.Blocks))
	for i, btBlock := range bt.Json.Blocks {
		b, err := btBlock.Decode()
		if err != nil {
			panic(err)
		}
		ethblocks[i] = b
	}
	if bf := ethblocks[0].BaseFee(); bf != nil {
		a.EvmKeeper.SetCurrBaseFeePerGas(a.GetContextForDeliverTx([]byte{}), sdk.NewDecFromBigInt(bf))
	} else {
		a.EvmKeeper.SetCurrBaseFeePerGas(a.GetContextForDeliverTx([]byte{}), sdk.ZeroDec())
	}
	for addr, genesisAccount := range a.EvmKeeper.BlockTest.Json.Pre {
		uhpx, wei := state.SplitUhpxWeiAmount(genesisAccount.Balance)
		paxAddr := a.EvmKeeper.GetPaxAddressOrDefault(a.GetContextForDeliverTx([]byte{}), addr)
		err := a.EvmKeeper.BankKeeper().AddCoins(a.GetContextForDeliverTx([]byte{}), paxAddr, sdk.NewCoins(sdk.NewCoin("uhpx", uhpx)), true)
		if err != nil {
			panic(err)
		}
		err = a.EvmKeeper.BankKeeper().AddWei(a.GetContextForDeliverTx([]byte{}), a.EvmKeeper.GetPaxAddressOrDefault(a.GetContextForDeliverTx([]byte{}), addr), wei)
		if err != nil {
			panic(err)
		}
		a.EvmKeeper.SetNonce(a.GetContextForDeliverTx([]byte{}), addr, genesisAccount.Nonce)
		a.EvmKeeper.SetCode(a.GetContextForDeliverTx([]byte{}), addr, genesisAccount.Code)
		for key, value := range genesisAccount.Storage {
			a.EvmKeeper.SetState(a.GetContextForDeliverTx([]byte{}), addr, key, value)
		}
		params := a.EvmKeeper.GetParams(a.GetContextForDeliverTx([]byte{}))
		params.MinimumFeePerGas = sdk.NewDecFromInt(sdk.NewInt(0))
		a.EvmKeeper.SetParams(a.GetContextForDeliverTx([]byte{}), params)
	}

	if len(bt.Json.Blocks) == 0 {
		panic("no blocks found")
	}

	for i, btBlock := range bt.Json.Blocks {
		h := int64(i + 1)
		b, err := btBlock.Decode()
		if err != nil {
			panic(err)
		}
		blockHash := b.Hash()
		parentHash := b.ParentHash()
		txs := utils.Map(b.Txs, func(tx *ethtypes.Transaction) []byte { return encodeTx(tx, a.GetTxConfig()) })
		_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
			Txs:               txs,
			Hash:              blockHash[:],
			DecidedLastCommit: abci.CommitInfo{Votes: []abci.VoteInfo{}},
			Header: &tmproto.Header{
				ChainID:         gendoc.ChainID,
				ProposerAddress: a.EvmKeeper.GetPaxAddressOrDefault(a.GetCheckCtx(), b.Coinbase()),
				LastBlockId:     tmproto.BlockID{Hash: parentHash[:]},
				Height:          h,
				Time:            time.Now(),
			},
		})
		if err != nil {
			panic(err)
		}
		if i+1 < len(ethblocks) {
			if bf := ethblocks[i+1].BaseFee(); bf != nil {
				a.EvmKeeper.SetCurrBaseFeePerGas(a.GetContextForDeliverTx([]byte{}), sdk.NewDecFromBigInt(bf))
			} else {
				a.EvmKeeper.SetCurrBaseFeePerGas(a.GetContextForDeliverTx([]byte{}), sdk.ZeroDec())
			}
		}
		_, err = a.Commit(context.Background())
		if err != nil {
			panic(err)
		}
	}

	// Check post-state after all blocks are run
	ctx := a.GetCheckCtx()
	for addr, accountData := range bt.Json.Post {
		if IsWithdrawalAddress(addr, ethblocks) {
			fmt.Println("Skipping withdrawal address: ", addr)
			continue
		}
		// Not checking compliance with EIP-4788
		if addr == params.BeaconRootsAddress {
			fmt.Println("Skipping beacon roots storage address: ", addr)
			continue
		}
		a.EvmKeeper.VerifyAccount(ctx, addr, accountData)
		fmt.Println("Successfully verified account: ", addr)
	}
}

func encodeTx(tx *ethtypes.Transaction, txConfig client.TxConfig) []byte {
	var txData ethtx.EthereumTxData
	var err error
	switch tx.Type() {
	case ethtypes.LegacyTxType:
		txData, err = ethtx.NewLegacyTx(tx)
	case ethtypes.DynamicFeeTxType:
		txData, err = ethtx.NewDynamicFeeTx(tx)
	case ethtypes.AccessListTxType:
		txData, err = ethtx.NewAccessListTx(tx)
	case ethtypes.BlobTxType:
		txData, err = ethtx.NewBlobTx(tx)
	}
	if err != nil {
		if strings.Contains(err.Error(), ethcore.ErrTipAboveFeeCap.Error()) {
			return nil
		}
		panic(err)
	}
	msg, err := evmtypes.NewMsgEVMTransaction(txData)
	if err != nil {
		panic(err)
	}
	txBuilder := txConfig.NewTxBuilder()
	if err = txBuilder.SetMsgs(msg); err != nil {
		panic(err)
	}
	txbz, encodeErr := txConfig.TxEncoder()(txBuilder.GetTx())
	if encodeErr != nil {
		panic(encodeErr)
	}
	return txbz
}

func IsWithdrawalAddress(addr common.Address, blocks []*ethtypes.Block) bool {
	for _, block := range blocks {
		for _, w := range block.Withdrawals() {
			if w.Address == addr {
				return true
			}
		}
	}
	return false
}
