import { test, before, after } from "node:test";
import assert from "node:assert/strict";
import { spawn, execFileSync, type ChildProcess } from "node:child_process";
import {
  mkdtemp,
  mkdir,
  writeFile,
  chmod,
  chown,
  readdir,
  rm,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve, join } from "node:path";
import { createInterface } from "node:readline";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import type { GraphQLEndpoint } from "../src/core/graphql/client";
import type { TradeSettlement } from "../src/types/perps";
import type { SettlementCollateral } from "../src/widgets/portfolio/settlementCollateral";

let directory: string;
let postgresBin: string;
let postgresStarted = false;
let producer: ChildProcess | undefined;
let producerExit: Promise<number | null> | undefined;
let endpoint: GraphQLEndpoint;
let settlements: TradeSettlement[];
const collateral = new Map<string, SettlementCollateral>();

function postgres(command: string, args: string[]) {
  const binary = join(postgresBin, command);
  const root = process.getuid?.() === 0;
  return execFileSync(
    root ? "runuser" : binary,
    root ? ["-u", "postgres", "--", binary, ...args] : args,
    { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] },
  );
}

before(
  async function startRealSettlementProducer() {
    directory = await mkdtemp(join(tmpdir(), "settlement-proof-"));
    await chmod(directory, 0o755);
    const versions = await readdir("/usr/lib/postgresql");
    assert.ok(versions.length, "PostgreSQL server binaries must be installed");
    postgresBin = join("/usr/lib/postgresql", versions.sort().at(-1)!, "bin");
    const data = join(directory, "data");
    const socket = join(directory, "socket");
    await mkdir(data);
    await mkdir(socket);
    if (process.getuid?.() === 0) {
      const uid = Number(
        execFileSync("id", ["-u", "postgres"], { encoding: "utf8" }).trim(),
      );
      const gid = Number(
        execFileSync("id", ["-g", "postgres"], { encoding: "utf8" }).trim(),
      );
      await chown(data, uid, gid);
      await chown(socket, uid, gid);
    }
    postgres("initdb", [
      "-D",
      data,
      "-A",
      "trust",
      "-U",
      "postgres",
      "--no-sync",
    ]);
    postgres("pg_ctl", [
      "-D",
      data,
      "-l",
      join(data, "server.log"),
      "-o",
      `-F -k ${socket} -h ''`,
      "-w",
      "start",
    ]);
    postgresStarted = true;
    const root = resolve(process.cwd(), "../..");
    const main = join(
      root,
      "services/platform/perps-data/cmd/perps-replay/main.go",
    );
    const driver = join(
      root,
      "apps/perps/src/widgets/portfolio/testdata/settlement-producer.go",
    );
    const overlay = join(directory, "overlay.json");
    // Go's overlay keeps the test driver in scope while giving its imports the
    // real perps-data internal-package boundary. Production sources stay intact.
    await writeFile(overlay, JSON.stringify({ Replace: { [main]: driver } }));
    producer = spawn(
      "go",
      ["run", "-mod=readonly", "-overlay", overlay, main],
      {
        cwd: root,
        env: {
          ...process.env,
          SETTLEMENT_PROOF_DATABASE_URL: `postgresql:///postgres?host=${socket}&user=postgres&sslmode=disable`,
        },
        stdio: ["pipe", "pipe", "pipe"],
      },
    );
    let stderr = "";
    producer.stderr!.setEncoding("utf8").on("data", (chunk) => {
      stderr += chunk;
    });
    producerExit = new Promise((done, reject) => {
      producer!.once("error", reject);
      producer!.once("exit", done);
    });
    const lines = createInterface({ input: producer.stdout! });
    const url = await Promise.race([
      (async () => {
        for await (const line of lines)
          if (line.startsWith("http://")) return line;
        throw new Error(
          `Settlement producer closed before readiness: ${stderr}`,
        );
      })(),
      producerExit.then((code) => {
        throw new Error(`Settlement producer exited ${code}: ${stderr}`);
      }),
    ]);
    endpoint = { url };
    process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = url;
    const { gqlQuery } = await import("../src/core/graphql/client");
    const { TRADE_SETTLEMENTS_QUERY } = await import(
      "../src/hooks/perps/useSubgraph"
    );
    const result = await gqlQuery<{ tradeSettlements: TradeSettlement[] }>(
      endpoint,
      TRADE_SETTLEMENTS_QUERY,
      { userAddress: "0x00000000000000000000000000000000000000a1", limit: 50 },
    );
    settlements = result.tradeSettlements;
    assert.equal(
      settlements.length,
      3,
      "all three decoded events cross the real settlement query",
    );
    const { fetchSettlementCollateral } = await import(
      "../src/widgets/portfolio/settlementCollateral"
    );
    for (const settlement of settlements) {
      collateral.set(
        settlement.positionId,
        await fetchSettlementCollateral(settlement.positionId, endpoint),
      );
    }
  },
  { timeout: 600_000 },
);

after(async function stopRealSettlementProducer() {
  try {
    producer?.stdin?.end();
    if (producerExit)
      assert.equal(await producerExit, 0, "real producer exits cleanly");
  } finally {
    if (postgresStarted)
      postgres("pg_ctl", [
        "-D",
        join(directory, "data"),
        "-m",
        "immediate",
        "-w",
        "stop",
      ]);
    if (directory) await rm(directory, { recursive: true, force: true });
  }
});

async function renderedCells(
  settlement: TradeSettlement,
  metadata = collateral,
) {
  const { TradeSettlementRows } = await import(
    "../src/widgets/portfolio/TradeSettlementHistory"
  );
  const html = renderToStaticMarkup(
    createElement(
      "table",
      null,
      createElement(
        "tbody",
        null,
        createElement(TradeSettlementRows, {
          settlements: [settlement],
          labels: [],
          collateralByPosition: metadata,
        }),
      ),
    ),
  );
  return [...html.matchAll(/<td([^>]*)>(.*?)<\/td>/g)].map((match) => ({
    attributes: match[1],
    html: match[2],
  }));
}

async function f036_signedFundingCrossesSettlementQueryAndRows() {
  const { settlementChartData } = await import(
    "../src/widgets/portfolio/TradeSettlementHistory"
  );
  for (const [positionId, producerValue, cashFlow, text, color] of [
    ["1", "2000000000000000000", -2, "-$2.00", "text-red-400"],
    ["2", "-2000000000000000000", 2, "+$2.00", "text-emerald-500"],
    ["3", "0", 0, "$0.00", "text-muted-foreground"],
  ] as const) {
    const settlement = settlements.find(
      (row) => row.positionId === positionId,
    )!;
    assert.equal(
      settlement.fundingPaidUsd,
      producerValue,
      "decoder sign survives GraphQL",
    );
    const chart = settlementChartData([settlement], []);
    assert.equal(
      chart[0].funding,
      cashFlow,
      "chart negates the signed payment exactly once",
    );
    const cells = await renderedCells(settlement);
    assert.equal(cells[6].html, text, "row funding is the signed cash flow");
    assert.ok(
      cells[6].attributes.includes(color),
      "row color follows cash flow",
    );
  }
}

async function f037_nativePayoutResolvesPositionCollateralThroughGraphQL() {
  for (const [positionId, raw, displayed] of [
    ["1", "119000000", "119 USDL"],
    ["2", "119000001", "119.000001 USDL"],
    ["3", "0", "0 USDL"],
  ]) {
    const settlement = settlements.find(
      (row) => row.positionId === positionId,
    )!;
    assert.equal(
      settlement.netPayoutTokens,
      raw,
      "native units survive decoder, store and GraphQL",
    );
    const token = collateral.get(positionId)!;
    assert.equal(token.decimals, 6);
    assert.equal(
      token.tokenAddress.toLowerCase(),
      "0x85fcd13735f4309833a503ee804ea32395851479",
    );
    const cells = await renderedCells(settlement);
    assert.equal(cells[7].html.replace(/<[^>]+>/g, ""), displayed);
    assert.ok(
      cells[7].html.toLowerCase().includes(token.tokenAddress.toLowerCase()),
      "row retains full token identity",
    );
    assert.doesNotMatch(cells[7].html, /\$/);
  }
  const { fetchSettlementCollateral } = await import(
    "../src/widgets/portfolio/settlementCollateral"
  );
  try {
    await fetchSettlementCollateral("999", endpoint);
    assert.fail("unknown position must refuse collateral resolution");
  } catch (error) {
    assert.ok(error instanceof Error);
    assert.match(error.message, /Settlement collateral unavailable/);
    const { TradeSettlementRows } = await import(
      "../src/widgets/portfolio/TradeSettlementHistory"
    );
    const unavailable = renderToStaticMarkup(
      createElement(TradeSettlementRows, {
        settlements: [settlements[0]],
        labels: [],
        collateralByPosition: new Map([[settlements[0].positionId, error]]),
      }),
    );
    assert.match(unavailable, /Collateral unavailable/);
    assert.doesNotMatch(unavailable, /119 USDL/);
  }
  const pending = await renderedCells(settlements[0], new Map());
  assert.equal(
    pending[7].html,
    "Loading collateral…",
    "missing metadata never implies 18 decimals or USD",
  );
}

test(f036_signedFundingCrossesSettlementQueryAndRows);
test(f037_nativePayoutResolvesPositionCollateralThroughGraphQL);
