package engine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/constants"
	"github.com/ethereum/go-ethereum/ethclient"
	goredis "github.com/redis/go-redis/v9"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// This peer is a real embedded transactional database. The builder, monetary
// fold, persisted codecs and committed relay are production code.
type repair1051Database struct{ db *leveldb.DB }
type repair1051Transaction struct{ tx *leveldb.Transaction }

func repair1051Open(t *testing.T, path string) *repair1051Database {
	t.Helper()
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &repair1051Database{db: db}
}
func (s *repair1051Database) Transaction(ctx context.Context, pool, identity string, apply func(developmentKV) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tx, err := s.db.OpenTransaction()
	if err != nil {
		return err
	}
	defer tx.Discard()
	if err = apply(&repair1051Transaction{tx: tx}); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return tx.Commit()
}
func (t *repair1051Transaction) Get(key string) ([]byte, error) {
	raw, err := t.tx.Get([]byte(key), nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		return nil, nil
	}
	return raw, err
}
func (t *repair1051Transaction) Put(key string, raw []byte) error {
	return t.tx.Put([]byte(key), raw, &opt.WriteOptions{Sync: true})
}
func (t *repair1051Transaction) Claim(swap SwapEvent) (bool, error) {
	key, raw, err := developmentClaim(swap)
	if err != nil {
		return false, err
	}
	old, err := t.Get(key)
	if err != nil {
		return false, err
	}
	if len(old) > 0 {
		if !bytes.Equal(old, raw) {
			return false, errors.New("conflicting source identity")
		}
		return false, nil
	}
	return true, t.Put(key, raw)
}
func repair1051Read(t *testing.T, db *repair1051Database, key string, target any) bool {
	t.Helper()
	raw, err := db.db.Get([]byte(key), nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
	return true
}
func repair1051Candle(t *testing.T, db *repair1051Database, pool, tf string, timestamp int64) developmentCandle {
	t.Helper()
	start := timestamp / constants.Timeframes[tf] * constants.Timeframes[tf]
	var c developmentCandle
	if !repair1051Read(t, db, developmentKey("candle", pool, tf, strconv.FormatInt(start, 10)), &c) {
		t.Fatal("missing candle", tf, start)
	}
	return c
}
func repair1051Pending(t *testing.T, db *repair1051Database, pool string) []developmentDelivery {
	t.Helper()
	var pending []developmentDelivery
	repair1051Read(t, db, developmentKey("outbox", pool), &pending)
	return pending
}

// A loopback RESP transcript peer exercises the real go-redis codec and EVAL
// call. Replies are committed fixtures; it does not emulate a Redis database.
type repair1051Redis struct {
	listener    net.Listener
	mu          sync.Mutex
	connections map[net.Conn]bool
	events      []CandleUpdateEvent
	reply       []byte
	dropAck     bool
	check       func(CandleUpdateEvent) error
	failures    chan error
	wg          sync.WaitGroup
}

func repair1051Fixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "repair_m1051", name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func repair1051StartRedis(t *testing.T, reply string) (*repair1051Redis, *goredis.Client) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	peer := &repair1051Redis{listener: listener, connections: map[net.Conn]bool{}, reply: repair1051Fixture(t, reply), failures: make(chan error, 256)}
	peer.wg.Add(1)
	go func() {
		defer peer.wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			peer.mu.Lock()
			peer.connections[conn] = true
			peer.mu.Unlock()
			peer.wg.Add(1)
			go peer.serve(conn)
		}
	}()
	client := goredis.NewClient(&goredis.Options{Addr: listener.Addr().String(), Protocol: 2, DisableIdentity: true, MaxRetries: -1, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second})
	t.Cleanup(func() { client.Close(); peer.close(t) })
	return peer, client
}
func repair1051Command(reader *bufio.Reader) ([]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(line, "*") {
		return nil, errors.New("non-array RESP command")
	}
	count, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil || count < 1 || count > 16 {
		return nil, errors.New("RESP argument bound")
	}
	parts := make([]string, count)
	for i := range parts {
		line, err = reader.ReadString('\n')
		if err != nil || !strings.HasPrefix(line, "$") {
			return nil, errors.New("invalid bulk argument")
		}
		size, err := strconv.Atoi(strings.TrimSpace(line[1:]))
		if err != nil || size < 0 || size > 1<<20 {
			return nil, errors.New("RESP bulk bound")
		}
		raw := make([]byte, size+2)
		if _, err = io.ReadFull(reader, raw); err != nil {
			return nil, err
		}
		if string(raw[size:]) != "\r\n" {
			return nil, errors.New("invalid RESP terminator")
		}
		parts[i] = string(raw[:size])
	}
	return parts, nil
}
func (p *repair1051Redis) serve(conn net.Conn) {
	defer p.wg.Done()
	defer conn.Close()
	defer func() { p.mu.Lock(); delete(p.connections, conn); p.mu.Unlock() }()
	reader := bufio.NewReader(conn)
	for {
		parts, err := repair1051Command(reader)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				p.failures <- err
			}
			return
		}
		switch strings.ToLower(parts[0]) {
		case "hello":
			_, err = io.WriteString(conn, "-ERR unknown command 'hello'\r\n")
		case "client":
			_, err = io.WriteString(conn, "+OK\r\n")
		case "eval":
			if len(parts) != 7 || parts[1] != developmentPublishScript || parts[2] != "1" || parts[5] != constants.ChannelCandleUpdate {
				p.failures <- errors.New("unexpected committed-delivery protocol")
				return
			}
			var event CandleUpdateEvent
			if err = json.Unmarshal([]byte(parts[6]), &event); err != nil {
				p.failures <- err
				return
			}
			expectedFence := developmentKey("candle:delivered", event.PoolAddress, event.Timeframe, strconv.FormatInt(event.CandleStart, 10))
			if parts[3] != expectedFence || parts[4] != strconv.Itoa(event.Revision) || event.Revision != event.TradeCount || event.EventID == "" {
				p.failures <- errors.New("unbound delivery revision or identity")
				return
			}
			p.mu.Lock()
			p.events = append(p.events, event)
			check, drop, reply := p.check, p.dropAck, append([]byte(nil), p.reply...)
			p.mu.Unlock()
			if check != nil {
				if err = check(event); err != nil {
					p.failures <- err
					return
				}
			}
			if drop {
				return
			}
			_, err = conn.Write(reply)
		default:
			p.failures <- fmt.Errorf("unexpected RESP command %q", parts[0])
			return
		}
		if err != nil {
			return
		}
	}
}
func (p *repair1051Redis) close(t *testing.T) {
	t.Helper()
	p.listener.Close()
	p.mu.Lock()
	for conn := range p.connections {
		conn.Close()
	}
	p.mu.Unlock()
	p.wg.Wait()
	for {
		select {
		case err := <-p.failures:
			t.Error(err)
		default:
			return
		}
	}
}
func (p *repair1051Redis) snapshot() []CandleUpdateEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]CandleUpdateEvent(nil), p.events...)
}
func repair1051Builder(db *repair1051Database, client *goredis.Client) *Builder {
	b := New(nil, client, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	b.storage = db
	return b
}
func repair1051Swap(index int, timestamp int64, price string) SwapEvent {
	return SwapEvent{PoolAddress: "0x" + strings.Repeat("a", 40), Sender: "0x" + fmt.Sprintf("%040x", index+1), IsBuy: true,
		AmountIn: "100", AmountOut: "200", Fee: "1", Price: price, BlockTimestamp: timestamp,
		TxHash: "0x" + fmt.Sprintf("%064x", index+1), LogIndex: 0, Position: &ChainOrder{Height: uint64(100 + index)}}
}

func TestRepairM_1051_ConcurrentDistinctSwaps(t *testing.T) {
	db := repair1051Open(t, filepath.Join(t.TempDir(), "state"))
	defer db.db.Close()
	peer, client := repair1051StartRedis(t, "redis-ack.resp")
	builders := []*Builder{repair1051Builder(db, client), repair1051Builder(db, client)}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	base := int64(1704067260)
	const swaps = 24
	for phase := 0; phase < 2; phase++ {
		// Phase 0 contends on the first insertion; phase 1 on existing buckets.
		start := make(chan struct{})
		results := make(chan error, swaps)
		for i := 0; i < swaps; i++ {
			go func(i int) {
				<-start
				swap := repair1051Swap(phase*swaps+i, base+int64(i%20), "1000000000000000000")
				if i%2 == 1 {
					swap.IsBuy = false
					swap.AmountIn, swap.AmountOut = "200", "100"
				}
				results <- builders[i%2].ProcessSwap(ctx, swap)
			}(i)
		}
		close(start)
		for i := 0; i < swaps; i++ {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		total := (phase + 1) * swaps
		for _, tf := range constants.TimeframeKeys {
			c := repair1051Candle(t, db, repair1051Swap(0, base, "1").PoolAddress, tf, base)
			if c.Row.TradeCount != total || c.Row.VolumeUsdl != strconv.Itoa(total*100) || c.Row.VolumeToken != strconv.Itoa(total*200) || c.Row.BuyVolumeUsdl != strconv.Itoa(total*50) || c.Row.SellVolumeUsdl != strconv.Itoa(total*50) || c.Row.UniqueTraders != total || len(c.Traders) != total {
				t.Fatalf("lost committed concurrent economics %s: %+v", tf, c.Row)
			}
		}
	}
	if len(repair1051Pending(t, db, repair1051Swap(0, base, "1").PoolAddress)) != 0 {
		t.Fatal("committed delivery left pending")
	}
	if got := len(peer.snapshot()); got != 2*swaps*len(constants.TimeframeKeys) {
		t.Fatal("missing committed delivery", got)
	}
}

func TestRepairM_1051_RollbackAndRedisRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	db := repair1051Open(t, path)
	peer, client := repair1051StartRedis(t, "redis-ack.resp")
	b := repair1051Builder(db, client)
	ctx := context.Background()
	first := repair1051Swap(0, 1704067260, "1000000000000000000")
	if err := b.ProcessSwap(ctx, first); err != nil {
		t.Fatal(err)
	}
	original := repair1051Candle(t, db, first.PoolAddress, "5m", first.BlockTimestamp)
	key := developmentKey("candle", first.PoolAddress, "5m", strconv.FormatInt(original.Row.CandleStart, 10))
	corrupt := original
	corrupt.Row.High = "corrupt persisted amount"
	if err := db.Transaction(ctx, first.PoolAddress, "", func(tx developmentKV) error { return saveDevelopment(tx, key, corrupt) }); err != nil {
		t.Fatal(err)
	}
	before := repair1051Candle(t, db, first.PoolAddress, "1m", first.BlockTimestamp)
	second := repair1051Swap(1, first.BlockTimestamp+1, "2000000000000000000")
	if err := b.ProcessSwap(ctx, second); err == nil {
		t.Fatal("corrupt later timeframe should roll back all effects")
	}
	if got := repair1051Candle(t, db, first.PoolAddress, "1m", first.BlockTimestamp); !reflect.DeepEqual(got, before) {
		t.Fatal("early timeframe or trader escaped rollback", got)
	}
	if len(peer.snapshot()) != len(constants.TimeframeKeys) || len(repair1051Pending(t, db, first.PoolAddress)) != 0 {
		t.Fatal("phantom rolled-back publication")
	}
	claim, _, _ := developmentClaim(second)
	if exists, err := db.db.Has([]byte(claim), nil); err != nil || exists {
		t.Fatal("rolled-back claim survived", err)
	}
	if err := db.Transaction(ctx, first.PoolAddress, "", func(tx developmentKV) error { return saveDevelopment(tx, key, original) }); err != nil {
		t.Fatal(err)
	}
	// Stop both the builder's database and the Redis transport, then reopen.
	client.Close()
	peer.close(t)
	if err := db.db.Close(); err != nil {
		t.Fatal(err)
	}
	b = nil
	db = repair1051Open(t, path)
	defer db.db.Close()
	restartedPeer, restartedClient := repair1051StartRedis(t, "redis-ack.resp")
	b = repair1051Builder(db, restartedClient)
	restartedPeer.mu.Lock()
	restartedPeer.check = func(event CandleUpdateEvent) error {
		var committed developmentCandle
		raw, err := db.db.Get([]byte(developmentKey("candle", event.PoolAddress, event.Timeframe, strconv.FormatInt(event.CandleStart, 10))), nil)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(raw, &committed); err != nil {
			return err
		}
		if committed.Row.TradeCount != event.TradeCount || committed.Row.UniqueTraders != event.UniqueTraders {
			return errors.New("uncommitted state reached Redis")
		}
		return nil
	}
	restartedPeer.mu.Unlock()
	if err := b.ProcessSwap(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := b.ProcessSwap(ctx, second); err != nil {
		t.Fatal("durable duplicate retry", err)
	}
	third := repair1051Swap(2, first.BlockTimestamp+2, "3000000000000000000")
	third.Sender = first.Sender
	if err := b.ProcessSwap(ctx, third); err != nil {
		t.Fatal(err)
	}
	for _, tf := range constants.TimeframeKeys {
		got := repair1051Candle(t, db, first.PoolAddress, tf, first.BlockTimestamp)
		if got.Row.TradeCount != 3 || got.Row.UniqueTraders != 2 || got.Row.VolumeUsdl != "300" || len(got.Traders) != 2 {
			t.Fatal("restart changed committed economics", tf, got)
		}
	}
	if len(restartedPeer.snapshot()) != 2*len(constants.TimeframeKeys) {
		t.Fatal("restart or retry emitted a duplicate/phantom fold")
	}
}

func TestRepairM_1051_CommittedOutboxRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	db := repair1051Open(t, path)
	peer, client := repair1051StartRedis(t, "redis-error.resp")
	b := repair1051Builder(db, client)
	swap := repair1051Swap(0, 1704067260, "1000000000000000000")
	ctx := context.Background()
	if err := b.ProcessSwap(ctx, swap); err == nil {
		t.Fatal("transport failure was swallowed")
	}
	pending := repair1051Pending(t, db, swap.PoolAddress)
	if len(pending) != len(constants.TimeframeKeys) {
		t.Fatal("failed delivery lost committed outbox", pending)
	}
	for _, tf := range constants.TimeframeKeys {
		c := repair1051Candle(t, db, swap.PoolAddress, tf, swap.BlockTimestamp)
		if c.Row.TradeCount != 1 || c.Row.UniqueTraders != 1 {
			t.Fatal("delivery failure altered commit", c)
		}
	}
	client.Close()
	peer.close(t)
	if err := db.db.Close(); err != nil {
		t.Fatal(err)
	}
	db = repair1051Open(t, path)
	defer db.db.Close()
	// An ambiguous delivery acknowledgement must retain the exact same event
	// identity and bytes. The duplicate transcript acknowledges the retry.
	retryPeer, retryClient := repair1051StartRedis(t, "redis-duplicate.resp")
	retryPeer.mu.Lock()
	retryPeer.dropAck = true
	retryPeer.mu.Unlock()
	b = repair1051Builder(db, retryClient)
	if err := b.ProcessSwap(ctx, swap); err == nil {
		t.Fatal("lost acknowledgement accepted")
	}
	if !reflect.DeepEqual(repair1051Pending(t, db, swap.PoolAddress), pending) {
		t.Fatal("ambiguous acknowledgement changed outbox")
	}
	retryPeer.mu.Lock()
	retryPeer.dropAck = false
	retryPeer.mu.Unlock()
	if err := b.ProcessSwap(ctx, swap); err != nil {
		t.Fatal(err)
	}
	received := retryPeer.snapshot()
	if len(received) != 1+len(pending) || !reflect.DeepEqual(received[0], received[1]) {
		t.Fatal("retry identity or payload changed", received)
	}
	for i, record := range pending {
		if !reflect.DeepEqual(record.Event, received[i+1]) {
			t.Fatal("outbox transport/consumer mismatch", i)
		}
	}
	if len(repair1051Pending(t, db, swap.PoolAddress)) != 0 {
		t.Fatal("acknowledged outbox retained")
	}
	if err := b.ProcessSwap(ctx, swap); err != nil {
		t.Fatal(err)
	}
	if len(retryPeer.snapshot()) != len(received) {
		t.Fatal("acknowledged event republished")
	}
}

func TestRepairM_1051_LateSourceOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	db := repair1051Open(t, path)
	_, client := repair1051StartRedis(t, "redis-ack.resp")
	b := repair1051Builder(db, client)
	ctx := context.Background()
	base := int64(1704067260)
	latest := repair1051Swap(30, base+190, "9000000000000000000")
	if err := b.ProcessSwap(ctx, latest); err != nil {
		t.Fatal(err)
	}
	cursorKey := developmentKey("cursor", latest.PoolAddress, "1m")
	var cursor store.CursorRow
	if !repair1051Read(t, db, cursorKey, &cursor) {
		t.Fatal("cursor missing")
	}
	// A missing old bucket must not inherit the future cursor's price.
	late := repair1051Swap(10, base+10, "3000000000000000000")
	if err := b.ProcessSwap(ctx, late); err != nil {
		t.Fatal(err)
	}
	// Stop/reopen before an earlier source position reaches that same bucket.
	if err := db.db.Close(); err != nil {
		t.Fatal(err)
	}
	db = repair1051Open(t, path)
	defer db.db.Close()
	b = repair1051Builder(db, client)
	earliest := repair1051Swap(5, base+5, "1000000000000000000")
	if err := b.ProcessSwap(ctx, earliest); err != nil {
		t.Fatal(err)
	}
	middle := repair1051Swap(7, base+7, "5000000000000000000")
	if err := b.ProcessSwap(ctx, middle); err != nil {
		t.Fatal(err)
	}
	old := repair1051Candle(t, db, latest.PoolAddress, "1m", base)
	if old.Row.Open != earliest.Price || old.Row.Close != late.Price || old.Row.High != middle.Price || old.Row.Low != earliest.Price || old.Row.TradeCount != 3 || old.Row.LastTradeTs != late.BlockTimestamp || old.First != *earliest.Position || old.Last != *late.Position {
		t.Fatal("arrival order contaminated historical OHLC", old)
	}
	var after store.CursorRow
	repair1051Read(t, db, cursorKey, &after)
	if after != cursor {
		t.Fatal("old missing/existing bucket moved latest cursor", cursor, after)
	}
	// Late source order in the latest existing bucket must preserve its close.
	existing := repair1051Swap(25, latest.BlockTimestamp-1, "7000000000000000000")
	if err := b.ProcessSwap(ctx, existing); err != nil {
		t.Fatal(err)
	}
	current := repair1051Candle(t, db, latest.PoolAddress, "1m", latest.BlockTimestamp)
	if current.Row.Open != existing.Price || current.Row.Close != latest.Price || current.Row.LastTradeTs != latest.BlockTimestamp {
		t.Fatal("latest close follows arrival order", current)
	}
	repair1051Read(t, db, cursorKey, &after)
	if after != cursor {
		t.Fatal("late existing-bucket close changed cursor", after)
	}
	// Equal timestamps still order by transaction and log position.
	tied := repair1051Swap(31, latest.BlockTimestamp, "11000000000000000000")
	tied.Position = &ChainOrder{Height: latest.Position.Height, TransactionIndex: 1, LogIndex: 0}
	if err := b.ProcessSwap(ctx, tied); err != nil {
		t.Fatal(err)
	}
	repair1051Read(t, db, cursorKey, &after)
	if after.LastClose != tied.Price || after.LastCandleStart != cursor.LastCandleStart {
		t.Fatal("within-block source order lost", after)
	}
	invalid := repair1051Swap(40, base, "1")
	invalid.Position = nil
	if err := b.ProcessSwap(ctx, invalid); err == nil {
		t.Fatal("unavailable source ordering was guessed")
	}
}

func TestRepairM_1051_ArchiveNodeIdentity(t *testing.T) {
	raw := repair1051Fixture(t, "block.json")
	var block struct {
		Hash      string `json:"hash"`
		Number    string `json:"number"`
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal(raw, &block); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "bad request", 400)
			return
		}
		if request.Method != "eth_getBlockByNumber" || len(request.Params) != 2 || string(request.Params[0]) != `"0x65"` || string(request.Params[1]) != "false" {
			t.Error("unbound block request", request)
			http.Error(w, "bad request", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Version string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Result  json.RawMessage `json:"result"`
		}{"2.0", request.ID, raw})
	}))
	defer server.Close()
	rpc, err := ethclient.DialContext(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer rpc.Close()
	header, err := rpc.HeaderByNumber(context.Background(), big.NewInt(101))
	if err != nil {
		t.Fatal(err)
	}
	if header.Hash().Hex() == block.Hash {
		t.Fatal("fixture does not distinguish node identity from local rehash")
	}
	reader := store.ArchiveReader{BlockTime: archiveBlockTime(chainread.Reader{RPC: rpc})}
	timestamp, err := reader.BlockTime(context.Background(), 101, block.Hash)
	if err != nil || timestamp != int64(header.Time) {
		t.Fatal("committed node-reported identity refused", timestamp, err)
	}
	for _, foreign := range []string{header.Hash().Hex(), "0x" + strings.Repeat("f", 64)} {
		if _, err = reader.BlockTime(context.Background(), 101, foreign); !errors.Is(err, store.ErrIntegrity) {
			t.Fatal("foreign archive hash accepted", foreign, err)
		}
	}
}
