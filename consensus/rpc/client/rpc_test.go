package client_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/mempool"
	rpccore "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	tmmath "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/math"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/service"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/require"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	rpclocal "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func getHTTPClient(t *testing.T, conf *config.Config) *rpchttp.HTTP {
	t.Helper()

	rpcAddr := conf.RPC.ListenAddress
	c, err := rpchttp.NewWithClient(rpcAddr, http.DefaultClient)
	require.NoError(t, err)
	ctx := t.Context()
	require.NoError(t, c.Start(ctx))

	t.Cleanup(func() {
		require.NoError(t, c.Stop())
	})

	return c
}

func getHTTPClientWithTimeout(t *testing.T, conf *config.Config, timeout time.Duration) *rpchttp.HTTP {
	t.Helper()

	rpcAddr := conf.RPC.ListenAddress

	tclient := &http.Client{Timeout: timeout}
	c, err := rpchttp.NewWithClient(rpcAddr, tclient)
	require.NoError(t, err)
	ctx := t.Context()
	require.NoError(t, c.Start(ctx))

	t.Cleanup(func() {
		require.NoError(t, c.Stop())
	})

	return c
}

// GetClients returns a slice of clients for table-driven tests
func GetClients(t *testing.T, ns service.Service, conf *config.Config) []client.Client {
	t.Helper()

	node, ok := ns.(rpclocal.NodeService)
	require.True(t, ok)

	ncl, err := rpclocal.New(node)
	require.NoError(t, err)

	return []client.Client{
		ncl,
		getHTTPClient(t, conf),
	}
}

func TestClientOperations(t *testing.T) {
	ctx := t.Context()

	_, conf := NodeSuite(ctx, t)

	t.Run("NilCustomHTTPClient", func(t *testing.T) {
		_, err := rpchttp.NewWithClient("http://example.com", nil)
		require.Error(t, err)

		_, err = rpcclient.NewWithHTTPClient("http://example.com", nil)
		require.Error(t, err)
	})
	t.Run("ParseInvalidAddress", func(t *testing.T) {
		// should remove trailing /
		invalidRemote := conf.RPC.ListenAddress + "/"
		_, err := rpchttp.New(invalidRemote)
		require.NoError(t, err)
	})
	t.Run("CustomHTTPClient", func(t *testing.T) {
		remote := conf.RPC.ListenAddress
		c, err := rpchttp.NewWithClient(remote, http.DefaultClient)
		require.NoError(t, err)
		status, err := c.Status(ctx)
		require.NoError(t, err)
		require.NotNil(t, status)
	})
	t.Run("CorsEnabled", func(t *testing.T) {
		origin := conf.RPC.CORSAllowedOrigins[0]
		remote := strings.ReplaceAll(conf.RPC.ListenAddress, "tcp", "http")

		req, err := http.NewRequestWithContext(ctx, "GET", remote, nil)
		require.NoError(t, err, "%+v", err)
		req.Header.Set("Origin", origin)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "%+v", err)
		defer resp.Body.Close()

		require.Equal(t, resp.Header.Get("Access-Control-Allow-Origin"), origin)
	})
	t.Run("Batching", func(t *testing.T) {
		t.Run("JSONRPCCalls", func(t *testing.T) {

			c := getHTTPClient(t, conf)
			testBatchedJSONRPCCalls(ctx, c)
		})
		t.Run("JSONRPCCallsCancellation", func(t *testing.T) {
			_, _, tx1 := MakeTxKV()
			_, _, tx2 := MakeTxKV()

			c := getHTTPClient(t, conf)
			batch := c.NewBatch()
			_, err := batch.BroadcastTxCommit(ctx, tx1)
			require.NoError(t, err)
			_, err = batch.BroadcastTxCommit(ctx, tx2)
			require.NoError(t, err)
			// we should have 2 requests waiting
			require.Equal(t, 2, batch.Count())
			// we want to make sure we cleared 2 pending requests
			require.Equal(t, 2, batch.Clear())
			// now there should be no batched requests
			require.Equal(t, 0, batch.Count())
		})
		t.Run("SendingEmptyRequest", func(t *testing.T) {
			c := getHTTPClient(t, conf)
			batch := c.NewBatch()
			_, err := batch.Send(ctx)
			require.Error(t, err, "sending an empty batch of JSON RPC requests should result in an error")
		})
		t.Run("ClearingEmptyRequest", func(t *testing.T) {
			c := getHTTPClient(t, conf)
			batch := c.NewBatch()
			require.Zero(t, batch.Clear(), "clearing an empty batch of JSON RPC requests should result in a 0 result")
		})
		t.Run("ConcurrentJSONRPC", func(t *testing.T) {
			var wg sync.WaitGroup
			c := getHTTPClient(t, conf)
			for range 50 {
				wg.Go(func() { testBatchedJSONRPCCalls(ctx, c) })
			}
			wg.Wait()
		})
	})
}

// Make sure info is correct (we connect properly)
func TestClientMethodCalls(t *testing.T) {

	n, conf := NodeSuite(t.Context(), t)

	// for broadcast tx tests
	pool := getMempool(t, n)

	// for evidence tests
	pv, err := privval.LoadOrGenFilePV(conf.PrivValidator.KeyFile(), conf.PrivValidator.StateFile())
	require.NoError(t, err)

	for i, c := range GetClients(t, n, conf) {
		t.Run(fmt.Sprintf("%T", c), func(t *testing.T) {
			t.Run("Status", func(t *testing.T) {
				status, err := c.Status(t.Context())
				require.NoError(t, err, "%d: %+v", i, err)
				require.Equal(t, conf.Moniker, status.NodeInfo.Moniker)
			})
			t.Run("Info", func(t *testing.T) {
				ctx := t.Context()
				info, err := c.ABCIInfo(ctx)
				require.NoError(t, err)

				status, err := c.Status(ctx)
				require.NoError(t, err)

				require.GreaterOrEqual(t, status.SyncInfo.LatestBlockHeight, info.Response.LastBlockHeight)
				require.True(t, strings.Contains(info.Response.Data, "size"))
			})
			t.Run("NetInfo", func(t *testing.T) {
				nc, ok := c.(client.NetworkClient)
				require.True(t, ok, "%d", i)
				netinfo, err := nc.NetInfo(t.Context())
				require.NoError(t, err, "%d: %+v", i, err)
				require.True(t, netinfo.Listening)
				require.Equal(t, 0, len(netinfo.Peers))
			})
			t.Run("DumpConsensusState", func(t *testing.T) {
				// FIXME: fix server so it doesn't panic on invalid input
				nc, ok := c.(client.NetworkClient)
				require.True(t, ok, "%d", i)
				cons, err := nc.DumpConsensusState(t.Context())
				require.NoError(t, err, "%d: %+v", i, err)
				require.NotEmpty(t, cons.RoundState)
				require.Empty(t, cons.Peers)
			})
			t.Run("ConsensusState", func(t *testing.T) {
				// FIXME: fix server so it doesn't panic on invalid input
				nc, ok := c.(client.NetworkClient)
				require.True(t, ok, "%d", i)
				cons, err := nc.ConsensusState(t.Context())
				require.NoError(t, err, "%d: %+v", i, err)
				require.NotEmpty(t, cons.RoundState)
			})
			t.Run("Health", func(t *testing.T) {
				nc, ok := c.(client.NetworkClient)
				require.True(t, ok, "%d", i)
				_, err := nc.Health(t.Context())
				require.NoError(t, err, "%d: %+v", i, err)
			})
			t.Run("GenesisAndValidators", func(t *testing.T) {
				ctx := t.Context()
				// make sure this is the right genesis file
				gen, err := c.Genesis(ctx)
				require.NoError(t, err, "%d: %+v", i, err)
				// get the genesis validator
				require.Equal(t, 1, len(gen.Genesis.Validators))
				gval := gen.Genesis.Validators[0]

				// get the current validators
				h := int64(1)
				vals, err := c.Validators(ctx, &h, nil, nil)
				require.NoError(t, err, "%d: %+v", i, err)
				require.Equal(t, 1, len(vals.Validators))
				require.Equal(t, 1, vals.Count)
				require.Equal(t, 1, vals.Total)
				val := vals.Validators[0]

				// make sure the current set is also the genesis set
				require.Equal(t, gval.Power, val.VotingPower)
				require.Equal(t, gval.PubKey, val.PubKey)
			})
			t.Run("GenesisChunked", func(t *testing.T) {
				ctx := t.Context()
				first, err := c.GenesisChunked(ctx, 0)
				require.NoError(t, err)

				decoded := make([]string, 0, first.TotalChunks)
				for i := 0; i < first.TotalChunks; i++ {
					chunk, err := c.GenesisChunked(ctx, uint(i))
					require.NoError(t, err)
					data, err := base64.StdEncoding.DecodeString(chunk.Data)
					require.NoError(t, err)
					decoded = append(decoded, string(data))

				}
				doc := []byte(strings.Join(decoded, ""))

				var out types.GenesisDoc
				require.NoError(t, tmjson.Unmarshal(doc, &out),
					"first: %+v, doc: %s", first, string(doc))
			})
			t.Run("ABCIQuery", func(t *testing.T) {
				ctx := t.Context()
				// write something
				k, v, tx := MakeTxKV()
				status, err := c.Status(ctx)
				require.NoError(t, err)
				_, err = c.BroadcastTxSync(ctx, tx)
				require.NoError(t, err, "%d: %+v", i, err)
				apph := status.SyncInfo.LatestBlockHeight + 2 // this is where the tx will be applied to the state

				// wait before querying
				err = client.WaitForHeight(ctx, c, apph, nil)
				require.NoError(t, err)
				res, err := c.ABCIQuery(ctx, "/key", k)
				require.NoError(t, err)
				qres := res.Response
				require.True(t, qres.IsOK())
				require.Equal(t, v, qres.Value)
			})
			t.Run("AppCalls", func(t *testing.T) {
				ctx := t.Context()
				// get an offset of height to avoid racing and guessing
				s, err := c.Status(ctx)
				require.NoError(t, err)
				// sh is start height or status height
				sh := s.SyncInfo.LatestBlockHeight

				// look for the future
				h := sh + 20
				_, err = c.Block(ctx, &h)
				require.Error(t, err) // no block yet

				// write something
				k, v, tx := MakeTxKV()
				bres, err := c.BroadcastTxCommit(ctx, tx)
				require.NoError(t, err)
				require.True(t, bres.TxResult.IsOK())
				txh := bres.Height
				apph := txh + 1 // this is where the tx will be applied to the state

				// wait before querying
				err = client.WaitForHeight(ctx, c, apph, nil)
				require.NoError(t, err)

				_qres, err := c.ABCIQueryWithOptions(ctx, "/key", k, client.ABCIQueryOptions{Prove: false})
				require.NoError(t, err)
				qres := _qres.Response
				require.True(t, qres.IsOK())
				require.Equal(t, k, qres.Key)
				require.Equal(t, v, qres.Value)

				// make sure we can lookup the tx with proof
				ptx, err := c.Tx(ctx, bres.Hash, true)
				require.NoError(t, err)
				require.Equal(t, txh, ptx.Height)
				require.Equal(t, tx, ptx.Tx)

				// and we can even check the block is added
				block, err := c.Block(ctx, &apph)
				require.NoError(t, err)
				appHash := block.Block.Header.AppHash
				require.True(t, len(appHash) > 0)
				require.Equal(t, apph, block.Block.Header.Height)

				blockByHash, err := c.BlockByHash(ctx, block.BlockID.Hash)
				require.NoError(t, err)
				require.Equal(t, block, blockByHash)

				// check that the header matches the block hash
				header, err := c.Header(ctx, &apph)
				require.NoError(t, err)
				require.Equal(t, block.Block.Header, *header.Header)

				headerByHash, err := c.HeaderByHash(ctx, block.BlockID.Hash)
				require.NoError(t, err)
				require.Equal(t, header, headerByHash)

				// now check the results
				blockResults, err := c.BlockResults(ctx, &txh)
				require.NoError(t, err, "%d: %+v", i, err)
				require.Equal(t, txh, blockResults.Height)
				require.Len(t, blockResults.TxsResults, 1)
				// check success code
				require.Equal(t, uint32(0), blockResults.TxsResults[0].Code)

				// check blockchain info, now that we know there is info
				info, err := c.BlockchainInfo(ctx, apph, apph)
				require.NoError(t, err)
				require.True(t, info.LastHeight >= apph)
				require.Len(t, info.BlockMetas, 1)
				lastMeta := info.BlockMetas[0]
				require.Equal(t, apph, lastMeta.Header.Height)
				blockData := block.Block
				require.Equal(t, blockData.Header.AppHash, lastMeta.Header.AppHash)
				require.Equal(t, block.BlockID, lastMeta.BlockID)

				// and get the corresponding commit with the same apphash
				commit, err := c.Commit(ctx, &apph)
				require.NoError(t, err)
				cappHash := commit.Header.AppHash
				require.Equal(t, appHash, cappHash)
				require.NotNil(t, commit.Commit)

				// compare the commits (note Commit(2) has commit from Block(3))
				h = apph - 1
				commit2, err := c.Commit(ctx, &h)
				require.NoError(t, err)
				require.Equal(t, block.Block.LastCommitHash, commit2.Commit.Hash())

				// and we got a proof that works!
				_pres, err := c.ABCIQueryWithOptions(ctx, "/key", k, client.ABCIQueryOptions{Prove: true})
				require.NoError(t, err)
				pres := _pres.Response
				require.True(t, pres.IsOK())

				// XXX Test proof
			})
			t.Run("BlockchainInfo", func(t *testing.T) {
				ctx := t.Context()

				err := client.WaitForHeight(ctx, c, 10, nil)
				require.NoError(t, err)

				res, err := c.BlockchainInfo(ctx, 0, 0)
				require.NoError(t, err, "%d: %+v", i, err)
				require.True(t, res.LastHeight > 0)
				require.True(t, len(res.BlockMetas) > 0)

				res, err = c.BlockchainInfo(ctx, 1, 1)
				require.NoError(t, err, "%d: %+v", i, err)
				require.True(t, res.LastHeight > 0)
				require.True(t, len(res.BlockMetas) == 1)

				res, err = c.BlockchainInfo(ctx, 1, 10000)
				require.NoError(t, err, "%d: %+v", i, err)
				require.True(t, res.LastHeight > 0)
				require.True(t, len(res.BlockMetas) < 100)
				for _, m := range res.BlockMetas {
					require.NotNil(t, m)
				}

				res, err = c.BlockchainInfo(ctx, 10000, 1)
				require.Error(t, err)
				require.Nil(t, res)
				require.Contains(t, err.Error(), "can't be greater than max")
			})
			t.Run("BroadcastTxCommit", func(t *testing.T) {
				ctx := t.Context()
				_, _, tx := MakeTxKV()
				bres, err := c.BroadcastTxCommit(ctx, tx)
				require.NoError(t, err, "%d: %+v", i, err)
				require.True(t, bres.CheckTx.IsOK())
				require.True(t, bres.TxResult.IsOK())

				require.Equal(t, 0, pool.Size())
			})
			t.Run("BroadcastTxSync", func(t *testing.T) {
				ctx := t.Context()
				_, _, tx := MakeTxKV()
				initMempoolSize := pool.Size()
				bres, err := c.BroadcastTxSync(ctx, tx)
				require.NoError(t, err, "%d: %+v", i, err)
				require.Equal(t, bres.Code, abci.CodeTypeOK) // FIXME

				require.Equal(t, initMempoolSize+1, pool.Size())

				txs, _ := pool.ReapTxs(mempool.ReapLimits{MaxTxs: utils.Some(uint64(len(tx)))}, false)
				require.Equal(t, tx, txs[0])
				pool.Flush()
			})
			t.Run("CheckTx", func(t *testing.T) {
				ctx := t.Context()
				_, _, tx := MakeTxKV()

				res, err := c.CheckTx(ctx, tx)
				require.NoError(t, err)
				require.Equal(t, abci.CodeTypeOK, res.Code)

				require.Equal(t, 0, pool.Size(), "mempool must be empty")
			})
			t.Run("Events", func(t *testing.T) {
				t.Run("Header", func(t *testing.T) {
					ctx, cancel := context.WithTimeout(t.Context(), waitForEventTimeout)
					defer cancel()
					query := types.QueryForEvent(types.EventNewBlockHeaderValue).String()
					evt, err := client.WaitForOneEvent(ctx, c, query)
					require.NoError(t, err, "%d: %+v", i, err)
					_, ok := evt.(types.EventDataNewBlockHeader)
					require.True(t, ok, "%d: %#v", i, evt)
					// TODO: more checks...
				})
				t.Run("Block", func(t *testing.T) {
					ctx := t.Context()
					const subscriber = "TestBlockEvents"

					eventCh, err := c.Subscribe(ctx, subscriber, types.QueryForEvent(types.EventNewBlockValue).String())
					require.NoError(t, err)
					t.Cleanup(func() {
						// At this point the ctx is cancelled, so the cleanup needs to run with a background context.
						if err := c.UnsubscribeAll(context.Background(), subscriber); err != nil {
							t.Error(err)
						}
					})

					var firstBlockHeight int64
					for i := range int64(3) {
						event := <-eventCh

						blockEvent, ok := event.Data.(types.LegacyEventDataNewBlock)
						if !ok {
							blockEventPtr, okPtr := event.Data.(*types.LegacyEventDataNewBlock)
							if okPtr {
								blockEvent = *blockEventPtr
							}
							ok = okPtr
						}
						require.True(t, ok)

						block := blockEvent.Block

						if firstBlockHeight == 0 {
							firstBlockHeight = block.Header.Height
						}

						require.Equal(t, firstBlockHeight+i, block.Header.Height)
					}
				})
				t.Run("BroadcastTxAsync", func(t *testing.T) {
					ctx := t.Context()
					testTxEventsSent(ctx, t, "async", c)
				})
				t.Run("BroadcastTxSync", func(t *testing.T) {
					ctx := t.Context()
					testTxEventsSent(ctx, t, "sync", c)
				})
			})
			t.Run("Evidence", func(t *testing.T) {
				t.Run("BroadcastDuplicateVote", func(t *testing.T) {
					ctx := t.Context()

					chainID := conf.ChainID()

					// make sure that the node has produced enough blocks
					waitForBlock(ctx, t, c, 2)
					evidenceHeight := int64(1)
					block, _ := c.Block(ctx, &evidenceHeight)
					ts := block.Block.Time
					correct, fakes := makeEvidences(t, pv, chainID, ts)

					result, err := c.BroadcastEvidence(ctx, correct)
					require.NoError(t, err, "BroadcastEvidence(%s) failed", correct)
					require.Equal(t, correct.Hash(), result.Hash, "expected result hash to match evidence hash")

					status, err := c.Status(ctx)
					require.NoError(t, err)
					err = client.WaitForHeight(ctx, c, status.SyncInfo.LatestBlockHeight+2, nil)
					require.NoError(t, err)

					result2, err := c.ABCIQuery(ctx, "/val", pv.Key.PubKey.Bytes())
					require.NoError(t, err)
					qres := result2.Response
					require.True(t, qres.IsOK())

					var v abci.ValidatorUpdate
					err = proto.Unmarshal(qres.Value, &v)
					require.NoError(t, err, "Error reading query result, value %v", qres.Value)

					pk, err := crypto.PubKeyFromProto(v.PubKey)
					require.NoError(t, err)

					require.Equal(t, pv.Key.PubKey, pk, "Stored PubKey not equal with expected, value %v", string(qres.Value))
					require.Equal(t, int64(9), v.Power, "Stored Power not equal with expected, value %v", string(qres.Value))

					for _, fake := range fakes {
						_, err := c.BroadcastEvidence(ctx, fake)
						require.Error(t, err, "BroadcastEvidence(%s) succeeded, but the evidence was fake", fake)
					}
				})
				t.Run("BroadcastEmpty", func(t *testing.T) {
					ctx := t.Context()
					_, err := c.BroadcastEvidence(ctx, nil)
					require.Error(t, err)
				})
			})
		})
	}
}

func getMempool(t *testing.T, srv service.Service) *mempool.TxMempool {
	t.Helper()
	n, ok := srv.(interface {
		RPCEnvironment() *rpccore.Environment
	})
	require.True(t, ok)
	mp, ok := n.RPCEnvironment().Mempool.Get()
	require.True(t, ok)
	return mp
}

// these cases are roughly the same as the TestClientMethodCalls, but
// they have to loop over their clients in the individual test cases,
// so making a separate suite makes more sense, though isn't strictly
// speaking desirable.
func TestClientMethodCallsAdvanced(t *testing.T) {
	ctx := t.Context()

	n, conf := NodeSuite(ctx, t)
	t.Run("Tx", func(t *testing.T) {

		c := getHTTPClient(t, conf)

		// first we broadcast a tx
		_, _, tx := MakeTxKV()
		bres, err := c.BroadcastTxCommit(ctx, tx)
		require.NoError(t, err, "%+v", err)

		txHeight := bres.Height
		txHash := bres.Hash

		anotherTxHash := types.Tx("a different tx").Hash()

		cases := []struct {
			valid bool
			prove bool
			hash  []byte
		}{
			// only valid if correct hash provided
			{true, false, txHash},
			{true, true, txHash},
			{false, false, anotherTxHash.Bytes()},
			{false, true, anotherTxHash.Bytes()},
			{false, false, nil},
			{false, true, nil},
		}

		for _, c := range GetClients(t, n, conf) {
			t.Run(fmt.Sprintf("%T", c), func(t *testing.T) {
				for j, tc := range cases {
					t.Run(fmt.Sprintf("Case%d", j), func(t *testing.T) {
						// now we query for the tx.
						// since there's only one tx, we know index=0.
						ptx, err := c.Tx(ctx, tc.hash, tc.prove)

						if !tc.valid {
							require.Error(t, err)
						} else {
							require.NoError(t, err, "%+v", err)
							require.Equal(t, txHeight, ptx.Height)
							require.Equal(t, tx, ptx.Tx)
							require.Zero(t, ptx.Index)
							require.True(t, ptx.TxResult.IsOK())
							require.Equal(t, txHash, ptx.Hash)

							// time to verify the proof
							proof := ptx.Proof
							if tc.prove {
								require.Equal(t, tx, proof.Data)
								require.NoError(t, proof.Proof.Verify(proof.RootHash, txHash))
							}
						}
					})
				}
			})
		}
	})
	t.Run("TxSearchWithTimeout", func(t *testing.T) {

		timeoutClient := getHTTPClientWithTimeout(t, conf, 10*time.Second)

		_, _, tx := MakeTxKV()
		_, err := timeoutClient.BroadcastTxCommit(ctx, tx)
		require.NoError(t, err)

		// query using a compositeKey (see kvstore application)
		result, err := timeoutClient.TxSearch(ctx, "app.creator='Cosmoshi Netowoko'", false, nil, nil, "asc")
		require.NoError(t, err)
		require.Greater(t, len(result.Txs), 0, "expected a lot of transactions")
	})
	t.Run("TxSearch", testTxSearch)
}

func testTxSearch(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	n, conf := NodeSuite(ctx, t)
	producer := getHTTPClient(t, conf)
	const txCount = 10
	txs := make([]types.Tx, txCount)
	committed := make([]*coretypes.ResultBroadcastTxCommit, txCount)
	for i := range txs {
		txs[i] = types.Tx(fmt.Sprintf("repair-m1110-%02d=value-%02d", i, i))
		result, err := producer.BroadcastTxCommit(ctx, txs[i])
		require.NoError(t, err)
		require.True(t, result.CheckTx.IsOK())
		require.True(t, result.TxResult.IsOK())
		require.Equal(t, txs[i].Hash().Bytes(), []byte(result.Hash))
		require.Greater(t, result.Height, int64(0))
		if i > 0 {
			require.Greater(t, result.Height, committed[i-1].Height)
		}
		committed[i] = result
	}

	for _, c := range GetClients(t, n, conf) {
		t.Run(fmt.Sprintf("%T", c), func(t *testing.T) {
			checkTx := func(got *coretypes.ResultTx, index int, prove bool) {
				t.Helper()
				want := committed[index]
				require.Equal(t, want.Height, got.Height)
				require.Zero(t, got.Index)
				require.Equal(t, txs[index], got.Tx)
				require.Equal(t, want.Hash, got.Hash)
				require.True(t, got.TxResult.IsOK())
				if prove {
					require.Equal(t, txs[index], got.Proof.Data)
					require.NoError(t, got.Proof.Proof.Verify(got.Proof.RootHash, want.Hash))
					block, err := c.Block(ctx, &want.Height)
					require.NoError(t, err)
					require.Equal(t, []byte(block.Block.DataHash), []byte(got.Proof.RootHash))
				} else {
					require.Empty(t, got.Proof.Data)
				}
			}
			find := committed[txCount-1]
			result, err := c.TxSearch(ctx, fmt.Sprintf("tx.hash='%X'", []byte(find.Hash)), true, nil, nil, "asc")
			require.NoError(t, err)
			require.Equal(t, 1, result.TotalCount)
			require.Len(t, result.Txs, 1)
			checkTx(result.Txs[0], txCount-1, true)

			result, err = c.TxSearch(ctx, fmt.Sprintf("tx.height=%d", find.Height), true, nil, nil, "asc")
			require.NoError(t, err)
			require.Equal(t, 1, result.TotalCount)
			require.Len(t, result.Txs, 1)
			checkTx(result.Txs[0], txCount-1, true)

			for _, query := range []string{
				fmt.Sprintf("tx.hash='%X'", types.Tx("a different tx").Hash().Bytes()),
				"app.noindex_key='index is working'",
				"app.creator='Cosmoshi Neetowoko'",
			} {
				perPage := 1
				result, err := c.TxSearch(ctx, query, true, nil, &perPage, "asc")
				require.NoError(t, err)
				require.Zero(t, result.TotalCount)
				require.Len(t, result.Txs, 0)
			}

			for _, query := range []string{
				"app.creator='Cosmoshi Netowoko'",
				"app.index_key='index is working'",
				fmt.Sprintf("app.creator='Cosmoshi Netowoko' AND tx.height<=%d", find.Height),
				"tx.height >= 1",
			} {
				result, err := c.TxSearch(ctx, query, true, nil, nil, "asc")
				require.NoError(t, err)
				require.Equal(t, txCount, result.TotalCount)
				require.Len(t, result.Txs, txCount)
				for i, tx := range result.Txs {
					checkTx(tx, i, true)
				}
			}

			for _, order := range []string{"asc", "desc"} {
				result, err := c.TxSearch(ctx, "tx.height >= 1", false, nil, nil, order)
				require.NoError(t, err)
				require.Equal(t, txCount, result.TotalCount)
				require.Len(t, result.Txs, txCount)
				for i, tx := range result.Txs {
					index := i
					if order == "desc" {
						index = txCount - 1 - i
					}
					checkTx(tx, index, false)
				}

				perPage := 3
				pages := (txCount + perPage - 1) / perPage
				seen := map[int64]bool{}
				for page := 1; page <= pages; page++ {
					result, err := c.TxSearch(ctx, "tx.height >= 1", true, &page, &perPage, order)
					require.NoError(t, err)
					require.Equal(t, txCount, result.TotalCount)
					start := (page - 1) * perPage
					require.Len(t, result.Txs, min(perPage, txCount-start))
					for i, tx := range result.Txs {
						index := start + i
						if order == "desc" {
							index = txCount - 1 - index
						}
						checkTx(tx, index, true)
						require.False(t, seen[tx.Height], "duplicate height %d on page %d", tx.Height, page)
						seen[tx.Height] = true
					}
				}
				require.Len(t, seen, txCount)
			}
		})
	}
}

func TestMempoolRPCOnFullNode(t *testing.T) {
	ctx := t.Context()

	// Use a full node so background consensus does not consume mempool
	// transactions while these RPC assertions are running.
	n, conf := FullNodeSuite(ctx, t)
	pool := getMempool(t, n)

	t.Run("UnconfirmedTxs", func(t *testing.T) {
		txs := make([]types.Tx, 5)
		for i := range txs {
			_, _, tx := MakeTxKV()
			txs[i] = tx
			_, err := pool.CheckTx(ctx, tx)
			require.NoError(t, err)
		}

		// UnconfirmedTxs reads from the mempool's recent snapshot, so force a
		// snapshot recomputation after inserting the test transactions.
		pool.ReapTxs(mempool.ReapLimits{}, false)

		for _, c := range GetClients(t, n, conf) {
			mc, ok := c.(client.MempoolClient)
			require.True(t, ok)
			for i := 1; i <= 2; i++ {
				page, perPage := i, 3
				res, err := mc.UnconfirmedTxs(ctx, &page, &perPage)
				require.NoError(t, err)

				if i == 2 {
					perPage = 2
				}
				require.Equal(t, perPage, int(res.Count))
				require.Equal(t, 5, int(res.Total))
				require.Equal(t, pool.SizeBytes(), uint64(res.TotalBytes))
				for _, tx := range res.Txs {
					require.Contains(t, txs, tx)
				}
			}
		}

		pool.Flush()
	})

	t.Run("NumUnconfirmedTxs", func(t *testing.T) {
		_, _, tx := MakeTxKV()
		_, err := pool.CheckTx(ctx, tx)
		require.NoError(t, err)

		mempoolSize := pool.Size()
		for i, c := range GetClients(t, n, conf) {
			mc, ok := c.(client.MempoolClient)
			require.True(t, ok, "%d", i)
			res, err := mc.NumUnconfirmedTxs(ctx)
			require.NoError(t, err, "%d: %+v", i, err)

			require.Equal(t, mempoolSize, int(res.Count))
			require.Equal(t, mempoolSize, int(res.Total))
			require.Equal(t, pool.SizeBytes(), uint64(res.TotalBytes))
		}

		pool.Flush()
	})
}

// WARNING: this function is called in subgoroutines to it shouldn't use testing.T.
func testBatchedJSONRPCCalls(ctx context.Context, c *rpchttp.HTTP) {
	k1, _, tx1 := MakeTxKV()
	k2, _, tx2 := MakeTxKV()

	batch := c.NewBatch()
	r1 := utils.OrPanic1(batch.BroadcastTxCommit(ctx, tx1))
	r2 := utils.OrPanic1(batch.BroadcastTxCommit(ctx, tx2))
	utils.OrPanic(utils.TestDiff(2, batch.Count()))
	bresults := utils.OrPanic1(batch.Send(ctx))
	utils.OrPanic(utils.TestDiff(bresults, []any{r1, r2}))
	utils.OrPanic(utils.TestDiff(0, batch.Count()))
	apph := tmmath.MaxInt64(r1.Height, r2.Height) + 1

	utils.OrPanic(client.WaitForHeight(ctx, c, apph, nil))

	q1 := utils.OrPanic1(batch.ABCIQuery(ctx, "/key", k1))
	q2 := utils.OrPanic1(batch.ABCIQuery(ctx, "/key", k2))
	utils.OrPanic(utils.TestDiff(2, batch.Count()))
	qresults := utils.OrPanic1(batch.Send(ctx))
	utils.OrPanic(utils.TestDiff(qresults, []any{q1, q2}))
}
