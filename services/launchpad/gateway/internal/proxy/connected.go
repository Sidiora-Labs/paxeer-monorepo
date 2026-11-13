package proxy

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/cleanup"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

type ConnectedRoute struct {
	Method       string
	Template     string
	Authenticate bool
	Optional     bool
	Upload       bool
}
type RequestIdentityAuthority interface {
	AuthenticateRequest(context.Context, *http.Request) (sharedauth.IdentityPrincipal, error)
}
type ConnectedDeps struct {
	Name       string
	URL        string
	Routes     []ConnectedRoute
	Identity   RequestIdentityAuthority
	SigningKey ed25519.PrivateKey
	TLS        *tls.Config
}
type Connected struct {
	target   *url.URL
	routes   []ConnectedRoute
	identity RequestIdentityAuthority
	key      ed25519.PrivateKey
	audience string
	client   *http.Client
}

func NewConnected(d ConnectedDeps) (*Connected, error) {
	if d.Name != metadataServiceName && d.Name != userServiceName && d.Name != socialServiceName && d.Name != livestreamServiceName {
		return nil, errors.New("unknown media upstream")
	}
	u, e := url.Parse(d.URL)
	if e != nil || u.Scheme != httpsScheme || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil || len(d.SigningKey) != ed25519.PrivateKeySize || d.TLS == nil || d.TLS.InsecureSkipVerify || d.TLS.RootCAs == nil || len(d.TLS.Certificates) == 0 || len(d.Routes) == 0 || len(d.Routes) > 256 {
		return nil, errors.New("private media channel unavailable")
	}
	if d.Identity == nil {
		return nil, errors.New("identity authority unavailable")
	}
	routes := append([]ConnectedRoute(nil), d.Routes...)
	seen := map[string]bool{}
	for _, r := range routes {
		if !allowedConnectedRoute(d.Name, r) || seen[r.Method+" "+r.Template] {
			return nil, errors.New("unknown media route")
		}
		seen[r.Method+" "+r.Template] = true
	}
	tlsConfig := d.TLS.Clone()
	if tlsConfig.MinVersion < tls.VersionTLS12 {
		tlsConfig.MinVersion = tls.VersionTLS12
	}
	// The response-header wait matches the longest per-route deadline (10s:
	// uploads and metadata publication admission); every request also runs
	// under its route's context deadline (3s by default), which is what
	// bounds the ordinary routes.
	transport := &http.Transport{TLSClientConfig: tlsConfig, Proxy: nil, MaxIdleConns: 128, MaxIdleConnsPerHost: 32, IdleConnTimeout: 90 * time.Second, ResponseHeaderTimeout: 10 * time.Second, DisableCompression: true}
	return &Connected{u, routes, d.Identity, append(ed25519.PrivateKey(nil), d.SigningKey...), d.Name, &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func FrozenConnectedSurface() map[string]map[string]string {
	return map[string]map[string]string{
		metadataServiceName:   {"/v2/tokens/{tokenAddress}/metadata": getPatchMethods, "/v2/tokens/{tokenAddress}/media": getMethod, "/v2/tokens/{tokenAddress}/assets/{assetId}": "GET PUT", "/v2/accounts/{wallet}/assets/{assetId}": "GET PUT"},
		userServiceName:       {profileRoute: getPatchMethods, batchProfilesRoute: getMethod, "/v2/me/watchlist": getMethod, "/v2/me/watchlist/{poolAddress}": putDeleteMethods},
		livestreamServiceName: {streamsRoute: getPostMethods, "/v2/streams/operations/{operationId}": getMethod, "/v2/streams/{streamId}:start": postMethod, "/v2/streams/{streamId}:end": postMethod, "/v2/streams/{streamId}/viewers:heartbeat": postMethod, "/v2/streams/{streamId}/ingest-credential": postMethod},
		socialServiceName:     {followingRoute: putDeleteMethods, "/v2/profiles/{wallet}/followers": getMethod, "/v2/profiles/{wallet}/follow-stats": getMethod, "/v2/pools/{poolAddress}/comments": getPostMethods, "/v2/comments/{commentId}": patchDeleteMethods, "/v2/comments/{commentId}/likes/me": putDeleteMethods, "/v2/reports": postMethod, "/v2/callouts": postMethod, "/v2/pools/{poolAddress}/callouts": getMethod, "/v2/bounties": getPostMethods, "/v2/bounties/stats": getMethod, "/v2/bounties/{bountyId}": getMethod, "/v2/bounties/{bountyId}/submissions": getPostMethods, "/v2/bounties/{bountyId}/submissions/{submissionId}:accept": postMethod},
	}
}

func allowedConnectedRoute(name string, r ConnectedRoute) bool {
	routes := FrozenConnectedSurface()
	if r.Optional && (r.Authenticate || r.Method != getMethod) {
		return false
	}
	if name == userServiceName {
		for _, route := range UserConnectedRoutes() {
			if route.Template == r.Template && route.Method == r.Method {
				return r.Authenticate == route.Authenticate && r.Optional == route.Optional && !r.Upload
			}
		}
	}
	if name == metadataServiceName {
		for _, route := range MetadataConnectedRoutes() {
			if route.Template == r.Template && route.Method == r.Method {
				return r.Authenticate == route.Authenticate && r.Optional == route.Optional && !r.Upload
			}
		}
	}
	if name == livestreamServiceName {
		for _, route := range LivestreamConnectedRoutes() {
			if route.Template == r.Template && route.Method == r.Method {
				return r.Authenticate == route.Authenticate && r.Optional == route.Optional && !r.Upload
			}
		}
	}
	if name == socialServiceName {
		for _, route := range SocialConnectedRoutes() {
			if route.Template == r.Template && route.Method == r.Method {
				return r.Authenticate == route.Authenticate && r.Optional == route.Optional && !r.Upload
			}
		}
	}
	methods, ok := routes[name][r.Template]
	if !ok || !strings.Contains(" "+methods+" ", " "+r.Method+" ") {
		return false
	}
	if r.Method != getMethod && !r.Authenticate {
		return false
	}
	if strings.HasPrefix(r.Template, "/v2/me/") && !r.Authenticate {
		return false
	}
	if strings.Contains(r.Template, "/assets/") && !r.Authenticate {
		return false
	}
	if strings.Contains(r.Template, "/operations/") && !r.Authenticate {
		return false
	}
	if name == metadataServiceName && r.Method == putMethod && strings.Contains(r.Template, "/assets/") && !r.Upload {
		return false
	}
	return !r.Upload || (name == metadataServiceName && (r.Method == postMethod || r.Method == putMethod) && strings.Contains(r.Template, "/assets/"))
}
func connectedMatch(template, path string) bool {
	a, b := strings.Split(template, "/"), strings.Split(path, "/")
	if len(a) != len(b) {
		return false
	}
	for i, part := range a {
		if strings.HasPrefix(part, "{") {
			end := strings.IndexByte(part, '}')
			if end < 2 {
				return false
			}
			suffix := part[end+1:]
			if !strings.HasSuffix(b[i], suffix) {
				return false
			}
			value := strings.TrimSuffix(b[i], suffix)
			if value == "" || len(value) > 128 || strings.ContainsAny(value, "%\\: ") || strings.IndexFunc(value, func(c rune) bool { return c < 0x21 || c == 0x7f }) >= 0 {
				return false
			}
		} else if part != b[i] {
			return false
		}
	}
	return true
}
func (p *Connected) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.URL == nil || r.URL.RawPath != "" || strings.ContainsAny(r.URL.Path, "%\\") || strings.Contains(r.URL.Path, "..") || len(r.URL.RequestURI()) > 8192 {
		http.NotFound(w, r)
		return
	}
	var selected *ConnectedRoute
	for i := range p.routes {
		if p.routes[i].Method == r.Method && connectedMatch(p.routes[i].Template, r.URL.Path) {
			selected = &p.routes[i]
			break
		}
	}
	if selected == nil {
		http.NotFound(w, r)
		return
	}
	timeout := 3 * time.Second
	if selected.Upload || p.slowRoute(selected) {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	r = r.WithContext(ctx)
	var principal sharedauth.IdentityPrincipal
	recovery := p.audience == userServiceName && deletionStatusRoute(selected) && len(r.Header.Values("X-Recovery-Receipt")) != 0
	if recovery && !validRecoveryHeader(r.Header, r.URL.Path[strings.LastIndexByte(r.URL.Path, '/')+1:]) {
		connectedError(w, 400, "recovery_receipt_invalid")
		return
	}
	if p.audience == livestreamServiceName && selected.Method == http.MethodPost && selected.Template == ingestKeyRoute && !sessionCredentials(r) {
		connectedError(w, http.StatusUnauthorized, "identity_unavailable")
		return
	}
	if (selected.Authenticate || (selected.Optional && sessionCredentials(r))) && !recovery {
		var e error
		principal, e = p.identity.AuthenticateRequest(r.Context(), r)
		if e == nil {
			e = principal.Validate(time.Now())
		}
		if e != nil {
			status := http.StatusServiceUnavailable
			var authority *sharedauth.IdentityAuthorityError
			if errors.As(e, &authority) {
				status = authority.StatusCode()
			} else if errors.Is(e, sharedauth.ErrScopeDenied) {
				status = http.StatusForbidden
			}
			connectedError(w, status, "identity_unavailable")
			return
		}
	}
	var body []byte
	if r.Body != nil {
		controller := http.NewResponseController(w)
		deadline, _ := ctx.Deadline()
		_ = controller.SetReadDeadline(deadline)
		defer func() { cleanup.Report("reset read deadline", controller.SetReadDeadline(time.Time{})) }()
		var e error
		limit := int64(1 << 20)
		if selected.Upload && r.Method == putMethod {
			limit = 6 << 20
		}
		if p.audience == metadataServiceName && selected.Template == metadataPublications && r.Method == http.MethodPost {
			limit = MetadataPublicationBodyLimit
		}
		body, e = io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
		if e != nil {
			var oversized *http.MaxBytesError
			if errors.As(e, &oversized) {
				connectedError(w, 413, "request_too_large")
			} else if errors.Is(e, context.DeadlineExceeded) || ctx.Err() != nil {
				connectedError(w, 408, "request_timeout")
			} else {
				w.Header().Set("Retry-After", "1")
				connectedError(w, 503, "request_ingress_unavailable")
			}
			return
		}
	}
	target := *p.target
	target.Path = r.URL.Path
	target.RawQuery = r.URL.RawQuery
	req, e := http.NewRequestWithContext(ctx, r.Method, target.String(), bytes.NewReader(body))
	if e != nil {
		connectedError(w, 503, "upstream_unavailable")
		return
	}
	if p.audience == userServiceName && selected.Method == http.MethodGet && selected.Template == "/v2/me/exports/{jobId}/download" {
		if values := r.Header.Values("X-Export-Authorization"); len(values) != 0 {
			if len(values) != 1 || !validExportAuthorization(values[0]) {
				connectedError(w, 400, "export_authorization_invalid")
				return
			}
			req.Header.Set("X-Export-Authorization", values[0])
		}
	}
	for _, name := range []string{"Content-Type", "Accept", "If-None-Match", "If-Match", "Idempotency-Key", "X-Action-Signature", "X-Action-Nonce", "X-Action-Expires", "X-Action-Audience", "X-Kindle-Operation", "X-Kindle-Operation-Signature", "X-Kindle-Signature", "X-Request-ID", "Origin", "X-CSRF-Token", "X-Recovery-Receipt"} {
		if values := r.Header.Values(name); len(values) == 1 {
			req.Header.Set(name, values[0])
		} else if len(values) > 1 {
			connectedError(w, 400, "ambiguous_header")
			return
		}
	}
	if p.audience == livestreamServiceName && selected.Template == "/v2/provider-webhooks/livepeer" {
		if values := r.Header.Values("Livepeer-Signature"); len(values) == 1 {
			req.Header.Set("Livepeer-Signature", values[0])
		} else if len(values) > 1 {
			connectedError(w, 400, "ambiguous_header")
			return
		}
	}
	if principal.Wallet != "" {
		if authorization := r.Header.Get("Authorization"); authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		if cookie, e := r.Cookie("__Host-kindle_access"); e == nil {
			req.AddCookie(cookie)
		}
		if supplied := r.Header.Get("X-Action-Wallet"); supplied != "" && supplied != principal.Wallet {
			connectedError(w, 403, "actor_mismatch")
			return
		}
		req.Header.Set("X-Action-Wallet", principal.Wallet)
		requestID := req.Header.Get("X-Request-ID")
		if _, err := uuid.Parse(requestID); err != nil {
			requestID = uuid.NewString()
			req.Header.Set("X-Request-ID", requestID)
		}
		signer := sharedauth.ServiceAssertionIssuer{Issuer: gatewayServiceName, Audience: connectedAssertionAudience(p.audience, selected.Template), Key: p.key, Transport: actorAssertionTransport(p.audience)}
		if e = signer.SignRequest(req, principal, []string{"owner"}, body); e != nil {
			connectedError(w, 403, "actor_assertion_denied")
			return
		}
	}
	if recovery {
		requestID := req.Header.Get("X-Request-ID")
		if _, e := uuid.Parse(requestID); e != nil {
			req.Header.Set("X-Request-ID", uuid.NewString())
		}
		signer := sharedauth.ServiceRequestIssuer{Issuer: gatewayServiceName, Audience: "account", Key: p.key, Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.IdentityAssertionHeader, SignatureHeader: sharedauth.IdentitySignatureHeader}}
		if e = signer.SignRequest(req, nil); e != nil {
			connectedError(w, 403, "recovery_assertion_denied")
			return
		}
	}
	resp, e := p.client.Do(req)
	if e != nil {
		connectedError(w, 503, "upstream_unavailable")
		return
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			cleanup.Report("close response body", closeErr)
		}
	}()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 && resp.StatusCode != 304 {
		connectedError(w, 502, "upstream_redirect_denied")
		return
	}
	if resp.StatusCode >= 500 {
		upstreamUnavailable(w, resp)
		return
	}
	if resp.StatusCode == 304 || resp.StatusCode == 204 {
		if etag := resp.Header.Get("ETag"); len(etag) <= 128 && !strings.ContainsAny(etag, "\r\n") {
			w.Header().Set("ETag", etag)
		}
		w.WriteHeader(resp.StatusCode)
		return
	}
	if p.audience == socialServiceName && selected.Template == "/v2/voice-rooms/{roomId}/recording/object" && resp.StatusCode == 200 {
		if resp.Header.Get("Content-Type") != "audio/ogg" {
			connectedError(w, 502, "upstream_response_invalid")
			return
		}
		w.Header().Set("Content-Type", "audio/ogg")
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if resp.ContentLength >= 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
		}
		if _, err := io.Copy(w, resp.Body); err != nil {
			panic(http.ErrAbortHandler)
		}
		return
	}
	if p.audience == metadataServiceName && selected.Template == metadataLogo && resp.StatusCode == 200 {
		image, e := io.ReadAll(io.LimitReader(resp.Body, metadataLogoLimit+1))
		if e != nil || len(image) > metadataLogoLimit || resp.Header.Get("Content-Type") != "image/png" || len(image) < 8 || !bytes.Equal(image[:8], []byte("\x89PNG\r\n\x1a\n")) {
			connectedError(w, 502, "upstream_response_invalid")
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(image)
		return
	}
	result, e := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if e != nil || len(result) > 1<<20 || !json.Valid(result) {
		connectedError(w, 502, "upstream_response_invalid")
		return
	}
	privateIngest := p.audience == livestreamServiceName && selected.Method == http.MethodPost && selected.Template == ingestKeyRoute && resp.StatusCode == http.StatusOK
	if privateIngest && !validIngestResponse(result, strings.Split(r.URL.Path, "/")[3], time.Now()) {
		connectedError(w, 502, "upstream_response_invalid")
		return
	}
	if !privateIngest && containsSecret(result) {
		connectedError(w, 502, "upstream_response_restricted")
		return
	}
	if p.audience == userServiceName && deletionCreateRoute(selected) && resp.StatusCode == http.StatusAccepted {
		var accepted struct {
			Data struct {
				Job struct {
					JobID     string `json:"jobId"`
					AccountID string `json:"accountId"`
				} `json:"job"`
			} `json:"data"`
		}
		if json.Unmarshal(result, &accepted) != nil || !validRecoveryHeader(resp.Header, accepted.Data.Job.JobID) || recoveryAccount(resp.Header.Get("X-Recovery-Receipt")) != accepted.Data.Job.AccountID {
			connectedError(w, 502, "upstream_recovery_receipt_invalid")
			return
		}
		w.Header().Set("X-Recovery-Receipt", resp.Header.Get("X-Recovery-Receipt"))
	}
	for _, name := range []string{"ETag", "Retry-After"} {
		if v := resp.Header.Get(name); len(v) <= 128 && !strings.ContainsAny(v, "\r\n") {
			w.Header().Set(name, v)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(result)
}
func containsSecret(raw []byte) bool {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return true
	}
	var inspect func(any) bool
	inspect = func(v any) bool {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				switch strings.ToLower(k) {
				case "secretaccesskey", "accesskeyid", "servicetoken", "providertoken", "providersecret", "streamkey", "ingestkey", "privatekey", "authorization":
					return true
				}
				if inspect(v) {
					return true
				}
			}
		case []any:
			for _, v := range x {
				if inspect(v) {
					return true
				}
			}
		case string:
			lower := strings.ToLower(x)
			if strings.Contains(lower, "x-amz-credential=") || strings.Contains(lower, "x-amz-signature=") {
				return true
			}
		}
		return false
	}
	return inspect(value)
}

// sessionCredentials reports whether the caller presented a session: an
// Optional route authenticates only then, so an anonymous read stays anonymous
// while a stale credential is rejected instead of silently downgraded.
func sessionCredentials(r *http.Request) bool {
	if r.Header.Get("Authorization") != "" {
		return true
	}
	_, e := r.Cookie("__Host-kindle_access")
	return e == nil
}

// SocialContractRoutes are the frozen v2 follow-graph paths served by the
// social upstream, which owns the follow store. The reads are public and
// viewer-aware (Optional), the writes require a session.
func SocialContractRoutes() []ConnectedRoute {
	return []ConnectedRoute{
		{Method: http.MethodPut, Template: followingRoute, Authenticate: true},
		{Method: http.MethodDelete, Template: followingRoute, Authenticate: true},
		{Method: http.MethodGet, Template: "/v2/profiles/{wallet}/followers", Optional: true},
		{Method: http.MethodGet, Template: "/v2/profiles/{wallet}/follow-stats", Optional: true},
	}
}

// upstreamUnavailableMessage is the only message relayed for an upstream 5xx;
// upstream wording and details (dependency and capability names) stay inside.
const upstreamUnavailableMessage = "Upstream service unavailable"

// upstreamUnavailable collapses every upstream 5xx into a 503. When the
// upstream answered with a valid v2 Error envelope, the 503 body is a v2 Error
// envelope carrying the upstream's stable code, retry hints and request id, so
// a client can tell a coded upstream condition (DEPENDENCY_UNAVAILABLE, ...)
// from a dead upstream; the upstream message and details are replaced. Any
// other 5xx (no envelope, oversized, invalid) answers the gateway's own
// {"code":"upstream_unavailable"}. Upstream 4xx envelopes are relayed
// verbatim by ServeHTTP, so in every case the client reads the top-level
// "code": UPPER_SNAKE codes come from the upstream v2 contract, lower_snake
// codes from the gateway itself.
func upstreamUnavailable(w http.ResponseWriter, resp *http.Response) {
	raw, e := io.ReadAll(io.LimitReader(resp.Body, (16<<10)+1))
	var upstream v2.Error
	if e != nil || len(raw) > 16<<10 || !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") || v2.ValidateJSON("Error", raw) != nil || json.Unmarshal(raw, &upstream) != nil || !upstreamCode.MatchString(upstream.Code) {
		connectedError(w, 503, "upstream_unavailable")
		return
	}
	out := v2.Error{Code: upstream.Code, Message: upstreamUnavailableMessage, RequestId: upstream.RequestId, Retryable: upstream.Retryable, RetryAfterMs: upstream.RetryAfterMs, FieldErrors: []v2.FieldError{}}
	if !out.Retryable {
		out.RetryAfterMs = nil
	}
	body, e := json.Marshal(out)
	if e != nil || v2.ValidateJSON("Error", body) != nil {
		connectedError(w, 503, "upstream_unavailable")
		return
	}
	if retry := resp.Header.Get("Retry-After"); out.Retryable && retryAfterSeconds.MatchString(retry) {
		w.Header().Set("Retry-After", retry)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write(body)
}

var (
	upstreamCode      = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)
	retryAfterSeconds = regexp.MustCompile(`^[0-9]{1,3}$`)
)

func connectedError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code})
}

func RetiredConnectedRoutes() []ConnectedRoute {
	return []ConnectedRoute{
		{Method: http.MethodPost, Template: "/v2/assets/uploads"},
		{Method: http.MethodGet, Template: "/v2/assets/{assetId}"},
		{Method: http.MethodPost, Template: "/v2/assets/{assetId}:complete"},
		{Method: http.MethodGet, Template: "/v2/rooms/{roomId}/messages"},
		{Method: http.MethodPost, Template: "/v2/rooms/{roomId}/messages"},
	}
}

func SocialConnectedRoutes() []ConnectedRoute {
	methods := map[string]string{
		"/v2/chat/history": getMethod, "/v2/chat/conversations": getMethod, "/v2/chat/read": postMethod, "/v2/chat/revise": postMethod,
		"/v2/social/users/{wallet}/followers": getMethod, "/v2/social/users/{wallet}/following": getMethod, "/v2/social/users/{wallet}/follow-stats": getMethod,
		"/v2/social/follow/{followee}": putDeleteMethods, "/v2/social/pool/{poolAddress}/comments": getPostMethods, "/v2/social/comments/{id}": patchDeleteMethods, "/v2/social/comments/{id}/hide": postMethod, "/v2/social/comments/{id}/like": putDeleteMethods, "/v2/social/callouts/{id}/references": postMethod, "/v2/social/me/social/privacy": getPatchMethods, "/v2/social/me/social/blocks/{wallet}": putDeleteMethods,
		"/v2/safety/reports": postMethod, "/v2/safety/cases": getMethod, "/v2/safety/cases/{caseId}": "PATCH", "/v2/safety/cases/{caseId}/bans": postMethod, "/v2/safety/cases/{caseId}/restore": postMethod, "/v2/safety/appeals": postMethod, "/v2/safety/appeals/{appealId}": getMethod, "/v2/safety/appeals/{appealId}/reverse-ban": postMethod,
		"/v2/voice-rooms": postMethod, "/v2/voice-rooms/{roomId}/start": postMethod, "/v2/voice-rooms/{roomId}/end": postMethod, "/v2/voice-rooms/{roomId}/moderation-actions": postMethod, "/v2/voice-rooms/{roomId}/prompt": getMethod, "/v2/voice-rooms/{roomId}/tickets": postMethod, "/v2/voice-rooms/{roomId}/recording": getPostMethods, "/v2/voice-rooms/{roomId}/recording/object": getMethod, "/v2/voice-rooms/{roomId}/recording/consent": postMethod, "/v2/voice-rooms/{roomId}/recording/start": postMethod, "/v2/voice-rooms/{roomId}/recording/stop": postMethod, "/v2/voice-rooms/{roomId}/speaker-requests": postMethod, "/v2/voice-rooms/{roomId}/speakers": postMethod, "/v2/voice-rooms/{roomId}/leave": postMethod,
		"/v3/voice-rooms": postMethod, "/v3/voice-rooms/{roomId}/moderation-actions": postMethod, "/v3/voice-rooms/{roomId}/tickets": postMethod,
		"/v2/voice-rooms/{roomId}": getMethod, "/v2/voice-rooms/{roomId}:end": postMethod, "/v2/voice-rooms/{roomId}:leave": postMethod,
	}
	paths := make([]string, 0, len(methods))
	for path := range methods {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	routes := SocialContractRoutes()
	seen := map[string]bool{}
	for _, route := range routes {
		seen[route.Method+" "+route.Template] = true
	}
	frozen := FrozenConnectedSurface()[socialServiceName]
	frozenPaths := make([]string, 0, len(frozen))
	for path := range frozen {
		frozenPaths = append(frozenPaths, path)
	}
	sort.Strings(frozenPaths)
	for _, path := range frozenPaths {
		for _, method := range strings.Fields(frozen[path]) {
			if !seen[method+" "+path] {
				routes = append(routes, ConnectedRoute{Method: method, Template: path, Authenticate: method != http.MethodGet, Optional: method == http.MethodGet})
				seen[method+" "+path] = true
			}
		}
	}
	for _, path := range paths {
		for _, method := range strings.Fields(methods[path]) {
			routes = append(routes, ConnectedRoute{Method: method, Template: path, Authenticate: true})
		}
	}
	return routes
}

// LivestreamConnectedRoutes is the media/livestream v2 surface the web calls
// (internal/streams RegisterRoutes /v2 group). The directory, pool listing and
// stream document are public reads; viewer sessions and heartbeats are
// anonymous viewer writes authorised by the viewer-session token in their
// body (no wallet session); every broadcaster action requires a session.
func LivestreamConnectedRoutes() []ConnectedRoute {
	return []ConnectedRoute{
		{Method: http.MethodGet, Template: "/v2/streams/live"},
		{Method: http.MethodGet, Template: "/v2/streams/pool/{poolAddress}"},
		{Method: http.MethodGet, Template: "/v2/streams/operations/{operationId}", Authenticate: true},
		{Method: http.MethodGet, Template: "/v2/streams/{streamId}"},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}/viewer-session"},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}/heartbeat"},
		{Method: http.MethodPost, Template: streamsRoute, Authenticate: true},
		{Method: http.MethodPatch, Template: "/v2/streams/{streamId}", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}/go-live", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}/end", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}/rotate-key", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}/restore", Authenticate: true},
		{Method: http.MethodGet, Template: "/v2/streams/{streamId}/key-rotation", Authenticate: true},
		{Method: http.MethodGet, Template: "/v2/streams/{streamId}/private-playback", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}/ingest-credential", Authenticate: true},
		{Method: http.MethodPost, Template: ingestKeyRoute, Authenticate: true},
		{Method: http.MethodGet, Template: streamsRoute},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}:start", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}:end", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}/viewers:heartbeat"},
		{Method: http.MethodPost, Template: "/v2/provider-webhooks/livepeer"},
	}
}

func UserConnectedRoutes() []ConnectedRoute {
	methods := map[string]string{
		profileRoute: getPatchMethods, batchProfilesRoute: getMethod, v3ProfileRoute: getPatchMethods, "/v3/me/preferences": getPatchMethods, "/v1/me/preferences": getPatchMethods, "/v2/me/preferences": getPatchMethods, "/v1/me/devices": getMethod, "/v1/me/devices/{deviceId}": putDeleteMethods, "/v1/me/exports": postMethod, "/v1/me/exports/{jobId}": getMethod, "/v2/me/exports/{jobId}/download": getMethod, "/v1/me/deletion-requests": postMethod, "/v1/me/deletion-requests/{jobId}": getMethod, "/v1/profiles/{wallet}": getPatchMethods,
		"/v1/me/watchlist": getMethod, "/v1/me/watchlist/{poolAddress}": putDeleteMethods,
		"/v2/me/watchlist": getMethod, "/v2/me/watchlist/{poolAddress}": putDeleteMethods,
		"/v2/me/devices/{deviceId}": putDeleteMethods, "/v2/me/exports": postMethod, "/v2/me/exports/{jobId}": getMethod, "/v2/me/deletion-requests": postMethod, "/v2/me/deletion-requests/{jobId}": getMethod,
	}
	paths := make([]string, 0, len(methods))
	for path := range methods {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	routes := []ConnectedRoute{}
	for _, path := range paths {
		for _, method := range strings.Fields(methods[path]) {
			public := method == http.MethodGet && (path == v3ProfileRoute || path == profileRoute || path == batchProfilesRoute)
			routes = append(routes, ConnectedRoute{Method: method, Template: path, Authenticate: !public})
		}
	}
	return routes
}

// DevelopmentSocialRoutes is the social surface the development gateway
// forwards verbatim to the social upstream: the production social allowlist
// (SocialConnectedRoutes, which includes SocialContractRoutes) plus the
// social upstream routes of the production gateway runtime document
// (comments, reports, callouts, bounties, rooms) and the bounty reads the web
// issues. GET routes are anonymous-capable (Authenticate false: the actor is
// attached when a session is present); every other method requires a session.
func DevelopmentSocialRoutes() []ConnectedRoute {
	extra := map[string]string{
		"/v2/pools/{poolAddress}/comments": getPostMethods, "/v2/comments/{commentId}": patchDeleteMethods, "/v2/comments/{commentId}/likes/me": putDeleteMethods,
		"/v2/reports": postMethod, "/v2/callouts": postMethod, "/v2/pools/{poolAddress}/callouts": getMethod,
		"/v2/bounties": getPostMethods, "/v2/bounties/stats": getMethod, "/v2/bounties/{bountyId}": getMethod, "/v2/bounties/{bountyId}/submissions": getPostMethods, "/v2/bounties/{bountyId}/submissions/{submissionId}:accept": postMethod,
	}
	routes := []ConnectedRoute{}
	seen := map[string]bool{}
	add := func(method, template string) {
		if seen[method+" "+template] {
			return
		}
		seen[method+" "+template] = true
		routes = append(routes, ConnectedRoute{Method: method, Template: template, Authenticate: method != http.MethodGet, Optional: method == http.MethodGet})
	}
	for _, route := range SocialConnectedRoutes() {
		add(route.Method, route.Template)
	}
	paths := make([]string, 0, len(extra))
	for path := range extra {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		for _, method := range strings.Fields(extra[path]) {
			add(method, path)
		}
	}
	return routes
}

// DevelopmentUserRoutes are the account owner writes the development gateway
// forwards verbatim to media/user, which serves them outside production over
// its legacy profile store: the web profile editor's
// PATCH /v3/profiles/{wallet}. They require a session here, and media/user
// re-authenticates the forwarded bearer against the identity authority.
func DevelopmentUserRoutes() []ConnectedRoute {
	return []ConnectedRoute{{Method: http.MethodPatch, Template: v3ProfileRoute, Authenticate: true}}
}

// The media/metadata routes behind the gateway beyond the frozen v2 media
// surface: the /v3 metadata publication the token-create flow drives
// (PublicationService.RegisterRoutes) and the legacy public reads
// (ProductionLegacyReads.RegisterRoutes) the web's metadata client issues
// against NEXT_PUBLIC_METADATA_API_URL, which is the gateway origin because
// metadata's production listener is mTLS-only.
const (
	metadataPublicationAuthority = "/v3/metadata-publication-authority"
	metadataPublications         = "/v3/tokens/{tokenAddress}/metadata-publications"
	metadataPublication          = "/v3/tokens/{tokenAddress}/metadata-publications/{operationId}"
	metadataLogo                 = "/logo/{file}"
	// MetadataPublicationBodyLimit is media/metadata's PublicationBodyLimit
	// (V2BodyLimit 32 KiB + 4 KiB): the edge refuses a larger publication
	// body before it crosses the private channel.
	MetadataPublicationBodyLimit = 32<<10 + 4096
	// metadataLogoLimit bounds a relayed legacy logo (the approved 512px PNG
	// variant; metadata's upload ceiling is 6 MiB).
	metadataLogoLimit = 6 << 20
)

// MetadataConnectedRoutes is the media/metadata surface the gateway adds to
// the metadata upstream: publication authority, enqueue and status require
// the creator's session (metadata re-authenticates the forwarded bearer and,
// for the enqueue, verifies the gateway's actor assertion and the creator's
// operation envelope); the legacy metadata/logo/banner reads are public.
func MetadataConnectedRoutes() []ConnectedRoute {
	return []ConnectedRoute{
		{Method: http.MethodGet, Template: metadataPublicationAuthority, Authenticate: true},
		{Method: http.MethodPost, Template: metadataPublications, Authenticate: true},
		{Method: http.MethodGet, Template: metadataPublication, Authenticate: true},
		{Method: http.MethodGet, Template: "/metadata/batch"},
		{Method: http.MethodGet, Template: "/metadata/{tokenAddress}"},
		{Method: http.MethodGet, Template: metadataLogo},
		{Method: http.MethodGet, Template: "/banner/{file}"},
	}
}

// slowRoute is a route whose upstream work is bounded by more than the
// default 3s: metadata publication admission authenticates the session,
// confirms the identity and verifies the creator's committed operation
// against the chain inside one transaction.
func (p *Connected) slowRoute(route *ConnectedRoute) bool {
	return p.audience == metadataServiceName && route.Method == http.MethodPost && route.Template == metadataPublications
}

// MetadataActorTransport is the header pair media/metadata's private channel
// (internal/app metadataPrivateChannel) reads the gateway's actor assertion
// from. Every other upstream reads the shared actor headers.
var MetadataActorTransport = sharedauth.AssertionTransport{AssertionHeader: "X-Internal-Assertion", SignatureHeader: "X-Internal-Signature"}

func connectedAssertionAudience(upstream, template string) string {
	if upstream == userServiceName {
		return "account"
	}
	return upstream
}

func actorAssertionTransport(upstream string) sharedauth.AssertionTransport {
	if upstream == metadataServiceName {
		return MetadataActorTransport
	}
	return sharedauth.AssertionTransport{AssertionHeader: sharedauth.ActorAssertionHeader, SignatureHeader: sharedauth.ActorSignatureHeader}
}
