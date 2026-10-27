// Copyright 2020 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package evmrpc

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"
)

// HTTPConfig is the JSON-RPC/HTTP configuration.
type HTTPConfig struct {
	Modules            []string
	CorsAllowedOrigins []string
	Vhosts             []string
	DenyList           []string
	// PaxLegacyAllowlist is BuildPaxLegacyEnabledSet(app.toml enabled_legacy_pax_apis); nil skips the HTTP gate
	// for gated pax_* and pax2_* methods.
	PaxLegacyAllowlist map[string]struct{}
	prefix             string // path prefix on which to mount http handler
	RPCEndpointConfig
}

// WsConfig is the JSON-RPC/Websocket configuration
type WsConfig struct {
	Origins  []string
	Modules  []string
	DenyList []string
	prefix   string // path prefix on which to mount ws handler
	RPCEndpointConfig
}

type RPCEndpointConfig struct {
	JwtSecret              []byte // optional JWT secret
	batchItemLimit         int
	batchResponseSizeLimit int
	readLimit              int64
	ipRateLimitRPS         float64
	ipRateLimitBurst       int
}

type rpcHandler struct {
	http.Handler
	server  *rpc.Server
	limiter *ipRateLimiter
}

func (l *ipRateLimiter) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":null,"error":{"code":-32005,"message":"rate limit exceeded"}}`)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type HTTPServer struct {
	timeouts rpc.HTTPTimeouts
	mux      http.ServeMux // registered handlers go here

	mu       sync.Mutex
	server   *http.Server
	listener net.Listener // non-nil when server is running
	onStop   []func()

	// HTTP RPC handler things.

	HTTPConfig  HTTPConfig
	httpHandler atomic.Value // *rpcHandler
	apis        []rpc.API

	// WebSocket handler things.
	WsConfig  WsConfig
	wsHandler atomic.Value // *rpcHandler
	wsAPIs    []rpc.API

	// These are set by SetListenAddr.
	endpoint string
	host     string
	port     int

	handlerNames map[string]string
}

const (
	shutdownTimeout        = 5 * time.Second
	metricsPrinterInterval = 5 * time.Second
)

func NewHTTPServer(timeouts rpc.HTTPTimeouts) *HTTPServer {
	CheckTimeouts(&timeouts)
	h := &HTTPServer{timeouts: timeouts, handlerNames: make(map[string]string)}

	h.httpHandler.Store((*rpcHandler)(nil))
	h.wsHandler.Store((*rpcHandler)(nil))
	return h
}

// SetListenAddr configures the listening address of the server.
// The address can only be set while the server isn't running.
func (h *HTTPServer) SetListenAddr(host string, port int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.listener != nil && (host != h.host || port != h.port) {
		return fmt.Errorf("HTTP server already running on %s", h.endpoint)
	}

	h.host, h.port = host, port
	h.endpoint = net.JoinHostPort(host, fmt.Sprintf("%d", port))
	return nil
}

// ListenAddr returns the listening address of the server.
func (h *HTTPServer) ListenAddr() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.listener != nil {
		return h.listener.Addr().String()
	}
	return h.endpoint
}

// Start starts the HTTP server if it is enabled and not already running.
func (h *HTTPServer) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.endpoint == "" || h.listener != nil {
		return nil // already running or not configured
	}

	// Initialize the server.
	h.server = &http.Server{
		Handler:           h,
		ReadTimeout:       h.timeouts.ReadTimeout,
		ReadHeaderTimeout: h.timeouts.ReadHeaderTimeout,
		WriteTimeout:      h.timeouts.WriteTimeout,
		IdleTimeout:       h.timeouts.IdleTimeout,
	}

	// Start the server.
	listener, err := net.Listen("tcp", h.endpoint)
	if err != nil {
		// If the server fails to start, we need to clear out the RPC and WS
		// configuration so they can be configured another time.
		h.disableRPC()
		h.disableWS()
		return err
	}
	h.listener = listener
	go func() {
		if err := h.server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped erroneously", "err", err)
		}
	}()

	if h.wsAllowed() {
		url := fmt.Sprintf("ws://%v", listener.Addr())
		if h.WsConfig.prefix != "" {
			url += h.WsConfig.prefix
		}
		logger.Info("WebSocket enabled", "url", url)
	}
	// if server is websocket only, return after logging
	if !h.rpcAllowed() {
		return nil
	}
	// Log http endpoint.
	logger.Info("HTTP server started",
		"endpoint", listener.Addr(), "auth", (h.HTTPConfig.JwtSecret != nil),
		"prefix", h.HTTPConfig.prefix,
		"cors", strings.Join(h.HTTPConfig.CorsAllowedOrigins, ","),
		"vhosts", strings.Join(h.HTTPConfig.Vhosts, ","),
	)

	// Start metrics printer
	// Prometheus metrics are always exported; stdout printing requires EVM_DEBUG_METRICS=true
	StartMetricsPrinter(metricsPrinterInterval)

	// Log all handlers mounted on server.
	paths := make([]string, 0, len(h.handlerNames))
	for path := range h.handlerNames {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	logged := make(map[string]bool, len(paths))
	for _, path := range paths {
		name := h.handlerNames[path]
		if !logged[name] {
			logger.Info(name+" enabled", "url", "http://"+listener.Addr().String()+path)
			logged[name] = true
		}
	}
	return nil
}

func (h *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// check if ws request and serve if ws enabled
	ws := h.wsHandler.Load().(*rpcHandler)
	if ws != nil && websocket.IsWebSocketUpgrade(r) {
		if CheckPath(r, h.WsConfig.prefix) {
			ws.ServeHTTP(w, r)
		}
		return
	}

	// if http-rpc is enabled, try to serve request
	rpc := h.httpHandler.Load().(*rpcHandler)
	if rpc != nil {
		// First try to route in the mux.
		// Requests to a path below root are handled by the mux,
		// which has all the handlers registered via Node.RegisterHandler.
		// These are made available when RPC is enabled.
		muxHandler, pattern := h.mux.Handler(r)
		if pattern != "" {
			muxHandler.ServeHTTP(w, r)
			return
		}

		if CheckPath(r, h.HTTPConfig.prefix) {
			rpc.ServeHTTP(w, r)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

// CheckPath checks whether a given request URL matches a given path prefix.
func CheckPath(r *http.Request, path string) bool {
	// if no prefix has been specified, request URL must be on root
	if path == "" {
		return r.URL.Path == "/"
	}
	// otherwise, check to make sure prefix matches
	return len(r.URL.Path) >= len(path) && r.URL.Path[:len(path)] == path
}

func (h *HTTPServer) addStopHook(hook func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onStop = append(h.onStop, hook)
}

// Stop shuts down the HTTP server.
func (h *HTTPServer) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	logger.Info("Stopping EVM HTTP Server")
	h.doStop()
	logger.Info("EVM HTTP Server stopped")
}

func (h *HTTPServer) doStop() {
	defer func() { h.onStop = nil }()
	if h.listener == nil {
		h.disableRPC()
		h.disableWS()
		for _, hook := range h.onStop {
			hook()
		}
		return // not running
	}

	// Stop metrics printer
	StopMetricsPrinter()

	// Shut down the server.
	httpHandler := h.httpHandler.Load().(*rpcHandler)
	wsHandler := h.wsHandler.Load().(*rpcHandler)
	if httpHandler != nil {
		h.httpHandler.Store((*rpcHandler)(nil))
		httpHandler.server.Stop()
		httpHandler.limiter.stop()
		h.apis = nil
	}
	if wsHandler != nil {
		h.wsHandler.Store((*rpcHandler)(nil))
		wsHandler.server.Stop()
		wsHandler.limiter.stop()
		h.wsAPIs = nil
	}
	for _, hook := range h.onStop {
		hook()
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	err := h.server.Shutdown(ctx)
	if err != nil {
		if errors.Is(err, ctx.Err()) {
			logger.Error("HTTP server graceful shutdown timed out")
		} else {
			logger.Error("HTTP server graceful shutdown failed", "err", err)
		}
		_ = h.server.Close()
	}

	_ = h.listener.Close()
	logger.Info("HTTP server stopped", "endpoint", h.listener.Addr())

	// Clear out everything to allow re-configuring it later.
	h.host, h.port, h.endpoint = "", 0, ""
	h.server, h.listener = nil, nil
}

// EnableRPC turns on JSON-RPC over HTTP on the server.
func (h *HTTPServer) EnableRPC(apis []rpc.API, config HTTPConfig) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rpcAllowed() {
		return fmt.Errorf("JSON-RPC over HTTP is already enabled")
	}

	// Create RPC server and handler.
	srv := rpc.NewServer()
	srv.SetBatchLimits(config.batchItemLimit, config.batchResponseSizeLimit)
	logger.Info("Registering apis for evm rpc")
	if err := RegisterApis(apis, config.Modules, srv); err != nil {
		return err
	}
	logger.Info("Registering deny list for evm rpc", "deny-list", config.DenyList)
	for _, method := range config.DenyList {
		srv.RegisterDenyList(method)
	}
	h.HTTPConfig = config
	h.apis = registeredAPIs(apis, config.Modules)
	base := NewHTTPHandlerStack(srv, config.CorsAllowedOrigins, config.Vhosts, config.JwtSecret)
	limiter := newIPRateLimiter(config.ipRateLimitRPS, config.ipRateLimitBurst)
	h.httpHandler.Store(&rpcHandler{
		Handler: limiter.wrap(wrapPaxLegacyHTTP(base, config.PaxLegacyAllowlist)),
		server:  srv,
		limiter: limiter,
	})
	return nil
}

// disableRPC stops the HTTP RPC handler. This is internal, the caller must hold h.mu.
func (h *HTTPServer) disableRPC() bool {
	handler := h.httpHandler.Load().(*rpcHandler)
	if handler != nil {
		h.httpHandler.Store((*rpcHandler)(nil))
		handler.server.Stop()
		handler.limiter.stop()
		h.apis = nil
	}
	return handler != nil
}

// EnableWS turns on JSON-RPC over WebSocket on the server.
func (h *HTTPServer) EnableWS(apis []rpc.API, config WsConfig) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.wsAllowed() {
		return fmt.Errorf("JSON-RPC over WebSocket is already enabled")
	}
	// Create RPC server and handler.
	srv := rpc.NewServer()
	srv.SetBatchLimits(config.batchItemLimit, config.batchResponseSizeLimit)
	srv.SetReadLimits(config.readLimit)
	logger.Info("Registering apis for evm websocket")
	if err := RegisterApis(apis, config.Modules, srv); err != nil {
		return err
	}
	logger.Info("Registering deny list for evm websocket", "deny-list", config.DenyList)
	for _, method := range config.DenyList {
		srv.RegisterDenyList(method)
	}
	h.WsConfig = config
	h.wsAPIs = registeredAPIs(apis, config.Modules)
	limiter := newIPRateLimiter(config.ipRateLimitRPS, config.ipRateLimitBurst)
	h.wsHandler.Store(&rpcHandler{
		Handler: limiter.wrap(NewWSHandlerStack(newDenyListWSHandler(srv, config), config.JwtSecret)),
		server:  srv,
		limiter: limiter,
	})
	return nil
}

type denyListWSConn struct {
	*websocket.Conn
	mu sync.Mutex
}

func (c *denyListWSConn) SetWriteDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.SetWriteDeadline(deadline)
}

func (c *denyListWSConn) writeJSON(value interface{}, _ bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteJSON(value)
}

func newDenyListWSHandler(srv *rpc.Server, config WsConfig) http.Handler {
	if len(config.DenyList) == 0 {
		return srv.WebsocketHandler(config.Origins)
	}
	denied := make(map[string]struct{}, len(config.DenyList))
	for _, method := range config.DenyList {
		denied[method] = struct{}{}
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return wsOriginAllowed(r, config.Origins)
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conn.SetReadLimit(config.readLimit)
		c := &denyListWSConn{Conn: conn}
		stopped := make(chan struct{})
		defer close(stopped)
		conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Time{}) })
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopped:
					return
				case <-ticker.C:
					_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
					if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
						_ = conn.Close()
						return
					}
				}
			}
		}()
		decode := func(value interface{}) error {
			for {
				var raw json.RawMessage
				if err := conn.ReadJSON(&raw); err != nil {
					return err
				}
				var requests []json.RawMessage
				batch := bytes.HasPrefix(bytes.TrimSpace(raw), []byte("["))
				if batch {
					if err := json.Unmarshal(raw, &requests); err != nil {
						return err
					}
				} else {
					requests = []json.RawMessage{raw}
				}
				var blocked *jsonrpcMessage
				for _, request := range requests {
					var message jsonrpcMessage
					if json.Unmarshal(request, &message) == nil {
						if _, found := denied[message.Method]; found {
							blocked = &message
							break
						}
					}
				}
				if blocked == nil {
					return json.Unmarshal(raw, value)
				}
				response := struct {
					Version string          `json:"jsonrpc"`
					ID      json.RawMessage `json:"id"`
					Error   struct {
						Code    int    `json:"code"`
						Message string `json:"message"`
					} `json:"error"`
				}{Version: "2.0", ID: orNullID(blocked.ID)}
				response.Error.Code = -32601
				response.Error.Message = fmt.Sprintf("the method %s does not exist/is not available", blocked.Method)
				var reply interface{} = response
				if batch {
					responses := make([]interface{}, 0, len(requests))
					for _, request := range requests {
						var message jsonrpcMessage
						if json.Unmarshal(request, &message) == nil && message.hasValidID() {
							response.ID = message.ID
							responses = append(responses, response)
						}
					}
					if len(responses) == 0 {
						continue
					}
					reply = responses
				} else if blocked.ID == nil {
					continue
				}
				if err := c.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
					return err
				}
				if err := c.writeJSON(reply, true); err != nil {
					return err
				}
			}
		}
		srv.ServeCodec(rpc.NewFuncCodec(c, c.writeJSON, decode), 0)
	})
}

func wsOriginAllowed(r *http.Request, origins []string) bool {
	if _, present := r.Header["Origin"]; !present {
		return true
	}
	allowed := make([]string, 0, len(origins))
	for _, origin := range origins {
		if origin != "" {
			allowed = append(allowed, origin)
		}
	}
	if len(allowed) == 0 {
		allowed = append(allowed, "http://localhost")
		if hostname, err := os.Hostname(); err == nil {
			allowed = append(allowed, "http://"+hostname)
		}
	}
	origin, err := url.Parse(strings.ToLower(r.Header.Get("Origin")))
	if err != nil {
		return false
	}
	for _, rule := range allowed {
		if rule == "*" {
			return true
		}
		parsed, err := url.Parse(rule)
		if err != nil {
			continue
		}
		scheme, host, port := parsed.Scheme, parsed.Hostname(), parsed.Port()
		if !strings.Contains(rule, "://") {
			scheme, host, port = "", parsed.Scheme, parsed.Opaque
			if host == "" {
				host = rule
			}
		}
		if (scheme == "" || scheme == origin.Scheme) && (host == "" || host == origin.Hostname()) && (port == "" || port == origin.Port()) {
			return true
		}
	}
	return false
}

// disableWS disables the WebSocket handler. This is internal, the caller must hold h.mu.
func (h *HTTPServer) disableWS() bool {
	ws := h.wsHandler.Load().(*rpcHandler)
	if ws != nil {
		h.wsHandler.Store((*rpcHandler)(nil))
		ws.server.Stop()
		ws.limiter.stop()
		h.wsAPIs = nil
	}
	return ws != nil
}

// rpcAllowed returns true when JSON-RPC over HTTP is enabled.
func (h *HTTPServer) rpcAllowed() bool {
	return h.httpHandler.Load().(*rpcHandler) != nil
}

// wsAllowed returns true when JSON-RPC over WebSocket is enabled.
func (h *HTTPServer) wsAllowed() bool {
	return h.wsHandler.Load().(*rpcHandler) != nil
}

// NewHTTPHandlerStack returns wrapped http-related handlers.
func NewHTTPHandlerStack(srv http.Handler, cors []string, vhosts []string, JwtSecret []byte) http.Handler {
	// Wrap the CORS-handler within a host-handler
	handler := newCorsHandler(srv, cors)
	handler = newVHostHandler(vhosts, handler)
	if len(JwtSecret) != 0 {
		handler = newJWTHandler(JwtSecret, handler)
	}
	return NewGzipHandler(handler)
}

// NewWSHandlerStack returns a wrapped ws-related handler.
func NewWSHandlerStack(srv http.Handler, JwtSecret []byte) http.Handler {
	handler := srv
	if len(JwtSecret) != 0 {
		handler = newJWTHandler(JwtSecret, handler)
	}
	return NewWSConnectionHandler(handler)
}

func newCorsHandler(srv http.Handler, allowedOrigins []string) http.Handler {
	// disable CORS support if user has not specified a custom CORS configuration
	if len(allowedOrigins) == 0 {
		return srv
	}
	c := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{http.MethodPost, http.MethodGet},
		AllowedHeaders: []string{"*"},
		MaxAge:         600,
	})
	return c.Handler(srv)
}

// virtualHostHandler is a handler which validates the Host-header of incoming requests.
// Using virtual hosts can help prevent DNS rebinding attacks, where a 'random' domain name points to
// the service ip address (but without CORS headers). By verifying the targeted virtual host, we can
// ensure that it's a destination that the node operator has defined.
type virtualHostHandler struct {
	vhosts map[string]struct{}
	next   http.Handler
}

func newVHostHandler(vhosts []string, next http.Handler) http.Handler {
	vhostMap := make(map[string]struct{})
	for _, allowedHost := range vhosts {
		vhostMap[strings.ToLower(allowedHost)] = struct{}{}
	}
	return &virtualHostHandler{vhostMap, next}
}

// ServeHTTP serves JSON-RPC requests over HTTP, implements http.Handler
func (h *virtualHostHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// if r.Host is not set, we can continue serving since a browser would set the Host header
	if r.Host == "" {
		h.next.ServeHTTP(w, r)
		return
	}
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		// Either invalid (too many colons) or no port specified
		host = r.Host
	}
	if ipAddr := net.ParseIP(host); ipAddr != nil {
		// It's an IP address, we can serve that
		h.next.ServeHTTP(w, r)
		return
	}
	host = strings.ToLower(host)
	// Not an IP address, but a hostname. Need to validate
	if _, exist := h.vhosts["*"]; exist {
		h.next.ServeHTTP(w, r)
		return
	}
	if _, exist := h.vhosts[host]; exist {
		h.next.ServeHTTP(w, r)
		return
	}
	http.Error(w, "invalid host specified", http.StatusForbidden)
}

var gzPool = sync.Pool{
	New: func() any {
		w := gzip.NewWriter(io.Discard)
		return w
	},
}

type gzipResponseWriter struct {
	resp http.ResponseWriter

	gz            *gzip.Writer
	contentLength uint64 // total length of the uncompressed response
	written       uint64 // amount of written bytes from the uncompressed response
	hasLength     bool   // true if uncompressed response had Content-Length
	inited        bool   // true after init was called for the first time
}

// init runs just before response headers are written. Among other things, this function
// also decides whether compression will be applied at all.
func (w *gzipResponseWriter) init() {
	if w.inited {
		return
	}
	w.inited = true

	hdr := w.resp.Header()
	length := hdr.Get("content-length")
	if len(length) > 0 {
		if n, err := strconv.ParseUint(length, 10, 64); err == nil {
			w.hasLength = true
			w.contentLength = n
		}
	}

	// Setting Transfer-Encoding to "identity" explicitly disables compression. net/http
	// also recognizes this header value and uses it to disable "chunked" transfer
	// encoding, trimming the header from the response. This means downstream handlers can
	// set this without harm, even if they aren't wrapped by NewGzipHandler.
	//
	// In go-ethereum, we use this signal to disable compression for certain error
	// responses which are flushed out close to the write deadline of the response. For
	// these cases, we want to avoid chunked transfer encoding and compression because
	// they require additional output that may not get written in time.
	passthrough := hdr.Get("transfer-encoding") == "identity"
	if !passthrough {
		w.gz = gzPool.Get().(*gzip.Writer)
		w.gz.Reset(w.resp)
		hdr.Del("content-length")
		hdr.Set("content-encoding", "gzip")
	}
}

func (w *gzipResponseWriter) Header() http.Header {
	return w.resp.Header()
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	w.init()
	w.resp.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	w.init()

	if w.gz == nil {
		// Compression is disabled.
		return w.resp.Write(b)
	}

	n, err := w.gz.Write(b)
	w.written += uint64(n) //nolint:gosec
	if w.hasLength && w.written >= w.contentLength {
		// The HTTP handler has finished writing the entire uncompressed response. Close
		// the gzip stream to ensure the footer will be seen by the client in case the
		// response is flushed after this call to write.
		err = w.gz.Close()
		gzPool.Put(w.gz)
		w.gz = nil
	}
	return n, err
}

func (w *gzipResponseWriter) Flush() {
	if w.gz != nil {
		_ = w.gz.Flush()
	}
	if f, ok := w.resp.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *gzipResponseWriter) close() {
	if w.gz == nil {
		return
	}
	_ = w.gz.Close()
	gzPool.Put(w.gz)
	w.gz = nil
}

func NewGzipHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		wrapper := &gzipResponseWriter{resp: w}
		defer wrapper.close()

		next.ServeHTTP(wrapper, r)
	})
}

// RegisterApis checks the given modules' availability, generates an allowlist based on the allowed modules,
// and then registers all of the APIs exposed by the services.
func RegisterApis(apis []rpc.API, modules []string, srv *rpc.Server) error {
	if bad, available := checkModuleAvailability(modules, apis); len(bad) > 0 {
		return fmt.Errorf("unavailable RPC modules %q; available modules are %q", bad, available)
	}
	// Generate the allow list based on the allowed modules
	allowList := make(map[string]bool)
	for _, module := range modules {
		allowList[module] = true
	}
	// Register all the APIs exposed by the services
	for _, api := range apis {
		if allowList[api.Namespace] || len(allowList) == 0 {
			if err := srv.RegisterName(api.Namespace, api.Service); err != nil {
				return err
			}
		}
	}
	return nil
}
