import assert from "node:assert/strict";
import { existsSync, readFileSync, statSync } from "node:fs";
import test from "node:test";

const root = new URL("../../../", import.meta.url);
const routerPath =
  "services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex";
const dataBoundaryPath = "tests/explorer/paxeer-x-data-boundary.py";

const retiredRoutes = [
  {
    id: "interface.14.http-get-api-v2-paxeer-x-anchors",
    pattern: /\bget\s*\(?\s*["'](?:\/api\/v2\/paxeer-x|\/paxeer-x)?\/anchors["']\s*,/u,
  },
  {
    id: "interface.14.http-get-api-v2-paxeer-x-receipts",
    pattern: /\bget\s*\(?\s*["'](?:\/api\/v2\/paxeer-x|\/paxeer-x)?\/receipts["']\s*,/u,
  },
  {
    id: "interface.14.http-get-api-v2-paxeer-x-receipts-id",
    pattern: /\bget\s*\(?\s*["'](?:\/api\/v2\/paxeer-x|\/paxeer-x)?\/receipts\/:[^"']+["']\s*,/u,
  },
] as const;

const retiredEnvironmentVariables = [
  "LAYERX_KERNEL_RECEIPT_RELAY_FIXTURE",
  "LAYERX_KERNEL_RECEIPT_SUBMISSION_AUTHORIZATION",
  "LAYERX_KERNEL_RECEIPT_SUBMISSIONS_DIR",
  "LAYERX_EXPLORER_PROGRAM_LISTEN",
  "LAYERX_EXPLORER_PROGRAM_BEARER_TOKEN",
  "LAYERX_EXPLORER_NODE_BEARER_TOKEN",
  "LAYERX_EXPLORER_AUTHORITY_BEARER_TOKEN",
  "LAYERX_EXPLORER_PROGRAM_MAX_STALENESS_MS",
  "LAYERX_EXPLORER_NODE_ENDPOINT",
  "LAYERX_EXPLORER_AUTHORITY_ENDPOINT",
  "LAYERX_EXPLORER_AUTHORITY_CA_DER",
  "LAYERX_EXPLORER_AUTHORITY_REPLICA_ID",
  "LAYERX_EXPLORER_SEQUENCER_TRUST_HISTORY",
  "LAYERX_EXPLORER_DEPLOYMENT_JOURNAL",
  "LAYERX_EXPLORER_VERIFIED_SOURCE_STORE",
  "LAYERX_EXPLORER_PROGRAM_PROBE_ID",
  "LAYERX_EXPLORER_GENESIS_TRUST",
  "LAYERX_EXPLORER_FINALITY_POLICY",
  "LAYERX_EXPLORER_READ_KEY_FILE",
  "LAYERX_EXPLORER_READ_ENDPOINT",
  "LAYERX_EXPLORER_READ_CA_DER",
  "LAYERX_EXPLORER_READ_SEQUENCER_PUBLIC_KEY_FILE",
  "LAYERX_EXPLORER_READ_NETWORK_ID",
  "LAYERX_EXPLORER_READ_FEE_LIMIT",
  "LAYERX_EXPLORER_NAMING_PROGRAM",
  "LAYERX_NETWORK_GATEWAY_ENDPOINT",
  "LAYERX_EXPLORER_LNI_SOCKET",
  "LAYERX_EXPLORER_NETWORK_ID",
  "LAYERX_EXPLORER_PROTOCOL_VERSION",
  "LAYERX_EXPLORER_INGEST_INTERVAL_MS",
  "LAYERX_EXPLORER_INGEST_CURSOR",
  "PAXEER_X_LAYERX_SEND_CMD",
  "PAXEER_X_LAYERX_ASSETS",
  "PAXEER_X_STANDALONE_RECEIPTS_ACCOUNT",
  "PAXEER_X_NODE_PRIVATE_DIR",
  "LAYERX_EXPLORER_INDEX_BIN",
] as const;

const retiredSurfaces = [
  ...retiredRoutes.map((route) => ({ ...route, path: routerPath })),
  ...retiredEnvironmentVariables.map((name) => ({
    id: `interface.14.env-test-${name.toLowerCase().replaceAll("_", "-")}`,
    path: dataBoundaryPath,
    pattern: new RegExp(`\\b${name}\\b`, "u"),
  })),
];

const plannedPaths = [
  "apps/web/e2e/copy-verify-retired.test.ts",
  "apps/explorer/mocks/paxeerX/capabilities.spec.ts",
  "services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/retired_routes_test.exs",
  "services/explorer/backend/apps/block_scout_web/test/block_scout_web/views/api/v2/paxeer_x/unified_account_view_gone_test.exs",
  "tests/explorer_fixture_test.py",
  "tests/explorer/test_paxeer_x_data_boundary_retired_cases.py",
  "apps/explorer/types/api/paxeerXArchive.ts",
  "apps/explorer/lib/api/services/general/paxeerXLists.spec.ts",
  "apps/explorer/lib/hooks/useNavItemsArchive.spec.tsx",
  "apps/explorer/ui/tx/details/TxDetailsSequencerSignature.tsx",
  "apps/explorer/ui/tx/details/TxDetailsSequencerSignature.spec.tsx",
  "services/explorer/tests/test_frontend_env_example.py",
] as const;

function web_completion_retired_surfaces_absent() {
  assert.equal(retiredSurfaces.length, 39, "all 39 retired interfaces are gated");
  assert.equal(new Set(retiredSurfaces.map(({ id }) => id)).size, 39);
  assert.equal(plannedPaths.length, 12, "all 12 planned paths are gated");
  assert.equal(new Set(plannedPaths).size, 12);

  const failures: string[] = [];
  const sources = new Map<string, string>();
  for (const { id, path, pattern } of retiredSurfaces) {
    const url = new URL(path, root);
    if (!existsSync(url)) continue;
    let source = sources.get(path);
    if (source === undefined) {
      source = readFileSync(url, "utf8");
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

  assert.deepEqual(failures, [], "Domain 14 completion blocked");
}

test(web_completion_retired_surfaces_absent);
