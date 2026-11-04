package e2e_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"golang.org/x/time/rate"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	cstypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p/conn"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/tcp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpc "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/app"
	noderuntime "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/node/runtime"
	e2e "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/pkg"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func repair1192Manifest(t *testing.T, manifest e2e.Manifest) *e2e.Testnet {
	t.Helper()
	file := filepath.Join(t.TempDir(), "peers.toml")
	require.NoError(t, manifest.Save(file))
	testnet, err := e2e.LoadTestnet(file)
	require.NoError(t, err)
	return testnet
}

func repair1192Routers(t *testing.T, testnet *e2e.Testnet, discovery bool) (map[*e2e.Node]*p2p.Router, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	t.Cleanup(cancel)
	endpoints := make(map[*e2e.Node]p2p.Endpoint)
	for _, node := range testnet.Nodes {
		node.IP = net.ParseIP("127.0.0.1")
		endpoints[node] = p2p.Endpoint{AddrPort: tcp.TestReserveAddr()}
	}
	routers := make(map[*e2e.Node]*p2p.Router)
	for _, node := range testnet.Nodes {
		key := p2p.NodeSecretKey(node.NodeKey)
		endpoint := endpoints[node]
		info := types.NodeInfo{NodeID: key.Public().NodeID(), Network: "repair1192", Moniker: node.Name, ListenAddr: endpoint.String()}
		options := &p2p.RouterOptions{
			Endpoint: endpoint, SelfAddress: utils.Some(endpoint.NodeAddress(info.NodeID)),
			Connection: conn.DefaultMConnConfig(), PexOnHandshake: discovery, MaxDialRate: utils.Some(rate.Inf), MaxAcceptRate: utils.Some(rate.Inf),
			IncomingConnectionWindow: utils.Some(time.Duration(0)),
		}
		for _, peer := range node.PersistentPeers {
			options.PersistentPeers = append(options.PersistentPeers, endpoints[peer].NodeAddress(types.NodeIDFromPubKey(peer.NodeKey.Public())))
		}
		for _, seed := range node.Seeds {
			options.BootstrapPeers = append(options.BootstrapPeers, endpoints[seed].NodeAddress(types.NodeIDFromPubKey(seed.NodeKey.Public())))
		}
		db := dbm.NewMemDB()
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		router, err := p2p.NewRouter(p2p.NopMetrics(), key, func() *types.NodeInfo { return &info }, db, options)
		require.NoError(t, err)
		require.NoError(t, router.Start(ctx))
		t.Cleanup(router.Stop)
		routers[node] = router
	}
	return routers, ctx
}

func TestRepairM_1192_PeerTopology(t *testing.T) {
	testnet := repair1192Manifest(t, e2e.Manifest{Nodes: map[string]*e2e.ManifestNode{
		"a": {PersistentPeers: []string{"b"}},
		"b": {PersistentPeers: []string{"a"}},
		"c": {PersistentPeers: []string{"d"}},
		"d": {PersistentPeers: []string{"c"}},
	}})
	routers, ctx := repair1192Routers(t, testnet, false)
	a, b, c := testnet.LookupNode("a"), testnet.LookupNode("b"), testnet.LookupNode("c")
	env := &core.Environment{Router: routers[a]}
	mux := http.NewServeMux()
	rpc.RegisterRPCFuncs(mux, core.NewRoutesMap(env, nil))
	server := httptest.NewServer(mux)
	defer server.Close()
	client, err := rpchttp.New(server.URL)
	require.NoError(t, err)
	var info *coretypes.ResultNetInfo
	require.Eventually(t, func() bool {
		info, err = client.NetInfo(ctx)
		return err == nil && len(info.PeerConnections) == 1
	}, 10*time.Second, 10*time.Millisecond)
	require.NoError(t, err)
	require.Less(t, len(info.PeerConnections), len(testnet.Nodes)-1, "fixture must not be fully meshed")
	require.NoError(t, checkPeerTopology(*a, info), "peer IDs must resolve to manifest names")

	// A manifest peer without a connection must not satisfy a persistent edge.
	a.PersistentPeers = append(a.PersistentPeers, c)
	// The captured real RPC result has only the configured a-b connection.
	require.ErrorContains(t, checkPeerTopology(*a, info), "missing persistent connection to c")
	a.PersistentPeers = []*e2e.Node{b}

	originalIP := b.IP
	b.IP = net.ParseIP("192.0.2.1")
	require.ErrorContains(t, checkPeerTopology(*a, info), "unexpected address")
	b.IP = originalIP
	originalNodes := testnet.Nodes
	testnet.Nodes = []*e2e.Node{a, c, testnet.LookupNode("d")}
	require.ErrorContains(t, checkPeerTopology(*a, info), "unknown or self peer")
	testnet.Nodes = originalNodes

	// Discovery requires a usable stateful peer, but not a lasting seed edge.
	a.PersistentPeers = nil
	a.Seeds = []*e2e.Node{c}
	c.Mode = e2e.ModeSeed
	require.NoError(t, checkPeerTopology(*a, info))
	b.Mode = e2e.ModeSeed
	require.ErrorContains(t, checkPeerTopology(*a, info), "no stateful connection after discovery")
	b.Mode = e2e.ModeValidator
}

func TestRepairM_1192_PeerAddressBookIsNotConnectivity(t *testing.T) {
	testnet := repair1192Manifest(t, e2e.Manifest{Nodes: map[string]*e2e.ManifestNode{"a": {}, "b": {}}})
	a, b := testnet.LookupNode("a"), testnet.LookupNode("b")
	a.IP, b.IP = net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.1")
	peer := p2p.Endpoint{AddrPort: tcp.TestReserveAddr()}.NodeAddress(types.NodeIDFromPubKey(b.NodeKey.Public()))
	key := p2p.NodeSecretKey(a.NodeKey)
	info := types.NodeInfo{NodeID: key.Public().NodeID(), Network: "repair1192"}
	db := dbm.NewMemDB()
	defer func() { require.NoError(t, db.Close()) }()
	router, err := p2p.NewRouter(p2p.NopMetrics(), key, func() *types.NodeInfo { return &info }, db,
		&p2p.RouterOptions{Endpoint: p2p.Endpoint{AddrPort: tcp.TestReserveAddr()}, Connection: conn.DefaultMConnConfig(), PersistentPeers: []p2p.NodeAddress{peer}})
	require.NoError(t, err)
	// The real router knows the configured address but has not connected.
	env := &core.Environment{Router: router}
	mux := http.NewServeMux()
	rpc.RegisterRPCFuncs(mux, core.NewRoutesMap(env, nil))
	server := httptest.NewServer(mux)
	defer server.Close()
	client, err := rpchttp.New(server.URL)
	require.NoError(t, err)
	result, err := client.NetInfo(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, result.NPeers)
	require.Empty(t, result.PeerConnections)
	require.ErrorContains(t, checkPeerTopology(*a, result), "missing persistent connection to b")
}

func TestRepairM_1192_PeerDiscovery(t *testing.T) {
	testnet := repair1192Manifest(t, e2e.Manifest{Nodes: map[string]*e2e.ManifestNode{
		"a":    {Seeds: []string{"seed"}},
		"b":    {Seeds: []string{"seed"}},
		"seed": {Mode: string(e2e.ModeSeed)},
	}})
	routers, ctx := repair1192Routers(t, testnet, true)
	a, b := testnet.LookupNode("a"), testnet.LookupNode("b")
	env := &core.Environment{Router: routers[a]}
	mux := http.NewServeMux()
	rpc.RegisterRPCFuncs(mux, core.NewRoutesMap(env, nil))
	server := httptest.NewServer(mux)
	defer server.Close()
	client, err := rpchttp.New(server.URL)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return routers[a].Connected(types.NodeIDFromPubKey(b.NodeKey.Public()))
	}, 10*time.Second, 10*time.Millisecond, "seed exchange must discover a stateful peer")
	info, err := client.NetInfo(ctx)
	require.NoError(t, err)
	require.NoError(t, checkPeerTopology(*a, info))
}

// Starts the production runtime that the builtin E2E runner entrypoint calls.
// Only addresses and directories are relocated to the local test fixture.
func repair1192Sequencer(t *testing.T) (*e2e.Testnet, *rpchttp.HTTP, context.Context) {
	t.Helper()
	testnet, err := e2e.LoadTestnet("../networks/sequencer.toml")
	require.NoError(t, err)
	testnet.Dir = t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)
	endpoints := make(map[*e2e.Node]string)
	for _, node := range testnet.Nodes {
		endpoints[node] = tcp.TestReserveAddr().String()
	}
	genesis := types.GenesisDoc{ChainID: testnet.Name, InitialHeight: testnet.InitialHeight,
		GenesisTime: time.Now(), ConsensusParams: types.DefaultConsensusParams()}
	updates := map[string]int64{}
	for _, node := range testnet.Nodes {
		power := testnet.Validators[node]
		key := node.PrivvalKey.Public()
		genesis.Validators = append(genesis.Validators, types.GenesisValidator{Name: node.Name, Address: key.Address(), PubKey: key, Power: power})
		updates[base64.StdEncoding.EncodeToString(key.Bytes())] = power
	}
	require.NoError(t, genesis.ValidateAndComplete())
	var firstClient *rpchttp.HTTP
	for _, node := range testnet.Nodes {
		home := filepath.Join(testnet.Dir, node.Name)
		config.EnsureRoot(home)
		cfg := config.DefaultConfig().SetRoot(home)
		cfg.Mode = config.ModeValidator
		cfg.Moniker = node.Name
		cfg.DBBackend = "memdb"
		cfg.P2P.ListenAddress = "tcp://" + endpoints[node]
		cfg.P2P.ExternalAddress = cfg.P2P.ListenAddress
		cfg.P2P.PersistentPeers = ""
		for _, peer := range node.PersistentPeers {
			if cfg.P2P.PersistentPeers != "" {
				cfg.P2P.PersistentPeers += ","
			}
			cfg.P2P.PersistentPeers += types.NodeIDFromPubKey(peer.NodeKey.Public()).AddressString(endpoints[peer])
		}
		cfg.RPC.ListenAddress = "tcp://" + tcp.TestReserveAddr().String()
		cfg.RPC.PprofListenAddress = ""
		cfg.Instrumentation.Prometheus = false
		require.NoError(t, config.WriteConfigFile(home, cfg))
		require.NoError(t, genesis.SaveAs(cfg.GenesisFile()))
		require.NoError(t, types.NodeKey(node.NodeKey).SaveAs(cfg.NodeKeyFile()))
		require.NoError(t, privval.NewFilePV(node.PrivvalKey, cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile()).Save())
		appConfig := noderuntime.Config{ChainID: testnet.Name, Protocol: "builtin", Mode: string(node.Mode),
			Dir: "data/app", PersistInterval: 1, LeaderPinHeight: testnet.LeaderPinHeight,
			ValidatorUpdates: map[string]map[string]int64{"0": updates}}
		for _, leader := range testnet.LeaderPin {
			appConfig.LeaderPin = append(appConfig.LeaderPin, hex.EncodeToString(leader.PrivvalKey.Public().Bytes()))
		}
		var encoded bytes.Buffer
		require.NoError(t, toml.NewEncoder(&encoded).Encode(appConfig))
		file := filepath.Join(home, "config", "app.toml")
		require.NoError(t, os.WriteFile(file, encoded.Bytes(), 0600))
		done := make(chan error, 1)
		go func() { done <- noderuntime.Run(ctx, file, home) }()
		t.Cleanup(func() {
			cancel()
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(10 * time.Second):
				t.Error("runtime did not stop")
			}
		})
		if firstClient == nil {
			firstClient, err = rpchttp.New(cfg.RPC.ListenAddress)
			require.NoError(t, err)
		}
	}
	return testnet, firstClient, ctx
}

func TestRepairM_1192_SequencerProposals(t *testing.T) {
	testnet, client, ctx := repair1192Sequencer(t)
	require.Equal(t, int64(3), testnet.LeaderPinHeight)
	require.Len(t, testnet.Validators, 3)
	require.Len(t, testnet.LeaderPin, 2)
	require.Eventually(t, func() bool {
		status, err := client.Status(ctx)
		return err == nil && status.SyncInfo.LatestBlockHeight >= 10
	}, 45*time.Second, 20*time.Millisecond, "real sequencer runtime must commit across activation")
	var blocks []*types.Block
	commits := map[int64]*types.Commit{}
	for height := testnet.InitialHeight; height <= 10; height++ {
		result, err := client.Block(ctx, &height)
		require.NoError(t, err)
		require.NotNil(t, result.Block)
		commit, err := client.Commit(ctx, &height)
		require.NoError(t, err)
		commits[height] = commit.Commit
		blocks = append(blocks, result.Block)
	}
	for _, node := range testnet.Nodes {
		expected, proposed, err := checkValidatorProposals(*node, blocks, commits)
		require.NoError(t, err, node.Name)
		if expected > 0 {
			require.Positive(t, proposed, node.Name)
		}
		if expected > 5 {
			require.GreaterOrEqual(t, proposed, 3)
		}
	}
	excluded := testnet.LookupNode("validator03").PrivvalKey.Public().Address()
	for _, block := range blocks {
		if block.Height >= testnet.LeaderPinHeight {
			require.NotEqual(t, excluded, block.ProposerAddress, "excluded validator proposed after activation")
		}
	}
	// Corrupt real producer output to establish that both activation sides are checked.
	for _, index := range []int{0, 2} {
		original := blocks[index]
		block := &types.Block{Header: original.Header, Data: original.Data, Evidence: original.Evidence, LastCommit: original.LastCommit}
		if index == 2 {
			block.ProposerAddress = excluded
		} else {
			block.ProposerAddress = types.GenNodeKey().PubKey().Address()
		}
		commit := *commits[block.Height]
		commit.BlockID.Hash = block.Hash()
		_, _, err := checkValidatorProposals(*testnet.Nodes[0], []*types.Block{block}, map[int64]*types.Commit{block.Height: &commit})
		require.ErrorContains(t, err, "unexpected proposer")
	}
}

func TestRepairM_1192_LeaderPolicyRounds(t *testing.T) {
	testnet, err := e2e.LoadTestnet("../networks/sequencer.toml")
	require.NoError(t, err)
	cfg := app.DefaultConfig(t.TempDir())
	cfg.LeaderPinHeight = testnet.LeaderPinHeight
	for _, leader := range testnet.LeaderPin {
		cfg.LeaderPin = append(cfg.LeaderPin, hex.EncodeToString(leader.PrivvalKey.Public().Bytes()))
	}
	application, err := app.NewApplication(cfg)
	require.NoError(t, err)
	var source types.LeaderSource = application
	pin, err := proposerPin(testnet)
	require.NoError(t, err)
	schedule := newValidatorSchedule(*testnet)
	for height := testnet.InitialHeight; height <= 8; height++ {
		require.NoError(t, schedule.Increment(height-schedule.height))
		activation, leaders, err := source.LeaderSchedule(height)
		require.NoError(t, err)
		productionPin, err := cstypes.LeaderPinFromSchedule(activation, leaders)
		require.NoError(t, err)
		for round := int32(0); round < 6; round++ {
			producer := cstypes.RoundState{HRS: cstypes.HRS{Height: height, Round: round}, Validators: schedule.Set, LeaderPin: productionPin}
			encoded, err := tmjson.Marshal(producer.NewRoundEvent())
			require.NoError(t, err)
			var event types.EventDataNewRound
			require.NoError(t, tmjson.Unmarshal(encoded, &event))
			expected := cstypes.RoundState{HRS: cstypes.HRS{Height: height, Round: round}, Validators: schedule.Set, LeaderPin: pin}
			require.Equal(t, expected.Leader().Address(), event.Proposer.Address)
			if height < activation {
				unpinned := cstypes.RoundState{HRS: cstypes.HRS{Height: height, Round: round}, Validators: schedule.Set}
				require.Equal(t, unpinned.Leader().Address(), event.Proposer.Address)
			} else {
				require.Equal(t, testnet.LeaderPin[int(round)%len(testnet.LeaderPin)].PrivvalKey.Public().Address(), event.Proposer.Address)
				require.NotEqual(t, testnet.LookupNode("validator03").PrivvalKey.Public().Address(), event.Proposer.Address)
			}
		}
	}
	// The zero-value policy follows the real election for unpinned manifests too.
	testnet.LeaderPin = nil
	testnet.LeaderPinHeight = 0
	disabled, err := proposerPin(testnet)
	require.NoError(t, err)
	require.False(t, disabled.Applies(100))
}
