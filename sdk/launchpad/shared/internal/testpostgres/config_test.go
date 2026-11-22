package testpostgres

import (
	"net/url"
	"testing"
)

func TestParseConfigUsesSuppliedRoute(t *testing.T) {
	t.Setenv("PGUSER", "ambient")
	t.Setenv("PGHOST", "ambient")
	t.Setenv("PGPORT", "1")
	t.Setenv("PGDATABASE", "ambient")
	for _, dsn := range []string{
		"postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/kindle_fixture?sslmode=disable",
		"postgres://u:p@host:5432/mydb?sslmode=disable",
	} {
		cfg, err := ParseConfig(dsn)
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.ConnConfig.User != u.User.Username() || cfg.ConnConfig.Host != u.Hostname() || cfg.ConnConfig.Database != u.Path[1:] || cfg.ConnConfig.Port == 1 {
			t.Fatal("supplied route changed")
		}
	}
}

func TestParseConfigRequiresExplicitRoute(t *testing.T) {
	for _, dsn := range []string{
		"", "postgres://", "postgres://host:5432/mydb",
		"postgres://u:p@:5432/mydb", "postgres://u:p@host/mydb",
		"postgres://u:p@host:0/mydb", "postgres://u:p@host:5432/",
		"postgres://u:p@host:5432/mydb?host=other",
		"postgres://u:p@host:5432/mydb?user=other",
		"postgres://u:p@host:5432/mydb?port=1",
		"postgres://u:p@host:5432/mydb?dbname=other",
	} {
		if cfg, err := ParseConfig(dsn); err == nil || cfg != nil {
			t.Fatal("incomplete or overridden route admitted")
		}
	}
}
