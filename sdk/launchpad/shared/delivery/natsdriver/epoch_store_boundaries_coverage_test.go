package natsdriver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestEpochStoreRejectsChangedDurableAuthority(t *testing.T) {
	pool, schema := epochPostgresFixture(t)
	ctx := context.Background()
	quoted := pgx.Identifier{schema}.Sanitize()
	store := &postgresEpochFenceStore{pool: pool, schema: quoted}
	for _, mode := range []string{"actor", "bind principal", "revision", "configuration", "progress", "proof shape", "proof digest", "confirmation"} {
		t.Run(mode, func(t *testing.T) {
			proposal := "authority-" + strings.ReplaceAll(mode, " ", "-")
			if _, err := pool.Exec(ctx, "INSERT INTO "+quoted+".recovery_proposals(proposal_id,expected_epoch,next_region,revision,checkpoint_sha256,state,actor) VALUES($1,3,'region-recovery',2,$2,'validated','fixture-operator')", proposal, strings.Repeat("a", 64)); err != nil {
				t.Fatal(err)
			}
			id := epochFenceIdentity{Proposal: proposal, Actor: "fixture-operator", Epoch: 3, Region: "region-primary", ConfigurationSHA256: strings.Repeat("a", 64)}
			called := false
			work := func(record *epochFenceRecord, save func() error) error {
				called = true
				record.Members["member"] = &epochMemberProgress{Stage: "prepared", Acks: map[string]string{}}
				return save()
			}
			if mode == "actor" {
				id.Actor = "other"
			}
			if mode == "revision" || mode == "configuration" || mode == "progress" || mode == "proof shape" || mode == "proof digest" {
				var revision int64 = 2
				config := id.ConfigurationSHA256
				state := "applying"
				evidence := `{}`
				var proof any
				switch mode {
				case "revision":
					revision = 3
				case "configuration":
					config = strings.Repeat("b", 64)
				case "proof shape":
					state = "verified"
					proof = strings.Repeat("c", 64)
				case "proof digest":
					state = "verified"
					proof = strings.Repeat("c", 64)
					evidence = `{"transitions":{}}`
				}
				if _, err := pool.Exec(ctx, "INSERT INTO "+quoted+".recovery_broker_fences(proposal_id,expected_epoch,region,revision,actor,configuration_sha256,state,evidence,proof_sha256,verified_at) VALUES($1,3,'region-primary',$2,'fixture-operator',$3,$4,$5::jsonb,$6,clock_timestamp())", proposal, revision, config, state, evidence, proof); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "confirmation" {
				work = func(record *epochFenceRecord, save func() error) error {
					called = true
					if _, err := pool.Exec(ctx, "UPDATE "+quoted+".recovery_broker_fences SET configuration_sha256=$2 WHERE proposal_id=$1", proposal, strings.Repeat("b", 64)); err != nil {
						return err
					}
					return save()
				}
			}
			err := store.run(ctx, id, mode == "bind principal", work)
			if err == nil {
				t.Fatal("changed durable epoch authority admitted")
			}
			if mode != "confirmation" && called {
				t.Fatal("invalid epoch authority reached member transition")
			}
		})
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := store.run(cancelled, epochFenceIdentity{}, false, func(*epochFenceRecord, func() error) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	id := epochFenceIdentity{Proposal: "actual-cutover", Actor: "fixture-operator", Epoch: 3, Region: "region-primary", ConfigurationSHA256: strings.Repeat("a", 64)}
	key := fmt.Sprintf("%s:%d:%s", store.schema, id.Epoch, id.Region)
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1,0))", key); err != nil {
		t.Fatal(err)
	}
	if err := store.run(ctx, id, false, func(*epochFenceRecord, func() error) error {
		t.Fatal("concurrent epoch transition invoked")
		return nil
	}); err == nil {
		t.Fatal("concurrent epoch authority admitted")
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_unlock(hashtextextended($1,0))", key); err != nil {
		t.Fatal(err)
	}
}

func TestEpochStoreBindsAuthenticatedBrokerEvidence(t *testing.T) {
	member, _, _ := epochBrokerFixture(t)
	pool, schema := epochPostgresFixture(t)
	ctx := context.Background()
	quoted := pgx.Identifier{schema}.Sanitize()
	var actor string
	if err := pool.QueryRow(ctx, "SELECT current_user").Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+quoted+".recovery_proposals SET actor=$1 WHERE proposal_id='actual-cutover'", actor); err != nil {
		t.Fatal(err)
	}
	controller, err := NewBrokerEpochController(pool, schema, 3, "region-primary", []EpochMemberAuthority{member})
	if err != nil {
		t.Fatal(err)
	}
	result, err := controller.FenceAndBind(ctx, "actual-cutover", actor)
	if err != nil || result.ProofSHA256 == "" {
		t.Fatalf("authenticated broker proof: %+v %v", result, err)
	}
	var bound string
	if err := pool.QueryRow(ctx, "SELECT fence_proof_sha256 FROM "+quoted+".recovery_proposals WHERE proposal_id='actual-cutover'").Scan(&bound); err != nil || bound != result.ProofSHA256 {
		t.Fatalf("durable consumed proof: %s %v", bound, err)
	}

	var resumed EpochFenceResult
	durable := &postgresEpochFenceStore{pool: pool, schema: quoted}
	identity := epochFenceIdentity{Proposal: result.Proposal, Actor: actor, Epoch: result.Epoch, Region: result.Region, ConfigurationSHA256: result.ConfigurationSHA256}
	err = durable.run(ctx, identity, true, func(record *epochFenceRecord, save func() error) error {
		if record.Result == nil {
			return errors.New("verified broker proof absent")
		}
		resumed = *record.Result
		record.BoundProof = resumed.ProofSHA256
		return save()
	})
	if err != nil || resumed.ProofSHA256 != result.ProofSHA256 {
		t.Fatalf("resumed broker proof: %+v %v", resumed, err)
	}

	refreshed, err := controller.FenceAndBind(ctx, "actual-cutover", actor)
	if err != nil {
		t.Fatal(err)
	}
	expected := result
	expected.CheckedUnix = refreshed.CheckedUnix
	expected.ProofSHA256 = ""
	normalized := refreshed
	normalized.ProofSHA256 = ""
	canonical, err := json.Marshal(expected)
	if err != nil || refreshed.CheckedUnix < result.CheckedUnix || !reflect.DeepEqual(normalized, expected) || refreshed.ProofSHA256 != epochSHA(canonical) {
		t.Fatalf("refreshed broker proof: %+v %v", refreshed, err)
	}
	if err := pool.QueryRow(ctx, "SELECT fence_proof_sha256 FROM "+quoted+".recovery_proposals WHERE proposal_id='actual-cutover'").Scan(&bound); err != nil || bound != refreshed.ProofSHA256 {
		t.Fatalf("refreshed durable proof: %s %v", bound, err)
	}
	record := &epochFenceRecord{Identity: epochFenceIdentity{Proposal: result.Proposal, Epoch: result.Epoch, Region: result.Region, ConfigurationSHA256: result.ConfigurationSHA256}, State: valueVerified, Members: result.Transitions, Result: &result, BoundProof: result.ProofSHA256}
	if err := validateEpochRecord(record, true); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"identity", "proof", "member", "consumption"} {
		t.Run(mode, func(t *testing.T) {
			changed := *record
			copyResult := result
			changed.Result = &copyResult
			switch mode {
			case "identity":
				changed.Identity.Region = "other"
			case "proof":
				changed.Result.ProofSHA256 = strings.Repeat("0", 64)
			case "member":
				changed.Members = map[string]*epochMemberProgress{member.Name: nil}
			case "consumption":
				changed.BoundProof = strings.Repeat("0", 64)
			}
			if err := validateEpochRecord(&changed, true); err == nil {
				t.Fatal("changed proof admitted")
			}
		})
	}
}
