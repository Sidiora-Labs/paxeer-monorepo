package sourcebootstrap_test

import (
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap"
)

func TestCheckpointTuplesAreFrozen(t *testing.T) {
	pool := "0xce20da50f92d21d8bee497910c782e9aa25b6263"
	publication := accounting.HashTuple("fixture-publication", pool)
	payload := accounting.Digest(accounting.HashTuple("fixture-payload", pool))
	hash := "0x81a18d8200000000000000000000000000000000000000000000000000192b"
	stats := sourcebootstrap.StatsEvidence{SourceStream: "chain125-canonical", Pool: pool, ProjectionVersion: "accounting-market-v1", PublicationID: publication, PayloadDigest: payload, ReplayFromHeight: 33963700, CreationTransactionHash: "0xe200486fb275ad363101dd09455b23842f1a002b839062732230fd4c908cbfca", CreationLogIndex: 7, FirstCommitID: "trades", FirstCommitLastHeight: 33963761, FirstCommitBlockHash: hash}
	charts := sourcebootstrap.ChartsEvidence{SourceStream: "chain125-canonical", Pool: pool, ProjectionVersion: "accounting-market-v1", PublicationID: publication, PayloadDigest: payload, ReplayFromHeight: 33963700, CreationTransactionHash: "0xe200486fb275ad363101dd09455b23842f1a002b839062732230fd4c908cbfca", CreationLogIndex: 7, FirstCommitID: "trades", FirstCommitLastHeight: 33963761, FirstCommitBlockHash: hash}
	if got := stats.Checkpoint(); got != "62d75616dc849849387de1b1b5115d4deebe7f320d4172a1058026140eb29973" {
		t.Fatal("stats checkpoint tuple changed", got)
	}
	if got := charts.Checkpoint(); got != "d887ad24c516574788b95d883f7fd1d9be08029aca0faa122ea41a2f101993ba" {
		t.Fatal("charts checkpoint tuple changed", got)
	}
	legacy := accounting.Digest(accounting.HashTuple("stats-source-bootstrap-v1", stats.SourceStream, stats.Pool, stats.ProjectionVersion, stats.PublicationID, string(stats.PayloadDigest), "32226214", stats.FirstCommitID, "33963761", stats.FirstCommitBlockHash))
	if legacy == stats.Checkpoint() {
		t.Fatal("legacy stats checkpoint admitted")
	}
	other := stats
	other.Pool = "0x2222222222222222222222222222222222222222"
	if other.Checkpoint() == stats.Checkpoint() {
		t.Fatal("checkpoint is not bound to the pool")
	}
}
