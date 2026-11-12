// Package app wires the core/api gateway together (config, pgx pool, Redis, the
// realtime broker + WSS/SSE multiplexer, the rate-limited REST snapshot surface,
// caching, load-shedding, graceful shutdown) and owns its lifecycle.
//
// Topology (per the push-first design): the core/* services WRITE to Postgres +
// Redis; this gateway READS from them. A single broker subscriber fans Redis
// pub/sub out to WSS/SSE clients; REST serves only cached snapshot reads. Public
// ingress is rate-limited per IP/key with a global in-flight load-shed
// (invariants i11 + i12).
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
	"github.com/Sidiora-Technologies/KindleLaunch/core/notifications"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	sharedsecrets "github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/telemetry"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	sharedlog "github.com/Sidiora-Technologies/KindleLaunch/shared/log"
	sharedprocess "github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/cache"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/ratelimit"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/sse"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/ws"
)

// App is a fully-wired, ready-to-run core/api gateway.
type App struct {
	secretStop context.CancelFunc
	tracker    *sharedprocess.Tracker
	Router     http.Handler
	Broker     *broker.Broker
	cfg        config.Config
	logger     *slog.Logger

	pool             *pgxpool.Pool
	sqlDB            *sql.DB
	nats             *nats.Conn
	privateStream    *broker.PostgresStream
	devStreamStop    context.CancelFunc
	fanout           *broker.TopicFanout
	streamConsumers  []natsdriver.ConsumerConfig
	streamRelay      *streamRelay
	admission        *rest.Admission
	authority        *networkAuthority
	discoveryRuntime *notifications.HTTPRuntime
	redis            *goredis.Client
	subRdb           *goredis.Client
	wsHub            *ws.Hub
	sseHub           *sse.Hub
	cache            *cache.Cache
	limiter          *ratelimit.Limiter
}

// New builds every component. core/api owns no schema and runs no migrations: it
// reads the schemas the core/* services own (invariant i2).
func New(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration_missing")
	}
	copied := *cfg
	cfg = &copied
	secretCheck, secretStop, secretErr := cfg.ResolveProductionSecrets(ctx)
	if secretErr != nil {
		return nil, secretErr
	}
	secretOwned := false
	defer func() {
		if !secretOwned {
			secretStop()
		}
	}()
	tracker := sharedprocess.NewTracker()
	var metrics *telemetry.Metrics
	if cfg.NodeEnv == literalProduction {
		var e error
		metrics, e = telemetry.New("apid_"+cfg.APIRole, cfg.RuntimeMetricsEnvironment, cfg.RuntimeZone)
		if e != nil {
			return nil, e
		}
	}
	secretChecks := map[string]sharedhttp.Check{}
	if secretCheck != nil {
		secretChecks["secrets"] = secretCheck
	}

	runtimeRole, roleErr := cfg.RuntimeRole()
	if roleErr != nil {
		return nil, roleErr
	}
	pool, err := cfg.OpenRuntimePool(ctx, runtimeRole, shareddb.PoolOptions{
		MaxConns:         20,
		MaxConnIdleTime:  30 * time.Second,
		ConnectTimeout:   3 * time.Second,
		StatementTimeout: 10 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	rdb, err := sharedredis.NewClient(cfg.RedisURL)
	if err != nil {
		pool.Close()
		return nil, err
	}
	// A dedicated client for the broker's long-lived pub/sub subscription, kept
	// separate from the read/rate-limit client.
	subRdb, err := sharedredis.NewClient(cfg.RedisURL)
	if err != nil {
		_ = rdb.Close()
		pool.Close()
		return nil, err
	}

	resourcesOwned := false
	var ownedNATS *nats.Conn
	var ownedAuthority *networkAuthority
	defer func() {
		if !resourcesOwned {
			if ownedNATS != nil {
				ownedNATS.Close()
			}
			if ownedAuthority != nil {
				ownedAuthority.Close()
			}
			_ = subRdb.Close()
			_ = rdb.Close()
			pool.Close()
		}
	}()
	st := store.New(pool, rdb)
	responseCache := cache.New(4096)
	br := broker.New(broker.Options{Redis: subRdb, Logger: logger})

	var privateStream *broker.PostgresStream
	var nc *nats.Conn
	var streamConsumers []natsdriver.ConsumerConfig
	var authority *networkAuthority
	var relay *streamRelay
	var identity sharedauth.StreamAuthority
	var identityRequest requestIdentity
	fanout := broker.NewTopicFanout()
	admissionCapacity, admissionWaiting := cfg.AdmissionCapacity, cfg.AdmissionMaxWaiting
	if cfg.NodeEnv != literalProduction {
		if admissionCapacity == 0 {
			admissionCapacity = 1000
		}
		if admissionWaiting == 0 {
			admissionWaiting = 1000
		}
	}
	admission, err := rest.NewAdmission(admissionCapacity, admissionWaiting, 50*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if cfg.NodeEnv == literalProduction {
		identityConcrete, identityErr := runtimeIdentity(cfg)
		if identityErr != nil {
			return nil, identityErr
		}
		identity = identityConcrete
		identityRequest = identityConcrete
		privateStream, err = broker.NewPostgresStream(pool, cfg.StreamSchema, []byte(cfg.StreamSigningKey), identity)
		if err != nil {
			return nil, err
		}
		if cfg.APIRole == literalSockets {
			if err = config.ReadBoundDocument(cfg.StreamConsumerFile, cfg.StreamConsumerSHA256, &streamConsumers); err != nil {
				return nil, err
			}
			if len(streamConsumers) < 1 || len(streamConsumers) > 128 {
				return nil, errors.New("bounded stream topic assignments required")
			}
			seen := map[string]bool{}
			for _, consumer := range streamConsumers {
				if _, err = consumer.BrokerConsumerConfig(); err != nil {
					return nil, err
				}
				if _, err = broker.CanonicalTopics([]string{consumer.Subject}); err != nil {
					return nil, err
				}
				if seen[consumer.Subject] {
					return nil, errors.New("duplicate stream assignment")
				}
				seen[consumer.Subject] = true
			}
			nc, err = connectRuntimeNATS(cfg)
			if err != nil {
				return nil, err
			}
			ownedNATS = nc
			secretChecks["jetstream"] = func(context.Context) error {
				if !nc.IsConnected() {
					return broker.ErrStreamUnavailable
				}
				return nil
			}
		} else {
			authority, err = newNetworkAuthority(ctx, pool, cfg)
			if err != nil {
				return nil, err
			}
			ownedAuthority = authority
			secretChecks["network_authority"] = authority.Ready
			// The api_stream outbox relay: committed frames become JetStream
			// messages on their topic subjects for API_ROLE=sockets.
			relayDoc, relayErr := readStreamRelayDocument(cfg)
			if relayErr != nil {
				return nil, relayErr
			}
			if relayDoc == nil {
				secretChecks["stream_relay"] = func(context.Context) error {
					return errors.New("api_stream outbox relay not configured (STREAM_RELAY_CONFIG)")
				}
			} else {
				nc, err = connectRuntimeNATS(cfg)
				if err != nil {
					return nil, err
				}
				ownedNATS = nc
				if err = verifyRelayStreams(ctx, nc, relayDoc.Routes); err != nil {
					return nil, err
				}
				if relay, err = newStreamRelay(ctx, pool, nc, cfg.RuntimeInstanceID, *relayDoc); err != nil {
					return nil, err
				}
				secretChecks["stream_relay"] = relay.Ready
			}
		}
	}
	// Development v2 stream source (snapshots, WS and SSE over the
	// development projection); see dev_stream.go.
	var devStream *rest.DevelopmentStream
	var topicRegistry ws.TopicRegistry
	if cfg.NodeEnv != literalProduction {
		if devStream = newDevelopmentStream(ctx, cfg, pool, st, rdb, logger); devStream != nil {
			privateStream = devStream.Stream
			// Topics naming a pool absent from indexer.pools answer 404
			// NOT_FOUND / close 1008 instead of a retryable 503.
			topicRegistry = devStream
		}
	}
	allowedOrigins := map[string]bool{}
	for _, origin := range strings.Split(cfg.CORSAllowedOrigins, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" && origin != "*" {
			allowedOrigins[origin] = true
		}
	}
	trustedProxies := []string{}
	for _, cidr := range strings.Split(cfg.TrustedProxyCIDRs, ",") {
		if cidr = strings.TrimSpace(cidr); cidr != "" {
			trustedProxies = append(trustedProxies, cidr)
		}
	}
	wsHub := ws.NewHub(ws.Options{PrivateStream: privateStream, TopicRegistry: topicRegistry, Fanout: fanout, TrustedProxyCIDRs: trustedProxies, AllowedOrigins: strings.Split(cfg.CORSAllowedOrigins, ","),
		Broker:     br,
		Logger:     logger,
		SendBuffer: cfg.ClientSendBuffer,
		Flush:      cfg.CoalesceFlush(),
		MaxConns:   cfg.WSMaxConnections,
		MaxPerIP:   cfg.WSMaxPerIP,
	})
	sseHub := sse.NewHub(sse.Options{PrivateStream: privateStream, Fanout: fanout, TrustedProxyCIDRs: trustedProxies, AllowedOrigins: strings.Split(cfg.CORSAllowedOrigins, ","),
		Broker:     br,
		Logger:     logger,
		SendBuffer: cfg.ClientSendBuffer,
		Flush:      cfg.CoalesceFlush(),
		MaxConns:   cfg.SSEMaxConnections,
		MaxPerIP:   cfg.SSEMaxPerIP,
	})
	limiter := ratelimit.NewLimiter(cfg.MaxInFlightRequests)

	router := sharedhttp.NewRouter(sharedhttp.ServerOptions{
		CORSOrigins: cfg.CORSAllowedOrigins,
		Health: sharedhttp.HealthDeps{
			Custom: secretChecks,
			State:  tracker.Ready,
			DB:     func(c context.Context) error { return cfg.CheckRuntimeSchema(c, pool) },
			Redis:  func(c context.Context) error { return rdb.Ping(c).Err() },
		},
	})

	app := &App{secretStop: secretStop, tracker: tracker,
		Router: runtimeHandler(tracker, router, metrics),
		Broker: br,
		cfg:    *cfg,
		logger: logger,
		pool:   pool,
		nats:   nc, privateStream: privateStream, fanout: fanout, streamConsumers: streamConsumers, streamRelay: relay, admission: admission, authority: authority,
		redis:   rdb,
		subRdb:  subRdb,
		wsHub:   wsHub,
		sseHub:  sseHub,
		cache:   responseCache,
		limiter: limiter,
	}

	// Realtime group: connection-capped + a light per-IP connect rate limit, but
	// NOT the in-flight limiter (streams are long-lived and would pin slots).
	if cfg.APIRole != literalRest {
		router.Group(func(gr chi.Router) {
			gr.Use(sharedhttp.RateLimit(sharedhttp.RateLimitOptions{
				Max:    cfg.RateLimitMax,
				Window: cfg.RateLimitWindow(),
				Redis:  rdb,
			}))
			if cfg.NodeEnv == literalProduction {
				gr.Get("/v2/stream", wsHub.HandleV2)
				gr.Get("/v2/stream/sse", sseHub.HandleV2)
			} else {
				wsHub.Register(gr)
				sseHub.Register(gr)
			}
		})
	}

	// The production accounting read model backs the frozen v2 reads
	// registered below (RegisterAccountingV2WithAdmission, RegisterProductionReads).
	var accountingModel *store.AccountingReadModel
	if cfg.NodeEnv == literalProduction && cfg.APIRole == literalRest {
		var readConfig store.AccountingReadConfig
		if err := config.ReadBoundDocument(cfg.AccountingReadFile, cfg.AccountingReadSHA256, &readConfig); err != nil {
			app.Close()
			return nil, err
		}
		model, err := store.NewAccountingReadModel(pool, readConfig)
		if err != nil {
			app.Close()
			return nil, err
		}
		accountingModel = model
	}

	if cfg.NodeEnv == literalProduction && cfg.APIRole == literalRest {
		app.sqlDB = stdlib.OpenDBFromPool(pool)
	}
	if err := app.registerREST(ctx, router, st, accountingModel, identityRequest, secretChecks, allowedOrigins, trustedProxies); err != nil {
		app.Close()
		return nil, err
	}
	if metrics != nil {
		router.Handle("/metrics", metrics)
	}
	if devStream != nil && cfg.APIRole != literalRest {
		runCtx, stop := context.WithCancel(ctx)
		app.devStreamStop = stop
		go devStream.Run(runCtx)
	}
	if err := tracker.SetReady(); err != nil {
		return nil, err
	}
	resourcesOwned = true
	secretOwned = true
	return app, nil
}

// registerREST installs every snapshot registration under the same client quota
// and in-flight limit, including handlers that have their own workload admission.
func (a *App) registerREST(ctx context.Context, router chi.Router, st *store.Store, model *store.AccountingReadModel, identity requestIdentity, checks map[string]sharedhttp.Check, allowedOrigins map[string]bool, trustedProxies []string) error {
	if a.cfg.APIRole == literalSockets {
		return nil
	}
	var registrationErr error
	router.Group(func(gr chi.Router) {
		gr.Use(a.limiter.Middleware)
		gr.Use(sharedhttp.RateLimit(sharedhttp.RateLimitOptions{
			Max:     a.cfg.RateLimitMax,
			Window:  a.cfg.RateLimitWindow(),
			Redis:   a.redis,
			KeyFunc: ratelimit.ClientKey,
		}))
		gr.Get("/status", a.statusHandler)
		if a.cfg.NodeEnv != literalProduction {
			registrationErr = registerDevelopmentReads(gr, developmentReadDeps{cfg: &a.cfg, store: st, cache: a.cache, admission: a.admission})
			return
		}
		if a.cfg.APIRole != literalRest {
			return
		}
		if registrationErr = rest.RegisterAccountingV2WithAdmission(gr, model, []byte(a.cfg.StreamSigningKey), nil, a.admission); registrationErr != nil {
			return
		}
		// Production snapshots include launched chains, their revenue and pool home chains.
		if registrationErr = rest.RegisterProductionReads(gr, rest.ProductionReadOptions{Model: model, Stream: a.privateStream, Store: st, StatsOrigin: a.cfg.StatsURL, RegisterSnapshots: true, Admission: a.admission, AllowedOrigins: allowedOrigins, TrustedProxyCIDRs: trustedProxies}); registrationErr != nil {
			return
		}
		a.authority.Register(gr, a.admission)
		a.authority.RegisterNetworkReads(gr, a.admission)
		if registrationErr = a.registerProductionOperations(ctx, gr, identity, a.admission, checks); registrationErr != nil {
			return
		}
		registrationErr = rest.RegisterV2SearchProxy(gr, a.cfg.StatsURL)
	})
	return registrationErr
}

// statusHandler reports live gateway gauges (subscribers, connections, cache,
// in-flight, pool) for ops dashboards.
func (a *App) statusHandler(w http.ResponseWriter, _ *http.Request) {
	delivered, dropped := a.Broker.Stats()
	hits, misses := a.cache.Stats()
	sharedhttp.WriteJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"subscribers":     a.Broker.Subscribers(),
		"deliveredFrames": delivered,
		"droppedFrames":   dropped,
		"wsConnections":   a.wsHub.Connections(),
		"sseConnections":  a.sseHub.Connections(),
		"cacheHits":       hits,
		"cacheMisses":     misses,
		"inFlight":        a.limiter.InFlight(),
		"dbPools":         shareddb.PoolMetrics(),
		"admission":       a.admission.Stats(),
		"publicCache":     rest.MarketCacheMetrics(),
	})
}

// Close releases all resources.
func (a *App) Close() {
	if a.devStreamStop != nil {
		a.devStreamStop()
	}
	if a.secretStop != nil {
		a.secretStop()
	}
	if a.nats != nil {
		a.nats.Close()
	}
	if a.authority != nil {
		a.authority.Close()
	}
	if a.sqlDB != nil {
		_ = a.sqlDB.Close()
	}
	if a.pool != nil {
		a.pool.Close()
	}
	if a.redis != nil {
		_ = a.redis.Close()
	}
	if a.subRdb != nil {
		_ = a.subRdb.Close()
	}
}

// Run loads config, builds the App, starts the broker fan-out + HTTP server, and
// blocks until shutdown.
func Run(parent context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := sharedlog.New("core-api", cfg.LogLevel)
	logger.Info("starting core-api gateway")

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	app, err := New(ctx, &cfg, logger)
	if err != nil {
		return err
	}
	defer app.Close()

	// Broker fan-out loop (single Redis subscriber for all clients).
	if app.discoveryRuntime != nil {
		go func() {
			if e := app.discoveryRuntime.Run(ctx); e != nil && ctx.Err() == nil {
				logger.Error("discovery runtime stopped", slog.Any("err", e))
			}
		}()
	}
	if app.streamRelay != nil {
		go app.streamRelay.Run(ctx, logger)
	}
	if cfg.NodeEnv == literalProduction && cfg.APIRole == literalSockets {
		for _, consumer := range app.streamConsumers {
			consumer := consumer
			go func() {
				if e := app.privateStream.RunCommitted(ctx, app.nats, consumer, app.fanout); e != nil && ctx.Err() == nil {
					logger.Error("committed stream stopped", slog.Any("err", e))
					cancel()
				}
			}()
		}
	} else if cfg.APIRole != literalRest {
		go func() {
			if err := app.Broker.Run(ctx); err != nil {
				logger.Error("broker stopped", slog.Any("err", err))
				cancel()
			}
		}()
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.BindAddress, cfg.Port),
		Handler:           app.Router,
		ConnState:         app.tracker.ConnState,
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: SSE/WSS are long-lived; per-write deadlines bound
		// slow clients instead (internal/sse uses http.ResponseController).
	}
	serveErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", slog.Int("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			cancel()
		}
	}()

	shutErr := sharedprocess.Run(ctx, sharedprocess.Options{Tracker: app.tracker,
		Logger: logger,
		OnShutdown: func(sctx context.Context) error {
			app.wsHub.Drain()
			app.sseHub.Drain()
			shutdown := make(chan error, 1)
			go func() { shutdown <- srv.Shutdown(sctx) }()
			if app.wsHub.Connections() > 0 || app.sseHub.Connections() > 0 {
				timer := time.NewTimer(30 * time.Second)
				select {
				case <-timer.C:
				case <-sctx.Done():
				}
				timer.Stop()
			}
			cancel()
			return <-shutdown
		},
	})

	cancel()

	select {
	case err := <-serveErr:
		return err
	default:
		return shutErr
	}
}

func runtimeHandler(tracker *sharedprocess.Tracker, next http.Handler, metrics *telemetry.Metrics) http.Handler {
	observed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if metrics == nil || r.Method != http.MethodGet || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		started := time.Now()
		writer := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(writer, r)
		status := writer.Status()
		if status == 0 {
			status = http.StatusOK
		}
		exclusion := ""
		if r.Context().Err() != nil {
			exclusion = "client_cancel"
		}
		if status == 400 || status == 422 {
			exclusion = "invalid_request"
		}
		_ = metrics.RecordRead("other", status, time.Since(started), exclusion)
	})
	tracked := tracker.Middleware(observed)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/health/live" || r.URL.Path == "/health/ready" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		tracked.ServeHTTP(w, r)
	})
}

type discoveryRuntimeDocument struct {
	QuoteTokenAddress string                       `json:"quoteTokenAddress"`
	D11Base           string                       `json:"d11Base"`
	D11TokenRef       string                       `json:"d11TokenRef"`
	D12Base           string                       `json:"d12Base"`
	D12TokenRef       string                       `json:"d12TokenRef"`
	CursorKeyRef      string                       `json:"cursorKeyRef"`
	InternalTokenRef  string                       `json:"internalTokenRef"`
	DeletePeerDNS     string                       `json:"deletePeerDNS"`
	ChatPeerDNS       string                       `json:"chatPeerDNS"`
	Push              notifications.RuntimeOptions `json:"push"`
}

func (a *App) registerProductionOperations(ctx context.Context, router chi.Router, identity requestIdentity, admission *rest.Admission, checks map[string]sharedhttp.Check) error {
	if err := a.registerDiscovery(ctx, router, identity, admission, checks); err != nil {
		return err
	}
	registerUnservedOperations(router, admission, productionUnservedOperations)
	return nil
}

func (a *App) registerDiscovery(ctx context.Context, router chi.Router, identity requestIdentity, admission *rest.Admission, checks map[string]sharedhttp.Check) error {
	principal := func(r *http.Request) (notifications.Principal, error) {
		if identity == nil {
			return notifications.Principal{}, sharedauth.ErrScopeDenied
		}
		p, err := identity.AuthenticateRequest(r.Context(), r)
		return notifications.Principal{AccountID: p.AccountID, Wallet: p.Wallet}, err
	}
	authenticate := func(r *http.Request) (string, error) {
		if identity == nil {
			return "", sharedauth.ErrScopeDenied
		}
		p, err := identity.AuthenticateRequest(r.Context(), r)
		return p.AccountID, err
	}
	wallet := func(r *http.Request) (string, error) {
		if identity == nil {
			return "", sharedauth.ErrScopeDenied
		}
		p, err := identity.AuthenticateRequest(r.Context(), r)
		return p.Wallet, err
	}
	if a.cfg.DiscoveryRuntimeFile != "" || a.cfg.DiscoveryRuntimeDocument != "" {
		var document discoveryRuntimeDocument
		if err := config.ReadBoundDocument(a.cfg.DiscoveryRuntimeFile, a.cfg.DiscoveryRuntimeSHA256, &document); err != nil {
			return err
		}
		concrete, ok := identity.(sharedauth.MTLSIdentityAuthority)
		if !ok {
			return errors.New("discovery identity transport unavailable")
		}
		var err error
		runtimeConfig := notifications.RuntimeConfig{Principal: principal, Secrets: sharedsecrets.Options{Provider: sharedsecrets.ProviderType(a.cfg.SecretsProvider), CacheTTL: 0}, Client: concrete.Client, D11Base: document.D11Base, D11TokenRef: document.D11TokenRef, D12Base: document.D12Base, D12TokenRef: document.D12TokenRef, CursorKeyRef: document.CursorKeyRef, InternalTokenRef: document.InternalTokenRef, DeletePeerDNS: document.DeletePeerDNS, ChatPeerDNS: document.ChatPeerDNS, Push: document.Push}
		runtimeConfig.Pools, runtimeConfig.Intervals, err = notifications.NewRuntimeAuthorities(ctx, a.sqlDB, runtimeConfig, document.QuoteTokenAddress)
		if err != nil {
			return err
		}
		runtime, err := notifications.NewHTTPRuntime(ctx, a.sqlDB, authenticate, runtimeConfig)
		if err == nil {
			a.discoveryRuntime = runtime
			handler := rest.ResponseMetadata(admission.Middleware(rest.AdmissionAggregate)(runtime.Handler()))
			router.Handle("/v2/discovery/*", handler)
			router.Handle("/v2/discovery/notifications", handler)
			router.Handle("/v2/squads", handler)
			router.Handle("/v2/squads/*", handler)
			router.Handle("/v2/me/notifications", handler)
			router.Handle("/v2/me/notifications/*", handler)
			router.Handle("/v2/me/alerts", handler)
			router.Handle("/v2/me/alerts/*", handler)
			checks["discovery_notifications"] = runtime.Ready
			return nil
		}
		return err
	}
	ds := discovery.Store{DB: a.sqlDB}
	ns := notifications.Store{DB: a.sqlDB}
	discoveryHTTP := discovery.HTTP{Store: ds, Callouts: discovery.CalloutService{Store: ds}, Authenticate: authenticate, Wallet: wallet}
	notificationsHTTP := notifications.HTTP{Store: ns, Discovery: ds, Authenticate: authenticate, Principal: principal}
	router.Handle("/v2/discovery/notifications", rest.ResponseMetadata(admission.Middleware(rest.AdmissionAggregate)(notificationsHTTP.Handler())))
	router.Handle("/v2/discovery/notifications/*", rest.ResponseMetadata(admission.Middleware(rest.AdmissionAggregate)(notificationsHTTP.Handler())))
	router.Handle("/v2/me/notifications", rest.ResponseMetadata(admission.Middleware(rest.AdmissionAggregate)(notificationsHTTP.Handler())))
	router.Handle("/v2/me/notifications/*", rest.ResponseMetadata(admission.Middleware(rest.AdmissionAggregate)(notificationsHTTP.Handler())))
	router.Handle("/v2/squads", rest.ResponseMetadata(admission.Middleware(rest.AdmissionAggregate)(discoveryHTTP.Handler())))
	router.Handle("/v2/squads/*", rest.ResponseMetadata(admission.Middleware(rest.AdmissionAggregate)(discoveryHTTP.Handler())))
	router.Handle("/v2/me/alerts", rest.ResponseMetadata(admission.Middleware(rest.AdmissionAggregate)(discoveryHTTP.Handler())))
	router.Handle("/v2/me/alerts/*", rest.ResponseMetadata(admission.Middleware(rest.AdmissionAggregate)(discoveryHTTP.Handler())))
	router.Handle("/v2/discovery/*", rest.ResponseMetadata(admission.Middleware(rest.AdmissionAggregate)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v2/discovery/feed") || r.URL.Path == "/v2/discovery/leaderboards" || strings.HasPrefix(r.URL.Path, "/v2/discovery/callouts") {
			// The discovery runtime is not configured in this deployment.
			rest.WriteV2NotConfiguredError(w, r)
			return
		}
		discoveryHTTP.Handler().ServeHTTP(w, r)
	}))))
	checks["discovery_sources"] = func(context.Context) error {
		return errors.New("discovery source visibility and interval authorities unavailable")
	}
	checks["notification_delivery"] = func(context.Context) error {
		return errors.New("push target authority and provider credentials unavailable")
	}
	return nil
}
