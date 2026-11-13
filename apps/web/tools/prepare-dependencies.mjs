import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("../", import.meta.url));
const walletRoot = fileURLToPath(new URL("../../../sdk/wallet/", import.meta.url));
const agentSdkRoot = fileURLToPath(new URL("../../../services/agent/sdk/typescript/", import.meta.url));

function run(command, args) {
  const result = spawnSync(command, args, { cwd: webRoot, stdio: "inherit" });
  if (result.error) throw result.error;
  if (result.signal) throw new Error(`${command} terminated by ${result.signal}`);
  if (result.status === null) throw new Error(`${command} returned no exit status`);
  if (result.status !== 0) process.exit(result.status);
}

run("pnpm", ["--dir", walletRoot, "install", "--ignore-workspace", "--frozen-lockfile", "--ignore-scripts"]);
run("npm", ["--prefix", agentSdkRoot, "run", "build", "--workspaces=false"]);
run("pnpm", ["--dir", walletRoot, "exec", "tsc", "-p", "tsconfig.json"]);
run("npm", ["run", "build:ui"]);
