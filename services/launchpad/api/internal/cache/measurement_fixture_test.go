package cache

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
)

func TestRealAdapterMeasurementWithPrivateDatabase(t *testing.T) {
	dsn, db := internaltest.NewPostgresWithDSN(t)
	redisURL := internaltest.NewRedisURL(t)
	if _, err := db.Exec(context.Background(), `CREATE TABLE stats.accounting_market_commits(publication_id text NOT NULL,sequence bigint NOT NULL,position jsonb NOT NULL,fill jsonb NOT NULL);
		INSERT INTO stats.accounting_market_commits VALUES('publication-1',1,'{"height":"1"}','{"poolAddress":"0x1111111111111111111111111111111111111111"}')`); err != nil {
		t.Fatal(err)
	}
	t.Setenv("L09_DATABASE_URL", dsn)
	t.Setenv("L09_REDIS_URL", redisURL)
	t.Setenv("L09_NAMESPACE", "kd-421d7541932d2fd2136b3973_measurement")
	t.Run("durable projection measurements", TestRealAdapterMeasurementCapture)
	t.Run("shared hot expiry", TestRealHotExpiryAcrossTenReplicas)
}
