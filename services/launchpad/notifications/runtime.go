package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
)

type Credentials struct {
	mu        sync.RWMutex
	values    map[string]string
	provider  secrets.Provider
	refreshed time.Time
	ready     bool
	wait      runtimeWait
}

func NewCredentials(ctx context.Context, options secrets.Options, keys ...string) (*Credentials, error) {
	options.CacheTTL = 0
	p, e := secrets.NewChecked(options)
	if e != nil {
		return nil, e
	}
	c := &Credentials{provider: p, values: map[string]string{}}
	for _, key := range keys {
		if key == "" {
			return nil, ErrInvalid
		}
		value, e := p.GetRequired(ctx, key)
		if e != nil {
			return nil, fmt.Errorf("%w: load credential %s: %w", ErrUnavailable, key, e)
		}
		c.values[key] = value
	}
	c.ready = true
	c.refreshed = time.Now()
	return c, nil
}
func (c *Credentials) Get(key string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.ready || time.Since(c.refreshed) > 60*time.Second {
		return "", ErrUnavailable
	}
	value, ok := c.values[key]
	if !ok {
		return "", ErrUnavailable
	}
	return value, nil
}
func (c *Credentials) Refresh(ctx context.Context) error {
	c.mu.RLock()
	keys := make([]string, 0, len(c.values))
	for k := range c.values {
		keys = append(keys, k)
	}
	c.mu.RUnlock()
	next := map[string]string{}
	for _, k := range keys {
		value, e := c.provider.GetRequired(ctx, k)
		if e != nil || value == "" {
			if e == nil {
				e = errors.New("credential value is empty")
			}
			return fmt.Errorf("%w: refresh credential %s: %w", ErrUnavailable, k, e)
		}
		next[k] = value
	}
	c.mu.Lock()
	c.values = next
	c.refreshed = time.Now()
	c.ready = true
	c.mu.Unlock()
	return nil
}
func (c *Credentials) Run(ctx context.Context) error {
	wait := c.wait
	if wait == nil {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		wait = func(ctx context.Context, _ time.Duration) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				return nil
			}
		}
	}
	for {
		if err := wait(ctx, 30*time.Second); err != nil {
			return err
		}
		refresh, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := c.Refresh(refresh)
		cancel()
		if err != nil {
			log.Printf("notification runtime job credentials failed: %v", err)
		}
	}
}

func (c *Credentials) Ready(ctx context.Context) error {
	_, _ = ctx.Deadline()
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.ready || time.Since(c.refreshed) > 60*time.Second {
		return ErrUnavailable
	}
	return nil
}

type RuntimeOptions struct {
	Secrets                                                                                                     secrets.Options
	APNSKey, APNSKeyID, APNSTeamID, APNSTopic, FCMServiceAccount, VAPIDKey, VAPIDSubject, Owner, DeepLinkOrigin string
	APNSSandbox                                                                                                 bool
	EnableAPNS, EnableFCM, EnableWebPush                                                                        bool
}
type PushRuntime struct {
	runtimeJobs
	Credentials *Credentials
	Worker      Worker
}

func NewPushRuntime(ctx context.Context, db *sql.DB, authority TargetAuthority, o RuntimeOptions) (*PushRuntime, error) {
	if db == nil || authority == nil || o.Owner == "" {
		return nil, ErrInvalid
	}
	u, e := url.Parse(o.DeepLinkOrigin)
	if e != nil || u.Scheme != httpsScheme || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, ErrInvalid
	}
	if !o.EnableAPNS && (o.APNSKey != "" || o.APNSKeyID != "" || o.APNSTeamID != "" || o.APNSTopic != "") {
		return nil, ErrInvalid
	}
	if !o.EnableFCM && o.FCMServiceAccount != "" {
		return nil, ErrInvalid
	}
	if !o.EnableWebPush && (o.VAPIDKey != "" || o.VAPIDSubject != "") {
		return nil, ErrInvalid
	}
	refs := []string{}
	if o.EnableAPNS {
		refs = append(refs, o.APNSKey, o.APNSKeyID, o.APNSTeamID, o.APNSTopic)
	}
	if o.EnableFCM {
		refs = append(refs, o.FCMServiceAccount)
	}
	if o.EnableWebPush {
		refs = append(refs, o.VAPIDKey, o.VAPIDSubject)
	}
	credentials, e := NewCredentials(ctx, o.Secrets, refs...)
	if e != nil {
		return nil, e
	}
	adapters := map[string]ChannelAdapter{}
	if o.EnableAPNS {
		a, e := NewAPNS(credentials, authority, o.APNSKey, o.APNSKeyID, o.APNSTeamID, o.APNSTopic, o.APNSSandbox)
		if e != nil {
			return nil, e
		}
		adapters[a.Provider()] = a
	}
	if o.EnableFCM {
		a, e := NewFCM(credentials, authority, o.FCMServiceAccount)
		if e != nil {
			return nil, e
		}
		adapters[a.Provider()] = a
	}
	if o.EnableWebPush {
		a, e := NewWebPush(credentials, authority, o.VAPIDKey, o.VAPIDSubject)
		if e != nil {
			return nil, e
		}
		adapters[a.Provider()] = a
	}
	origin := (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
	return &PushRuntime{Credentials: credentials, Worker: Worker{Store: Store{DB: db}, Authority: authority, Adapters: adapters, Owner: o.Owner, DeepLinkOrigin: origin}}, nil
}
func (r *PushRuntime) Run(ctx context.Context) error {
	if r == nil || r.Credentials == nil {
		return ErrInvalid
	}
	return r.run(ctx, []runtimeJob{
		{name: "push_credentials", interval: 30 * time.Second, timeout: 5 * time.Second, pass: r.Credentials.Refresh},
		{name: "push_delivery", interval: time.Second, timeout: 30 * time.Second, pass: func(ctx context.Context) error { return r.Worker.Once(ctx, 20) }},
	})
}

type RuntimeConfig struct {
	Principal                                                                                                                    AuthenticatePrincipal
	Secrets                                                                                                                      secrets.Options
	Client                                                                                                                       *http.Client
	D11Base, D11TokenRef, D12Base, D12TokenRef, PnLBase, PnLTokenRef, CursorKeyRef, InternalTokenRef, DeletePeerDNS, ChatPeerDNS string
	Push                                                                                                                         RuntimeOptions
	Pools                                                                                                                        discovery.SourceAuthority
	Intervals                                                                                                                    discovery.IntervalAuthority

	Logger *slog.Logger
}
type HTTPRuntime struct {
	runtimeJobs
	DB            *sql.DB
	Credentials   *Credentials
	Accounts      *AccountClient
	Push          *PushRuntime
	Discovery     discovery.HTTP
	Notifications HTTP
	Events        discovery.Runtime
	Config        RuntimeConfig
}

func NewHTTPRuntime(ctx context.Context, db *sql.DB, auth func(*http.Request) (string, error), config RuntimeConfig) (*HTTPRuntime, error) {
	if db == nil || auth == nil || config.Principal == nil || config.Pools == nil || config.Intervals == nil || config.Client == nil || config.D11Base == "" || config.D12Base == "" || config.D11TokenRef == "" || config.D12TokenRef == "" || config.CursorKeyRef == "" || config.InternalTokenRef == "" || config.DeletePeerDNS == "" || config.ChatPeerDNS == "" {
		return nil, ErrInvalid
	}
	if config.PnLBase != "" || config.PnLTokenRef != "" {
		return nil, ErrInvalid
	}
	refs := []string{config.D11TokenRef, config.D12TokenRef, config.CursorKeyRef, config.InternalTokenRef}
	credentials, e := NewCredentials(ctx, config.Secrets, refs...)
	if e != nil {
		return nil, e
	}
	key, e := credentials.Get(config.CursorKeyRef)
	if e != nil || len(key) < 32 {
		return nil, ErrInvalid
	}
	accounts, e := NewAccountClient(config.D11Base, config.Client, credentials, config.D11TokenRef)
	if e != nil {
		return nil, e
	}
	socialRPC, e := newAccountRPC(config.D12Base, config.Client, credentials, config.D12TokenRef)
	if e != nil {
		return nil, e
	}
	if pools, ok := config.Pools.(CanonicalPools); ok {
		pools.Accounts = accounts
		config.Pools = pools
	}
	if intervals, ok := config.Intervals.(discovery.SQLIntervalAuthority); ok {
		if _, owned := intervals.Attribution.(*AccountClient); owned {
			intervals.Attribution = accounts
			config.Intervals = intervals
		}
	}
	ds := discovery.Store{DB: db}
	ns := Store{DB: db}
	social := socialAuthority{rpc: socialRPC, store: ds}
	external := sourceAuthority{pools: config.Pools, accounts: accounts, social: social}
	callouts := discovery.CalloutService{Store: ds, Authority: external}
	owned := discovery.OwnedSources{External: external, Callouts: callouts}
	intervals := config.Intervals
	policy := InternalPolicy{Credentials: credentials, TokenRef: config.InternalTokenRef, DeletePeerDNS: config.DeletePeerDNS, ChatPeerDNS: config.ChatPeerDNS}
	r := &HTTPRuntime{runtimeJobs: runtimeJobs{logger: config.Logger}, DB: db, Credentials: credentials, Accounts: accounts, Config: config}
	r.Discovery = discovery.HTTP{Store: ds, Callouts: callouts, Authenticate: discovery.Authenticate(auth), Wallet: func(req *http.Request) (string, error) { p, err := config.Principal(req); return p.Wallet, err }, Intervals: intervals, Feed: discovery.Feed{Store: ds, Sources: owned, Preferences: accounts, Graph: social, Signer: discovery.CursorSigner{Key: []byte(key)}}}
	r.Notifications = HTTP{Store: ns, Discovery: ds, Authenticate: Authenticate(auth), Principal: config.Principal, Recipient: RecipientConsumer{Store: ns, Authority: accounts}, InternalAuthorize: policy.Authorize}
	r.Events = discovery.Runtime{Store: ds, Publisher: eventPublisher{triggers: TriggerConsumer{Store: ns, Authority: accounts}, social: social}}
	if config.Push.EnableAPNS || config.Push.EnableFCM || config.Push.EnableWebPush {
		config.Push.Secrets = config.Secrets
		r.Push, e = NewPushRuntime(ctx, db, accounts, config.Push)
		if e != nil {
			return nil, e
		}
	}
	return r, nil
}
func (r *HTTPRuntime) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/v2/discovery/notifications", r.Notifications.Handler())
	mux.Handle("/v2/discovery/notifications/", r.Notifications.Handler())
	mux.Handle("/v2/me/notifications", r.Notifications.Handler())
	mux.Handle("/v2/me/notifications/", r.Notifications.Handler())
	discoveryHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if r.Config.Pools == nil && (strings.HasPrefix(req.URL.Path, "/v2/discovery/feed") || strings.HasPrefix(req.URL.Path, "/v2/discovery/callouts")) {
			respond(w, nil, ErrUnavailable)
			return
		}
		key, e := r.Credentials.Get(r.Config.CursorKeyRef)
		if e != nil || len(key) < 32 {
			respond(w, nil, ErrUnavailable)
			return
		}
		h := r.Discovery
		h.Feed.Signer = discovery.CursorSigner{Key: []byte(key)}
		h.Handler().ServeHTTP(w, req)
	})
	for _, path := range []string{"/v2/discovery/", "/v2/squads", "/v2/squads/", "/v2/me/alerts", "/v2/me/alerts/"} {
		mux.Handle(path, discoveryHandler)
	}
	return mux
}

// InternalHandler must be served only on a private listener that requires and
// verifies a client certificate, never on a public router. The runtime's
// InternalPolicy authorizes both the account removal and chat intent routes.
func (r *HTTPRuntime) InternalHandler() http.Handler { return r.Notifications.InternalHandler() }
func (r *HTTPRuntime) Ready(ctx context.Context) error {
	if r == nil || r.DB == nil {
		return ErrUnavailable
	}
	if r.ready() != nil || r.Credentials == nil || r.Config.Pools == nil || r.Discovery.Intervals == nil {
		return ErrUnavailable
	}
	if e := r.Credentials.Ready(ctx); e != nil {
		return e
	}
	if r.Push != nil {
		if e := r.Push.Credentials.Ready(ctx); e != nil {
			return e
		}
	}
	var exists bool
	e := r.DB.QueryRowContext(ctx, `SELECT to_regclass('narrative_discovery.sources') IS NOT NULL AND to_regclass('discovery_notifications.inbox') IS NOT NULL`).Scan(&exists)
	if e != nil {
		return e
	}
	if !exists {
		return ErrUnavailable
	}
	return nil
}
func (r *HTTPRuntime) Run(ctx context.Context) error {
	if r == nil || r.DB == nil || r.Credentials == nil {
		return ErrInvalid
	}
	jobs := []runtimeJob{
		{name: "credentials", interval: 30 * time.Second, timeout: 5 * time.Second, pass: r.Credentials.Refresh},
		{name: "discovery_events", interval: time.Second, timeout: 10 * time.Second, pass: r.eventsOnce},
		{name: "maintenance", interval: 30 * time.Second, timeout: 10 * time.Second, pass: r.maintain},
	}
	if r.Push != nil {
		// Push jobs share the HTTP runtime's pass state and shutdown boundary.
		if r.Push.Credentials == nil {
			return ErrInvalid
		}
		jobs = append(jobs,
			runtimeJob{name: "push_credentials", interval: 30 * time.Second, timeout: 5 * time.Second, pass: r.Push.Credentials.Refresh},
			runtimeJob{name: "push_delivery", interval: time.Second, timeout: 30 * time.Second, pass: func(ctx context.Context) error { return r.Push.Worker.Once(ctx, 20) }},
		)
	}
	return r.run(ctx, jobs)
}
func (r *HTTPRuntime) eventsOnce(ctx context.Context) error {
	evaluation, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, err := r.Events.EvaluateBatch(evaluation, 100, time.Now().UTC())
	cancel()
	if err != nil {
		return fmt.Errorf("evaluate discovery alerts: %w", err)
	}
	_, err = r.Events.PublishBatch(ctx, 100)
	return err
}
func (r *HTTPRuntime) maintain(ctx context.Context) error {
	if err := r.refreshSources(ctx); err != nil {
		return fmt.Errorf("refresh sources: %w", err)
	}
	if _, err := r.Notifications.Store.Retain(ctx, time.Now().UTC(), 90, 1000); err != nil {
		return fmt.Errorf("retain notifications: %w", err)
	}
	if err := r.Discovery.Store.ExpireSnapshots(ctx, time.Now().UTC()); err != nil {
		return fmt.Errorf("expire snapshots: %w", err)
	}
	return nil
}

type publicSourceLister interface {
	PublicSources(context.Context) ([]discovery.SourceRef, error)
}

func (r *HTTPRuntime) refreshSources(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	current := []discovery.SourceRef{}
	if provider, ok := r.Config.Pools.(publicSourceLister); ok {
		var err error
		current, err = provider.PublicSources(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		if len(current) > 2000 {
			return ErrInvalid
		}
	}
	rows, e := r.DB.QueryContext(ctx, `SELECT document FROM narrative_discovery.sources ORDER BY published_at DESC,event_id LIMIT 2000`)
	if e != nil {
		return e
	}
	stored := map[string]storedRefreshSource{}
	remaining := []discovery.SourceRef{}
	listed := map[string]bool{}
	for _, ref := range current {
		if ref.Kind == resourcePool {
			listed[ref.ID] = true
		}
	}
	for rows.Next() {
		var raw json.RawMessage
		var ref discovery.SourceRef
		if e = rows.Scan(&raw); e == nil {
			e = json.Unmarshal(raw, &ref)
		}
		if e != nil {
			return errors.Join(e, rows.Close())
		}
		stored[ref.ID] = storedRefreshSource{ref: ref, document: raw}
		if !listed[ref.ID] {
			remaining = append(remaining, ref)
		}
	}
	if e = errors.Join(rows.Err(), rows.Close()); e != nil {
		return e
	}
	pass := sourceRefreshPass{runtime: r, stored: stored, listed: listed, seen: map[string]bool{}}
	if err := pass.group(ctx, current, true); err != nil {
		return err
	}
	if err := pass.group(ctx, remaining, false); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if pass.failures != 0 {
		return fmt.Errorf("%d failed sources; first %s %s: %s", pass.failures, pass.firstKind, pass.firstID, pass.firstCause)
	}
	return nil
}
