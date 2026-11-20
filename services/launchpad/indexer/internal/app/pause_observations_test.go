package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
)

type pauseChain struct {
	ctx                           context.Context
	client                        *ethclient.Client
	auth                          *bind.TransactOpts
	observation                   *pauseObservations
	factory, beacon               *bind.BoundContract
	factoryAddress, beaconAddress common.Address
	factoryABI, poolABI           abi.ABI
	endpoint                      string
	through                       int64
}

func pauseArtifact(t *testing.T, name string) (abi.ABI, []byte) {
	t.Helper()
	var paths []string
	err := filepath.WalkDir("../../../../../contracts/launchpad/artifacts/contracts", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == name+".json" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil || len(paths) != 1 {
		t.Fatalf("compiled contract %s: %v %v", name, paths, err)
	}
	body, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		ABI      json.RawMessage
		Bytecode string
	}
	if err = json.Unmarshal(body, &artifact); err != nil {
		t.Fatal(err)
	}
	parsed, err := abi.JSON(bytes.NewReader(artifact.ABI))
	if err != nil {
		t.Fatal(err)
	}
	code := common.FromHex(artifact.Bytecode)
	if len(code) == 0 {
		t.Fatal("contract deployment code required")
	}
	return parsed, code
}

func (c *pauseChain) mined(t *testing.T, tx *types.Transaction) *types.Receipt {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		receipt, err := c.client.TransactionReceipt(c.ctx, tx.Hash())
		if err == nil {
			if receipt == nil || receipt.Status != types.ReceiptStatusSuccessful {
				t.Fatalf("mined contract transaction: %v", receipt)
			}
			return receipt
		}
		if !errors.Is(err, ethereum.NotFound) {
			t.Fatalf("contract receipt: %v", err)
		}
		select {
		case <-c.ctx.Done():
			t.Fatal(c.ctx.Err())
		case <-ticker.C:
		}
	}
}

func (c *pauseChain) deploy(t *testing.T, name string, args ...any) (common.Address, *bind.BoundContract, abi.ABI) {
	t.Helper()
	parsed, code := pauseArtifact(t, name)
	address, tx, contract, err := bind.DeployContract(c.auth, parsed, code, c.client, args...)
	if err != nil {
		t.Fatalf("deploy %s: %v", name, err)
	}
	c.mined(t, tx)
	return address, contract, parsed
}
func (c *pauseChain) proxy(t *testing.T, name string, args ...any) (common.Address, *bind.BoundContract) {
	t.Helper()
	implementation, _, parsed := c.deploy(t, name)
	input, err := parsed.Pack("initialize", args...)
	if err != nil {
		t.Fatal(err)
	}
	address, _, _ := c.deploy(t, "ERC1967Proxy", implementation, input)
	return address, bind.NewBoundContract(address, parsed, c.client, c.client, c.client)
}
func (c *pauseChain) transact(t *testing.T, contract *bind.BoundContract, method string, args ...any) *types.Receipt {
	t.Helper()
	tx, err := contract.Transact(c.auth, method, args...)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return c.mined(t, tx)
}
func newPauseChain(t *testing.T) *pauseChain {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	t.Cleanup(cancel)
	endpoint := lifecycleNode(t)
	client, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	key, err := crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(125))
	if err != nil {
		t.Fatal(err)
	}
	auth.Context = ctx
	auth.GasLimit = 8000000
	auth.GasPrice = big.NewInt(2000000000)
	_, pool := canonicaltest.New(t)
	partitions, err := delivery.PartitionSchemaDDL("indexer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, partitions); err != nil {
		t.Fatal(err)
	}
	ddl, err := os.ReadFile("../../../../../migrations/190001.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(ddl)); err != nil {
		t.Fatal(err)
	}
	c := &pauseChain{ctx: ctx, client: client, auth: auth, endpoint: endpoint, observation: &pauseObservations{pool: pool, rpc: client, stream: "ranking-pause"}}
	usdl, _, _ := c.deploy(t, "SidioraERC20", "USD Ledger", "USDL", big.NewInt(1000000000000000), auth.From)
	config, _ := c.proxy(t, "ProtocolConfig", usdl, common.Address{}, auth.From)
	treasury, _ := c.proxy(t, "Treasury", common.Address{}, auth.From)
	registry, registryContract := c.proxy(t, "PoolRegistry", common.Address{}, auth.From)
	accumulator, accumulatorContract := c.proxy(t, "FeeAccumulator", config, treasury, registry, common.Address{}, usdl, auth.From)
	nft, nftContract := c.proxy(t, "SidioraNFT", "Pool NFT", "NFT", common.Address{}, auth.From)
	implementation, _, parsed := c.deploy(t, "SidioraPool")
	c.poolABI = parsed
	beacon, beaconContract, _ := c.deploy(t, "PoolBeacon", implementation, auth.From)
	c.beacon, c.beaconAddress = beaconContract, beacon
	factory, factoryContract := c.proxy(t, "SidioraFactory", beacon, nft, registry, common.Address{}, config, treasury, accumulator, usdl, auth.From)
	c.factory, c.factoryAddress = factoryContract, factory
	c.factoryABI, _ = pauseArtifact(t, "SidioraFactory")
	for _, contract := range []*bind.BoundContract{registryContract, accumulatorContract} {
		c.transact(t, contract, "grantRole", crypto.Keccak256Hash([]byte("FACTORY_ROLE")), factory)
	}
	c.transact(t, nftContract, "grantRole", crypto.Keccak256Hash([]byte("MINTER_ROLE")), factory)
	c.transact(t, factoryContract, "grantRole", crypto.Keccak256Hash([]byte("ROUTER_ROLE")), auth.From)
	return c
}

// The inventory and raw store retain identities and bytes emitted by the
// deployed factory and pools, including blocks with no pause events.
func (c *pauseChain) retain(t *testing.T, receipt *types.Receipt) {
	t.Helper()
	pool := c.observation.pool
	batch := new(pgx.Batch)
	for height := c.through + 1; height <= receipt.BlockNumber.Int64(); height++ {
		block, err := c.client.BlockByNumber(c.ctx, big.NewInt(height))
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, tx := range block.Transactions() {
			r, err := c.client.TransactionReceipt(c.ctx, tx.Hash())
			if err != nil {
				t.Fatal(err)
			}
			for _, event := range r.Logs {
				topics := make([]string, len(event.Topics))
				for i, topic := range event.Topics {
					topics[i] = strings.ToLower(topic.Hex())
				}
				raw, err := json.Marshal(topics)
				if err != nil {
					t.Fatal(err)
				}
				batch.Queue(`INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8)`, strings.ToLower(event.TxHash.Hex()), int64(event.Index), height, int64(event.TxIndex), strings.ToLower(event.BlockHash.Hex()), strings.ToLower(event.Address.Hex()), raw, event.Data)
				count++
				if event.Address == c.factoryAddress && event.Topics[0] == c.factoryABI.Events["MarketCreated"].ID {
					values, err := c.factoryABI.Events["MarketCreated"].Inputs.NonIndexed().Unpack(event.Data)
					if err != nil {
						t.Fatal(err)
					}
					fact := map[string]any{"token": strings.ToLower(common.BytesToAddress(event.Topics[1].Bytes()).Hex()), "pool": strings.ToLower(common.BytesToAddress(event.Topics[2].Bytes()).Hex()), "creator": strings.ToLower(common.BytesToAddress(event.Topics[3].Bytes()).Hex()), "factory": strings.ToLower(c.factoryAddress.Hex()), "nftId": values[0].(*big.Int).String()}
					payload, err := json.Marshal(fact)
					if err != nil {
						t.Fatal(err)
					}
					batch.Queue(`INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'{}',$13)`, fact["token"], fact["pool"], fact["creator"], fact["factory"], strings.ToLower(values[1].(common.Address).Hex()), fact["nftId"], height, strings.ToLower(block.Hash().Hex()), strings.ToLower(event.TxHash.Hex()), int64(event.TxIndex), int64(event.Index), int64(block.Time()), payload)
				}
			}
		}
		batch.Queue(`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,$4,true,true)`, c.observation.stream, height, strings.ToLower(block.Hash().Hex()), count)
	}
	batch.Queue(`INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,$1,$2,$2) ON CONFLICT(chain_id,stream_id) DO UPDATE SET raw_complete_height=EXCLUDED.raw_complete_height,typed_complete_height=EXCLUDED.typed_complete_height`, c.observation.stream, receipt.BlockNumber.Int64())
	results := pool.SendBatch(c.ctx, batch)
	if err := results.Close(); err != nil {
		t.Fatal(err)
	}
	c.through = receipt.BlockNumber.Int64()
}
func (c *pauseChain) create(t *testing.T, name string) (common.Address, *bind.BoundContract, *types.Receipt) {
	t.Helper()
	receipt := c.transact(t, c.factory, "createMarketFor", c.auth.From, name, "PAUSE", uint8(0), common.Address{})
	var address common.Address
	for _, event := range receipt.Logs {
		if event.Address == c.factoryAddress && event.Topics[0] == c.factoryABI.Events["MarketCreated"].ID {
			address = common.BytesToAddress(event.Topics[2].Bytes())
		}
	}
	if address == (common.Address{}) {
		t.Fatal("factory pool creation event required")
	}
	return address, bind.NewBoundContract(address, c.poolABI, c.client, c.client, c.client), receipt
}
func (c *pauseChain) refresh(t *testing.T) {
	t.Helper()
	if err := c.observation.refresh(c.ctx); err != nil {
		t.Fatal(err)
	}
}
func (c *pauseChain) coverRaw(pools ...common.Address) {
	c.observation.epochs = []source.AddressEpoch{{Address: c.beaconAddress.Hex(), Role: "PoolBeacon", ActivationHeight: 0}}
	for _, pool := range pools {
		c.observation.epochs = append(c.observation.epochs, source.AddressEpoch{Address: pool.Hex(), Role: "SidioraPool", ActivationHeight: 0})
	}
}

type pauseConsumerPool struct {
	Paused        *bool  `json:"paused"`
	PauseEvidence string `json:"pauseEvidence"`
	SelfVolume    *struct {
		Raw         string `json:"raw"`
		WindowStart int64  `json:"windowStart"`
		WindowEnd   int64  `json:"windowEnd"`
		Evidence    string `json:"evidence"`
	} `json:"selfVolume"`
}
type pauseConsumerEvidence struct {
	Revision     string                       `json:"revision"`
	CutoffHeight uint64                       `json:"cutoffHeight"`
	CutoffTime   int64                        `json:"cutoffTime"`
	BlockHash    string                       `json:"blockHash"`
	Pools        map[string]pauseConsumerPool `json:"pools"`
}

func pauseConsumerDigest(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == 32 && hex.EncodeToString(raw) == value
}

// This decoder applies canonical_evidence.go's transport and observation
// bounds before exposing the pool pause state to the reader.
func decodePauseConsumer(response *http.Response, height uint64, cutoff int64, hash string) (pauseConsumerEvidence, error) {
	var out pauseConsumerEvidence
	if height == 0 || cutoff < 86400 || response.StatusCode != http.StatusOK {
		return out, errPauseUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (64<<20)+1))
	if err != nil || len(raw) > 64<<20 || json.Unmarshal(raw, &out) != nil || !pauseConsumerDigest(out.Revision) || out.CutoffHeight != height || out.CutoffTime != cutoff || out.BlockHash != hash || out.Pools == nil || len(out.Pools) > 1000000 {
		return out, errPauseUnavailable
	}
	for pool, evidence := range out.Pools {
		if len(pool) != 42 || pool[:2] != "0x" {
			return out, errPauseUnavailable
		}
		if _, err = hex.DecodeString(pool[2:]); err != nil {
			return out, err
		}
		if evidence.Paused != nil && !pauseConsumerDigest(evidence.PauseEvidence) {
			return out, errPauseUnavailable
		}
		if evidence.SelfVolume != nil {
			value := evidence.SelfVolume
			if value.WindowStart != cutoff-86400 || value.WindowEnd != cutoff || !pauseConsumerDigest(value.Evidence) {
				return out, errPauseUnavailable
			}
			// Canonical USDL is a nonnegative base-ten integer with at most 156 digits.
			if len(value.Raw) == 0 || len(value.Raw) > 156 || len(value.Raw) > 1 && value.Raw[0] == '0' {
				return out, errPauseUnavailable
			}
			for _, digit := range value.Raw {
				if digit < '0' || digit > '9' {
					return out, errPauseUnavailable
				}
			}
		}
	}
	return out, nil
}
func pauseURL(server string, height uint64, cutoff int64, hash string) string {
	return server + "/internal/ranking-evidence?" + url.Values{"cutoffHeight": {strconv.FormatUint(height, 10)}, "cutoffTime": {strconv.FormatInt(cutoff, 10)}, "blockHash": {hash}}.Encode()
}
func readPauseResponse(t *testing.T, server *httptest.Server, r *types.Receipt) pauseConsumerEvidence {
	t.Helper()
	cutoff := time.Now().Unix() + 1
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pauseURL(server.URL, r.BlockNumber.Uint64(), cutoff, strings.ToLower(r.BlockHash.Hex())), nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	out, err := decodePauseConsumer(response, r.BlockNumber.Uint64(), cutoff, strings.ToLower(r.BlockHash.Hex()))
	if err != nil {
		t.Fatalf("canonical evidence decoder: status=%d error=%v", response.StatusCode, err)
	}
	for _, pool := range out.Pools {
		if pool.SelfVolume != nil {
			t.Fatal("self volume must be absent")
		}
	}
	return out
}
func assertPauseState(t *testing.T, out pauseConsumerEvidence, pool common.Address, want bool) {
	t.Helper()
	key := strings.ToLower(pool.Hex())
	evidence, ok := out.Pools[key]
	if !ok || evidence.Paused == nil || *evidence.Paused != want {
		t.Fatalf("pool %s pause state: %+v", key, evidence)
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("launchpad.pause.v1\n125\n%s\n%s\n%t\n", key, out.BlockHash, want)))
	if evidence.PauseEvidence != hex.EncodeToString(digest[:]) {
		t.Fatal("pool evidence digest differs from its chain, pool, hash and state")
	}
}

func TestPauseObservationCutoffs(t *testing.T) {
	c := newPauseChain(t)
	address, pool, created := c.create(t, "Pause history")
	c.retain(t, created)
	c.refresh(t)
	server := httptest.NewServer(c.observation)
	defer server.Close()
	t.Run("ReaderForm", func(t *testing.T) {
		out := readPauseResponse(t, server, created)
		if len(out.Pools) != 1 {
			t.Fatalf("canonical pool count %d", len(out.Pools))
		}
		assertPauseState(t, out, address, false)
		again := readPauseResponse(t, server, created)
		if out.Revision != again.Revision && out.CutoffTime == again.CutoffTime {
			t.Fatal("revision is not deterministic")
		}
	})
	paused := c.transact(t, pool, "pause")
	c.retain(t, paused)
	c.refresh(t)
	unpaused := c.transact(t, pool, "unpause")
	c.retain(t, unpaused)
	c.refresh(t)
	t.Run("PauseAndEndBetweenCutoffs", func(t *testing.T) {
		assertPauseState(t, readPauseResponse(t, server, paused), address, true)
		assertPauseState(t, readPauseResponse(t, server, unpaused), address, false)
		assertPauseState(t, readPauseResponse(t, server, created), address, false)
	})
	laterAddress, later, laterCreation := c.create(t, "Later pool")
	laterPause := c.transact(t, later, "pause")
	c.retain(t, laterPause)
	c.refresh(t)
	t.Run("CreatedAfterBase", func(t *testing.T) {
		if _, ok := readPauseResponse(t, server, created).Pools[strings.ToLower(laterAddress.Hex())]; ok {
			t.Fatal("pool existed before creation")
		}
		assertPauseState(t, readPauseResponse(t, server, laterCreation), laterAddress, false)
		assertPauseState(t, readPauseResponse(t, server, laterPause), laterAddress, true)
	})
	t.Run("RejectCutoffs", func(t *testing.T) {
		cutoff := time.Now().Unix() + 1
		valid := pauseURL(server.URL, laterPause.BlockNumber.Uint64(), cutoff, strings.ToLower(laterPause.BlockHash.Hex()))
		for _, target := range []string{
			pauseURL(server.URL, laterPause.BlockNumber.Uint64(), cutoff, "0x"+strings.Repeat("b", 64)),
			pauseURL(server.URL, laterPause.BlockNumber.Uint64()+1, cutoff, strings.ToLower(laterPause.BlockHash.Hex())),
			strings.Replace(valid, "cutoffHeight=", "cutoffHeight=-", 1),
			strings.Replace(valid, "cutoffTime=", "cutoffTime=x", 1),
			strings.Replace(valid, "blockHash=0x", "blockHash=0X", 1),
			valid + "&cutoffHeight=1", valid + "&other=1", valid + "&bad=%zz",
			pauseURL(server.URL, laterPause.BlockNumber.Uint64(), 86400, strings.ToLower(laterPause.BlockHash.Hex())),
		} {
			response, err := server.Client().Get(target)
			if err != nil {
				t.Fatal(err)
			}
			wire, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode < 400 || response.StatusCode >= 500 || bytes.Contains(wire, []byte(`"pools"`)) {
				t.Fatalf("invalid cutoff response %d %s", response.StatusCode, wire)
			}
		}
		request, err := http.NewRequest(http.MethodPost, valid, nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 405 || response.Header.Get("Allow") != http.MethodGet {
			t.Fatal("GET admission required")
		}
	})
	t.Run("RawLogCoverage", func(t *testing.T) {
		c.coverRaw(address, laterAddress)
		closed, err := ethclient.DialContext(c.ctx, c.endpoint)
		if err != nil {
			t.Fatal(err)
		}
		closed.Close()
		c.observation.rpc = closed
		c.refresh(t)
		c.observation.rpc = c.client
		assertPauseState(t, readPauseResponse(t, server, paused), address, true)
		assertPauseState(t, readPauseResponse(t, server, unpaused), address, false)
	})
	t.Run("WithoutBaseAndBelowBase", func(t *testing.T) {
		retained := c.observation.snapshot.Swap(nil)
		checkPauseUnavailable(t, server, laterPause)
		c.refresh(t)
		if c.observation.snapshot.Load().base.block.height != laterPause.BlockNumber.Int64() {
			t.Fatal("rebuilt base not pinned to complete head")
		}
		checkPauseUnavailable(t, server, created)
		assertPauseState(t, readPauseResponse(t, server, laterPause), laterAddress, true)
		c.observation.snapshot.Store(retained)
	})
	t.Run("UnreadPool", func(t *testing.T) {
		target, err := url.Parse(c.endpoint)
		if err != nil {
			t.Fatal(err)
		}
		forward := httputil.NewSingleHostReverseProxy(target)
		var reads atomic.Int64
		var failureReads atomic.Int64
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			var call struct {
				Method string
				Params []json.RawMessage
			}
			if err = json.Unmarshal(raw, &call); err != nil {
				t.Error(err)
				return
			}
			if call.Method == "eth_call" {
				reads.Add(1)
				if len(call.Params) != 2 || !bytes.Contains(call.Params[1], []byte(`"blockHash"`)) || !bytes.Contains(call.Params[1], []byte(strings.ToLower(laterPause.BlockHash.Hex()))) {
					t.Error("pause base read lacks the exact complete hash")
				}
				if bytes.Contains(bytes.ToLower(call.Params[0]), []byte(strings.ToLower(address.Hex()))) {
					failureReads.Add(1)
					http.Error(w, "pool read unavailable", http.StatusServiceUnavailable)
					return
				}
			}
			r.Body = io.NopCloser(bytes.NewReader(raw))
			r.ContentLength = int64(len(raw))
			forward.ServeHTTP(w, r)
		}))
		defer proxy.Close()
		client, err := ethclient.DialContext(c.ctx, proxy.URL)
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		c.observation.rpc = client
		c.observation.snapshot.Store(nil)
		c.refresh(t)
		out := readPauseResponse(t, server, laterPause)
		if _, ok := out.Pools[strings.ToLower(address.Hex())]; ok {
			t.Fatal("unread pool received an observation")
		}
		assertPauseState(t, out, laterAddress, true)
		before := reads.Load()
		readPauseResponse(t, server, laterPause)
		if before != reads.Load() || failureReads.Load() != 1 || before != 2 {
			t.Fatal("request performed a chain read or the base did not read each pool once")
		}
		c.observation.rpc = c.client
	})
	t.Run("BeaconUpgradeAndBaseCycle", func(t *testing.T) {
		c.observation.epochs = nil
		implementation, _, _ := c.deploy(t, "SidioraPool")
		upgrade := c.transact(t, c.beacon, "upgradeTo", implementation)
		c.retain(t, upgrade)
		c.refresh(t)
		checkPauseUnavailable(t, server, upgrade)
		old := c.observation.snapshot.Load()
		agedBase := *old.base
		agedBase.built = time.Now().Add(-pauseBaseCycle)
		aged := *old
		aged.base = &agedBase
		c.observation.snapshot.Store(&aged)
		c.refresh(t)
		if c.observation.snapshot.Load().base.block.height != upgrade.BlockNumber.Int64() {
			t.Fatal("base cycle did not advance to the upgraded block")
		}
		assertPauseState(t, readPauseResponse(t, server, upgrade), address, false)
		assertPauseState(t, readPauseResponse(t, server, upgrade), laterAddress, true)
	})
	t.Run("RestartRebuildsFromCompleteBlock", func(t *testing.T) {
		restarted := &pauseObservations{pool: c.observation.pool, rpc: c.client, stream: c.observation.stream}
		restartedServer := httptest.NewServer(restarted)
		defer restartedServer.Close()
		latest, err := c.client.HeaderByNumber(c.ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		cutoff := &types.Receipt{BlockNumber: latest.Number, BlockHash: latest.Hash()}
		checkPauseUnavailable(t, restartedServer, cutoff)
		workers := newRuntimeWorkers(slog.New(slog.NewTextHandler(io.Discard, nil)))
		workers.AddPeriodic("pause_observations", time.Second, false, restarted.refresh)
		if err = workers.Start(c.ctx); err != nil {
			t.Fatal(err)
		}
		defer workers.Close()
		deadline := time.NewTimer(pauseBaseTimeout + pauseRefreshTimeout)
		defer deadline.Stop()
		for restarted.snapshot.Load() == nil {
			select {
			case <-deadline.C:
				t.Fatal("background pause base unavailable")
			case <-time.After(time.Millisecond):
			}
		}
		assertPauseState(t, readPauseResponse(t, restartedServer, cutoff), laterAddress, true)
	})
}

func checkPauseUnavailable(t *testing.T, server *httptest.Server, r *types.Receipt) {
	t.Helper()
	response, err := server.Client().Get(pauseURL(server.URL, r.BlockNumber.Uint64(), time.Now().Unix()+1, strings.ToLower(r.BlockHash.Hex())))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 503 || bytes.Contains(raw, []byte(`"pools"`)) {
		t.Fatalf("unknown observation response %d %s", response.StatusCode, raw)
	}
}

func TestPauseObservationTenThousandPools(t *testing.T) {
	started := time.Now()
	c := newPauseChain(t)
	ctx, cancel := context.WithDeadline(c.ctx, started.Add(5*time.Minute))
	defer cancel()
	c.ctx, c.auth.Context = ctx, ctx
	first, _, created := c.create(t, "Ranking pool")
	c.retain(t, created)
	code, err := c.client.CodeAt(ctx, first, created.BlockNumber)
	if err != nil || len(code) == 0 {
		t.Fatalf("pool runtime required: %v", err)
	}
	beaconSlot := common.BigToHash(new(big.Int).Sub(crypto.Keccak256Hash([]byte("eip1967.proxy.beacon")).Big(), big.NewInt(1)))
	beacon, err := c.client.StorageAt(ctx, first, beaconSlot, created.BlockNumber)
	if err != nil || common.BytesToAddress(beacon) != c.beaconAddress {
		t.Fatalf("pool beacon storage required: %v", err)
	}
	// Each address runs the deployed proxy and pool implementation with the
	// same beacon and initially unpaused storage, without mining creations.
	batch := new(pgx.Batch)
	for i := int64(1); i < 10000; i++ {
		address := common.BigToAddress(big.NewInt(0x100000 + i))
		if err = c.client.Client().CallContext(ctx, nil, "anvil_setCode", address.Hex(), "0x"+hex.EncodeToString(code)); err != nil {
			t.Fatal(err)
		}
		if err = c.client.Client().CallContext(ctx, nil, "anvil_setStorageAt", address.Hex(), beaconSlot.Hex(), "0x"+hex.EncodeToString(beacon)); err != nil {
			t.Fatal(err)
		}
		batch.Queue(`INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) SELECT $1,$1,creator,factory,optical,nft_id+$2,height,block_hash,transaction_hash,transaction_index,log_index+$2,block_timestamp,provenance,payload || jsonb_build_object('token',$1::text,'pool',$1::text,'nftId',(nft_id+$2)::text) FROM indexer.indexer_canonical_creations WHERE pool=$3`, strings.ToLower(address.Hex()), i, strings.ToLower(first.Hex()))
	}
	if err = c.observation.pool.SendBatch(ctx, batch).Close(); err != nil {
		t.Fatal(err)
	}
	// Seal the installed code and storage in the complete block used by the base.
	if err = c.client.Client().CallContext(ctx, nil, "evm_mine"); err != nil {
		t.Fatal(err)
	}
	head, err := c.client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	last := &types.Receipt{BlockNumber: head.Number, BlockHash: head.Hash()}
	c.retain(t, last)
	c.refresh(t)
	if len(c.observation.snapshot.Load().base.pools) != 10000 {
		t.Fatalf("read base pools=%d", len(c.observation.snapshot.Load().base.pools))
	}
	// Closing RPC proves the handler uses retained observations exclusively.
	c.client.Close()
	server := httptest.NewServer(c.observation)
	defer server.Close()
	start := time.Now()
	out := readPauseResponse(t, server, last)
	elapsed := time.Since(start)
	if elapsed >= 2*time.Second {
		t.Fatalf("ten thousand pools response took %s", elapsed)
	}
	t.Logf("ten thousand pools response took %s", elapsed)
	if len(out.Pools) != 10000 {
		t.Fatalf("observations=%d", len(out.Pools))
	}
	assertPauseState(t, out, first, false)
	keys := make([]string, 0, len(out.Pools))
	for key := range out.Pools {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	revision := sha256.New()
	fmt.Fprintf(revision, "launchpad.pause.revision.v1\n125\n%d\n%d\n%s\n", out.CutoffHeight, out.CutoffTime, out.BlockHash)
	for _, key := range keys {
		evidence := out.Pools[key]
		if evidence.Paused == nil || *evidence.Paused {
			t.Fatalf("initial pool pause state %s", key)
		}
		fmt.Fprintf(revision, "%s\n%t\n", key, *evidence.Paused)
	}
	if out.Revision != hex.EncodeToString(revision.Sum(nil)) {
		t.Fatal("revision does not bind the cutoff and sorted pool states")
	}
	if elapsed := time.Since(started); elapsed >= 5*time.Minute {
		t.Fatalf("ten thousand pools case took %s", elapsed)
	}
}

func TestPauseObservationInternalListener(t *testing.T) {
	ctx := context.Background()
	cfg := legacyLifecycleConfig(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := New(ctx, cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	server := httptest.NewServer(app.Router)
	defer server.Close()
	response, err := server.Client().Get(pauseURL(server.URL, 1, time.Now().Unix(), "0x"+strings.Repeat("1", 64)))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 503 && response.StatusCode != 409 {
		t.Fatalf("internal evidence route status=%d", response.StatusCode)
	}

}
