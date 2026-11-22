package enrolmenttest

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestOperatorPostgresVersion(t *testing.T) {
	t.Setenv("COORDINATOR_TEST_POSTGRES_DSN", "")
	dsn := Start(t)
	connection, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal("connect private PostgreSQL")
	}
	defer connection.Close(context.Background())
	var version string
	if err := connection.QueryRow(context.Background(), "SELECT version()").Scan(&version); err != nil || !strings.HasPrefix(version, "PostgreSQL ") {
		t.Fatal("read private PostgreSQL version")
	}
	t.Setenv("COORDINATOR_TEST_POSTGRES_DSN", dsn)
	if Start(t) != dsn {
		t.Fatal("configured PostgreSQL address was not reused")
	}
}
