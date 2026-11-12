package control

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPayoutProviderRejectsUnconfirmedResponses(t *testing.T) {
	for _, sample := range []struct {
		name   string
		status int
		body   string
	}{
		{"server refusal", http.StatusServiceUnavailable, `{"state":"pending"}`},
		{"unknown state", http.StatusOK, `{"state":"sent"}`},
		{"missing reference", http.StatusOK, `{"state":"confirmed"}`},
		{"invalid reference", http.StatusOK, `{"state":"confirmed","reference":"0x1234"}`},
		{"extra field", http.StatusOK, `{"state":"pending","unbound":true}`},
		{"duplicate field", http.StatusOK, `{"state":"pending","state":"confirmed"}`},
		{"trailing object", http.StatusOK, `{"state":"pending"} {}`},
		{"oversized response", http.StatusOK, strings.Repeat("x", 65537)},
	} {
		t.Run(sample.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.Header.Get("Idempotency-Key") != "bounty.response" {
					t.Errorf("payout lookup: %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(sample.status)
				if _, err := w.Write([]byte(sample.body)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			provider, err := NewPayoutProvider(server.URL, strings.Repeat("p", 32), "payout controller", custody, repairM1020Transport(server))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := provider.Lookup(context.Background(), "bounty.response"); !errors.Is(err, Unavailable) {
				t.Fatalf("unconfirmed payout response: %v", err)
			}
		})
	}
}

func TestRevocationProviderRequiresBoundAcknowledgement(t *testing.T) {
	incident := Incident{ID: "incident-response", ServiceIdentity: "service-response"}
	for _, sample := range []struct {
		name   string
		status int
		body   string
	}{
		{"server refusal", http.StatusServiceUnavailable, `{}`},
		{"missing acknowledgements", http.StatusOK, `{"incident":"incident-response","identity":"service-response"}`},
		{"different identity", http.StatusOK, `{"incident":"incident-response","identity":"other","rotation":"r1","integrity":"i1","secretsRevoked":true,"cachesInvalidated":true}`},
		{"unknown field", http.StatusOK, `{"unbound":true}`},
		{"duplicate field", http.StatusOK, `{"incident":"a","incident":"b"}`},
		{"oversized response", http.StatusOK, strings.Repeat("x", 65537)},
	} {
		t.Run(sample.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut || r.Header.Get("Idempotency-Key") != incident.ID {
					t.Errorf("revocation request: %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(sample.status)
				if _, err := w.Write([]byte(sample.body)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			provider, err := NewRevocationProvider(server.URL, strings.Repeat("r", 32), repairM1020Transport(server))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := provider.Revoke(context.Background(), incident); !errors.Is(err, Unavailable) {
				t.Fatalf("unbound revocation response: %v", err)
			}
		})
	}
}

func TestReplayProviderRequiresBoundOwnerResult(t *testing.T) {
	payload := ReplayPayload{ChainID: "125", FromHeight: "1", ToHeight: "2", Projection: "candles", DryRun: true}
	jobID := id()
	job := ReplayJob{ActionID: id(), ActionKey: "replay-result", Kind: controlReplay, Scope: "service", PayloadHash: digest(payload), PayloadReference: "replay-result", CompletionIdentity: "control-replay:" + jobID}
	request := ReplayExecution{JobID: jobID, Job: job, Payload: payload}
	for _, sample := range []struct {
		name   string
		status int
		body   func() string
	}{
		{"server refusal", http.StatusServiceUnavailable, func() string { return `{}` }},
		{"empty result", http.StatusOK, func() string { return `{}` }},
		{"unknown field", http.StatusOK, func() string { return `{"unbound":true}` }},
		{"duplicate field", http.StatusOK, func() string { return `{"State":"succeeded","State":"failed"}` }},
		{"different job", http.StatusOK, func() string {
			wire, _ := json.Marshal(ReplayResult{ActionID: job.ActionID, JobID: id(), PayloadHash: job.PayloadHash, PayloadReference: job.PayloadReference, CompletionIdentity: job.CompletionIdentity, State: controlSucceeded, Reference: "checkpoint:3"})
			return string(wire)
		}},
		{"oversized response", http.StatusOK, func() string { return strings.Repeat("x", 65537) }},
	} {
		t.Run(sample.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut || r.Header.Get("Idempotency-Key") != job.CompletionIdentity {
					t.Errorf("replay request: %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(sample.status)
				if _, err := w.Write([]byte(sample.body())); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			provider, err := NewReplayProvider(server.URL, strings.Repeat("r", 32), repairM1020Transport(server))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := provider.Execute(context.Background(), request); !errors.Is(err, Unavailable) {
				t.Fatalf("unbound replay result: %v", err)
			}
		})
	}
}
