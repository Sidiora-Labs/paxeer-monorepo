package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestDeadLetterInspectionBoundsPaginationAndDurableMetadata(t *testing.T) {
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	ddl, err := delivery.SchemaDDL("indexer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	operations := AuditedDeadLetters{DB: pool, DeliverySchema: "indexer"}
	inspect := func(query string, want int) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		operations.inspect(ctx, response, httptest.NewRequest(http.MethodGet, "/status/dead-letter"+query, nil))
		if response.Code != want {
			t.Fatalf("inspection status %d: %s", response.Code, response.Body.String())
		}
		return response
	}
	for _, query := range []string{"?limit=word", "?limit=0", "?limit=101", "?after=" + strings.Repeat("a", 2049), "?after=%25%25%25"} {
		inspect(query, http.StatusBadRequest)
	}
	for _, cursor := range []string{`{`, `{} {}`, `{"unknown":true}`, `{"envelopeHash":"invalid"}`, `{"envelopeHash":"` + strings.Repeat("a", 64) + `","consumer":"` + strings.Repeat("x", 129) + `"}`} {
		inspect("?after="+base64.RawURLEncoding.EncodeToString([]byte(cursor)), http.StatusBadRequest)
	}
	hash := strings.Repeat("a", 64)
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.delivery_quarantine(consumer_id,projection_generation,envelope_hash,subject,envelope,failure_class) VALUES('consumer','generation',$1,'events.swap',$2,'private-detail')`, hash, []byte("retained envelope")); err != nil {
		t.Fatal(err)
	}
	response := inspect("?limit=1", http.StatusOK)
	var page struct {
		Entries []deadLetterMetadata `json:"entries"`
		Next    string               `json:"nextCursor"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || len(page.Entries) != 1 || page.Entries[0].FailureClass != "unclassified" || page.Next == "" {
		t.Fatalf("bounded metadata page: %s, %v", response.Body.String(), err)
	}
	next := inspect("?after="+page.Next, http.StatusOK)
	if err := json.Unmarshal(next.Body.Bytes(), &page); err != nil || len(page.Entries) != 0 || page.Next != "" {
		t.Fatalf("metadata cursor: %s, %v", next.Body.String(), err)
	}
	if strings.Contains(response.Body.String(), "retained envelope") || strings.Contains(response.Body.String(), "private-detail") {
		t.Fatal("inspection exposed retained content")
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.delivery_quarantine SET consumer_id=$1`, strings.Repeat("x", 129)); err != nil {
		t.Fatal(err)
	}
	inspect("", http.StatusServiceUnavailable)
	if _, err := pool.Exec(ctx, `UPDATE indexer.delivery_quarantine SET consumer_id='consumer'; ALTER TABLE indexer.delivery_quarantine ALTER COLUMN created_at DROP DEFAULT; ALTER TABLE indexer.delivery_quarantine ALTER COLUMN created_at TYPE text USING created_at::text`); err != nil {
		t.Fatal(err)
	}
	inspect("", http.StatusServiceUnavailable)
	if _, err := pool.Exec(ctx, `ALTER TABLE indexer.delivery_quarantine RENAME TO unavailable_quarantine`); err != nil {
		t.Fatal(err)
	}
	inspect("", http.StatusServiceUnavailable)
	policyResponse := httptest.NewRecorder()
	(AuditedDeadLetters{}).Handler().ServeHTTP(policyResponse, httptest.NewRequest(http.MethodGet, "/status/dead-letter", nil))
	if policyResponse.Code != http.StatusServiceUnavailable || strings.TrimSpace(policyResponse.Body.String()) != "dead-letter policy unavailable" {
		t.Fatalf("absent policy: %d %s", policyResponse.Code, policyResponse.Body.String())
	}
}
