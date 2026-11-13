package upload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/cleanup"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	gatewayauth "github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/proxy"
	gatewaystorage "github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/storage"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

var connectedAssetPath = regexp.MustCompile(`^/v2/(tokens|accounts)/0x[0-9a-f]{40}/assets/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type ConnectedDeps struct {
	Upstream              *proxy.Connected
	Identity              proxy.RequestIdentityAuthority
	Admission             *Admission
	IngressBytesPerSecond int64
	Observer              *gatewaystorage.MediaObserver
}
type Connected struct {
	upstream       *proxy.Connected
	identity       proxy.RequestIdentityAuthority
	admission      *Admission
	slots          chan struct{}
	bytesPerSecond int64
	observer       *gatewaystorage.MediaObserver
}

func NewConnected(d ConnectedDeps) (*Connected, error) {
	if d.Upstream == nil || d.Admission == nil || d.Identity == nil || d.IngressBytesPerSecond < 32768 || d.IngressBytesPerSecond > 1<<30 {
		return nil, errors.New("upload dependencies unavailable")
	}
	return &Connected{d.Upstream, d.Identity, d.Admission, make(chan struct{}, d.Admission.global), d.IngressBytesPerSecond, d.Observer}, nil
}
func (h *Connected) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.observer != nil {
		observed := &uploadResponse{ResponseWriter: w}
		w = observed
		defer func() {
			outcome := "unavailable"
			switch observed.status {
			case 200:
				outcome = "approved"
			case 202:
				outcome = "quarantined"
			case 401, 403:
				outcome = "unauthenticated"
			case 429:
				outcome = "rate_limited"
			case 400, 408, 413, 415, 422:
				outcome = "rejected"
			}
			_ = h.observer.RecordUpload(outcome)
		}()
	}
	w.Header().Set("Cache-Control", "no-store")
	requestID := w.Header().Get("X-Request-ID")
	if _, err := uuid.Parse(requestID); err != nil || len(requestID) != 36 {
		requestID = r.Header.Get("X-Request-ID")
		if _, err = uuid.Parse(requestID); err != nil || len(requestID) != 36 {
			requestID = uuid.NewString()
		}
	}
	w.Header().Set("X-Request-ID", requestID)
	if r.URL == nil || r.URL.RawPath != "" || r.Method != "PUT" || !connectedAssetPath.MatchString(r.URL.Path) {
		sharedhttp.NotFoundHandler()(w, r)
		return
	}
	identityContext, identityCancel := context.WithTimeout(r.Context(), 3*time.Second)
	p, e := h.identity.AuthenticateRequest(identityContext, r)
	identityCancel()
	if e != nil {
		var public *gatewayauth.PublicError
		if errors.As(e, &public) {
			var delay time.Duration
			if public.RetryAfterMS != nil {
				delay = time.Duration(*public.RetryAfterMS) * time.Millisecond
			}
			if delay > 0 {
				w.Header().Set("Retry-After", strconv.FormatInt(int64((delay+time.Second-1)/time.Second), 10))
			}
			sharedhttp.WriteErrorEnvelope(w, public.Status, public.Code, "upload identity unavailable", public.Retryable, delay, nil, nil)
		} else {
			status := http.StatusServiceUnavailable
			var authority *sharedauth.IdentityAuthorityError
			if errors.As(e, &authority) {
				status = authority.StatusCode()
			} else if errors.Is(e, sharedauth.ErrScopeDenied) {
				status = http.StatusForbidden
			}
			writeConnectedRefusal(w, status, "upload identity unavailable", 0)
		}
		return
	}
	if !walletPattern.MatchString(p.Wallet) {
		writeConnectedRefusal(w, http.StatusForbidden, "upload identity unavailable", 0)
		return
	}
	if r.ContentLength > 6<<20 {
		sharedhttp.WriteRequestBodyError(w, http.StatusRequestEntityTooLarge, sharedhttp.RuleMaxBytes, "upload exceeds size limit")
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		w.Header().Set("Retry-After", "1")
		writeConnectedRefusal(w, http.StatusServiceUnavailable, "upload busy", time.Second)
		return
	}
	release, e := h.admission.Acquire(r.Context(), p.Wallet)
	if e != nil {
		w.Header().Set("Retry-After", "60")
		status := 503
		if errors.Is(e, ErrBusy) {
			status = 429
		}
		writeConnectedRefusal(w, status, "upload admission unavailable", time.Minute)
		return
	}
	defer release()
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	now := time.Now()
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(now.Add(5 * time.Second))
	defer func() { cleanup.Report("reset read deadline", controller.SetReadDeadline(time.Time{})) }()
	r.Body = http.MaxBytesReader(w, &aggregateIngressReader{ReadCloser: &ingressReader{ReadCloser: r.Body, controller: controller, window: now, deadline: now.Add(30 * time.Second)}, ctx: ctx, db: h.admission.db, key: h.admission.ns + ":upload:bytes", limit: h.bytesPerSecond}, 6<<20)
	response := &connectedUploadResponse{ResponseWriter: w}
	h.upstream.ServeHTTP(response, r)
	response.finish()
}

type aggregateIngressReader struct {
	io.ReadCloser
	ctx   context.Context
	db    *redis.Client
	key   string
	limit int64
}

func (r *aggregateIngressReader) Read(buffer []byte) (int, error) {
	if len(buffer) > 32768 {
		buffer = buffer[:32768]
	}
	reserved := int64(len(buffer))
	if reserved == 0 {
		return 0, nil
	}
	for {
		permitted, e := r.db.Eval(r.ctx, `local t=redis.call('TIME');local now=tonumber(t[1])*1000+math.floor(tonumber(t[2])/1000);local last=tonumber(redis.call('HGET',KEYS[1],'time') or now);local cap=tonumber(ARGV[1]);local tokens=tonumber(redis.call('HGET',KEYS[1],'tokens') or cap);tokens=math.min(cap,tokens+math.max(0,now-last)*cap/1000);if tokens<tonumber(ARGV[2]) then redis.call('HSET',KEYS[1],'time',now,'tokens',tokens);redis.call('PEXPIRE',KEYS[1],2000);return 0 end;redis.call('HSET',KEYS[1],'time',now,'tokens',tokens-tonumber(ARGV[2]));redis.call('PEXPIRE',KEYS[1],2000);return 1`, []string{r.key}, r.limit, reserved).Int()
		if e != nil {
			return 0, ErrBusy
		}
		if permitted == 1 {
			break
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-r.ctx.Done():
			timer.Stop()
			return 0, r.ctx.Err()
		case <-timer.C:
		}
	}
	return r.ReadCloser.Read(buffer)
}

type uploadResponse struct {
	http.ResponseWriter
	status int
}

func (w *uploadResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *uploadResponse) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.ResponseWriter.Write(data)
}
func (w *uploadResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func writeConnectedRefusal(w http.ResponseWriter, status int, message string, delay time.Duration) {
	code := sharedhttp.CodeDependencyUnavailable
	retryable := status >= 500 || status == http.StatusRequestTimeout || status == http.StatusTooManyRequests
	switch status {
	case http.StatusUnauthorized:
		code = "AUTH_EXPIRED"
	case http.StatusForbidden:
		code = "SCOPE_DENIED"
	case http.StatusNotFound:
		code = sharedhttp.CodeNotFound
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		code = sharedhttp.CodeFieldInvalid
	case http.StatusTooManyRequests:
		code = sharedhttp.CodeRateLimited
	}
	sharedhttp.WriteErrorEnvelope(w, status, code, message, retryable, delay, nil, nil)
}

// The private proxy can refuse after admission (including while reading an
// unknown-length body). Keep those refusals on the frozen upload interface too.
type connectedUploadResponse struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *connectedUploadResponse) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	if status < 400 {
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *connectedUploadResponse) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if w.status >= 400 {
		return w.body.Write(data)
	}
	return w.ResponseWriter.Write(data)
}
func (w *connectedUploadResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *connectedUploadResponse) finish() {
	if w.status < 400 {
		return
	}
	if envelope, err := v2.DecodeError(w.body.Bytes()); err == nil {
		w.Header().Set("X-Request-ID", envelope.RequestId)
		w.ResponseWriter.WriteHeader(w.status)
		_, _ = w.ResponseWriter.Write(w.body.Bytes())
		return
	}
	var delay time.Duration
	if seconds, err := strconv.ParseInt(w.Header().Get("Retry-After"), 10, 32); err == nil && seconds > 0 {
		delay = time.Duration(seconds) * time.Second
	}
	if w.status == http.StatusRequestEntityTooLarge {
		sharedhttp.WriteRequestBodyError(w.ResponseWriter, w.status, sharedhttp.RuleMaxBytes, "upload exceeds size limit")
		return
	}
	writeConnectedRefusal(w.ResponseWriter, w.status, "upload request could not be completed", delay)
}
