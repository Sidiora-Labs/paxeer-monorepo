import fs from "node:fs";
import path from "node:path";
import solc from "solc";

const root = path.join(process.argv[2], "contracts/launchpad");
const contracts = {
  SidioraERC20: "core",
  SidioraNFT: "core",
  SidioraPool: "core",
  FeesRouter: "periphery",
  FeeAccumulator: "data",
  PoolRegistry: "data",
  ProtocolConfig: "protocol",
  UUPSProxy: "test",
};
const sources = {};
function addSource(file) {
  if (sources[file]) return;
  const content = fs.readFileSync(path.join(root, file), "utf8");
  sources[file] = { content };
  for (const match of content.matchAll(/\bimport\s+(?:[^;"']*?\bfrom\s*)?["']([^"']+)["']\s*;/g)) {
    addSource(path.posix.normalize(path.posix.join(path.posix.dirname(file), match[1])));
  }
}
for (const [name, directory] of Object.entries(contracts)) {
  addSource(`contracts/${directory}/${name === "UUPSProxy" ? "MockUUPS" : name}.sol`);
}
const output = JSON.parse(
  solc.compile(
    JSON.stringify({
      language: "Solidity",
      sources,
      settings: {
        optimizer: { enabled: true, runs: 200 },
        viaIR: true,
        evmVersion: "cancun",
        outputSelection: { "*": { "*": ["abi", "evm.bytecode.object"] } },
      },
    })
  )
);
const errors = (output.errors ?? []).filter((error) => error.severity === "error");
if (errors.length) throw new Error(errors.map((error) => error.formattedMessage).join("\n"));
console.log(
  JSON.stringify(
    Object.fromEntries(
      Object.entries(contracts).map(([name, directory]) => {
        const file = `contracts/${directory}/${name === "UUPSProxy" ? "MockUUPS" : name}.sol`;
        const artifact = output.contracts[file][name];
        return [name, { abi: artifact.abi, bytecode: artifact.evm.bytecode.object }];
      })
    )
  )
);
