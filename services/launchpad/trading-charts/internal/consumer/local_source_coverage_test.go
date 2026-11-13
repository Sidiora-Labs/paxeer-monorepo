package consumer

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"math/big"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/jackc/pgx/v5"
)

type localChartChain struct {
	rpc        *ethclient.Client
	creation   *types.Receipt
	swaps      []*types.Receipt
	headers    map[int64]chainread.Block
	pool       common.Address
	token      common.Address
	settlement common.Address
	sender     common.Address
}

func chartEventRuntime(topics []common.Hash, data []byte) []byte {
	offset := 14 + 33*len(topics)
	code := []byte{0x60, byte(len(data)), 0x61, byte(offset >> 8), byte(offset), 0x60, 0, 0x39}
	for i := len(topics) - 1; i >= 0; i-- {
		code = append(code, 0x7f)
		code = append(code, topics[i].Bytes()...)
	}
	code = append(code, 0x60, byte(len(data)), 0x60, 0, byte(0xa0+len(topics)), 0)
	return append(code, data...)
}

func chartWord(value *big.Int) []byte { return common.LeftPadBytes(value.Bytes(), 32) }

func localChartSource(t *testing.T) localChartChain {
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
	poolAddress, token, settlement := address("pool"), address("token"), address("settlement")
	factory, emitter := address("factory"), address("emitter")
	created := append(chartWord(big.NewInt(1)), chartWord(new(big.Int).SetBytes(token.Bytes()))...)
	factoryCode := chartEventRuntime([]common.Hash{marketCreatedTopic, common.BytesToHash(token.Bytes()), common.BytesToHash(poolAddress.Bytes()), common.BytesToHash(sender.Bytes())}, created)
	swapCode := []byte{0x60, 0xa0, 0x60, 0x60, 0x60, 0x00, 0x37, 0x42, 0x60, 0xa0, 0x52, 0x43, 0x60, 0xc0, 0x52, 0x60, 0x40, 0x35, 0x60, 0x20, 0x35, 0x60, 0x00, 0x35, 0x60, 0xe0, 0x60, 0x00, 0xa3, 0x00}
	backend := simulated.NewBackend(types.GenesisAlloc{
		sender:  {Balance: new(big.Int).Exp(big.NewInt(10), big.NewInt(21), nil)},
		factory: {Code: factoryCode},
		emitter: {Code: swapCode},
	}, func(n *node.Config, eth *ethconfig.Config) {
		chain := *eth.Genesis.Config
		chain.ChainID = big.NewInt(125)
		eth.Genesis.Config = &chain
		n.HTTPHost = "127.0.0.1"
		n.HTTPPort = port
		n.HTTPModules = []string{"eth"}
	})
	t.Cleanup(func() { _ = backend.Close() })
	backend.Commit()
	ctx := context.Background()
	client := backend.Client()
	request := func(key *ecdsa.PrivateKey, nonce uint64, to common.Address, data []byte) *types.Receipt {
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
	creation := request(key, 0, factory, nil)
	swapData := append([]byte{}, swapEventTopic.Bytes()...)
	swapData = append(swapData, common.BytesToHash(poolAddress.Bytes()).Bytes()...)
	swapData = append(swapData, common.BytesToHash(sender.Bytes()).Bytes()...)
	for _, amount := range []int64{1, 1000000, 2000000, 1000, 500000} {
		swapData = append(swapData, chartWord(big.NewInt(amount))...)
	}
	swaps := []*types.Receipt{request(key, 1, emitter, swapData), request(key, 2, emitter, swapData)}
	rpc, err := ethclient.Dial("http://127.0.0.1:" + strconv.Itoa(port))
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
	return localChartChain{rpc: rpc, creation: creation, swaps: swaps, headers: headers, pool: poolAddress, token: token, settlement: settlement, sender: sender}
}

func TestSourceVerifiedConsumerFromLocalCanonicalChain(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	chain := localChartSource(t)
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	productionMigrations(t, ctx, pool)
	write := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	stream, projection := "canonical", "market-v1"
	counts := map[int64]int{}
	for _, receipt := range append([]*types.Receipt{chain.creation}, chain.swaps...) {
		for _, log := range receipt.Logs {
			topics := make([]string, len(log.Topics))
			for i, topic := range log.Topics {
				topics[i] = topic.Hex()
			}
			encoded, err := json.Marshal(topics)
			if err != nil {
				t.Fatal(err)
			}
			write(`INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8)`, log.TxHash.Hex(), int64(log.Index), int64(log.BlockNumber), int64(log.TxIndex), log.BlockHash.Hex(), strings.ToLower(log.Address.Hex()), encoded, log.Data)
			counts[int64(log.BlockNumber)]++
		}
	}
	last := chain.swaps[1].BlockNumber.Int64()
	for height := int64(1); height <= last; height++ {
		write(`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,$4,true,true)`, stream, height, chain.headers[height].Hash, counts[height])
	}
	commit := func(id string, previous, height int64) {
		t.Helper()
		count := 0
		for current := previous + 1; current <= height; current++ {
			count += counts[current]
		}
		write(`INSERT INTO indexer.indexer_ingest_commits(chain_id,stream_id,batch_id,previous_height,last_complete_height,block_hash,record_count,outbox_count) VALUES(125,$1,$2,$3,$4,$5,$6,0)`, stream, id, previous, height, chain.headers[height].Hash, count)
	}
	creationHeight := chain.creation.BlockNumber.Int64()
	commit("activation", 0, creationHeight-1)
	commit("creation", creationHeight-1, creationHeight)
	commit("trades", creationHeight, last)
	write(`INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,$1,$2,$2)`, stream, last)
	write(`INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,$1,'local-chart',1,clock_timestamp()+interval '1 hour')`, stream)
	created := chain.creation.Logs[0]
	write(`INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$4,$5,'1',$6,$7,$8,$9,$10,$11,'{}','{}')`, strings.ToLower(chain.token.Hex()), strings.ToLower(chain.pool.Hex()), strings.ToLower(chain.sender.Hex()), strings.ToLower(created.Address.Hex()), strings.ToLower(chain.token.Hex()), creationHeight, created.BlockHash.Hex(), created.TxHash.Hex(), int64(created.TxIndex), int64(created.Index), chain.headers[creationHeight].Time)
	indexerPartitions, err := delivery.NewPartitionStore(pool, "indexer")
	if err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true}, EventTypes: map[string]bool{"market": true}}
	var fills []accounting.FillV1
	var wires [][]byte
	var publicationIDs []string
	var publications []accounting.PublicationV1
	for _, receipt := range chain.swaps {
		log := receipt.Logs[0]
		fill := accountingFixture()
		fill.Position = accounting.SourcePosition{ChainID: "125", Height: strconv.FormatUint(log.BlockNumber, 10), TransactionIndex: strconv.FormatUint(uint64(log.TxIndex), 10), LogIndex: strconv.FormatUint(uint64(log.Index), 10), TransactionHash: log.TxHash.Hex(), BlockHash: log.BlockHash.Hex()}
		fill.ActualEmitter = strings.ToLower(log.Address.Hex())
		fill.PoolAddress = strings.ToLower(chain.pool.Hex())
		fill.Token.Address = strings.ToLower(chain.token.Hex())
		fill.Settlement.Address = strings.ToLower(chain.settlement.Hex())
		fill.Attribution.TransactionSender = strings.ToLower(chain.sender.Hex())
		fill.TokenAmountRaw, fill.USDLAmountRaw, fill.PaidInputRaw, fill.FeeRaw = "2000000", "1000000", "1000000", "1000"
		fill.BlockTimestampSeconds = strconv.FormatInt(chain.headers[int64(log.BlockNumber)].Time, 10)
		fill.Provenance.DeploymentID = "local-chart"
		fill.Provenance.ReceiptSHA256 = receiptDigest(t, receipt)
		fill.Provenance.HeaderSHA256 = receiptDigest(t, chain.headers[int64(log.BlockNumber)])
		fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
		if err = accounting.ValidateFillV1(fill); err != nil {
			t.Fatal(err)
		}
		digest, err := accounting.FillDigest(fill)
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(fill)
		if err != nil {
			t.Fatal(err)
		}
		write(`INSERT INTO indexer.indexer_accounting_fills(source_fill_id,source_fact_digest,chain_id,transaction_hash,log_index,pool_address,payload) VALUES($1,$2,125,$3,$4,$5,$6)`, fill.SourceFillID, string(digest), fill.Position.TransactionHash, int64(log.Index), fill.PoolAddress, body)
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		position, err := indexerPartitions.AppendTx(ctx, tx, delivery.AppendInput{ProjectionVersion: projection, PartitionKind: "market", PartitionKey: "125:" + fill.PoolAddress, SourceEventID: fill.SourceFillID, EffectIdentity: accounting.EffectIdentity(fill.SourceFillID, "market", projection, "", "market", "125:"+fill.PoolAddress), PayloadDigest: digest, SourceOrder: fill.Position})
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		publication, err := accounting.NewMarketPublication(fill, []accounting.PartitionPosition{position})
		if err != nil {
			t.Fatal(err)
		}
		payload, err := accounting.CanonicalPublicationV1(publication)
		if err != nil {
			t.Fatal(err)
		}
		write(`INSERT INTO indexer.indexer_ingest_outbox(chain_id,transaction_hash,log_index,event_kind,payload) VALUES(125,$1,$2,'accounting.market.v1',$3)`, fill.Position.TransactionHash, int64(log.Index), payload)
		envelope := delivery.Envelope{EventID: publication.PublicationID, EventType: "market", SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: fill.Position.Height, TransactionIndex: fill.Position.TransactionIndex, LogIndex: fill.Position.LogIndex}, EmittedAt: time.Now().UTC(), TraceID: "local-chart", PayloadHash: delivery.Hash(payload), Payload: payload}
		wire, err := envelope.Encode(policy, accounting.MarketSubject)
		if err != nil {
			t.Fatal(err)
		}
		fills = append(fills, fill)
		wires = append(wires, wire)
		publicationIDs = append(publicationIDs, publication.PublicationID)
		publications = append(publications, publication)
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	deliveries, err := delivery.NewStore(pool, "candles", policy)
	if err != nil {
		t.Fatal(err)
	}
	partitions, err := delivery.NewPartitionStore(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := NewSourceVerifiedAccountingConsumer(SourceVerifiedAccountingConfig{Pool: pool, Deliveries: deliveries, Partitions: partitions, Exact: exact, ConsumerID: "charts", Generation: "local", ProjectionVersion: projection, Owner: "local-chart", SourceStream: stream, SourcePartitionSchema: "indexer", RPC: chain.rpc, MaxReplayBlocks: 4, MaxReplayFills: 10, SeriesV2: true, StreamWriter: &delivery.StreamWriter{Schema: "api_stream"}})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := consumer.config.Lease(ctx, fills[0])
	if err != nil || lease.Epoch < 1 {
		t.Fatalf("canonical pool bootstrap: %+v, %v", lease, err)
	}
	broker := newFaultBroker(t)
	nc := broker.start()
	publisher, reader := faultProvision(t, ctx, nc, accounting.MarketSubject, "local")
	for i, wire := range wires {
		if _, err = publisher.Publish(ctx, accounting.MarketSubject, publicationIDs[i], wire); err != nil {
			t.Fatal(err)
		}
	}
	consumed := 0
	for consumed < len(wires) {
		for _, message := range faultFetch(t, ctx, reader) {
			if err = consumer.Consume(ctx, message); err != nil {
				t.Fatalf("source verified delivery %d: %v", consumed, err)
			}
			consumed++
		}
	}
	var inbox, watermark int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_inbox`).Scan(&inbox); err != nil || inbox != len(fills) {
		t.Fatalf("committed source fills: %d, %v", inbox, err)
	}
	completenessConfig := consumer.completeness.store.(postgresCompletenessStore).config
	for _, tc := range []struct {
		name, statement, want string
		change                func(*accounting.PublicationV1)
	}{
		{"missing pool ownership", `DELETE FROM candles.chart_pool_fences`, pgx.ErrNoRows.Error(), nil},
		{"unavailable pool ownership", `ALTER TABLE candles.chart_pool_fences RENAME TO chart_pool_fences_held`, "chart_pool_fences", nil},
		{"missing accepted fill", `DELETE FROM candles.chart_inbox`, ErrAccountingAuthority.Error(), nil},
		{"unavailable accepted fill", `ALTER TABLE candles.chart_inbox RENAME TO chart_inbox_held`, "chart_inbox", nil},
		{"invalid partition sequence", "", "invalid syntax", func(p *accounting.PublicationV1) { p.Positions[0].Sequence = "invalid" }},
		{"missing committed position", `DELETE FROM candles.delivery_partition_commits`, ErrAccountingAuthority.Error(), nil},
		{"unavailable committed position", `ALTER TABLE candles.delivery_partition_commits RENAME TO delivery_partition_commits_held`, "delivery_partition_commits", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transaction, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = transaction.Rollback(ctx) }()
			if tc.statement != "" {
				if _, err := transaction.Exec(ctx, tc.statement); err != nil {
					t.Fatal(err)
				}
			}
			publication := publications[len(publications)-1]
			publication.Positions = append([]accounting.PartitionPosition(nil), publication.Positions...)
			if tc.change != nil {
				tc.change(&publication)
			}
			checker := postgresCompletenessTransaction{postgresSourceAuthority: postgresSourceAuthority{tx: transaction, config: completenessConfig}}
			if err := checker.Applied(ctx, publication); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("committed publication authority %s: %v", tc.name, err)
			}
		})
	}
	if err = pool.QueryRow(ctx, `SELECT watermark_height FROM candles.chart_series_clock WHERE chain_id='125' AND pool=$1`, fills[0].PoolAddress).Scan(&watermark); err != nil || watermark != int(last) {
		t.Fatalf("canonical series watermark: %d, %v", watermark, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE candles.chart_series_clock SET watermark_height=$2 WHERE chain_id='125' AND pool=$1`, fills[0].PoolAddress, last-1); err != nil {
		t.Fatal(err)
	}
	if err = consumer.RefreshCompleteness(ctx, publications[len(publications)-1]); err != nil {
		t.Fatalf("refresh committed canonical pool: %v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT watermark_height FROM candles.chart_series_clock WHERE chain_id='125' AND pool=$1`, fills[0].PoolAddress).Scan(&watermark); err != nil || watermark != int(last) {
		t.Fatalf("refreshed series watermark: %d, %v", watermark, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prior, err := consumer.config.SeriesCheckpoint(ctx, tx, publications[0])
	if err != nil || !prior.HasGaps || prior.Height != uint64(last) {
		_ = tx.Rollback(ctx)
		t.Fatalf("earlier publication retains gap marker: %+v, %v", prior, err)
	}
	changed := fills[0]
	changed.USDLAmountRaw, changed.PaidInputRaw = "1000001", "1000001"
	if err = consumer.config.Authorize(ctx, tx, changed); !errors.Is(err, ErrAccountingAuthority) {
		_ = tx.Rollback(ctx)
		t.Fatalf("source fill differs from committed fact: %v", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=typed_complete_height-1 WHERE stream_id=$1`, stream); err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = consumer.config.Authorize(ctx, tx, fills[0]); !errors.Is(err, ErrAccountingSourceUnavailable) {
		_ = tx.Rollback(ctx)
		t.Fatalf("incomplete source watermark: %v", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=typed_complete_height WHERE stream_id=$1`, stream); err != nil {
		t.Fatal(err)
	}
	previousDigest := string(delivery.Hash(wires[0]))
	if err = exact.RecordAttempt(ctx, "charts", "reprocess_local", publicationIDs[0], previousDigest, 1, "received", false); err != nil {
		t.Fatal(err)
	}
	reprocessConfig := SourceVerifiedAccountingConfig{Pool: pool, Deliveries: deliveries, Partitions: partitions, Exact: exact, ConsumerID: "charts", Generation: "reprocess_local", ProjectionVersion: projection, Owner: "local-chart", SourceStream: stream, SourcePartitionSchema: "indexer", RPC: chain.rpc, MaxReplayBlocks: 4, MaxReplayFills: 10, SeriesV2: true, StreamWriter: &delivery.StreamWriter{Schema: "api_stream"}}
	request := ReprocessRequest{PublicationID: publicationIDs[0], PreviousDigest: previousDigest, CorrectedDigest: strings.Repeat("e", 64), DecoderVersion: "accounting-fill-v2", OperatorID: "local-chart"}
	reprocessor, err := NewReprocessingAccountingConsumer(ctx, reprocessConfig, request)
	if err != nil || reprocessor.reprocess == nil || *reprocessor.reprocess != request {
		t.Fatalf("isolated audited reprocess consumer: %+v, %v", reprocessor, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*SourceVerifiedAccountingConfig, *ReprocessRequest)
	}{
		{"unisolated generation", func(c *SourceVerifiedAccountingConfig, _ *ReprocessRequest) { c.Generation = "local" }},
		{"unbound canonical source", func(c *SourceVerifiedAccountingConfig, _ *ReprocessRequest) { c.RPC = nil }},
		{"unrecorded prior digest", func(_ *SourceVerifiedAccountingConfig, r *ReprocessRequest) {
			r.PreviousDigest = strings.Repeat("0", 64)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate, authority := reprocessConfig, request
			tc.change(&candidate, &authority)
			opened, err := NewReprocessingAccountingConsumer(ctx, candidate, authority)
			if opened != nil || err == nil {
				t.Fatalf("unbound reprocess authority: %+v, %v", opened, err)
			}
		})
	}
	next, err := consumer.refreshCompletenessBatch(ctx, "", completenessBatchSize)
	if err != nil || next != "" {
		t.Fatalf("committed pool sweep cursor: %q, %v", next, err)
	}
	bootstrap := sourceBootstrapper{config: reprocessConfig, authorize: consumer.config.Authorize}
	for _, tc := range []struct {
		name, sql string
		args      []any
		want      error
	}{
		{"unavailable first market position", `ALTER VIEW indexer.indexer_market_partition_positions_v1 RENAME TO market_positions_held`, nil, nil},
		{"missing first market position", `DELETE FROM indexer.delivery_partition_positions WHERE partition_key=$1 AND sequence=1`, []any{"125:" + fills[0].PoolAddress}, ErrAccountingSourceUnavailable},
		{"malformed first market position", `UPDATE indexer.delivery_partition_positions SET position=$2 WHERE partition_key=$1 AND sequence=1`, []any{"125:" + fills[0].PoolAddress, []byte("invalid")}, nil},
		{"changed first market position", `UPDATE indexer.delivery_partition_positions SET position=$2 WHERE partition_key=$1 AND sequence=1`, []any{"125:" + fills[0].PoolAddress, []byte(`{"sequence":"2"}`)}, ErrAccountingAuthority},
		{"missing first market publication", `DELETE FROM indexer.indexer_ingest_outbox WHERE payload->>'sourceFillId'=$1`, []any{fills[0].SourceFillID}, pgx.ErrNoRows},
		{"unavailable first market publication", `ALTER VIEW indexer.indexer_accounting_publications_v1 RENAME TO market_publications_held`, nil, nil},
		{"changed first market publication", `UPDATE indexer.indexer_ingest_outbox SET payload=jsonb_set(payload,'{fill}','null'::jsonb) WHERE payload->>'sourceFillId'=$1`, []any{fills[0].SourceFillID}, nil},
		{"missing pool creation", `DELETE FROM indexer.indexer_canonical_creations WHERE pool=$1`, []any{fills[0].PoolAddress}, ErrAccountingSourceUnavailable},
		{"unavailable pool creation", `ALTER VIEW indexer.indexer_canonical_creations_v1 RENAME TO canonical_creations_held`, nil, nil},
		{"invalid creation topics", `UPDATE indexer.indexer_raw_logs SET topics='"invalid"'::jsonb WHERE transaction_hash=$1`, []any{chain.creation.TxHash.Hex()}, ErrAccountingAuthority},
		{"creation log omits pool", `UPDATE indexer.indexer_raw_logs SET topics='[]'::jsonb WHERE transaction_hash=$1`, []any{chain.creation.TxHash.Hex()}, ErrAccountingAuthority},
		{"missing creation commit", `UPDATE indexer.indexer_ingest_commits SET stream_id='other' WHERE batch_id='creation'`, nil, ErrAccountingSourceUnavailable},
		{"unavailable source commits", `ALTER VIEW indexer.indexer_ingest_commits_v1 RENAME TO ingest_commits_held`, nil, nil},
		{"missing first fill commit", `UPDATE indexer.indexer_ingest_commits SET stream_id='other' WHERE batch_id='trades'`, nil, ErrAccountingAuthority},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if _, err = tx.Exec(ctx, tc.sql, tc.args...); err != nil {
				t.Fatal(err)
			}
			_, err = bootstrap.load(ctx, tx, fills[0].PoolAddress)
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("source bootstrap admitted changed evidence: %v", err)
			}
		})
	}
	baselineTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := bootstrap.load(ctx, baselineTx, fills[0].PoolAddress)
	if err != nil {
		_ = baselineTx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = bootstrap.verify(ctx, baselineTx, baseline); err != nil {
		_ = baselineTx.Rollback(ctx)
		t.Fatalf("canonical bootstrap evidence: %v", err)
	}
	if err = baselineTx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, sql      string
		args           []any
		change         func(*chartSourceBootstrap)
		maxReplayFills int
		want           error
	}{
		{"missing creation block", "", nil, func(b *chartSourceBootstrap) { b.Creation.Height += 1000 }, 0, nil},
		{"changed creation block", "", nil, func(b *chartSourceBootstrap) { b.Creation.BlockHash = "0x" + strings.Repeat("f", 64) }, 0, ErrAccountingAuthority},
		{"missing creation receipt", "", nil, func(b *chartSourceBootstrap) { b.Creation.TransactionHash = "0x" + strings.Repeat("f", 64) }, 0, nil},
		{"changed creation transaction order", "", nil, func(b *chartSourceBootstrap) { b.Creation.TransactionIndex++ }, 0, ErrAccountingAuthority},
		{"changed creation log order", "", nil, func(b *chartSourceBootstrap) { b.Creation.LogIndex++ }, 0, ErrAccountingAuthority},
		{"changed creation topic", "", nil, func(b *chartSourceBootstrap) { b.Creation.Topics[0] = "0x" + strings.Repeat("f", 64) }, 0, ErrAccountingAuthority},
		{"missing commit block", "", nil, func(b *chartSourceBootstrap) { b.FirstCommitLastHeight += 1000 }, 0, nil},
		{"changed commit block", "", nil, func(b *chartSourceBootstrap) { b.FirstCommitBlockHash = "0x" + strings.Repeat("f", 64) }, 0, ErrAccountingAuthority},
		{"unavailable committed range", `ALTER VIEW indexer.indexer_ingest_commits_v1 RENAME TO ingest_commits_held`, nil, nil, 0, nil},
		{"missing first fill block", "", nil, func(b *chartSourceBootstrap) { b.Fill.Position.Height = "999999" }, 0, nil},
		{"changed fill block", "", nil, func(b *chartSourceBootstrap) { b.Fill.Position.BlockHash = "0x" + strings.Repeat("f", 64) }, 0, ErrAccountingAuthority},
		{"changed fill time", "", nil, func(b *chartSourceBootstrap) { b.Fill.BlockTimestampSeconds = "1" }, 0, ErrAccountingAuthority},
		{"bounded first fill replay", "", nil, func(b *chartSourceBootstrap) {
			b.FirstCommitLastHeight = last
			b.FirstCommitBlockHash = chain.headers[last].Hash
			b.Fill.Position.Height = strconv.FormatInt(last, 10)
			b.Fill.Position.BlockHash = chain.headers[last].Hash
			b.Fill.BlockTimestampSeconds = strconv.FormatInt(chain.headers[last].Time, 10)
		}, 1, ErrAccountingSourceUnavailable},
		{"omitted earlier swap", "", nil, func(b *chartSourceBootstrap) { b.Fill.Position.LogIndex = "999" }, 0, ErrAccountingAuthority},
		{"missing pool swap", "", nil, func(b *chartSourceBootstrap) { b.Fill.PoolAddress = "0x" + strings.Repeat("f", 40) }, 0, ErrAccountingAuthority},
		{"changed indexed swap", `UPDATE indexer.indexer_raw_logs SET data=$2 WHERE transaction_hash=$1`, []any{fills[0].Position.TransactionHash, []byte{1}}, nil, 0, ErrAccountingAuthority},
		{"missing committed fill", `DELETE FROM indexer.indexer_accounting_fills WHERE source_fill_id=$1`, []any{fills[0].SourceFillID}, nil, 0, ErrAccountingAuthority},
		{"incomplete replay block", `UPDATE indexer.indexer_block_completeness SET fetch_complete=false WHERE stream_id=$1 AND height=$2`, []any{stream, chain.creation.BlockNumber.Int64()}, nil, 0, ErrAccountingAuthority},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if tc.sql != "" {
				if _, err = tx.Exec(ctx, tc.sql, tc.args...); err != nil {
					t.Fatal(err)
				}
			}
			changed := baseline
			changed.Creation.Topics = append([]string(nil), baseline.Creation.Topics...)
			if tc.change != nil {
				tc.change(&changed)
			}
			verifier := bootstrap
			if tc.maxReplayFills > 0 {
				verifier.config.MaxReplayFills = tc.maxReplayFills
			}
			err = verifier.verify(ctx, tx, changed)
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("changed canonical bootstrap evidence admitted: %v", err)
			}
		})
	}
	for _, tc := range []struct {
		name, sql string
		args      []any
		call      func(pgx.Tx) error
		want      error
	}{
		{"missing source watermark", `DELETE FROM indexer.indexer_ingest_watermarks WHERE stream_id=$1`, []any{stream}, func(tx pgx.Tx) error { return consumer.config.Authorize(ctx, tx, fills[0]) }, ErrAccountingSourceUnavailable},
		{"missing source lease", `DELETE FROM indexer.indexer_ingest_leases WHERE stream_id=$1`, []any{stream}, func(tx pgx.Tx) error { return consumer.config.Authorize(ctx, tx, fills[0]) }, ErrAccountingSourceUnavailable},
		{"changed source raw log", `UPDATE indexer.indexer_raw_logs SET block_hash=$2 WHERE transaction_hash=$1`, []any{fills[0].Position.TransactionHash, "0x" + strings.Repeat("f", 64)}, func(tx pgx.Tx) error { return consumer.config.Authorize(ctx, tx, fills[0]) }, ErrAccountingAuthority},
		{"changed source publication", `UPDATE indexer.indexer_ingest_outbox SET payload='{}'::jsonb WHERE transaction_hash=$1`, []any{fills[0].Position.TransactionHash}, func(tx pgx.Tx) error { return consumer.config.AuthorizePublication(ctx, tx, publications[0]) }, ErrAccountingAuthority},
		{"unavailable source positions", `ALTER VIEW indexer.indexer_market_partition_positions_v1 RENAME TO indexer_market_partition_positions_held`, nil, func(tx pgx.Tx) error {
			_, err := consumer.config.SeriesCheckpoint(ctx, tx, publications[1])
			return err
		}, nil},
		{"missing checkpoint block", `DELETE FROM indexer.indexer_block_completeness WHERE stream_id=$1 AND height=$2`, []any{stream, last}, func(tx pgx.Tx) error {
			_, err := consumer.config.SeriesCheckpoint(ctx, tx, publications[1])
			return err
		}, pgx.ErrNoRows},
		{"changed checkpoint block", `UPDATE indexer.indexer_block_completeness SET block_hash=$3 WHERE stream_id=$1 AND height=$2`, []any{stream, last, "0x" + strings.Repeat("f", 64)}, func(tx pgx.Tx) error {
			_, err := consumer.config.SeriesCheckpoint(ctx, tx, publications[1])
			return err
		}, ErrAccountingAuthority},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if _, err = tx.Exec(ctx, tc.sql, tc.args...); err != nil {
				t.Fatal(err)
			}
			err = tc.call(tx)
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("source authority %s: %v", tc.name, err)
			}
		})
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	authorityReader := postgresSourceAuthority{tx: tx, config: reprocessConfig}
	for _, tc := range []struct {
		name string
		edit func(*accounting.FillV1)
	}{
		{"height", func(fill *accounting.FillV1) { fill.Position.Height = "invalid" }},
		{"transaction order", func(fill *accounting.FillV1) { fill.Position.TransactionIndex = "invalid" }},
		{"log order", func(fill *accounting.FillV1) { fill.Position.LogIndex = "invalid" }},
	} {
		changed := fills[0]
		tc.edit(&changed)
		if same, err := authorityReader.MatchesFill(ctx, changed, "", nil); same || err == nil {
			t.Fatalf("invalid %s source position: %t, %v", tc.name, same, err)
		}
	}
	invalidPublication := publications[0]
	invalidPublication.Positions = append([]accounting.PartitionPosition(nil), invalidPublication.Positions...)
	invalidPublication.Positions[0].Sequence = "invalid"
	if same, err := authorityReader.MatchesPublication(ctx, invalidPublication, nil, nil); same || err == nil {
		t.Fatalf("invalid publication sequence: %t, %v", same, err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := consumer.Consume(ctx, nil); err == nil || !strings.Contains(err.Error(), "real bounded market broker message required") {
		t.Fatalf("missing broker delivery: %v", err)
	}
	if err := consumer.Run(ctx, nil); err == nil || !strings.Contains(err.Error(), "real broker driver required") {
		t.Fatalf("missing broker driver: %v", err)
	}
	envelope, err := delivery.Decode(wires[0], policy, accounting.MarketSubject)
	if err != nil {
		t.Fatal(err)
	}
	envelope.TraceID = delivery.Hash([]byte(publicationIDs[0] + "/changed-broker-publication"))
	changedWire, err := envelope.Encode(policy, accounting.MarketSubject)
	if err != nil || string(changedWire) == string(wires[0]) {
		t.Fatalf("changed broker envelope: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	if _, err := publisher.Publish(ctx, accounting.MarketSubject, publicationIDs[0], changedWire); err != nil {
		t.Fatal(err)
	}
	changedMessages := faultFetch(t, ctx, reader)
	if len(changedMessages) != 1 || string(changedMessages[0].Wire) != string(changedWire) {
		t.Fatalf("changed retained broker publication: %d", len(changedMessages))
	}
	if err := reprocessor.Consume(ctx, changedMessages[0]); !errors.Is(err, ErrAccountingAuthority) {
		t.Fatalf("reprocess digest differs from admitted correction: %v", err)
	}
	unadmitted := *reprocessor
	unadmitted.reprocess = &ReprocessRequest{PublicationID: publicationIDs[0], CorrectedDigest: delivery.Hash(changedWire)}
	if err := unadmitted.Consume(ctx, changedMessages[0]); err == nil {
		t.Fatal("unadmitted corrected broker delivery was accepted")
	}
	if err := consumer.Consume(ctx, changedMessages[0]); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("conflicting broker publication identity: %v", err)
	}
	var frozen bool
	if err := pool.QueryRow(ctx, `SELECT frozen FROM candles.chart_pool_fences WHERE chain_id='125' AND pool=$1`, fills[0].PoolAddress).Scan(&frozen); err != nil || !frozen {
		t.Fatalf("source identity conflict did not freeze pool: %t, %v", frozen, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_inbox`).Scan(&inbox); err != nil || inbox != len(fills) {
		t.Fatalf("conflicting source changed accepted fills: %d, %v", inbox, err)
	}
	otherPool := "0x" + strings.Repeat("c", 40)
	if _, err := pool.Exec(ctx, `UPDATE candles.chart_inbox SET pool=$2 WHERE tx_hash=$1`, fills[0].Position.TransactionHash, otherPool); err != nil {
		t.Fatal(err)
	}
	conflicted := *consumer
	conflicted.config.Generation = "attempt_conflict"
	if err := exact.RecordAttempt(ctx, "charts", conflicted.config.Generation, changedMessages[0].EventID, strings.Repeat("f", 64), int64(changedMessages[0].Sequence), "received", false); err != nil {
		t.Fatal(err)
	}
	if err := conflicted.Consume(ctx, changedMessages[0]); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("conflicting durable broker digest: %v", err)
	}
	var quarantine int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.delivery_ordered_quarantine WHERE consumer_id='charts' AND projection_generation=$1`, conflicted.config.Generation).Scan(&quarantine); err != nil || quarantine != 1 {
		t.Fatalf("conflicting broker identity quarantine: %d, %v", quarantine, err)
	}
	if err := pool.QueryRow(ctx, `SELECT frozen FROM candles.chart_pool_fences WHERE chain_id='125' AND pool=$1`, otherPool).Scan(&frozen); err != nil || !frozen {
		t.Fatalf("conflicting original pool fence: %t, %v", frozen, err)
	}
}
