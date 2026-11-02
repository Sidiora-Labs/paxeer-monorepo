import assert from "node:assert/strict";
import { existsSync, readFileSync, statSync } from "node:fs";
import test from "node:test";

const root = new URL("../../../../", import.meta.url);

const retiredSurfaces = [
  {
    id: "interface.15.met-legacy-x-slashing-abci-29-j56",
    path: "sdk/x/slashing/abci.go",
    pattern: /\bModuleMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-slashing-keeper-keeper-74-j56",
    path: "sdk/x/slashing/keeper/keeper.go",
    pattern: /\bIncrValidatorSlashedCounter\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-slashing-keeper-infractions-143-j56",
    path: "sdk/x/slashing/keeper/infractions.go",
    pattern: /\bIncrValidatorSlashedCounter\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-capability-abci-22-j56",
    path: "sdk/x/capability/abci.go",
    pattern: /\bModuleMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-auth-vesting-msg-server-89-j56",
    path: "sdk/x/auth/vesting/msg_server.go",
    pattern: /\bIncrCounter\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-auth-vesting-msg-server-95-j56",
    path: "sdk/x/auth/vesting/msg_server.go",
    pattern: /\bSetGaugeWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-upgrade-abci-36-j56",
    path: "sdk/x/upgrade/abci.go",
    pattern: /\bModuleMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-upgrade-abci-64-j56",
    path: "sdk/x/upgrade/abci.go",
    pattern: /\bSetGaugeWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-upgrade-keeper-keeper-207-j56",
    path: "sdk/x/upgrade/keeper/keeper.go",
    pattern: /\bSetGaugeWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-distribution-abci-20-j56",
    path: "sdk/x/distribution/abci.go",
    pattern: /\bModuleMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-distribution-keeper-msg-server-75-j56",
    path: "sdk/x/distribution/keeper/msg_server.go",
    pattern: /\bSetGaugeWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-distribution-keeper-msg-server-111-j56",
    path: "sdk/x/distribution/keeper/msg_server.go",
    pattern: /\bSetGaugeWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-bank-keeper-msg-server-66-j56",
    path: "sdk/x/bank/keeper/msg_server.go",
    pattern: /\bSetGaugeWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-staking-abci-20-j56",
    path: "sdk/x/staking/abci.go",
    pattern: /\bModuleMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-staking-abci-32-j56",
    path: "sdk/x/staking/abci.go",
    pattern: /\bModuleMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-staking-keeper-msg-server-230-j56",
    path: "sdk/x/staking/keeper/msg_server.go",
    pattern: /\bIncrCounter\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-staking-keeper-msg-server-233-j56",
    path: "sdk/x/staking/keeper/msg_server.go",
    pattern: /\bSetGaugeWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-staking-keeper-msg-server-299-j56",
    path: "sdk/x/staking/keeper/msg_server.go",
    pattern: /\bIncrCounter\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-staking-keeper-msg-server-302-j56",
    path: "sdk/x/staking/keeper/msg_server.go",
    pattern: /\bSetGaugeWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-staking-keeper-msg-server-365-j56",
    path: "sdk/x/staking/keeper/msg_server.go",
    pattern: /\bIncrCounter\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-staking-keeper-msg-server-368-j56",
    path: "sdk/x/staking/keeper/msg_server.go",
    pattern: /\bSetGaugeWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-gov-abci-22-j56",
    path: "sdk/x/gov/abci.go",
    pattern: /\bModuleMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-gov-keeper-msg-server-37-j56",
    path: "sdk/x/gov/keeper/msg_server.go",
    pattern: /\bIncrCounter\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-gov-keeper-msg-server-81-j56",
    path: "sdk/x/gov/keeper/msg_server.go",
    pattern: /\bIncrCounterWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-gov-keeper-msg-server-115-j56",
    path: "sdk/x/gov/keeper/msg_server.go",
    pattern: /\bIncrCounterWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-gov-keeper-msg-server-149-j56",
    path: "sdk/x/gov/keeper/msg_server.go",
    pattern: /\bIncrCounterWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-x-evidence-abci-23-j56",
    path: "sdk/x/evidence/abci.go",
    pattern: /\bModuleMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-types-module-module-614-j56",
    path: "sdk/types/module/module.go",
    pattern: /\bMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-types-module-module-625-j56",
    path: "sdk/types/module/module.go",
    pattern: /\bModuleMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-tasks-scheduler-271-j56",
    path: "sdk/tasks/scheduler.go",
    pattern: /\bIncrCounter\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-tasks-scheduler-274-j56",
    path: "sdk/tasks/scheduler.go",
    pattern: /\bIncrCounter\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-storev2-rootmulti-store-146-j56",
    path: "sdk/storev2/rootmulti/store.go",
    pattern: /\bMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-storev2-rootmulti-store-219-j56",
    path: "sdk/storev2/rootmulti/store.go",
    pattern: /\bSetGauge\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-storev2-rootmulti-store-229-j56",
    path: "sdk/storev2/rootmulti/store.go",
    pattern: /\bSetGauge\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-storev2-rootmulti-store-738-j56",
    path: "sdk/storev2/rootmulti/store.go",
    pattern: /\bIncrCounterWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-storev2-rootmulti-store-751-j56",
    path: "sdk/storev2/rootmulti/store.go",
    pattern: /\bIncrCounterWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-storev2-rootmulti-store-1046-j56",
    path: "sdk/storev2/rootmulti/store.go",
    pattern: /\bSetGaugeWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-storev2-rootmulti-store-1055-j56",
    path: "sdk/storev2/rootmulti/store.go",
    pattern: /\bSetGaugeWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-storev2-rootmulti-store-1064-j56",
    path: "sdk/storev2/rootmulti/store.go",
    pattern: /\bSetGaugeWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-storev2-rootmulti-store-1093-j56",
    path: "sdk/storev2/rootmulti/store.go",
    pattern: /\bIncrCounter\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-store-types-cache-73-j56",
    path: "sdk/store/types/cache.go",
    pattern: /\bSetGaugeWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-store-types-gas-124-j56",
    path: "sdk/store/types/gas.go",
    pattern: /\bIncrCounterWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-abci-111-j56",
    path: "sdk/baseapp/abci.go",
    pattern: /\bMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-abci-128-j56",
    path: "sdk/baseapp/abci.go",
    pattern: /\bMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-abci-188-j56",
    path: "sdk/baseapp/abci.go",
    pattern: /\bMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-abci-191-j56",
    path: "sdk/baseapp/abci.go",
    pattern: /\bIncrCounter\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-abci-194-j56",
    path: "sdk/baseapp/abci.go",
    pattern: /\bIncrCounter\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-abci-197-j56",
    path: "sdk/baseapp/abci.go",
    pattern: /\bSetGauge\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-abci-200-j56",
    path: "sdk/baseapp/abci.go",
    pattern: /\bSetGauge\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-abci-274-j56",
    path: "sdk/baseapp/abci.go",
    pattern: /\bMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-abci-440-j56",
    path: "sdk/baseapp/abci.go",
    pattern: /\bMeasureSinceWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-abci-991-j56",
    path: "sdk/baseapp/abci.go",
    pattern: /\bMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-abci-1058-j56",
    path: "sdk/baseapp/abci.go",
    pattern: /\bMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-abci-1121-j56",
    path: "sdk/baseapp/abci.go",
    pattern: /\bMeasureSince\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-baseapp-858-j56",
    path: "sdk/baseapp/baseapp.go",
    pattern: /\bMeasureThroughputSinceWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-baseapp-1019-j56",
    path: "sdk/baseapp/baseapp.go",
    pattern: /\bMeasureThroughputSinceWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-baseapp-1062-j56",
    path: "sdk/baseapp/baseapp.go",
    pattern: /\bMeasureSinceWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-legacy-baseapp-baseapp-1082-j56",
    path: "sdk/baseapp/baseapp.go",
    pattern: /\bMeasureSinceWithLabels\s*\(/u,
  },
  {
    id: "interface.15.met-tel-pax-cosmos-validator-slashed",
    path: "sdk/telemetry/metrics.go",
    pattern: /\bpax_cosmos_validator_slashed\b|["`]pax["`]\s*,\s*["`]cosmos["`]\s*,\s*["`]validator["`]\s*,\s*["`]slashed["`]/u,
  },
  {
    id: "interface.15.met-tel-cosmos-upgrade-plan-height",
    path: "sdk/telemetry/metrics.go",
    pattern: /\bcosmos_upgrade_plan_height\b|["`]cosmos["`]\s*,\s*["`]upgrade["`]\s*,\s*["`]plan["`]\s*,\s*["`]height["`]/u,
  },
];

const retiredHelpers = [
  {
    id: "interface.15.go-telemetry-incr-validator-slashed-counter-j56",
    path: "sdk/telemetry/wrapper.go",
    pattern: /\bIncrValidatorSlashedCounter\s*\(/u,
  },
];

const plannedPaths = [
  "sdk/x/slashing/keeper/legacy_metrics_test.go",
  "sdk/x/capability/legacy_metrics_test.go",
  "sdk/x/auth/vesting/legacy_metrics_test.go",
  "sdk/x/upgrade/keeper/legacy_metrics_test.go",
  "sdk/x/distribution/keeper/legacy_metrics_test.go",
  "sdk/x/bank/keeper/legacy_send_amount_test.go",
  "sdk/x/staking/legacy_metrics_test.go",
  "sdk/x/staking/keeper/legacy_metrics_test.go",
  "sdk/x/gov/keeper/legacy_metrics_test.go",
  "sdk/x/evidence/legacy_metrics_test.go",
  "sdk/types/module/legacy_metrics_test.go",
  "sdk/tasks/legacy_metrics_test.go",
  "sdk/storev2/rootmulti/legacy_metrics_test.go",
  "sdk/store/types/legacy_metrics_test.go",
  "sdk/baseapp/legacy_abci_metrics_test.go",
  "sdk/baseapp/legacy_runtx_metrics_test.go",
  "sdk/telemetry/legacy_definitions_test.go",
];

function withoutGoComments(source) {
  return source.replace(
    /"(?:\\.|[^"\\])*"|`[^`]*`|'(?:\\.|[^'\\])*'|\/\/[^\r\n]*|\/\*[\s\S]*?\*\//gu,
    (token) =>
      token.startsWith("//") || token.startsWith("/*") ? " " : token,
  );
}

function cosmos_fork_completion_retired_surfaces_absent() {
  assert.equal(retiredSurfaces.length, 60, "all 60 retired interfaces are gated");
  assert.equal(new Set(retiredSurfaces.map(({ id }) => id)).size, 60);
  assert.equal(retiredHelpers.length, 1, "the retired Go helper is also gated");
  const allRetiredSurfaces = [...retiredSurfaces, ...retiredHelpers];
  assert.equal(new Set(allRetiredSurfaces.map(({ id }) => id)).size, 61);
  assert.equal(plannedPaths.length, 17, "all 17 planned paths are gated");
  assert.equal(new Set(plannedPaths).size, 17);

  const failures = [];
  const sources = new Map();
  for (const { id, path, pattern } of allRetiredSurfaces) {
    const url = new URL(path, root);
    if (!existsSync(url)) continue;
    let source = sources.get(path);
    if (source === undefined) {
      source = withoutGoComments(readFileSync(url, "utf8"));
      sources.set(path, source);
    }
    if (pattern.test(source)) {
      failures.push(`Retired surface ${id} remains at ${path}`);
    }
  }

  for (const path of plannedPaths) {
    const url = new URL(path, root);
    if (!existsSync(url) || !statSync(url).isFile()) {
      failures.push(`Missing planned file: ${path}`);
    }
  }

  assert.deepEqual(failures, [], "Domain 15 completion blocked");
}

test(cosmos_fork_completion_retired_surfaces_absent);
