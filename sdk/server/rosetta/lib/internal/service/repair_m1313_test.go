package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	ros "github.com/coinbase/rosetta-sdk-go/types"
	"github.com/stretchr/testify/require"

	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta"
	crgerrs "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta/lib/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta/lib/internal/service"
	crgtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta/lib/types"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

type repair1313Hold struct {
	response chan []byte
	release  chan struct{}
}

type repair1313Network struct {
	network   *network.Network
	rpc       *rpchttp.HTTP
	client    *rosetta.Client
	api       crgtypes.API
	netID     *ros.NetworkIdentifier
	key       *secp256k1.PrivKey
	address   sdk.AccAddress
	recipient sdk.AccAddress
	mu        sync.Mutex
	hold      *repair1313Hold
}

func repair1313Start(t *testing.T) *repair1313Network {
	t.Helper()
	cfg := network.DefaultConfig(t)
	cfg.ChainID = "repair-m1313"
	cfg.TimeoutCommit = 100 * time.Millisecond
	cfg.AppConstructor = func(network.Validator) servertypes.Application {
		return app.Setup(t, true, false, false)
	}
	h := &repair1313Network{key: secp256k1.GenPrivKey()}
	h.address = sdk.AccAddress(h.key.PubKey().Address())
	h.recipient = sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	var authGenesis authtypes.GenesisState
	cfg.Codec.MustUnmarshalJSON(cfg.GenesisState[authtypes.ModuleName], &authGenesis)
	accounts, err := authtypes.PackAccounts([]authtypes.GenesisAccount{authtypes.NewBaseAccount(h.address, h.key.PubKey(), 0, 0)})
	require.NoError(t, err)
	authGenesis.Accounts = append(authGenesis.Accounts, accounts...)
	cfg.GenesisState[authtypes.ModuleName] = cfg.Codec.MustMarshalJSON(&authGenesis)
	var bankGenesis banktypes.GenesisState
	cfg.Codec.MustUnmarshalJSON(cfg.GenesisState[banktypes.ModuleName], &bankGenesis)
	bankGenesis.Balances = append(bankGenesis.Balances, banktypes.Balance{
		Address: h.address.String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1000000)),
	})
	cfg.GenesisState[banktypes.ModuleName] = cfg.Codec.MustMarshalJSON(&bankGenesis)
	h.network = network.New(t, cfg)
	t.Cleanup(h.network.Cleanup)
	_, err = h.network.WaitForHeightWithTimeout(2, 30*time.Second)
	require.NoError(t, err)
	val := h.network.Validators[0]
	h.rpc, err = rpchttp.New(val.RPCAddress)
	require.NoError(t, err)
	target, err := url.Parse(strings.Replace(val.RPCAddress, "tcp://", "http://", 1))
	require.NoError(t, err)
	proxy := httputil.NewSingleHostReverseProxy(target)
	// Forward the real node's bytes unchanged, with a barrier after block lookup.
	transport := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))
		var request struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		recorded := httptest.NewRecorder()
		proxy.ServeHTTP(recorded, r)
		if request.Method == "block" || request.Method == "block_by_hash" {
			h.mu.Lock()
			hold := h.hold
			h.hold = nil
			h.mu.Unlock()
			if hold != nil {
				hold.response <- append([]byte(nil), recorded.Body.Bytes()...)
				select {
				case <-hold.release:
				case <-r.Context().Done():
					return
				}
			}
		}
		for key, values := range recorded.Header() {
			w.Header()[key] = values
		}
		w.WriteHeader(recorded.Code)
		_, _ = w.Write(recorded.Body.Bytes())
	}))
	t.Cleanup(transport.Close)
	conf := &rosetta.Config{
		Blockchain: "paxeer", Network: cfg.ChainID,
		TendermintRPC: transport.URL, GRPCEndpoint: val.AppConfig.GRPC.Address,
		Codec: cfg.Codec.(*codec.ProtoCodec), InterfaceRegistry: cfg.InterfaceRegistry,
	}
	h.client, err = rosetta.NewClient(conf)
	require.NoError(t, err)
	require.NoError(t, h.client.Bootstrap())
	h.netID = conf.NetworkIdentifier()
	h.api, err = service.NewOnlineNetwork(h.netID, h.client)
	require.NoError(t, err)
	return h
}

func (h *repair1313Network) send(t *testing.T) *coretypes.ResultTx {
	t.Helper()
	ctx := context.Background()
	options, err := (rosetta.PreprocessOperationsOptionsResponse{
		ExpectedSigners: []string{h.address.String()}, GasLimit: 1000000, GasPrice: "10000uhpx",
	}).ToMetadata()
	require.NoError(t, err)
	meta, err := h.client.ConstructionMetadataFromOptions(ctx, options)
	require.NoError(t, err)
	var construction rosetta.ConstructionMetadata
	require.NoError(t, construction.FromMetadata(meta))
	require.Len(t, construction.SignersData, 1)
	account := construction.SignersData[0]
	txConfig := h.network.Config.TxConfig
	builder := txConfig.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(&banktypes.MsgSend{
		FromAddress: h.address.String(), ToAddress: h.recipient.String(),
		Amount: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 17)),
	}))
	builder.SetGasLimit(1000000)
	builder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10000)))
	sig, err := clienttx.SignWithPrivKey(signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		authsigning.SignerData{ChainID: h.netID.Network, AccountNumber: account.AccountNumber, Sequence: account.Sequence},
		builder, h.key, txConfig, account.Sequence)
	require.NoError(t, err)
	require.NoError(t, builder.SetSignatures(sig))
	raw, err := txConfig.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	broadcast, err := h.rpc.BroadcastTxSync(ctx, raw)
	require.NoError(t, err)
	require.Zero(t, broadcast.Code, broadcast.Log)
	var result *coretypes.ResultTx
	require.Eventually(t, func() bool {
		result, err = h.rpc.Tx(ctx, broadcast.Hash, false)
		return err == nil && result != nil
	}, 20*time.Second, 50*time.Millisecond)
	require.Zero(t, result.TxResult.Code, result.TxResult.Log)
	return result
}

func TestRepairM_1313_BalanceHeight(t *testing.T) {
	h := repair1313Start(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, name := range []string{"latest", "empty", "hash", "height", "zero-height", "genesis-height"} {
		t.Run(name, func(t *testing.T) {
			latest, err := h.client.BlockByHeight(ctx, nil)
			require.NoError(t, err)
			var identifier *ros.PartialBlockIdentifier
			switch name {
			case "empty":
				identifier = &ros.PartialBlockIdentifier{}
			case "hash":
				identifier = &ros.PartialBlockIdentifier{Hash: &latest.Block.Hash}
			case "height":
				identifier = &ros.PartialBlockIdentifier{Index: &latest.Block.Index}
			case "zero-height":
				height := int64(0)
				identifier = &ros.PartialBlockIdentifier{Index: &height}
			case "genesis-height":
				height := int64(-1)
				identifier = &ros.PartialBlockIdentifier{Index: &height}
			}
			hold := &repair1313Hold{response: make(chan []byte, 1), release: make(chan struct{})}
			var release sync.Once
			unblock := func() { release.Do(func() { close(hold.release) }) }
			defer unblock()
			h.mu.Lock()
			h.hold = hold
			h.mu.Unlock()
			type balanceResult struct {
				response *ros.AccountBalanceResponse
				err      *ros.Error
			}
			finished := make(chan balanceResult, 1)
			go func() {
				response, err := h.api.AccountBalance(ctx, &ros.AccountBalanceRequest{
					NetworkIdentifier: h.netID, AccountIdentifier: &ros.AccountIdentifier{Address: h.address.String()},
					BlockIdentifier: identifier,
				})
				finished <- balanceResult{response: response, err: err}
			}()
			var raw []byte
			select {
			case raw = <-hold.response:
			case <-ctx.Done():
				t.Fatal("block lookup did not reach the transport barrier")
			}
			var envelope struct {
				Result json.RawMessage `json:"result"`
			}
			require.NoError(t, json.Unmarshal(raw, &envelope))
			var captured coretypes.ResultBlock
			require.NoError(t, tmjson.Unmarshal(envelope.Result, &captured))
			require.NotNil(t, captured.Block)
			height := captured.Block.Height
			expected, err := h.client.Balances(ctx, h.address.String(), &height)
			require.NoError(t, err)
			require.NotEmpty(t, expected)
			committed := h.send(t)
			require.Greater(t, committed.Height, height)
			current, err := h.client.Balances(ctx, h.address.String(), &committed.Height)
			require.NoError(t, err)
			require.NotEqual(t, expected, current, "the real bank state must change before releasing lookup")
			unblock()
			select {
			case result := <-finished:
				require.Nil(t, result.err)
				require.NotNil(t, result.response)
				require.Equal(t, height, result.response.BlockIdentifier.Index)
				require.Equal(t, captured.BlockID.Hash.String(), result.response.BlockIdentifier.Hash)
				require.Equal(t, expected, result.response.Balances)
			case <-ctx.Done():
				t.Fatal("balance lookup did not finish")
			}
		})
	}
}

func TestRepairM_1313_BlockMembership(t *testing.T) {
	h := repair1313Start(t)
	ctx := context.Background()
	committed := h.send(t)
	block, err := h.client.BlockTransactionsByHeight(ctx, &committed.Height)
	require.NoError(t, err)
	require.Len(t, block.Transactions, 3, "begin, ordinary and end transactions must all be present")
	_, err = h.network.WaitForHeightWithTimeout(committed.Height+1, 20*time.Second)
	require.NoError(t, err)
	height := committed.Height + 1
	other, err := h.client.BlockByHeight(ctx, &height)
	require.NoError(t, err)
	require.NotEqual(t, block.Block.Hash, other.Block.Hash)
	for i, tx := range block.Transactions {
		t.Run([]string{"begin", "ordinary", "end"}[i], func(t *testing.T) {
			// Establish that the transaction really exists independently of membership lookup.
			existing, err := h.client.GetTx(ctx, tx.TransactionIdentifier.Hash)
			require.NoError(t, err)
			require.Equal(t, tx, existing)
			response, rosErr := h.api.BlockTransaction(ctx, &ros.BlockTransactionRequest{
				NetworkIdentifier: h.netID, BlockIdentifier: block.Block, TransactionIdentifier: tx.TransactionIdentifier,
			})
			require.Nil(t, rosErr)
			require.Equal(t, tx, response.Transaction)
			for _, requested := range []*ros.BlockIdentifier{
				other.Block,
				{Hash: block.Block.Hash, Index: other.Block.Index},
				{Hash: other.Block.Hash, Index: block.Block.Index},
			} {
				response, rosErr := h.api.BlockTransaction(ctx, &ros.BlockTransactionRequest{
					NetworkIdentifier: h.netID, BlockIdentifier: requested, TransactionIdentifier: tx.TransactionIdentifier,
				})
				require.Nil(t, response)
				require.NotNil(t, rosErr)
				require.Equal(t, crgerrs.ToRosetta(crgerrs.ErrNotFound).Code, rosErr.Code)
			}
		})
	}
	response, rosErr := h.api.BlockTransaction(ctx, &ros.BlockTransactionRequest{
		NetworkIdentifier: h.netID, BlockIdentifier: block.Block,
		TransactionIdentifier: &ros.TransactionIdentifier{Hash: strings.Repeat("AB", 32)},
	})
	require.Nil(t, response)
	require.NotNil(t, rosErr)
	require.Equal(t, crgerrs.ToRosetta(crgerrs.ErrNotFound).Code, rosErr.Code)
}
