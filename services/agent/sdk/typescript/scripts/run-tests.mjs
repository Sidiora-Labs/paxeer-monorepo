import { spawnSync } from "node:child_process";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { fileURLToPath } from "node:url";

const packageRoot = new URL("../", import.meta.url);
const sourceRoot = new URL("test/", packageRoot);
const manifest = JSON.parse(readFileSync(new URL("package.json", packageRoot), "utf8"));
const [mode, ...args] = process.argv.slice(2);

function discoverTests(directory, prefix = "") {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const name = `${prefix}${entry.name}`;
    if (entry.isDirectory()) return discoverTests(new URL(`${entry.name}/`, directory), `${name}/`);
    return entry.isFile() && name.endsWith(".test.ts") ? [name] : [];
  }).sort();
}

const discovered = discoverTests(sourceRoot);
if (discovered.length === 0) throw new Error("No TypeScript test sources discovered");
const requirements = manifest.sdkTestRequirements;
const special = Object.values(requirements).flat();
if (new Set(special).size !== special.length) throw new Error("Duplicate test requirement");
for (const name of special) {
  if (!discovered.includes(name)) throw new Error(`Test requirement names an absent source: ${name}`);
}

let selected;
let testArgs = [];
switch (mode) {
  case "self-contained":
    selected = discovered.filter((name) => !special.includes(name));
    break;
  case "anvil":
  case "case-file":
    selected = requirements[mode];
    if (mode === "case-file" && !process.env.PAXEER_X_AGENT_ENVELOPE_CASE) {
      throw new Error("PAXEER_X_AGENT_ENVELOPE_CASE must name the private harness input");
    }
    break;
  case "ci":
    selected = discovered.filter((name) => !requirements["case-file"].includes(name));
    break;
  case "external-node":
    if (args.length !== 1) throw new Error("external-node requires one node-produced programs proof JSON path");
    selected = ["ai-market.test.ts"];
    testArgs = ["--programs-proof", args[0]];
    break;
  case "native-effect-case":
    if (!process.env.NATIVE_EFFECT_CROSS_LANGUAGE_REQUEST) {
      throw new Error("NATIVE_EFFECT_CROSS_LANGUAGE_REQUEST must name the producer request JSON");
    }
    selected = ["native-effect.test.ts"];
    break;
  default:
    throw new Error(`Unknown test mode: ${mode}`);
}
if (mode !== "external-node" && args.length !== 0) throw new Error("Unexpected test arguments");
if (selected.length === 0) throw new Error(`No tests selected for ${mode}`);

// Resolve from maintained sources, so stale dist files cannot enter the inventory.
const compiled = selected.map((name) => {
  if (!discovered.includes(name)) throw new Error(`Missing test source: ${name}`);
  const target = new URL(`dist/test/${name.replace(/\.ts$/u, ".js")}`, packageRoot);
  if (!statSync(target).isFile()) throw new Error(`Missing compiled test: ${name}`);
  return target;
});

let failures = 0;
for (const target of compiled) {
  console.log(`Running ${fileURLToPath(target)}`);
  const result = spawnSync(process.execPath, [fileURLToPath(target), ...testArgs], {
    cwd: fileURLToPath(packageRoot),
    stdio: "inherit",
  });
  if (result.error || result.signal || result.status !== 0) {
    failures += 1;
    console.error(`Failed ${fileURLToPath(target)}: ${result.error?.message ?? result.signal ?? result.status}`);
  }
}
console.log(`${compiled.length} test files executed; ${failures} failed`);
process.exitCode = failures === 0 ? 0 : 1;
