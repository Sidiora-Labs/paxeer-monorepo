package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

type addressFlags struct {
	v6     bool
	values []netip.Addr
}

func (f *addressFlags) String() string {
	var values []string
	for _, value := range f.values {
		values = append(values, value.String())
	}
	return strings.Join(values, ",")
}

func (f *addressFlags) Set(value string) error {
	address, err := netip.ParseAddr(value)
	if err != nil || address.Zone() != "" || f.v6 && (!address.Is6() || address.Is4In6()) || !f.v6 && !address.Is4() {
		return fmt.Errorf("invalid shell IPv%d address %q", map[bool]int{false: 4, true: 6}[f.v6], value)
	}
	f.values = append(f.values, address)
	return nil
}

func contractAddress(value string) bool {
	if len(value) != 42 || !strings.HasPrefix(value, "0x") {
		return false
	}
	nonzero := false
	for _, c := range value[2:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
		nonzero = nonzero || c != '0'
	}
	return nonzero
}

func main() {
	listen := flag.String("listen", ":5353", "UDP and TCP listen address")
	rpc := flag.String("rpc", "", "HTTP JSON-RPC endpoint")
	registry := flag.String("registry", "", "registry contract address")
	resolver := flag.String("resolver", "", "resolver contract address")
	ttl := flag.Uint64("ttl", 60, "record TTL in seconds")
	a, aaaa := addressFlags{}, addressFlags{v6: true}
	flag.Var(&a, "shell-a", "shell IPv4 address (repeatable)")
	flag.Var(&aaaa, "shell-aaaa", "shell IPv6 address (repeatable)")
	flag.Parse()
	endpoint, err := url.Parse(*rpc)
	if err != nil || endpoint.Host == "" || endpoint.Scheme != "http" && endpoint.Scheme != "https" || !contractAddress(*registry) || !contractAddress(*resolver) || *ttl > 0xffffffff || flag.NArg() != 0 || len(a.values)+len(aaaa.values) == 0 {
		log.Fatal("provide -rpc, nonzero -registry and -resolver addresses, at least one shell address, and a TTL within uint32")
	}
	handler := Handler{RPCURL: *rpc, Registry: *registry, Resolver: *resolver, ShellA: a.values, ShellAAAA: aaaa.values, TTL: uint32(*ttl)}
	tcp, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	defer tcp.Close()
	udp, err := net.ListenPacket("udp", tcp.Addr().String())
	if err != nil {
		log.Fatal(err)
	}
	defer udp.Close()
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopped)
	failures := make(chan error, 2)
	go func() { failures <- handler.serveTCP(tcp) }()
	go func() { failures <- handler.serveUDP(udp) }()
	log.Printf("serving pax. on %s (UDP and TCP)", tcp.Addr())
	select {
	case <-stopped:
	case err := <-failures:
		if !errors.Is(err, net.ErrClosed) {
			log.Fatal(err)
		}
	}
}
