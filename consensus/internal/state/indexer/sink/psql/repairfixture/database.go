// Package repairfixture shares the existing embedded SQL fixture with indexer
// integration tests. SQL is executed by SQLite, never a mock of the sink or DB.
package repairfixture

import (
	"database/sql"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink/psql/internal/repairdb"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open(repairdb.Name, path)
	if err == nil {
		db.SetMaxOpenConns(1)
	}
	return db, err
}
