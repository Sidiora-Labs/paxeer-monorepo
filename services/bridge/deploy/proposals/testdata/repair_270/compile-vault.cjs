const fs = require("node:fs");
const path = require("node:path");
const solc = require("solc");

const root = path.resolve(__dirname, "../../../../../..");
const sourceRoot = path.join(root, "contracts/bridge/evm/src");
const sources = {};
for (const name of ["PaxeerXVault.sol", "BridgeAttestation.sol"]) {
  sources[name] = { content: fs.readFileSync(path.join(sourceRoot, name), "utf8") };
}
const input = {
  language: "Solidity",
  sources,
  settings: {
    optimizer: { enabled: true, runs: 100000 },
    viaIR: true,
    evmVersion: "cancun",
    metadata: { bytecodeHash: "none", appendCBOR: false },
    outputSelection: { "*": { "*": ["abi", "evm.bytecode.object"] } },
  },
};
const output = JSON.parse(
  solc.compile(JSON.stringify(input), {
    import(name) {
      if (!name.startsWith("@openzeppelin/contracts/")) {
        return { error: `unexpected Solidity import ${name}` };
      }
      return { contents: fs.readFileSync(require.resolve(name), "utf8") };
    },
  }),
);
const errors = (output.errors || []).filter((entry) => entry.severity === "error");
if (errors.length) {
  throw new Error(errors.map((entry) => entry.formattedMessage).join("\n"));
}
const vault = output.contracts["PaxeerXVault.sol"].PaxeerXVault;
process.stdout.write(JSON.stringify({ abi: vault.abi, bytecode: vault.evm.bytecode.object }));
