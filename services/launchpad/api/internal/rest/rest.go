// Package rest is the thin REST snapshot surface of core/api.
//
// By design the gateway is PUSH-FIRST: live updates ride the WSS/SSE multiplexer
// (internal/ws, internal/sse), never client polling. REST is reserved for the
// frozen v2 bootstrap SNAPSHOTS a client needs once before it starts consuming
// deltas (RegisterProductionReads, RegisterAccountingV2WithAdmission). The
// unversioned legacy routes (/udf/*, /stats/*, /rankings/*, /platform/metrics,
// /bff/token/*) were removed: they had no frozen contract and core/api's
// production roles cannot read the legacy stats tables they served.
package rest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
)

func RegisterV2SearchProxy(r chi.Router, origin string) error {
	target, err := url.Parse(origin)
	if err != nil || target.Host == "" || target.User != nil || target.RawQuery != "" || target.Fragment != "" || target.Path != "" && target.Path != "/" || (target.Scheme != literalHttps && target.Scheme != literalHttp) {
		return fmt.Errorf("stats_service_origin_invalid")
	}
	proxy := &httputil.ReverseProxy{Transport: &http.Transport{ResponseHeaderTimeout: 2 * time.Second, MaxIdleConnsPerHost: 8}}
	proxy.Rewrite = func(req *httputil.ProxyRequest) {
		req.SetURL(target)
		req.SetXForwarded()
		req.Out.Header.Del("Authorization")
		req.Out.Header.Del("Cookie")
		req.Out.Header.Del("X-Actor-Wallet")
		req.Out.Header.Del("X-Forwarded-Host")
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		WriteV2APIError(w, req, http.StatusServiceUnavailable, literalDEPENDENCYUNAVAILABLE)
	}
	r.Method(http.MethodGet, "/v2/search", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		id := responseRequestID(w)
		timed := http.TimeoutHandler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Request-ID", id)
			proxy.ServeHTTP(w, req)
		}), 3*time.Second, "")
		ctx, cancel := context.WithTimeout(req.Context(), 3*time.Second)
		defer cancel()
		req = req.WithContext(ctx)
		timed.ServeHTTP(&searchTimeoutResponse{ResponseWriter: w, request: req}, req)
	}))
	return nil
}

// TimeoutHandler buffers the upstream response, including its body, but writes
// an HTML error on expiry. Replace that response at the write boundary so the
// timeout uses the same envelope and headers as a transport failure.
type searchTimeoutResponse struct {
	http.ResponseWriter
	request  *http.Request
	timedOut bool
}

func (w *searchTimeoutResponse) WriteHeader(status int) {
	if status == http.StatusServiceUnavailable && w.request.Context().Err() != nil {
		w.timedOut = true
		WriteV2APIError(w.ResponseWriter, w.request, status, literalDEPENDENCYUNAVAILABLE)
		return
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *searchTimeoutResponse) Write(body []byte) (int, error) {
	if w.timedOut {
		return len(body), nil
	}
	return w.ResponseWriter.Write(body)
}
