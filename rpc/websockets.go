package evmrpc

import "net/http"

type wsConnectionHandler struct {
	underlying http.Handler
}

func (h *wsConnectionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Attempts include JWT, origin and protocol rejections. Established
	// connections are counted only after Upgrade succeeds in rpcstack.go.
	recordWebsocketConnectAttempt(r.Context())
	h.underlying.ServeHTTP(w, r)
}

func NewWSConnectionHandler(handler http.Handler) http.Handler {
	return &wsConnectionHandler{underlying: handler}
}
