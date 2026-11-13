package natsdriver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/util"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type epochFenceIdentity struct {
	Proposal, Actor, Region, ConfigurationSHA256 string
	Epoch                                        int64
}

type epochMemberProgress struct {
	Stage    string                    `json:"stage"`
	Acks     map[string]string         `json:"acks"`
	Evidence []EpochCredentialEvidence `json:"evidence,omitempty"`
}

type epochFenceRecord struct {
	Identity    epochFenceIdentity
	Revision    int64
	State       string
	Members     map[string]*epochMemberProgress
	Result      *EpochFenceResult
	BoundProof  string
	ProofSHA256 string
}

type epochFenceStore interface {
	run(context.Context, epochFenceIdentity, bool, func(*epochFenceRecord, func() error) error) error
}

// EpochRecoveryPlan is an explicit, privately admitted operational authority.
// The journal binds the plan and the entire member inventory on first use.
type EpochRecoveryPlan struct {
	Proposal string
	Actor    string
	Epoch    int64
	Region   string
	Revision int64
}

type fileEpochFenceStore struct {
	path string
	plan EpochRecoveryPlan
}

// NewJournalBrokerEpochController supports an explicitly admitted local recovery
// operation. It never substitutes for a missing PostgreSQL authority.
func NewJournalBrokerEpochController(path string, plan EpochRecoveryPlan, members []EpochMemberAuthority) (*BrokerEpochController, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || !epochName.MatchString(plan.Proposal) || plan.Actor == "" || len(plan.Actor) > 128 || plan.Revision < 1 {
		return nil, errors.New("natsdriver: explicit private journal and recovery plan required")
	}
	if err := privateEpochDirectory(filepath.Dir(path)); err != nil {
		return nil, err
	}
	return newBrokerEpochController(&fileEpochFenceStore{path: path, plan: plan}, plan.Epoch, plan.Region, members)
}

func privateEpochDirectory(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved != path {
		return errors.New("natsdriver: canonical journal directory required")
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return errors.New("natsdriver: private journal directory required")
	}
	return nil
}

func readEpochJournal(path string) ([]byte, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer func() {
		if cleanupErr := file.Close(); cleanupErr != nil {
			log.Printf("cleanup file.Close: %v", cleanupErr)
		}
	}()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 1<<20 {
		return nil, errors.New("natsdriver: private bounded journal required")
	}
	return io.ReadAll(io.LimitReader(file, 1<<20+1))
}

func writeEpochJournal(path string, record *epochFenceRecord) error {
	wire, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if len(wire) > 1<<20 {
		return errors.New("natsdriver: recovery journal exceeds bound")
	}
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".epoch-journal-")
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := os.Remove(temp.Name()); cleanupErr != nil {
			log.Printf("cleanup os.Remove: %v", cleanupErr)
		}
	}()
	if _, err = temp.Write(wire); err == nil {
		err = temp.Sync()
	}
	closed := temp.Close()
	if err == nil {
		err = closed
	}
	if err != nil {
		return err
	}
	if err = os.Rename(temp.Name(), path); err != nil {
		return err
	}
	directory, err := util.OpenPath(dir)
	if err != nil {
		return err
	}
	err = directory.Sync()
	closed = directory.Close()
	if err == nil {
		err = closed
	}
	return err
}

func (s *fileEpochFenceStore) run(ctx context.Context, id epochFenceIdentity, bind bool, work func(*epochFenceRecord, func() error) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id.Proposal != s.plan.Proposal || id.Actor != s.plan.Actor || id.Epoch != s.plan.Epoch || id.Region != s.plan.Region {
		return errors.New("natsdriver: recovery proposal authority mismatch")
	}
	if err := privateEpochDirectory(filepath.Dir(s.path)); err != nil {
		return err
	}
	fd, err := syscall.Open(s.path+".lock", syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	lock := os.NewFile(uintptr(fd), s.path+".lock")
	defer func() {
		if cleanupErr := lock.Close(); cleanupErr != nil {
			log.Printf("cleanup lock.Close: %v", cleanupErr)
		}
	}()
	info, err := lock.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return errors.New("natsdriver: private journal lock required")
	}
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("natsdriver: recovery operation already running")
	}
	defer func() {
		if cleanupErr := syscall.Flock(fd, syscall.LOCK_UN); cleanupErr != nil {
			log.Printf("cleanup syscall.Flock: %v", cleanupErr)
		}
	}()
	record := &epochFenceRecord{Identity: id, Revision: s.plan.Revision, State: valueApplying, Members: map[string]*epochMemberProgress{}}
	wire, err := readEpochJournal(s.path)
	if err == nil {
		if json.Unmarshal(wire, record) != nil || record.Identity != id || record.Revision != s.plan.Revision || record.Members == nil || (record.State != valueApplying && record.State != valueFailed && record.State != valueVerified) {
			return errors.New("natsdriver: durable recovery journal authority mismatch")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	save := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := validateEpochRecord(record, bind); err != nil {
			return err
		}
		return writeEpochJournal(s.path, record)
	}
	if record.Result != nil {
		record.Result.ProofSHA256 = record.ProofSHA256
	}
	if err = validateEpochRecord(record, false); err != nil {
		return err
	}
	err = work(record, save)
	if err != nil {
		record.State = valueFailed
		record.Result = nil
		record.BoundProof = ""
		_ = writeEpochJournal(s.path, record)
	}
	return err
}

func validateEpochRecord(record *epochFenceRecord, bind bool) error {
	if record.State != valueVerified {
		return nil
	}
	if record.Result == nil || record.Result.Proposal != record.Identity.Proposal || record.Result.Epoch != record.Identity.Epoch || record.Result.Region != record.Identity.Region || record.Result.ConfigurationSHA256 != record.Identity.ConfigurationSHA256 || len(record.Result.Evidence) == 0 || len(record.Result.Transitions) != len(record.Members) {
		return errors.New("natsdriver: incomplete durable fence proof")
	}
	wire, err := json.Marshal(record.Result)
	if err != nil || epochSHA(wire) != record.Result.ProofSHA256 {
		return errors.New("natsdriver: durable fence proof mismatch")
	}
	for _, progress := range record.Members {
		if progress == nil || progress.Stage != valueVerified || len(progress.Evidence) == 0 {
			return errors.New("natsdriver: unverified member in aggregate proof")
		}
	}
	if bind && record.BoundProof != record.Result.ProofSHA256 {
		return errors.New("natsdriver: proof consumption mismatch")
	}
	return nil
}

type postgresEpochFenceStore struct {
	pool   *pgxpool.Pool
	schema string
}

func (s *postgresEpochFenceStore) run(ctx context.Context, id epochFenceIdentity, bind bool, work func(*epochFenceRecord, func() error) error) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	lockKey := fmt.Sprintf("%s:%d:%s", s.schema, id.Epoch, id.Region)
	var locked bool
	if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(hashtextextended($1,0))", lockKey).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return errors.New("natsdriver: recovery operation already running")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var unlocked bool
		if conn.QueryRow(cleanup, "SELECT pg_advisory_unlock(hashtextextended($1,0))", lockKey).Scan(&unlocked) != nil || !unlocked {
			_ = conn.Conn().Close(cleanup)
		}
	}()
	record := &epochFenceRecord{Identity: id, State: valueApplying, Members: map[string]*epochMemberProgress{}}
	transact := func(action func(pgx.Tx) error) error {
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = tx.Rollback(cleanup)
		}()
		if _, err = tx.Exec(ctx, "SELECT "+s.schema+".assert_epoch($1,$2)", id.Epoch, id.Region); err != nil {
			return err
		}
		var actor, state string
		var epoch, revision int64
		if err = tx.QueryRow(ctx, "SELECT expected_epoch,revision,actor,state FROM "+s.schema+".recovery_proposals WHERE proposal_id=$1 FOR UPDATE", id.Proposal).Scan(&epoch, &revision, &actor, &state); err != nil {
			return err
		}
		if epoch != id.Epoch || actor != id.Actor || (state != "proposed" && state != "validated") || (record.Revision != 0 && record.Revision != revision) {
			return errors.New("natsdriver: recovery proposal authority mismatch")
		}
		if bind {
			var session, current string
			if err = tx.QueryRow(ctx, "SELECT session_user,current_user").Scan(&session, &current); err != nil {
				return err
			}
			if session != id.Actor || current != session {
				return errors.New("natsdriver: authenticated recovery principal required")
			}
		}
		record.Revision = revision
		if err = action(tx); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	err = transact(func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO "+s.schema+".recovery_broker_fences(proposal_id,expected_epoch,region,revision,actor,configuration_sha256,state) VALUES($1,$2,$3,$4,$5,$6,'applying') ON CONFLICT(proposal_id) DO NOTHING", id.Proposal, id.Epoch, id.Region, record.Revision, id.Actor, id.ConfigurationSHA256)
		if err != nil {
			return err
		}
		var got epochFenceIdentity
		var revision int64
		var wire []byte
		var proof *string
		err = tx.QueryRow(ctx, "SELECT proposal_id,expected_epoch,region,revision,actor,configuration_sha256,state,evidence,proof_sha256 FROM "+s.schema+".recovery_broker_fences WHERE proposal_id=$1 FOR UPDATE", id.Proposal).Scan(&got.Proposal, &got.Epoch, &got.Region, &revision, &got.Actor, &got.ConfigurationSHA256, &record.State, &wire, &proof)
		if err != nil {
			return err
		}
		if got != id || revision != record.Revision {
			return errors.New("natsdriver: durable broker fence authority mismatch")
		}
		if len(wire) > 0 {
			if record.State == valueVerified {
				var result EpochFenceResult
				if json.Unmarshal(wire, &result) != nil || proof == nil || result.Transitions == nil {
					return errors.New("natsdriver: durable broker proof cannot resume")
				}
				canonical, marshalErr := json.Marshal(result)
				if marshalErr != nil || epochSHA(canonical) != *proof {
					return errors.New("natsdriver: durable broker proof identity mismatch")
				}
				result.ProofSHA256 = *proof
				record.Result = &result
				record.Members = result.Transitions
			} else {
				var saved epochFenceRecord
				if json.Unmarshal(wire, &saved) != nil || saved.Identity != id || saved.Revision != revision || saved.Members == nil {
					return errors.New("natsdriver: durable member progress mismatch")
				}
				record.Members = saved.Members
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	save := func() error {
		return transact(func(tx pgx.Tx) error {
			if err := validateEpochRecord(record, bind); err != nil {
				return err
			}
			var wire []byte
			var err error
			var proof any
			if record.State == valueVerified {
				wire, err = json.Marshal(record.Result)
				proof = record.Result.ProofSHA256
			} else {
				wire, err = json.Marshal(record)
			}
			if err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, "UPDATE "+s.schema+".recovery_broker_fences SET state=$2,evidence=$3::jsonb,proof_sha256=$4,failure_code=CASE WHEN $2='failed' THEN 'BROKER_EPOCH_FENCE_FAILED' ELSE NULL END,verified_at=CASE WHEN $2='verified' THEN clock_timestamp() ELSE NULL END WHERE proposal_id=$1 AND configuration_sha256=$5", id.Proposal, record.State, string(wire), proof, id.ConfigurationSHA256)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return errors.New("natsdriver: broker fence confirmation conflict")
			}
			if bind && record.State == valueVerified {
				// Read the durable row, rather than binding an unchecked in-memory digest.
				tag, err = tx.Exec(ctx, "UPDATE "+s.schema+".recovery_proposals p SET fence_proof_sha256=f.proof_sha256 FROM "+s.schema+".recovery_broker_fences f WHERE p.proposal_id=$1 AND f.proposal_id=p.proposal_id AND f.state='verified' AND f.proof_sha256=$2 AND f.actor=p.actor AND f.expected_epoch=p.expected_epoch AND f.revision=p.revision", id.Proposal, record.BoundProof)
				if err != nil {
					return err
				}
				if tag.RowsAffected() != 1 {
					return fmt.Errorf("natsdriver: durable fence consumption conflict")
				}
			}
			return nil
		})
	}
	err = work(record, save)
	if err != nil {
		record.State = valueFailed
		record.Result = nil
		record.BoundProof = ""
		_ = save()
	}
	return err
}

func lockEpochMembers(members []EpochMemberAuthority) (func(), error) {
	files := []*os.File{}
	release := func() {
		for i := len(files) - 1; i >= 0; i-- {
			_ = syscall.Flock(int(files[i].Fd()), syscall.LOCK_UN)
			_ = files[i].Close()
		}
	}
	for _, member := range members {
		if err := privateEpochDirectory(filepath.Dir(member.ConfigPath)); err != nil {
			release()
			return nil, err
		}
		fd, err := syscall.Open(member.ConfigPath+".epoch.lock", syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW, 0600)
		if err != nil {
			release()
			return nil, err
		}
		file := os.NewFile(uintptr(fd), member.ConfigPath+".epoch.lock")
		files = append(files, file)
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			release()
			return nil, errors.New("natsdriver: private member transition lock required")
		}
		if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			release()
			return nil, errors.New("natsdriver: member transition already running")
		}
	}
	return release, nil
}
