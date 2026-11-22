package testpostgres

import (
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ParseConfig(dsn string) (*pgxpool.Config, error) {
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.User == nil || u.User.Username() == "" || u.Hostname() == "" || strings.TrimPrefix(u.Path, "/") == "" {
		return nil, errors.New("explicit PostgreSQL fixture user, host, port and database required")
	}
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil || port == 0 {
		return nil, errors.New("explicit PostgreSQL fixture port required")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid PostgreSQL fixture configuration")
	}
	if cfg.ConnConfig.User != u.User.Username() || cfg.ConnConfig.Host != u.Hostname() || cfg.ConnConfig.Port != uint16(port) || cfg.ConnConfig.Database != strings.TrimPrefix(u.Path, "/") {
		return nil, errors.New("PostgreSQL fixture route differs from supplied DSN")
	}
	return cfg, nil
}
