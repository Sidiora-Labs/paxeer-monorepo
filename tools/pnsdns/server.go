package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/sha3"
	"golang.org/x/net/dns/dnsmessage"
)

type Handler struct {
	RPCURL    string
	Registry  string
	Resolver  string
	ShellA    []netip.Addr
	ShellAAAA []netip.Addr
	TTL       uint32
	Client    *http.Client
}

var zone = dnsmessage.MustNewName("pax.")

func (h *Handler) soa() dnsmessage.Resource {
	return dnsmessage.Resource{
		Header: dnsmessage.ResourceHeader{Name: zone, Class: dnsmessage.ClassINET, TTL: h.TTL},
		Body:   &dnsmessage.SOAResource{NS: zone, MBox: dnsmessage.MustNewName("hostmaster.pax."), Serial: 1, Refresh: 3600, Retry: 600, Expire: 86400, MinTTL: h.TTL},
	}
}

func (h *Handler) addresses(name dnsmessage.Name, kind dnsmessage.Type) []dnsmessage.Resource {
	var records []dnsmessage.Resource
	header := dnsmessage.ResourceHeader{Name: name, Class: dnsmessage.ClassINET, TTL: h.TTL}
	if kind == dnsmessage.TypeA {
		for _, address := range h.ShellA {
			records = append(records, dnsmessage.Resource{Header: header, Body: &dnsmessage.AResource{A: address.As4()}})
		}
	}
	if kind == dnsmessage.TypeAAAA {
		for _, address := range h.ShellAAAA {
			records = append(records, dnsmessage.Resource{Header: header, Body: &dnsmessage.AAAAResource{AAAA: address.As16()}})
		}
	}
	return records
}

func validLabel(label string) bool {
	if len(label) < 3 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for i := range label {
		c := label[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

func keccak(value []byte) []byte {
	hash := sha3.NewLegacyKeccak256()
	hash.Write(value)
	return hash.Sum(nil)
}

func namehash(label string) []byte {
	node := keccak(append(make([]byte, 32), keccak([]byte("pax"))...))
	return keccak(append(node, keccak([]byte(label))...))
}

func (h *Handler) rpc(ctx context.Context, method string, params any) (string, error) {
	request, err := json.Marshal(struct {
		Version string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{"2.0", 1, method, params})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.RPCURL, bytes.NewReader(request))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := h.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("RPC HTTP status %d", response.StatusCode)
	}
	var result struct {
		Version string  `json:"jsonrpc"`
		ID      int     `json:"id"`
		Result  *string `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != nil {
		return "", fmt.Errorf("RPC %d: %s", result.Error.Code, result.Error.Message)
	}
	if result.Version != "2.0" || result.ID != 1 || result.Result == nil {
		return "", errors.New("invalid RPC response")
	}
	return *result.Result, nil
}

func (h *Handler) call(ctx context.Context, address, signature string, node []byte, block string) ([]byte, error) {
	data := append(keccak([]byte(signature))[:4], node...)
	result, err := h.rpc(ctx, "eth_call", []any{map[string]string{"to": address, "data": "0x" + hex.EncodeToString(data)}, block})
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(result, "0x") {
		return nil, errors.New("RPC data is not hex")
	}
	return hex.DecodeString(result[2:])
}

func decodeContenthash(data []byte) ([]byte, error) {
	if len(data) < 64 || !bytes.Equal(data[:32], append(make([]byte, 31), 32)) {
		return nil, errors.New("invalid contenthash ABI offset")
	}
	if !bytes.Equal(data[32:63], make([]byte, 31)) {
		return nil, errors.New("invalid contenthash ABI length")
	}
	size := int(data[63])
	if size == 0 && len(data) == 64 {
		return nil, nil
	}
	if size != 36 || len(data) != 128 || !bytes.Equal(data[64:68], []byte("pax1")) || !bytes.Equal(data[100:], make([]byte, 28)) {
		return nil, errors.New("invalid Paxeer contenthash")
	}
	return data[64:100], nil
}

func (h *Handler) Answer(msg dnsmessage.Message) dnsmessage.Message {
	answer := dnsmessage.Message{
		Header:    dnsmessage.Header{ID: msg.ID, Response: true, Authoritative: true, OpCode: msg.OpCode, RecursionDesired: msg.RecursionDesired},
		Questions: msg.Questions,
	}
	if msg.Response || len(msg.Questions) != 1 {
		answer.RCode = dnsmessage.RCodeFormatError
		return answer
	}
	if msg.OpCode != 0 {
		answer.RCode = dnsmessage.RCodeNotImplemented
		return answer
	}
	question := msg.Questions[0]
	lower := []byte(question.Name.String())
	for i, c := range lower {
		if c >= 'A' && c <= 'Z' {
			lower[i] = c + ('a' - 'A')
		}
	}
	name := string(lower)
	if question.Class != dnsmessage.ClassINET || name != "pax." && !strings.HasSuffix(name, ".pax.") {
		answer.RCode = dnsmessage.RCodeRefused
		return answer
	}
	negative := func(code dnsmessage.RCode) dnsmessage.Message {
		answer.RCode = code
		answer.Authorities = []dnsmessage.Resource{h.soa()}
		return answer
	}
	if name == "pax." {
		switch question.Type {
		case dnsmessage.TypeSOA:
			answer.Answers = []dnsmessage.Resource{h.soa()}
		case dnsmessage.TypeNS:
			answer.Answers = []dnsmessage.Resource{{Header: dnsmessage.ResourceHeader{Name: zone, Class: dnsmessage.ClassINET, TTL: h.TTL}, Body: &dnsmessage.NSResource{NS: zone}}}
			answer.Additionals = append(h.addresses(zone, dnsmessage.TypeA), h.addresses(zone, dnsmessage.TypeAAAA)...)
		case dnsmessage.TypeA, dnsmessage.TypeAAAA:
			answer.Answers = h.addresses(question.Name, question.Type)
		}
		if len(answer.Answers) == 0 {
			return negative(dnsmessage.RCodeSuccess)
		}
		return answer
	}
	label := strings.TrimSuffix(name, ".pax.")
	metadata := strings.HasPrefix(label, "_pns.")
	if metadata {
		label = strings.TrimPrefix(label, "_pns.")
	}
	if !validLabel(label) {
		return negative(dnsmessage.RCodeNameError)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	block, err := h.rpc(ctx, "eth_blockNumber", []any{})
	if err != nil || !strings.HasPrefix(block, "0x") || len(block) < 3 {
		answer.RCode = dnsmessage.RCodeServerFailure
		return answer
	}
	height, err := strconv.ParseUint(block[2:], 16, 64)
	if err != nil || len(block) > 3 && block[2] == '0' {
		answer.RCode = dnsmessage.RCodeServerFailure
		return answer
	}
	node := namehash(label)
	owner, err := h.call(ctx, h.Registry, "owner(bytes32)", node, block)
	if err != nil || len(owner) != 32 || !bytes.Equal(owner[:12], make([]byte, 12)) {
		answer.RCode = dnsmessage.RCodeServerFailure
		return answer
	}
	if bytes.Equal(owner, make([]byte, 32)) {
		return negative(dnsmessage.RCodeNameError)
	}
	if metadata && question.Type == dnsmessage.TypeTXT {
		encoded, err := h.call(ctx, h.Resolver, "contenthash(bytes32)", node, block)
		if err != nil {
			answer.RCode = dnsmessage.RCodeServerFailure
			return answer
		}
		content, err := decodeContenthash(encoded)
		if err != nil {
			answer.RCode = dnsmessage.RCodeServerFailure
			return answer
		}
		if len(content) == 0 {
			return negative(dnsmessage.RCodeSuccess)
		}
		answer.Answers = []dnsmessage.Resource{{Header: dnsmessage.ResourceHeader{Name: question.Name, Class: dnsmessage.ClassINET, TTL: h.TTL}, Body: &dnsmessage.TXTResource{TXT: []string{"contenthash=0x" + hex.EncodeToString(content), "height=" + strconv.FormatUint(height, 10)}}}}
	} else if !metadata {
		answer.Answers = h.addresses(question.Name, question.Type)
	}
	if len(answer.Answers) == 0 {
		return negative(dnsmessage.RCodeSuccess)
	}
	return answer
}

func (h *Handler) answerWire(wire []byte, udp bool) ([]byte, error) {
	var query dnsmessage.Message
	if err := query.Unpack(wire); err != nil {
		return nil, err
	}
	answer := h.Answer(query)
	result, err := answer.Pack()
	if err == nil && udp && len(result) > 512 {
		answer.Truncated = true
		answer.Answers = nil
		answer.Authorities = nil
		answer.Additionals = nil
		result, err = answer.Pack()
	}
	return result, err
}

func (h *Handler) serveUDP(conn net.PacketConn) error {
	workers := make(chan struct{}, 128)
	for {
		buf := make([]byte, 65535)
		n, peer, err := conn.ReadFrom(buf)
		if err != nil {
			return err
		}
		workers <- struct{}{}
		go func() {
			defer func() { <-workers }()
			if answer, err := h.answerWire(buf[:n], true); err == nil {
				conn.WriteTo(answer, peer)
			}
		}()
	}
}

// Each admitted connection processes one request at a time, so this also caps
// concurrent TCP answer/RPC work. Pipelined frames stay on that connection and
// are answered in order without allocating a goroutine for each request.
const tcpConnectionLimit = 128

func (h *Handler) serveTCP(listener net.Listener) error {
	slots := make(chan struct{}, tcpConnectionLimit)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		select {
		case slots <- struct{}{}:
			go func() {
				// serveTCPConn closes the socket before returning, including on
				// malformed frames, EOF, deadline and write errors.
				defer func() { <-slots }()
				h.serveTCPConn(conn)
			}()
		default:
			// Refuse overload by closing immediately. Reading a question to
			// send a DNS error would itself admit unbounded slow-peer work.
			conn.Close()
		}
	}
}

func (h *Handler) serveTCPConn(conn net.Conn) {
	defer conn.Close()
	for {
		if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
			return
		}
		var size [2]byte
		if _, err := io.ReadFull(conn, size[:]); err != nil {
			return
		}
		wire := make([]byte, int(binary.BigEndian.Uint16(size[:])))
		if _, err := io.ReadFull(conn, wire); err != nil {
			return
		}
		answer, err := h.answerWire(wire, false)
		if err != nil {
			return
		}
		binary.BigEndian.PutUint16(size[:], uint16(len(answer)))
		if _, err := io.Copy(conn, bytes.NewReader(append(size[:], answer...))); err != nil {
			return
		}
	}
}
