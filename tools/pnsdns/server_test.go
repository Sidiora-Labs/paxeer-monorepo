package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/sha3"
	"golang.org/x/net/dns/dnsmessage"
)

func TestPnsDnsAnswersAAndTxtAndNxdomain(t *testing.T) {
	const registry = "0x1111111111111111111111111111111111111111"
	const resolver = "0x2222222222222222222222222222222222222222"
	const content = "70617831" + "abababababababababababababababababababababababababababababababab"
	nodeFor := func(label string) string {
		node := make([]byte, 32)
		for _, part := range []string{"pax", label} {
			labelHash := sha3.NewLegacyKeccak256()
			labelHash.Write([]byte(part))
			nodeHash := sha3.NewLegacyKeccak256()
			nodeHash.Write(node)
			nodeHash.Write(labelHash.Sum(nil))
			node = nodeHash.Sum(nil)
		}
		return hex.EncodeToString(node)
	}
	exampleNode, nobodyNode := nodeFor("example"), nodeFor("nobody")
	var calls atomic.Int32
	var failure atomic.Int32
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request struct {
			Version string            `json:"jsonrpc"`
			ID      int               `json:"id"`
			Method  string            `json:"method"`
			Params  []json.RawMessage `json:"params"`
		}
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Error("RPC must use a JSON POST")
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if request.Version != "2.0" || request.ID != 1 {
			t.Errorf("invalid RPC envelope: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		if failure.Load() == 1 {
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"error":{"code":-32000,"message":"unavailable"}}`, request.ID)
			return
		}
		var result string
		switch request.Method {
		case "eth_blockNumber":
			if len(request.Params) != 0 {
				t.Error("eth_blockNumber requires no parameters")
			}
			result = "0x2a"
		case "eth_call":
			if len(request.Params) != 2 {
				t.Error("eth_call requires transaction and block")
				http.Error(w, "invalid params", http.StatusBadRequest)
				return
			}
			var transaction map[string]string
			var block string
			if err := json.Unmarshal(request.Params[0], &transaction); err != nil {
				t.Error(err)
			}
			if err := json.Unmarshal(request.Params[1], &block); err != nil || block != "0x2a" {
				t.Errorf("read must be pinned to block 0x2a: %s", request.Params[1])
			}
			switch transaction["data"] {
			case "0x02571be3" + exampleNode:
				if transaction["to"] != registry {
					t.Error("owner call must target registry")
				}
				result = "0x" + strings.Repeat("0", 24) + strings.Repeat("12", 20)
			case "0x02571be3" + nobodyNode:
				if transaction["to"] != registry {
					t.Error("owner call must target registry")
				}
				result = "0x" + strings.Repeat("0", 64)
			case "0xbc1c58d1" + exampleNode:
				if transaction["to"] != resolver {
					t.Error("contenthash call must target resolver")
				}
				result = "0x" + fmt.Sprintf("%064x%064x", 32, 36) + content + strings.Repeat("0", 56)
				if failure.Load() == 2 {
					result = "0x" + fmt.Sprintf("%064x%064x", 32, 36) + "00000000" + content[8:] + strings.Repeat("0", 56)
				}
				if failure.Load() == 3 {
					result = "0x" + fmt.Sprintf("%064x%064x", 32, 0)
				}
			default:
				t.Errorf("unexpected call: %+v", transaction)
				http.Error(w, "unexpected call", http.StatusBadRequest)
				return
			}
		default:
			t.Errorf("unexpected RPC method %q", request.Method)
			http.Error(w, "unexpected method", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	defer rpc.Close()
	handler := &Handler{RPCURL: rpc.URL, Registry: registry, Resolver: resolver, ShellA: []netip.Addr{netip.MustParseAddr("192.0.2.80"), netip.MustParseAddr("192.0.2.81")}, ShellAAAA: []netip.Addr{netip.MustParseAddr("2001:db8::80")}, TTL: 60, Client: rpc.Client()}
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	udpDone := make(chan error, 1)
	go func() { udpDone <- handler.serveUDP(udp) }()
	tcp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	tcpDone := make(chan error, 1)
	go func() { tcpDone <- handler.serveTCP(tcp) }()
	query := func(t *testing.T, network, name string, kind dnsmessage.Type) dnsmessage.Message {
		t.Helper()
		question := dnsmessage.Question{Name: dnsmessage.MustNewName(name), Type: kind, Class: dnsmessage.ClassINET}
		msg := dnsmessage.Message{Header: dnsmessage.Header{ID: 1234, RecursionDesired: true}, Questions: []dnsmessage.Question{question}}
		wire, err := msg.Pack()
		if err != nil {
			t.Fatal(err)
		}
		address := udp.LocalAddr().String()
		if network == "tcp" {
			address = tcp.Addr().String()
		}
		conn, err := net.DialTimeout(network, address, 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if network == "tcp" {
			var length [2]byte
			binary.BigEndian.PutUint16(length[:], uint16(len(wire)))
			wire = append(length[:], wire...)
		}
		if _, err := conn.Write(wire); err != nil {
			t.Fatal(err)
		}
		var response []byte
		if network == "tcp" {
			var length [2]byte
			if _, err := io.ReadFull(conn, length[:]); err != nil {
				t.Fatal(err)
			}
			response = make([]byte, int(binary.BigEndian.Uint16(length[:])))
			if _, err := io.ReadFull(conn, response); err != nil {
				t.Fatal(err)
			}
		} else {
			buf := make([]byte, 65535)
			n, err := conn.Read(buf)
			if err != nil {
				t.Fatal(err)
			}
			response = buf[:n]
		}
		var answer dnsmessage.Message
		if err := answer.Unpack(response); err != nil {
			t.Fatal(err)
		}
		if answer.ID != msg.ID || !answer.Response || !answer.Authoritative || !answer.RecursionDesired || answer.RecursionAvailable || !reflect.DeepEqual(answer.Questions, msg.Questions) {
			t.Fatalf("invalid authoritative response: %+v", answer)
		}
		for _, resource := range append(append(answer.Answers, answer.Authorities...), answer.Additionals...) {
			if resource.Header.TTL != 60 || resource.Header.Class != dnsmessage.ClassINET {
				t.Fatalf("invalid TTL or class: %+v", resource.Header)
			}
		}
		return answer
	}
	assertSOA := func(t *testing.T, answer dnsmessage.Message, code dnsmessage.RCode) {
		t.Helper()
		if answer.RCode != code || len(answer.Answers) != 0 || len(answer.Authorities) != 1 {
			t.Fatalf("expected negative response with SOA: %+v", answer)
		}
		soa, ok := answer.Authorities[0].Body.(*dnsmessage.SOAResource)
		if !ok || answer.Authorities[0].Header.Name.String() != "pax." || soa.NS.String() != "pax." || soa.MinTTL != 60 {
			t.Fatalf("invalid negative SOA: %+v", answer.Authorities)
		}
	}
	for _, network := range []string{"udp", "tcp"} {
		t.Run(network, func(t *testing.T) {
			t.Run("A", func(t *testing.T) {
				answer := query(t, network, "ExAmPlE.PAX.", dnsmessage.TypeA)
				if answer.RCode != dnsmessage.RCodeSuccess || len(answer.Answers) != 2 {
					t.Fatalf("missing A records: %+v", answer)
				}
				for i, record := range answer.Answers {
					body, ok := record.Body.(*dnsmessage.AResource)
					if !ok || body.A != handler.ShellA[i].As4() || record.Header.Name.String() != "ExAmPlE.PAX." {
						t.Fatalf("wrong A answer: %+v", record)
					}
				}
			})
			t.Run("TXT", func(t *testing.T) {
				answer := query(t, network, "_pns.example.pax.", dnsmessage.TypeTXT)
				if answer.RCode != dnsmessage.RCodeSuccess || len(answer.Answers) != 1 {
					t.Fatalf("missing TXT answer: %+v", answer)
				}
				txt, ok := answer.Answers[0].Body.(*dnsmessage.TXTResource)
				if !ok || !reflect.DeepEqual(txt.TXT, []string{"contenthash=0x" + content, "height=42"}) {
					t.Fatalf("wrong TXT answer: %+v", answer.Answers)
				}
			})
			t.Run("NXDOMAIN", func(t *testing.T) {
				assertSOA(t, query(t, network, "nobody.pax.", dnsmessage.TypeA), dnsmessage.RCodeNameError)
			})
			t.Run("REFUSED", func(t *testing.T) {
				before := calls.Load()
				answer := query(t, network, "example.com.", dnsmessage.TypeA)
				if answer.RCode != dnsmessage.RCodeRefused || len(answer.Answers) != 0 || calls.Load() != before {
					t.Fatalf("foreign zone was not refused locally: %+v", answer)
				}
			})
			t.Run("AAAA", func(t *testing.T) {
				answer := query(t, network, "example.pax.", dnsmessage.TypeAAAA)
				if answer.RCode != dnsmessage.RCodeSuccess || len(answer.Answers) != 1 {
					t.Fatalf("missing AAAA: %+v", answer)
				}
				body, ok := answer.Answers[0].Body.(*dnsmessage.AAAAResource)
				if !ok || body.AAAA != handler.ShellAAAA[0].As16() {
					t.Fatalf("wrong AAAA: %+v", answer.Answers)
				}
			})
			t.Run("zone", func(t *testing.T) {
				before := calls.Load()
				soa := query(t, network, "pax.", dnsmessage.TypeSOA)
				if soa.RCode != dnsmessage.RCodeSuccess || len(soa.Answers) != 1 {
					t.Fatalf("missing zone SOA: %+v", soa)
				}
				if _, ok := soa.Answers[0].Body.(*dnsmessage.SOAResource); !ok {
					t.Fatal("wrong SOA type")
				}
				ns := query(t, network, "pax.", dnsmessage.TypeNS)
				if ns.RCode != dnsmessage.RCodeSuccess || len(ns.Answers) != 1 || len(ns.Additionals) != 3 {
					t.Fatalf("missing NS/glue: %+v", ns)
				}
				if body, ok := ns.Answers[0].Body.(*dnsmessage.NSResource); !ok || body.NS.String() != "pax." {
					t.Fatal("wrong NS")
				}
				if calls.Load() != before {
					t.Fatal("zone records must not use RPC")
				}
			})
			t.Run("invalid_labels", func(t *testing.T) {
				before := calls.Load()
				for _, name := range []string{"ab.pax.", "-abc.pax.", "abc-.pax.", "abc_def.pax.", "a.bbb.pax.", "_pns.ab.pax.", "Kelvin.pax."} {
					assertSOA(t, query(t, network, name, dnsmessage.TypeA), dnsmessage.RCodeNameError)
				}
				if calls.Load() != before {
					t.Fatal("invalid labels reached RPC")
				}
			})
			t.Run("unavailable", func(t *testing.T) {
				failure.Store(1)
				defer failure.Store(0)
				answer := query(t, network, "example.pax.", dnsmessage.TypeA)
				if answer.RCode != dnsmessage.RCodeServerFailure || len(answer.Answers) != 0 || len(answer.Authorities) != 0 {
					t.Fatalf("RPC failure misreported: %+v", answer)
				}
			})
			t.Run("malformed_contenthash", func(t *testing.T) {
				failure.Store(2)
				defer failure.Store(0)
				answer := query(t, network, "_pns.example.pax.", dnsmessage.TypeTXT)
				if answer.RCode != dnsmessage.RCodeServerFailure || len(answer.Answers) != 0 {
					t.Fatalf("malformed contenthash accepted: %+v", answer)
				}
			})
			t.Run("unset_contenthash", func(t *testing.T) {
				failure.Store(3)
				defer failure.Store(0)
				assertSOA(t, query(t, network, "_pns.example.pax.", dnsmessage.TypeTXT), dnsmessage.RCodeSuccess)
			})
		})
	}
	if err := udp.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-udpDone; err == nil {
		t.Fatal("UDP listener returned no close error")
	}
	if err := tcp.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-tcpDone; err == nil {
		t.Fatal("TCP listener returned no close error")
	}
	if bytes.Equal(namehash("example"), namehash("nobody")) {
		t.Fatal("distinct names share a node")
	}
}
