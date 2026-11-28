import assert from "node:assert/strict";
import test from "node:test";
import {
  claimedReadTarget,
  rewardsLedgerEndpoint,
} from "../packages/hooks/src/trading-rewards/claimedTarget";

function noEnvironmentNamesAnEndpoint() {
  for (const env of ["prod", "staging", "qa", "dev", "other"]) {
    assert.equal(rewardsLedgerEndpoint(env), "");
  }
  assert.equal(
    claimedReadTarget(rewardsLedgerEndpoint("prod"), "ledger"),
    null,
  );
}

test(noEnvironmentNamesAnEndpoint);

function withoutAnEndpointThereIsNoTarget() {
  for (const endpoint of ["", "   ", undefined, null, 5]) {
    assert.equal(claimedReadTarget(endpoint, "ledger"), null);
  }
}

test(withoutAnEndpointThereIsNoTarget);

function withoutALedgerThereIsNoTarget() {
  for (const ledger of ["", "   ", undefined, null, 5]) {
    assert.equal(claimedReadTarget("endpoint", ledger), null);
  }
}

test(withoutALedgerThereIsNoTarget);

function aTargetCarriesEndpointAndLedgerTrimmed() {
  assert.deepEqual(claimedReadTarget(" endpoint ", " ledger "), {
    endpoint: "endpoint",
    ledger: "ledger",
  });
}

test(aTargetCarriesEndpointAndLedgerTrimmed);
