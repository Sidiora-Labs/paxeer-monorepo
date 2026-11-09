package bindings_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol"
	b "github.com/Sidiora-Technologies/KindleLaunch/protocol/bindings"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gorilla/websocket"
)

type repairM948Chain struct {
	backend  *repairM948Replay
	client   *ethclient.Client
	endpoint string
	auth     []*bind.TransactOpts
}

func newRepairM948Chain(t *testing.T) *repairM948Chain {
	t.Helper()
	s := &repairM948Chain{}
	for i := 0; i < 3; i++ {
		key, err := crypto.HexToECDSA(fmt.Sprintf("%064x", i+1))
		if err != nil {
			t.Fatal(err)
		}
		a, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(125))
		if err != nil {
			t.Fatal(err)
		}
		s.auth = append(s.auth, a)
	}
	s.backend = newRepairM948Replay(t)
	s.endpoint = s.backend.endpoint
	s.reopen(t)
	// The node reserves pending-state RPC for its sequencer. These serial
	// scenarios sign with the committed nonce and advance it only on mining.
	for _, auth := range s.auth {
		nonce, err := s.client.NonceAt(context.Background(), auth.From, nil)
		if err != nil {
			t.Fatal(err)
		}
		auth.Nonce = new(big.Int).SetUint64(nonce)
	}
	t.Cleanup(func() {
		s.client.Close()
		if err := s.backend.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}
func (s *repairM948Chain) reopen(t *testing.T) {
	t.Helper()
	if s.client != nil {
		s.client.Close()
	}
	c, err := ethclient.Dial(s.endpoint)
	if err != nil {
		t.Fatal(err)
	}
	s.client = c
}
func (s *repairM948Chain) miner(t *testing.T) func(*types.Transaction, error) *types.Receipt {
	return func(tx *types.Transaction, err error) *types.Receipt {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		r, err := bind.WaitMined(context.Background(), s.client, tx)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != types.ReceiptStatusSuccessful {
			t.Fatalf("transaction %s reverted", tx.Hash())
		}
		sender, err := types.Sender(types.LatestSignerForChainID(big.NewInt(125)), tx)
		if err != nil {
			t.Fatal(err)
		}
		for _, auth := range s.auth {
			if auth.From == sender {
				auth.Nonce = new(big.Int).SetUint64(tx.Nonce() + 1)
			}
		}
		return r
	}
}
func repairM948Artifact(t *testing.T, name string) (abi.ABI, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "repair37", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var a struct {
		ABI      json.RawMessage
		Bytecode string
	}
	if err = json.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	parsed, err := abi.JSON(bytes.NewReader(a.ABI))
	if err != nil {
		t.Fatal(err)
	}
	return parsed, common.FromHex(a.Bytecode)
}
func (s *repairM948Chain) deploy(t *testing.T, name string, args ...any) common.Address {
	t.Helper()
	a, code := repairM948Artifact(t, name)
	addr, tx, _, err := bind.DeployContract(s.auth[0], a, code, s.client, args...)
	s.miner(t)(tx, err)
	return addr
}
func repairM948Unauthorized(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("repairM948Unauthorized operation accepted")
	}
	data, ok := ethclient.RevertErrorData(err)
	if !ok || !bytes.Equal(data, crypto.Keccak256([]byte("Unauthorized()"))[:4]) {
		t.Fatalf("want Unauthorized selector, got %x (%v)", data, err)
	}
}
func TestRepairM_948_BeaconOwnershipHandoff(t *testing.T) {
	repair37Artifact(t, "PoolBeacon", b.PoolBeaconMetaData)
	s := newRepairM948Chain(t)
	mine := s.miner(t)
	implementation := s.deploy(t, "EventEmitter")
	address := s.deploy(t, "PoolBeacon", implementation, s.auth[0].From)
	beacon, err := b.NewPoolBeacon(address, s.client)
	if err != nil {
		t.Fatal(err)
	}
	assertOwners := func(binding *b.PoolBeacon, owner, pending common.Address) {
		t.Helper()
		o, err := binding.Owner(nil)
		if err != nil || o != owner {
			t.Fatalf("owner = %s, %v; want %s", o, err, owner)
		}
		p, err := binding.PendingOwner(nil)
		if err != nil || p != pending {
			t.Fatalf("pending = %s, %v; want %s", p, err, pending)
		}
		impl, err := binding.Implementation(nil)
		if err != nil || impl != implementation {
			t.Fatalf("implementation = %s, %v", impl, err)
		}
	}
	assertOwners(beacon, s.auth[0].From, common.Address{})
	receipt := mine(beacon.TransferOwnership(s.auth[0], s.auth[1].From))
	if len(receipt.Logs) != 1 {
		t.Fatalf("handoff start logs = %d", len(receipt.Logs))
	}
	started, err := beacon.ParseOwnershipTransferStarted(*receipt.Logs[0])
	if err != nil || started.PreviousOwner != s.auth[0].From || started.PendingOwner != s.auth[1].From {
		t.Fatalf("handoff start = %+v, %v", started, err)
	}
	assertOwners(beacon, s.auth[0].From, s.auth[1].From)
	// Reconnect over JSON-RPC, discarding the previous client and binding.
	s.reopen(t)
	beacon, err = b.NewPoolBeacon(address, s.client)
	if err != nil {
		t.Fatal(err)
	}
	assertOwners(beacon, s.auth[0].From, s.auth[1].From)
	_, err = beacon.AcceptOwnership(s.auth[2])
	repairM948Unauthorized(t, err)
	_, err = beacon.AcceptOwnership(s.auth[0])
	repairM948Unauthorized(t, err)
	assertOwners(beacon, s.auth[0].From, s.auth[1].From)
	receipt = mine(beacon.AcceptOwnership(s.auth[1]))
	transferred, err := beacon.ParseOwnershipTransferred(*receipt.Logs[0])
	if err != nil || transferred.PreviousOwner != s.auth[0].From || transferred.NewOwner != s.auth[1].From {
		t.Fatalf("ownership accepted = %+v, %v", transferred, err)
	}
	assertOwners(beacon, s.auth[1].From, common.Address{})
	_, err = beacon.TransferOwnership(s.auth[0], s.auth[2].From)
	repairM948Unauthorized(t, err)
	_, err = beacon.AcceptOwnership(s.auth[1])
	repairM948Unauthorized(t, err)
	s.reopen(t)
	beacon, err = b.NewPoolBeacon(address, s.client)
	if err != nil {
		t.Fatal(err)
	}
	assertOwners(beacon, s.auth[1].From, common.Address{})
	// The second owner can initiate the next handoff, retaining ownership until acceptance.
	mine(beacon.TransferOwnership(s.auth[1], s.auth[0].From))
	assertOwners(beacon, s.auth[1].From, s.auth[0].From)
	mine(beacon.AcceptOwnership(s.auth[0]))
	assertOwners(beacon, s.auth[0].From, common.Address{})
}

func TestRepairM_948_RetainedEmitterV2(t *testing.T) {
	repair37Artifact(t, "EventEmitter", b.EventEmitterMetaData)
	repair37Artifact(t, "ERC1967Proxy", nil)
	s := newRepairM948Chain(t)
	mine := s.miner(t)
	implementation := s.deploy(t, "EventEmitter")
	a, _ := b.EventEmitterMetaData.GetAbi()
	init, err := a.Pack("initialize", s.auth[0].From)
	if err != nil {
		t.Fatal(err)
	}
	address := s.deploy(t, "ERC1967Proxy", implementation, init)
	emitter, err := b.NewEventEmitter(address, s.client)
	if err != nil {
		t.Fatal(err)
	}
	version, err := emitter.VERSION(nil)
	if err != nil || version != "2.0.0" {
		t.Fatalf("VERSION = %s, %v", version, err)
	}
	role, err := emitter.EVENTEMITTERROLE(nil)
	if err != nil || role != crypto.Keccak256Hash([]byte("EVENT_EMITTER_ROLE")) {
		t.Fatalf("EVENT_EMITTER_ROLE = %x, %v", role, err)
	}
	if _, err = emitter.ReinitializeV2(s.auth[2], s.auth[2].From); err == nil {
		t.Fatal("nonadmin migrated v2")
	}
	mine(emitter.ReinitializeV2(s.auth[0], s.auth[1].From))
	granted, err := emitter.HasRole(nil, role, s.auth[1].From)
	if err != nil || !granted {
		t.Fatalf("v2 role grant = %v, %v", granted, err)
	}
	_, err = emitter.ReinitializeV2(s.auth[0], s.auth[1].From)
	repairM948Unauthorized(t, err)
	mine(emitter.SetAuthorizedEmitter(s.auth[1], s.auth[1].From, true))
	selector := [4]byte(a.Methods["emitFeeFlow"].ID)
	allowed, err := emitter.IsTypedEmitter(nil, s.auth[1].From, selector)
	if err != nil || allowed {
		t.Fatalf("ungranted typed permission = %v, %v", allowed, err)
	}
	_, err = emitter.EmitFeeFlow(s.auth[1], 1, s.auth[1].From, s.auth[2].From, big.NewInt(90), big.NewInt(10), big.NewInt(80), big.NewInt(7))
	repairM948Unauthorized(t, err)
	if _, err = emitter.SetTypedEmitter(s.auth[1], s.auth[1].From, selector, true); err == nil {
		t.Fatal("emitter role granted typed authority")
	}
	permission := mine(emitter.SetTypedEmitter(s.auth[0], s.auth[1].From, selector, true))
	grant, err := emitter.ParseTypedEmitterPermission(*permission.Logs[0])
	if err != nil || grant.Emitter != s.auth[1].From || grant.Selector != selector || !grant.Allowed {
		t.Fatalf("typed grant = %+v, %v", grant, err)
	}
	allowed, err = emitter.IsTypedEmitter(nil, s.auth[1].From, selector)
	if err != nil || !allowed {
		t.Fatalf("typed permission = %v, %v", allowed, err)
	}
	feeReceipt := mine(emitter.EmitFeeFlow(s.auth[1], 1, s.auth[1].From, s.auth[2].From, big.NewInt(90), big.NewInt(10), big.NewInt(80), big.NewInt(7)))
	fee, err := emitter.ParseFeeFlow(*feeReceipt.Logs[0])
	if err != nil || fee.Kind != 1 || fee.Pool != s.auth[1].From || fee.Party != s.auth[2].From || fee.Amount.Int64() != 90 || fee.ProtocolCut.Int64() != 10 || fee.PoolCut.Int64() != 80 || fee.Epoch.Int64() != 7 {
		t.Fatalf("typed fee = %+v, %v", fee, err)
	}
	mine(emitter.SetTypedEmitter(s.auth[0], s.auth[1].From, selector, false))
	_, err = emitter.EmitFeeFlow(s.auth[1], 1, s.auth[1].From, s.auth[2].From, big.NewInt(90), big.NewInt(10), big.NewInt(80), big.NewInt(7))
	repairM948Unauthorized(t, err)
	data := repairM948TupleData(s.auth[1].From)
	name := "RepairM948 λ generic"
	nameHash := crypto.Keccak256Hash([]byte(name))
	topic1 := crypto.Keccak256Hash([]byte("pool"))
	topic2 := crypto.Keccak256Hash([]byte("user"))
	_, err = emitter.EmitEventLog(s.auth[2], name, data)
	repairM948Unauthorized(t, err)
	sink := make(chan *b.EventEmitterEventLog, 1)
	sub, err := emitter.WatchEventLog(&bind.WatchOpts{Context: context.Background()}, sink, []common.Address{s.auth[1].From}, [][32]byte{nameHash})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()
	receipt := mine(emitter.EmitEventLog(s.auth[1], name, data))
	log, err := emitter.ParseEventLog(*receipt.Logs[0])
	if err != nil {
		t.Fatal(err)
	}
	assertRepairM948Generic(t, s, receipt, log.MsgSender, log.EventNameHash, log.EventName, log.EventData, log.Timestamp, log.BlockNumber, data, name)
	select {
	case observed := <-sink:
		if !reflect.DeepEqual(observed.EventData, data) || observed.Raw.TxHash != receipt.TxHash {
			t.Fatalf("watch = %+v", observed)
		}
	case err := <-sub.Err():
		t.Fatalf("watch failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not observe real log")
	}
	iterator, err := emitter.FilterEventLog(&bind.FilterOpts{Start: receipt.BlockNumber.Uint64()}, []common.Address{s.auth[1].From}, [][32]byte{nameHash})
	if err != nil {
		t.Fatal(err)
	}
	defer iterator.Close()
	if !iterator.Next() || !reflect.DeepEqual(iterator.Event.EventData, data) || iterator.Event.Raw.TxHash != receipt.TxHash {
		t.Fatal("filter did not round trip tuple")
	}
	if iterator.Next() || iterator.Error() != nil {
		t.Fatalf("filter completion: %v", iterator.Error())
	}
	receipt = mine(emitter.EmitEventLog1(s.auth[1], name, topic1, data))
	log1, err := emitter.ParseEventLog1(*receipt.Logs[0])
	if err != nil {
		t.Fatal(err)
	}
	assertRepairM948Generic(t, s, receipt, log1.MsgSender, log1.EventNameHash, log1.EventName, log1.EventData, log1.Timestamp, log1.BlockNumber, data, name)
	if len(receipt.Logs[0].Topics) != 4 || log1.Topic1 != topic1 || receipt.Logs[0].Topics[3] != topic1 {
		t.Fatalf("EventLog1 indexed topic = %+v", log1)
	}
	receipt = mine(emitter.EmitEventLog2(s.auth[1], name, topic1, topic2, data))
	log2, err := emitter.ParseEventLog2(*receipt.Logs[0])
	if err != nil {
		t.Fatal(err)
	}
	assertRepairM948Generic(t, s, receipt, log2.MsgSender, log2.EventNameHash, log2.EventName, log2.EventData, log2.Timestamp, log2.BlockNumber, data, name)
	if len(receipt.Logs[0].Topics) != 4 || log2.Topic1 != topic1 || log2.Topic2 != topic2 || receipt.Logs[0].Topics[3] != topic1 {
		t.Fatalf("EventLog2 topic encoding = %+v", log2)
	}
	malformed := *receipt.Logs[0]
	malformed.Data = malformed.Data[:31]
	if _, err = emitter.ParseEventLog2(malformed); err == nil {
		t.Fatal("truncated tuple accepted")
	}
}
func assertRepairM948Generic(t *testing.T, s *repairM948Chain, r *types.Receipt, sender common.Address, hash [32]byte, name string, data b.IEventEmitterEventData, timestamp, number *big.Int, want b.IEventEmitterEventData, wantName string) {
	t.Helper()
	h, err := s.client.HeaderByNumber(context.Background(), r.BlockNumber)
	if err != nil {
		t.Fatal(err)
	}
	if sender != s.auth[1].From || hash != crypto.Keccak256Hash([]byte(wantName)) || name != wantName || !reflect.DeepEqual(data, want) || timestamp.Uint64() != h.Time || number.Cmp(r.BlockNumber) != 0 {
		t.Fatalf("generic log did not preserve producer fields: %s %+v", name, data)
	}
	if r.Logs[0].Topics[1] != common.BytesToHash(sender.Bytes()) || r.Logs[0].Topics[2] != hash {
		t.Fatal("indexed provenance topics differ")
	}
}
func repairM948TupleData(address common.Address) b.IEventEmitterEventData {
	word := crypto.Keccak256Hash([]byte("bytes32"))
	huge := new(big.Int).Lsh(big.NewInt(1), 200)
	return b.IEventEmitterEventData{
		AddressItems: b.IEventEmitterAddressItems{Items: []b.IEventEmitterAddressKeyValue{{Key: "owner", Value: address}}, ArrayItems: []b.IEventEmitterAddressArrayKeyValue{{Key: "owners", Value: []common.Address{address, {}}}}},
		UintItems:    b.IEventEmitterUintItems{Items: []b.IEventEmitterUintKeyValue{{Key: "amount", Value: huge}}, ArrayItems: []b.IEventEmitterUintArrayKeyValue{{Key: "amounts", Value: []*big.Int{new(big.Int).SetBytes(make([]byte, 32)), huge}}}},
		IntItems:     b.IEventEmitterIntItems{Items: []b.IEventEmitterIntKeyValue{{Key: "delta", Value: big.NewInt(-8)}}, ArrayItems: []b.IEventEmitterIntArrayKeyValue{{Key: "deltas", Value: []*big.Int{big.NewInt(-9), big.NewInt(10)}}}},
		BoolItems:    b.IEventEmitterBoolItems{Items: []b.IEventEmitterBoolKeyValue{{Key: "enabled", Value: true}}, ArrayItems: []b.IEventEmitterBoolArrayKeyValue{{Key: "flags", Value: []bool{false, true}}}},
		Bytes32Items: b.IEventEmitterBytes32Items{Items: []b.IEventEmitterBytes32KeyValue{{Key: "id", Value: word}}, ArrayItems: []b.IEventEmitterBytes32ArrayKeyValue{{Key: "ids", Value: [][32]byte{word, {}}}}},
		BytesItems:   b.IEventEmitterBytesItems{Items: []b.IEventEmitterBytesKeyValue{{Key: "payload", Value: []byte{0, 1, 255}}}, ArrayItems: []b.IEventEmitterBytesArrayKeyValue{{Key: "payloads", Value: [][]byte{{0, 255}, {1, 2}}}}},
		StringItems:  b.IEventEmitterStringItems{Items: []b.IEventEmitterStringKeyValue{{Key: "label", Value: "λ launchpad"}}, ArrayItems: []b.IEventEmitterStringArrayKeyValue{{Key: "labels", Value: []string{"one", "二"}}}},
	}
}

// This scenario also captures real public-method execution on the local node;
// the gate consumes only its committed RPC transcript.
func TestRepairM_948_NativeExecutorNode(t *testing.T) {
	s := newRepairM948Chain(t)
	// Native precompiles have no bytecode for bind's estimator to inspect;
	// submit an explicit gas limit and require a successful mined receipt.
	s.auth[0].GasLimit = 7_000_000
	mine := s.miner(t)
	assertRepairM948ProductionRefusals(t, s)
	at := protocol.ActiveAddresses.Precompile
	router, err := b.NewRouter(at, s.client)
	if err != nil {
		t.Fatal(err)
	}
	factory, err := b.NewSidioraFactory(protocol.ActiveAddresses.Roles["SidioraFactory"], s.client)
	if err != nil {
		t.Fatal(err)
	}
	quote, err := router.UsdlAddress(nil)
	if err != nil {
		t.Fatal(err)
	}
	nonce, err := factory.GetNonce(nil, s.auth[0].From)
	if err != nil {
		t.Fatal(err)
	}
	erc20, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"approve","stateMutability":"nonpayable","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"type":"bool"}]},{"type":"function","name":"totalSupply","stateMutability":"view","inputs":[],"outputs":[{"type":"uint256"}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	quoteContract := bind.NewBoundContract(quote, erc20, s.client, s.client, s.client)
	mine(quoteContract.Transact(s.auth[0], "approve", at, big.NewInt(1_000_000_000_000)))
	name, symbol := "RepairM948 Native", "M948"
	receipt := mine(router.CreateMarket(s.auth[0], name, symbol, 0, common.Address{}))
	var market *b.RouterMarketCreated
	for _, log := range receipt.Logs {
		if log.Address == at {
			if parsed, err := router.ParseMarketCreated(*log); err == nil {
				if market != nil {
					t.Fatal("duplicate Router.MarketCreated")
				}
				market = parsed
			}
		}
	}
	if market == nil || market.Creator != s.auth[0].From || market.Token == (common.Address{}) || market.Pool == (common.Address{}) || market.NftId.Sign() <= 0 {
		t.Fatalf("native market result = %+v", market)
	}
	poolParser, err := b.NewRouter(market.Pool, s.client)
	if err != nil {
		t.Fatal(err)
	}
	initialized := 0
	initializedTopic := crypto.Keccak256Hash([]byte("Initialized(uint64)"))
	for _, log := range receipt.Logs {
		if len(log.Topics) > 0 && log.Topics[0] == initializedTopic {
			initialized++
			parsed, err := poolParser.ParseInitialized(*log)
			if err != nil || log.Address != market.Pool || parsed.Version != 1 {
				t.Fatalf("market Initialized origin/version = %+v, %v", log, err)
			}
		}
	}
	if initialized != 1 {
		t.Fatalf("market Initialized logs = %d, want one at the new pool", initialized)
	}
	nft, err := b.NewSidioraNFT(protocol.ActiveAddresses.Roles["SidioraNFT"], s.client)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := b.NewPoolRegistry(protocol.ActiveAddresses.Roles["PoolRegistry"], s.client)
	if err != nil {
		t.Fatal(err)
	}
	assertRepairM948RoleLog(t, receipt, protocol.ActiveAddresses.Roles["SidioraFactory"], factory, b.SidioraFactoryMetaData, "MarketCreated", map[string]any{"token": market.Token, "pool": market.Pool, "creator": s.auth[0].From, "nftId": market.NftId, "optical": common.Address{}})
	assertRepairM948RoleLog(t, receipt, protocol.ActiveAddresses.Roles["SidioraNFT"], nft, b.SidioraNFTMetaData, "Transfer", map[string]any{"from": common.Address{}, "to": s.auth[0].From, "tokenId": market.NftId})
	assertRepairM948RoleLog(t, receipt, protocol.ActiveAddresses.Roles["SidioraNFT"], nft, b.SidioraNFTMetaData, "PoolNFTMinted", map[string]any{"tokenId": market.NftId, "creator": s.auth[0].From, "pool": market.Pool})
	header, err := s.client.HeaderByNumber(context.Background(), receipt.BlockNumber)
	if err != nil {
		t.Fatal(err)
	}
	assertRepairM948RoleLog(t, receipt, protocol.ActiveAddresses.Roles["PoolRegistry"], registry, b.PoolRegistryMetaData, "PoolRegistered", map[string]any{"pool": market.Pool, "token": market.Token, "creator": s.auth[0].From, "optical": common.Address{}, "nftId": market.NftId, "timestamp": new(big.Int).SetUint64(header.Time)})
	tokenContract := bind.NewBoundContract(market.Token, erc20, s.client, s.client, s.client)
	var supply []any
	if err := tokenContract.Call(nil, &supply, "totalSupply"); err != nil {
		t.Fatal(err)
	}
	if len(supply) != 1 || supply[0].(*big.Int).Sign() <= 0 {
		t.Fatalf("native token supply = %v", supply)
	}
	salt := crypto.Keccak256Hash(s.auth[0].From.Bytes(), []byte(name), []byte(symbol), common.LeftPadBytes(nonce.Bytes(), 32))
	poolID := [32]byte(common.BytesToHash(market.Pool.Bytes()))
	assertRepairM948NodeEvent(t, s, receipt, "TokenDeployed", map[string]any{"token": market.Token, "pool": market.Pool, "creator": s.auth[0].From, "salt": [32]byte(salt), "name": name, "symbol": symbol, "decimals": uint8(6), "totalSupply": supply[0]})
	assertRepairM948NodeEvent(t, s, receipt, "MarketCreated", map[string]any{"poolId": poolID, "token": market.Token, "creator": s.auth[0].From, "pool": market.Pool, "optical": common.Address{}})
	assertRepairM948NodeEvent(t, s, receipt, "NftMint", map[string]any{"tokenId": market.NftId, "creator": s.auth[0].From, "pool": market.Pool, "strategy": uint8(0)})
	assertRepairM948NodeEvent(t, s, receipt, "PoolRegistered", map[string]any{"pool": market.Pool, "token": market.Token, "creator": s.auth[0].From, "optical": common.Address{}, "nftId": market.NftId})

	// Both Router surfaces must preserve the actual origin of their role log,
	// while native-family logs remain at the precompile.
	for _, origin := range []common.Address{at, protocol.ActiveAddresses.Roles["Router"]} {
		route, err := b.NewRouter(origin, s.client)
		if err != nil {
			t.Fatal(err)
		}
		mine(quoteContract.Transact(s.auth[0], "approve", origin, big.NewInt(1_000_000_000_000)))
		header, err := s.client.HeaderByNumber(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		amount := big.NewInt(100_000_000)
		receipt = mine(route.Buy(s.auth[0], market.Pool, amount, big.NewInt(0), new(big.Int).SetUint64(header.Time+3600)))
		var buy *b.RouterBuy
		for _, log := range receipt.Logs {
			if log.Address == origin {
				if parsed, err := route.ParseBuy(*log); err == nil {
					buy = parsed
				}
			}
		}
		if buy == nil || buy.Pool != market.Pool || buy.Buyer != s.auth[0].From || buy.UsdlIn.Cmp(amount) != 0 || buy.TokensOut.Sign() <= 0 {
			t.Fatalf("native Router.Buy = %+v", buy)
		}
		assertRepairM948RoleLog(t, receipt, origin, route, b.RouterMetaData, "Buy", map[string]any{"pool": market.Pool, "buyer": s.auth[0].From, "usdlIn": amount, "tokensOut": buy.TokensOut})
		assertRepairM948NodeEvent(t, s, receipt, "Swap", map[string]any{"poolId": poolID, "sender": s.auth[0].From, "isBuy": true, "amountIn": amount, "amountOut": buy.TokensOut})
		assertRepairM948NodeEvent(t, s, receipt, "RouterTrade", map[string]any{"kind": uint8(0), "pool": market.Pool, "sender": s.auth[0].From, "tokenIn": quote, "tokenOut": market.Token, "amountIn": amount, "amountOut": buy.TokensOut, "intermediateUsdl": big.NewInt(0)})
	}
	for i, origin := range []common.Address{at, protocol.ActiveAddresses.Roles["FeesRouter"]} {
		fees, err := b.NewFeesRouter(origin, s.client)
		if err != nil {
			t.Fatal(err)
		}
		old, next := uint8(i), uint8(1-i)
		receipt = mine(fees.SetFeeStrategy(s.auth[0], market.NftId, next))
		assertRepairM948NodeEvent(t, s, receipt, "FeeStrategyChanged", map[string]any{"poolId": poolID, "nftId": market.NftId, "oldStrategy": old, "newStrategy": next})
		// feeLog uses generation 2 identities even for a direct call.
		assertRepairM948RoleLog(t, receipt, protocol.ActiveAddresses.Roles["FeesRouter"], fees, b.FeesRouterMetaData, "FeeStrategyChanged", map[string]any{"nftId": market.NftId, "oldStrategy": old, "newStrategy": next})
		assertRepairM948RoleLog(t, receipt, protocol.ActiveAddresses.Roles["SidioraNFT"], nft, b.SidioraNFTMetaData, "FeeStrategyChanged", map[string]any{"tokenId": market.NftId, "oldStrategy": old, "newStrategy": next})
		var changed *b.FeesRouterFeeStrategyChanged
		for _, log := range receipt.Logs {
			if log.Address == protocol.ActiveAddresses.Roles["FeesRouter"] {
				if parsed, err := fees.ParseFeeStrategyChanged(*log); err == nil {
					changed = parsed
				}
			}
		}
		if changed == nil || changed.NftId.Cmp(market.NftId) != 0 || changed.OldStrategy != old || changed.NewStrategy != next {
			t.Fatalf("native FeesRouter origin %s: %+v", origin, changed)
		}
	}
	accumulator, err := b.NewFeeAccumulator(protocol.ActiveAddresses.Roles["FeeAccumulator"], s.client)
	if err != nil {
		t.Fatal(err)
	}
	fees, err := b.NewFeesRouter(protocol.ActiveAddresses.Roles["FeesRouter"], s.client)
	if err != nil {
		t.Fatal(err)
	}
	amount, err := accumulator.GetAccumulatedFees(nil, market.Pool)
	if err != nil || amount.Sign() <= 0 {
		t.Fatalf("accumulated fees = %v, %v", amount, err)
	}
	receipt = mine(fees.ClaimFees(s.auth[0], market.NftId))
	assertRepairM948RoleLog(t, receipt, protocol.ActiveAddresses.Roles["FeeAccumulator"], accumulator, b.FeeAccumulatorMetaData, "FeesClaimed", map[string]any{"pool": market.Pool, "recipient": s.auth[0].From, "amount": amount})
	assertRepairM948RoleLog(t, receipt, protocol.ActiveAddresses.Roles["FeesRouter"], fees, b.FeesRouterMetaData, "FeesClaimed", map[string]any{"nftId": market.NftId, "owner": s.auth[0].From, "amount": amount})
	remaining, err := accumulator.GetAccumulatedFees(nil, market.Pool)
	if err != nil || remaining.Sign() != 0 {
		t.Fatalf("fees after claim = %v, %v", remaining, err)
	}
	receipt = mine(nft.Approve(s.auth[0], s.auth[1].From, market.NftId))
	assertRepairM948RoleLog(t, receipt, protocol.ActiveAddresses.Roles["SidioraNFT"], nft, b.SidioraNFTMetaData, "Approval", map[string]any{"owner": s.auth[0].From, "approved": s.auth[1].From, "tokenId": market.NftId})
	receipt = mine(nft.SetApprovalForAll(s.auth[0], s.auth[2].From, true))
	assertRepairM948RoleLog(t, receipt, protocol.ActiveAddresses.Roles["SidioraNFT"], nft, b.SidioraNFTMetaData, "ApprovalForAll", map[string]any{"owner": s.auth[0].From, "operator": s.auth[2].From, "approved": true})
	receipt = mine(nft.TransferFrom(s.auth[0], s.auth[0].From, s.auth[1].From, market.NftId))
	assertRepairM948RoleLog(t, receipt, protocol.ActiveAddresses.Roles["SidioraNFT"], nft, b.SidioraNFTMetaData, "Transfer", map[string]any{"from": s.auth[0].From, "to": s.auth[1].From, "tokenId": market.NftId})
	assertRepairM948NodeEvent(t, s, receipt, "NftTransfer", map[string]any{"nft": protocol.ActiveAddresses.Roles["SidioraNFT"], "from": s.auth[0].From, "to": s.auth[1].From, "tokenId": market.NftId})
	owner, err := nft.OwnerOf(nil, market.NftId)
	if err != nil || owner != s.auth[1].From {
		t.Fatalf("native recovered NFT owner = %s, %v", owner, err)
	}
}

// Production calls and failed mined transactions are separate from emitAt schema checks.
func assertRepairM948ProductionRefusals(t *testing.T, s *repairM948Chain) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repair37Root(t), "precompiles/launchpad/abi.json"))
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := abi.JSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	adminError, exists := canonical.Errors["AccessControlUnauthorizedAccount"]
	if !exists || adminError.Sig != "AccessControlUnauthorizedAccount(address,bytes32)" {
		t.Fatal("missing canonical administrative error")
	}
	roles := []struct {
		name     string
		metadata *bind.MetaData
		views    []string
	}{
		{"FeeAccumulator", b.FeeAccumulatorMetaData, []string{"protocolConfig", "treasury", "poolRegistry", "eventEmitter", "usdlAddress", "accountingInitialized", "totalLiabilities"}},
		{"PoolRegistry", b.PoolRegistryMetaData, []string{"eventEmitter", "getPoolCount"}},
		{"FeesRouter", b.FeesRouterMetaData, []string{"nftContract", "feeAccumulator", "poolRegistry"}},
		{"ProtocolConfig", b.ProtocolConfigMetaData, []string{"usdlAddress", "eventEmitter", "creationFee", "virtualUsdlDefault", "virtualTokenDefault", "baseFeeBps"}},
		{"Quoter", b.QuoterMetaData, []string{"poolRegistry", "protocolConfig"}},
		{"Router", b.RouterMetaData, []string{"factory", "poolRegistry", "protocolConfig", "usdlAddress"}},
		{"SidioraFactory", b.SidioraFactoryMetaData, []string{"poolBeacon", "nftContract", "poolRegistry", "eventEmitter", "protocolConfig", "treasury", "feeAccumulator", "usdlAddress"}},
		{"SidioraNFT", b.SidioraNFTMetaData, []string{"name", "symbol", "totalSupply", "eventEmitter"}},
	}
	ctx := context.Background()
	for _, role := range roles {
		address := protocol.ActiveAddresses.Roles[role.name]
		metadata, err := role.metadata.GetAbi()
		if err != nil {
			t.Fatal(err)
		}
		contract := bind.NewBoundContract(address, *metadata, s.client, s.client, s.client)
		call := func(method string, args ...any) (string, error) {
			data, err := metadata.Pack(method, args...)
			if err != nil {
				t.Fatal(err)
			}
			var result string
			err = s.client.Client().CallContext(ctx, &result, "eth_call", map[string]any{"from": s.auth[0].From, "to": address, "data": "0x" + common.Bytes2Hex(data)}, "latest")
			return result, err
		}
		snapshot := func() map[string]string {
			state := map[string]string{}
			for _, method := range role.views {
				result, err := call(method)
				if err != nil {
					t.Fatalf("%s.%s: %v", role.name, method, err)
				}
				state[method] = result
			}
			for i, account := range []common.Address{s.auth[0].From, s.auth[2].From} {
				result, err := call("hasRole", [32]byte{}, account)
				if err != nil {
					t.Fatal(err)
				}
				state[fmt.Sprintf("admin%d", i)] = result
			}
			return state
		}
		before := snapshot()
		block, err := s.client.BlockNumber(ctx)
		if err != nil {
			t.Fatal(err)
		}
		logs := func() []types.Log {
			var result []types.Log
			if err := s.client.Client().CallContext(ctx, &result, "eth_getLogs", map[string]any{"address": address, "fromBlock": fmt.Sprintf("0x%x", block), "toBlock": "latest"}); err != nil {
				t.Fatal(err)
			}
			return result
		}
		logsBefore := logs()
		args := make([]any, len(metadata.Methods["initialize"].Inputs))
		for i, input := range metadata.Methods["initialize"].Inputs {
			if input.Type.T == abi.StringTy {
				args[i] = "Refused M948"
			} else {
				args[i] = s.auth[2].From
			}
		}
		_, err = call("initialize", args...)
		data, ok := ethclient.RevertErrorData(err)
		declared, exists := metadata.Errors["AlreadyInitialized"]
		if !exists || declared.Sig != "AlreadyInitialized()" || !ok || !bytes.Equal(data, declared.ID[:4]) || common.Bytes2Hex(data) != "0dc149f0" {
			t.Fatalf("%s.initialize exact production refusal = %x (%v)", role.name, data, err)
		}
		tx, err := contract.Transact(s.auth[0], "initialize", args...)
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := bind.WaitMined(ctx, s.client, tx)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.Status != types.ReceiptStatusFailed || len(receipt.Logs) != 0 || tx.To() == nil || *tx.To() != address {
			t.Fatalf("%s.initialize mined refusal = %+v", role.name, receipt)
		}
		s.auth[0].Nonce = new(big.Int).SetUint64(tx.Nonce() + 1)
		if after := snapshot(); !reflect.DeepEqual(before, after) {
			t.Fatalf("%s.initialize changed state: %v -> %v", role.name, before, after)
		}
		if after := logs(); !reflect.DeepEqual(logsBefore, after) {
			t.Fatalf("%s.initialize changed logs", role.name)
		}
		// Public administrative grants are refused; a non-holder's renounce is
		// a successful no-op, and cannot be mistaken for a RoleRevoked log.
		_, err = call("grantRole", [32]byte{}, s.auth[2].From)
		data, ok = ethclient.RevertErrorData(err)
		packed, packErr := adminError.Inputs.Pack(s.auth[0].From, [32]byte{})
		if packErr != nil || !ok || !bytes.Equal(data, append(adminError.ID[:4], packed...)) {
			t.Fatalf("%s.grantRole refusal = %x, %v", role.name, data, err)
		}
		r := s.miner(t)(contract.Transact(s.auth[0], "renounceRole", [32]byte{}, s.auth[0].From))
		if len(r.Logs) != 0 || !reflect.DeepEqual(before, snapshot()) || !reflect.DeepEqual(logsBefore, logs()) {
			t.Fatalf("%s non-holder renounce changed state or logs", role.name)
		}
	}
}

func assertRepairM948RoleLog(t *testing.T, receipt *types.Receipt, origin common.Address, consumer any, metadata *bind.MetaData, name string, want map[string]any) {
	t.Helper()
	parsed, err := metadata.GetAbi()
	if err != nil {
		t.Fatal(err)
	}
	event := parsed.Events[name]
	count := 0
	for _, log := range receipt.Logs {
		if len(log.Topics) == 0 || log.Topics[0] != event.ID || log.Address != origin {
			continue
		}
		count++
		if log.TxHash != receipt.TxHash || log.BlockHash != receipt.BlockHash || log.BlockNumber != receipt.BlockNumber.Uint64() {
			t.Fatalf("%s receipt provenance differs", name)
		}
		args := map[string]any{}
		for key, value := range want {
			raw, err := json.Marshal(repair37Humanize(value))
			if err != nil {
				t.Fatal(err)
			}
			var normalized any
			if err := json.Unmarshal(raw, &normalized); err != nil {
				t.Fatal(err)
			}
			args[key] = normalized
		}
		repairM948ParseExecution(t, consumer, event, repairM948Execution{Name: name, Log: *log, Args: args})
	}
	if count != 1 {
		t.Fatalf("%s at production origin %s: got %d logs", name, origin, count)
	}
}

func assertRepairM948NodeEvent(t *testing.T, s *repairM948Chain, receipt *types.Receipt, name string, want map[string]any) {
	t.Helper()
	native, err := b.NewNativeLaunchpad(protocol.ActiveAddresses.Precompile, s.client)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := b.NativeLaunchpadMetaData.GetAbi()
	if err != nil {
		t.Fatal(err)
	}
	var observed *types.Log
	for _, log := range receipt.Logs {
		if log.Address == protocol.ActiveAddresses.Precompile && len(log.Topics) > 0 && log.Topics[0] == metadata.Events[name].ID {
			if observed != nil {
				t.Fatalf("duplicate native %s", name)
			}
			observed = log
		}
	}
	if observed == nil {
		t.Fatalf("node did not emit native %s", name)
	}
	// Paxeer exposes its consensus block hash separately from Ethereum's
	// synthetic header hash; compare the hash returned by the actual RPC.
	var header struct {
		Hash      common.Hash
		Timestamp string
		Number    string
	}
	if err := s.client.Client().CallContext(context.Background(), &header, "eth_getBlockByNumber", fmt.Sprintf("0x%x", receipt.BlockNumber), false); err != nil {
		t.Fatal(err)
	}
	if observed.TxHash != receipt.TxHash || observed.BlockHash != receipt.BlockHash || observed.BlockHash != header.Hash || header.Number != fmt.Sprintf("0x%x", receipt.BlockNumber) {
		t.Fatalf("%s receipt/header provenance differs", name)
	}
	timestamp, ok := new(big.Int).SetString(header.Timestamp, 0)
	if !ok {
		t.Fatalf("invalid node block timestamp %q", header.Timestamp)
	}
	want["timestamp"] = timestamp
	want["blockNumber"] = receipt.BlockNumber
	results := reflect.ValueOf(native).MethodByName("Parse" + name).Call([]reflect.Value{reflect.ValueOf(*observed)})
	if !results[1].IsNil() {
		t.Fatal(results[1].Interface())
	}
	decoded := results[0].Elem()
	for key, value := range want {
		field := decoded.FieldByName(abi.ToCamelCase(key))
		if !field.IsValid() {
			t.Fatalf("native node %s lacks %s", name, key)
		}
		if integer, ok := value.(*big.Int); ok {
			got, ok := field.Interface().(*big.Int)
			if !ok || got.Cmp(integer) != 0 {
				t.Fatalf("native node %s.%s = %v, want %v", name, key, field.Interface(), value)
			}
		} else if !reflect.DeepEqual(field.Interface(), value) {
			t.Fatalf("native node %s.%s = %v, want %v", name, key, field.Interface(), value)
		}
	}
}

func repairM948ParseExecution(t *testing.T, consumer any, event abi.Event, observation repairM948Execution) {
	t.Helper()
	if len(observation.Log.Topics) == 0 || observation.Log.Topics[0] != event.ID {
		t.Fatalf("%s executor topic differs from canonical event", observation.Name)
	}
	parser := reflect.ValueOf(consumer).MethodByName("Parse" + event.Name)
	if !parser.IsValid() {
		t.Fatalf("missing generated parser %s", event.Name)
	}
	results := parser.Call([]reflect.Value{reflect.ValueOf(observation.Log)})
	if !results[1].IsNil() {
		t.Fatal(results[1].Interface())
	}
	decoded := results[0].Elem()
	if len(observation.Args) != len(event.Inputs) {
		t.Fatalf("%s expected field inventory differs", event.Name)
	}
	for _, argument := range event.Inputs {
		field := decoded.FieldByName(abi.ToCamelCase(argument.Name))
		want, ok := observation.Args[argument.Name]
		if !field.IsValid() || !ok {
			t.Fatalf("missing decoded field %s.%s", event.Name, argument.Name)
		}
		raw, err := json.Marshal(repair37Humanize(field.Interface()))
		if err != nil {
			t.Fatal(err)
		}
		var got any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s.%s = %#v, want %#v", event.Name, argument.Name, got, want)
		}
	}
	malformed := observation.Log
	malformed.Data = malformed.Data[:min(31, len(malformed.Data))]
	results = parser.Call([]reflect.Value{reflect.ValueOf(malformed)})
	if len(event.Inputs.NonIndexed()) > 0 && results[1].IsNil() {
		t.Fatalf("%s accepted malformed executor data", event.Name)
	}
	unknown := observation.Log
	unknown.Topics = append([]common.Hash(nil), observation.Log.Topics...)
	unknown.Topics[0] = common.HexToHash("0xdeadbeef")
	results = parser.Call([]reflect.Value{reflect.ValueOf(unknown)})
	if results[1].IsNil() {
		t.Fatalf("%s accepted an unknown topic", event.Name)
	}
}

type repairM948Execution struct {
	Role     string
	Name     string
	CalledAt common.Address
	Log      types.Log
	Args     map[string]any
}

func TestRepairM_948_RoleMetadata(t *testing.T) {
	proofLaunchpadRoleMetadata(t)
}

type repairM948Transcript struct {
	Revision     string
	SourceHashes map[string]string
	Family       []repairM948Execution
	Roles        []repairM948Execution
	Refusals     []struct {
		Role       string
		Name       string
		Error      string
		LogsBefore uint
		LogsAfter  uint
	}
}

func TestRepairM_948_NativeExecutorTransport(t *testing.T) {
	canonicalRaw, err := os.ReadFile(filepath.Join(repair37Root(t), "precompiles/launchpad/abi.json"))
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := abi.JSON(bytes.NewReader(canonicalRaw))
	if err != nil {
		t.Fatal(err)
	}
	native, err := b.NativeLaunchpadMetaData.GetAbi()
	if err != nil {
		t.Fatal(err)
	}
	assertProofABI(t, abi.ABI{Events: canonical.Events}, *native)
	// This helper-emitted transcript proves schemas and codecs only; production
	// role origins and initialization refusals are asserted by NativeExecutorNode.
	// Recorded executor output is the boundary between the node's geth fork
	// and this module's upstream geth types. Never synthesize producer logs.
	path := filepath.Join("testdata", "repair_m948", "native-execution.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("native executor evidence unavailable: %v; see repair_m948.kvx wiring.native_transcript", err)
	}
	var transcript repairM948Transcript
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&transcript); err != nil {
		t.Fatal(err)
	}
	if len(transcript.Revision) != 40 {
		t.Fatalf("missing producer revision: %q", transcript.Revision)
	}
	for _, path := range []string{"precompiles/launchpad/events.go", "modules/launchpad/keeper/keeper.go"} {
		source, err := os.ReadFile(filepath.Join(repair37Root(t), path))
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(source)
		if transcript.SourceHashes[path] != fmt.Sprintf("%x", hash) {
			t.Fatalf("executor transcript does not match %s", path)
		}
	}
	consumer, err := b.NewNativeLaunchpad(protocol.ActiveAddresses.Precompile, nil)
	if err != nil {
		t.Fatal(err)
	}
	families := []string{"Swap", "PoolStateUpdated", "TokenDeployed", "MarketCreated", "NftMint", "FeeStrategyChanged", "NftTransfer", "PoolRegistered", "RouterTrade", "ConfigUpdated", "FeeFlow", "FeeDistributed"}
	if len(transcript.Family) != len(families) {
		t.Fatalf("native family logs = %d, want %d", len(transcript.Family), len(families))
	}
	seen := map[string]bool{}
	for _, observation := range transcript.Family {
		if !slices.Contains(families, observation.Name) || seen[observation.Name] {
			t.Fatalf("unexpected or repeated family %s", observation.Name)
		}
		seen[observation.Name] = true
		if observation.Log.Address != protocol.ActiveAddresses.Precompile {
			t.Fatalf("%s originated at %s, want native 0x1017", observation.Name, observation.Log.Address)
		}
		repairM948ParseExecution(t, consumer, native.Events[observation.Name], observation)
		if observation.Args["timestamp"] == nil || observation.Args["blockNumber"] == nil {
			t.Fatalf("%s lacks native context", observation.Name)
		}
	}
	roles := map[string]struct {
		metadata *bind.MetaData
		bind     func(common.Address) (any, error)
	}{
		"FeeAccumulator": {b.FeeAccumulatorMetaData, func(a common.Address) (any, error) { return b.NewFeeAccumulator(a, nil) }},
		"PoolRegistry":   {b.PoolRegistryMetaData, func(a common.Address) (any, error) { return b.NewPoolRegistry(a, nil) }},
		"FeesRouter":     {b.FeesRouterMetaData, func(a common.Address) (any, error) { return b.NewFeesRouter(a, nil) }},
		"ProtocolConfig": {b.ProtocolConfigMetaData, func(a common.Address) (any, error) { return b.NewProtocolConfig(a, nil) }},
		"Quoter":         {b.QuoterMetaData, func(a common.Address) (any, error) { return b.NewQuoter(a, nil) }},
		"Router":         {b.RouterMetaData, func(a common.Address) (any, error) { return b.NewRouter(a, nil) }},
		"SidioraFactory": {b.SidioraFactoryMetaData, func(a common.Address) (any, error) { return b.NewSidioraFactory(a, nil) }},
		"SidioraNFT":     {b.SidioraNFTMetaData, func(a common.Address) (any, error) { return b.NewSidioraNFT(a, nil) }},
	}
	seenRoles := map[string]bool{}
	seenOrigins := map[string]map[common.Address]bool{}
	for _, observation := range transcript.Roles {
		role, ok := roles[observation.Role]
		if !ok || observation.Name == "Initialized" {
			t.Fatalf("invalid successful role emission %s.%s", observation.Role, observation.Name)
		}
		expected := protocol.ActiveAddresses.Roles[observation.Role]
		if observation.Name != "RoleGranted" && observation.Name != "RoleRevoked" {
			switch observation.Role {
			case "ProtocolConfig":
				expected = protocol.ActiveAddresses.Precompile
			case "Router", "FeesRouter":
				if observation.CalledAt != expected && observation.CalledAt != protocol.ActiveAddresses.Precompile {
					t.Fatalf("invalid called-at surface %s", observation.CalledAt)
				}
				expected = observation.CalledAt
			}
		}
		if observation.Log.Address != expected {
			t.Fatalf("%s.%s origin = %s, want %s", observation.Role, observation.Name, observation.Log.Address, expected)
		}
		consumer, err := role.bind(expected)
		if err != nil {
			t.Fatal(err)
		}
		metadata, err := role.metadata.GetAbi()
		if err != nil {
			t.Fatal(err)
		}
		event, ok := metadata.Events[observation.Name]
		if !ok {
			t.Fatalf("missing role event %s.%s", observation.Role, observation.Name)
		}
		repairM948ParseExecution(t, consumer, event, observation)
		seenRoles[observation.Role] = true
		if observation.Name != "RoleGranted" && observation.Name != "RoleRevoked" {
			if seenOrigins[observation.Role] == nil {
				seenOrigins[observation.Role] = map[common.Address]bool{}
			}
			seenOrigins[observation.Role][expected] = true
		}
	}
	for name := range roles {
		if !seenRoles[name] {
			t.Fatalf("no executor emission for role %s", name)
		}
	}
	for _, name := range []string{"Router", "FeesRouter"} {
		for _, address := range []common.Address{protocol.ActiveAddresses.Precompile, protocol.ActiveAddresses.Roles[name]} {
			if !seenOrigins[name][address] {
				t.Fatalf("missing %s helper schema origin %s", name, address)
			}
		}
	}
	refused := map[string]bool{}
	for _, refusal := range transcript.Refusals {
		if _, ok := roles[refusal.Role]; !ok || refused[refusal.Role] || refusal.Name != "Initialized" || refusal.Error != "launchpad: initialization event requires a new pool" || refusal.LogsBefore != refusal.LogsAfter {
			t.Fatalf("invalid non-pool initialization refusal: %+v", refusal)
		}
		refused[refusal.Role] = true
	}
	for name := range roles {
		if !refused[name] {
			t.Fatalf("missing %s Initialized helper guard refusal", name)
		}
	}
	if observed, ok := protocol.LookupEvent(common.HexToHash("0xdeadbeef")); ok || observed != nil {
		t.Fatal("lookup accepted unknown native topic")
	}
}

// The peer supplies only recorded responses from real contract execution.
// Exact method/parameter matching makes unrecorded traffic a hard failure.
type repairM948RPCCall struct {
	Method        string
	Params        json.RawMessage `json:",omitempty"`
	Result        json.RawMessage
	Error         json.RawMessage
	Notifications []json.RawMessage
}

type repairM948Replay struct {
	mu       sync.Mutex
	calls    []repairM948RPCCall
	next     int
	server   *httptest.Server
	endpoint string
	peers    map[*websocket.Conn]bool
}

func newRepairM948Replay(t *testing.T) *repairM948Replay {
	t.Helper()
	name := "emitter-rpc.json"
	if strings.Contains(t.Name(), "BeaconOwnershipHandoff") {
		name = "beacon-rpc.json"
	} else if strings.Contains(t.Name(), "NativeExecutorNode") {
		name = "native-node-rpc.json"
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "repair_m948", name))
	if err != nil {
		t.Fatalf("recorded retained-contract execution unavailable: %v; see repair_m948.kvx wiring.retained_transcripts", err)
	}
	var transcript struct {
		Revision       string
		ArtifactHashes map[string]string
		SourceHashes   map[string]string
		Calls          []repairM948RPCCall
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&transcript); err != nil {
		t.Fatal(err)
	}
	if len(transcript.Revision) != 40 || len(transcript.Calls) == 0 {
		t.Fatal("missing retained execution provenance or traffic")
	}
	for _, name := range []string{"EventEmitter", "ERC1967Proxy", "PoolBeacon"} {
		raw, err := os.ReadFile(filepath.Join("testdata", "repair37", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(raw)
		if transcript.ArtifactHashes[name] != fmt.Sprintf("%x", hash) {
			t.Fatalf("recorded execution does not match reviewed %s artifact", name)
		}
	}
	if name == "native-node-rpc.json" {
		revision, err := os.ReadFile("testdata/repair_m948/producer-revision.txt")
		if err != nil || transcript.Revision != strings.TrimSpace(string(revision)) {
			t.Fatalf("native producer provenance differs: %s, %v", transcript.Revision, err)
		}
		for _, role := range []string{"FeeAccumulator", "PoolRegistry", "FeesRouter", "ProtocolConfig", "Quoter", "Router", "SidioraFactory", "SidioraNFT"} {
			path := "precompiles/launchpad/abi/" + role + ".json"
			raw, err := os.ReadFile(filepath.Join(repair37Root(t), path))
			if err != nil || transcript.SourceHashes[path] != fmt.Sprintf("%x", sha256.Sum256(raw)) {
				t.Fatalf("native node ABI provenance differs for %s: %v", role, err)
			}
		}
		for _, path := range []string{"precompiles/launchpad/events.go", "precompiles/launchpad/router.go", "precompiles/launchpad/fees.go", "precompiles/launchpad/nft.go", "precompiles/launchpad/views.go", "precompiles/launchpad/abi.go", "precompiles/launchpad/abi.json", "precompiles/launchpad/run.go", "precompiles/launchpad/errors.go", "modules/launchpad/types/errors.go", "modules/launchpad/keeper/roles.go", "modules/launchpad/keeper/keeper.go", "node/testdata/v6120-halt-export.json"} {
			raw, err := os.ReadFile(filepath.Join(repair37Root(t), path))
			if err != nil {
				t.Fatal(err)
			}
			if transcript.SourceHashes[path] != fmt.Sprintf("%x", sha256.Sum256(raw)) {
				t.Fatalf("native node recording does not match %s", path)
			}
		}
	}
	r := &repairM948Replay{calls: transcript.Calls, peers: map[*websocket.Conn]bool{}}
	upgrader := websocket.Upgrader{}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		connection, err := upgrader.Upgrade(w, request, nil)
		if err != nil {
			t.Errorf("recorded RPC websocket: %v", err)
			return
		}
		r.mu.Lock()
		r.peers[connection] = true
		r.mu.Unlock()
		defer func() {
			connection.Close()
			r.mu.Lock()
			delete(r.peers, connection)
			r.mu.Unlock()
		}()
		for {
			var call struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      json.RawMessage `json:"id"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params"`
			}
			if err := connection.ReadJSON(&call); err != nil {
				return
			}
			r.mu.Lock()
			if r.next == len(r.calls) {
				r.mu.Unlock()
				t.Errorf("unrecorded RPC method %s", call.Method)
				return
			}
			want := r.calls[r.next]
			var gotParams, wantParams any
			var gotErr, wantErr error
			// An omitted params member is valid for a no-argument request.
			// Preserve that wire shape: omitted and explicit null must differ.
			if len(call.Params) != 0 || len(want.Params) != 0 {
				gotErr = json.Unmarshal(call.Params, &gotParams)
				wantErr = json.Unmarshal(want.Params, &wantParams)
			}
			if call.JSONRPC != "2.0" || call.Method != want.Method || gotErr != nil || wantErr != nil || !reflect.DeepEqual(gotParams, wantParams) {
				r.mu.Unlock()
				t.Errorf("RPC #%d = %s %s, want %s %s", r.next, call.Method, call.Params, want.Method, want.Params)
				return
			}
			r.next++
			r.mu.Unlock()
			response := map[string]any{"jsonrpc": "2.0", "id": call.ID}
			if len(want.Error) != 0 && !bytes.Equal(want.Error, []byte("null")) {
				response["error"] = want.Error
			} else {
				response["result"] = want.Result
			}
			if err := connection.WriteJSON(response); err != nil {
				t.Errorf("recorded RPC response: %v", err)
				return
			}
			for _, notification := range want.Notifications {
				if err := connection.WriteMessage(websocket.TextMessage, notification); err != nil {
					t.Errorf("recorded RPC notification: %v", err)
					return
				}
			}
		}
	}))
	r.endpoint = "ws" + strings.TrimPrefix(r.server.URL, "http")
	return r
}

func (r *repairM948Replay) Close() error {
	r.mu.Lock()
	for peer := range r.peers {
		peer.Close()
	}
	remaining := len(r.calls) - r.next
	r.mu.Unlock()
	r.server.Close()
	if remaining != 0 {
		return errors.New(fmt.Sprintf("%d recorded RPC calls were not consumed", remaining))
	}
	return nil
}
