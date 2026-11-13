// Offline fixture recorder. Requires solc 0.8.27; never invoked by the Go gate.
const fs = require("node:fs");
const path = require("node:path");
const crypto = require("node:crypto");
const solc = require("solc");
const root = path.resolve(__dirname, "../../../../../..");
const contracts = path.join(root, "contracts/launchpad");
const sourceName = "contracts/opticals/OpticalRegistry.sol";
if (!solc.version().startsWith("0.8.27+commit.40a35a09")) {
  throw new Error("fixture requires the declared Solidity 0.8.27 compiler");
}
const sources = {};
function load(name) {
  if (sources[name]) return;
  const content = fs.readFileSync(path.join(contracts, name), "utf8");
  sources[name] = { content };
  for (const match of content.matchAll(
    /import\s+(?:[^;]*?from\s+)?["']([^"']+)["']\s*;/g,
  )) {
    load(
      path.posix.normalize(path.posix.join(path.posix.dirname(name), match[1])),
    );
  }
}
load(sourceName);
const input = {
  language: "Solidity",
  sources,
  settings: {
    optimizer: { enabled: true, runs: 200 },
    viaIR: true,
    outputSelection: {
      "*": { "*": ["abi", "evm.bytecode", "evm.deployedBytecode", "metadata"] },
    },
  },
};
const output = JSON.parse(solc.compile(JSON.stringify(input)));
for (const error of output.errors || []) {
  if (error.severity === "error") throw new Error(error.formattedMessage);
}
const encode = (value) => Buffer.from(JSON.stringify(value, null, 2) + "\n");
const sha256 = (value) =>
  crypto.createHash("sha256").update(value).digest("hex");
const artifact = output.contracts[sourceName].OpticalRegistry;
const artifactRaw = encode(artifact);
const buildRaw = encode({ solcLongVersion: solc.version(), input });
// Match encoding/json's recursive map-key ordering for the ABI review identity.
function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === "object")
    return Object.fromEntries(
      Object.keys(value)
        .sort()
        .map((key) => [key, canonical(value[key])]),
    );
  return value;
}
const bundle = {
  name: "OpticalRegistry",
  sourceName,
  artifactPath:
    "artifacts/contracts/opticals/OpticalRegistry.sol/OpticalRegistry.json",
  artifactSha256: sha256(artifactRaw),
  abiSha256: sha256(JSON.stringify(canonical(artifact.abi))),
  sourceSha256: sha256(sources[sourceName].content),
  buildInfoSha256: sha256(buildRaw),
  compilerVersion: solc.version(),
  abi: artifact.abi,
};
const reviewRaw = encode({
  schemaVersion: 1,
  kind: "reviewed-optical-registry",
  bundle,
});
for (const [name, raw] of [
  ["artifact", artifactRaw],
  ["build-input", buildRaw],
  ["review", reviewRaw],
]) {
  fs.writeFileSync(path.join(__dirname, name + ".json"), raw);
}
process.stdout.write("Review SHA-256: " + sha256(reviewRaw) + "\n");
