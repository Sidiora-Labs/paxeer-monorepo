package l16

import (
	"context"
	"crypto/sha256"
	"encoding"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
)

const liveEnrollmentPages int64 = 4
const liveLogRangeBlocks int64 = 2000

var liveEnrollmentPageTime time.Duration = 30 * time.Second

var liveHistoryUnavailableTexts = [...]string{"pruned", "before earliest available block", "unknown block", "block not found by hash"}

var errLiveEnrollmentPending = errors.New("wallet enrollment in progress")

type liveReplayProgress struct {
	Snapshot        RebuildSnapshot
	Records         []EconomicRecord
	PageProofs      []string
	OpenPageMoves   []movement
	CreationTime    uint64
	BaseDigest      string
	TargetHash      string
	DayHashState    []byte
	RecordHashState []byte
	RecordCount     int
}

type liveReplayHistory struct {
	snapshotDigest string
	records        []EconomicRecord
}

type liveWalletInput struct {
	Generation, Release, Wallet, Hash string
	Height                            int64
	Snapshots                         []RebuildSnapshot
}

func liveHistoryUnavailable(err error) bool {
	if errors.Is(err, ethereum.NotFound) {
		return true
	}
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, text := range liveHistoryUnavailableTexts {
		if strings.Contains(message, text) {
			return true
		}
	}
	return false
}

func liveRead[T any](ctx context.Context, a *BaselineAuthority, read func(*ethclient.Client) (T, error)) (T, error) {
	value, err := read(a.rpc)
	if err == nil || a.liveFallback == nil || !liveHistoryUnavailable(err) {
		return value, err
	}
	chain, e := a.liveFallback.ChainID(ctx)
	if e != nil {
		return value, e
	}
	if chain == nil || chain.Cmp(big.NewInt(125)) != 0 {
		return value, delivery.ErrIntegrity
	}
	return read(a.liveFallback)
}

func (a *BaselineAuthority) liveHeaderByNumber(ctx context.Context, height int64) (*types.Header, string, error) {
	type result struct {
		header *types.Header
		hash   string
	}
	value, err := liveRead(ctx, a, func(client *ethclient.Client) (result, error) {
		header, hash, err := reportedHeaderByNumber(ctx, client, big.NewInt(height))
		return result{header, hash}, err
	})
	return value.header, value.hash, err
}

func (a *BaselineAuthority) liveHeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return liveRead(ctx, a, func(client *ethclient.Client) (*types.Header, error) { return client.HeaderByHash(ctx, hash) })
}

func (s *Service) readLiveProgressTx(ctx context.Context, tx pgx.Tx, partition, binding string, seed *RebuildSnapshot) (liveReplayProgress, string, error) {
	var progress liveReplayProgress
	a := s.authority
	key := "live-enrollment:" + partition + ":" + binding
	var raw []byte
	var digest string
	err := tx.QueryRow(ctx, `SELECT s.payload,s.snapshot_digest FROM `+a.table("inventory_l16_active_v2")+` a JOIN `+a.table("inventory_l16_snapshots_v2")+` s USING(generation,partition,snapshot_digest) WHERE a.generation=$1 AND a.partition=$2`, a.generation, key).Scan(&raw, &digest)
	if errors.Is(err, pgx.ErrNoRows) {
		if seed != nil {
			progress, err = s.resumeLiveSnapshotTx(ctx, tx, *seed)
			return progress, key, err
		}
		return progress, key, nil
	}
	if err != nil {
		return progress, key, err
	}
	if delivery.Hash(raw) != digest || json.Unmarshal(raw, &progress) != nil || progress.Snapshot.Generation != a.generation || progress.Snapshot.Partition != partition || progress.Snapshot.Algorithm != Algorithm || progress.Snapshot.ReleaseDigest != s.release || progress.Snapshot.Height <= 0 {
		return progress, key, delivery.ErrIntegrity
	}
	if seed != nil {
		raw, err = json.Marshal(seed)
		if err != nil {
			return progress, key, err
		}
		seedDigest := delivery.Hash(raw)
		if progress.Snapshot.Height < seed.Height || progress.Snapshot.Height == seed.Height && !sameReplaySnapshot(progress.Snapshot, *seed) || progress.Snapshot.Height > seed.Height && progress.BaseDigest != seedDigest {
			return progress, key, delivery.ErrIntegrity
		}
		progress.BaseDigest = seedDigest
	}
	return progress, key, nil
}

func (s *Service) writeLiveProgressTx(ctx context.Context, tx pgx.Tx, partition string, progress liveReplayProgress) error {
	raw, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	digest := delivery.Hash(raw)
	a := s.authority
	if _, err = tx.Exec(ctx, `INSERT INTO `+a.table("inventory_l16_snapshots_v2")+` (generation,partition,snapshot_digest,payload,height,block_hash,release_digest) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, a.generation, partition, digest, raw, progress.Snapshot.Height, progress.Snapshot.BlockHash, s.release); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO `+a.table("inventory_l16_active_v2")+` (partition,generation,snapshot_digest,height) VALUES($1,$2,$3,$4) ON CONFLICT(partition) DO UPDATE SET generation=EXCLUDED.generation,snapshot_digest=EXCLUDED.snapshot_digest,height=EXCLUDED.height`, partition, a.generation, digest, progress.Snapshot.Height)
	return err
}

func liveCheckpoints(ctx context.Context, snapshot *RebuildSnapshot, progress *liveReplayProgress, records []EconomicRecord) error {
	if progress.CreationTime > 1<<63-1 {
		return delivery.ErrIntegrity
	}
	day := time.Unix(int64(progress.CreationTime), 0).UTC().Truncate(24 * time.Hour)
	endDay := snapshot.ObservedThrough.Truncate(24 * time.Hour)
	if endDay.Before(day) || endDay.Sub(day) > 3660*24*time.Hour {
		return ErrBaselineUnavailable
	}
	h := sha256.New()
	var cp Checkpoint
	checkpoints := append([]Checkpoint(nil), progress.Snapshot.Checkpoints...)
	if len(checkpoints) > 0 {
		cp = checkpoints[len(checkpoints)-1]
		var err error
		day, err = time.Parse("2006-01-02", cp.Day)
		if err != nil {
			return err
		}
		if err = h.(encoding.BinaryUnmarshaler).UnmarshalBinary(progress.DayHashState); err != nil {
			return err
		}
		checkpoints = checkpoints[:len(checkpoints)-1]
	} else {
		cp = Checkpoint{Day: day.Format("2006-01-02"), Algorithm: Algorithm, State: Empty(), CompleteHistory: true}
		raw, err := json.Marshal(cp)
		if err != nil {
			return err
		}
		_, _ = h.Write(raw)
	}
	offset := 0
	for !day.After(endDay) {
		next := day.AddDate(0, 0, 1)
		for offset < len(records) && records[offset].CommittedAt.Before(next) {
			if err := ctx.Err(); err != nil {
				return err
			}
			entry := records[offset].TimedLeg
			if entry.CommittedAt.Before(day) {
				return delivery.ErrIntegrity
			}
			leg := entry.Leg
			identity, err := leg.Identity.Key()
			if err != nil {
				return err
			}
			partition := "125:" + leg.Wallet + ":" + leg.Token
			if cp.Partition == "" {
				cp.Partition = partition
			} else if cp.Partition != partition {
				return delivery.ErrIntegrity
			}
			if leg.Predecessor != cp.LastIdentity || cp.LastIdentity != "" && (leg.Height < cp.LastHeight || leg.Height == cp.LastHeight && (leg.TransactionIndex < cp.LastTransactionIndex || leg.TransactionIndex == cp.LastTransactionIndex && leg.Identity.Log <= cp.LastLogIndex)) {
				return delivery.ErrIntegrity
			}
			cp.State, err = Apply(cp.State, leg)
			if err != nil {
				return err
			}
			cp.LastIdentity = identity
			cp.LastHeight = leg.Height
			cp.LastTransactionIndex = leg.TransactionIndex
			cp.LastLogIndex = leg.Identity.Log
			raw, err := json.Marshal(entry)
			if err != nil {
				return err
			}
			_, _ = h.Write(raw)
			offset++
		}
		cp.InputDigest = hex.EncodeToString(h.Sum(nil))
		checkpoints = append(checkpoints, cp)
		if next.After(endDay) {
			break
		}
		cp = Checkpoint{Partition: cp.Partition, Day: next.Format("2006-01-02"), Algorithm: Algorithm, State: cp.State, CompleteHistory: cp.CompleteHistory, LastIdentity: cp.LastIdentity, LastHeight: cp.LastHeight, LastTransactionIndex: cp.LastTransactionIndex, LastLogIndex: cp.LastLogIndex, ParentDigest: cp.InputDigest}
		h = sha256.New()
		raw, err := json.Marshal(cp)
		if err != nil {
			return err
		}
		_, _ = h.Write(raw)
		day = next
	}
	if offset != len(records) || cp.State != snapshot.State || cp.LastIdentity != snapshot.LastIdentity {
		return delivery.ErrIntegrity
	}
	var err error
	progress.DayHashState, err = h.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		return err
	}
	snapshot.Checkpoints = checkpoints
	return nil
}

func liveRecordDigest(progress *liveReplayProgress, records []EconomicRecord) (string, error) {
	h := sha256.New()
	if progress.RecordHashState == nil {
		_, _ = h.Write([]byte("["))
	} else if err := h.(encoding.BinaryUnmarshaler).UnmarshalBinary(progress.RecordHashState); err != nil {
		return "", err
	}
	for _, record := range records {
		if progress.RecordCount > 0 {
			_, _ = h.Write([]byte(","))
		}
		raw, err := json.Marshal(record)
		if err != nil {
			return "", err
		}
		_, _ = h.Write(raw)
		progress.RecordCount++
	}
	var err error
	progress.RecordHashState, err = h.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		return "", err
	}
	_, _ = h.Write([]byte("]"))
	return hex.EncodeToString(h.Sum(nil)), nil
}

func liveSeedTokens(seeds map[string]RebuildSnapshot) []string {
	tokens := make([]string, 0, len(seeds))
	for token := range seeds {
		tokens = append(tokens, token)
	}
	return tokens
}

func (w *LiveTransferWorker) acceptedSnapshotsTx(ctx context.Context, tx pgx.Tx, wallet string, height int64, hash, snapshotID, source *string) (map[string]RebuildSnapshot, error) {
	seeds := map[string]RebuildSnapshot{}
	if height == 0 {
		if hash != nil || snapshotID != nil || source != nil {
			return nil, delivery.ErrIntegrity
		}
		return seeds, nil
	}
	if hash == nil || snapshotID == nil || source == nil {
		return nil, delivery.ErrIntegrity
	}
	a := w.service.authority
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT s.payload FROM `+a.table("inventory_l16_snapshots_v2")+` s JOIN `+a.table("inventory_l16_live_commits_v1")+` c ON c.generation=s.generation AND c.wallet=$2 AND c.source_stream=$3 AND c.height=s.height AND c.block_hash=s.block_hash AND c.input_digest=s.snapshot_digest AND c.snapshot_id=$4::uuid WHERE s.generation=$1 AND s.partition=$5 AND s.snapshot_digest=$6 AND s.height=$7`, a.generation, wallet, a.sourceStream, *snapshotID, "live-wallet:"+wallet, *source, height).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		raw, err = w.readAcceptedSnapshotInputTx(ctx, tx, wallet, height, *hash, *snapshotID, *source)
	}
	if err != nil {
		return nil, err
	}
	var input liveWalletInput
	if delivery.Hash(raw) != *source || json.Unmarshal(raw, &input) != nil || input.Generation != a.generation || input.Wallet != wallet || input.Height != height || input.Hash != *hash {
		return nil, delivery.ErrIntegrity
	}
	for _, snapshot := range input.Snapshots {
		parts := strings.Split(snapshot.Partition, ":")
		if len(parts) != 3 || parts[0] != "125" || parts[1] != wallet || !address.MatchString(parts[2]) || snapshot.Generation != a.generation || snapshot.Algorithm != Algorithm || snapshot.ReleaseDigest != input.Release || snapshot.Height != height || snapshot.BlockHash != *hash {
			return nil, delivery.ErrIntegrity
		}
		if _, exists := seeds[parts[2]]; exists {
			return nil, delivery.ErrIntegrity
		}
		seeds[parts[2]] = snapshot
	}
	return seeds, nil
}

func (w *LiveTransferWorker) quarantineLiveWalletTx(ctx context.Context, tx pgx.Tx, wallet string, height int64, hash string) (bool, error) {
	a := w.service.authority
	raw, err := json.Marshal(struct {
		Wallet    string
		Height    int64
		BlockHash string
	}{wallet, height, hash})
	if err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO `+a.table("inventory_l16_quarantine_v2")+` (generation,partition,height,block_hash,reason,payload) VALUES($1,$2,$3,$4,'dual_node_or_inventory_parity',$5) ON CONFLICT DO NOTHING`, a.generation, "125:"+wallet+":canonical", height, hash, raw); err != nil {
		return false, err
	}
	if err = a.epochGuard(ctx, tx); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return false, ErrParity
}

func (w *LiveTransferWorker) readAcceptedSnapshotInputTx(ctx context.Context, tx pgx.Tx, wallet string, height int64, hash, snapshotID, source string) ([]byte, error) {
	a := w.service.authority
	input := liveWalletInput{Generation: a.generation, Wallet: wallet, Height: height, Hash: hash, Snapshots: []RebuildSnapshot{}}
	if err := tx.QueryRow(ctx, `SELECT release_digest FROM `+a.table("inventory_l16_live_commits_v1")+` WHERE generation=$1 AND wallet=$2 AND source_stream=$3 AND height=$4 AND block_hash=$5 AND snapshot_id=$6::uuid AND input_digest=$7`, a.generation, wallet, a.sourceStream, height, hash, snapshotID, source).Scan(&input.Release); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT s.payload,s.snapshot_digest FROM `+a.table("portfolio_snapshot_rows")+` r JOIN `+a.table("inventory_l16_live_commits_v1")+` c ON c.snapshot_id=r.snapshot_id JOIN `+a.table("inventory_l16_snapshots_v2")+` s ON s.partition='125:'||c.wallet||':'||(r.payload->>'tokenAddress') AND s.generation=c.generation AND s.height=c.height AND s.block_hash=c.block_hash AND s.release_digest=c.release_digest WHERE r.snapshot_id=$1::uuid AND convert_from(s.payload,'UTF8')::jsonb->>'sourceOwner'=c.source_owner AND (convert_from(s.payload,'UTF8')::jsonb->>'sourceEpoch')::bigint=c.source_epoch ORDER BY s.partition`, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var digest string
		if err = rows.Scan(&raw, &digest); err != nil {
			return nil, err
		}
		var snapshot RebuildSnapshot
		if delivery.Hash(raw) != digest || json.Unmarshal(raw, &snapshot) != nil {
			return nil, delivery.ErrIntegrity
		}
		input.Snapshots = append(input.Snapshots, snapshot)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	if delivery.Hash(raw) != source {
		return nil, delivery.ErrIntegrity
	}
	return raw, nil
}

func (s *Service) resumeLiveSnapshotTx(ctx context.Context, tx pgx.Tx, seed RebuildSnapshot) (liveReplayProgress, error) {
	progress := liveReplayProgress{Snapshot: seed}
	parts := strings.Split(seed.Partition, ":")
	if len(parts) != 3 || len(seed.Checkpoints) == 0 {
		return progress, delivery.ErrIntegrity
	}
	records, err := s.readReplayArchiveTx(ctx, tx, parts[1], seed)
	if err != nil {
		return progress, err
	}
	day, err := time.Parse("2006-01-02", seed.Checkpoints[0].Day)
	if err != nil {
		return progress, err
	}
	seconds := day.Unix()
	if seconds < 0 {
		return progress, delivery.ErrIntegrity
	}
	progress.CreationTime = uint64(seconds)
	checkpointSeed := liveReplayProgress{CreationTime: progress.CreationTime}
	rebuilt := seed
	rebuilt.Checkpoints = nil
	if err = liveCheckpoints(ctx, &rebuilt, &checkpointSeed, records); err != nil {
		return progress, err
	}
	if !sameReplaySnapshot(seed, rebuilt) {
		return progress, delivery.ErrIntegrity
	}
	progress.DayHashState = checkpointSeed.DayHashState
	digest, err := liveRecordDigest(&progress, records)
	if err != nil {
		return progress, err
	}
	if digest != seed.InputDigest {
		return progress, delivery.ErrIntegrity
	}
	progress.Records = records
	rows, err := tx.Query(ctx, `SELECT transaction_hash,log_index,height,transaction_index,block_hash,from_address,to_address,amount_raw FROM indexer.indexer_token_transfers_v1 WHERE chain_id=125 AND token=$1 AND height BETWEEN $2 AND $3 ORDER BY height,transaction_index,log_index`, parts[2], seed.CreationHeight, seed.Height)
	if err != nil {
		return progress, err
	}
	defer rows.Close()
	moves := []movement{}
	pageEnd := seed.CreationHeight + 9999
	for rows.Next() {
		var m movement
		if err = rows.Scan(&m.TransactionHash, &m.LogIndex, &m.Height, &m.TransactionIndex, &m.BlockHash, &m.From, &m.To, &m.Amount); err != nil {
			return progress, err
		}
		for m.Height > pageEnd {
			raw, e := json.Marshal(moves)
			if e != nil {
				return progress, e
			}
			progress.PageProofs = append(progress.PageProofs, delivery.Hash(raw))
			moves = []movement{}
			pageEnd += 10000
		}
		moves = append(moves, m)
	}
	if err = rows.Err(); err != nil {
		return progress, err
	}
	for pageEnd < seed.Height {
		raw, e := json.Marshal(moves)
		if e != nil {
			return progress, e
		}
		progress.PageProofs = append(progress.PageProofs, delivery.Hash(raw))
		moves = []movement{}
		pageEnd += 10000
	}
	raw, err := json.Marshal(moves)
	if err != nil {
		return progress, err
	}
	proofs := append([]string{}, progress.PageProofs...)
	proofs = append(proofs, delivery.Hash(raw))
	if pageEnd == seed.Height {
		progress.PageProofs = proofs
	} else {
		progress.OpenPageMoves = moves
	}
	raw, err = json.Marshal(proofs)
	if err != nil {
		return progress, err
	}
	if delivery.Hash(raw) != seed.MovementDigest {
		return progress, delivery.ErrIntegrity
	}
	raw, err = json.Marshal(seed)
	if err != nil {
		return progress, err
	}
	progress.BaseDigest = delivery.Hash(raw)
	return progress, nil
}

func liveHistoryResult(snapshot RebuildSnapshot, records []EconomicRecord, output []*liveReplayHistory) error {
	if len(output) == 0 {
		return nil
	}
	if len(output) != 1 || output[0] == nil {
		return delivery.ErrIntegrity
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	*output[0] = liveReplayHistory{snapshotDigest: delivery.Hash(raw), records: records}
	return nil
}
