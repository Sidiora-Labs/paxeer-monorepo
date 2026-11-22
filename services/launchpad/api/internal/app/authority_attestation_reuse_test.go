package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/capabilities"
	"github.com/ethereum/go-ethereum/common"
)

func TestAuthorityAttestationReuse(t *testing.T) {
	a, reader, head := newAuthorityReadinessState(t)
	counter := &authorityAttestationCounter{Reader: reader}
	a.reader = counter
	engine, err := capabilities.NewPreflightEngine(counter, a.config.Manifest, developmentPoolRuntime, developmentSerialInterface, authorityMaxAge, authorityAttestationTimeout)
	if err != nil {
		t.Fatal(err)
	}
	a.engine = engine
	commit := productionAuthorityHeads(t, a)
	commit(reader.header, "healthy")
	a.origins = map[string]bool{webOrigin: true}
	router := productionAuthorityRouter(t, a)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/network/authority", nil))
	var envelope authorityEnvelope
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &envelope) != nil || envelope.CommittedHead != head {
		t.Fatalf("authority = %d %s", response.Code, response.Body)
	}
	before := reader.reads.Load()
	if before == 0 || counter.attestations.Load() != 1 {
		t.Fatal("envelope did not read its identity once")
	}
	assertProductionPreflight(t, router, productionPreflightRequest(t, a, "trade", capabilities.Trading, common.HexToHash(head.Hash)), reader.header.Hash())
	// Head admission reads its header; the zero pool requires no pool reads.
	if reader.reads.Load() != before+1 || counter.attestations.Load() != 1 {
		t.Fatal("preflight reread the envelope identity")
	}
	before = reader.reads.Load()
	rebuilt, err := a.envelope(t.Context())
	if err != nil || rebuilt.CommittedHead != head || !rebuilt.Attestation.ObservedAt.Equal(envelope.Attestation.ObservedAt) {
		t.Fatalf("rebuilt authority = %+v, %v", rebuilt, err)
	}
	if reader.reads.Load() != before || counter.attestations.Load() != 1 {
		t.Fatal("rebuilt envelope reread its identity")
	}
}
