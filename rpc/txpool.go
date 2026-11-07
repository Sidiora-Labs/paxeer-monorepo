package evmrpc

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/rpc/rpcutils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/export"
	"github.com/ethereum/go-ethereum/rpc"
)

type TxPoolAPI struct {
	tmClient         client.LocalClient
	keeper           *keeper.Keeper
	ctxProvider      func(int64) sdk.Context
	txConfigProvider func(int64) client.TxConfig
	txPoolConfig     *TxPoolConfig
	connectionType   ConnectionType
}

type TxPoolConfig struct {
	maxNumTxs int
}

// NewTxPoolConfig creates a new TxPoolConfig primarily for tests.
func NewTxPoolConfig(maxNumTxs int) *TxPoolConfig {
	return &TxPoolConfig{maxNumTxs: maxNumTxs}
}

func NewTxPoolAPI(tmClient client.LocalClient, k *keeper.Keeper, ctxProvider func(int64) sdk.Context, txConfigProvider func(int64) client.TxConfig, txPoolConfig *TxPoolConfig, connectionType ConnectionType) *TxPoolAPI {
	return &TxPoolAPI{tmClient: tmClient, keeper: k, ctxProvider: ctxProvider, txConfigProvider: txConfigProvider, txPoolConfig: txPoolConfig, connectionType: connectionType}
}

// Content returns the content of the txpool.
// for now, we put all unconfirmed txs in pending and none in queued.
func (t *TxPoolAPI) Content(ctx context.Context) (result map[string]map[string]map[string]*export.RPCTransaction, returnErr error) {
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "txpool_content", t.connectionType, startTime, returnErr, recover())
	}()
	content := map[string]map[string]map[string]*export.RPCTransaction{
		"pending": make(map[string]map[string]*export.RPCTransaction),
		"queued":  make(map[string]map[string]*export.RPCTransaction),
	}

	total := t.txPoolConfig.maxNumTxs
	resUnconfirmedTxs, err := t.tmClient.UnconfirmedTxs(ctx, nil, &total)
	if err != nil {
		return nil, err
	}

	sdkCtx := t.ctxProvider(LatestCtxHeight)
	for _, tx := range resUnconfirmedTxs.Txs {
		ethTx := getEthTxForTxBz(tx, t.txConfigProvider(LatestCtxHeight).TxDecoder())
		if ethTx == nil || !domainAllowsTransaction(ctx, keeperDomainIndex{evm: t.keeper}, ethTx) { // not an evm tx
			continue
		}
		fromAddr, err := rpcutils.RecoverEVMSenderWithContext(sdkCtx, ethTx)
		if err != nil {
			return nil, err
		}
		nonce := ethTx.Nonce()
		chainConfig := types.DefaultChainConfig().EthereumConfig(t.keeper.ChainID(sdkCtx))
		res := export.NewRPCPendingTransaction(ethTx, nil, chainConfig)
		res.From = fromAddr
		nonceStr := strconv.FormatUint(nonce, 10)
		if content["pending"][fromAddr.String()] == nil {
			content["pending"][fromAddr.String()] = map[string]*export.RPCTransaction{
				nonceStr: res,
			}
		} else {
			content["pending"][fromAddr.String()][nonceStr] = res
		}
	}
	return content, nil
}

func (t *TxPoolAPI) Status(ctx context.Context) (result map[string]hexutil.Uint, returnErr error) {
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "txpool_status", t.connectionType, startTime, returnErr, recover())
	}()

	total := t.txPoolConfig.maxNumTxs
	resUnconfirmedTxs, err := t.tmClient.UnconfirmedTxs(ctx, nil, &total)
	if err != nil {
		return nil, err
	}

	var pending hexutil.Uint
	for _, tx := range resUnconfirmedTxs.Txs {
		ethTx := getEthTxForTxBz(tx, t.txConfigProvider(LatestCtxHeight).TxDecoder())
		if ethTx == nil || !domainAllowsTransaction(ctx, keeperDomainIndex{evm: t.keeper}, ethTx) {
			continue
		}
		pending++
	}
	return map[string]hexutil.Uint{"pending": pending, "queued": 0}, nil
}

// domainTxPoolAPI retains the connection scope on WebSocket calls.
type domainTxPoolAPI struct {
	*TxPoolAPI
	scope DomainScope
}

func (t *domainTxPoolAPI) Content(ctx context.Context) (map[string]map[string]map[string]*export.RPCTransaction, error) {
	return t.TxPoolAPI.Content(WithDomainScope(ctx, t.scope))
}

func (t *domainTxPoolAPI) Status(ctx context.Context) (map[string]hexutil.Uint, error) {
	return t.TxPoolAPI.Status(WithDomainScope(ctx, t.scope))
}

func installTxPoolWSScopes(server *HTTPServer) {
	handler := server.wsHandler.Load().(*rpcHandler)
	config := server.WsConfig
	baseAPIs := append([]rpc.API(nil), server.wsAPIs...)
	handler.Handler = handler.limiter.wrap(NewWSHandlerStack(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apis := baseAPIs
		if scope, ok := DomainScopeFrom(r.Context()); ok {
			apis = append([]rpc.API(nil), apis...)
			for i, api := range apis {
				switch service := api.Service.(type) {
				case *TxPoolAPI:
					apis[i].Service = &domainTxPoolAPI{TxPoolAPI: service, scope: scope}
				case *PaxTransactionAPI:
					apis[i].Service = &domainPaxTransactionAPI{PaxTransactionAPI: service, scope: scope}
				}
			}
		}
		server.newDomainWSHandler(handler.server, config, apis).ServeHTTP(w, r)
	}), config.JwtSecret))
}
