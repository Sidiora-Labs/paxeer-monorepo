import { spawn, execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import { environment } from "./environment.mjs";

function run(command, args) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: "inherit", env: environment });
    child.on("error", reject);
    child.on("exit", (code, signal) =>
      code === 0
        ? resolve()
        : reject(new Error(`${command} exited ${code ?? signal}`)),
    );
  });
}

await run(process.execPath, [
  "--test",
  "tools/ui-check/surfaces/trading/routes.test.mjs",
]);
const files = execFileSync(
  "git",
  [
    "ls-files",
    "-z",
    "--cached",
    "--others",
    "--exclude-standard",
    "apps/perps",
    "tools/ui-check/surfaces/trading/environment.mjs",
  ],
  { encoding: "utf8" },
)
  .split("\0")
  .filter(Boolean)
  .sort();
const inputs = createHash("sha256");
for (let offset = 0; offset < files.length; offset += 32) {
  const batch = files.slice(offset, offset + 32);
  const contents = await Promise.all(
    batch.map(async (file) => {
      try {
        return await readFile(file);
      } catch (error) {
        if (error.code !== "ENOENT") throw error;
        return "deleted";
      }
    }),
  );
  for (const [index, file] of batch.entries()) {
    inputs.update(file).update("\0").update(contents[index]).update("\0");
  }
}
inputs.update(process.version);
inputs.update(
  JSON.stringify(
    Object.entries(environment)
      .filter(([name]) => /^(NEXT_|NODE_ENV$|PAXEER_NETWORK_)/.test(name))
      .sort(),
  ),
);
const digest = inputs.digest("hex");
const output = "apps/perps/apps/web/.next-production";
const stamp = `${output}/ui-check-build.json`;
let previous;
try {
  previous = JSON.parse(await readFile(stamp, "utf8"));
} catch (error) {
  if (error.code !== "ENOENT") throw error;
}
let buildId;
try {
  buildId = await readFile(`${output}/BUILD_ID`, "utf8");
} catch (error) {
  if (error.code !== "ENOENT") throw error;
}
if (previous?.digest === digest && previous.buildId === buildId && buildId) {
  await readFile(`${output}/server/pages-manifest.json`);
  await readFile(`${output}/build-manifest.json`);
  console.log("Reusing the production build for unchanged inputs.");
} else {
  await run("pnpm", ["--dir", "apps/perps", "build"]);
  buildId = await readFile(`${output}/BUILD_ID`, "utf8");
  await writeFile(stamp, JSON.stringify({ digest, buildId }));
}
