package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/params"
	"golang.org/x/net/dns/dnsmessage"
)

func repairM866Query(t *testing.T, id uint16, name string, kind dnsmessage.Type) []byte {
	t.Helper()
	wire, err := (&dnsmessage.Message{
		Header:    dnsmessage.Header{ID: id, RecursionDesired: true},
		Questions: []dnsmessage.Question{{Name: dnsmessage.MustNewName(name), Type: kind, Class: dnsmessage.ClassINET}},
	}).Pack()
	if err != nil {
		t.Fatal(err)
	}
	return wire
}

func repairM866Frame(wire []byte) []byte {
	frame := make([]byte, 2+len(wire))
	binary.BigEndian.PutUint16(frame, uint16(len(wire)))
	copy(frame[2:], wire)
	return frame
}

func repairM866ReadTCP(conn net.Conn) (dnsmessage.Message, error) {
	var msg dnsmessage.Message
	var size [2]byte
	if _, err := io.ReadFull(conn, size[:]); err != nil {
		return msg, err
	}
	wire := make([]byte, int(binary.BigEndian.Uint16(size[:])))
	if _, err := io.ReadFull(conn, wire); err != nil {
		return msg, err
	}
	return msg, msg.Unpack(wire)
}

func repairM866ListenTCP(t *testing.T, h *Handler) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- h.serveTCP(listener) }()
	t.Cleanup(func() {
		listener.Close()
		select {
		case err := <-done:
			if !errors.Is(err, net.ErrClosed) {
				t.Errorf("TCP listener shutdown: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("TCP accept loop did not stop")
		}
	})
	return listener
}

func repairM866Dial(t *testing.T, network, address string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout(network, address, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return conn
}

func TestRepairM_866_TCPCapacity(t *testing.T) {
	h := &Handler{TTL: 60, ShellA: []netip.Addr{netip.MustParseAddr("192.0.2.80")}}
	listener := repairM866ListenTCP(t, h)
	query := repairM866Frame(repairM866Query(t, 77, "pax.", dnsmessage.TypeA))
	assertAnswer := func(msg dnsmessage.Message, err error, id uint16) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		if msg.ID != id || !msg.Response || !msg.Authoritative || msg.RCode != dnsmessage.RCodeSuccess || len(msg.Answers) != 1 {
			t.Fatalf("admitted connection returned invalid answer: %+v", msg)
		}
		body, ok := msg.Answers[0].Body.(*dnsmessage.AResource)
		if !ok || body.A != h.ShellA[0].As4() {
			t.Fatalf("wrong shell A record: %+v", msg.Answers)
		}
	}
	// A successful response proves each socket was admitted before the next
	// one is opened; filling the kernel accept queue alone would not prove it.
	peers := make([]net.Conn, tcpConnectionLimit)
	for i := range peers {
		peers[i] = repairM866Dial(t, "tcp", listener.Addr().String())
		if _, err := peers[i].Write(query); err != nil {
			t.Fatal(err)
		}
		msg, err := repairM866ReadTCP(peers[i])
		assertAnswer(msg, err, 77)
	}
	assertOverload := func() {
		t.Helper()
		conn := repairM866Dial(t, "tcp", listener.Addr().String())
		defer conn.Close()
		// A silent idle excess peer must be closed without waiting for a
		// question, allocating a worker or consuming an RPC timeout.
		var one [1]byte
		_, err := conn.Read(one[:])
		if err == nil {
			t.Fatal("overloaded TCP listener returned data")
		}
		if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			t.Fatal("overloaded TCP connection stayed open")
		}
	}
	assertOverload()

	// Pipeline several complete frames in one write at full capacity. They
	// must all be answered in order on the already admitted connection.
	var pipeline []byte
	for id := uint16(100); id < 108; id++ {
		pipeline = append(pipeline, repairM866Frame(repairM866Query(t, id, "pax.", dnsmessage.TypeA))...)
	}
	if _, err := peers[0].Write(pipeline); err != nil {
		t.Fatal(err)
	}
	for id := uint16(100); id < 108; id++ {
		msg, err := repairM866ReadTCP(peers[0])
		assertAnswer(msg, err, id)
	}

	// UDP remains available while TCP is full.
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	udpDone := make(chan error, 1)
	go func() { udpDone <- h.serveUDP(udp) }()
	t.Cleanup(func() { udp.Close(); <-udpDone })
	u := repairM866Dial(t, "udp", udp.LocalAddr().String())
	if _, err := u.Write(query[2:]); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 512)
	n, err := u.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	var answer dnsmessage.Message
	err = answer.Unpack(buf[:n])
	assertAnswer(answer, err, 77)

	// Exercise the real read-header, read-body, DNS decode and write-side
	// half-close paths. Each return must release exactly one admission slot.
	for i, ending := range []string{"EOF", "partial_header", "partial_body", "malformed_DNS", "pipeline_half_close"} {
		t.Logf("slot release: %s", ending)
		{
			conn := peers[i+1]
			switch ending {
			case "partial_header":
				if _, err := conn.Write([]byte{0}); err != nil {
					t.Fatal(err)
				}
			case "partial_body":
				if _, err := conn.Write([]byte{0, 10, 1}); err != nil {
					t.Fatal(err)
				}
			case "malformed_DNS":
				if _, err := conn.Write([]byte{0, 1, 0}); err != nil {
					t.Fatal(err)
				}
			case "pipeline_half_close":
				if _, err := conn.Write(pipeline); err != nil {
					t.Fatal(err)
				}
				if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
					t.Fatal(err)
				}
				for id := uint16(100); id < 108; id++ {
					msg, err := repairM866ReadTCP(conn)
					assertAnswer(msg, err, id)
				}
			}
			if ending != "pipeline_half_close" {
				if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
					t.Fatal(err)
				}
			}
			var one [1]byte
			if _, err := conn.Read(one[:]); err == nil {
				t.Fatal("connection did not close after terminal frame/EOF")
			} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				t.Fatalf("server retained terminal connection for %s", ending)
			}
			conn.Close()
			// Closing is asynchronous at the server; retry only admission,
			// within a deadline, until the released slot can answer again.
			deadline := time.Now().Add(2 * time.Second)
			for {
				replacement := repairM866Dial(t, "tcp", listener.Addr().String())
				_, writeErr := replacement.Write(query)
				msg, readErr := repairM866ReadTCP(replacement)
				if writeErr == nil && readErr == nil {
					assertAnswer(msg, nil, 77)
					peers[i+1] = replacement
					break
				}
				replacement.Close()
				if time.Now().After(deadline) {
					t.Fatalf("slot not released: write=%v read=%v", writeErr, readErr)
				}
				time.Sleep(time.Millisecond)
			}
			assertOverload()
		}
	}
}

// The bytecode is the committed output of the source-level Solidity test
// testResolverLayoutVectorIsCurrent, not a hand-written eth_call transcript.
// Pin both its producers (including imports/compiler settings) and its bytes so
// an upstream change requires an explicit fixture review rather than stale code.
func repairM866Contracts(t *testing.T) map[string]struct {
	CreationCode hexutil.Bytes `json:"creation_code"`
} {
	t.Helper()
	read := func(path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	var hashes map[string]string
	if err := json.Unmarshal(read("testdata/names_sources.json"), &hashes); err != nil {
		t.Fatal(err)
	}
	if len(hashes) != 7 {
		t.Fatalf("missing source bindings: %v", hashes)
	}
	for path, want := range hashes {
		sum := sha256.Sum256(read(filepath.Join("../..", path)))
		if hex.EncodeToString(sum[:]) != want {
			t.Fatalf("contract fixture source changed: %s; refresh the Solidity vector and source bindings", path)
		}
	}
	var layout struct {
		Version   int `json:"version"`
		Contracts map[string]struct {
			CreationCode hexutil.Bytes `json:"creation_code"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal(read("../../tests/vectors/names/resolver_layout_v1.json"), &layout); err != nil {
		t.Fatal(err)
	}
	if layout.Version != 1 || len(layout.Contracts) != 2 {
		t.Fatalf("unexpected contract fixture: %+v", layout)
	}
	for _, name := range []string{"PaxRegistry", "PaxPublicResolver"} {
		if len(layout.Contracts[name].CreationCode) == 0 {
			t.Fatalf("missing creation code for %s", name)
		}
	}
	return layout.Contracts
}

func TestRepairM_866_RealContractRPC(t *testing.T) {
	contracts := repairM866Contracts(t)
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	deployer := crypto.PubkeyToAddress(key.PublicKey)
	registry, resolver := crypto.CreateAddress(deployer, 0), crypto.CreateAddress(deployer, 1)
	parseABI := func(text string) abi.ABI {
		t.Helper()
		parsed, err := abi.JSON(strings.NewReader(text))
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	registryABI := parseABI(`[
		{"type":"function","name":"setSubnodeOwner","inputs":[{"type":"bytes32"},{"type":"bytes32"},{"type":"address"}]},
		{"type":"function","name":"setResolver","inputs":[{"type":"bytes32"},{"type":"address"}]},
		{"type":"function","name":"setOwner","inputs":[{"type":"bytes32"},{"type":"address"}]}]`)
	resolverABI := parseABI(`[
		{"type":"constructor","inputs":[{"type":"address"},{"type":"address"},{"type":"address"}]},
		{"type":"function","name":"setContenthash","inputs":[{"type":"bytes32"},{"type":"bytes"}]}]`)
	pack := func(contract abi.ABI, method string, args ...any) []byte {
		t.Helper()
		data, err := contract.Pack(method, args...)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	chainConfig := *params.AllDevChainProtocolChanges
	chainConfig.ChainID = big.NewInt(125)
	genesis := &core.Genesis{
		Config: &chainConfig, GasLimit: 30_000_000, Timestamp: 1000,
		Alloc: types.GenesisAlloc{deployer: {Balance: new(big.Int).Exp(big.NewInt(10), big.NewInt(21), nil)}},
	}
	paxLabel, exampleLabel := crypto.Keccak256Hash([]byte("pax")), crypto.Keccak256Hash([]byte("example"))
	paxNode := crypto.Keccak256Hash(make([]byte, 32), paxLabel[:])
	exampleNode := crypto.Keccak256Hash(paxNode[:], exampleLabel[:])
	content := append([]byte("pax1"), crypto.Keccak256([]byte("first manifest"))...)
	nextContent := append([]byte("pax1"), crypto.Keccak256([]byte("second manifest"))...)
	var nonce uint64
	engine := beacon.New(ethash.NewFaker())
	t.Cleanup(func() { engine.Close() })
	db, blocks, receipts := core.GenerateChainWithGenesis(genesis, engine, 4, func(i int, block *core.BlockGen) {
		send := func(to *common.Address, data []byte) {
			t.Helper()
			tx, err := types.SignTx(types.NewTx(&types.LegacyTx{
				Nonce: nonce, To: to, Gas: 5_000_000, GasPrice: big.NewInt(params.InitialBaseFee), Data: data,
			}), types.LatestSigner(&chainConfig), key)
			if err != nil {
				t.Fatal(err)
			}
			block.AddTx(tx)
			nonce++
		}
		switch i {
		case 0:
			send(nil, contracts["PaxRegistry"].CreationCode)
			send(nil, append(append([]byte(nil), contracts["PaxPublicResolver"].CreationCode...), pack(resolverABI, "", registry, deployer, deployer)...))
		case 1:
			send(&registry, pack(registryABI, "setSubnodeOwner", common.Hash{}, paxLabel, deployer))
			send(&registry, pack(registryABI, "setSubnodeOwner", paxNode, exampleLabel, deployer))
			send(&registry, pack(registryABI, "setResolver", exampleNode, resolver))
			send(&resolver, pack(resolverABI, "setContenthash", exampleNode, content))
		case 2:
			send(&resolver, pack(resolverABI, "setContenthash", exampleNode, nextContent))
		case 3:
			send(&registry, pack(registryABI, "setOwner", exampleNode, common.Address{}))
		}
	})
	t.Cleanup(func() { db.Close() })
	for i, group := range receipts {
		for _, receipt := range group {
			if receipt.Status != types.ReceiptStatusSuccessful {
				t.Fatalf("real contract transaction reverted in block %d: %+v", i+1, receipt)
			}
		}
	}
	if receipts[0][0].ContractAddress != registry || receipts[0][1].ContractAddress != resolver {
		t.Fatal("deployment address mismatch")
	}

	stack, err := node.New(&node.Config{
		Name: "pnsdns-contract-test", HTTPHost: "127.0.0.1", HTTPPort: 0,
		HTTPModules: []string{"eth"}, HTTPVirtualHosts: []string{"127.0.0.1"},
		P2P: p2p.Config{NoDiscovery: true, NoDial: true, ListenAddr: "", MaxPeers: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stack.Close(); err != nil {
			t.Error(err)
		}
	})
	config := ethconfig.Defaults
	config.Genesis, config.SyncMode = genesis, ethconfig.FullSync
	config.DatabaseCache, config.TrieCleanCache, config.TrieDirtyCache, config.SnapshotCache = 16, 16, 16, 0
	config.TxPool.Journal, config.BlobPool.Datadir, config.LogNoHistory = "", "", true
	service, err := eth.New(stack, &config)
	if err != nil {
		t.Fatal(err)
	}
	if err := stack.Start(); err != nil {
		t.Fatal(err)
	}
	insert := func(chain []*types.Block) {
		t.Helper()
		if _, err := service.BlockChain().InsertChain(chain); err != nil {
			t.Fatal(err)
		}
		if _, err := service.BlockChain().SetCanonical(chain[len(chain)-1]); err != nil {
			t.Fatal(err)
		}
	}
	insert(blocks[:2])
	h := &Handler{RPCURL: stack.HTTPEndpoint(), Registry: registry.Hex(), Resolver: resolver.Hex(), TTL: 60,
		ShellA: []netip.Addr{netip.MustParseAddr("192.0.2.80")}, ShellAAAA: []netip.Addr{netip.MustParseAddr("2001:db8::80")}}
	tcp := repairM866ListenTCP(t, h)
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	udpDone := make(chan error, 1)
	go func() { udpDone <- h.serveUDP(udp) }()
	t.Cleanup(func() { udp.Close(); <-udpDone })
	query := func(network, name string, kind dnsmessage.Type) dnsmessage.Message {
		t.Helper()
		wire := repairM866Query(t, 866, name, kind)
		address := udp.LocalAddr().String()
		if network == "tcp" {
			address = tcp.Addr().String()
		}
		conn := repairM866Dial(t, network, address)
		defer conn.Close()
		if network == "tcp" {
			wire = repairM866Frame(wire)
		}
		if _, err := conn.Write(wire); err != nil {
			t.Fatal(err)
		}
		var answer dnsmessage.Message
		if network == "tcp" {
			answer, err = repairM866ReadTCP(conn)
		} else {
			buf := make([]byte, 4096)
			var n int
			n, err = conn.Read(buf)
			if err == nil {
				err = answer.Unpack(buf[:n])
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		if answer.ID != 866 || !answer.Response || !answer.Authoritative || !answer.RecursionDesired || answer.RecursionAvailable || len(answer.Questions) != 1 || answer.Questions[0].Name.String() != name || answer.Questions[0].Type != kind {
			t.Fatalf("invalid DNS reply: %+v", answer)
		}
		for _, rr := range append(answer.Answers, answer.Authorities...) {
			if rr.Header.TTL != h.TTL || rr.Header.Class != dnsmessage.ClassINET {
				t.Fatalf("wrong resource header: %+v", rr.Header)
			}
		}
		return answer
	}
	assertNegative := func(answer dnsmessage.Message) {
		t.Helper()
		if answer.RCode != dnsmessage.RCodeNameError || len(answer.Answers) != 0 || len(answer.Authorities) != 1 {
			t.Fatalf("ownerless name must be NXDOMAIN with SOA: %+v", answer)
		}
		soa, ok := answer.Authorities[0].Body.(*dnsmessage.SOAResource)
		if !ok || soa.MinTTL != 60 || soa.NS.String() != "pax." {
			t.Fatalf("invalid NXDOMAIN SOA: %+v", answer.Authorities)
		}
	}
	assertTXT := func(answer dnsmessage.Message, content []byte, height int) {
		t.Helper()
		if answer.RCode != dnsmessage.RCodeSuccess || len(answer.Answers) != 1 {
			t.Fatalf("missing contract TXT: %+v", answer)
		}
		txt, ok := answer.Answers[0].Body.(*dnsmessage.TXTResource)
		want := []string{"contenthash=0x" + hex.EncodeToString(content), fmt.Sprintf("height=%d", height)}
		if !ok || !reflect.DeepEqual(txt.TXT, want) {
			t.Fatalf("contract TXT mismatch: got %+v want %v", answer.Answers, want)
		}
	}
	for _, network := range []string{"udp", "tcp"} {
		t.Logf("real contract DNS transport: %s", network)
		{
			assertNegative(query(network, "nobody.pax.", dnsmessage.TypeA))
			assertTXT(query(network, "_pns.example.pax.", dnsmessage.TypeTXT), content, 2)
			for _, kind := range []dnsmessage.Type{dnsmessage.TypeA, dnsmessage.TypeAAAA} {
				answer := query(network, "ExAmPlE.PAX.", kind)
				if answer.RCode != dnsmessage.RCodeSuccess || len(answer.Answers) != 1 {
					t.Fatalf("missing shell address: %+v", answer)
				}
				if kind == dnsmessage.TypeA {
					body, ok := answer.Answers[0].Body.(*dnsmessage.AResource)
					if !ok || body.A != h.ShellA[0].As4() {
						t.Fatalf("wrong A: %+v", answer.Answers)
					}
				} else {
					body, ok := answer.Answers[0].Body.(*dnsmessage.AAAAResource)
					if !ok || body.AAAA != h.ShellAAAA[0].As16() {
						t.Fatalf("wrong AAAA: %+v", answer.Answers)
					}
				}
			}
		}
	}
	// Advance real node state: stale or canned contract replies cannot pass.
	insert(blocks[2:3])
	for _, network := range []string{"udp", "tcp"} {
		assertTXT(query(network, "_pns.example.pax.", dnsmessage.TypeTXT), nextContent, 3)
	}
	insert(blocks[3:])
	for _, network := range []string{"udp", "tcp"} {
		for _, kind := range []dnsmessage.Type{dnsmessage.TypeA, dnsmessage.TypeAAAA} {
			assertNegative(query(network, "example.pax.", kind))
		}
		assertNegative(query(network, "_pns.example.pax.", dnsmessage.TypeTXT))
	}
}
