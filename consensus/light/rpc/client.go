package rpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	tmbytes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/bytes"
	tmmath "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/math"
	service "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/service"
	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// KeyPathFunc builds a merkle path out of the given path and key.
type KeyPathFunc func(path string, key []byte) (merkle.KeyPath, error)

// LightClient is an interface that contains functionality needed by Client from the light client.
//
//go:generate ../../scripts/mockery_generate.sh LightClient
type LightClient interface {
	ChainID() string
	Update(ctx context.Context, now time.Time) (*types.LightBlock, error)
	VerifyLightBlockAtHeight(ctx context.Context, height int64, now time.Time) (*types.LightBlock, error)
	TrustedLightBlock(height int64) (*types.LightBlock, error)
	Status(ctx context.Context) *types.LightClientInfo
}

var _ rpcclient.Client = (*Client)(nil)

// Client is an RPC client, which uses light#Client to verify data (if it can
// be proved). Note, merkle.DefaultProofRuntime is used to verify values
// returned by ABCI#Query.
type Client struct {
	service.BaseService

	next rpcclient.Client
	lc   LightClient

	// proof runtime used to verify values returned by ABCIQuery
	prt       *merkle.ProofRuntime
	keyPathFn KeyPathFunc

	subscriptionsMu sync.Mutex
	subscriptions   map[wsSubscriptionKey]*wsSubscription
	closers         []func()
	stopped         bool
}

type wsSubscriptionKey struct {
	subscriber string
	query      string
}

type wsSubscription struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Option allow you to tweak Client.
type Option func(*Client)

// KeyPathFn option can be used to set a function, which parses a given path
// and builds the merkle path for the prover. It must be provided if you want
// to call ABCIQuery or ABCIQueryWithOptions.
func KeyPathFn(fn KeyPathFunc) Option {
	return func(c *Client) {
		c.keyPathFn = fn
	}
}

// DefaultMerkleKeyPathFn creates a function used to generate merkle key paths
// from a path string and a key. This is the default used by the cosmos SDK.
// This merkle key paths are required when verifying /abci_query calls
func DefaultMerkleKeyPathFn() KeyPathFunc {
	// regexp for extracting store name from /abci_query path
	storeNameRegexp := regexp.MustCompile(`^/store/([^/]+)/key$`)

	return func(path string, key []byte) (merkle.KeyPath, error) {
		matches := storeNameRegexp.FindStringSubmatch(path)
		if len(matches) != 2 {
			return nil, fmt.Errorf("can't find store name in %s using %s", path, storeNameRegexp)
		}
		storeName := matches[1]

		kp := merkle.KeyPath{}
		kp = kp.AppendKey([]byte(storeName), merkle.KeyEncodingURL)
		kp = kp.AppendKey(key, merkle.KeyEncodingURL)
		return kp, nil
	}
}

// NewClient returns a new client.
func NewClient(next rpcclient.Client, lc LightClient, opts ...Option) *Client {
	c := &Client{
		next: next,
		lc:   lc,
		prt:  merkle.DefaultProofRuntime(),
	}
	c.BaseService = *service.NewBaseService("Client", c)
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *Client) OnStart(ctx context.Context) error {
	c.subscriptionsMu.Lock()
	defer c.subscriptionsMu.Unlock()
	nctx, ncancel := context.WithCancel(ctx)
	if err := c.next.Start(nctx); err != nil {
		ncancel()
		return err
	}
	c.closers = append(c.closers, ncancel)

	return nil
}

func (c *Client) OnStop() {
	c.subscriptionsMu.Lock()
	c.stopped = true
	var pending []*wsSubscription
	for key, sub := range c.subscriptions {
		sub.cancel()
		delete(c.subscriptions, key)
		pending = append(pending, sub)
	}
	for _, closer := range c.closers {
		closer()
	}
	c.closers = nil
	c.subscriptionsMu.Unlock()
	for _, sub := range pending {
		<-sub.done
	}
}

// Status reports the configured chain identity and local light-client trust state.
// Full-node sync and validator information are not available from a light client.
func (c *Client) Status(ctx context.Context) (*coretypes.ResultStatus, error) {
	lightClientInfo := c.lc.Status(ctx)

	return &coretypes.ResultStatus{
		NodeInfo:        types.NodeInfo{Network: c.lc.ChainID()},
		SyncInfo:        coretypes.SyncInfo{},
		ValidatorInfo:   coretypes.ValidatorInfo{},
		LightClientInfo: *lightClientInfo,
	}, nil
}

// LagStatus  return 0 for lag status
func (c *Client) LagStatus(ctx context.Context) (*coretypes.ResultLagStatus, error) {
	return &coretypes.ResultLagStatus{}, nil
}

func (c *Client) ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error) {
	return c.next.ABCIInfo(ctx)
}

// ABCIQuery requests proof by default.
func (c *Client) ABCIQuery(ctx context.Context, path string, data tmbytes.HexBytes) (*coretypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(ctx, path, data, rpcclient.DefaultABCIQueryOptions)
}

// ABCIQueryWithOptions returns an error if opts.Prove is false.
// ABCIQueryWithOptions returns the result for the given height (opts.Height).
// If no height is provided, the results of the block preceding the latest are returned.
func (c *Client) ABCIQueryWithOptions(ctx context.Context, path string, data tmbytes.HexBytes,
	opts rpcclient.ABCIQueryOptions) (*coretypes.ResultABCIQuery, error) {

	// always request the proof
	opts.Prove = true

	// Can't return the latest block results because we won't be able to
	// prove them. Return the results for the previous block instead.
	if opts.Height == 0 {
		res, err := c.next.Status(ctx)
		if err != nil {
			return nil, fmt.Errorf("can't get latest height: %w", err)
		}
		opts.Height = res.SyncInfo.LatestBlockHeight - 1
	}

	res, err := c.next.ABCIQueryWithOptions(ctx, path, data, opts)
	if err != nil {
		return nil, err
	}
	resp := res.Response

	// Validate the response.
	if resp.IsErr() {
		return nil, fmt.Errorf("err response code: %v", resp.Code)
	}
	if len(resp.Key) == 0 {
		return nil, errors.New("empty key")
	}
	if !bytes.Equal(resp.Key, data) {
		return nil, errors.New("response key does not match requested key")
	}
	if resp.ProofOps == nil || len(resp.ProofOps.Ops) == 0 {
		return nil, errors.New("no proof ops")
	}
	if resp.Height <= 0 {
		return nil, coretypes.ErrZeroOrNegativeHeight
	}

	if resp.Height != opts.Height {
		return nil, fmt.Errorf("response height %d does not match requested height %d", resp.Height, opts.Height)
	}

	// Update the light client if we're behind.
	// NOTE: AppHash for height H is in header H+1.
	nextHeight := resp.Height + 1
	l, err := c.updateLightClientIfNeededTo(ctx, &nextHeight)
	if err != nil {
		return nil, err
	}

	// Validate the value proof against the trusted header.

	// Derive the authenticated key path from the request, never provider fields.
	if c.keyPathFn == nil {
		return nil, errors.New("please configure Client with KeyPathFn option")
	}

	kp, err := c.keyPathFn(path, data)
	if err != nil {
		return nil, fmt.Errorf("can't build merkle key path: %w", err)
	}

	// verify value
	if resp.Value != nil {
		err = c.prt.VerifyValue(resp.ProofOps, l.AppHash, kp.String(), resp.Value)
		if err != nil {
			return nil, fmt.Errorf("verify value proof: %w", err)
		}
	} else { // OR validate the absence proof against the trusted header.
		err = c.prt.VerifyAbsence(resp.ProofOps, l.AppHash, kp.String())
		if err != nil {
			return nil, fmt.Errorf("verify absence proof: %w", err)
		}
	}

	return &coretypes.ResultABCIQuery{Response: resp}, nil
}

func (c *Client) BroadcastTxCommit(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTxCommit, error) {
	return c.next.BroadcastTxCommit(ctx, tx)
}

func (c *Client) BroadcastTxAsync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	return c.next.BroadcastTxAsync(ctx, tx)
}

func (c *Client) BroadcastTxSync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	return c.next.BroadcastTxSync(ctx, tx)
}

func (c *Client) BroadcastTx(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
	return c.next.BroadcastTx(ctx, tx)
}

func (c *Client) UnconfirmedTxs(ctx context.Context, page, perPage *int) (*coretypes.ResultUnconfirmedTxs, error) {
	return c.next.UnconfirmedTxs(ctx, page, perPage)
}

func (c *Client) NumUnconfirmedTxs(ctx context.Context) (*coretypes.ResultUnconfirmedTxs, error) {
	return c.next.NumUnconfirmedTxs(ctx)
}

func (c *Client) CheckTx(ctx context.Context, tx types.Tx) (*coretypes.ResultCheckTx, error) {
	return c.next.CheckTx(ctx, tx)
}

func (c *Client) NetInfo(ctx context.Context) (*coretypes.ResultNetInfo, error) {
	return c.next.NetInfo(ctx)
}

func (c *Client) DumpConsensusState(ctx context.Context) (*coretypes.ResultDumpConsensusState, error) {
	return c.next.DumpConsensusState(ctx)
}

func (c *Client) ConsensusState(ctx context.Context) (*coretypes.ResultConsensusState, error) {
	return c.next.ConsensusState(ctx)
}

func (c *Client) ConsensusParams(ctx context.Context, height *int64) (*coretypes.ResultConsensusParams, error) {
	res, err := c.next.ConsensusParams(ctx, height)
	if err != nil {
		return nil, err
	}

	// Validate res.
	if err := res.ConsensusParams.ValidateConsensusParams(); err != nil {
		return nil, err
	}
	if res.BlockHeight <= 0 {
		return nil, coretypes.ErrZeroOrNegativeHeight
	}

	// Update the light client if we're behind.
	l, err := c.updateLightClientIfNeededTo(ctx, &res.BlockHeight)
	if err != nil {
		return nil, err
	}

	// Verify hash.
	if cH, tH := res.ConsensusParams.HashConsensusParams(), l.ConsensusHash; !bytes.Equal(cH, tH) {
		return nil, fmt.Errorf("params hash %X does not match trusted hash %X",
			cH, tH)
	}

	return res, nil
}

func (c *Client) Events(ctx context.Context, req *coretypes.RequestEvents) (*coretypes.ResultEvents, error) {
	return c.next.Events(ctx, req)
}

func (c *Client) Health(ctx context.Context) (*coretypes.ResultHealth, error) {
	return c.next.Health(ctx)
}

// BlockchainInfo calls rpcclient#BlockchainInfo and then verifies every header
// returned.
func (c *Client) BlockchainInfo(ctx context.Context, minHeight, maxHeight int64) (*coretypes.ResultBlockchainInfo, error) {
	res, err := c.next.BlockchainInfo(ctx, minHeight, maxHeight)
	if err != nil {
		return nil, err
	}

	// Validate res.
	for i, meta := range res.BlockMetas {
		if meta == nil {
			return nil, fmt.Errorf("nil block meta %d", i)
		}
		if err := meta.ValidateBasic(); err != nil {
			return nil, fmt.Errorf("invalid block meta %d: %w", i, err)
		}
	}

	// Verify each returned height, including intermediate heights absent from a sparse store.
	for _, meta := range res.BlockMetas {
		h, err := c.updateLightClientIfNeededTo(ctx, &meta.Header.Height)
		if err != nil {
			return nil, fmt.Errorf("verify header %d: %w", meta.Header.Height, err)
		}
		if bmH, tH := meta.Header.Hash(), h.Hash(); !bytes.Equal(bmH, tH) {
			return nil, fmt.Errorf("block meta header %X does not match with trusted header %X",
				bmH, tH)
		}
	}

	return res, nil
}

func (c *Client) Genesis(ctx context.Context) (*coretypes.ResultGenesis, error) {
	return c.next.Genesis(ctx)
}

func (c *Client) GenesisChunked(ctx context.Context, id uint) (*coretypes.ResultGenesisChunk, error) {
	return c.next.GenesisChunked(ctx, id)
}

// Block calls rpcclient#Block and then verifies the result.
func (c *Client) Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error) {
	res, err := c.next.Block(ctx, height)
	if err != nil {
		return nil, err
	}

	// Validate res.
	if err := res.BlockID.ValidateBasic(); err != nil {
		return nil, err
	}
	if err := res.Block.ValidateBasic(types.DefaultConsensusPolicy()); err != nil {
		return nil, err
	}
	if bmH, bH := res.BlockID.Hash, res.Block.Hash(); !bytes.Equal(bmH, bH) {
		return nil, fmt.Errorf("blockID %X does not match with block %X",
			bmH, bH)
	}

	// Update the light client if we're behind.
	l, err := c.updateLightClientIfNeededTo(ctx, &res.Block.Height)
	if err != nil {
		return nil, err
	}

	// Verify block.
	if bH, tH := res.Block.Hash(), l.Hash(); !bytes.Equal(bH, tH) {
		return nil, fmt.Errorf("block header %X does not match with trusted header %X",
			bH, tH)
	}

	return res, nil
}

// BlockByHash calls rpcclient#BlockByHash and then verifies the result.
func (c *Client) BlockByHash(ctx context.Context, hash tmbytes.HexBytes) (*coretypes.ResultBlock, error) {
	res, err := c.next.BlockByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	if res.Block == nil && res.BlockID.IsNil() {
		return res, nil
	}

	// Validate res.
	if err := res.BlockID.ValidateBasic(); err != nil {
		return nil, err
	}
	if err := res.Block.ValidateBasic(types.DefaultConsensusPolicy()); err != nil {
		return nil, err
	}
	if bmH, bH := res.BlockID.Hash, res.Block.Hash(); !bytes.Equal(bmH, bH) {
		return nil, fmt.Errorf("blockID %X does not match with block %X",
			bmH, bH)
	}

	// Update the light client if we're behind.
	l, err := c.updateLightClientIfNeededTo(ctx, &res.Block.Height)
	if err != nil {
		return nil, err
	}

	// Verify block.
	if bH, tH := res.Block.Hash(), l.Hash(); !bytes.Equal(bH, tH) {
		return nil, fmt.Errorf("block header %X does not match with trusted header %X",
			bH, tH)
	}

	return res, nil
}

// BlockResults returns the block results for the given height. If no height is
// provided, the results of the block preceding the latest are returned.
func (c *Client) BlockResults(ctx context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	var h int64
	if height == nil {
		res, err := c.next.Status(ctx)
		if err != nil {
			return nil, fmt.Errorf("can't get latest height: %w", err)
		}
		// Can't return the latest block results here because we won't be able to
		// prove them. Return the results for the previous block instead.
		h = res.SyncInfo.LatestBlockHeight - 1
	} else {
		h = *height
	}

	res, err := c.next.BlockResults(ctx, &h)
	if err != nil {
		return nil, err
	}

	// Validate res.
	if res.Height <= 0 {
		return nil, coretypes.ErrZeroOrNegativeHeight
	}

	// Update the light client if we're behind.
	nextHeight := h + 1
	trustedBlock, err := c.updateLightClientIfNeededTo(ctx, &nextHeight)
	if err != nil {
		return nil, err
	}

	mh, err := sm.TxResultsHash(res.TxsResults)
	if err != nil {
		return nil, err
	}

	// Verify block results.
	if !types.SkipLastResultsHashValidation.Load() && !bytes.Equal(mh, trustedBlock.LastResultsHash) {
		return nil, fmt.Errorf("last results %X does not match with trusted last results %X",
			mh, trustedBlock.LastResultsHash)
	}

	return res, nil
}

// Header fetches and verifies the header directly via the light client
func (c *Client) Header(ctx context.Context, height *int64) (*coretypes.ResultHeader, error) {
	lb, err := c.updateLightClientIfNeededTo(ctx, height)
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultHeader{Header: lb.Header}, nil
}

// HeaderByHash calls rpcclient#HeaderByHash and updates the client if it's falling behind.
func (c *Client) HeaderByHash(ctx context.Context, hash tmbytes.HexBytes) (*coretypes.ResultHeader, error) {
	res, err := c.next.HeaderByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	if res.Header == nil {
		return res, nil
	}

	if err := res.Header.ValidateBasic(); err != nil {
		return nil, err
	}

	lb, err := c.updateLightClientIfNeededTo(ctx, &res.Header.Height)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(lb.Hash(), res.Header.Hash()) {
		return nil, fmt.Errorf("primary header hash does not match trusted header hash. (%X != %X)",
			lb.Hash(), res.Header.Hash())
	}

	return res, nil
}

func (c *Client) Commit(ctx context.Context, height *int64) (*coretypes.ResultCommit, error) {
	// Update the light client if we're behind and retrieve the light block at the requested height
	// or at the latest height if no height is provided.
	l, err := c.updateLightClientIfNeededTo(ctx, height)
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultCommit{
		SignedHeader:    *l.SignedHeader,
		CanonicalCommit: true,
	}, nil
}

// Tx calls rpcclient#Tx method and then verifies the proof if such was
// requested.
func (c *Client) Tx(ctx context.Context, hash tmbytes.HexBytes, prove bool) (*coretypes.ResultTx, error) {
	res, err := c.next.Tx(ctx, hash, prove)
	if err != nil || !prove {
		return res, err
	}

	// Validate res.
	if res.Height <= 0 {
		return nil, coretypes.ErrZeroOrNegativeHeight
	}

	if !bytes.Equal(res.Tx.Hash().Bytes(), hash) || !bytes.Equal(res.Hash, hash) {
		return nil, errors.New("transaction hash does not match requested hash")
	}
	if !bytes.Equal(res.Proof.Data, res.Tx) {
		return nil, errors.New("transaction proof data does not match returned transaction")
	}
	if res.Proof.Proof.Index != int64(res.Index) {
		return nil, errors.New("transaction proof index does not match returned index")
	}

	// Update the light client if we're behind.
	l, err := c.updateLightClientIfNeededTo(ctx, &res.Height)
	if err != nil {
		return nil, err
	}

	// Validate the proof.
	return res, res.Proof.Validate(l.DataHash)
}

func (c *Client) TxSearch(
	ctx context.Context,
	query string,
	prove bool,
	page, perPage *int,
	orderBy string,
) (*coretypes.ResultTxSearch, error) {
	return c.next.TxSearch(ctx, query, prove, page, perPage, orderBy)
}

func (c *Client) BlockSearch(
	ctx context.Context,
	query string,
	page, perPage *int,
	orderBy string,
) (*coretypes.ResultBlockSearch, error) {
	return c.next.BlockSearch(ctx, query, page, perPage, orderBy)
}

// Validators fetches and verifies validators.
func (c *Client) Validators(
	ctx context.Context,
	height *int64,
	pagePtr, perPagePtr *int,
) (*coretypes.ResultValidators, error) {

	// Update the light client if we're behind and retrieve the light block at the
	// requested height or at the latest height if no height is provided.
	l, err := c.updateLightClientIfNeededTo(ctx, height)
	if err != nil {
		return nil, err
	}

	totalCount := len(l.ValidatorSet.Validators)
	perPage := validatePerPage(perPagePtr)
	page, err := validatePage(pagePtr, perPage, totalCount)
	if err != nil {
		return nil, err
	}

	skipCount := validateSkipCount(page, perPage)
	v := l.ValidatorSet.Validators[skipCount : skipCount+tmmath.MinInt(int(perPage), totalCount-skipCount)] //nolint:gosec // perPage is bounded to maxPerPage (100); no overflow risk

	return &coretypes.ResultValidators{
		BlockHeight: l.Height,
		Validators:  v,
		Count:       len(v),
		Total:       totalCount,
	}, nil
}

func (c *Client) BroadcastEvidence(ctx context.Context, ev types.Evidence) (*coretypes.ResultBroadcastEvidence, error) {
	return c.next.BroadcastEvidence(ctx, ev)
}

func (c *Client) Subscribe(ctx context.Context, subscriber, query string,
	outCapacity ...int) (out <-chan coretypes.ResultEvent, err error) {
	return c.next.Subscribe(ctx, subscriber, query, outCapacity...) //nolint:staticcheck
}

func (c *Client) Unsubscribe(ctx context.Context, subscriber, query string) error {
	c.subscriptionsMu.Lock()
	err := c.next.Unsubscribe(ctx, subscriber, query) //nolint:staticcheck
	pending := c.cancelSubscriptionsLocked(subscriber, &query)
	c.subscriptionsMu.Unlock()
	for _, sub := range pending {
		<-sub.done
	}
	return err
}

func (c *Client) UnsubscribeAll(ctx context.Context, subscriber string) error {
	c.subscriptionsMu.Lock()
	err := c.next.UnsubscribeAll(ctx, subscriber) //nolint:staticcheck
	pending := c.cancelSubscriptionsLocked(subscriber, nil)
	c.subscriptionsMu.Unlock()
	for _, sub := range pending {
		<-sub.done
	}
	return err
}

func (c *Client) updateLightClientIfNeededTo(ctx context.Context, height *int64) (*types.LightBlock, error) {
	var (
		l   *types.LightBlock
		err error
	)
	if height == nil {
		l, err = c.lc.Update(ctx, time.Now())
	} else {
		l, err = c.lc.VerifyLightBlockAtHeight(ctx, *height, time.Now())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update light client: %w", err)
	}
	return l, nil
}

func (c *Client) RegisterOpDecoder(typ string, dec merkle.OpDecoder) {
	c.prt.RegisterOpDecoder(typ, dec)
}

// SubscribeWS subscribes for events using the given query and remote address as
// a subscriber, but does not verify responses (UNSAFE)!
// TODO: verify data
func (c *Client) SubscribeWS(ctx context.Context, query string) (*coretypes.ResultSubscribe, error) {
	c.subscriptionsMu.Lock()
	defer c.subscriptionsMu.Unlock()
	if c.stopped {
		return nil, errors.New("client is stopped")
	}
	callInfo := rpctypes.GetCallInfo(ctx)
	key := wsSubscriptionKey{subscriber: callInfo.RemoteAddr(), query: query}
	if _, exists := c.subscriptions[key]; exists {
		return nil, fmt.Errorf("already subscribed to %q", query)
	}
	bctx, bcancel := context.WithCancel(callInfo.WSConn.Context())
	out, err := c.next.Subscribe(bctx, key.subscriber, query) //nolint:staticcheck
	if err != nil {
		bcancel()
		return nil, err
	}
	sub := &wsSubscription{cancel: bcancel, done: make(chan struct{})}
	if c.subscriptions == nil {
		c.subscriptions = make(map[wsSubscriptionKey]*wsSubscription)
	}
	c.subscriptions[key] = sub

	go func() {
		defer func() {
			bcancel()
			c.subscriptionsMu.Lock()
			if c.subscriptions[key] == sub {
				delete(c.subscriptions, key)
			}
			close(sub.done)
			c.subscriptionsMu.Unlock()
		}()
		for {
			select {
			case resultEvent, ok := <-out:
				if !ok || bctx.Err() != nil {
					return
				}
				// We should have a switch here that performs a validation
				// depending on the event's type.
				callInfo.WSConn.TryWriteRPCResponse(bctx, callInfo.RPCRequest.MakeResponse(resultEvent))
			case <-bctx.Done():
				return
			}
		}
	}()

	return &coretypes.ResultSubscribe{}, nil
}

// cancelSubscriptionsLocked detaches forwarding state while subscription changes
// are serialized with upstream calls. Callers wait for completion after unlocking.
func (c *Client) cancelSubscriptionsLocked(subscriber string, query *string) []*wsSubscription {
	var pending []*wsSubscription
	for key, sub := range c.subscriptions {
		if key.subscriber == subscriber && (query == nil || key.query == *query) {
			sub.cancel()
			delete(c.subscriptions, key)
			pending = append(pending, sub)
		}
	}
	return pending
}

// UnsubscribeWS calls original client's Unsubscribe using remote address as a
// subscriber.
func (c *Client) UnsubscribeWS(ctx context.Context, query string) (*coretypes.ResultUnsubscribe, error) {
	err := c.Unsubscribe(ctx, rpctypes.GetCallInfo(ctx).RemoteAddr(), query)
	if err != nil {
		return nil, err
	}
	return &coretypes.ResultUnsubscribe{}, nil
}

// UnsubscribeAllWS calls original client's UnsubscribeAll using remote address
// as a subscriber.
func (c *Client) UnsubscribeAllWS(ctx context.Context) (*coretypes.ResultUnsubscribe, error) {
	err := c.UnsubscribeAll(ctx, rpctypes.GetCallInfo(ctx).RemoteAddr())
	if err != nil {
		return nil, err
	}
	return &coretypes.ResultUnsubscribe{}, nil
}

// XXX: Copied from rpc/core/env.go
const (
	// see README
	defaultPerPage = 30
	maxPerPage     = 100
)

func validatePage(pagePtr *int, perPage uint, totalCount int) (int, error) {

	if pagePtr == nil { // no page parameter
		return 1, nil
	}

	pages := ((totalCount - 1) / int(perPage)) + 1 //nolint:gosec // perPage is bounded to maxPerPage (100); no overflow risk
	if pages == 0 {
		pages = 1 // one page (even if it's empty)
	}
	page := *pagePtr
	if page <= 0 || page > pages {
		return 1, fmt.Errorf("%w expected range: [1, %d], given %d", coretypes.ErrPageOutOfRange, pages, page)
	}

	return page, nil
}

func validatePerPage(perPagePtr *int) uint {
	if perPagePtr == nil { // no per_page parameter
		return defaultPerPage
	}

	perPage := *perPagePtr
	if perPage < 1 {
		return defaultPerPage
	} else if perPage > maxPerPage {
		return maxPerPage
	}
	return uint(perPage) //nolint:gosec // perPage is bounds-checked above to [1, maxPerPage]
}

func validateSkipCount(page int, perPage uint) int {
	skipCount := (page - 1) * int(perPage) //nolint:gosec // perPage is bounded to maxPerPage (100) and page is validated; no overflow risk
	if skipCount < 0 {
		return 0
	}

	return skipCount
}
