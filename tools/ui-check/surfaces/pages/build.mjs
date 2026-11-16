import { execFileSync } from "node:child_process";
import { cp, mkdir, readFile, stat, writeFile } from "node:fs/promises";
import { DatabaseSync } from "node:sqlite";
import { tmpdir } from "node:os";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import {
  generateSnapshot,
  parseStatus,
  parseTrackedFiles,
  readActiveSpec,
} from "../../../codebase-map/generate.mjs";

execFileSync("cargo", ["component", "--version"], { stdio: "inherit" });

const directory = dirname(fileURLToPath(import.meta.url));
const root = resolve(directory, "../../../..");
const site = resolve(directory, "dist");
await cp(resolve(directory, "project.html"), resolve(site, "project.html"));
const source = resolve(root, "tools/wasm-tools");
const workspace = resolve(tmpdir(), "paxeer-pages-wasm-tools");
await mkdir(workspace, { recursive: true });
await cp(source, workspace, {
  recursive: true,
  preserveTimestamps: true,
  filter: (path) =>
    !["node_modules", "target", "dist", "component-built"].some((part) =>
      path.split("/").includes(part),
    ),
});
const playground = resolve(workspace, "playground");
const map = resolve(root, "tools/codebase-map");
const fixtures = resolve(map, "test/fixtures");
const json = async (path) => JSON.parse(await readFile(path, "utf8"));

execFileSync("npm", ["ci"], { cwd: playground, stdio: "inherit" });
execFileSync("npm", ["run", "build"], {
  cwd: playground,
  stdio: "inherit",
  env: { ...process.env, CARGO_PROFILE_RELEASE_OPT_LEVEL: "0" },
});
const component = resolve(
  workspace,
  "target/wasm32-wasip1/release/component.wasm",
);
execFileSync(
  "npm",
  [
    "run",
    "transpile",
    "--",
    "--base64-cutoff",
    String((await stat(component)).size + 1),
  ],
  { cwd: playground, stdio: "inherit" },
);
for (const step of ["bundle", "copy-files"]) {
  execFileSync("npm", ["run", step], { cwd: playground, stdio: "inherit" });
}
await cp(resolve(playground, "dist"), resolve(site, "playground"), {
  recursive: true,
});
for (const route of ["parse", "print"]) {
  await mkdir(resolve(site, "playground", route), { recursive: true });
  await cp(
    resolve(site, "playground", `${route}.html`),
    resolve(site, "playground", route, "index.html"),
  );
}
await mkdir(resolve(site, "peer"), { recursive: true });
for (const path of ["index.html", "src", "engine"]) {
  await cp(
    resolve(root, "apps/inference-peer", path),
    resolve(site, "peer", path),
    { recursive: true },
  );
}

const database = new DatabaseSync(":memory:");
let snapshot;
let emptySnapshot;
try {
  database.exec(
    await readFile(resolve(fixtures, "delivery-snapshot/graph.sql"), "utf8"),
  );
  const graph = await json(resolve(fixtures, "snapshot-setup/graph.json"));
  const git = await json(resolve(fixtures, "snapshot-setup/git.json"));
  for (const table of ["files", "symbols", "refs", "imports"]) {
    for (const row of graph[table]) {
      const columns = Object.keys(row);
      database
        .prepare(
          `INSERT INTO ${table} (${columns.join(",")}) VALUES (${columns.map(() => "?").join(",")})`,
        )
        .run(...columns.map((column) => row[column]));
    }
  }
  snapshot = generateSnapshot({
    root,
    database,
    tracked: parseTrackedFiles(Buffer.from(git.tracked)),
    statusByPath: parseStatus(Buffer.from(git.status)),
    commit: git.commit,
    branch: git.branch,
    graphUpdatedAt: git.graphUpdatedAt,
    platform: readActiveSpec(resolve(fixtures, "delivery-snapshot/repository")),
  });
  for (const table of ["refs", "imports", "symbols", "files"])
    database.exec(`DELETE FROM ${table}`);
  emptySnapshot = generateSnapshot({
    root,
    database,
    tracked: [],
    statusByPath: new Map(),
    commit: git.commit,
    branch: git.branch,
    graphUpdatedAt: git.graphUpdatedAt,
    platform: {
      ...readActiveSpec(resolve(fixtures, "delivery-snapshot/repository")),
      tasks: [],
      requirements: [],
    },
  });
} finally {
  database.close();
}
for (const variant of ["map", "map-empty", "map-error"]) {
  const destination = resolve(site, variant);
  await mkdir(resolve(destination, "data"), { recursive: true });
  for (const file of ["index.html", "styles.css", "app.js", "delivery.mjs"]) {
    await cp(resolve(map, file), resolve(destination, file));
  }
  const data =
    variant === "map-error"
      ? {}
      : variant === "map-empty"
        ? emptySnapshot
        : snapshot;
  await writeFile(
    resolve(destination, "data/codebase-map.json"),
    `${JSON.stringify(data)}\n`,
  );
}

const signal = await json(
  resolve(root, "apps/inference-peer/test/fixtures/host-signal-frames.json"),
);
const frames = signal.vectors
  .filter((entry) =>
    ["offer", "deal", "deal_live", "error"].includes(entry.name),
  )
  .map((entry) => JSON.parse(entry.json));
await writeFile(resolve(site, "peer/room.json"), `${JSON.stringify(frames)}\n`);
await cp(resolve(directory, "states.mjs"), resolve(site, "states.mjs"));
for (const page of [
  "peer/index.html",
  "playground/print.html",
  "playground/print/index.html",
]) {
  const filename = resolve(site, page);
  const html = await readFile(filename, "utf8");
  await writeFile(
    filename,
    html.replace(
      "</head>",
      '<script type="module" blocking="render" src="/states.mjs"></script>\n</head>',
    ),
  );
}
