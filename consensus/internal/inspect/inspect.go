package inspect

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventbus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/inspect/rpc"
	rpccore "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	tmstrings "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/strings"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/paxeer-network/paxlog"

	"golang.org/x/sync/errgroup"
)

var logger = paxlog.NewLogger("tendermint", "internal", "inspect")

// Inspector manages an RPC service that exports methods to debug a failed node.
// After a node shuts down due to a consensus failure, it will no longer start
// up its state cannot easily be inspected. An Inspector value provides a similar interface
// to the node, using the underlying Tendermint data stores, without bringing up
// any other components. A caller can query the Inspector service to inspect the
// persisted state and debug the failure.
type Inspector struct {
	routes rpccore.RoutesMap

	config *config.RPCConfig

	indexerService *indexer.Service
	eventBus       *eventbus.EventBus
	ownedDBs       []*inspectorDB
}

// inspectorDB lets both the indexer and constructor cleanup release an owned DB.
type inspectorDB struct {
	dbm.DB
	once sync.Once
	err  error
}

func (db *inspectorDB) Close() error {
	db.once.Do(func() { db.err = db.DB.Close() })
	return db.err
}

// New returns an Inspector that serves RPC on the specified BlockStore and StateStore.
// The Inspector type does not modify the state or block stores.
// The sinks are used to enable block and transaction querying via the RPC server.
// The caller is responsible for starting and stopping the Inspector service and
// closing the block and state stores. The indexer closes the sinks on shutdown.
func New(cfg *config.RPCConfig, bs state.BlockStore, ss state.Store, es []indexer.EventSink) *Inspector {
	eb := eventbus.NewDefault()

	return &Inspector{
		routes:   rpc.Routes(*cfg, ss, bs, es),
		config:   cfg,
		eventBus: eb,
		indexerService: indexer.NewService(indexer.ServiceArgs{
			Sinks:    es,
			EventBus: eb,
		}),
	}
}

// NewFromConfig constructs an Inspector using the values defined in the passed in config.
// It owns the opened databases and closes them on construction failure or when Run returns.
func NewFromConfig(cfg *config.Config) (_ *Inspector, err error) {
	var ownedDBs []*inspectorDB
	provider := func(ctx *config.DBContext) (dbm.DB, error) {
		db, err := config.DefaultDBProvider(ctx)
		if err != nil {
			return nil, err
		}
		owned := &inspectorDB{DB: db}
		ownedDBs = append(ownedDBs, owned)
		return owned, nil
	}
	defer func() {
		if err != nil {
			for _, db := range ownedDBs {
				if closeErr := db.Close(); closeErr != nil {
					err = errors.Join(err, closeErr)
				}
			}
		}
	}()

	bsDB, err := provider(&config.DBContext{ID: "blockstore", Config: cfg})
	if err != nil {
		return nil, err
	}
	bs := store.NewBlockStore(bsDB)
	sDB, err := provider(&config.DBContext{ID: "state", Config: cfg})
	if err != nil {
		return nil, err
	}
	genDoc, err := types.GenesisDocFromFile(cfg.GenesisFile())
	if err != nil {
		return nil, err
	}
	sinks, err := sink.EventSinksFromConfig(cfg, provider, genDoc.ChainID)
	if err != nil {
		return nil, err
	}
	ss := state.NewStore(sDB)
	ins := New(cfg.RPC, bs, ss, sinks)
	ins.ownedDBs = ownedDBs
	return ins, nil
}

// Run starts the Inspector servers and blocks until the servers shut down. The passed
// in context is used to control the lifecycle of the servers. Startup errors are
// returned after stopping started services, without canceling the caller's context.
func (ins *Inspector) Run(ctx context.Context) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		ins.indexerService.Wait()
		ins.eventBus.Wait()
		for _, db := range ins.ownedDBs {
			if closeErr := db.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}
	}()

	err = ins.eventBus.Start(ctx)
	if err != nil {
		return fmt.Errorf("error starting event bus: %w", err)
	}

	err = ins.indexerService.Start(ctx)
	if err != nil {
		return fmt.Errorf("error starting indexer service: %w", err)
	}

	return startRPCServers(ctx, ins.config, ins.routes)
}

func startRPCServers(ctx context.Context, cfg *config.RPCConfig, routes rpccore.RoutesMap) error {
	g, tctx := errgroup.WithContext(ctx)
	listenAddrs := tmstrings.SplitAndTrimEmpty(cfg.ListenAddress, ",", " ")
	rh := rpc.Handler(cfg, routes)
	for _, listenerAddr := range listenAddrs {
		server := rpc.Server{
			Config:  cfg,
			Handler: rh,
			Addr:    listenerAddr,
		}
		if cfg.IsTLSEnabled() {
			keyFile := cfg.KeyFile()
			certFile := cfg.CertFile()
			listenerAddr := listenerAddr
			g.Go(func() error {
				logger.Info("RPC HTTPS server starting", "address", listenerAddr,
					"certfile", certFile, "keyfile", keyFile)
				err := server.ListenAndServeTLS(tctx, certFile, keyFile)
				if !errors.Is(err, net.ErrClosed) && !errors.Is(err, http.ErrServerClosed) {
					return err
				}
				logger.Info("RPC HTTPS server stopped", "address", listenerAddr)
				return nil
			})
		} else {
			listenerAddr := listenerAddr
			g.Go(func() error {
				logger.Info("RPC HTTP server starting", "address", listenerAddr)
				err := server.ListenAndServe(tctx)
				if !errors.Is(err, net.ErrClosed) && !errors.Is(err, http.ErrServerClosed) {
					return err
				}
				logger.Info("RPC HTTP server stopped", "address", listenerAddr)
				return nil
			})
		}
	}
	return g.Wait()
}
