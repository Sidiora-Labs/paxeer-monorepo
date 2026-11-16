import { test } from "node:test";
import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("..", import.meta.url));

const DECLARED = [
  "NEXT_PUBLIC_PERPS_GATEWAY_URL",
  "NEXT_PUBLIC_PERPS_PRECOMPILE",
  "NEXT_PUBLIC_LAUNCHPAD_PRECOMPILE",
  "NEXT_PUBLIC_PECOR_MARKET_PRECOMPILE",
  "NEXT_PUBLIC_PECOR_ORDERS_PRECOMPILE",
  "NEXT_PUBLIC_PECOR_STOP_PRECOMPILE",
  "NEXT_PUBLIC_PECOR_QUOTER_PRECOMPILE",
  "NEXT_PUBLIC_PECOR_VAULT_PRECOMPILE",
] as const;

const REMOVED = [
  "DIAMOND_ADDRESS",
  "USID_ADDRESS",
  "USDC_ADDRESS",
  "USDT_ADDRESS",
  "NEXT_PUBLIC_INDEXER_URL",
  "NEXT_PUBLIC_INDEXER_WS_URL",
  "NEXT_PUBLIC_PERFDB_URL",
  "NEXT_PUBLIC_SPOT_",
] as const;

function read(rel: string): string {
  return readFileSync(path.join(ROOT, rel), "utf8");
}

function lines(rel: string): string[] {
  return read(rel).split("\n");
}

function filesUnder(rel: string): string[] {
  return readdirSync(path.join(ROOT, rel))
    .filter(
      (name) =>
        ![
          "node_modules",
          "dist",
          ".next-production",
          ".next-dev",
          "__test__",
          "__tests__",
        ].includes(name),
    )
    .flatMap((name) => {
      const child = path.join(rel, name);
      return statSync(path.join(ROOT, child)).isDirectory()
        ? filesUnder(child)
        : [child];
    });
}

function dockerfileEnvBlock(): string[] {
  const all = lines("Dockerfile");
  const start = all.findIndex((line) => line.startsWith("ENV NEXT_PUBLIC_"));
  assert.notEqual(start, -1, "Dockerfile has no NEXT_PUBLIC_ ENV list");
  const end = all.findIndex(
    (line, i) => i >= start && !line.trimEnd().endsWith("\\"),
  );
  return all.slice(start, end + 1).map((line) =>
    line
      .replace(/^ENV\s+/, "")
      .replace(/\\$/, "")
      .trim(),
  );
}

function deployFilesDeclareGateway() {
  const declarations: Record<string, (name: string) => boolean> = {
    "apps/web/.env.example": (name) =>
      lines("apps/web/.env.example").some((l) => l.startsWith(`${name}=`)),
    "deploy/.env.production.example": (name) =>
      lines("deploy/.env.production.example").some((l) =>
        l.startsWith(`${name}=`),
      ),
    "deploy/docker-compose.yml": (name) =>
      lines("deploy/docker-compose.yml").some(
        (l) =>
          l.trim() ===
          `${name}: \${${name}:${name === "NEXT_PUBLIC_PERPS_GATEWAY_URL" ? "?Set the public gateway origin" : "-"}}`,
      ),
    "Dockerfile ARG": (name) =>
      lines("Dockerfile").some((l) => l === `ARG ${name}=`),
    "Dockerfile ENV": (name) =>
      dockerfileEnvBlock().includes(`${name}=$${name}`),
    "deploy/README.md": (name) =>
      read("deploy/README.md").includes(`\`${name}\``),
  };
  for (const [file, declares] of Object.entries(declarations)) {
    for (const name of DECLARED) {
      assert.equal(declares(name), true, `${file} does not declare ${name}`);
    }
  }

  const railway = JSON.parse(read("railway.json")) as Record<
    string,
    Record<string, unknown>
  >;
  assert.deepEqual(Object.keys(railway).sort(), ["$schema", "build", "deploy"]);
  assert.deepEqual(railway.build, {
    builder: "DOCKERFILE",
    dockerfilePath: "Dockerfile",
  });
  assert.equal(
    existsSync(path.join(ROOT, "nixpacks.toml")),
    false,
    "nixpacks.toml still exists",
  );
  assert.equal(
    existsSync(path.join(ROOT, "deploy/Dockerfile")),
    false,
    "deploy/Dockerfile still exists",
  );
  assert.equal(existsSync(path.join(ROOT, "ecosystem.config.js")), false);
}
test(deployFilesDeclareGateway);

function productionGatewayIsHttpsOrigin() {
  const gateway = lines("deploy/build-args.production.env")
    .find((line) => line.startsWith("NEXT_PUBLIC_PERPS_GATEWAY_URL="))
    ?.slice("NEXT_PUBLIC_PERPS_GATEWAY_URL=".length);
  assert.equal(gateway, "https://perps-api.paxeer.app");
  const origin = new URL(gateway);
  assert.equal(origin.protocol, "https:");
  assert.equal(origin.username, "");
  assert.equal(origin.password, "");
  assert.equal(origin.pathname, "/");
  assert.equal(origin.search, "");
  assert.equal(origin.hash, "");
  assert.equal(gateway, origin.origin);
  assert.ok(
    lines("deploy/.env.production.example").includes(
      "NEXT_PUBLIC_PERPS_GATEWAY_URL=",
    ),
  );
}
test(productionGatewayIsHttpsOrigin);

function removedVariablesAbsent() {
  const scanned = [
    "apps/web/.env.example",
    ...filesUnder("deploy"),
    "railway.json",
    "Dockerfile",
  ];
  assert.ok(scanned.includes("apps/web/.env.example"));
  assert.ok(scanned.includes(path.join("deploy", "docker-compose.yml")));
  for (const file of scanned) {
    const text = read(file);
    for (const name of REMOVED) {
      assert.equal(text.includes(name), false, `${file} still names ${name}`);
    }
  }
  assert.equal(
    lines("apps/web/.env.example").some((line) =>
      line.startsWith("USDL_ADDRESS="),
    ),
    false,
  );
}
test(removedVariablesAbsent);

function deployedJsonRemoved() {
  assert.equal(
    existsSync(
      path.join(ROOT, "packages/paxeer-backend/src/env/deployed.json"),
    ),
    false,
  );
}
test(deployedJsonRemoved);

function imageDockerfileMatchesOffchainImages() {
  const dockerfile = read("Dockerfile");
  const all = lines("Dockerfile");
  assert.match(dockerfile, /^ARG NODE_IMAGE=node@sha256:[0-9a-f]{64}$/m);
  assert.match(dockerfile, /NODE_IMAGE must be pinned by digest/);
  assert.ok(
    all.includes(
      "ARG CHARTING_IMAGE=ghcr.io/sidiora-labs/hyperpax-charting-library@sha256:4a805ca00668766af6085043670140a8f215ccbd29c169e1bc4978d2984bad74",
    ),
  );
  assert.match(dockerfile, /CHARTING_IMAGE must be pinned by digest/);
  assert.ok(all.includes("FROM ${CHARTING_IMAGE} AS charting"));
  assert.ok(
    all.includes(
      "COPY --from=charting /charting_library apps/web/public/static/charting_library",
    ),
  );
  assert.ok(
    all.includes(
      "COPY --from=charting /datafeeds apps/web/public/static/datafeeds",
    ),
  );
  assert.match(
    dockerfile,
    /test -f apps\/web\/public\/static\/charting_library\/charting_library\.standalone\.js \|\| \{ echo .* exit 1; \}/,
  );
  assert.ok(all.includes("RUN pnpm install --frozen-lockfile"));
  assert.match(dockerfile, /PORT=3000/);
  assert.match(dockerfile, /\/api\/build-id/);

  const start = all.findIndex((line) => line.startsWith("ENV NEXT_PUBLIC_"));
  assert.notEqual(start, -1, "Dockerfile has no NEXT_PUBLIC_ ENV list");
  const end = all.findIndex(
    (line, i) => i >= start && !line.trimEnd().endsWith("\\"),
  );
  const env = all.slice(start, end + 1).map((line) =>
    line
      .replace(/^ENV\s+/, "")
      .replace(/\\$/, "")
      .trim(),
  );
  const read_ = new Set(
    [...filesUnder("apps/web/src"), ...filesUnder("packages")]
      .filter(
        (file) =>
          !file
            .split(path.sep)
            .some((part) =>
              ["node_modules", "dist", "__tests__", "__test__"].includes(part),
            ),
      )
      .filter((file) => /\.(ts|tsx|js|jsx)$/.test(file))
      .flatMap(
        (file) =>
          read(file).match(/process\.env\.NEXT_PUBLIC_[A-Z0-9_]+/g) ?? [],
      )
      .map((ref) => ref.slice("process.env.".length)),
  );
  assert.ok(read_.size > 0);
  for (const name of read_) {
    assert.ok(all.includes(`ARG ${name}=`), `Dockerfile has no ARG ${name}=`);
    assert.ok(
      env.includes(`${name}=$${name}`),
      `Dockerfile has no ENV ${name}`,
    );
  }

  const args = all
    .filter((line) => line.startsWith("ARG NEXT_PUBLIC_"))
    .map((line) => line.slice(4, -1));
  assert.deepEqual(args, [...read_].sort());
  assert.deepEqual(
    env,
    args.map((name) => `${name}=$${name}`),
  );
  const production = lines("deploy/build-args.production.env")
    .filter((line) => line.startsWith("NEXT_PUBLIC_"))
    .map((line) => line.split("=")[0]);
  assert.deepEqual(production, args);
  const examples = lines("deploy/.env.production.example")
    .filter((line) => line.startsWith("NEXT_PUBLIC_"))
    .map((line) => line.split("=")[0]);
  assert.deepEqual(examples.sort(), args);
  for (const name of [
    "MARKET_PROXY_ORIGIN",
    "CANDLE_TRUSTED_PROXY_IPS",
    "PRESENCE_TRUSTED_PROXY_IPS",
    "PRICE_FEED_ORIGIN",
    "PRICE_FEED_REST_API_KEY",
    "PRICE_FEED_WS_API_KEY",
    "RAILWAY_GIT_COMMIT_SHA",
    "VERCEL_GIT_COMMIT_SHA",
  ]) {
    assert.equal(
      new RegExp(`^ARG ${name}(?:=|$)`, "m").test(dockerfile),
      false,
    );
    assert.equal(
      new RegExp(`(?:^ENV |^\\s+)${name}=`, "m").test(dockerfile),
      false,
    );
  }
  assert.ok(all.includes("COPY pnpm-workspace.yaml .npmrc ./"));
  assert.ok(all.includes("COPY apps ./apps"));
  assert.ok(all.includes("COPY packages ./packages"));
  assert.ok(all.includes("COPY --from=deps /app ./"));
  assert.match(dockerfile, /PAXEER_NETWORK_BUILD_DIR=\.next-production/);
  assert.match(
    dockerfile,
    /COPY --from=builder --chown=nextjs:nodejs \/app\/apps\/web\/\.next-production\/standalone \.\//,
  );
  assert.match(
    dockerfile,
    /COPY --from=builder --chown=nextjs:nodejs \/app\/apps\/web\/\.next-production\/static \.\/apps\/web\/\.next-production\/static/,
  );
  assert.match(
    dockerfile,
    /COPY --from=builder --chown=nextjs:nodejs \/app\/apps\/web\/public \.\/apps\/web\/public/,
  );
  assert.match(dockerfile, /^USER nextjs$/m);
  assert.match(dockerfile, /^EXPOSE 3000$/m);
  assert.ok(all.includes('CMD ["node", "apps/web/server.js"]'));
  assert.equal(
    /rpcpaxeer\.online/.test(dockerfile),
    false,
    "Dockerfile names rpcpaxeer.online",
  );
}
test(imageDockerfileMatchesOffchainImages);

function railwayHealthcheck() {
  const railway = JSON.parse(read("railway.json")) as {
    deploy: Record<string, unknown>;
  };
  assert.equal(railway.deploy.healthcheckPath, "/api/build-id");
  assert.equal(railway.deploy.healthcheckTimeout, 300);
  assert.equal(railway.deploy.restartPolicyType, "ON_FAILURE");
  assert.equal(read("railway.json").includes("rpcpaxeer.online"), false);
  assert.ok(existsSync(path.join(ROOT, "apps/web/src/pages/api/build-id.ts")));
  assert.match(
    read("apps/web/src/pages/api/build-id.ts"),
    /process\.env\.RAILWAY_GIT_COMMIT_SHA\s*\|\|\s*process\.env\.VERCEL_GIT_COMMIT_SHA\s*\|\|\s*["']development["']/,
  );
}
test(railwayHealthcheck);

function webPlatformBuildsWorkspace() {
  const vercel = JSON.parse(read("vercel.json"));
  assert.equal(vercel.builds.length, 1);
  assert.equal(vercel.builds[0].src, "apps/web/package.json");
  assert.equal(vercel.builds[0].use, "@vercel/next");
  assert.deepEqual(vercel.builds[0].config, {
    outputDirectory: ".next-production",
    installCommand: "pnpm --dir ../.. install --frozen-lockfile",
    buildCommand: "pnpm --dir ../.. build",
  });
  const config = read("apps/web/next.config.js");
  assert.match(config, /output: ["']standalone["']/);
  assert.match(
    config,
    /outputFileTracingRoot: path.join\(__dirname, ["']\.\.\/\.\.["']\)/,
  );
  assert.match(config, /PAXEER_NETWORK_BUILD_DIR/);
  assert.match(config, /["']\.next-production["']/);
  for (const route of [
    "build-id.ts",
    "candle/[...path].ts",
    "presence.ts",
    "price-feed/[...path].ts",
  ]) {
    assert.ok(existsSync(path.join(ROOT, "apps/web/src/pages/api", route)));
  }
  const candle = read("apps/web/src/pages/api/candle/[...path].ts");
  assert.match(
    candle,
    /import \{ gatewayUrls \} from ["']@paxeer-network\/paxeer-backend\/core\/network\/gateway["']/,
  );
  assert.ok(
    candle.includes(
      "`${gatewayUrls().origin}/api/${normalized.toLowerCase()}`",
    ),
  );
  assert.equal(candle.includes("data.feed."), false);
}
test(webPlatformBuildsWorkspace);
