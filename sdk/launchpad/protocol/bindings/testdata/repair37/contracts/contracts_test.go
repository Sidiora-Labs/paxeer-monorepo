package contracts_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	b "github.com/Sidiora-Technologies/KindleLaunch/protocol/bindings"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
)

type chain struct {
	backend  *simulated.Backend
	client   *ethclient.Client
	endpoint string
	auth     []*bind.TransactOpts
}

func newChain(t *testing.T) *chain {
	t.Helper()
	s := &chain{endpoint: filepath.Join(t.TempDir(), "geth.ipc")}
	alloc := types.GenesisAlloc{}
	for i := 0; i < 3; i++ {
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		a, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
		if err != nil {
			t.Fatal(err)
		}
		s.auth = append(s.auth, a)
		alloc[a.From] = types.Account{Balance: new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil)}
	}
	s.backend = simulated.NewBackend(alloc, func(n *node.Config, e *ethconfig.Config) { n.IPCPath = s.endpoint })
	s.reopen(t)
	t.Cleanup(func() {
		s.client.Close()
		if err := s.backend.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}
func (s *chain) reopen(t *testing.T) {
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
func (s *chain) miner(t *testing.T) func(*types.Transaction, error) *types.Receipt {
	return func(tx *types.Transaction, err error) *types.Receipt {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		s.backend.Commit()
		r, err := s.client.TransactionReceipt(context.Background(), tx.Hash())
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != types.ReceiptStatusSuccessful {
			t.Fatalf("transaction %s reverted", tx.Hash())
		}
		return r
	}
}
func artifact(t *testing.T, name string) (abi.ABI, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", name+".json"))
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
func (s *chain) deploy(t *testing.T, name string, args ...any) common.Address {
	t.Helper()
	a, code := artifact(t, name)
	addr, tx, _, err := bind.DeployContract(s.auth[0], a, code, s.client, args...)
	s.miner(t)(tx, err)
	return addr
}
func unauthorized(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("unauthorized operation accepted")
	}
	data, ok := ethclient.RevertErrorData(err)
	if !ok || !bytes.Equal(data, crypto.Keccak256([]byte("Unauthorized()"))[:4]) {
		t.Fatalf("want Unauthorized selector, got %x (%v)", data, err)
	}
}
func TestRepair_37_beacon_ownership_handoff(t *testing.T) {
	s := newChain(t)
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
	// Reconnect over the actual node IPC transport, discarding the previous client and binding.
	s.reopen(t)
	beacon, err = b.NewPoolBeacon(address, s.client)
	if err != nil {
		t.Fatal(err)
	}
	assertOwners(beacon, s.auth[0].From, s.auth[1].From)
	_, err = beacon.AcceptOwnership(s.auth[2])
	unauthorized(t, err)
	_, err = beacon.AcceptOwnership(s.auth[0])
	unauthorized(t, err)
	assertOwners(beacon, s.auth[0].From, s.auth[1].From)
	receipt = mine(beacon.AcceptOwnership(s.auth[1]))
	transferred, err := beacon.ParseOwnershipTransferred(*receipt.Logs[0])
	if err != nil || transferred.PreviousOwner != s.auth[0].From || transferred.NewOwner != s.auth[1].From {
		t.Fatalf("ownership accepted = %+v, %v", transferred, err)
	}
	assertOwners(beacon, s.auth[1].From, common.Address{})
	_, err = beacon.TransferOwnership(s.auth[0], s.auth[2].From)
	unauthorized(t, err)
	_, err = beacon.AcceptOwnership(s.auth[1])
	unauthorized(t, err)
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

func TestRepair_37_retained_emitter_v2(t *testing.T) {
	s := newChain(t)
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
	unauthorized(t, err)
	mine(emitter.SetAuthorizedEmitter(s.auth[1], s.auth[1].From, true))
	selector := [4]byte(a.Methods["emitFeeFlow"].ID)
	allowed, err := emitter.IsTypedEmitter(nil, s.auth[1].From, selector)
	if err != nil || allowed {
		t.Fatalf("ungranted typed permission = %v, %v", allowed, err)
	}
	_, err = emitter.EmitFeeFlow(s.auth[1], 1, s.auth[1].From, s.auth[2].From, big.NewInt(90), big.NewInt(10), big.NewInt(80), big.NewInt(7))
	unauthorized(t, err)
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
	unauthorized(t, err)
	data := tupleData(s.auth[1].From)
	name := "Repair37 λ generic"
	nameHash := crypto.Keccak256Hash([]byte(name))
	topic1 := crypto.Keccak256Hash([]byte("pool"))
	topic2 := crypto.Keccak256Hash([]byte("user"))
	_, err = emitter.EmitEventLog(s.auth[2], name, data)
	unauthorized(t, err)
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
	assertGeneric(t, s, receipt, log.MsgSender, log.EventNameHash, log.EventName, log.EventData, log.Timestamp, log.BlockNumber, data, name)
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
	assertGeneric(t, s, receipt, log1.MsgSender, log1.EventNameHash, log1.EventName, log1.EventData, log1.Timestamp, log1.BlockNumber, data, name)
	if len(receipt.Logs[0].Topics) != 4 || log1.Topic1 != topic1 || receipt.Logs[0].Topics[3] != topic1 {
		t.Fatalf("EventLog1 indexed topic = %+v", log1)
	}
	receipt = mine(emitter.EmitEventLog2(s.auth[1], name, topic1, topic2, data))
	log2, err := emitter.ParseEventLog2(*receipt.Logs[0])
	if err != nil {
		t.Fatal(err)
	}
	assertGeneric(t, s, receipt, log2.MsgSender, log2.EventNameHash, log2.EventName, log2.EventData, log2.Timestamp, log2.BlockNumber, data, name)
	if len(receipt.Logs[0].Topics) != 4 || log2.Topic1 != topic1 || log2.Topic2 != topic2 || receipt.Logs[0].Topics[3] != topic1 {
		t.Fatalf("EventLog2 topic encoding = %+v", log2)
	}
	malformed := *receipt.Logs[0]
	malformed.Data = malformed.Data[:31]
	if _, err = emitter.ParseEventLog2(malformed); err == nil {
		t.Fatal("truncated tuple accepted")
	}
}
func assertGeneric(t *testing.T, s *chain, r *types.Receipt, sender common.Address, hash [32]byte, name string, data b.IEventEmitterEventData, timestamp, number *big.Int, want b.IEventEmitterEventData, wantName string) {
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
func tupleData(address common.Address) b.IEventEmitterEventData {
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
