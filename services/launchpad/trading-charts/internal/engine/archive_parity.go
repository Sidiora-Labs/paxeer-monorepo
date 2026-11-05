package engine

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/jackc/pgx/v5"
)

var ErrReconstructionCheckpoint = errors.New("reconstruction_economic_checkpoint_missing_or_invalid")

type retainedReconstructionCheckpoint struct {
	Pool             string `json:"pool"`
	From             int64  `json:"from_time"`
	To               int64  `json:"to_time"`
	Expected         string `json:"expected_digest"`
	Output           string `json:"output_digest"`
	Manifest         string `json:"manifest_digest"`
	Epoch            uint64 `json:"promotion_epoch"`
	Status           string `json:"status"`
	RetainedManifest string `json:"retained_manifest"`
}

func decodeReconstructionCheckpoint(raw []byte, pool string, from, to int64, manifest string, revision uint64) (string, error) {
	if len(raw) == 0 || len(raw) > 4096 {
		return "", ErrReconstructionCheckpoint
	}
	var checkpoint retainedReconstructionCheckpoint
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&checkpoint) != nil || decoder.Decode(new(any)) != io.EOF {
		return "", ErrReconstructionCheckpoint
	}
	digest, err := hex.DecodeString(checkpoint.Expected)
	if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != checkpoint.Expected || checkpoint.Expected != checkpoint.Output ||
		checkpoint.Pool != pool || checkpoint.From != from || checkpoint.To != to || from < 0 || to <= from ||
		checkpoint.Manifest != manifest || checkpoint.RetainedManifest != manifest || checkpoint.Status != "promoted" || checkpoint.Epoch == 0 || checkpoint.Epoch != revision {
		return "", ErrReconstructionCheckpoint
	}
	manifestBytes, err := hex.DecodeString(manifest)
	if err != nil || len(manifestBytes) != 32 || hex.EncodeToString(manifestBytes) != manifest {
		return "", ErrReconstructionCheckpoint
	}
	return checkpoint.Expected, nil
}

func verifyReconstructionParity(expected, actual string) error {
	if expected != actual {
		return errors.New("reconstruction_economic_checksum_mismatch")
	}
	return nil
}

func reconstructionExpectedDigest(ctx context.Context, tx pgx.Tx, schema, pool string, from, to int64, manifest string, revision uint64) (string, string, error) {
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT jsonb_build_object('pool',r.pool,'from_time',r.from_time,'to_time',r.to_time,'expected_digest',r.expected_digest,'output_digest',r.output_digest,'manifest_digest',r.manifest_digest,'promotion_epoch',r.promotion_epoch,'status',r.status,'retained_manifest',COALESCE((SELECT c.manifest_digest FROM `+schema+`.chart_archive_checkpoints c WHERE c.pool=r.pool AND c.from_time=r.from_time AND c.to_time=r.to_time),'')) FROM `+schema+`.chart_rebuild_runs r WHERE r.pool=$1 AND r.from_time=$2 AND r.to_time=$3 AND r.status='promoted' ORDER BY r.promotion_epoch DESC LIMIT 1`, pool, from, to).Scan(&raw)
	if err == nil {
		expected, err := decodeReconstructionCheckpoint(raw, pool, from, to, manifest, revision)
		if err != nil {
			return "", "", err
		}
		return expected, "retained_promoted_checkpoint", nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", err
	}
	// Initial parity qualification creates the durable promoted commitment. Once
	// any overlapping range has been checkpointed, recovery must use that authority.
	var checkpointed bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+schema+`.chart_archive_checkpoints WHERE pool=$1 AND from_time<$3 AND to_time>$2) OR EXISTS(SELECT 1 FROM `+schema+`.chart_rebuild_runs WHERE pool=$1 AND from_time<$3 AND to_time>$2 AND status='promoted')`, pool, from, to).Scan(&checkpointed)
	if err != nil {
		return "", "", err
	}
	if checkpointed {
		return "", "", ErrReconstructionCheckpoint
	}
	rows, err := tx.Query(ctx, `SELECT interval,start,snapshot FROM `+schema+`.chart_exact WHERE chain_id='125' AND pool=$1 AND start>=$2 AND start<$3 ORDER BY interval,start`, pool, from, to)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()
	bars := []QualifiedCandle{}
	for rows.Next() {
		var interval string
		var start int64
		var raw []byte
		var bar QualifiedCandle
		if err = rows.Scan(&interval, &start, &raw); err != nil {
			return "", "", err
		}
		if err = json.Unmarshal(raw, &bar); err != nil {
			return "", "", err
		}
		if bar.PoolAddress != pool || bar.Interval != interval || bar.Start != start {
			return "", "", store.ErrIntegrity
		}
		if _, err = IntervalSeconds(interval); err != nil {
			return "", "", err
		}
		bars = append(bars, bar)
	}
	if err = rows.Err(); err != nil {
		return "", "", err
	}
	digest, err := projectionDigest(bars)
	return digest, "initial_projection_qualification", err
}
