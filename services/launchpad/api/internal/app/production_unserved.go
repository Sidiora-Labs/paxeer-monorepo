package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
)

var productionUnservedOperations = []struct{ method, path string }{
	{http.MethodPost, "/v2/me/automation-campaigns"},
	{http.MethodPost, "/v2/me/automation-campaigns/{campaignId}/actions"},
	{http.MethodGet, "/v2/me/automation-campaigns/{campaignId}"},
	{http.MethodGet, "/v2/automation-campaigns/{campaignId}/disclosure"},
	{http.MethodPost, "/v2/me/funding/deposit-intents"},
	{http.MethodGet, "/v2/me/funding/{fundingId}"},
	{http.MethodPost, "/v2/me/funding/withdrawals:prepare"},
	{http.MethodGet, "/v2/me/funding/withdrawals/{preparationId}"},
	{http.MethodGet, "/v2/me/conversations"},
	{http.MethodPost, "/v2/me/conversations"},
	{http.MethodGet, "/v2/rooms/{roomId}/messages"},
	{http.MethodPost, "/v2/rooms/{roomId}/messages"},
	{http.MethodPost, "/v2/admin/moderation-actions"},
	{http.MethodGet, "/v2/admin/reports"},
	{http.MethodGet, "/v2/operator/dlq"},
	{http.MethodPost, "/v2/operator/dlq:redrive"},
	{http.MethodGet, "/v2/operator/health"},
	{http.MethodGet, "/v2/operator/jobs/{jobId}"},
	{http.MethodGet, "/v2/operator/metrics/query"},
	{http.MethodPost, "/v2/operator/replays"},
	{http.MethodPost, "/v2/quotes"},
}

func registerUnservedOperations(r chi.Router, admission *rest.Admission, ops []struct{ method, path string }) {
	var handler http.Handler = http.HandlerFunc(rest.WriteV2NotConfiguredError)
	if admission != nil {
		handler = admission.Middleware(rest.AdmissionCached)(handler)
	}
	handler = rest.ResponseMetadata(handler)
	for _, op := range ops {
		r.Method(op.method, op.path, handler)
	}
}
