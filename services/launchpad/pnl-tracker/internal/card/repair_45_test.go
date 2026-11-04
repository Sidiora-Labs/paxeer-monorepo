package card_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/card"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
)

func TestRepair_45_LegacyMintRetired(t *testing.T) {
	ctx := context.Background()
	// No pool or clock is configured: any persistence or snapshot capture
	// would fail rather than hiding a dependency behind a test double.
	st := store.New(nil)
	svc := card.New(st, "https://sidiora.fun", "https://og.example", nil)
	for _, owner := range []string{"0xowner", "0xnobody"} {
		minted, err := svc.Mint(ctx, owner, "0xpool")
		if minted != nil || !errors.Is(err, card.ErrMintRetired) {
			t.Fatalf("legacy mint for %s = %+v, %v; want nil, ErrMintRetired", owner, minted, err)
		}
	}

	router := chi.NewRouter()
	httpapi.RegisterCards(router, httpapi.CardDeps{Cards: svc})
	server := httptest.NewServer(router)
	defer server.Close()
	client := server.Client()
	for _, owner := range []string{"0xowner", "0xnobody"} {
		body, err := json.Marshal(map[string]string{"ownerAddress": owner, "poolAddress": "0xpool"})
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Post(server.URL+"/pnl/cards", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST live mint handler: %v", err)
		}
		payload, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusInternalServerError {
			t.Fatalf("retired mint status = %d, want current handler rejection 500: %s", response.StatusCode, payload)
		}
		var rejected sharedhttp.ErrorResponse
		if err := json.Unmarshal(payload, &rejected); err != nil {
			t.Fatal(err)
		}
		if rejected.StatusCode != response.StatusCode || rejected.Error != "Internal Server Error" || rejected.Message != "An unexpected error occurred" {
			t.Fatalf("unexpected mint rejection: %+v", rejected)
		}
		if response.Header.Get("Location") != "" || strings.Contains(string(payload), "/pnl/") {
			t.Fatalf("retired mint advertised a landing URL: %s", payload)
		}
		var failure map[string]json.RawMessage
		if err := json.Unmarshal(payload, &failure); err != nil {
			t.Fatalf("decode mint rejection: %v (body=%s)", err, payload)
		}
		for _, field := range []string{"cardId", "shortCode", "shareUrl", "ogUrl", "snapshot"} {
			if _, exists := failure[field]; exists {
				t.Fatalf("retired mint disclosed %s: %s", field, payload)
			}
		}
	}
	if st.Pool() != nil {
		t.Fatal("retired mint unexpectedly initialized persistence")
	}
}

func repair45ArchiveRow(t *testing.T) (store.CardRow, card.Snapshot) {
	t.Helper()
	price := "2500000"
	snapshot := card.Snapshot{
		Version: 1, OwnerAddress: "0xowner", PoolAddress: "0xpool", TokenAddress: "0xtoken",
		TokenSymbol: "OLD", TokenName: "Archived token", CapturedAt: 1234,
		Position: card.Position{
			TotalUsdlSpent: "1000000", TotalTokensBought: oneToken,
			TotalUsdlReceived: "0", TotalTokensSold: "0", AvgCostBasis: "1000000",
			CurrentHoldings: oneToken, RealizedPnlUsdl: "0", LastTradeTs: 100, TradeCount: 1,
		},
		Market: card.Market{PriceWad: &price},
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	row := store.CardRow{
		CardID: "00112233445566778899aabbccddeeff", ShortCode: "old1234",
		OwnerAddress: snapshot.OwnerAddress, PoolAddress: snapshot.PoolAddress, TokenAddress: snapshot.TokenAddress,
		Snapshot: payload, CreatedAt: snapshot.CapturedAt,
	}
	return row, snapshot
}

func TestRepair_45_ArchivedDisclosurePreserved(t *testing.T) {
	row, snapshot := repair45ArchiveRow(t)
	originalPayload := bytes.Clone(row.Snapshot)
	// Exercise the archive decoder used by Service.Get, then the same JSON
	// encoding and consumer decoding used by the HTTP response. Persistence
	// integration stays in the package's service-bound archive test.
	svc := card.New(store.New(nil), "https://sidiora.fun", "https://og.example", nil)
	hydrated, err := svc.DecodeArchive(row)
	if err != nil {
		t.Fatalf("decode archived disclosure: %v", err)
	}
	response := httptest.NewRecorder()
	sharedhttp.WriteJSON(response, http.StatusOK, hydrated)
	var archived card.Minted
	if err := json.NewDecoder(response.Body).Decode(&archived); err != nil {
		t.Fatalf("decode archived disclosure response: %v", err)
	}
	if archived.CardID != row.CardID || archived.ShortCode != row.ShortCode || archived.CreatedAt != row.CreatedAt || !reflect.DeepEqual(archived.Snapshot, snapshot) {
		t.Fatalf("archive differs from persisted disclosure: %+v", archived)
	}
	if archived.ShareURL != "" {
		t.Fatalf("archive advertised an unsupported public landing URL: %s", archived.ShareURL)
	}
	if archived.OgURL != "https://og.example/pnl/cards/"+row.CardID+"/og.png" {
		t.Fatalf("archive OG URL = %s", archived.OgURL)
	}
	if !bytes.Equal(row.Snapshot, originalPayload) {
		t.Fatalf("archive read mutated stored disclosure: %s", row.Snapshot)
	}
	rendered := card.BuildRenderInput(archived.Snapshot, "")
	if rendered.Title != "Archived token" || rendered.Pnl != "+$1.50" || rendered.Multiple != "2.50x" || rendered.Holdings != "1.00 OLD held" || rendered.Footer != "" {
		t.Fatalf("archive render input = %+v", rendered)
	}
	row.Snapshot = json.RawMessage(`{"version":`)
	if disclosure, err := svc.DecodeArchive(row); err == nil || disclosure != nil {
		t.Fatalf("corrupt archive = %+v, %v; want nil disclosure and decoding error", disclosure, err)
	}
}
