package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/config"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
)

const identityIngressReadTimeout = 5 * time.Second

func identityIngressDeadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		identity := strings.HasPrefix(path, "/auth/") ||
			strings.HasPrefix(path, "/v2/auth/") ||
			path == "/v2/me/wallet-links" || strings.HasPrefix(path, "/v2/me/wallet-links/") ||
			path == "/v2/stream/tickets" || strings.HasPrefix(path, "/internal/v2/identity/")
		if identity && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			controller := http.NewResponseController(w)
			deadline := time.Now().Add(identityIngressReadTimeout)
			if err := controller.SetReadDeadline(deadline); err != nil {
				sharedhttp.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{responseCodeField: "DEPENDENCY_UNAVAILABLE"})
				return
			}
			// net/http clears the deadline after finishing the response. Keep
			// it active while the server drains any unread, rejected body.
		}
		next.ServeHTTP(w, r)
	})
}

func identityDatabaseCheck(cfg config.Config, pool *pgxpool.Pool) sharedhttp.Check {
	return func(ctx context.Context) error {
		if pool == nil {
			return fmt.Errorf("identity_database_missing")
		}
		if cfg.NodeEnv != nodeEnvironmentProduction {
			return pool.Ping(ctx)
		}
		return cfg.CheckRuntimeSchema(ctx, pool)
	}
}
