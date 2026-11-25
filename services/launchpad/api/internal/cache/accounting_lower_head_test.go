package cache

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store/accountingtest"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
	"github.com/go-chi/chi/v5"
)

func TestAccountingLowerHeadMountedRoutes(t *testing.T) {
	for _, state := range []string{"pending", "common", "frozen"} {
		t.Run(state, func(t *testing.T) {
			f := accountingtest.New(t)
			internaltest.ApplyReadMigrations(t, f.DB, "90000")
			if state == "pending" {
				f.Publish(t)
			} else {
				for sequence := int64(1); sequence <= 3; sequence++ {
					fill, position := f.PublishSequence(t, sequence)
					f.CommitConsumer(t, f.Config.ChartsConsumerID, fill, position)
					if sequence < 3 {
						f.CommitConsumer(t, f.Config.StatsConsumerID, fill, position)
					}
				}
				if state == "frozen" {
					f.Exec(t, `UPDATE stats.accounting_pool_fences SET frozen=true`)
				}
			}
			client, err := sharedredis.NewBoundedClient(internaltest.NewRedisURL(t), 200*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			config := runtimeAdmissionConfig(t, 0)
			config.ReadModel = f.Config
			config.Adapter.Redis = client
			config.Adapter.Namespace = "lower_head_pages"
			config.Adapter.Budget = PostgresQueryBudget{Pool: f.DB, Schema: "api_limits", Namespace: "lower_head_pages", Limit: 100, QueryCost: 1, Window: time.Hour}
			config.Adapter.Local.FetchTimeout = time.Second
			config.Adapter.RedisTimeout = 200 * time.Millisecond
			config.Adapter.LeaseTTL = 2 * time.Second
			config.Allocation.RequestLimit = 100
			a := runtimeAdmissionRuntime(t, config)
			router := chi.NewRouter()
			if err := a.bindings.Register(router, a.admission); err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct{ path, document string }{
				{"/trades?limit=5", "TradesResponse"},
				{"/candles?limit=5&interval=1m&from=2023-11-14T22:13:00.000Z&to=2023-11-14T22:15:00.000Z", "CandlesResponse"},
			} {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/pools/"+accountingtest.Pool+tc.path, nil))
				if state == "frozen" {
					var result rest.APIError
					if response.Code != 503 || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.Code != "INTEGRITY_QUARANTINED" {
						t.Fatalf("frozen route: %d %s", response.Code, response.Body.String())
					}
					continue
				}
				if response.Code != 200 || interfaces.ValidateJSON(tc.document, response.Body.Bytes()) != nil {
					t.Fatalf("common route: %d %s", response.Code, response.Body.String())
				}
				var envelope struct {
					Revision   *string         `json:"revision"`
					Watermark  json.RawMessage `json:"watermark"`
					NextCursor *string         `json:"nextCursor"`
					Data       struct {
						Items []json.RawMessage `json:"items"`
					} `json:"data"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				if state == "pending" {
					if envelope.Revision != nil || string(envelope.Watermark) != "null" || envelope.NextCursor != nil || envelope.Data.Items == nil || len(envelope.Data.Items) != 0 {
						t.Fatalf("pending route: %s", response.Body.String())
					}
				} else if envelope.Revision == nil || *envelope.Revision != "2" || len(envelope.Data.Items) != 2 || response.Header().Get("X-Projection-Revision") != "2" {
					t.Fatalf("common revision route: %s", response.Body.String())
				}
			}
		})
	}
}
