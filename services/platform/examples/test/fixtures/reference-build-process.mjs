import { spawn } from "node:child_process";
import { once } from "node:events";
import { fileURLToPath } from "node:url";

const [endpoint, mode, ...arguments_] = process.argv.slice(2);
const keepAlive = setInterval(() => {}, 1000);
async function exchange(event, data = {}) {
  const response = await fetch(endpoint, {
    method: "POST", headers: { "content-type": "application/json" },
    body: JSON.stringify({ event, mode, pid: process.pid, ...data }),
  });
  if (response.status !== 200) throw new Error("fixture_peer_refused");
  return response.json();
}

if (mode === "descendant") {
  process.on("SIGTERM", () => { void exchange("term"); });
  await exchange("ready", { parent: process.ppid });
  process.send({ ready: true });
  process.disconnect();
} else {
  process.on("SIGTERM", () => {
    if (mode === "leader-exits") process.exit(0);
    void exchange("term");
  });
  let descendant;
  if (["tree", "leader-exits", "tree-fails"].includes(mode)) {
    descendant = spawn(process.execPath, [fileURLToPath(import.meta.url), endpoint, "descendant"], {
      stdio: ["ignore", "ignore", "ignore", "ipc"],
    });
    await once(descendant, "message");
  }
  const response = await exchange("ready", {
    arguments: arguments_, cwd: process.cwd(), jobs: process.env.CARGO_BUILD_JOBS,
    descendant: descendant?.pid ?? null,
  });
  if (mode === "success") {
    process.stderr.write("compiler diagnostic\n");
    process.stdout.write(`${JSON.stringify(response)}\n`);
    clearInterval(keepAlive);
  } else if (mode === "fails" || mode === "tree-fails") {
    process.stdout.write("O".repeat(512) + "stdout-tail");
    process.stderr.write("E".repeat(512) + "stderr-tail");
    process.exit(7);
  } else if (mode === "stdout-overflow" || mode === "stderr-overflow") {
    const stream = mode === "stdout-overflow" ? process.stdout : process.stderr;
    stream.write("X".repeat(response.data.bytes));
  } else if (mode === "combined-overflow") {
    process.stdout.write("X".repeat(response.data.bytes / 2));
    process.stderr.write("Y".repeat(response.data.bytes / 2));
  } else if (mode === "closes-pipes") {
    process.stdout.end();
    process.stderr.end();
  }
}
