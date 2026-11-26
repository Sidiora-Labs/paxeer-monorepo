package store_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store/accountingtest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap"
)

func TestAccountingReadsCreationBasedStatsCheckpoint(t *testing.T) {
	f := accountingtest.New(t)
	fill, position := f.Publish(t)
	f.Commit(t, fill, position)
	evidence, err := sourcebootstrap.LoadStats(t.Context(), f.DB, f.Config.SourcePartitionSchema, f.Config.SourceStream, f.Config.ProjectionVersion, accountingtest.Pool)
	if err != nil || evidence.ReplayFromHeight != 99 || evidence.CreationLogIndex != 1 || evidence.CreationTransactionHash == "" {
		t.Fatalf("creation evidence: %+v %v", evidence, err)
	}
	var checkpoint string
	if err = f.DB.QueryRow(t.Context(), `SELECT checkpoint_digest FROM stats.delivery_partition_offsets WHERE partition_key=$1`, position.PartitionKey).Scan(&checkpoint); err != nil || checkpoint != string(evidence.Checkpoint()) {
		t.Fatalf("stored creation checkpoint: %q %v", checkpoint, err)
	}
	request := store.AccountingPageRequest{Pool: accountingtest.Pool, Limit: 5}
	page, err := f.Model.Trades(t.Context(), request)
	if err != nil || len(page.Items) != 1 || page.Revision != "1" {
		t.Fatalf("creation checkpoint page: %+v %v", page, err)
	}
	legacy := accounting.HashTuple("stats-source-bootstrap-v1", evidence.SourceStream, evidence.Pool, evidence.ProjectionVersion, evidence.PublicationID, string(evidence.PayloadDigest), "99", evidence.FirstCommitID, strconv.FormatInt(evidence.FirstCommitLastHeight, 10), evidence.FirstCommitBlockHash)
	f.Exec(t, `UPDATE stats.delivery_partition_offsets SET checkpoint_digest=$1 WHERE partition_key=$2`, legacy, position.PartitionKey)
	if _, err = f.Model.Trades(t.Context(), request); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("legacy stats checkpoint: %v", err)
	}
}
