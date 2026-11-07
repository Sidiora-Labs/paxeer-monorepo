package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/sei-protocol/seictl/sidecar/engine"
	"github.com/sei-protocol/seilog"
)

var serverLog = seilog.NewLogger("seictl", "sidecar", "server")

const (
	ed25519PrivKeyLen   = 64 // seed (32) + public key (32)
	ed25519PubKeyOffset = 32
	cometbftAddressLen  = 20 // hex(SHA256(pubkey)[:20])
)

// Server is the HTTP API for the sidecar.
type Server struct {
	addr      string
	homeDir   string
	engine    *engine.Engine
	mux       *http.ServeMux
	handler   http.Handler // mux, possibly wrapped by trustedHeaderMiddleware
	protected bool
	configErr error
	admission *TaskStore
}

// TaskRequest is the JSON body for POST /v0/tasks. When ID is provided,
// the engine uses it as the task's canonical identifier; otherwise a
// random UUID is generated.
type TaskRequest struct {
	ID     string         `json:"id,omitempty"`
	Type   string         `json:"type"`
	Params map[string]any `json:"params,omitempty"`
}

func (r *TaskRequest) UnmarshalJSON(data []byte) error {
	var wire struct {
		ID     string          `json:"id,omitempty"`
		Type   string          `json:"type"`
		Params json.RawMessage `json:"params,omitempty"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	params, err := decodeTaskParams(wire.Params)
	if err != nil {
		return err
	}
	r.ID, r.Type, r.Params = wire.ID, wire.Type, params
	return nil
}

func decodeTaskParams(data []byte) (map[string]any, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	params := make(map[string]any, len(raw))
	for k, v := range raw {
		params[k] = v
	}
	return params, nil
}

// ErrorResponse is a standard JSON error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
}

// NewServer wires a Server to the engine. authnMode must come from
// AuthnMode() so the env read and validation happen once at startup.
func NewServer(addr string, eng *engine.Engine, homeDir, authnMode string) *Server {
	if authnMode == "" {
		authnMode = AuthnModeTrustedHeader
	}
	s := &Server{
		addr:      addr,
		homeDir:   homeDir,
		engine:    eng,
		mux:       http.NewServeMux(),
		protected: authnMode == AuthnModeTrustedHeader,
		configErr: validateListener(addr, authnMode),
	}
	s.protected = s.protected && s.configErr == nil
	if eng != nil && s.protected {
		if store, ok := eng.Config.Checkpointer.(*TaskStore); ok {
			s.admission = store
			if err := store.bindEngine(eng); err != nil {
				s.configErr = err
			}
		}
	}
	s.mux.HandleFunc("GET /v0/healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /v0/startupz", s.handleHealthz)
	s.mux.HandleFunc("GET /v0/livez", s.handleLivez)
	s.mux.HandleFunc("GET /v0/status", s.handleStatus)
	s.mux.Handle("GET /v0/metrics", promhttp.Handler())
	s.mux.HandleFunc("GET /v0/node-id", s.handleNodeID)
	s.mux.HandleFunc("POST /v0/tasks", s.handlePostTask)
	s.mux.HandleFunc("GET /v0/tasks", s.handleListTasks)
	s.mux.HandleFunc("GET /v0/tasks/{id}", s.handleGetTask)
	s.mux.HandleFunc("DELETE /v0/tasks/{id}", s.handleDeleteTask)

	s.handler = s.mux
	if authnMode == AuthnModeTrustedHeader {
		s.handler = trustedHeaderMiddleware(s.mux)
	}
	if s.configErr != nil {
		s.handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusServiceUnavailable, s.configErr.Error())
		})
	}
	return s
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck // best-effort response
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	if !s.engine.Healthz() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleLivez follows the kube-apiserver convention: bare 200/503
// status for kubelet probes. ?verbose=1 includes the underlying
// store error for human triage.
func (s *Server) handleLivez(w http.ResponseWriter, r *http.Request) {
	err := s.engine.Livez()
	if err == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.URL.Query().Get("verbose") == "1" {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.Status())
}

func (s *Server) handlePostTask(w http.ResponseWriter, r *http.Request) {
	if !s.protected {
		writeError(w, http.StatusForbidden, "task submission requires an authenticated loopback listener")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxTaskRequestBytes)
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "task request exceeds 64 KiB")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var req TaskRequest
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "request body must contain exactly one JSON object")
		return
	}
	if req.Type == "" {
		writeError(w, http.StatusBadRequest, "type is required")
		return
	}

	task := engine.Task{ID: req.ID, Type: engine.TaskType(req.Type), Params: req.Params}

	if s.admission == nil {
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusServiceUnavailable, ErrTaskAdmissionUnavailable.Error())
		return
	}
	id, err := s.admission.SubmitQueued(task)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrTaskOverload) {
			status = http.StatusTooManyRequests
		}
		if errors.Is(err, ErrTaskAdmissionUnavailable) {
			status = http.StatusServiceUnavailable
		}
		if status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
			w.Header().Set("Retry-After", "1")
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (s *Server) handleListTasks(w http.ResponseWriter, _ *http.Request) {
	results, err := s.engine.RecentResults()
	if err != nil {
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusServiceUnavailable, "failed to list tasks; retry")
		return
	}
	if results == nil {
		results = []engine.TaskResult{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing task ID")
		return
	}
	result, err := s.engine.GetResult(id)
	if err != nil {
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusServiceUnavailable, "failed to get task; retry")
		return
	}
	if result == nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	if !s.protected {
		writeError(w, http.StatusForbidden, "task deletion requires an authenticated loopback listener")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing task ID")
		return
	}
	deleted, err := s.engine.RemoveResult(id)
	if err != nil {
		// A failed delete leaves the row recoverable (RemoveResult cancels the
		// work but keeps the row and its registry entry), so the store hiccup is
		// transient and the DELETE is safe to retry. 503 + Retry-After signals
		// that to automated clients, which treat a bare 500 as fatal.
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusServiceUnavailable, "failed to delete task; retry")
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleNodeID reads node_key.json from the home directory and returns the
// Tendermint node ID. The node ID is hex(SHA256(ed25519_pubkey)[:20]), matching
// CometBFT's p2p.PubKeyToID derivation.
func (s *Server) handleNodeID(w http.ResponseWriter, _ *http.Request) {
	keyPath := filepath.Join(s.homeDir, "config", "node_key.json")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError,
			"node_key.json not found — generate-identity may not have run yet")
		return
	}

	var keyFile struct {
		PrivKey struct {
			Value string `json:"value"`
		} `json:"priv_key"`
	}
	if err := json.Unmarshal(data, &keyFile); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse node_key.json")
		return
	}

	keyBytes, err := base64.StdEncoding.DecodeString(keyFile.PrivKey.Value)
	if err != nil || len(keyBytes) != ed25519PrivKeyLen {
		writeError(w, http.StatusInternalServerError, "invalid Ed25519 key in node_key.json")
		return
	}

	pubKey := keyBytes[ed25519PubKeyOffset:]
	hash := sha256.Sum256(pubKey)
	nodeID := hex.EncodeToString(hash[:cometbftAddressLen])

	writeJSON(w, http.StatusOK, map[string]string{"nodeId": nodeID})
}

// ListenAndServe starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	if s.configErr != nil {
		return s.configErr
	}
	srv := &http.Server{
		Addr:              s.addr,
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		// Cap impersonation-header amplification in trusted-header
		// mode (proxy injects X-Remote-User / Group / Extra-*).
		// Go's default is 1MB.
		MaxHeaderBytes: 32 * 1024,
	}

	go func() {
		<-ctx.Done()
		// 25s leaves a 5s buffer under K8s's default 30s
		// terminationGracePeriodSeconds.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			serverLog.Warn("graceful shutdown failed", "err", err)
		}
	}()

	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
