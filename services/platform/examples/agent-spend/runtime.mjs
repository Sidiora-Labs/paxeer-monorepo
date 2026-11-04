export function assertAgentSpendRuntime(version = process.versions.node) {
  const parts = /^(\d+)\.(\d+)\.(\d+)$/u.exec(version);
  if (!parts || Number(parts[1]) < 22 || Number(parts[1]) === 22 && Number(parts[2]) < 18) {
    throw new Error("node_22_18_required");
  }
}
