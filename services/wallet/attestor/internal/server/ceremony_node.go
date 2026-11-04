package server

import (
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/audit"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/transport"
)

// CeremonyNodeConfig embeds the production HTTP and peer servers with their
// normal authority, inventory, audit and encrypted-store boundaries intact.
type CeremonyNodeConfig struct {
	NodeID, DataDir, NodeKeyFile                             string
	CertFile, KeyFile, CAFile, GatewayCAFile, OperatorCAFile string
	InventoryFile, InventoryPublicKeyFile, Issuer, Tenant    string
	ChainID                                                  uint64
	PolicyJSON                                               []byte
	BeforeRefreshCommit                                      func()
	peers                                                    []transport.Peer
}

func (c *CeremonyNodeConfig) AddPeer(id, address, pin string) {
	c.peers = append(c.peers, transport.Peer{ID: id, Address: address, SPKISHA256: pin})
}

type CeremonyNode struct {
	Server    *Server
	Store     *store.Store
	api       *http.Server
	transport *transport.Transport
	audit     *audit.Log
	once      sync.Once
	err       error
}

func (c CeremonyNodeConfig) Open(peer, api net.Listener) (_ *CeremonyNode, err error) {
	if peer == nil || api == nil {
		return nil, errors.New("ceremony node: listeners required")
	}
	doc, err := policy.Parse(c.PolicyJSON)
	if err != nil {
		return nil, err
	}
	key, err := config.ReadKeyFile(c.NodeKeyFile)
	if err != nil {
		return nil, err
	}
	defer func() {
		for i := range key {
			key[i] = 0
		}
	}()
	st, err := store.Open(filepath.Join(c.DataDir, "shares"), key)
	if err != nil {
		return nil, err
	}
	n := &CeremonyNode{Store: st}
	defer func() {
		if err != nil {
			_ = n.Close()
			_ = peer.Close()
			_ = api.Close()
		}
	}()
	n.audit, err = audit.Open(filepath.Join(c.DataDir, "audit"))
	if err != nil {
		return nil, err
	}
	n.transport, err = transport.New(transport.Config{SelfID: c.NodeID, CertFile: c.CertFile, KeyFile: c.KeyFile, CAFile: c.CAFile, OperatorCAFile: c.OperatorCAFile, Peers: c.peers})
	if err != nil {
		return nil, err
	}
	clients, err := LoadClientAuthorities(c.GatewayCAFile, c.OperatorCAFile)
	if err != nil {
		return nil, err
	}
	pins := make(map[string]string, len(c.peers))
	ids := make([]string, 0, len(c.peers))
	for _, p := range c.peers {
		pins[p.ID] = p.SPKISHA256
		ids = append(ids, p.ID)
	}
	inventory, err := NewInventory(c.InventoryFile, c.InventoryPublicKeyFile, c.Issuer, c.Tenant, st, pins)
	if err != nil {
		return nil, err
	}
	n.Server, err = New(Options{NodeID: c.NodeID, Region: "ceremony", ChainID: c.ChainID, Ceremony: true, Participants: ids, Store: st, Audit: n.audit, Transport: n.transport, Policy: policy.New(doc), Ledger: policy.NewMemoryLedger(time.Now), Clients: clients, Inventory: inventory})
	if err != nil {
		return nil, err
	}
	if c.BeforeRefreshCommit != nil {
		n.Server.beforeRefreshCommit = func(string) { c.BeforeRefreshCommit() }
	}
	tlsConfig, err := APITLSConfig(c.CertFile, c.KeyFile, clients)
	if err != nil {
		return nil, err
	}
	go func() { _ = n.transport.Serve(peer) }()
	n.api = n.Server.Serve(api, tlsConfig)
	return n, nil
}

func (n *CeremonyNode) Close() error {
	n.once.Do(func() {
		if n.api != nil {
			n.err = errors.Join(n.err, n.api.Close())
		}
		if n.transport != nil {
			n.err = errors.Join(n.err, n.transport.Close())
		}
		if n.Store != nil {
			n.err = errors.Join(n.err, n.Store.Close())
		}
		if n.audit != nil {
			n.err = errors.Join(n.err, n.audit.Close())
		}
	})
	return n.err
}
