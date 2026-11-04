import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { readFile, open, stat, readdir } from "node:fs/promises";
import { dirname, isAbsolute, resolve } from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { pathToFileURL } from "node:url";

const root = resolve(import.meta.dirname, "../../..");

const commandLimits = Object.freeze({
  timeoutMs: 15 * 60 * 1000,
  maxOutputBytes: 16 * 1024 * 1024,
  diagnosticBytes: 16 * 1024,
  terminationGraceMs: 1000,
});

function diagnosticTail(maximum) {
  const buffer = Buffer.alloc(maximum);
  let size = 0;
  return {
    append(chunk) {
      const kept = Math.min(chunk.length, maximum);
      const previous = Math.min(size, maximum - kept);
      buffer.copyWithin(0, size - previous, size);
      chunk.copy(buffer, previous, chunk.length - kept);
      size = previous + kept;
    },
    text() { return buffer.subarray(0, size).toString("utf8"); },
  };
}

export async function command(executable, arguments_, cwd = root, limits = {}) {
  const selected = { ...commandLimits, ...limits };
  for (const [key, maximum] of Object.entries(commandLimits)) {
    if (!Number.isSafeInteger(selected[key]) || selected[key] <= 0 || selected[key] > maximum) {
      throw new Error("invalid_reference_build_command_limits");
    }
  }
  const child = spawn(executable, arguments_, {
    cwd, env: { ...process.env, CARGO_BUILD_JOBS: "4" },
    detached: true, stdio: ["ignore", "pipe", "pipe"],
  });
  let output = Buffer.alloc(selected.maxOutputBytes);
  let outputSize = 0;
  let totalSize = 0;
  const stdout = diagnosticTail(selected.diagnosticBytes);
  const stderr = diagnosticTail(selected.diagnosticBytes);
  let failure;
  let cleanup;
  let exitCode;
  let exitSignal;
  function signalGroup(signal) {
    if (child.pid === undefined) return false;
    try {
      process.kill(-child.pid, signal);
      return true;
    } catch (error) {
      if (error.code === "ESRCH") return false;
      throw error;
    }
  }
  function stop(code, cause) {
    if (failure !== undefined) return;
    failure = { code, cause };
    output = undefined;
    // Keep draining both pipes while the entire group is terminated. Even if
    // the leader exits on TERM, its descendants still receive the later KILL.
    cleanup = (async () => {
      if (signalGroup("SIGTERM")) {
        await delay(selected.terminationGraceMs);
        signalGroup("SIGKILL");
      }
    })().catch((error) => error);
  }
  function collect(chunk, tail, isStdout) {
    tail.append(chunk);
    if (failure !== undefined) return;
    totalSize += chunk.length;
    if (totalSize > selected.maxOutputBytes) {
      stop("reference_build_output_bound");
      return;
    }
    if (isStdout) {
      chunk.copy(output, outputSize);
      outputSize += chunk.length;
    }
  }
  child.stdout.on("data", (chunk) => collect(chunk, stdout, true));
  child.stderr.on("data", (chunk) => collect(chunk, stderr, false));
  child.stdout.on("error", (error) => stop("reference_build_stdout_failed", error));
  child.stderr.on("error", (error) => stop("reference_build_stderr_failed", error));
  child.once("error", (error) => stop("reference_build_spawn_failed", error));
  child.once("exit", (code, signal) => {
    exitCode = code;
    exitSignal = signal;
    if (code !== 0) stop(`reference_build_failed_${executable}_${signal ?? code}`);
  });
  const deadline = setTimeout(() => stop("reference_build_deadline"), selected.timeoutMs);
  try {
    await new Promise((accept) => child.once("close", accept));
    const cleanupError = await cleanup;
    if (cleanupError !== undefined) throw new Error("reference_build_cleanup_failed", { cause: cleanupError });
    if (failure !== undefined) {
      const error = new Error(failure.code, { cause: failure.cause });
      Object.assign(error, {
        code: failure.code, exitCode, signal: exitSignal,
        stdout: stdout.text(), stderr: stderr.text(),
      });
      throw error;
    }
    return output.subarray(0, outputSize).toString("utf8");
  } finally {
    clearTimeout(deadline);
  }
}

async function buildReferenceApps() {
  const output = process.env.PAXEER_X_REFERENCE_APP_ARTIFACTS;
  if (typeof output !== "string" || !isAbsolute(output) || resolve(output) !== output || output.startsWith(`${root}/`)) throw new Error("private_reference_artifacts_path_required");
  const parent = await stat(dirname(output));
  if (!parent.isDirectory() || parent.uid !== process.getuid() || (parent.mode & 0o077) !== 0) throw new Error("private_reference_artifacts_directory_required");
  const revision = (await command("git", ["rev-parse", "HEAD"])).trim();
  if ((await command("git", ["status", "--porcelain"])).trim() !== "") throw new Error("reference_build_requires_clean_candidate");

  await command("npm", ["run", "build", "--workspace", "@sidiora/layerx-sdk", "--workspace", "@sidiora/layerx-seller-middleware", "--workspace", "@sidiora/layerx-buyer-middleware", "--workspace", "@sidiora/layerx-merchant-middleware"]);
  const compiled = await command(process.env.CARGO ?? "cargo", ["build", "--locked", "--manifest-path", "tools/platform/Cargo.toml", "-p", "layerx-platform-cli", "--bin", "layerx", "--message-format=json"]);
  const executables = compiled.split("\n").filter((line) => line.startsWith("{")).map((line) => JSON.parse(line))
    .filter((item) => item.reason === "compiler-artifact" && item.target?.name === "layerx" && typeof item.executable === "string");
  if (executables.length !== 1) throw new Error("compiler_omitted_unique_reference_cli");
  const cli = resolve(executables[0].executable);
  const directory = resolve(root, "services/platform/examples/marketplace");
  const built = JSON.parse((await command(cli, ["--json", "program", "build", "--manifest-path", resolve(directory, "program/Cargo.toml")], directory)).trim());
  if (built.ok !== true || typeof built.data?.artifact !== "string" || !Number.isSafeInteger(built.data?.abi_version)) throw new Error("actual_cli_omitted_reference_guest");
  const guest = resolve(directory, built.data.artifact);
  const hash = async (path) => createHash("sha256").update(await readFile(path)).digest("hex");
  const guestHash = await hash(guest);
  if (guestHash !== built.data.code_hash) throw new Error("actual_guest_code_hash_mismatch");
  if ((await command("git", ["rev-parse", "HEAD"])).trim() !== revision
    || (await command("git", ["status", "--porcelain"])).trim() !== "") throw new Error("reference_source_changed_during_build");
  async function compiledFiles(directory) {
    const entries = await readdir(directory, { withFileTypes: true });
    const files = [];
    for (const entry of entries.sort((a, b) => a.name.localeCompare(b.name))) {
      const path = resolve(directory, entry.name);
      if (entry.isDirectory()) files.push(...await compiledFiles(path));
      else if (entry.isFile()) files.push({ path, sha256: await hash(path) });
      else throw new Error("reference_node_output_not_regular");
    }
    return files;
  }
  const node = [];
  for (const path of ["services/agent/sdk/typescript/dist", "services/platform/middleware/seller/dist", "services/platform/middleware/buyer/dist", "services/platform/middleware/merchant/dist"]) {
    const files = await compiledFiles(resolve(root, path));
    if (files.length === 0) throw new Error("missing_compiled_reference_node_workspace");
    node.push({ directory: path, files });
  }
  const manifest = { schema: "layerx.reference-app-artifacts.v1", source_revision: revision, node,
    cli: { path: cli, sha256: await hash(cli) },
    marketplace: { path: guest, sha256: guestHash, code_hash: guestHash, abi_version: built.data.abi_version } };
  const file = await open(output, "wx", 0o600);
  try { await file.writeFile(`${JSON.stringify(manifest, null, 2)}\n`); await file.sync(); } finally { await file.close(); }
  process.stdout.write(`${JSON.stringify({ source_revision: revision, artifacts: ["cli", "marketplace"], build: "complete" })}\n`);
}

if (process.argv[1] !== undefined && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  await buildReferenceApps();
}
