package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestDurableWorkSelectionRespectsStateAndBackoff(t *testing.T) {
	env := repairM1020EnvironmentFor(t, nil)
	objects := map[string]json.RawMessage{}
	put := func(kind, key string, value any) {
		t.Helper()
		wire, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		objects[kind+"/"+key] = wire
	}
	for _, key := range []string{"alpha", "beta"} {
		put(controlSettlement, key, Obligation{ID: id(), Scope: "service", IdempotencyKey: key, State: controlPending})
	}
	put(controlSettlement, "finished", Obligation{ID: id(), Scope: "service", IdempotencyKey: "finished", State: controlSettled})
	put(controlSettlement, "later", Obligation{ID: id(), Scope: "service", IdempotencyKey: "later", State: controlUnknown})
	put("work_schedule", controlSettlement+".later", workSchedule{Next: time.Now().Add(time.Hour)})
	put(controlIncident, "contained", Incident{ID: "contained", Scope: "service", State: controlContained})
	put("incident_outbox", "contained", map[string]string{controlState: controlPending})
	put(controlIncident, "completed", Incident{ID: "completed", Scope: "service", State: controlContained})
	put("incident_outbox", "completed", map[string]string{controlState: controlCompleted})
	put(controlJob, "queued", ReplayJob{ActionKey: "queued", Scope: "service", State: "queued", Kind: controlReplay})
	put(controlJob, "done", ReplayJob{ActionKey: "done", Scope: "service", State: controlSucceeded, Kind: controlReplay})
	if err := env.store.local.save(objects, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := env.store.pendingWork(context.Background(), "unknown"); !errors.Is(err, Invalid) {
		t.Fatalf("unknown work kind: %v", err)
	}
	settlements, err := env.store.pendingWork(context.Background(), controlSettlement)
	if err != nil || len(settlements) != 2 || settlements[0].Key != "alpha" || settlements[1].Key != "beta" {
		t.Fatalf("eligible settlements: %+v, %v", settlements, err)
	}
	incidents, err := env.store.pendingWork(context.Background(), controlIncident)
	if err != nil || len(incidents) != 1 || incidents[0].Key != "contained" {
		t.Fatalf("eligible incident: %+v, %v", incidents, err)
	}
	jobs, err := env.store.pendingWork(context.Background(), controlJob)
	if err != nil || len(jobs) != 1 || jobs[0].Key != "queued" {
		t.Fatalf("eligible replay job: %+v, %v", jobs, err)
	}
	claimed, err := env.store.claimWork(context.Background(), env.credentials["finance"], controlSettlement, controlSettlement)
	if err != nil || len(claimed) != 2 || claimed[0].Key != "alpha" || claimed[1].Key != "beta" {
		t.Fatalf("claimed settlement work: %+v, %v", claimed, err)
	}
	for _, key := range []string{"alpha", "beta"} {
		schedule := repairM1020Read[workSchedule](t, env.store, "work_schedule", controlSettlement+"."+key)
		if schedule.Attempts != 1 || !schedule.Next.After(schedule.Last) {
			t.Fatalf("durable retry schedule %s: %+v", key, schedule)
		}
	}
	if pending, err := env.store.pendingWork(context.Background(), controlSettlement); err != nil || len(pending) != 0 {
		t.Fatalf("claimed work remained immediately eligible: %+v, %v", pending, err)
	}
	objects = make(map[string]json.RawMessage, len(env.store.local.objects))
	for key, body := range env.store.local.objects {
		objects[key] = body
	}
	for n := 0; n < 101; n++ {
		put(controlSettlement, fmt.Sprintf("bulk-%03d", n), Obligation{ID: id(), Scope: "service", State: controlPending})
	}
	if err := env.store.local.save(objects, nil); err != nil {
		t.Fatal(err)
	}
	if pending, err := env.store.pendingWork(context.Background(), controlSettlement); err != nil || len(pending) != 100 || pending[0].Key != "bulk-000" || pending[99].Key != "bulk-099" {
		t.Fatalf("bounded work page: %d, %v", len(pending), err)
	}
}
