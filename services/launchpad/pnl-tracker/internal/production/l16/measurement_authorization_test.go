package l16

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInventoryMeasurementJournalsAuthorizationRejections(t *testing.T) {
	ctx := context.Background()
	service, _ := recordedRebuildService(t, ctx)
	worker, plan := measurementPlanFixture(t)
	identity, principal := inventoryIdentityFixture(t, realMarketCreator)
	worker.service, worker.identity = service, identity
	plan.RequestsPerSecond, plan.Concurrency, plan.MaxDurationSeconds = 20, 32, 15
	requests := 0
	for i := range plan.Cases {
		if plan.Cases[i].Wallet == principal.Wallet {
			t.Fatal("authorization case must request a different wallet")
		}
		plan.Cases[i].ExpectedError = reasonAuthorization
		requests += plan.Cases[i].Requests
	}
	plan = bindMeasurementPlan(t, plan)
	output := filepath.Join(t.TempDir(), "measurement")
	manifest, err := RunInventoryMeasurement(ctx, worker, &http.Client{Timeout: time.Second}, plan, output)
	if err == nil || manifest.Offered != requests || manifest.Errors != requests || manifest.ExpectedAuthorizationRejections != requests || manifest.Completed != 0 || manifest.UnexpectedErrors != 0 || manifest.DependencyBlockedOperations != 0 || manifest.GeneratorSaturated != 0 || len(manifest.Failures) == 0 || manifest.GlobalAcceptanceQualified {
		t.Fatalf("authorization manifest = %+v, error = %v", manifest, err)
	}
	if manifest.ServiceSampleCount != 0 || manifest.OriginSampleCount != 0 || manifest.HTTPRoundTripSampleCount != 0 || manifest.ServiceP95NS != nil || manifest.OriginP95NS != nil || manifest.CorrectedP99NS != nil {
		t.Fatal("rejected requests contributed successful latency observations")
	}
	for _, artifact := range manifest.Artifacts {
		if _, err := measurementReadFile(artifact, output); err != nil {
			t.Fatal(artifact, err)
		}
	}
	journal, err := os.Open(filepath.Join(output, "samples.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = journal.Close() }()
	decoder := json.NewDecoder(journal)
	counts := map[string]int{}
	for sequence := 1; sequence <= requests; sequence++ {
		var sample InventoryMeasurementSample
		if err := decoder.Decode(&sample); err != nil {
			t.Fatal(err)
		}
		if sample.Sequence != sequence || sample.ErrorClass != reasonAuthorization || !sample.ExpectedErrorMatched || sample.Payload != nil || sample.PayloadSHA256 != "" || sample.HTTPStatus != 0 || sample.ServiceElapsedNS <= 0 || sample.FinishedOffsetNS < sample.StartOffsetNS {
			t.Fatalf("authorization sample = %+v", sample)
		}
		counts[sample.CaseID]++
	}
	if err := decoder.Decode(new(InventoryMeasurementSample)); err != io.EOF {
		t.Fatalf("journal trailing sample: %v", err)
	}
	for _, c := range plan.Cases {
		if counts[c.ID] != c.Requests {
			t.Fatalf("case %s has %d samples, want %d", c.ID, counts[c.ID], c.Requests)
		}
	}
}
